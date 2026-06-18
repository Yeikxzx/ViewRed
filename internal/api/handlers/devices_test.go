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

	"github.com/viewred/viewred/internal/storage"
)

// TestDevicesList_ReturnsItemsAndTotal exercises the happy path: the
// handler MUST forward the parsed DeviceFilter to the querier and emit
// a JSON body with "items" (the discovered device rows) and "total"
// (the unpaginated count). The two fields are the public contract —
// every other assertion is implementation detail. Mirrors the
// TestPacketsList_ReturnsItemsAndTotal pattern but uses Device and
// the DevicesList entry point.
func TestDevicesList_ReturnsItemsAndTotal(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	risk := 0.87
	want := storage.Device{
		ID:        42,
		IP:        "10.0.0.5",
		MAC:       "aa:bb:cc:dd:ee:ff",
		Vendor:    "Acme",
		Hostname:  "laptop-01",
		FirstSeen: now.Add(-24 * time.Hour),
		LastSeen:  now,
		RiskScore: &risk,
	}
	q := &storage.MockQuerier{
		DevicesFn: func(ctx context.Context, filter storage.DeviceFilter) ([]storage.Device, int, error) {
			return []storage.Device{want}, 1, nil
		},
	}
	r := gin.New()
	r.GET("/api/v1/devices", DevicesList(q))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Items []struct {
			ID        int64     `json:"id"`
			IP        string    `json:"ip"`
			MAC       string    `json:"mac"`
			Vendor    string    `json:"vendor"`
			Hostname  string    `json:"hostname"`
			FirstSeen time.Time `json:"first_seen"`
			LastSeen  time.Time `json:"last_seen"`
			RiskScore *float64  `json:"risk_score"`
		} `json:"items"`
		Total int `json:"total"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)

	require.Len(t, body.Items, 1)
	assert.Equal(t, 1, body.Total)
	assert.Equal(t, int64(42), body.Items[0].ID)
	assert.Equal(t, "10.0.0.5", body.Items[0].IP)
	assert.Equal(t, "aa:bb:cc:dd:ee:ff", body.Items[0].MAC)
	assert.Equal(t, "Acme", body.Items[0].Vendor)
	assert.Equal(t, "laptop-01", body.Items[0].Hostname)
	assert.True(t, body.Items[0].FirstSeen.Equal(want.FirstSeen))
	assert.True(t, body.Items[0].LastSeen.Equal(want.LastSeen))
	require.NotNil(t, body.Items[0].RiskScore)
	assert.InDelta(t, 0.87, *body.Items[0].RiskScore, 1e-9)
}

// TestDevicesList_NilRiskScoreSurvivesJSON proves the JSON contract for
// the nullable risk_score field. A device that has not been scored yet
// MUST serialize as `null`, not as 0.0 — the two are semantically
// different ("not yet known" vs "low risk") and the React device
// table renders them differently. A bug here would be invisible in
// tests that always populate the pointer.
func TestDevicesList_NilRiskScoreSurvivesJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	q := &storage.MockQuerier{
		DevicesFn: func(ctx context.Context, filter storage.DeviceFilter) ([]storage.Device, int, error) {
			return []storage.Device{
				{ID: 1, IP: "10.0.0.1", RiskScore: nil},
			}, 1, nil
		},
	}
	r := gin.New()
	r.GET("/api/v1/devices", DevicesList(q))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	// Decode into a generic map so we can assert the literal JSON
	// token for risk_score (null) instead of a Go zero-value that
	// would hide the distinction.
	var body map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)

	items, ok := body["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)

	item := items[0].(map[string]any)
	risk, present := item["risk_score"]
	assert.True(t, present, "risk_score key must be present in JSON even when nil")
	assert.Nil(t, risk, "nil RiskScore must serialize as JSON null, not 0.0")
}

// TestDevicesList_EmptyResult exercises the "no rows" branch. The
// handler MUST still return 200 with an empty (non-nil) items slice
// and total=0. A non-2xx here would be wrong because an empty result
// is a successful query, not a failure.
func TestDevicesList_EmptyResult(t *testing.T) {
	gin.SetMode(gin.TestMode)

	q := &storage.MockQuerier{
		DevicesFn: func(ctx context.Context, filter storage.DeviceFilter) ([]storage.Device, int, error) {
			return []storage.Device{}, 0, nil
		},
	}
	r := gin.New()
	r.GET("/api/v1/devices", DevicesList(q))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Items []any `json:"items"`
		Total int   `json:"total"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, []any{}, body.Items, "items must be empty array, not null")
	assert.Equal(t, 0, body.Total)
}

