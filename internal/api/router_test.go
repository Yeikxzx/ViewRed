package api

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

	"github.com/viewred/viewred/internal/api/middleware"
	"github.com/viewred/viewred/internal/config"
	"github.com/viewred/viewred/internal/health"
	"github.com/viewred/viewred/internal/sse"
	"github.com/viewred/viewred/internal/storage"
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

// ----------------------------------------------------------------------------
// NewRouterWithDeps tests (Plan 3 Task 16)
// ----------------------------------------------------------------------------

// startRouterTestBroker spins up an sse.Broker with production
// defaults on a goroutine and returns a cancel that stops the Run
// loop. The broker's 30s heartbeat is fine for these tests because
// none of them wait for a heartbeat; the cancel ensures the goroutine
// exits cleanly when the test ends.
func startRouterTestBroker(t *testing.T) (*sse.Broker, context.CancelFunc) {
	t.Helper()
	b := sse.NewBroker()
	ctx, cancel := context.WithCancel(context.Background())
	go b.Run(ctx)
	return b, cancel
}

// makeDeps builds a RouterDeps with the given querier and a real
// broker. The CORS config is empty by default so the CORS
// middleware is not wired in (the dedicated CORS test passes a
// non-empty config).
func makeDeps(q storage.Querier) RouterDeps {
	return RouterDeps{
		Health:  health.New(),
		Querier: q,
		Broker:  sse.NewBroker(),
		Config:  &config.Config{},
	}
}

// TestNewRouterWithDeps_WiresAllRoutes is the load-bearing assertion
// for Task 16: every documented /api/v1 route must be registered by
// NewRouterWithDeps and respond without a 5xx. The MockQuerier
// returns the zero value with no error, so every storage-backed
// route is a 200; the stub routes are 200 by definition; the
// health route is 200 with an empty registry.
func TestNewRouterWithDeps_WiresAllRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	broker, cancel := startRouterTestBroker(t)
	defer cancel()

	deps := RouterDeps{
		Health:  health.New(),
		Querier: &storage.MockQuerier{},
		Broker:  broker,
		Config:  &config.Config{},
	}
	r := NewRouterWithDeps(deps)

	syncRoutes := []struct {
		name   string
		method string
		path   string
	}{
		{"health", http.MethodGet, "/api/v1/health"},
		{"dashboard-summary", http.MethodGet, "/api/v1/dashboard/summary"},
		{"dashboard-top-talkers", http.MethodGet, "/api/v1/dashboard/top-talkers"},
		{"dashboard-protocols", http.MethodGet, "/api/v1/dashboard/protocols"},
		{"packets", http.MethodGet, "/api/v1/packets"},
		{"devices", http.MethodGet, "/api/v1/devices"},
		{"alerts", http.MethodGet, "/api/v1/alerts"},
		{"alert-ack", http.MethodPost, "/api/v1/alerts/foo/ack"},
		{"topology", http.MethodGet, "/api/v1/topology"},
		{"vulnerabilities", http.MethodGet, "/api/v1/vulnerabilities"},
		{"scan-history", http.MethodGet, "/api/v1/scan-history"},
		{"config", http.MethodGet, "/api/v1/config"},
	}
	for _, tt := range syncRoutes {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Less(t, w.Code, 500, "%s %s: got %d, want <500", tt.method, tt.path, w.Code)
		})
	}
}

// TestNewRouterWithDeps_RegistersSSEStreams asserts the four SSE
// routes are registered and respond with the canonical SSE headers.
// The test uses httptest.NewServer (not NewRecorder) because the SSE
// handler blocks in c.Stream; a recorder would deadlock. The
// per-route context is cancelled to abort the stream before any
// event arrives.
func TestNewRouterWithDeps_RegistersSSEStreams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	broker, cancel := startRouterTestBroker(t)
	defer cancel()

	deps := RouterDeps{
		Health:  health.New(),
		Querier: &storage.MockQuerier{},
		Broker:  broker,
		Config:  &config.Config{},
	}
	r := NewRouterWithDeps(deps)
	server := httptest.NewServer(r)
	defer server.Close()

	streamPaths := []string{
		"/api/v1/stream/metrics",
		"/api/v1/stream/alerts",
		"/api/v1/stream/topology",
		"/api/v1/stream/llm",
	}
	for _, path := range streamPaths {
		t.Run(path, func(t *testing.T) {
			ctx, cancelReq := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancelReq()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+path, nil)
			require.NoError(t, err)
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
		})
	}
}

