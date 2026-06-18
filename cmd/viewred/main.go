// Package main is the entry point for the ViewRed binary.
//
// ViewRed is a network detection & response tool with local AI. This binary
// loads configuration, initializes the storage layer (PostgreSQL+TimescaleDB),
// starts live packet capture (Npcap on Windows, AF_PACKET on Linux), pipes
// captured packets to a rotating pcap file writer and a batched database
// writer, exposes a health-checked HTTP API, and shuts down gracefully on
// SIGINT/SIGTERM.
//
// Shutdown is coordinated by internal/shutdown.Shutdown, which guarantees
// the required teardown order (spec §9.3): stop capture, close the packets
// channel (drains writers, fixes the goroutine leak in the pcap bridge),
// flush the final DB batch with a fresh context (fixes the dropped batch
// bug), close the pcap writer, drain the HTTP server, and close storage.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/viewred/viewred/internal/capture"
	"github.com/viewred/viewred/internal/config"
	"github.com/viewred/viewred/internal/health"
	"github.com/viewred/viewred/internal/shutdown"
	"github.com/viewred/viewred/internal/storage"
)

// Version is the semantic version of this build, set via -ldflags at build time.
var Version = "0.1.0-dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("viewred %s\n", Version)
		return
	}

	// Load config
	cfgPath := os.Getenv("VIEWRED_CONFIG")
	if cfgPath == "" {
		if dir, err := os.UserConfigDir(); err == nil {
			cfgPath = filepath.Join(dir, "viewred", "config.yaml")
		}
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	// Context with cancellation on SIGINT/SIGTERM (for the bridge
	// goroutine to abort on signal). The storage writer uses a
	// SEPARATE context so its final batch is not lost when this one
	// is cancelled — the shutdown function passes a fresh context to
	// Writer.Flush.
	//
	// We do NOT defer cancel() here; the wireUp cleanup defer
	// further down owns the cancellation sequence (broker.Shutdown →
	// cancel → stopWire) so the goroutines always see a consistent
	// order on both the signal and the panic path.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	// Initialize storage
	var store *storage.Storage
	store, err = storage.New(ctx, cfg.Database.URL)
	if err != nil {
		log.Printf("storage init failed (continuing without storage): %v", err)
		store = nil
	} else {
		// Defer is the panic-safety net; the explicit shutdown.Shutdown
		// call in the select below is the primary path.
		defer store.Close()
		migrationsDir := findMigrationsDir()
		if err := storage.RunMigrations(store, migrationsDir); err != nil {
			log.Printf("storage migrations failed: %v", err)
		}
	}

	// Initialize capture
	iface := os.Getenv("VIEWRED_INTERFACE")
	cap := capture.NewCaptureForOS()
	packets := make(chan capture.PacketEvent, 4096)
	if err := cap.Start(iface, ""); err != nil {
		log.Printf("capture start failed: %v", err)
	} else {
		// Defer is the panic-safety net.
		defer cap.Stop()
		// Bridge cap.Packets() → packets channel. We use a non-blocking
		// send with a drop counter to maintain backpressure: if
		// downstream writers (pcap / DB) are slow, we drop packets
		// rather than stall the capture driver.
		//
		// The bridge exits either when cap.Packets() is closed
		// (unlikely — drivers leave it open by design) or when ctx is
		// done. The shutdown function does NOT wait on this bridge;
		// it only closes `packets`, which the pcap-bridge and the
		// storage writer both consume from. A residual leak in this
		// specific bridge on driver implementations that never close
		// their Packets() channel is acceptable — process exit will
		// reclaim it.
		go func() {
			for e := range cap.Packets() {
				select {
				case packets <- e:
				case <-ctx.Done():
					return
				default:
					// channel full → drop
				}
			}
		}()
	}

	// Start the pcap file writer
	pcapDir := os.Getenv("VIEWRED_PCAP_DIR")
	if pcapDir == "" {
		pcapDir = filepath.Join(os.TempDir(), "viewred-pcap")
	}
	var pw *capture.PcapWriter
	pw, err = capture.NewPcapWriter(pcapDir, 100*1024*1024 /* 100MB */, 1*time.Hour)
	if err != nil {
		log.Printf("pcap writer init failed: %v", err)
		pw = nil
	} else {
		// Defer is the panic-safety net.
		defer pw.Close()
		// Bridge packets → pcap writer. Exits naturally when the
		// `packets` channel is closed by shutdown.Shutdown.
		go func() {
			for e := range packets {
				ci := capture.ToCaptureInfo(e)
				if err := pw.WritePacket(capture.PayloadBytes(e), ci); err != nil {
					log.Printf("pcap write: %v", err)
				}
			}
		}()
	}

	// Start the storage writer (if storage is available).
	// The writer is the second consumer of `packets`; both consumers
	// race for events on the buffered channel. With a buffer of 4096
	// this rarely contends in practice.
	//
	// The writer is also given its OWN context (not the signal ctx)
	// so that the shutdown function can close `packets` to make the
	// Run loop exit cleanly, then call Flush with a fresh context
	// for the final batch.
	var writer *storage.Writer
	if store != nil {
		writer = storage.NewWriter(store)
		writerCtx, writerCancel := context.WithCancel(context.Background())
		defer writerCancel()
		go writer.Run(writerCtx, packets)
	}

	// Health registry — populated with the real subsystem checks and
	// injected into the router so /api/v1/health reports them. This
	// is the fix for CRITICAL #1 (registry not wired to the API).
	rg := health.New()
	if store != nil {
		rg.Add("storage", func(ctx context.Context) error {
			pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			return store.Ping(pingCtx)
		})
	}
	rg.Add("capture", func(ctx context.Context) error {
		stats := cap.Stats()
		if stats.LastError != nil {
			return stats.LastError
		}
		return nil
	})

	// Build the wire-up (SSE broker + metrics ticker + router).
	// When storage is up, the real *Storage is the Querier; when it
	// is down, an ErrorQuerier is injected so every handler maps to
	// 503 uniformly without any nil checks.
	var querier storage.Querier
	if store != nil {
		querier = store
	} else {
		querier = &storage.ErrorQuerier{Err: errors.New("storage unavailable")}
	}
	wire := newWireUp(&cfg, rg, querier)

	// Start the SSE broker and the 1Hz metrics ticker in
	// goroutines. The returned stop function blocks until both
	// goroutines have exited, so it is safe to call in the
	// shutdown defer below.
	stopWire := wire.Start(ctx)

	// Cleanup defer for the wire-up. Order matters:
	//   1. cancel() — signal the metrics ticker and broker Run
	//      goroutines to exit on their next select iteration.
	//   2. stopWire() — block until both goroutines have actually
	//      exited. The single-defer pattern makes the order
	//      explicit instead of relying on LIFO trivia.
	//
	// The broker's own Shutdown is NOT in this defer: the explicit
	// shutdown.Shutdown call below passes wire.Broker via
	// shutdown.Params, so the broker is shut down in the right
	// position (after the HTTP server, before storage). Doing it
	// here would put it AFTER storage.Close, violating the spec
	// order (server -> broker -> store).
	defer func() {
		cancel()
		stopWire()
	}()

	// Start the API server. We use a real *http.Server (not
	// gin.Engine.Run) so we can call Shutdown(ctx) on it for a
	// graceful drain in the shutdown sequence.
	addr := fmt.Sprintf("%s:%d", cfg.Server.BindAddress, cfg.Server.Port)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: wire.Router,
	}
	log.Printf("ViewRed %s listening on http://%s", Version, addr)

	// Run server until ctx cancelled.
	serverErr := make(chan error, 1)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case <-ctx.Done():
		log.Printf("shutdown signal received, exiting")
	case err := <-serverErr:
		if err != nil {
			log.Printf("server failed: %v", err)
		}
	}

	// Explicit, ordered teardown. This is the primary shutdown path;
	// the defers above are a safety net for panics.
	//
	// Broker is wired from wire.Broker so the broker shuts down
	// inside shutdown.Shutdown's ordered teardown (after the HTTP
	// server, before storage). Without this field the explicit
	// shutdown.Shutdown would skip the broker entirely and a
	// separate defer would have to call wire.Broker.Shutdown,
	// running it AFTER storage.Close and violating the spec order
	// (server -> broker -> store).
	if err := shutdown.Shutdown(shutdown.Params{
		Capturer:   captureAdapter{cap},
		Packets:    packets,
		PcapWriter: pcapWriterAdapter{pw},
		Storage:    storeAdapter{store},
		Writer:     writerAdapter{writer},
		Server:     httpServer,
		Broker:     wire.Broker,
	}); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

