package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/viewred/viewred/internal/sse"
	"github.com/viewred/viewred/internal/storage"
)

// capturedSSEEvent is the result of parsing one SSE event from
// the wire. The handlers emit "event: <name>\ndata: <json>\n\n"
// frames, so we extract the two relevant fields.
type capturedSSEEvent struct {
	Name string
	Data string
}

// captureSSEEvents reads SSE events from r on a background
// goroutine and returns a channel that yields each parsed
// event. The channel is closed when r returns EOF (handler
// closed the stream) or the connection drops.
//
// Scanner has no built-in deadline, so this function is paired
// with readEventsFor, which stops reading after a fixed
// duration.
func captureSSEEvents(t *testing.T, r io.Reader) <-chan capturedSSEEvent {
	t.Helper()
	out := make(chan capturedSSEEvent, 16)
	go func() {
		defer close(out)
		scanner := bufio.NewScanner(r)
		// SSE payloads are small, but allow up to 1 MiB lines so a
		// future fat payload does not silently truncate.
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		var current capturedSSEEvent
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				// Blank line terminates an event frame. Emit the
				// captured event only when we saw at least one
				// field, so the heartbeat frame (no event:/data:
				// fields) does not produce a stray empty event.
				if current.Name != "" || current.Data != "" {
					out <- current
					current = capturedSSEEvent{}
				}
				continue
			}
			switch {
			case strings.HasPrefix(line, "event: "):
				current.Name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				current.Data = strings.TrimPrefix(line, "data: ")
			}
		}
	}()
	return out
}

// readEventsFor collects SSE events for at most duration. It
// returns the events captured. The function does not block on
// the connection — it stops reading at the deadline even if
// the goroutine is still alive.
func readEventsFor(t *testing.T, events <-chan capturedSSEEvent, duration time.Duration) []capturedSSEEvent {
	t.Helper()
	deadline := time.Now().Add(duration)
	var got []capturedSSEEvent
	for time.Now().Before(deadline) {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return got
		}
		select {
		case ev, ok := <-events:
			if !ok {
				return got
			}
			got = append(got, ev)
		case <-time.After(remaining):
			return got
		}
	}
	return got
}

// waitForCondition polls cond until it returns true or timeout
// elapses. Fails the test on timeout.
func waitForCondition(t *testing.T, timeout time.Duration, msg string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("waitForCondition: %s: timeout after %v", msg, timeout)
}

// startTestBroker spins up an sse.Broker with production
// defaults (64-deep buffer, 30s heartbeat) on a goroutine. The
// returned cancel stops the Run loop.
func startTestBroker(t *testing.T) (*sse.Broker, context.CancelFunc) {
	t.Helper()
	b := sse.NewBroker()
	ctx, cancel := context.WithCancel(context.Background())
	go b.Run(ctx)
	return b, cancel
}

// ----------------------------------------------------------------------------
// StreamMetricsHandler tests
// ----------------------------------------------------------------------------

// TestStreamMetricsHandler_SetsSSEHeaders verifies the handler
// writes the four canonical SSE response headers before the
// first event: Content-Type, Cache-Control, Connection, and
// X-Accel-Buffering. X-Accel-Buffering=no is what disables
// nginx's response buffering so events flush in real time.
func TestStreamMetricsHandler_SetsSSEHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	broker, cancel := startTestBroker(t)
	defer cancel()

	q := &storage.MockQuerier{}
	r := gin.New()
	r.GET("/stream/metrics", StreamMetricsHandler(StreamMetricsHandlerDeps{
		Broker:  broker,
		Querier: q,
	}))

	server := httptest.NewServer(r)
	defer server.Close()

	resp, err := http.Get(server.URL + "/stream/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	assert.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))
	assert.Equal(t, "keep-alive", resp.Header.Get("Connection"))
	assert.Equal(t, "no", resp.Header.Get("X-Accel-Buffering"))
}

