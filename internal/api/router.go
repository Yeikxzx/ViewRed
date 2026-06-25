package api

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"github.com/viewred/viewred/internal/api/handlers"
	"github.com/viewred/viewred/internal/api/middleware"
	"github.com/viewred/viewred/internal/config"
	"github.com/viewred/viewred/internal/health"
	"github.com/viewred/viewred/internal/sse"
	"github.com/viewred/viewred/internal/storage"
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
	r := newBaseRouter()
	v1 := r.Group("/api/v1")
	{
		v1.GET("/health", handlers.Health(rg))
	}
	mountWebAssets(r)
	return r
}

// RouterDeps groups every dependency the full router needs. It is the
// additive export seam introduced in Plan 3 Task 16: main.go (Task 17)
// constructs a RouterDeps from its already-initialised subsystems
// (storage, SSE broker, config, health registry) and passes it to
// NewRouterWithDeps.
//
// Health, Querier, Broker, and Config are required for a router that
// serves the production route set. CORSConfig is optional — when its
// AllowedOrigins slice is empty the CORS middleware is omitted so a
// same-origin deployment does not need to configure it.
type RouterDeps struct {
	Health     *health.Registry
	Querier    storage.Querier
	Broker     *sse.Broker
	Config     *config.Config
	Capture    *handlers.CaptureHandle
	CORSConfig middleware.CORSConfig
}

// NewRouterWithDeps builds a router that wires every middleware and
// route the ViewRed API surface requires:
//
//	Recovery → RequestID → [CORS] → RequestLogging → routes
//
// The CORS middleware is only inserted when deps.CORSConfig.AllowedOrigins
// is non-empty, so single-host deployments (e.g. the dev server bound
// to 127.0.0.1) get the same behaviour as before without having to
// declare a CORS policy.
//
// Every documented /api/v1 route is mounted: the four dashboard
// readouts, the packet and device lists, the stub endpoints (alerts,
// topology, vulnerabilities, config), the alert ack POST, and the
// four SSE streams. The web-asset fallthrough is mounted last so
// /api/* takes precedence over static files.
func NewRouterWithDeps(deps RouterDeps) *gin.Engine {
	r := newBaseRouter()

	// Mount web assets before global middlewares (like CORS) so static files
	// don't get blocked by CORS origin checks when loaded via <script crossorigin>.
	mountWebAssets(r)

	r.Use(middleware.RequestID())
	if len(deps.CORSConfig.AllowedOrigins) > 0 {
		r.Use(middleware.NewCORS(deps.CORSConfig))
	}
	r.Use(middleware.RequestLogging())

	v1 := r.Group("/api/v1")
	{
		v1.GET("/health", handlers.Health(deps.Health))
		v1.GET("/dashboard/summary", handlers.DashboardSummary(deps.Querier))
		v1.GET("/dashboard/top-talkers", handlers.TopTalkers(deps.Querier))
		v1.GET("/dashboard/protocols", handlers.ProtocolDistribution(deps.Querier))
		v1.GET("/rules", handlers.GetRules(deps.Querier))
		v1.POST("/rules", handlers.CreateRule(deps.Querier))
		v1.DELETE("/rules/:id", handlers.DeleteRule(deps.Querier))
		v1.GET("/packets", handlers.PacketsList(deps.Querier))
		v1.GET("/devices", handlers.DevicesList(deps.Querier))
		v1.GET("/alerts", handlers.AlertsList(deps.Querier))
		v1.POST("/alerts/:id/ack", handlers.AlertAcknowledge(deps.Querier))
		v1.GET("/topology", handlers.TopologyGet(deps.Querier))
		v1.GET("/vulnerabilities", handlers.GetVulnerabilities(deps.Querier))
		v1.POST("/vulnerabilities/:id/mitigate", handlers.MitigateVulnerability(deps.Querier))
		v1.POST("/scan", handlers.TriggerScan(deps.Querier))
		v1.GET("/config", handlers.ConfigGet)
		v1.GET("/interfaces", handlers.GetInterfaces)
		
		captureConfigDeps := handlers.CaptureConfigDeps{
			Config:  deps.Config,
			Capture: deps.Capture,
		}
		v1.GET("/capture/config", handlers.GetCaptureConfig(captureConfigDeps))
		v1.POST("/capture/config", handlers.UpdateCaptureConfig(captureConfigDeps))
		v1.POST("/capture/restart", handlers.RestartCapture(captureConfigDeps))

		v1.POST("/chat", handlers.LLMChatHandler(deps.Querier))
		v1.GET("/chat/history/:conversation_id", handlers.GetChatHistoryHandler(deps.Querier))
		v1.GET("/stream/metrics", handlers.StreamMetricsHandler(handlers.StreamMetricsHandlerDeps{
			Broker: deps.Broker,
		}))
		v1.GET("/stream/alerts", handlers.StreamStubHandler(handlers.StreamStubHandlerDeps{
			Broker: deps.Broker,
			Topic:  "alerts",
		}))
		v1.GET("/stream/topology", handlers.StreamStubHandler(handlers.StreamStubHandlerDeps{
			Broker: deps.Broker,
			Topic:  "topology",
		}))
		v1.GET("/stream/llm", handlers.StreamStubHandler(handlers.StreamStubHandlerDeps{
			Broker: deps.Broker,
			Topic:  "llm",
		}))
		v1.GET("/stream/capture-status", handlers.StreamStubHandler(handlers.StreamStubHandlerDeps{
			Broker: deps.Broker,
			Topic:  "capture-status",
		}))
	}

	return r
}

// newBaseRouter returns a gin.Engine with the always-on middleware
// (Recovery) installed. The other middleware are added by the
// caller (NewRouterWithRegistry adds nothing further, NewRouterWithDeps
// adds RequestID/CORS/RequestLogging) so the back-compat router
// stays minimal.
func newBaseRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	return r
}

// mountWebAssets wires the static-asset serving on r. If the
// built web/dist directory is present, its files are served under
// the canonical paths; otherwise a NoRoute handler returns a
// helpful 404 so an unbuilt UI does not crash with a generic
// "page not found" body.
func mountWebAssets(r *gin.Engine) {
	if dist := findWebDist(); dist != "" {
		r.StaticFS("/assets", http.Dir(filepath.Join(dist, "assets")))
		r.StaticFile("/", filepath.Join(dist, "index.html"))
		r.StaticFile("/index.html", filepath.Join(dist, "index.html"))
		r.NoRoute(func(c *gin.Context) {
			// Skip serving index.html for missing /api routes
			if len(c.Request.URL.Path) >= 4 && c.Request.URL.Path[:4] == "/api" {
				c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
				return
			}
			c.File(filepath.Join(dist, "index.html"))
		})
		return
	}
	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "not_found",
			"message": "Web UI not built. Run `make web-build` or start Vite dev server.",
		})
	})
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
