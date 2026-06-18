package shutdown

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/viewred/viewred/internal/capture"
)

// recording is a thread-safe ordered log of step names.
type recording struct {
	mu    sync.Mutex
	steps []string
}

func (r *recording) add(step string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.steps = append(r.steps, step)
}

func (r *recording) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.steps))
	copy(out, r.steps)
	return out
}

// mockCapturer records when Stop() is called.
type mockCapturer struct {
	stopErr error
	record  *recording
}

func (m *mockCapturer) Stop() error {
	m.record.add("cap.Stop")
	return m.stopErr
}

// mockCloser records when Close() is called.
type mockCloser struct {
	closeErr error
	record   *recording
}

func (m *mockCloser) Close() error {
	m.record.add("pw.Close")
	return m.closeErr
}

// mockServer records when Shutdown() is called and what context it received.
type mockServer struct {
	shutdownErr error
	record      *recording
}

func (m *mockServer) Shutdown(ctx context.Context) error {
	m.record.add("server.Shutdown")
	return m.shutdownErr
}

// mockStorage records when Close() is called.
type mockStorage struct {
	record *recording
}

func (m *mockStorage) Close() {
	m.record.add("store.Close")
}

// mockWriter records when Run starts and returns. The Done channel is
// closed when the for-range on the events channel exits (i.e. when
// Shutdown closes the channel).
type mockWriter struct {
	record     *recording
	runStarted chan struct{}
	doneCh     chan struct{}
	once       sync.Once
}

func newMockWriter(record *recording) *mockWriter {
	return &mockWriter{
		record:     record,
		runStarted: make(chan struct{}, 1),
		doneCh:     make(chan struct{}),
	}
}

func (m *mockWriter) Run(ctx context.Context, events <-chan capture.PacketEvent) {
	m.record.add("writer.Run.start")
	select {
	case m.runStarted <- struct{}{}:
	default:
	}
	// Block until the events channel is closed (which Shutdown does).
	for range events {
	}
	m.record.add("writer.Run.done")
	m.once.Do(func() { close(m.doneCh) })
}

func (m *mockWriter) Done() <-chan struct{} {
	return m.doneCh
}

// TestShutdownOrder is the regression test for CRITICAL #2 (shutdown
// defer order) and CRITICAL #3 (goroutine leak). It asserts the exact
// teardown order required by the spec:
//
//  1. cap.Stop
//  2. packets channel close → writer.Run.done
//  3. pw.Close
//  4. server.Shutdown
//  5. store.Close
//
// Before the fix, the LIFO defers in main.go would have closed pcap
// writer, capture, and storage in the wrong order, AND the pcap-bridge
// goroutine would have leaked because the packets channel was never
// closed.
func TestShutdownOrder(t *testing.T) {
	rec := &recording{}
	cap := &mockCapturer{record: rec}
	pw := &mockCloser{record: rec}
	store := &mockStorage{record: rec}
	server := &mockServer{record: rec}
	writer := newMockWriter(rec)

	packets := make(chan capture.PacketEvent, 16)

	// Start the writer in the background so Shutdown can close packets
	// and let Run() return.
	go writer.Run(context.Background(), packets)

	// Wait for the writer to be running before triggering shutdown.
	select {
	case <-writer.runStarted:
	case <-time.After(1 * time.Second):
		t.Fatal("writer did not start within 1s")
	}

	err := Shutdown(Params{
		Capturer:   cap,
		Packets:    packets,
		PcapWriter: pw,
		Storage:    store,
		Writer:     writer,
		Server:     server,
	})
	require.NoError(t, err)

	// writer.Run.start is recorded during test setup, before Shutdown
	// is called. The shutdown sequence is everything after that.
	expected := []string{
		"writer.Run.start", // (test setup: writer is running)
		"cap.Stop",         // 1. capture stops pushing events
		"writer.Run.done",  // 2. packets channel close → Run exits
		"pw.Close",         // 3. pcap writer closed
		"server.Shutdown",  // 4. HTTP server drained
		"store.Close",      // 5. storage pool closed last
	}
	assert.Equal(t, expected, rec.snapshot(),
		"shutdown order mismatch: got %v", rec.snapshot())
}