// TestNewRouterWithDeps_StorageRoutesReturn503OnError asserts that
// every storage-backed route maps a Querier error to 503. The
// task spec explicitly allows 503 as an "appropriate" response.
func TestNewRouterWithDeps_StorageRoutesReturn503OnError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	broker, cancel := startRouterTestBroker(t)
	defer cancel()

	deps := RouterDeps{
		Health:  health.New(),
		Querier: &storage.ErrorQuerier{Err: errors.New("db down")},
		Broker:  broker,
		Config:  &config.Config{},
	}
	r := NewRouterWithDeps(deps)

	storageRoutes := []string{
		"/api/v1/dashboard/summary",
		"/api/v1/dashboard/top-talkers",
		"/api/v1/dashboard/protocols",
		"/api/v1/packets",
		"/api/v1/devices",
	}
	for _, path := range storageRoutes {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusServiceUnavailable, w.Code, "%s: got %d, want 503", path, w.Code)
		})
	}
}

// TestNewRouterWithDeps_SetsRequestIDHeader asserts the RequestID
// middleware is wired in: every response must carry an X-Request-Id
// header (either echoed from the request or freshly generated).
func TestNewRouterWithDeps_SetsRequestIDHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	broker, cancel := startRouterTestBroker(t)
	defer cancel()

	deps := RouterDeps{
		Health:  health.New(),
		Querier: &storage.MockQuerier{},
		Broker:  broker,
		Config:  &config.Config{},
	}
	r := NewRouterWithDeps(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.NotEmpty(t, w.Header().Get("X-Request-Id"), "X-Request-Id must be set by middleware")
}

// TestNewRouterWithDeps_EchoesInboundRequestID asserts the
// RequestID middleware reuses the inbound X-Request-Id when the
// caller supplies one. This is what lets operators correlate logs
// across services: the request id is the same from entry to exit.
func TestNewRouterWithDeps_EchoesInboundRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	broker, cancel := startRouterTestBroker(t)
	defer cancel()

	deps := RouterDeps{
		Health:  health.New(),
		Querier: &storage.MockQuerier{},
		Broker:  broker,
		Config:  &config.Config{},
	}
	r := NewRouterWithDeps(deps)

	const inbound = "abc-123"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set(middleware.RequestIDHeader, inbound)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, inbound, w.Header().Get(middleware.RequestIDHeader))
}

// TestNewRouterWithDeps_AppliesCORSWhenConfigured asserts the CORS
// middleware is wired in when AllowedOrigins is non-empty, and that
// an allowed origin gets the canonical Access-Control-Allow-Origin
// response header.
func TestNewRouterWithDeps_AppliesCORSWhenConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	broker, cancel := startRouterTestBroker(t)
	defer cancel()

	deps := RouterDeps{
		Health:  health.New(),
		Querier: &storage.MockQuerier{},
		Broker:  broker,
		Config:  &config.Config{},
		CORSConfig: middleware.CORSConfig{
			AllowedOrigins: []string{"http://localhost:5173"},
		},
	}
	r := NewRouterWithDeps(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, "http://localhost:5173", w.Header().Get("Access-Control-Allow-Origin"))
}

// TestNewRouterWithDeps_SkipsCORSWhenNotConfigured asserts the CORS
// middleware is NOT wired in when AllowedOrigins is empty. A
// cross-origin request must NOT receive an Access-Control-Allow-Origin
// header (otherwise the browser would silently allow it).
func TestNewRouterWithDeps_SkipsCORSWhenNotConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	broker, cancel := startRouterTestBroker(t)
	defer cancel()

	deps := RouterDeps{
		Health:  health.New(),
		Querier: &storage.MockQuerier{},
		Broker:  broker,
		Config:  &config.Config{},
	}
	r := NewRouterWithDeps(deps)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"), "CORS header must not be set when AllowedOrigins is empty")
}

// TestNewRouter_StillWorks is the explicit back-compat assertion for
// NewRouter: the constructor must continue to return a router that
// serves /api/v1/health. TestNewRouterWithRegistryExposesSubsystems
// (above) covers the NewRouterWithRegistry path.
func TestNewRouter_StillWorks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
