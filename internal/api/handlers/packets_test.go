package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/viewred/viewred/internal/capture"
	"github.com/viewred/viewred/internal/storage"
)

// TestPacketsList_ReturnsItemsAndTotal exercises the happy path: the
// handler MUST forward the parsed PacketFilter to the querier and emit
// a JSON body with "items" (the captured packet events) and "total"
// (the unpaginated row count from the COUNT query). The two fields
// are the public contract — every other assertion is implementation
// detail.
func TestPacketsList_ReturnsItemsAndTotal(t *testing.T) {
	gin.SetMode(gin.TestMode)

	want := capture.PacketEvent{
		Timestamp: time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC),
		SrcIP:     "10.0.0.1",
		DstIP:     "10.0.0.2",
		SrcPort:   1234,
		DstPort:   80,
		Protocol:  "TCP",
		Length:    64,
	}
	q := &storage.MockQuerier{
		PacketsFn: func(ctx context.Context, filter storage.PacketFilter) ([]capture.PacketEvent, int, error) {
			return []capture.PacketEvent{want}, 1, nil
		},
	}
	r := gin.New()
	r.GET("/api/v1/packets", PacketsList(q))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/packets", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Items []struct {
			Time     time.Time `json:"time"`
			SrcIP    string    `json:"src_ip"`
			DstIP    string    `json:"dst_ip"`
			SrcPort  uint16    `json:"src_port"`
			DstPort  uint16    `json:"dst_port"`
			Protocol string    `json:"protocol"`
			Length   int       `json:"length"`
		} `json:"packets"`
		Total  int `json:"total"`
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)

	require.Len(t, body.Items, 1)
	assert.Equal(t, 1, body.Total)
	assert.Equal(t, 100, body.Limit)
	assert.Equal(t, 0, body.Offset)
	assert.Equal(t, "10.0.0.1", body.Items[0].SrcIP)
	assert.Equal(t, "10.0.0.2", body.Items[0].DstIP)
	assert.Equal(t, "TCP", body.Items[0].Protocol)
	assert.Equal(t, uint16(1234), body.Items[0].SrcPort)
	assert.Equal(t, uint16(80), body.Items[0].DstPort)
	assert.Equal(t, 64, body.Items[0].Length)
	assert.True(t, body.Items[0].Time.Equal(want.Timestamp))
}

