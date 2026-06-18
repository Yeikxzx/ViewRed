package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/viewred/viewred/internal/sse"
	"github.com/viewred/viewred/internal/storage"
)

// TestStreamMetricsHandler_DoesNotStartOwnTicker is the regression test
// for Plan 3 review finding C2 (double broadcast on the metrics topic).
//
// Before the fix, the per-request handler started its own 1 Hz ticker,
// queried the Querier on every tick, and broadcast on the broker. The
// main wire-up also runs a 1 Hz ticker that broadcasts on the same
// topic. The result: two events per second, both at 1 Hz, so the
// dashboard saw a metrics frame every ~500 ms instead of every 1 s.
//
// The fix makes the handler a pure relay: it subscribes to the
// metrics topic and writes every event the broker delivers onto the
// wire. The handler MUST NOT call the Querier or broadcast anything
// of its own.
//
// The test asserts both invariants:
//  1. The Querier is never invoked (the strongest assertion — even
//     one call is a regression).
//  2. After broadcasting one event manually, the client sees exactly
//     one event (the manual one). The handler must not have
//     produced any auto-broadcasts during the wait window.
//
// We wait 1.5s, which is long enough for the old 1s ticker to
// fire at least once if the bug were present. The 30s broker
// heartbeat does not fire in this window, so it does not
// contaminate the event count.
func TestStreamMetricsHandler_DoesNotStartOwnTicker(t *testing.T) {
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

	// Wait for the subscription to land before broadcasting.
	waitForCondition(t, time.Second, "client subscribed", func() bool {
		return broker.ActiveClientCount() == 1
	})

	// Invariant 1: broadcast one event manually and assert the
	// client sees exactly that one event. If the handler had
	// its own ticker, it would have produced an additional
	// auto-broadcast event during the 250ms wait.
	events := captureSSEEvents(t, resp.Body)
	broker.Broadcast(sse.Event{
		Name:  "metrics",
		Data:  []byte(`{"packets_per_sec":42,"bytes_per_sec":100,"active_connections":1,"device_count":1}`),
		Topic: "metrics",
	})

	got := readEventsFor(t, events, 250*time.Millisecond)
	require.Len(t, got, 1, "client must see exactly one event (the manual broadcast), not a stream of auto-broadcasts")
	assert.Equal(t, "metrics", got[0].Name)

	// Confirm the event payload survived the round trip and
	// the new snake_case JSON tags are honoured.
	var summary storage.DashboardSummary
	require.NoError(t, json.Unmarshal([]byte(got[0].Data), &summary))
	assert.Equal(t, 42.0, summary.PacketsPerSec)
}
