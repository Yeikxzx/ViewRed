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
// The router creates a fresh, empty health.Registry internally for callers
// that do not need to pre-populate subsystem checks (e.g. unit tests of
// the static asset serving path). Production wiring should use
// NewRouterWithRegistry to inject a registry populated with the real
// subsystem checks (storage ping, capture stats, etc.).
func NewRouter() *gin.Engine {
	return NewRouterWithRegistry(health.New())
}

// NewRouterWithRegistry builds a router that uses the given health
// registry for the /api/v1/health endpoint. Callers (typically main.go)
// register subsystem checks on rg before passing it in. This is the
// export seam for the health wiring — see the regression test
// TestNewRouterWithRegistryExposesSubsystems for the failure mode it
// prevents.
func NewRouterWithRegistry(rg *health.Registry) *gin.Engine {
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
