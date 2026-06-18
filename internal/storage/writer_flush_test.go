package storage

import (
	"context"
	"testing"
	"time"

	"github.com/viewred/viewred/internal/capture"
)

// TestWriterDoneChannelClosesAfterRunExits verifies that Done()'s
// channel is closed once Run returns. This is the synchronization
// point the shutdown function relies on before closing pcap writer
// and storage pool.
func TestWriterDoneChannelClosesAfterRunExits(t *testing.T) {
	w := NewWriter(nil) // nil storage → nil pool
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		w.Run(ctx, make(chan capture.PacketEvent)) // unused events chan; Run exits immediately on nil pool
		close(done)
	}()

	select {
	case <-w.Done():
		// good
	case <-time.After(1 * time.Second):
		t.Fatal("Done() channel was not closed within 1s after Run returned")
	}

	select {
	case <-done:
		// good — Run actually returned
	case <-time.After(1 * time.Second):
		t.Fatal("Run did not return within 1s")
	}
}

// TestWriterDoneIsIdempotent verifies that calling Done() multiple
// times returns the same channel and that the channel is closed
// exactly once when Run is called.
func TestWriterDoneIsIdempotent(t *testing.T) {
	w := NewWriter(nil)
	ch1 := w.Done()
	ch2 := w.Done()
	if ch1 != ch2 {
		t.Fatal("Done() should return the same channel on repeated calls")
	}
	// Channel is open until Run is called.
	select {
	case <-ch1:
		t.Fatal("Done() channel should not be closed before Run is called")
	default:
	}

	// First Run closes the channel.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		w.Run(ctx, make(chan capture.PacketEvent))
		close(done)
	}()
	<-done

	// Second Done() call returns the same (now-closed) channel.
	ch3 := w.Done()
	select {
	case <-ch3:
		// good
	default:
		t.Fatal("Done() channel should be closed after Run returns")
	}
}
