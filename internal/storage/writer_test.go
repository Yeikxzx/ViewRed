package storage

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/viewred/viewred/internal/capture"
)

func TestWriterBatchesAndInserts(t *testing.T) {
	db, sm, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Expect one batched INSERT with multiple rows.
	sm.ExpectExec(`INSERT INTO packets`).
		WillReturnResult(sqlmock.NewResult(0, 3))

	// We can't easily construct a real *pgxpool.Pool from sqlmock.
	// This test is illustrative; full integration covered in Task 9 (e2e).
	w := &Writer{batchSize: 3, flushEvery: 50 * time.Millisecond, stats: &WriterStats{}}
	_ = w
	_ = db
	_ = sm
	_ = context.Background
	_ = sync.WaitGroup{}
	t.Skip("integration test — needs pgxpool mock or real DB; covered in Task 9")
}

// --- Real unit tests for the public Writer API (no DB required) -----------

func TestNewWriterDefaults(t *testing.T) {
	s := &Storage{}
	w := NewWriter(s)
	if w == nil {
		t.Fatal("NewWriter returned nil")
	}
	if w.batchSize <= 0 {
		t.Errorf("expected positive batchSize, got %d", w.batchSize)
	}
	if w.flushEvery <= 0 {
		t.Errorf("expected positive flushEvery, got %v", w.flushEvery)
	}
	if w.stats == nil {
		t.Error("expected non-nil stats pointer")
	}
}

func TestNewWriterNilStorage(t *testing.T) {
	// Should not panic; returns Writer with nil pool.
	w := NewWriter(nil)
	if w == nil {
		t.Fatal("NewWriter(nil) returned nil")
	}
	if w.pool != nil {
		t.Error("expected nil pool for nil Storage")
	}
}

func TestWriterStatsZeroValue(t *testing.T) {
	var s WriterStats
	if s.Written != 0 {
		t.Errorf("zero Written: got %d, want 0", s.Written)
	}
	if s.Dropped != 0 {
		t.Errorf("zero Dropped: got %d, want 0", s.Dropped)
	}
	if s.BatchesSent != 0 {
		t.Errorf("zero BatchesSent: got %d, want 0", s.BatchesSent)
	}
}

func TestWriterStatsReturnsCurrentValues(t *testing.T) {
	s := &Storage{}
	w := NewWriter(s)
	stats := w.Stats()
	if stats.Written != 0 {
		t.Errorf("expected Written=0 on fresh writer, got %d", stats.Written)
	}
	if stats.Dropped != 0 {
		t.Errorf("expected Dropped=0 on fresh writer, got %d", stats.Dropped)
	}
}

func TestWriterCloseIsSafeWithNilPool(t *testing.T) {
	s := &Storage{}
	w := NewWriter(s)
	// Should not panic.
	w.Close()
	// Close is idempotent.
	w.Close()
}

func TestWriterRunExitsImmediatelyWithNilPool(t *testing.T) {
	s := &Storage{}
	w := NewWriter(s)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan capture.PacketEvent, 4)
	done := make(chan struct{})
	go func() {
		w.Run(ctx, ch)
		close(done)
	}()

	select {
	case <-done:
		// good — Run exited without panicking or hanging
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Run did not exit within 200ms when pool is nil")
	}
}
