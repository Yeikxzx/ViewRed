package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/viewred/viewred/internal/api"
	"github.com/viewred/viewred/internal/api/handlers"
	"github.com/viewred/viewred/internal/api/middleware"
	"github.com/viewred/viewred/internal/config"
	"github.com/viewred/viewred/internal/geoip"
	"github.com/viewred/viewred/internal/health"
	"github.com/viewred/viewred/internal/resolver"
	"github.com/viewred/viewred/internal/sse"
	"github.com/viewred/viewred/internal/storage"
)

// wireUp owns the SSE broker, the storage Querier, the configured
// router, and the metrics-ticker configuration. It is the unit
// extracted from main.go so Task 17's wire-up logic can be unit-tested
// without bringing up a real HTTP server or a real storage pool.
//
// The wireUp is intentionally minimal: it does NOT own capture, pcap,
// or storage lifecycle. Those remain in main.go so the explicit
// shutdown.Shutdown teardown order is unchanged. wireUp only owns the
// things added by Plan 3: the broker, the metrics ticker, and the
// router that depends on them.
type wireUp struct {
	// Broker is the SSE hub. Created here so the same instance is
	// shared by the metrics ticker (producer) and the router's
	// stream handlers (consumers).
	Broker *sse.Broker
	// Querier is the read surface the router's storage-backed
	// handlers and the metrics ticker both depend on. main.go
	// chooses this (real *Storage when the DB is up, or
	// *storage.ErrorQuerier otherwise) so the wireUp stays
	// DB-agnostic and unit-testable.
	Querier storage.Querier
	// Resolver is the process-local DNS resolver. The capture
	// pipeline feeds A/AAAA answers into it; the packets handler
	// reads from it to enrich API responses with hostnames. Kept
	// on the wireUp (rather than as a package-level singleton) so
	// tests can inject a closed-over fixture.
	Resolver *resolver.Resolver
	// Router is the gin.Engine produced from RouterDeps. The
	// returned handler is what main.go hands to http.Server.
	Router *gin.Engine

	// metricsInterval is the cadence at which the metrics ticker
	// queries the dashboard and broadcasts. Configurable so the
	// unit test can use a 10ms cadence and finish in well under
	// a second.
	metricsInterval time.Duration
	// metricsTopic is the broker routing key the metrics ticker
	// broadcasts on. The /api/v1/stream/metrics handler subscribes
	// to the same key (the constant is duplicated in
	// internal/api/handlers/sse.go — moving it to a shared location
	// is a follow-up, not a Task 17 concern).
	metricsTopic string
	// metricsEvent is the SSE event name the metrics ticker emits
	// in the `event:` field. The React dashboard EventSource
	// filters on this name.
	metricsEvent string
	// metricsLookback is the time window passed to
	// QueryDashboardSummary. One hour matches the dashboard's
	// default and gives a meaningful rate even when no recent
	// activity exists.
	metricsLookback time.Duration
}

const (
	// defaultMetricsInterval is the production cadence for the
	// metrics ticker. 1 Hz matches what the React dashboard
	// renders and keeps the SQL aggregates cheap.
	defaultMetricsInterval = 1 * time.Second

	// metricsStreamTopic is the broker routing key for metrics
	// events. Must match the topic the /api/v1/stream/metrics
	// handler subscribes to (see internal/api/handlers/sse.go).
	metricsStreamTopic = "metrics"

	// metricsStreamEventName is the SSE event name the ticker
	// emits in the `event:` field. Uses "message" so that the default
	// EventSource 'message' listener in the React UI catches it.
	metricsStreamEventName = "message"

	// defaultMetricsLookback is the time window the ticker
	// queries. 1 hour matches the dashboard summary's default
	// and gives a non-zero rate even on a quiet network.
	defaultMetricsLookback = 1 * time.Hour
)