// TestShutdownWaitsForWriterDone is the regression test for the
// ordering dependency between the writer's Run loop and the
// pcap writer close. Before the fix, the shutdown function closed
// the pcap writer immediately after closing the packets channel,
// while Run was still draining. The fix: wait for Writer.Done() so
// Run has fully exited before any other teardown step runs.
func TestShutdownWaitsForWriterDone(t *testing.T) {
	rec := &recording{}
	cap := &mockCapturer{record: rec}
	pw := &mockCloser{record: rec}
	store := &mockStorage{record: rec}
	writer := newMockWriter(rec)

	packets := make(chan capture.PacketEvent, 16)
	go writer.Run(context.Background(), packets)

	select {
	case <-writer.runStarted:
	case <-time.After(1 * time.Second):
		t.Fatal("writer did not start within 1s")
	}

	err := Shutdown(Params{
		Capturer:   cap,
		Packets:    packets,
		PcapWriter: pw,
		Storage:    store,
		Writer:     writer,
	})
	require.NoError(t, err)

	// The Done() channel must be closed before pw.Close.
	steps := rec.snapshot()
	runDoneIdx := indexOf(steps, "writer.Run.done")
	pwCloseIdx := indexOf(steps, "pw.Close")
	require.NotEqual(t, -1, runDoneIdx, "writer.Run.done was not recorded")
	require.NotEqual(t, -1, pwCloseIdx, "pw.Close was not recorded")
	assert.Less(t, runDoneIdx, pwCloseIdx,
		"writer.Run.done must be recorded before pw.Close (got %v)", steps)
}

// TestShutdownReturnsFirstError verifies that Shutdown returns the
// first error encountered but still attempts all subsequent steps.
func TestShutdownReturnsFirstError(t *testing.T) {
	rec := &recording{}
	capErr := errors.New("capture boom")
	cap := &mockCapturer{stopErr: capErr, record: rec}
	pw := &mockCloser{record: rec}
	store := &mockStorage{record: rec}
	writer := newMockWriter(rec)

	packets := make(chan capture.PacketEvent, 16)
	go writer.Run(context.Background(), packets)

	select {
	case <-writer.runStarted:
	case <-time.After(1 * time.Second):
		t.Fatal("writer did not start within 1s")
	}

	err := Shutdown(Params{
		Capturer:   cap,
		Packets:    packets,
		PcapWriter: pw,
		Storage:    store,
		Writer:     writer,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, capErr)

	// All steps should still have been attempted.
	steps := rec.snapshot()
	assert.Contains(t, steps, "cap.Stop", "capture Stop should have been called")
	assert.Contains(t, steps, "writer.Run.done", "writer Run should have returned")
	assert.Contains(t, steps, "pw.Close", "pcap writer Close should have been called")
	assert.Contains(t, steps, "store.Close", "storage Close should have been called")
}

// TestShutdownHandlesNilParams verifies Shutdown is safe when some
// subsystems were not initialized (e.g. storage init failed, pcap
// writer init failed). This matches main.go's pattern of setting
// store/pw to nil on init failure.
func TestShutdownHandlesNilParams(t *testing.T) {
	rec := &recording{}
	cap := &mockCapturer{record: rec}
	writer := newMockWriter(rec)

	packets := make(chan capture.PacketEvent, 16)
	go writer.Run(context.Background(), packets)

	select {
	case <-writer.runStarted:
	case <-time.After(1 * time.Second):
		t.Fatal("writer did not start within 1s")
	}

	// Only capturer, packets, and writer are present. PcapWriter,
	// Storage, and Server are nil.
	err := Shutdown(Params{
		Capturer: cap,
		Packets:  packets,
		Writer:   writer,
	})
	require.NoError(t, err, "shutdown should not fail with nil subsystems")

	select {
	case <-writer.Done():
	case <-time.After(1 * time.Second):
		t.Fatal("writer did not return within 1s")
	}
}

// indexOf returns the index of target in steps, or -1 if not found.
func indexOf(steps []string, target string) int {
	for i, s := range steps {
		if s == target {
			return i
		}
	}
	return -1
}
