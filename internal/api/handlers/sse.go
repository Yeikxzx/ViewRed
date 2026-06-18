// Package handlers contains HTTP handlers for the ViewRed API.
//
// This file hosts the Server-Sent Events (SSE) handlers that power
// the React dashboard's live updates. Two handlers live here:
//
//   - StreamMetricsHandler: a pure relay that subscribes to the
//     broker's "metrics" topic and writes every event it delivers
//     onto the wire. The handler does NOT start a ticker or call
//     the storage Querier; the shared 1 Hz ticker in
//     cmd/viewred/main_wire.go is the sole producer on the topic.
//     The per-request ticker that used to live here was removed in
//     the Plan 3 review follow-up: running two producers (per-request
//     and shared) on the same topic doubled the event rate and
//     broke the dashboard.
//
//   - StreamStubHandler: a thin consumer for the alerts, topology,
//     and LLM streams that exist in the UI but do not yet have
//     backing producers. The stub subscribes to a topic and writes
//     every event to the wire; future plans will add the producers
//     that feed those topics.
//
// Both handlers set the canonical SSE headers
// (Content-Type: text/event-stream, Cache-Control: no-cache,
// Connection: keep-alive, X-Accel-Buffering: no) and frame every
// message as `event: <name>\ndata: <json>\n\n`. X-Accel-Buffering
// is the nginx hint that disables proxy buffering, so events flush
// in real time.
package handlers

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/viewred/viewred/internal/sse"
)

// metricsStreamTopic is the broker routing key the metrics
// handler subscribes to. The frontend EventSource listens on
// /stream/metrics and reaches the broker through this same topic.
// The shared ticker in cmd/viewred/main_wire.go broadcasts on the
// same topic; the handler is a pure relay.
const metricsStreamTopic = "metrics"

// StreamMetricsHandlerDeps groups the dependencies of
// StreamMetricsHandler. Only the Broker is required — the handler
// is a pure relay and does not need the storage Querier.
type StreamMetricsHandlerDeps struct {
	Broker *sse.Broker
}

// StreamMetricsHandler serves GET /api/v1/stream/metrics. It sets
// the canonical SSE response headers, subscribes to the broker's
// "metrics" topic, and writes every event the broker delivers onto
// the wire as `event: <name>\ndata: <json>\n\n`. The handler does
// NOT start a ticker and does NOT call any storage layer — the
// shared 1 Hz ticker in cmd/viewred/main_wire.go is the sole
// producer on the topic, so all SSE clients see the same metrics
// stream at the same cadence.
//
// On client disconnect the handler unsubscribes so the broker
// can reclaim the buffer slot. The broker also auto-unsubscribes
// when the request context is cancelled, so the explicit
// unsubscribe is the panic-safety net.
func StreamMetricsHandler(deps StreamMetricsHandlerDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		// SSE headers MUST be set before the first Write. Once
		// gin's writer flushes the status line, headers are
		// locked.
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("X-Accel-Buffering", "no")
		c.Writer.WriteHeader(http.StatusOK)
		c.Writer.Flush()

		// Subscribe to the metrics topic. The broker's
		// Subscribe is bounded by c.Request.Context(), so a
		// client that disconnects before the broker accepts
		// the registration will not deadlock.
		ch, unsubscribe := deps.Broker.Subscribe(c.Request.Context(), []string{metricsStreamTopic})
		defer unsubscribe()

		c.Stream(func(w io.Writer) bool {
			select {
			case ev, ok := <-ch:
				if !ok {
					// Broker closed the channel (either
					// shutdown or the client was
					// unregistered). Stop streaming.
					return false
				}
				writeSSEFrame(w, ev.Name, ev.Data)
				return true
			case <-c.Request.Context().Done():
				// Client disconnected. Returning false
				// makes gin's Stream exit cleanly.
				return false
			}
		})
	}
}

// StreamStubHandlerDeps groups the dependencies of
// StreamStubHandler. Both fields are required.
type StreamStubHandlerDeps struct {
	Broker *sse.Broker
	Topic  string
}

// StreamStubHandler is the placeholder for the alerts,
// topology, and LLM streams. It sets the SSE headers,
// subscribes to the configured topic, and relays every event
// the broker delivers onto the wire. There is no ticker and
// no query: the stub has no producer. Real producers land in
// later plans (alerts in Plan 5, topology in Plan 6, LLM in
// Plan 9).
//
// The Topic field is the broker routing key. A broadcast on
// any other topic is filtered out by the broker, so the stub
// never sees cross-topic noise.
func StreamStubHandler(deps StreamStubHandlerDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("X-Accel-Buffering", "no")
		c.Writer.WriteHeader(http.StatusOK)
		c.Writer.Flush()

		ch, unsubscribe := deps.Broker.Subscribe(c.Request.Context(), []string{deps.Topic})
		defer unsubscribe()

		c.Stream(func(w io.Writer) bool {
			select {
			case ev, ok := <-ch:
				if !ok {
					return false
				}
				writeSSEFrame(w, ev.Name, ev.Data)
				return true
			case <-c.Request.Context().Done():
				return false
			}
		})
	}
}

// writeSSEFrame writes a single SSE event frame to w. The
// format follows the SSE spec: an `event:` line, a `data:`
// line, then a blank line that terminates the event.
//
// Multi-line data payloads would need to be split into
// multiple `data:` lines; the current handlers always emit
// single-line JSON, so a single `data:` line is correct.
func writeSSEFrame(w io.Writer, name string, data []byte) {
	// Fprintf returns the byte count and an error. We do not
	// surface the error: gin's Stream already detects client
	// disconnect on the next iteration, and an error here
	// would just abort the stream a few bytes earlier.
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, data)
}
