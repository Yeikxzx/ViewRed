package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/viewred/viewred/internal/config"
	"github.com/viewred/viewred/internal/health"
	"github.com/viewred/viewred/internal/storage"
)

// TestNewWireUp_ReturnsNonNilParts asserts the constructor returns a
// fully populated wireUp: broker, querier, and router must all be
// non-nil so main.go can use them without nil checks.
func TestNewWireUp_ReturnsNonNilParts(t *testing.T) {
	w := newWireUp(&config.Config{}, health.New(), &storage.MockQuerier{})

	require.NotNil(t, w.Broker, "broker must be created")
	require.NotNil(t, w.Querier, "querier must be stored")
	require.NotNil(t, w.Router, "router must be built")
}

// TestNewWireUp_ExposesProvidedQuerier asserts the wireUp holds the
// exact Querier the caller passed in (no wrapping, no copying). This
// is what lets main.go inject an ErrorQuerier on the storage-down path
// and have every handler see the 503-mappable error.
func TestNewWireUp_ExposesProvidedQuerier(t *testing.T) {
	q := &storage.MockQuerier{}
	w := newWireUp(&config.Config{}, health.New(), q)

	assert.Same(t, q, w.Querier, "wireUp must expose the same querier the caller passed")
}

// TestNewWireUp_PassesHealthRegistryToRouter asserts the
// /api/v1/health endpoint reports the subsystems registered on the
// passed-in registry. This is the regression fix for "router built
// its own empty health registry": main.go's pre-populated registry
// must reach the endpoint.
func TestNewWireUp_PassesHealthRegistryToRouter(t *testing.T) {
	rg := health.New()
	rg.Add("wire-test-subsystem", func(ctx context.Context) error { return nil })

	w := newWireUp(&config.Config{}, rg, &storage.MockQuerier{})

	server := httptest.NewServer(w.Router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	var report health.Report
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&report))

	require.Len(t, report.Subsystems, 1, "expected the passed-in registry's subsystem to be reported")
	assert.Equal(t, "wire-test-subsystem", report.Subsystems[0].Name)
}

// TestNewWireUp_RouterHasSSEStream asserts the /api/v1/stream/metrics
// route is registered and answers with the canonical SSE headers.
// Uses httptest.NewServer because the handler blocks in c.Stream;
// httptest.NewRecorder would deadlock.
func TestNewWireUp_RouterHasSSEStream(t *testing.T) {
	w := newWireUp(&config.Config{}, health.New(), &storage.MockQuerier{})

	server := httptest.NewServer(w.Router)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/v1/stream/metrics", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
}

// TestNewWireUp_StorageRoutesReturn503OnErrorQuerier asserts that an
// ErrorQuerier injected at wire-up time propagates to the handlers as
// 503 responses. This is the contract that lets main.go skip nil
// checks on the store pointer.
func TestNewWireUp_StorageRoutesReturn503OnErrorQuerier(t *testing.T) {
	w := newWireUp(&config.Config{}, health.New(), &storage.ErrorQuerier{Err: errWireUpStorageDown})

	storageRoutes := []string{
		"/api/v1/dashboard/summary",
		"/api/v1/dashboard/top-talkers",
		"/api/v1/dashboard/protocols",
		"/api/v1/packets",
		"/api/v1/devices",
	}
	for _, path := range storageRoutes {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			wrec := httptest.NewRecorder()
			w.Router.ServeHTTP(wrec, req)
			assert.Equal(t, http.StatusServiceUnavailable, wrec.Code)
		})
	}
}

// errWireUpStorageDown is the error used by the 503 test. Defined
// at package scope so the assertion does not depend on
// storage.ErrorQuerier's default fallback.
var errWireUpStorageDown = &wireUpTestError{message: "wire: storage down"}

// wireUpTestError is a small error type used to inject a known
// sentinel into storage.ErrorQuerier without depending on
// storage-internal error values.
type wireUpTestError struct{ message string }

func (e *wireUpTestError) Error() string { return e.message }

