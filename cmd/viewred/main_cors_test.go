package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/viewred/viewred/internal/config"
	"github.com/viewred/viewred/internal/health"
	"github.com/viewred/viewred/internal/rules"
	"github.com/viewred/viewred/internal/storage"
)

// TestNewWireUp_WiresCORSFromConfig is the regression test for
// Plan 3 review finding C4 (CORS never wired in production).
//
// Before the fix, main_wire.go built the router with an empty
// CORSConfig, so the router's "if len(deps.CORSConfig.AllowedOrigins)
// > 0" guard skipped the CORS middleware entirely. The Vite dev
// server at http://localhost:5173 could not reach the API on
// http://127.0.0.1:8080 and the browser rejected every fetch with
// a CORS error.
//
// The fix wires CORSConfig from cfg.Server.CORSOrigins (which
// defaults to ["http://localhost:5173"] in dev). This test
// asserts the wireUp produces a router that:
//  1. Returns Access-Control-Allow-Origin for a request from the
//     configured origin (Vite dev).
//  2. Rejects a request from a non-allowed origin with 403.
func TestNewWireUp_WiresCORSFromConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{}
	// config.Config has a default CORSOrigins of
	// ["http://localhost:5173"] (see internal/config/config.go).
	// Loading via the helper ensures the same default path the
	// production binary takes.
	loaded, err := config.Load("")
	require.NoError(t, err)
	cfg = &loaded
	require.NotEmpty(t, cfg.Server.CORSOrigins, "test precondition: config must have a default CORSOrigins")

	w := newWireUp(cfg, health.New(), &storage.MockQuerier{}, nil, nil, nil, rules.NewEngine())

	server := httptest.NewServer(w.Router)
	defer server.Close()

	t.Run("allowed origin receives Allow-Origin header", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/health", nil)
		require.NoError(t, err)
		req.Header.Set("Origin", "http://localhost:5173")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "http://localhost:5173", resp.Header.Get("Access-Control-Allow-Origin"),
			"wireUp must wire CORSConfig from config so the Vite dev origin is allowed")
		assert.Equal(t, "Origin", resp.Header.Get("Vary"))
	})

	t.Run("disallowed origin is rejected with 403", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/health", nil)
		require.NoError(t, err)
		req.Header.Set("Origin", "http://evil.example.com")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusForbidden, resp.StatusCode,
			"CORS middleware must reject non-allowed origins")
	})
}

// TestNewWireUp_PassesCORSConfigOverrides verifies that a custom
// CORSOrigins in the config replaces the dev default. This is the
// production path: a deployment sets server.cors_origins in
// config.yaml and the wireUp must propagate it.
func TestNewWireUp_PassesCORSConfigOverrides(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Server: config.ServerConfig{
			BindAddress: "127.0.0.1",
			Port:        8080,
			CORSOrigins: []string{"https://viewred.example.com"},
		},
	}

	w := newWireUp(cfg, health.New(), &storage.MockQuerier{}, nil, nil, nil, rules.NewEngine())

	server := httptest.NewServer(w.Router)
	defer server.Close()

	t.Run("custom origin is allowed", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/health", nil)
		require.NoError(t, err)
		req.Header.Set("Origin", "https://viewred.example.com")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "https://viewred.example.com", resp.Header.Get("Access-Control-Allow-Origin"))
	})

	t.Run("dev default is NOT allowed when overridden", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/health", nil)
		require.NoError(t, err)
		req.Header.Set("Origin", "http://localhost:5173")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusForbidden, resp.StatusCode,
			"override CORSOrigins must replace the dev default; localhost:5173 should not be allowed")
	})
}
