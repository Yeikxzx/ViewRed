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

func TestDashboardSummary_ReturnsKPIs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	q := &storage.MockQuerier{
		DashboardSummaryFn: func(ctx context.Context, since time.Time) (storage.DashboardSummary, error) {
			return storage.DashboardSummary{
				PacketsPerSec:     1234.5,
				BytesPerSec:       4.2e6,
				ActiveConnections: 234,
				DeviceCount:       47,
			}, nil
		},
	}
	r := gin.New()
	r.GET("/api/v1/dashboard/summary", DashboardSummary(q))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/summary", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)

	assert.Equal(t, 1234.5, body["packets_per_sec"])
	assert.Equal(t, 4.2e6, body["bytes_per_sec"])
	assert.Equal(t, float64(234), body["active_connections"])
	assert.Equal(t, float64(47), body["device_count"])
	assert.Contains(t, body, "since")
}

func TestDashboardSummary_Returns503OnError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	q := &storage.ErrorQuerier{Err: errors.New("db down")}
	r := gin.New()
	r.GET("/api/v1/dashboard/summary", DashboardSummary(q))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/summary", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusServiceUnavailable, w.Code)

	var body map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Contains(t, body, "error")
}

func TestDashboardSummary_DefaultsSinceToOneHourAgo(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var capturedSince time.Time
	q := &storage.MockQuerier{
		DashboardSummaryFn: func(ctx context.Context, since time.Time) (storage.DashboardSummary, error) {
			capturedSince = since
			return storage.DashboardSummary{}, nil
		},
	}
	r := gin.New()
	r.GET("/api/v1/dashboard/summary", DashboardSummary(q))

	before := time.Now().Add(-1 * time.Hour)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/summary", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	after := time.Now().Add(-1 * time.Hour)

	require.Equal(t, http.StatusOK, w.Code)
	// captured since must be in [before-2s, after+2s] (small tolerance for clock drift)
	if capturedSince.Before(before.Add(-2*time.Second)) || capturedSince.After(after.Add(2*time.Second)) {
		t.Fatalf("expected since ~ now-1h, got %v (expected between %v and %v)", capturedSince, before, after)
	}
}

func TestDashboardSummary_ParsesSinceQueryParam(t *testing.T) {
	gin.SetMode(gin.TestMode)

	want := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	var capturedSince time.Time
	q := &storage.MockQuerier{
		DashboardSummaryFn: func(ctx context.Context, since time.Time) (storage.DashboardSummary, error) {
			capturedSince = since
			return storage.DashboardSummary{}, nil
		},
	}
	r := gin.New()
	r.GET("/api/v1/dashboard/summary", DashboardSummary(q))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/summary?since=2026-06-18T10:00:00Z", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.True(t, capturedSince.Equal(want), "expected %v, got %v", want, capturedSince)
}

func TestTopTalkers_ReturnsList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	q := &storage.MockQuerier{
		TopTalkersFn: func(ctx context.Context, since time.Time, limit int) ([]storage.TopTalker, error) {
			return []storage.TopTalker{
				{SrcIP: "10.0.0.1", Bytes: 1_000_000, Packets: 10},
				{SrcIP: "10.0.0.2", Bytes: 500_000, Packets: 20},
			}, nil
		},
	}
	r := gin.New()
	r.GET("/api/v1/dashboard/top-talkers", TopTalkers(q))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/top-talkers", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Talkers []struct {
			SrcIP   string `json:"src_ip"`
			Bytes   int64  `json:"bytes"`
			Packets int64  `json:"packets"`
		} `json:"top_talkers"`
		Since time.Time `json:"since"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	require.Len(t, body.Talkers, 2)
	assert.Equal(t, "10.0.0.1", body.Talkers[0].SrcIP)
	assert.Equal(t, int64(1_000_000), body.Talkers[0].Bytes)
	assert.Equal(t, int64(10), body.Talkers[0].Packets)
	assert.Equal(t, "10.0.0.2", body.Talkers[1].SrcIP)
	assert.Equal(t, int64(500_000), body.Talkers[1].Bytes)
	assert.Equal(t, int64(20), body.Talkers[1].Packets)
	assert.Contains(t, w.Body.String(), "since")
}

func TestTopTalkers_Returns503OnError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	q := &storage.ErrorQuerier{Err: errors.New("db down")}
	r := gin.New()
	r.GET("/api/v1/dashboard/top-talkers", TopTalkers(q))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/top-talkers", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestTopTalkers_AppliesLimitParam(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var capturedLimit int
	q := &storage.MockQuerier{
		TopTalkersFn: func(ctx context.Context, since time.Time, limit int) ([]storage.TopTalker, error) {
			capturedLimit = limit
			return nil, nil
		},
	}
	r := gin.New()
	r.GET("/api/v1/dashboard/top-talkers", TopTalkers(q))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/top-talkers?limit=7", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 7, capturedLimit)
}

func TestProtocolDistribution_ReturnsList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	q := &storage.MockQuerier{
		ProtocolDistFn: func(ctx context.Context, since time.Time) ([]storage.ProtocolShare, error) {
			return []storage.ProtocolShare{
				{Protocol: "TCP", Percentage: 60.5},
				{Protocol: "UDP", Percentage: 30.0},
				{Protocol: "ICMP", Percentage: 9.5},
			}, nil
		},
	}
	r := gin.New()
	r.GET("/api/v1/dashboard/protocols", ProtocolDistribution(q))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/protocols", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Protocols []struct {
			Protocol   string  `json:"protocol"`
			Percentage float64 `json:"percentage"`
		} `json:"protocols"`
		Since time.Time `json:"since"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	require.Len(t, body.Protocols, 3)
	assert.Equal(t, "TCP", body.Protocols[0].Protocol)
	assert.Equal(t, 60.5, body.Protocols[0].Percentage)
	assert.Equal(t, "UDP", body.Protocols[1].Protocol)
	assert.Equal(t, 30.0, body.Protocols[1].Percentage)
}

func TestProtocolDistribution_Returns503OnError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	q := &storage.ErrorQuerier{Err: errors.New("db down")}
	r := gin.New()
	r.GET("/api/v1/dashboard/protocols", ProtocolDistribution(q))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/protocols", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusServiceUnavailable, w.Code)
}
