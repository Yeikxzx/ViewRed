package api

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/viewred/viewred/internal/api/handlers"
	"github.com/viewred/viewred/internal/api/middleware"
	"github.com/viewred/viewred/internal/config"
	"github.com/viewred/viewred/internal/geoip"
	"github.com/viewred/viewred/internal/health"
	"github.com/viewred/viewred/internal/resolver"
	"github.com/viewred/viewred/internal/rules"
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
	GeoIP      *geoip.Service
	// Resolver is the process-local DNS resolver. Optional — when
	// nil, the packets handler omits the src_hostname/dst_hostname
	// enrichment and tests that don't care about DNS stay
	// unbloated. The production wire-up always sets this.
	Resolver   *resolver.Resolver
	CORSConfig  middleware.CORSConfig
	RulesEngine *rules.Engine
	// WebFS is the embedded web/dist filesystem. When non-zero,
	// static assets are served from the embedded filesystem
	// (production mode). The zero value triggers a filesystem
	// fallback for development and tests.
	WebFS embed.FS
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
	mountWebAssetsFromEmbed(r, deps.WebFS)

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
		v1.POST("/rules", handlers.CreateRule(deps.Querier, deps.RulesEngine))
		v1.DELETE("/rules/:id", handlers.DeleteRule(deps.Querier, deps.RulesEngine))
		v1.GET("/packets", handlers.PacketsListWithResolver(deps.Querier, deps.Resolver))
		v1.GET("/devices", handlers.DevicesList(deps.Querier))
		v1.GET("/alerts", handlers.AlertsList(deps.Querier))
		v1.POST("/alerts/:id/ack", handlers.AlertAcknowledge(deps.Querier))
		v1.DELETE("/alerts/:id", handlers.AlertDelete(deps.Querier))
		v1.DELETE("/alerts", handlers.AlertClearAll(deps.Querier))
		v1.GET("/topology", handlers.TopologyGet(deps.Querier))
		v1.GET("/vulnerabilities", handlers.GetVulnerabilities(deps.Querier))
		v1.POST("/vulnerabilities/:id/mitigate", handlers.MitigateVulnerability(deps.Querier))
		v1.GET("/scan-history", handlers.GetScanHistory(deps.Querier))
		v1.POST("/scan", handlers.TriggerScan(handlers.ScanDeps{
			Querier: deps.Querier,
			Config:  deps.Config,
		}))
		v1.GET("/config", handlers.ConfigGet)
		v1.GET("/interfaces", handlers.GetInterfaces)
		v1.GET("/system/metrics", handlers.GetSystemMetrics(time.Now()))
		v1.GET("/dns/cache", handlers.DnsCacheGet(deps.Resolver))
		
		captureConfigDeps := handlers.CaptureConfigDeps{
			Config:  deps.Config,
			Capture: deps.Capture,
		}
		v1.GET("/capture/config", handlers.GetCaptureConfig(captureConfigDeps))
		v1.POST("/capture/config", handlers.UpdateCaptureConfig(captureConfigDeps))
		v1.GET("/capture/storage-policy", handlers.GetStoragePolicy(captureConfigDeps))
		v1.POST("/capture/storage-policy", handlers.UpdateStoragePolicy(captureConfigDeps))
		v1.POST("/capture/restart", handlers.RestartCapture(captureConfigDeps))
		v1.POST("/capture/stop", handlers.StopCapture(captureConfigDeps))
		
		pcapDir := deps.Config.Capture.PcapDir
		if pcapDir == "" {
			pcapDir = filepath.Join("data", "pcaps")
		}
		v1.GET("/pcaps", handlers.ListPcaps(pcapDir))
		v1.DELETE("/pcaps/:filename", handlers.DeletePcap(pcapDir))
		v1.DELETE("/pcaps", handlers.DeleteAllPcaps(pcapDir))
		
		reportDeps := handlers.ReportDeps{
			Querier: deps.Querier,
			OutputDir: filepath.Join("data", "reports"),
		}
		v1.POST("/reports/generate", handlers.GenerateReport(reportDeps))
		v1.GET("/reports", handlers.ListReports(reportDeps))
		v1.GET("/reports/download/:filename", handlers.DownloadReport(reportDeps))
		v1.DELETE("/reports/:filename", handlers.DeleteReport(reportDeps))
		v1.GET("/reports/config", handlers.GetReportConfig(reportDeps))
		v1.POST("/reports/config", handlers.SaveReportConfig(reportDeps))

		// GeoIP returns the network's approximate geographic location
		// so the frontend can render a real-world map with devices as
		// points. The service is optional — when nil the endpoints
		// return 503 instead of crashing.
		if deps.GeoIP != nil {
			geoipDeps := handlers.GeoIPDeps{GeoIP: deps.GeoIP}
			v1.GET("/geoip", handlers.GeoIPNetworkLocation(geoipDeps))
			v1.POST("/geoip/batch", handlers.GeoIPLookupBatch(geoipDeps))
			v1.GET("/geoip/:ip", handlers.GeoIPLookup(geoipDeps))
		}

		// Ollama test endpoint — used by the UI "Test Conexion" button
		// to verify the local Ollama is reachable and the configured
		// model is available.
		if deps.Config != nil {
			ollamaDeps := handlers.OllamaTestDeps{Config: deps.Config}
			v1.GET("/ollama/test", handlers.OllamaTest(ollamaDeps))
		}

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

// mountWebAssetsFromEmbed wires static asset serving on r. When webFS
// contains a "webdist" subdirectory (production binary with embedded
// assets), it serves from the embedded filesystem. Otherwise it falls
// back to the filesystem-based findWebDist() search (development mode
// and tests).
func mountWebAssetsFromEmbed(r *gin.Engine, webFS embed.FS) {
	// Try the embedded filesystem first.
	if distFS, err := fs.Sub(webFS, "webdist"); err == nil {
		// Serve /assets/* from embedded dist/assets.
		r.StaticFS("/assets", http.FS(distFS))
		r.GET("/", func(c *gin.Context) {
			data, _ := fs.ReadFile(distFS, "index.html")
			c.Data(http.StatusOK, "text/html; charset=utf-8", data)
		})
		r.GET("/index.html", func(c *gin.Context) {
			data, _ := fs.ReadFile(distFS, "index.html")
			c.Data(http.StatusOK, "text/html; charset=utf-8", data)
		})
		r.NoRoute(func(c *gin.Context) {
			// Skip serving index.html for /api paths — return
			// JSON 404 instead so API consumers get a machine-
			// readable response.
			if len(c.Request.URL.Path) >= 4 && c.Request.URL.Path[:4] == "/api" {
				c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
				return
			}
			// SPA fallback: serve index.html for any non-API
			// route so client-side routing works.
			data, readErr := fs.ReadFile(distFS, "index.html")
			if readErr == nil && data != nil {
				c.Data(http.StatusOK, "text/html; charset=utf-8", data)
			} else {
				c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			}
		})
		return
	}

	// Fallback: filesystem-based search (development mode).
	mountWebAssets(r)
}