// newWireUp constructs the broker, builds the router from
// RouterDeps, and returns the wireUp. It does NOT start any
// goroutines — the caller invokes Start(ctx) for that.
//
// Querier is the storage read surface for the API and the metrics
// ticker. main.go is responsible for picking the concrete Querier
// (real *Storage or *storage.ErrorQuerier) before calling newWireUp,
// keeping the wireUp itself dependency-light and unit-testable.
func newWireUp(cfg *config.Config, rg *health.Registry, querier storage.Querier, dnsResolver *resolver.Resolver, capture *handlers.CaptureHandle, geoipSvc *geoip.Service) *wireUp {
	return newWireUpWithInterval(cfg, rg, querier, dnsResolver, defaultMetricsInterval, capture, geoipSvc)
}

// newWireUpWithInterval is the test-friendly form of newWireUp. The
// production path goes through newWireUp, which uses the 1 Hz
// default; tests inject a sub-second cadence so the metrics ticker
// fires at least once before the test budget elapses.
func newWireUpWithInterval(cfg *config.Config, rg *health.Registry, querier storage.Querier, dnsResolver *resolver.Resolver, interval time.Duration, capture *handlers.CaptureHandle, geoipSvc *geoip.Service) *wireUp {
	broker := sse.NewBroker()
	router := api.NewRouterWithDeps(api.RouterDeps{
		Health:   rg,
		Querier:  querier,
		Broker:   broker,
		Config:   cfg,
		Capture:  capture,
		GeoIP:    geoipSvc,
		Resolver: dnsResolver,
		// Wire CORS from config so the React UI (running on
		// Vite at http://localhost:5173 by default) can
		// reach the API. Without this field the router
		// would skip the CORS middleware entirely and
		// every cross-origin request would be rejected by
		// the browser. Production deployments override
		// cfg.Server.CORSOrigins via config.yaml.
		CORSConfig: middleware.CORSConfig{
			AllowedOrigins: cfg.Server.CORSOrigins,
			AllowedMethods: []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{"Content-Type", "X-Request-Id"},
		},
	})
	return &wireUp{
		Broker:          broker,
		Querier:         querier,
		Resolver:        dnsResolver,
		Router:          router,
		metricsInterval: interval,
		metricsTopic:    metricsStreamTopic,
		metricsEvent:    metricsStreamEventName,
		metricsLookback: defaultMetricsLookback,
	}
}

// Start runs the SSE broker and the metrics ticker in goroutines.
// The returned stop function cancels the ticker and blocks until
// both goroutines have exited, so a deferred stop() in main.go (or
// a test) can rely on a quiescent state once stop returns.
//
// The caller is responsible for cancelling ctx; stop() only waits
// for the goroutines to notice. The standard pattern is:
//
//	ctx, cancel := signal.NotifyContext(...)
//	defer stop()    // runs second (LIFO)
//	defer cancel()  // runs first  (LIFO) — wires ctx.Done()
//
// When the function returns, cancel() runs first, then stop() waits
// for the goroutines to exit.
func (w *wireUp) Start(ctx context.Context) (stop func()) {
	brokerDone := make(chan struct{})
	go func() {
		defer close(brokerDone)
		w.Broker.Run(ctx)
	}()

	ticker := time.NewTicker(w.metricsInterval)
	tickerDone := make(chan struct{})
	go func() {
		defer close(tickerDone)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.broadcastTick(ctx)
			}
		}
	}()

	return func() {
		ticker.Stop()
		<-brokerDone
		<-tickerDone
	}
}

// broadcastTick runs one cycle of the metrics loop: query the
// dashboard, marshal the result, and broadcast an Event on the
// broker. On any error (storage down, marshal failure) the tick is
// silently dropped — the next tick will try again, and a successful
// query will heal the stream.
//
// ctx is the parent context (typically the signal context). When it
// is cancelled the query aborts promptly.
func (w *wireUp) broadcastTick(ctx context.Context) {
	summary, err := w.Querier.QueryDashboardSummary(ctx, time.Now().Add(-w.metricsLookback))
	if err != nil {
		return
	}
	data, err := json.Marshal(summary)
	if err != nil {
		return
	}
	w.Broker.Broadcast(sse.Event{
		Name:  w.metricsEvent,
		Data:  data,
		Topic: w.metricsTopic,
	})
}