// TestDevicesList_Returns503OnError is the degraded-DB path. The
// ErrorQuerier pattern (see internal/storage/querier.go) is the
// production code path when storage initialization fails in main.go,
// so this test is the one that proves the wiring is correct: error
// in -> 503 out, no nil-check needed in the handler.
func TestDevicesList_Returns503OnError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	q := &storage.ErrorQuerier{Err: errors.New("db down")}
	r := gin.New()
	r.GET("/api/v1/devices", DevicesList(q))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusServiceUnavailable, w.Code)

	var body map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Contains(t, body, "error")
}

// TestDevicesList_DefaultsLimitTo100 covers the "limit=100" default
// from the task summary. The storage layer's default is 50, so this
// test is the one that proves the handler applies its own 100 default
// (not relying on storage's 50).
func TestDevicesList_DefaultsLimitTo100(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var capturedLimit int
	q := &storage.MockQuerier{
		DevicesFn: func(ctx context.Context, filter storage.DeviceFilter) ([]storage.Device, int, error) {
			capturedLimit = filter.Limit
			return nil, 0, nil
		},
	}
	r := gin.New()
	r.GET("/api/v1/devices", DevicesList(q))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 100, capturedLimit, "default limit should be 100, not the storage layer's 50")
}

// TestDevicesList_DefaultsOffsetToZero covers the "offset=0" default.
// Zero is the natural zero-value, but a buggy handler that misread
// "offset" as a different query param would fail this test.
func TestDevicesList_DefaultsOffsetToZero(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var capturedOffset int
	q := &storage.MockQuerier{
		DevicesFn: func(ctx context.Context, filter storage.DeviceFilter) ([]storage.Device, int, error) {
			capturedOffset = filter.Offset
			return nil, 0, nil
		},
	}
	r := gin.New()
	r.GET("/api/v1/devices", DevicesList(q))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 0, capturedOffset)
}

// TestDevicesList_ParsesAllQueryParams uses a single request to
// exercise the full query-param surface. The captured filter must
// reflect every param the handler claims to parse; this is the
// integration check that the parsing function and the handler agree.
func TestDevicesList_ParsesAllQueryParams(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var captured storage.DeviceFilter
	q := &storage.MockQuerier{
		DevicesFn: func(ctx context.Context, filter storage.DeviceFilter) ([]storage.Device, int, error) {
			captured = filter
			return nil, 0, nil
		},
	}
	r := gin.New()
	r.GET("/api/v1/devices", DevicesList(q))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices?limit=25&offset=50", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 25, captured.Limit)
	assert.Equal(t, 50, captured.Offset)
}

// TestDevicesList_RejectsInvalidParams is table-driven so we cover
// every "400 on invalid param" branch in one place. The handler MUST
// reject each one with 400 and a JSON error body — silent fallback
// to a default would mask client bugs.
func TestDevicesList_RejectsInvalidParams(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name string
		url  string
	}{
		{"negative limit", "/api/v1/devices?limit=-1"},
		{"non-numeric limit", "/api/v1/devices?limit=abc"},
		{"negative offset", "/api/v1/devices?offset=-5"},
		{"non-numeric offset", "/api/v1/devices?offset=xyz"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			// Querier must NOT be called when the params are invalid —
			// asserting this via the mock's invocation count proves the
			// handler short-circuits before reaching storage.
			called := false
			q := &storage.MockQuerier{
				DevicesFn: func(ctx context.Context, filter storage.DeviceFilter) ([]storage.Device, int, error) {
					called = true
					return nil, 0, nil
				},
			}
			r := gin.New()
			r.GET("/api/v1/devices", DevicesList(q))

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

// TestDevicesList_ClampsLimitToMax covers the "limit too high" case.
// Unlike an outright invalid value (which 400s), a too-high limit is a
// legitimate request with a benign client mistake — clamp to the max
// rather than 400. The cap matches the storage layer's
// maxDevicesLimit=500 so the handler and storage agree.
func TestDevicesList_ClampsLimitToMax(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var capturedLimit int
	q := &storage.MockQuerier{
		DevicesFn: func(ctx context.Context, filter storage.DeviceFilter) ([]storage.Device, int, error) {
			capturedLimit = filter.Limit
			return nil, 0, nil
		},
	}
	r := gin.New()
	r.GET("/api/v1/devices", DevicesList(q))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices?limit=99999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 500, capturedLimit, "limit must be clamped to 500, not echoed verbatim")
}