// TestStart_MetricsTickerBroadcastsOnBroker asserts the metrics ticker
// queries the Querier on each tick and broadcasts a metrics event on
// the broker. The test uses a 10ms interval (instead of 1s) so it
// runs in under a second.
func TestStart_MetricsTickerBroadcastsOnBroker(t *testing.T) {
	var called atomic.Int32
	q := &storage.MockQuerier{
		DashboardSummaryFn: func(ctx context.Context, since time.Time) (storage.DashboardSummary, error) {
			called.Add(1)
			return storage.DashboardSummary{
				PacketsPerSec:     42.5,
				BytesPerSec:       1024,
				ActiveConnections: 7,
				DeviceCount:       3,
			}, nil
		},
	}

	w := newWireUpWithInterval(&config.Config{}, health.New(), q, 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	stop := w.Start(ctx)
	// Defer order matters: stop() blocks until both goroutines
	// exit, which requires ctx to be cancelled first. The
	// single-defer pattern makes the ordering explicit instead of
	// relying on LIFO trivia.
	defer func() {
		cancel()
		stop()
	}()

	subCtx, subCancel := context.WithTimeout(ctx, 1*time.Second)
	defer subCancel()

	ch, unsubscribe := w.Broker.Subscribe(subCtx, []string{metricsStreamTopic})
	defer unsubscribe()

	select {
	case ev, ok := <-ch:
		require.True(t, ok, "broker channel closed before any event")
		assert.Equal(t, metricsStreamEventName, ev.Name)
		assert.Equal(t, metricsStreamTopic, ev.Topic)

		var summary storage.DashboardSummary
		require.NoError(t, json.Unmarshal(ev.Data, &summary), "broadcast data must be valid JSON DashboardSummary")
		assert.Equal(t, 42.5, summary.PacketsPerSec)
		assert.Equal(t, int64(7), summary.ActiveConnections)

		assert.GreaterOrEqual(t, called.Load(), int32(1), "Querier.QueryDashboardSummary must be called by the ticker")
	case <-time.After(2 * time.Second):
		t.Fatal("did not receive a metrics broadcast within 2 seconds")
	}
}

// TestStart_MetricsTickerStopsOnContextCancel asserts the metrics
// goroutine exits when ctx is cancelled and that the stop function
// returns promptly. This is the test for "no goroutine leak on
// shutdown" — without it, a cancel race in main.go would silently
// leak a metrics goroutine per restart.
func TestStart_MetricsTickerStopsOnContextCancel(t *testing.T) {
	w := newWireUpWithInterval(&config.Config{}, health.New(), &storage.MockQuerier{}, 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	stop := w.Start(ctx)

	// Give the goroutines a moment to actually start.
	time.Sleep(30 * time.Millisecond)

	// Cancel and ensure stop() returns within a sane budget. A
	// hang here would mean the goroutines did not respect
	// ctx.Done().
	cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		stop()
	}()

	select {
	case <-done:
		// Stop returned — goroutines exited.
	case <-time.After(2 * time.Second):
		t.Fatal("stop() did not return within 2s after context cancel — goroutine leak")
	}
}

// TestStart_QuerierErrorSilentlyDropsTick asserts that a Querier
// error does not panic the ticker or broadcast garbage. The ticker
// is expected to drop the tick and try again on the next interval;
// the test verifies it keeps running across at least one error and
// recovers once the Querier succeeds.
func TestStart_QuerierErrorSilentlyDropsTick(t *testing.T) {
	var calls atomic.Int32
	q := &storage.MockQuerier{
		DashboardSummaryFn: func(ctx context.Context, since time.Time) (storage.DashboardSummary, error) {
			n := calls.Add(1)
			if n < 2 {
				return storage.DashboardSummary{}, errWireUpStorageDown
			}
			return storage.DashboardSummary{PacketsPerSec: 1}, nil
		},
	}

	w := newWireUpWithInterval(&config.Config{}, health.New(), q, 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	stop := w.Start(ctx)
	defer func() {
		cancel()
		stop()
	}()

	subCtx, subCancel := context.WithTimeout(ctx, 2*time.Second)
	defer subCancel()

	ch, unsubscribe := w.Broker.Subscribe(subCtx, []string{metricsStreamTopic})
	defer unsubscribe()

	// We expect the FIRST error tick to be silently dropped (no
	// broadcast on the broker) and a SUCCESSFUL tick to follow.
	var received atomic.Int32
	timeout := time.After(2 * time.Second)
	for received.Load() == 0 {
		select {
		case ev, ok := <-ch:
			if !ok {
				t.Fatal("broker channel closed before success tick")
			}
			var summary storage.DashboardSummary
			require.NoError(t, json.Unmarshal(ev.Data, &summary))
			assert.Equal(t, 1.0, summary.PacketsPerSec)
			received.Add(1)
		case <-timeout:
			t.Fatalf("did not receive a successful broadcast within 2s; calls=%d, received=%d", calls.Load(), received.Load())
		}
	}

	assert.GreaterOrEqual(t, calls.Load(), int32(2), "Querier must be retried after a failure")
}
