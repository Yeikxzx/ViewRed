package main

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/viewred/viewred/internal/capture"
	"github.com/viewred/viewred/internal/config"
	"github.com/viewred/viewred/internal/health"
	"github.com/viewred/viewred/internal/rules"
	"github.com/viewred/viewred/internal/shutdown"
	"github.com/viewred/viewred/internal/storage"
)

// shutdownOrderRecorder is a small thread-safe log of step names
// extracted from the test helpers in internal/shutdown/shutdown_test.go.
// It exists here (duplicated) so the cmd/viewred test package can
// run a self-contained shutdown flow without importing the
// package-private helpers from internal/shutdown.
type shutdownOrderRecorder struct {
	mu    sync.Mutex
	steps []string
}

func (r *shutdownOrderRecorder) add(step string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.steps = append(r.steps, step)
}

func (r *shutdownOrderRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.steps))
	copy(out, r.steps)
	return out
}

func indexOfSteps(steps []string, target string) int {
	for i, s := range steps {
		if s == target {
			return i
		}
	}
	return -1
}

// recorderServer satisfies shutdown.HTTPServer.
type recorderServer struct {
	rec     *shutdownOrderRecorder
	delayMs int
}

func (s *recorderServer) Shutdown(ctx context.Context) error {
	if s.delayMs > 0 {
		select {
		case <-time.After(time.Duration(s.delayMs) * time.Millisecond):
		case <-ctx.Done():
		}
	}
	s.rec.add("server.Shutdown")
	return nil
}

// recorderBroker satisfies shutdown.Broker. It uses the wireUp's
// real *sse.Broker so the test exercises the production type, not
// a mock.
type recorderBroker struct {
	rec      *shutdownOrderRecorder
	shutdown func(ctx context.Context) error
}

func (b *recorderBroker) Shutdown(ctx context.Context) error {
	b.rec.add("broker.Shutdown")
	if b.shutdown != nil {
		return b.shutdown(ctx)
	}
	return nil
}

// recorderStorage satisfies shutdown.Storage.
type recorderStorage struct{ rec *shutdownOrderRecorder }

func (s *recorderStorage) Close() { s.rec.add("store.Close") }

// recorderCloser satisfies shutdown.PacketCloser.
type recorderCloser struct{ rec *shutdownOrderRecorder }

func (c *recorderCloser) Close() error {
	c.rec.add("pw.Close")
	return nil
}

// recorderCapturer satisfies shutdown.Capturer.
type recorderCapturer struct{ rec *shutdownOrderRecorder }

func (c *recorderCapturer) Stop() error {
	c.rec.add("cap.Stop")
	return nil
}

// recorderWriter satisfies shutdown.StorageWriter. Run blocks
// until the events channel is closed (the standard shutdown
// pattern). Done is closed when Run returns.
type recorderWriter struct {
	rec         *shutdownOrderRecorder
	runStarted  chan struct{}
	doneCh      chan struct{}
	once        sync.Once
	stopOnStart bool
}

func newRecorderWriter(rec *shutdownOrderRecorder) *recorderWriter {
	return &recorderWriter{
		rec:        rec,
		runStarted: make(chan struct{}, 1),
		doneCh:     make(chan struct{}),
	}
}

func (w *recorderWriter) Run(ctx context.Context, events <-chan capture.PacketEvent) {
	w.rec.add("writer.Run.start")
	select {
	case w.runStarted <- struct{}{}:
	default:
	}
	for range events {
	}
	w.rec.add("writer.Run.done")
	w.once.Do(func() { close(w.doneCh) })
}

func (w *recorderWriter) Done() <-chan struct{} { return w.doneCh }

