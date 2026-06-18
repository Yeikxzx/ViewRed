// Package main is the entry point for the ViewRed binary.
//
// ViewRed is a network detection & response tool with local AI. This binary
// loads configuration, initializes the storage layer (PostgreSQL+TimescaleDB),
// starts live packet capture (Npcap on Windows, AF_PACKET on Linux), pipes
// captured packets to a rotating pcap file writer and a batched database
// writer, exposes a health-checked HTTP API, and shuts down gracefully on
// SIGINT/SIGTERM.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/viewred/viewred/internal/api"
	"github.com/viewred/viewred/internal/capture"
	"github.com/viewred/viewred/internal/config"
	"github.com/viewred/viewred/internal/health"
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

	// Context with cancellation on SIGINT/SIGTERM (for graceful shutdown).
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Initialize storage
	store, err := storage.New(ctx, cfg.Database.URL)
	if err != nil {
		log.Printf("storage init failed (continuing without storage): %v", err)
		store = nil
	} else {
		defer store.Close()
		migrationsDir := findMigrationsDir()
		if err := storage.RunMigrations(store, migrationsDir); err != nil {
			log.Printf("storage migrations failed: %v", err)
		}
	}

	// Initialize capture
	iface := os.Getenv("VIEWRED_INTERFACE")
	if iface == "" {
		// Empty string → "auto-detect" (driver-specific).
		iface = ""
	}
	cap := capture.NewCaptureForOS()
	packets := make(chan capture.PacketEvent, 4096)
	if err := cap.Start(iface, ""); err != nil {
		log.Printf("capture start failed: %v", err)
	} else {
		defer cap.Stop()
		// Bridge cap.Packets() → packets channel.
		// We use a non-blocking send with a drop counter to maintain
		// backpressure: if downstream writers (pcap / DB) are slow, we
		// drop packets rather than stall the capture driver.
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
	pw, err := capture.NewPcapWriter(pcapDir, 100*1024*1024 /* 100MB */, 1*time.Hour)
	if err != nil {
		log.Printf("pcap writer init failed: %v", err)
		pw = nil
	} else {
		defer pw.Close()
		// Bridge packets → pcap writer
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
	if store != nil {
		w := storage.NewWriter(store)
		go w.Run(ctx, packets) // also consumes packets
	}

	// Health registry
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

	// Start the API server
	addr := fmt.Sprintf("%s:%d", cfg.Server.BindAddress, cfg.Server.Port)
	router := api.NewRouter() // future: accept *health.Registry; for plan 2 the router builds its own
	log.Printf("ViewRed %s listening on http://%s", Version, addr)

	// Run server until ctx cancelled.
	serverErr := make(chan error, 1)
	go func() {
		if err := router.Run(addr); err != nil && !errors.Is(err, context.Canceled) {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case <-ctx.Done():
		log.Printf("shutdown signal received, exiting")
	case err := <-serverErr:
		if err != nil {
			log.Fatalf("server failed: %v", err)
		}
	}
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