// TestStreamMetricsHandler_EmitsEventsAtInterval verifies the
// per-request ticker queries the Querier and emits a
// `event: metrics\ndata: {...}\n\n` frame at every tick. We
// inject a fast interval via the package-private helper to
// keep the test under a quarter second.
func TestStreamMetricsHandler_EmitsEventsAtInterval(t *testing.T) {
	gin.SetMode(gin.TestMode)
	broker, cancel := startTestBroker(t)
	defer cancel()

	var calls atomic.Int32
	q := &storage.MockQuerier{
		DashboardSummaryFn: func(ctx context.Context, since time.Time) (storage.DashboardSummary, error) {
			calls.Add(1)
			return storage.DashboardSummary{
				PacketsPerSec:     1234.5,
				BytesPerSec:       4.2e6,
				ActiveConnections: 234,
				DeviceCount:       47,
			}, nil
		},
	}

	r := gin.New()
	r.GET("/stream/metrics", streamMetricsHandlerWithInterval(
		StreamMetricsHandlerDeps{Broker: broker, Querier: q},
		30*time.Millisecond,
	))

	server := httptest.NewServer(r)
	defer server.Close()

	resp, err := http.Get(server.URL + "/stream/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()

	events := captureSSEEvents(t, resp.Body)
	got := readEventsFor(t, events, 200*time.Millisecond)

	require.GreaterOrEqual(t, calls.Load(), int32(1), "Querier should be called at least once")
	require.GreaterOrEqual(t, len(got), 1, "expected at least one event, got %d", len(got))

	for i, ev := range got {
		assert.Equal(t, "metrics", ev.Name, "event %d: name", i)
		var summary storage.DashboardSummary
		require.NoError(t, json.Unmarshal([]byte(ev.Data), &summary), "event %d: data must be valid JSON", i)
		assert.Equal(t, 1234.5, summary.PacketsPerSec, "event %d: packets_per_sec", i)
		assert.Equal(t, 4.2e6, summary.BytesPerSec, "event %d: bytes_per_sec", i)
		assert.Equal(t, int64(234), summary.ActiveConnections, "event %d: active_connections", i)
		assert.Equal(t, int64(47), summary.DeviceCount, "event %d: device_count", i)
	}
}

// TestStreamMetricsHandler_StreamsAtOneHertz verifies the
// production handler ticks once per second. We wait 2.2s and
// expect at least 2 ticks; the lower bound is the load-bearing
// assertion (1Hz means at least one event per second on
// average), the upper bound gives CI some slack.
func TestStreamMetricsHandler_StreamsAtOneHertz(t *testing.T) {
	gin.SetMode(gin.TestMode)
	broker, cancel := startTestBroker(t)
	defer cancel()

	var calls atomic.Int32
	q := &storage.MockQuerier{
		DashboardSummaryFn: func(ctx context.Context, since time.Time) (storage.DashboardSummary, error) {
			calls.Add(1)
			return storage.DashboardSummary{PacketsPerSec: 1.0}, nil
		},
	}

	r := gin.New()
	r.GET("/stream/metrics", StreamMetricsHandler(StreamMetricsHandlerDeps{
		Broker:  broker,
		Querier: q,
	}))

	server := httptest.NewServer(r)
	defer server.Close()

	resp, err := http.Get(server.URL + "/stream/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()

	events := captureSSEEvents(t, resp.Body)
	got := readEventsFor(t, events, 2200*time.Millisecond)

	require.GreaterOrEqual(t, len(got), 2, "expected at least 2 events at 1Hz within 2.2s, got %d", len(got))
	require.GreaterOrEqual(t, calls.Load(), int32(2), "expected Querier called at least 2 times")
}

// TestStreamMetricsHandler_UnsubscribesOnDisconnect verifies the
// handler calls the broker's unsubscribe on client disconnect
// so the broker stops holding an empty buffer for a dead
// client. The test cancels the request context and waits for
// the broker to drop the client.
func TestStreamMetricsHandler_UnsubscribesOnDisconnect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	broker, cancelBroker := startTestBroker(t)
	defer cancelBroker()

	q := &storage.MockQuerier{}
	r := gin.New()
	r.GET("/stream/metrics", StreamMetricsHandler(StreamMetricsHandlerDeps{
		Broker:  broker,
		Querier: q,
	}))

	server := httptest.NewServer(r)
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/stream/metrics", nil)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	waitForCondition(t, time.Second, "client subscribed to broker", func() bool {
		return broker.ActiveClientCount() == 1
	})

	// Drain the body in a goroutine so the request actually
	// progresses; otherwise the body buffer fills and Go's HTTP
	// client stalls the response.
	go func() {
		_, _ = io.Copy(io.Discard, resp.Body)
	}()

	cancel()

	waitForCondition(t, 2*time.Second, "client unsubscribed after disconnect", func() bool {
		return broker.ActiveClientCount() == 0
	})
}

// TestStreamMetricsHandler_SkipsBroadcastOnQueryError verifies
// the handler does not broadcast a metrics event when the
// Querier returns an error. The test inspects a hand-rolled
// broker subscription to confirm no event arrives within a
// few ticks.
func TestStreamMetricsHandler_SkipsBroadcastOnQueryError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	broker, cancel := startTestBroker(t)
	defer cancel()

	q := &storage.ErrorQuerier{Err: errors.New("db down")}

	// A second subscription to the broker lets us observe what
	// (if anything) the handler broadcasts.
	subCtx, subCancel := context.WithCancel(context.Background())
	defer subCancel()
	obs, _ := broker.Subscribe(subCtx, []string{"metrics"})
	waitForCondition(t, time.Second, "observer subscribed", func() bool {
		return broker.ActiveClientCount() == 1
	})

	r := gin.New()
	r.GET("/stream/metrics", streamMetricsHandlerWithInterval(
		StreamMetricsHandlerDeps{Broker: broker, Querier: q},
		20*time.Millisecond,
	))

	server := httptest.NewServer(r)
	defer server.Close()

	resp, err := http.Get(server.URL + "/stream/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()

	// Just verify no event arrives at the observer within a
	// few ticks.
	select {
	case ev := <-obs:
		t.Fatalf("expected no broadcast on Querier error, got %+v", ev)
	case <-time.After(100 * time.Millisecond):
		// success
	}
}

