// Package handlers contains HTTP handlers for the ViewRed API.
//
// This file hosts the Server-Sent Events (SSE) handlers that power
// the React dashboard's live updates. Two handlers live here:
//
//   - StreamMetricsHandler: a producer+consumer that ticks once per
//     second, queries the dashboard summary via the storage layer,
//     and broadcasts a metrics event on the broker. The handler is
//     also a consumer of that topic and relays every event onto the
//     wire. The per-request ticker keeps the test suite fast and
//     matches the spec in Plan 3 Task 15; main.go later replaces
//     this with a single shared ticker in Task 17.
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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/viewred/viewred/internal/sse"
	"github.com/viewred/viewred/internal/storage"
)

// defaultMetricsStreamInterval is the cadence at which
// StreamMetricsHandler queries the dashboard and broadcasts a
// metrics event. 1 Hz matches what the React dashboard renders
// and keeps the SQL aggregates cheap.
const defaultMetricsStreamInterval = 1 * time.Second

// metricsStreamTopic is the broker routing key the metrics
// handler subscribes to and broadcasts on. The frontend
// EventSource listens on /stream/metrics and reaches the broker
// through this same topic.
const metricsStreamTopic = "metrics"

// metricsStreamEventName is the SSE event name the metrics
// handler emits. Matches the value the React dashboard
// EventSource filters on.
const metricsStreamEventName = "metrics"

// metricsStreamLookback is the lookback window used when
// querying the dashboard. One hour matches the dashboard
// summary's default and gives the SSE client a meaningful rate
// even when no recent activity exists.
const metricsStreamLookback = 1 * time.Hour

// StreamMetricsHandlerDeps groups the dependencies of
// StreamMetricsHandler. Both fields are required.
type StreamMetricsHandlerDeps struct {
	Broker  *sse.Broker
	Querier storage.Querier
}

// StreamMetricsHandler streams dashboard metrics at 1 Hz.
//
// On each tick the handler queries the dashboard summary via
// Querier and broadcasts an Event on the broker's "metrics"
// topic. The handler is also subscribed to that topic, so it
// relays every event (including its own broadcasts) onto the
// wire as `event: metrics\ndata: <json>\n\n`. On client
// disconnect the handler unsubscribes and stops the ticker so
// the broker can reclaim the buffer slot.
func StreamMetricsHandler(deps StreamMetricsHandlerDeps) gin.HandlerFunc {
	return streamMetricsHandlerWithInterval(deps, defaultMetricsStreamInterval)
}

// streamMetricsHandlerWithInterval is the test-friendly form
// of StreamMetricsHandler that lets the unit test inject a
// faster tick. Production callers should use
// StreamMetricsHandler so the cadence stays at 1 Hz.
func streamMetricsHandlerWithInterval(deps StreamMetricsHandlerDeps, interval time.Duration) gin.HandlerFunc {
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

		// Subscribe BEFORE starting the ticker so a tick that
		// fires immediately is not lost. The broker's
		// Subscribe is bounded by c.Request.Context(), so a
		// client that disconnects before the broker accepts
		// the registration will not deadlock.
		ch, unsubscribe := deps.Broker.Subscribe(c.Request.Context(), []string{metricsStreamTopic})
		defer unsubscribe()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

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
			case <-ticker.C:
				broadcastMetricsTick(c, deps)
				return true
			case <-c.Request.Context().Done():
				// Client disconnected. Returning false
				// makes gin's Stream exit cleanly.
				return false
			}
		})
	}
}

// broadcastMetricsTick runs one cycle of the metrics loop:
// query the dashboard, marshal the result, and broadcast an
// Event on the broker. On any error (storage down, marshal
// failure) the tick is silently dropped — the next tick will
// try again, and a successful query will heal the stream.
//
// ctx is the request context, so when the client disconnects
// the underlying query aborts promptly.
func broadcastMetricsTick(c *gin.Context, deps StreamMetricsHandlerDeps) {
	summary, err := deps.Querier.QueryDashboardSummary(c.Request.Context(), time.Now().Add(-metricsStreamLookback))
	if err != nil {
		return
	}
	data, err := json.Marshal(summary)
	if err != nil {
		return
	}
	deps.Broker.Broadcast(sse.Event{
		Name:  metricsStreamEventName,
		Data:  data,
		Topic: metricsStreamTopic,
	})
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
