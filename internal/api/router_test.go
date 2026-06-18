package api

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

func TestRouterHasHealthEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRouterServesWebAssets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Less(t, w.Code, 500, "router should not 5xx on unknown paths")
}

// TestNewRouterWithRegistryExposesSubsystems is a regression test for the
// whole-branch review finding that main.go built a populated health registry
// but api.NewRouter() constructed its own empty one internally, so the
// /api/v1/health endpoint always reported subsystems: [].
//
// The fix is to expose NewRouterWithRegistry so main.go can inject the
// real registry. This test asserts that subsystems registered on the
// passed-in registry are visible in the HTTP response.
func TestNewRouterWithRegistryExposesSubsystems(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rg := health.New()
	rg.Add("storage", func(ctx context.Context) error { return nil })
	rg.Add("capture", func(ctx context.Context) error { return errors.New("interface down") })

	r := NewRouterWithRegistry(rg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var report health.Report
	err := json.Unmarshal(w.Body.Bytes(), &report)
	require.NoError(t, err)

	assert.Len(t, report.Subsystems, 2, "expected 2 subsystems, got %d", len(report.Subsystems))

	names := make(map[string]health.Status, len(report.Subsystems))
	for _, s := range report.Subsystems {
		names[s.Name] = s.Status
	}
	assert.Contains(t, names, "storage", "expected subsystem 'storage' in report")
	assert.Contains(t, names, "capture", "expected subsystem 'capture' in report")
	assert.Equal(t, health.StatusOK, names["storage"])
	assert.Equal(t, health.StatusDown, names["capture"])
}