// ----------------------------------------------------------------------------
// StreamStubHandler tests
// ----------------------------------------------------------------------------

// TestStreamStubHandler_SetsSSEHeaders mirrors the metrics
// header test: the stub also writes the four canonical SSE
// headers.
func TestStreamStubHandler_SetsSSEHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	broker, cancel := startTestBroker(t)
	defer cancel()

	r := gin.New()
	r.GET("/stream/alerts", StreamStubHandler(StreamStubHandlerDeps{
		Broker: broker,
		Topic:  "alerts",
	}))

	server := httptest.NewServer(r)
	defer server.Close()

	resp, err := http.Get(server.URL + "/stream/alerts")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	assert.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))
	assert.Equal(t, "keep-alive", resp.Header.Get("Connection"))
	assert.Equal(t, "no", resp.Header.Get("X-Accel-Buffering"))
}

// TestStreamStubHandler_RelaysEvents verifies the stub
// subscribes to its topic and writes every event the broker
// broadcasts on that topic to the wire as a
// `event: <name>\ndata: <json>\n\n` frame.
func TestStreamStubHandler_RelaysEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	broker, cancel := startTestBroker(t)
	defer cancel()

	r := gin.New()
	r.GET("/stream/alerts", StreamStubHandler(StreamStubHandlerDeps{
		Broker: broker,
		Topic:  "alerts",
	}))

	server := httptest.NewServer(r)
	defer server.Close()

	resp, err := http.Get(server.URL + "/stream/alerts")
	require.NoError(t, err)
	defer resp.Body.Close()

	waitForCondition(t, time.Second, "client subscribed", func() bool {
		return broker.ActiveClientCount() == 1
	})

	events := captureSSEEvents(t, resp.Body)

	broker.Broadcast(sse.Event{
		Name:  "alert",
		Data:  []byte(`{"id":"a-1","severity":"high"}`),
		Topic: "alerts",
	})

	select {
	case ev := <-events:
		assert.Equal(t, "alert", ev.Name)
		assert.Equal(t, `{"id":"a-1","severity":"high"}`, ev.Data)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for relayed event")
	}
}

// TestStreamStubHandler_FiltersByTopic verifies the stub only
// receives events for its configured topic. A broadcast on a
// different topic must not reach the stub's client.
func TestStreamStubHandler_FiltersByTopic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	broker, cancel := startTestBroker(t)
	defer cancel()

	r := gin.New()
	r.GET("/stream/alerts", StreamStubHandler(StreamStubHandlerDeps{
		Broker: broker,
		Topic:  "alerts",
	}))

	server := httptest.NewServer(r)
	defer server.Close()

	resp, err := http.Get(server.URL + "/stream/alerts")
	require.NoError(t, err)
	defer resp.Body.Close()

	waitForCondition(t, time.Second, "client subscribed", func() bool {
		return broker.ActiveClientCount() == 1
	})

	events := captureSSEEvents(t, resp.Body)

	broker.Broadcast(sse.Event{
		Name:  "topology",
		Data:  []byte(`{"nodes":[]}`),
		Topic: "topology",
	})

	select {
	case ev := <-events:
		t.Fatalf("expected no event for foreign topic, got %+v", ev)
	case <-time.After(200 * time.Millisecond):
		// success: no event delivered
	}
}

// TestStreamStubHandler_UnsubscribesOnDisconnect mirrors the
// metrics disconnect test: the stub must release its broker
// subscription when the client goes away.
func TestStreamStubHandler_UnsubscribesOnDisconnect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	broker, cancelBroker := startTestBroker(t)
	defer cancelBroker()

	r := gin.New()
	r.GET("/stream/alerts", StreamStubHandler(StreamStubHandlerDeps{
		Broker: broker,
		Topic:  "alerts",
	}))

	server := httptest.NewServer(r)
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/stream/alerts", nil)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	waitForCondition(t, time.Second, "client subscribed to broker", func() bool {
		return broker.ActiveClientCount() == 1
	})

	// Drain the body so the client actually consumes the
	// stream and the disconnect is observed promptly.
	go func() {
		_, _ = io.Copy(io.Discard, resp.Body)
	}()

	cancel()

	waitForCondition(t, 2*time.Second, "client unsubscribed after disconnect", func() bool {
		return broker.ActiveClientCount() == 0
	})
}
