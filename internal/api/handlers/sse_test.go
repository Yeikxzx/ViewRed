package handlers

import (
	"bufio"
	"context"
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
// the connection — it stops reading at the deadline even if the
// goroutine is still alive.
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

	r := gin.New()
	r.GET("/stream/metrics", StreamMetricsHandler(StreamMetricsHandlerDeps{
		Broker: broker,
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

// TestStreamMetricsHandler_RelaysBrokerEvents verifies the
// handler is a pure relay: it subscribes to the "metrics"
// topic and writes every event the broker delivers onto the
// wire as `event: <name>\ndata: <json>\n\n`. The handler
// does NOT start a ticker and does NOT call any storage
// layer — the production ticker in main_wire.go is the sole
// producer. The test broadcasts manually and asserts the
// single event reaches the client intact.
func TestStreamMetricsHandler_RelaysBrokerEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	broker, cancel := startTestBroker(t)
	defer cancel()

	r := gin.New()
	r.GET("/stream/metrics", StreamMetricsHandler(StreamMetricsHandlerDeps{
		Broker: broker,
	}))

	server := httptest.NewServer(r)
	defer server.Close()

	resp, err := http.Get(server.URL + "/stream/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()

	waitForCondition(t, time.Second, "client subscribed", func() bool {
		return broker.ActiveClientCount() == 1
	})

	events := captureSSEEvents(t, resp.Body)
	broker.Broadcast(sse.Event{
		Name:  "metrics",
		Data:  []byte(`{"packets_per_sec":42,"bytes_per_sec":100,"active_connections":1,"device_count":1}`),
		Topic: "metrics",
	})

	select {
	case ev := <-events:
		assert.Equal(t, "metrics", ev.Name)
		assert.Equal(t, `{"packets_per_sec":42,"bytes_per_sec":100,"active_connections":1,"device_count":1}`, ev.Data)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for relayed event")
	}
}

// TestStreamMetricsHandler_FiltersByTopic verifies the
// metrics handler only relays events whose topic matches
// the "metrics" routing key. A broadcast on a different
// topic must not reach the metrics stream.
func TestStreamMetricsHandler_FiltersByTopic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	broker, cancel := startTestBroker(t)
	defer cancel()

	r := gin.New()
	r.GET("/stream/metrics", StreamMetricsHandler(StreamMetricsHandlerDeps{
		Broker: broker,
	}))

	server := httptest.NewServer(r)
	defer server.Close()

	resp, err := http.Get(server.URL + "/stream/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()

	waitForCondition(t, time.Second, "client subscribed", func() bool {
		return broker.ActiveClientCount() == 1
	})

	events := captureSSEEvents(t, resp.Body)

	broker.Broadcast(sse.Event{
		Name:  "alert",
		Data:  []byte(`{"id":"a-1"}`),
		Topic: "alerts",
	})

	select {
	case ev := <-events:
		t.Fatalf("expected no event for foreign topic, got %+v", ev)
	case <-time.After(200 * time.Millisecond):
		// success: no event delivered
	}
}

// TestStreamMetricsHandler_UnsubscribesOnDisconnect verifies
// the handler calls the broker's unsubscribe on client
// disconnect so the broker stops holding an empty buffer for
// a dead client. The test cancels the request context and
// waits for the broker to drop the client.
func TestStreamMetricsHandler_UnsubscribesOnDisconnect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	broker, cancelBroker := startTestBroker(t)
	defer cancelBroker()

	r := gin.New()
	r.GET("/stream/metrics", StreamMetricsHandler(StreamMetricsHandlerDeps{
		Broker: broker,
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

// Compile-time guard: confirm the package-level error helper
// still works after the refactor. (Kept here so a future
// refactor that drops the storage import doesn't silently
// compile and lose coverage.)
var _ storage.Querier = (*storage.MockQuerier)(nil)
var _ = atomic.AddInt32 // keep the import live across edits
var _ = errors.New
