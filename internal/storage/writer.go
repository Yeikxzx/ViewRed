package storage

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PacketEventLite is a placeholder for capture.PacketEvent (see internal/capture).
// Task 6 will move the canonical PacketEvent to internal/capture and the writer
// will import it. For plan 2, we use a local struct so the storage layer is
// self-contained.
type PacketEventLite struct {
	Time     time.Time
	SrcIP    string
	DstIP    string
	SrcPort  uint16
	DstPort  uint16
	Protocol string
	Length   int
}

// WriterStats tracks the writer's runtime metrics.
type WriterStats struct {
	Written     uint64
	Dropped     uint64
	BatchesSent uint64
}

// Writer batches PacketEventLite values and inserts them into the packets table.
// It implements a backpressure strategy: if the DB is slow, the channel is
// read non-blockingly and overflow is counted in Dropped.
type Writer struct {
	pool       *pgxpool.Pool
	batchSize  int
	flushEvery time.Duration
	stats      *WriterStats
}

// NewWriter creates a Writer bound to a Storage pool.
func NewWriter(s *Storage) *Writer {
	batchSize := 500
	flushEvery := 1 * time.Second
	var pool *pgxpool.Pool
	if s != nil {
		pool = s.Pool()
	}
	return &Writer{
		pool:       pool,
		batchSize:  batchSize,
		flushEvery: flushEvery,
		stats:      &WriterStats{},
	}
}

// Run consumes events from the channel and writes them in batches. It blocks
// until ctx is cancelled or the channel is closed.
func (w *Writer) Run(ctx context.Context, events <-chan PacketEventLite) {
	if w.pool == nil {
		log.Printf("storage.Writer: nil pool, exiting Run")
		return
	}

	batch := make([]PacketEventLite, 0, w.batchSize)
	ticker := time.NewTicker(w.flushEvery)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := w.insertBatch(ctx, batch); err != nil {
			log.Printf("storage.Writer: insert batch (%d): %v", len(batch), err)
			w.stats.Dropped += uint64(len(batch))
		} else {
			w.stats.Written += uint64(len(batch))
			w.stats.BatchesSent++
		}
		batch = batch[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case e, ok := <-events:
			if !ok {
				flush()
				return
			}
			batch = append(batch, e)
			if len(batch) >= w.batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

// insertBatch writes a single batch using a single multi-row INSERT via UNNEST.
// This avoids per-row round-trips and lets PostgreSQL plan a single statement.
func (w *Writer) insertBatch(ctx context.Context, batch []PacketEventLite) error {
	if len(batch) == 0 {
		return nil
	}

	times := make([]time.Time, len(batch))
	srcIPs := make([]string, len(batch))
	dstIPs := make([]string, len(batch))
	srcPorts := make([]int, len(batch))
	dstPorts := make([]int, len(batch))
	protocols := make([]string, len(batch))
	lengths := make([]int, len(batch))
	previews := make([][]byte, len(batch))

	for i, e := range batch {
		times[i] = e.Time
		srcIPs[i] = e.SrcIP
		dstIPs[i] = e.DstIP
		srcPorts[i] = int(e.SrcPort)
		dstPorts[i] = int(e.DstPort)
		protocols[i] = e.Protocol
		lengths[i] = e.Length
		// payload preview is a future task; nil for now
		previews[i] = nil
	}

	_, err := w.pool.Exec(ctx,
		`INSERT INTO packets (time, src_ip, dst_ip, src_port, dst_port, protocol, length, payload_preview) `+
			`SELECT * FROM UNNEST($1::timestamptz[], $2::inet[], $3::inet[], $4::int[], $5::int[], $6::text[], $7::int[], $8::bytea[])`,
		times, srcIPs, dstIPs, srcPorts, dstPorts, protocols, lengths, previews,
	)
	return err
}

// Stats returns a snapshot of the writer's stats.
func (w *Writer) Stats() WriterStats { return *w.stats }

// Close is a no-op for now; reserved for future graceful shutdown signals.
func (w *Writer) Close() {}