// --- shutdown adapter types -------------------------------------------------
//
// The internal/shutdown package works against small interfaces so it
// can be tested without touching the real subsystems. main.go wraps
// the concrete pointers in tiny adapter types to satisfy those
// interfaces. This is the export seam between main and the shutdown
// orchestrator.

// captureAdapter wraps a capture.PacketCapture so it satisfies
// shutdown.Capturer without leaking the full interface.
type captureAdapter struct {
	c capture.PacketCapture
}

func (a captureAdapter) Stop() error {
	if a.c == nil {
		return nil
	}
	return a.c.Stop()
}

// pcapWriterAdapter wraps *capture.PcapWriter.
type pcapWriterAdapter struct {
	pw *capture.PcapWriter
}

func (a pcapWriterAdapter) Close() error {
	if a.pw == nil {
		return nil
	}
	return a.pw.Close()
}

// storeAdapter wraps *storage.Storage.
type storeAdapter struct {
	s *storage.Storage
}

func (a storeAdapter) Close() {
	if a.s == nil {
		return
	}
	a.s.Close()
}

// writerAdapter wraps *storage.Writer.
type writerAdapter struct {
	w *storage.Writer
}

func (a writerAdapter) Run(ctx context.Context, events <-chan capture.PacketEvent) {
	if a.w == nil {
		return
	}
	a.w.Run(ctx, events)
}

func (a writerAdapter) Done() <-chan struct{} {
	if a.w == nil {
		// Return an already-closed channel so shutdown.Shutdown
		// proceeds immediately for the no-writer case.
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return a.w.Done()
}

// findMigrationsDir returns the absolute path to internal/storage/migrations/.
// It assumes the binary is run from the repo root or from cmd/viewred/.
// Tries a few common relative paths and returns the first that resolves
// to an existing directory; falls back to the most likely candidate
// (internal/storage/migrations) if none match.
func findMigrationsDir() string {
	candidates := []string{
		"internal/storage/migrations",
		"../internal/storage/migrations",
		"../../internal/storage/migrations",
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	return "internal/storage/migrations" // best-effort fallback
}