// TestPacketsList_EmptyResult exercises the "no rows" branch. The
// handler MUST still return 200 with an empty (non-nil) items slice
// and total=0. A non-2xx here would be wrong because an empty result
// is a successful query, not a failure.
func TestPacketsList_EmptyResult(t *testing.T) {
	gin.SetMode(gin.TestMode)

	q := &storage.MockQuerier{
		PacketsFn: func(ctx context.Context, filter storage.PacketFilter) ([]capture.PacketEvent, int, error) {
			return []capture.PacketEvent{}, 0, nil
		},
	}
	r := gin.New()
	r.GET("/api/v1/packets", PacketsList(q))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/packets?protocol=UNKNOWN", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Items  []any `json:"packets"`
		Total  int   `json:"total"`
		Limit  int   `json:"limit"`
		Offset int   `json:"offset"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, []any{}, body.Items, "items must be empty array, not null")
	assert.Equal(t, 0, body.Total)
}

// TestPacketsList_Returns503OnError is the degraded-DB path. The
// ErrorQuerier pattern (see internal/storage/querier.go) is the
// production code path when storage initialization fails in main.go,
// so this test is the one that proves the wiring is correct: error
// in -> 503 out, no nil-check needed in the handler.
func TestPacketsList_Returns503OnError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	q := &storage.ErrorQuerier{Err: errors.New("db down")}
	r := gin.New()
	r.GET("/api/v1/packets", PacketsList(q))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/packets", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusServiceUnavailable, w.Code)

	var body map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Contains(t, body, "error")
}

// TestPacketsList_DefaultsSinceToOneHourAgo verifies the spec-mandated
// "since=1h ago" default. A 2-second tolerance accounts for clock
// drift between the test's time.Now() call and the handler's.
func TestPacketsList_DefaultsSinceToOneHourAgo(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var capturedSince time.Time
	q := &storage.MockQuerier{
		PacketsFn: func(ctx context.Context, filter storage.PacketFilter) ([]capture.PacketEvent, int, error) {
			capturedSince = filter.Since
			return nil, 0, nil
		},
	}
	r := gin.New()
	r.GET("/api/v1/packets", PacketsList(q))

	before := time.Now().Add(-1 * time.Hour)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/packets", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	after := time.Now().Add(-1 * time.Hour)

	require.Equal(t, http.StatusOK, w.Code)
	if capturedSince.Before(before.Add(-2*time.Second)) || capturedSince.After(after.Add(2*time.Second)) {
		t.Fatalf("expected since ~ now-1h, got %v (expected between %v and %v)", capturedSince, before, after)
	}
}

// TestPacketsList_DefaultsLimitTo100 covers the "limit=100" default
// from the task summary. The storage layer's default is 50, so this
// test is the one that proves the handler applies its own 100 default
// (not relying on storage's 50).
func TestPacketsList_DefaultsLimitTo100(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var capturedLimit int
	q := &storage.MockQuerier{
		PacketsFn: func(ctx context.Context, filter storage.PacketFilter) ([]capture.PacketEvent, int, error) {
			capturedLimit = filter.Limit
			return nil, 0, nil
		},
	}
	r := gin.New()
	r.GET("/api/v1/packets", PacketsList(q))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/packets", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 100, capturedLimit, "default limit should be 100, not the storage layer's 50")
}

// TestPacketsList_DefaultsOffsetToZero covers the "offset=0" default.
// Zero is the natural zero-value, but a buggy handler that misread
// "offset" as a different query param would fail this test.
func TestPacketsList_DefaultsOffsetToZero(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var capturedOffset int
	q := &storage.MockQuerier{
		PacketsFn: func(ctx context.Context, filter storage.PacketFilter) ([]capture.PacketEvent, int, error) {
			capturedOffset = filter.Offset
			return nil, 0, nil
		},
	}
	r := gin.New()
	r.GET("/api/v1/packets", PacketsList(q))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/packets", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 0, capturedOffset)
}

// TestPacketsList_ParsesAllQueryParams uses a single request to
// exercise the full query-param surface. The captured filter must
// reflect every param the handler claims to parse; this is the
// integration check that the parsing function and the handler agree.
func TestPacketsList_ParsesAllQueryParams(t *testing.T) {
	gin.SetMode(gin.TestMode)

	wantSince := time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC)
	wantUntil := time.Date(2026, 6, 18, 11, 0, 0, 0, time.UTC)

	var captured storage.PacketFilter
	q := &storage.MockQuerier{
		PacketsFn: func(ctx context.Context, filter storage.PacketFilter) ([]capture.PacketEvent, int, error) {
			captured = filter
			return nil, 0, nil
		},
	}
	r := gin.New()
	r.GET("/api/v1/packets", PacketsList(q))

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/packets?since=2026-06-18T09:00:00Z&until=2026-06-18T11:00:00Z&protocol=TCP&src_ip=10.0.0.1&dst_ip=10.0.0.2&limit=25&offset=50",
		nil,
	)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.True(t, captured.Since.Equal(wantSince), "Since: want %v, got %v", wantSince, captured.Since)
	assert.True(t, captured.Until.Equal(wantUntil), "Until: want %v, got %v", wantUntil, captured.Until)
	assert.Equal(t, "TCP", captured.Protocol)
	assert.Equal(t, "10.0.0.1", captured.SrcIP)
	assert.Equal(t, "10.0.0.2", captured.DstIP)
	assert.Equal(t, 25, captured.Limit)
	assert.Equal(t, 50, captured.Offset)
}

// TestPacketsList_RejectsInvalidParams is table-driven so we cover
// every "400 on invalid param" branch in one place. The cases mirror
// the spec's "Invalid limit | limit=-1 | 400" scenario, extended to
// every documented query param. The handler MUST reject each one with
// 400 and a JSON error body — silent fallback to a default would mask
// client bugs (and the dashboard handler's "silent fallback" is a
// deviation from the spec that this handler does NOT repeat).
func TestPacketsList_RejectsInvalidParams(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name string
		url  string
	}{
		{"invalid since", "/api/v1/packets?since=not-a-time"},
		{"invalid until", "/api/v1/packets?until=yesterday"},
		{"negative limit", "/api/v1/packets?limit=-1"},
		{"non-numeric limit", "/api/v1/packets?limit=abc"},
		{"negative offset", "/api/v1/packets?offset=-5"},
		{"non-numeric offset", "/api/v1/packets?offset=xyz"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			// Querier must NOT be called when the params are invalid —
			// asserting this via the mock's invocation count proves the
			// handler short-circuits before reaching storage.
			called := false
			q := &storage.MockQuerier{
				PacketsFn: func(ctx context.Context, filter storage.PacketFilter) ([]capture.PacketEvent, int, error) {
					called = true
					return nil, 0, nil
				},
			}
			r := gin.New()
			r.GET("/api/v1/packets", PacketsList(q))

			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			require.Equal(t, http.StatusBadRequest, w.Code, "expected 400 for %s, got %d (body=%s)", tt.url, w.Code, w.Body.String())
			assert.False(t, called, "querier must not be called on invalid params")

			var body map[string]any
			err := json.Unmarshal(w.Body.Bytes(), &body)
			require.NoError(t, err)
			assert.Contains(t, body, "error", "400 body must include an error field")
		})
	}
}

// TestPacketsList_ClampsLimitToMax covers the "limit too high" case.
// Unlike an outright invalid value (which 400s), a too-high limit is a
// legitimate request with a benign client mistake — clamp to the max
// rather than 400. The cap matches the storage layer's
// maxPacketsLimit=500 so the handler and storage agree.
func TestPacketsList_ClampsLimitToMax(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var capturedLimit int
	q := &storage.MockQuerier{
		PacketsFn: func(ctx context.Context, filter storage.PacketFilter) ([]capture.PacketEvent, int, error) {
			capturedLimit = filter.Limit
			return nil, 0, nil
		},
	}
	r := gin.New()
	r.GET("/api/v1/packets", PacketsList(q))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/packets?limit=99999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 500, capturedLimit, "limit must be clamped to 500, not echoed verbatim")
}
