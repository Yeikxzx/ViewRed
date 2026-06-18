// Package shutdown coordinates the ordered teardown of the ViewRed
// subsystems (capture, storage writer, pcap writer, HTTP server, and
// storage pool) so that no subsystem is closed while another subsystem
// is still trying to use it.
//
// Shutdown is the single source of truth for teardown order; main.go
// calls it explicitly on SIGINT/SIGTERM and falls back to the deferred
// Close() chain only on panic.
//
// The required order (spec §9.3) is:
//
//  1. Stop the capture driver (no new packets).
//  2. Close the packets channel (signals downstream consumers to exit).
//  3. Wait for the storage writer's Run loop to return.
//  4. Flush the final batch with a FRESH context (not the cancelled
//     RunContext) so the last batch is not dropped.
//  5. Close the pcap writer (releases the file).
//  6. Shutdown the HTTP server (drains in-flight requests).
//  7. Close the storage pool last.
package shutdown

import (
	"context"
	"time"

	"github.com/viewred/viewred/internal/capture"
)

// Capturer is the subset of capture.PacketCapture that Shutdown needs.
type Capturer interface {
	Stop() error
}

// PacketCloser is anything with a Close() error method (pcap writer, etc.).
type PacketCloser interface {
	Close() error
}

// HTTPServer is the subset of *http.Server that Shutdown needs.
type HTTPServer interface {
	Shutdown(ctx context.Context) error
}

// Storage is the subset of *storage.Storage that Shutdown needs.
type Storage interface {
	Close()
}

// StorageWriter is the subset of *storage.Writer that Shutdown needs.
// Run is the main loop (consumes events from a channel). Done returns
// a channel that is closed when Run returns — Shutdown waits on this
// before closing the pcap writer and storage pool, to make sure Run's
// final batch flush (which uses a fresh context internally) has
// completed.
type StorageWriter interface {
	Run(ctx context.Context, events <-chan capture.PacketEvent)
	Done() <-chan struct{}
}

// Params configures a Shutdown call. All fields are optional (nil is
// safe and means "skip this subsystem") except FlushTimeout which
// defaults to 10 seconds (spec §9.3: "flush DB writes (10s timeout)").
type Params struct {
	// Capturer is the packet capture driver (Npcap/AF_PACKET/Mock).
	// nil is safe (e.g. capture failed to start).
	Capturer Capturer
	// Packets is the channel from capture to writers. Shutdown closes
	// this channel after Capturer.Stop returns, which unblocks the
	// pcap-writer bridge goroutine and signals the storage writer's
	// Run loop to exit. nil is safe.
	Packets chan capture.PacketEvent
	// PcapWriter is the rotating pcap file writer. nil is safe.
	PcapWriter PacketCloser
	// Storage is the pgx pool wrapper. nil is safe.
	Storage Storage
	// Writer is the batched DB writer. nil is safe.
	Writer StorageWriter
	// Server is the HTTP API server. nil is safe.
	Server HTTPServer

	// FlushTimeout is the budget for the final DB flush and for the
	// HTTP server graceful drain. If zero, defaults to 10 seconds.
	FlushTimeout time.Duration
}

// Shutdown tears down all subsystems in the order required by the spec.
// It returns the first error encountered; subsequent steps are still
// attempted so each subsystem gets a chance to release its resources.
//
// A nil pointer in any of the subsystem fields is safe — Shutdown
// simply skips that step. This matches main.go's pattern of setting
// store/pw to nil on init failure.
//
// CRITICAL #4 (final batch dropped on shutdown) is fixed inside
// *storage.Writer.Run itself: the final flush uses a fresh context
// (context.WithTimeout(context.Background(), finalFlushTimeout)) — NOT
// the cancelled run context. So Shutdown only needs to wait for
// Writer.Done() to confirm the final flush is complete.
func Shutdown(p Params) error {
	if p.FlushTimeout <= 0 {
		p.FlushTimeout = 10 * time.Second
	}

	var firstErr error
	record := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	// 1. Stop capture first so it stops pushing events into the packets
	//    channel. The Npcap/AF_PACKET drivers intentionally leave their
	//    Packets() channel open (per design comment in npcap_windows.go).
	if p.Capturer != nil {
		record(p.Capturer.Stop())
	}

	// 2. Close the packets channel so the storage writer's Run() returns
	//    (its select on the channel will see ok=false) and the pcap-writer
	//    bridge's for-range loop exits naturally. This is the fix for the
	//    goroutine leak in the pcap bridge (CRITICAL #3).
	if p.Packets != nil {
		close(p.Packets)
	}

	// 3. Wait for the storage writer's Run() to return. The writer's
	//    final flush (with a fresh context) happens inside Run, so by
	//    the time Done() is closed, the in-memory batch is safely
	//    persisted and the goroutine has fully exited.
	if p.Writer != nil {
		<-p.Writer.Done()
	}

	// 4. Close the pcap writer. The pcap-bridge goroutine has already
	//    exited (step 2), so this just releases the file handle.
	if p.PcapWriter != nil {
		record(p.PcapWriter.Close())
	}

	// 5. Shutdown the HTTP server with a fresh timeout context so we
	//    still drain in-flight requests even if RunContext is cancelled.
	if p.Server != nil {
		srvCtx, cancel := context.WithTimeout(context.Background(), p.FlushTimeout)
		record(p.Server.Shutdown(srvCtx))
		cancel()
	}

	// 6. Close the storage pool last so no other subsystem tries to use
	//    it after shutdown.
	if p.Storage != nil {
		p.Storage.Close()
	}

	return firstErr
}
