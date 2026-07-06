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
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/viewred/viewred/internal/api/handlers"
	"github.com/viewred/viewred/internal/capture"
	"github.com/viewred/viewred/internal/config"
	"github.com/viewred/viewred/internal/discovery"
	"github.com/viewred/viewred/internal/geoip"
	"github.com/viewred/viewred/internal/health"
	"github.com/viewred/viewred/internal/ml"
	"github.com/viewred/viewred/internal/resolver"
	"github.com/viewred/viewred/internal/rules"
	"github.com/viewred/viewred/internal/scanner"
	"github.com/viewred/viewred/internal/shutdown"
	"github.com/viewred/viewred/internal/sse"
	"github.com/viewred/viewred/internal/storage"
	"github.com/viewred/viewred/internal/reports"
	"github.com/viewred/viewred/internal/updater"
	"github.com/viewred/viewred/internal/version"
)

// defaultRules returns the built-in SSH/Telnet detection rules used when no
// custom rules are configured or when storage is unavailable. The rules are
// SYN-only to suppress noise from ongoing sessions (ACKs, keepalives, data).
func defaultRules() []string {
	return rules.DefaultRuleStrings()
}

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println(version.Get().String())
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
		// Try filesystem migrations first (development mode).
		migrateErr := storage.RunMigrations(store, migrationsDir)
		// If that failed (e.g., running from packaged binary where the
		// filesystem path doesn't exist), try embedded migrations.
		if migrateErr != nil {
			if subFS, subErr := fs.Sub(migrationsFS, "migrations"); subErr == nil {
				migrateErr = storage.RunMigrationsFromFS(store, subFS)
			}
		}
		if migrateErr != nil {
			log.Printf("storage migrations failed: %v", migrateErr)
		}
	}

	// Initialize capture
	iface := cfg.Capture.Interface
	if iface == "" {
		iface = os.Getenv("VIEWRED_INTERFACE")
	}
	if iface == "" {
		iface = capture.AutoDetectInterface()
		if iface != "" {
			log.Printf("VIEWRED_INTERFACE not set, auto-selected interface: %s", iface)
		}
	}
	cap := capture.NewCaptureForOS()
	var captureRunning bool
	packets := make(chan capture.PacketEvent, 4096)
	evalPackets := make(chan capture.PacketEvent, 4096)
	bpfFilter := cfg.Capture.BPF

	// DNS resolver: process-local IP→hostname cache populated
	// from DNS responses on the wire. The capture pipeline
	// extracts A/AAAA records onto each PacketEvent; the
	// bridge goroutine below feeds them into the resolver.
	// HTTP handlers consult the resolver to enrich packet
	// responses with hostnames ("192.168.100.8 → google.com")
	// without touching the storage layer.
	dnsResolver := resolver.New()
	dnsResolver.Run()
	defer dnsResolver.Stop()

	if err := cap.Start(iface, bpfFilter); err != nil {
		log.Printf("capture start failed: %v", err)
	} else {
		captureRunning = true
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
				// Feed DNS A/AAAA answers into the resolver before
				// forwarding. This is non-blocking (in-memory map
				// update) and runs in the same goroutine as the
				// bridge so the cache stays in sync with the
				// packet stream. Skipping it here would mean the
				// HTTP handler can't resolve hostnames until
				// the next reader-writer cycle completes.
				for _, ans := range e.DNSAnswers {
					dnsResolver.Set(ans.IP, ans.Hostname, ans.TTL)
				}

				// Send to primary queue (storage/pcap)
				select {
				case packets <- e:
				case <-ctx.Done():
					return
				default:
					// channel full → drop
				}
				// Send to ML/Rules eval queue
				select {
				case evalPackets <- e:
				case <-ctx.Done():
					return
				default:
					// channel full → drop
				}
			}
		}()
	}

	// Start the pcap file writer
	pcapDir := cfg.Capture.PcapDir
	if pcapDir == "" {
		pcapDir = filepath.Join("data", "pcaps")
	}
	// Ensure directory exists
	if err := os.MkdirAll(pcapDir, 0755); err != nil {
		log.Printf("failed to create pcap dir: %v", err)
	}

	var pw *capture.PcapWriter
	pw, err = capture.NewPcapWriter(pcapDir, int64(cfg.Capture.MaxPcapSizeMB)*1024*1024, time.Duration(cfg.Capture.MaxPcapAgeHours)*time.Hour)
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

	var writer *storage.Writer
	var discEngine *discovery.Engine
	if store != nil {
		writer = storage.NewWriter(store)
		writerCtx, writerCancel := context.WithCancel(context.Background())
		defer writerCancel()
		go writer.Run(writerCtx, packets)

		discEngine = discovery.NewEngine(writer)
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
		if !stats.Running {
			return errors.New("capture is not running")
		}
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
	captureHandle := &handlers.CaptureHandle{
		Capturer:    cap,
		Packets:     packets,
		EvalPackets: evalPackets,
	}

	// GeoIP service for the topology map. Uses ip-api.com with a 1h
	// cache so repeat topology polling doesn't hit the external API.
	geoipSvc := geoip.NewService(1 * time.Hour)

	// Initialize Rules Engine
	rulesEngine := rules.NewEngine()
	if store != nil {
		customRules, err := store.GetCustomRules(ctx)
		if err == nil && len(customRules) > 0 {
			for _, r := range customRules {
				if r.Enabled {
					_ = rulesEngine.AddRule(r.RuleText)
				}
			}
		} else {
			// Fallback defaults if no custom rules exist
			for _, rule := range defaultRules() {
				_ = rulesEngine.AddRule(rule)
			}
		}
	} else {
		// Fallback defaults when storage is unavailable
		for _, rule := range defaultRules() {
			_ = rulesEngine.AddRule(rule)
		}
	}

	// Start auto-reports scheduler
	if store != nil {
		reportsDir := filepath.Join("data", "reports")
		os.MkdirAll(reportsDir, 0o755)
		reports.StartScheduler(ctx, store, reportsDir)
	}

	wire := newWireUp(&cfg, rg, querier, dnsResolver, captureHandle, geoipSvc, rulesEngine)
	// Set broker after wire creation to break the circular dependency:
	// captureHandle → broker ← wire → captureHandle → RouterDeps.
	captureHandle.Broker = wire.Broker

	// Start ML and Rules evaluation worker
	// Initialize ML Engine.
	// Pass the project's onnxruntime.dll explicitly to avoid loading
	// an older system-wide version from C:\WINDOWS\system32\.
	mlDLL := "onnxruntime.dll"
	if abs, err := filepath.Abs(mlDLL); err == nil {
		mlDLL = abs
	}
	if _, statErr := os.Stat(mlDLL); statErr != nil {
		mlDLL = "" // let the runtime search PATH
	}
	mlEngine, err := ml.NewEngine(mlDLL, "models")
	if err != nil {
		log.Printf("ml engine init failed: %v", err)
	} else {
		defer mlEngine.Close()
	}

	// Flood detector — statistical heuristics independent of ONNX model.
	floodDetector := ml.NewFloodDetector()

	// Brute-force detector — behavioral SYN counting for SSH/Telnet.
	bruteForceDetector := ml.NewBruteForceDetector()

	// Scan/recon detector — behavioral unique-destination-port counting for TCP SYN scans.
	scanReconDetector := ml.NewScanReconDetector()

	// Start evaluation worker pool
	// We use 4 workers to parallelize ML and Rules evaluation, preventing the bridge from blocking.
	evalDone := make(chan struct{})
	go func() {
		defer close(evalDone)
		workerDone := make(chan struct{})
		numWorkers := 4
		for i := 0; i < numWorkers; i++ {
			go func() {
				defer func() { workerDone <- struct{}{} }()
				for e := range evalPackets {
					// 0. Scanner Traffic Filter (Auto-Attack exclusion)
					// Skip packets to/from actively scanned targets
					if _, scanningSrc := scanner.ActiveScans.Load(e.SrcIP); scanningSrc {
						continue
					}
					if _, scanningDst := scanner.ActiveScans.Load(e.DstIP); scanningDst {
						continue
					}

					if discEngine != nil {
						discEngine.Process(ctx, e)
					}

					// 0b. Flood Detector — record every packet for rate analysis
					floodDetector.Record(e)

					// 0c. Brute Force Detector — record SYN attempts to SSH/Telnet
					bruteForceDetector.Record(e)

					// 0d. Scan/Recon Detector — record TCP SYN packets for port-scan analysis
					scanReconDetector.Record(e)

					// 1. Rule Engine
					matches := rulesEngine.Evaluate(e)
					for _, m := range matches {
						// log.Printf("Rule match: %s (Action: %s)", m.Options["msg"], m.Action)
						desc := fmt.Sprintf("Rule matched on %s %s:%d -> %s:%d", e.Protocol, e.SrcIP, e.SrcPort, e.DstIP, e.DstPort)
						alert := storage.Alert{
							Time:        e.Timestamp,
							Severity:    m.Options["severity"],
							Source:      e.SrcIP,
							Score:       100.0,
							Title:       m.Options["msg"],
							Description: &desc,
							Context:     []byte(fmt.Sprintf(`{"protocol":"%s", "action":"%s", "rule":"%s"}`, e.Protocol, m.Action, m.Options["msg"])),
						}
						if alert.Severity == "" {
							alert.Severity = "WARNING"
						}
						if writer != nil {
							_ = writer.WriteAlert(ctx, alert)
							// Broadcast alert
							if alertJSON, err := json.Marshal(alert); err == nil {
								wire.Broker.Broadcast(sse.Event{
									Name:  "alert",
									Topic: "alerts",
									Data:  alertJSON,
								})
							}
						}
					}

					// 2. Flood Detector — evaluate thresholds
					floodAlert := floodDetector.Evaluate(e)
					if floodAlert != nil {
						if writer != nil {
							_ = writer.WriteAlert(ctx, *floodAlert)
							if alertJSON, err := json.Marshal(floodAlert); err == nil {
								wire.Broker.Broadcast(sse.Event{
									Name:  "alert",
									Topic: "alerts",
									Data:  alertJSON,
								})
							}
						}
					}

					// 2b. Brute Force Detector — evaluate SSH/Telnet thresholds
					bruteForceAlert := bruteForceDetector.Evaluate(e)
					if bruteForceAlert != nil {
						if writer != nil {
							_ = writer.WriteAlert(ctx, *bruteForceAlert)
							if alertJSON, err := json.Marshal(bruteForceAlert); err == nil {
								wire.Broker.Broadcast(sse.Event{
									Name:  "alert",
									Topic: "alerts",
									Data:  alertJSON,
								})
							}
						}
					}

					// 2c. Scan/Recon Detector — evaluate TCP SYN scan thresholds
					scanReconAlert := scanReconDetector.Evaluate(e)
					if scanReconAlert != nil {
						if writer != nil {
							_ = writer.WriteAlert(ctx, *scanReconAlert)
							if alertJSON, err := json.Marshal(scanReconAlert); err == nil {
								wire.Broker.Broadcast(sse.Event{
									Name:  "alert",
									Topic: "alerts",
									Data:  alertJSON,
								})
							}
						}
					}

					// 3. ML Engine (ONNX-based anomaly detection)
					mlAlert, _ := mlEngine.Evaluate(e)
					if mlAlert != nil {
						if writer != nil {
							_ = writer.WriteAlert(ctx, *mlAlert)
							if alertJSON, err := json.Marshal(mlAlert); err == nil {
								wire.Broker.Broadcast(sse.Event{
									Name:  "alert",
									Topic: "alerts",
									Data:  alertJSON,
								})
							}
						}
					}
				}
			}()
		}
		for i := 0; i < numWorkers; i++ {
			<-workerDone
		}
	}()

	// Start the SSE broker and the 1Hz metrics ticker in
	// goroutines. The returned stop function blocks until both
	// goroutines have exited, so it is safe to call in the
	// shutdown defer below.
	stopWire := wire.Start(ctx)

	// Broadcast initial capture status to the frontend.
	// This runs after the broker is started so the event is buffered
	// and delivered to any connecting SSE client.
	captureStatus := "paused"
	if captureRunning {
		captureStatus = "running"
	}
	if statusJSON, err := json.Marshal(map[string]string{"status": captureStatus}); err == nil {
		wire.Broker.Broadcast(sse.Event{
			Name:  "capture-status",
			Topic: "capture-status",
			Data:  statusJSON,
		})
	}

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
	log.Printf("ViewRed %s listening on http://%s", version.Version, addr)

	// Start auto-updater in background. It checks for updates every 6 hours.
	// On dev builds ("0.1.0-dev") the updater silently disables itself.
	up, upErr := updater.New()
	if upErr != nil {
		log.Printf("updater: %v", upErr)
	} else {
		go up.Run(ctx)
	}

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
		Capturer:    captureAdapter{cap},
		Packets:     packets,
		EvalPackets: evalPackets,
		PcapWriter:  pcapWriterAdapter{pw},
		Storage:     storeAdapter{store},
		Writer:      writerAdapter{writer},
		Server:      httpServer,
		Broker:      wire.Broker,
		EvalDone:    evalDone,
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
			return filepath.ToSlash(c)
		}
	}
	return "internal/storage/migrations" // best-effort fallback
}
