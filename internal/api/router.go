package api

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"github.com/viewred/viewred/internal/api/handlers"
	"github.com/viewred/viewred/internal/health"
)

// NewRouter returns a configured Gin engine with all API routes mounted.
// API routes are versioned under /api/v1.
//
// The router creates a fresh, empty health.Registry internally for plan 2.
// Task 14 (main.go wiring) is expected to register the real subsystem
// checks before serving traffic. Tests can pass a pre-populated registry
// via newRouterWithRegistry if they need to assert on subsystem state.
func NewRouter() *gin.Engine {
	return newRouterWithRegistry(health.New())
}

// newRouterWithRegistry builds a router that uses the given health registry
// for the /api/v1/health endpoint.
func newRouterWithRegistry(rg *health.Registry) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	v1 := r.Group("/api/v1")
	{
		v1.GET("/health", handlers.Health(rg))
	}

	if dist := findWebDist(); dist != "" {
		r.StaticFS("/assets", http.Dir(filepath.Join(dist, "assets")))
		r.StaticFile("/", filepath.Join(dist, "index.html"))
		r.StaticFile("/index.html", filepath.Join(dist, "index.html"))
	} else {
		r.NoRoute(func(c *gin.Context) {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "not_found",
				"message": "Web UI not built. Run `make web-build` or start Vite dev server.",
			})
		})
	}

	return r
}

func findWebDist() string {
	candidates := []string{
		"web/dist",
		"../web/dist",
		"./web/dist",
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	return ""
}