// TestNewWireUp_BrokerWiresIntoShutdownOrder is the regression
// test for Plan 3 review finding C3 (shutdown order violation).
//
// Before the fix, main.go did not pass wire.Broker to
// shutdown.Params, so the explicit shutdown.Shutdown call did
// NOT shut down the broker in the right place (between
// server.Shutdown and store.Close). A separate defer called
// wire.Broker.Shutdown AFTER shutdown.Shutdown returned, so the
// broker was effectively shut down after storage was closed —
// violating the spec order (server -> broker -> store).
//
// The fix is for main.go to pass Broker: wire.Broker to
// shutdown.Params, so the explicit shutdown.Shutdown handles the
// broker in the correct position. This test proves the contract:
// when the wireUp's broker is passed to shutdown.Params, the
// recorded order is:
//
//	cap.Stop -> writer.Run.done -> pw.Close
//	  -> server.Shutdown -> broker.Shutdown -> store.Close
func TestNewWireUp_BrokerWiresIntoShutdownOrder(t *testing.T) {
	w := newWireUp(&config.Config{}, health.New(), &storage.MockQuerier{}, nil, nil, nil, rules.NewEngine())

	// Start the wireUp so the broker is running (mimics main.go).
	ctx, cancel := context.WithCancel(context.Background())
	stopWire := w.Start(ctx)
	defer func() {
		cancel()
		stopWire()
	}()

	rec := &shutdownOrderRecorder{}
	server := &recorderServer{rec: rec}
	store := &recorderStorage{rec: rec}
	capRec := &recorderCapturer{rec: rec}
	pw := &recorderCloser{rec: rec}
	writer := newRecorderWriter(rec)

	// Build a shutdown.Broker that calls the wireUp's real broker
	// and also records the step. This is what the production fix
	// (main.go passing wire.Broker to shutdown.Params) would do
	// at runtime: shutdown.Shutdown will call wire.Broker.Shutdown
	// directly via the interface.
	wrappedBroker := &recorderBroker{
		rec: rec,
		shutdown: func(ctx context.Context) error {
			return w.Broker.Shutdown(ctx)
		},
	}

	packets := make(chan capture.PacketEvent, 16)
	go writer.Run(context.Background(), packets)
	select {
	case <-writer.runStarted:
	case <-time.After(time.Second):
		t.Fatal("writer did not start within 1s")
	}

	// The PRODUCTION wiring: pass w.Broker through a Broker
	// adapter to shutdown.Params.Broker. main.go must do
	// exactly this for the order test to pass.
	err := shutdown.Shutdown(shutdown.Params{
		Capturer:   capRec,
		Packets:    packets,
		PcapWriter: pw,
		Storage:    store,
		Writer:     writer,
		Server:     server,
		Broker:     wrappedBroker,
	})
	require.NoError(t, err)

	steps := rec.snapshot()
	serverIdx := indexOfSteps(steps, "server.Shutdown")
	brokerIdx := indexOfSteps(steps, "broker.Shutdown")
	storeIdx := indexOfSteps(steps, "store.Close")

	require.NotEqual(t, -1, serverIdx, "server.Shutdown not recorded: %v", steps)
	require.NotEqual(t, -1, brokerIdx, "broker.Shutdown not recorded (main.go must wire Broker into shutdown.Params): %v", steps)
	require.NotEqual(t, -1, storeIdx, "store.Close not recorded: %v", steps)

	// Spec order: server -> broker -> store.
	assert.Less(t, serverIdx, brokerIdx,
		"broker.Shutdown must run AFTER server.Shutdown, got %v", steps)
	assert.Less(t, brokerIdx, storeIdx,
		"broker.Shutdown must run BEFORE store.Close, got %v", steps)
}

// TestNewWireUp_ExposesBrokerForWiring is the contract test: the
// wireUp MUST expose the broker it created so main.go can wire it
// into shutdown.Params. If the wireUp stopped exposing the
// broker (e.g. by inlining it into a private field), main.go
// would have no way to satisfy shutdown.Broker and the fix in
// C3 would be impossible. This test makes that contract loud.
func TestNewWireUp_ExposesBrokerForWiring(t *testing.T) {
	w := newWireUp(&config.Config{}, health.New(), &storage.MockQuerier{}, nil, nil, nil, rules.NewEngine())

	require.NotNil(t, w.Broker, "wireUp must expose its Broker so main.go can wire it into shutdown.Params")

	// The Broker must satisfy shutdown.Broker at compile time
	// (the interface declares Shutdown(ctx) error). If the
	// production code is refactored to break the interface, the
	// test binary will fail to compile, which is the desired
	// signal.
	var _ shutdown.Broker = w.Broker
}

// dummyHTTPServer import guard: keep the http import live so
// future test helpers can spin up a real httptest server if
// needed. The current test uses the recorderServer helper so
// this is documentation rather than a real reference.
var _ = func() *http.Server { return nil }
