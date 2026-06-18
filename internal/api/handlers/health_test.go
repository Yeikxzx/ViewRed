package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/viewred/viewred/internal/health"
)

func TestHealthReturnsOK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rg := health.New()

	r := gin.New()
	r.GET("/api/v1/health", Health(rg))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)

	assert.Equal(t, "ok", body["status"])
	assert.Contains(t, body, "uptime_seconds")
	assert.Contains(t, body, "subsystems")
}

func TestHealthRejectsNonGET(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rg := health.New()
	r := gin.New()
	r.GET("/api/v1/health", Health(rg))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHealthReturnsAggregatedReport(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rg := health.New()
	rg.Add("db", func(ctx context.Context) error { return nil })
	rg.Add("capture", func(ctx context.Context) error { return nil })

	r := gin.New()
	r.GET("/api/v1/health", Health(rg))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var raw map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &raw)
	require.NoError(t, err)

	assert.Equal(t, "ok", raw["status"])
	assert.Contains(t, raw, "uptime_seconds")
	assert.Contains(t, raw, "subsystems")

	var report health.Report
	err = json.Unmarshal(w.Body.Bytes(), &report)
	require.NoError(t, err)

	assert.Equal(t, health.StatusOK, report.Status)
	assert.Len(t, report.Subsystems, 2)

	names := make(map[string]bool, len(report.Subsystems))
	for _, s := range report.Subsystems {
		names[s.Name] = true
	}
	assert.True(t, names["db"], "expected subsystem 'db' in report")
	assert.True(t, names["capture"], "expected subsystem 'capture' in report")
}

func TestHealthReturns200ForDegraded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rg := health.New()
	rg.Add("db", func(ctx context.Context) error { return nil })
	rg.Add("capture", func(ctx context.Context) error { return errors.New("down") })

	r := gin.New()
	r.GET("/api/v1/health", Health(rg))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var report health.Report
	err := json.Unmarshal(w.Body.Bytes(), &report)
	require.NoError(t, err)

	assert.Equal(t, health.StatusDegraded, report.Status)
}

func TestHealthReturns503WhenAllDown(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rg := health.New()
	rg.Add("db", func(ctx context.Context) error { return errors.New("db unreachable") })
	rg.Add("capture", func(ctx context.Context) error { return errors.New("capture stopped") })

	r := gin.New()
	r.GET("/api/v1/health", Health(rg))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusServiceUnavailable, w.Code)

	var report health.Report
	err := json.Unmarshal(w.Body.Bytes(), &report)
	require.NoError(t, err)

	assert.Equal(t, health.StatusDown, report.Status)
	assert.Len(t, report.Subsystems, 2)
}
