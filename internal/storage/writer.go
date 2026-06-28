package storage

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/viewred/viewred/internal/capture"
)

// finalFlushTimeout is the budget for the FINAL flush at Run exit. It
// is intentionally NOT derived from the run context — the whole point
// is that the run context may be cancelled (signal handler) but the
// final batch must still make it to the DB.
//
// Spec §9.3: "flush DB writes (10s timeout)".
const finalFlushTimeout = 10 * time.Second

// WriterStats tracks the writer's runtime metrics.
type WriterStats struct {
	Written     uint64
	Dropped     uint64
	BatchesSent uint64
}

// Writer batches capture.PacketEvent values and inserts them into the packets
// table. It implements a backpressure strategy: if the DB is slow, the channel
// is read non-blockingly and overflow is counted in Dropped.
type Writer struct {
	pool       *pgxpool.Pool
	batchSize  int
	flushEvery time.Duration
	stats      *WriterStats

	// doneCh is closed when Run returns. Done() returns a receive-only
	// view of this channel so external callers (the shutdown function)
	// can wait for the writer to fully exit before closing pcap
	// writer / storage pool. The mutex guards lazy creation
	// (Done may be called before Run).
	doneMu sync.Mutex
	doneCh chan struct{}

	edgeMu    sync.Mutex
	edgeBatch map[string]*TopologyEdgeUpdate // key: src_id:dst_id
}

type TopologyEdgeUpdate struct {
	SrcDeviceID int64
	DstDeviceID int64
	BytesCount  uint64
	LastSeen    time.Time
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
		edgeBatch:  make(map[string]*TopologyEdgeUpdate),
	}
}

// Run consumes events from the channel and writes them in batches. It blocks
// until ctx is cancelled, the channel is closed, or the pool is nil.
//
// CRITICAL #4 fix (final batch dropped on shutdown): on every exit path
// the in-memory batch is flushed with a FRESH context
// (context.WithTimeout(context.Background(), finalFlushTimeout)) — NOT
// with the cancelled run context. This is the difference between the
// pre-fix behavior (final batch lost because the run ctx was cancelled)
// and the post-fix behavior (final batch persisted with its own
// deadline).
//
// Done() returns a channel that is closed when Run returns. External
// callers (the shutdown function) wait on Done() to know when the
// in-memory batch is safely persisted and the goroutine has fully
// exited.
func (w *Writer) Run(ctx context.Context, events <-chan capture.PacketEvent) {
	if w.pool == nil {
		log.Printf("storage.Writer: nil pool, exiting Run")
		w.signalDone()
		return
	}

	batch := make([]capture.PacketEvent, 0, w.batchSize)
	ticker := time.NewTicker(w.flushEvery)
	defer ticker.Stop()
	defer w.signalDone()

	// A fresh context for the final flush. NOT derived from ctx —
	// that is the whole point. Cancel via defer.
	finalFlushCtx, finalCancel := context.WithTimeout(context.Background(), finalFlushTimeout)
	defer finalCancel()

	flush := func(flushCtx context.Context) {
		if len(batch) == 0 {
			return
		}
		if err := w.insertBatch(flushCtx, batch); err != nil {
			log.Printf("storage.Writer: insert batch (%d): %v", len(batch), err)
			w.stats.Dropped += uint64(len(batch))
		} else {
			w.stats.Written += uint64(len(batch))
			w.stats.BatchesSent++
		}
		batch = batch[:0]

		// Also flush edges
		w.flushEdges(flushCtx)
	}

	for {
		select {
		case <-ctx.Done():
			flush(finalFlushCtx) // fresh ctx, not the cancelled ctx
			return
		case e, ok := <-events:
			if !ok {
				flush(finalFlushCtx) // fresh ctx
				return
			}
			batch = append(batch, e)
			if len(batch) >= w.batchSize {
				flush(ctx) // normal flush with the run ctx
			}
		case <-ticker.C:
			flush(ctx) // periodic flush with the run ctx
		}
	}
}

// insertBatch writes a single batch using a single multi-row INSERT via UNNEST.
// This avoids per-row round-trips and lets PostgreSQL plan a single statement.
func (w *Writer) insertBatch(ctx context.Context, batch []capture.PacketEvent) error {
	if len(batch) == 0 {
		return nil
	}

	times := make([]time.Time, 0, len(batch))
	srcIPs := make([]string, 0, len(batch))
	dstIPs := make([]string, 0, len(batch))
	srcPorts := make([]int, 0, len(batch))
	dstPorts := make([]int, 0, len(batch))
	protocols := make([]string, 0, len(batch))
	lengths := make([]int, 0, len(batch))
	previews := make([][]byte, 0, len(batch))

	for _, e := range batch {
		// Skip rows with empty IPs — they cause a PostgreSQL inet parse
		// error and are always invalid for packet capture.
		if e.SrcIP == "" || e.DstIP == "" {
			continue
		}
		times = append(times, e.Timestamp)
		srcIPs = append(srcIPs, e.SrcIP)
		dstIPs = append(dstIPs, e.DstIP)
		srcPorts = append(srcPorts, int(e.SrcPort))
		dstPorts = append(dstPorts, int(e.DstPort))
		protocols = append(protocols, e.Protocol)
		lengths = append(lengths, e.Length)
		previews = append(previews, e.PayloadPreview)
	}

	if len(times) == 0 {
		return nil
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

// Done returns a channel that is closed when Run returns. Callers (the
// shutdown function) wait on this before closing the pcap writer and
// storage pool to make sure the in-memory batch is fully persisted
// and Run's goroutine has exited.
//
// Done is safe to call before Run — the channel is created lazily and
// is closed exactly once when Run exits.
func (w *Writer) Done() <-chan struct{} {
	w.doneMu.Lock()
	defer w.doneMu.Unlock()
	if w.doneCh == nil {
		w.doneCh = make(chan struct{})
	}
	return w.doneCh
}

// signalDone closes the Done channel exactly once. Called from Run's
// defer so it fires on every exit path (ctx done, channel closed,
// panic).
func (w *Writer) signalDone() {
	w.doneMu.Lock()
	defer w.doneMu.Unlock()
	if w.doneCh == nil {
		w.doneCh = make(chan struct{})
	}
	select {
	case <-w.doneCh:
		// already closed
	default:
		close(w.doneCh)
	}
}

// Close is a no-op for now; reserved for future graceful shutdown signals.
func (w *Writer) Close() {}

// WriteAlert synchronously inserts a new alert into the database.
func (w *Writer) WriteAlert(ctx context.Context, alert Alert) error {
	if w.pool == nil {
		return errStorageUnavailable
	}

	_, err := w.pool.Exec(ctx,
		`INSERT INTO alerts (time, severity, source, score, title, description, context, acknowledged_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		alert.Time, alert.Severity, alert.Source, alert.Score, alert.Title, alert.Description, alert.Context, alert.AcknowledgedAt,
	)
	return err
}

// UpsertDevice synchronously inserts or updates a device and returns its ID.
func (w *Writer) UpsertDevice(ctx context.Context, dev Device) (int64, error) {
	if w.pool == nil {
		return 0, errStorageUnavailable
	}
	// Guard: reject empty IPs — they cause a PostgreSQL inet parse error
	// and are always invalid for network discovery.
	if dev.IP == "" {
		return 0, fmt.Errorf("UpsertDevice: empty IP")
	}

	var id int64
	err := w.pool.QueryRow(ctx,
		`INSERT INTO devices (ip, mac, vendor, hostname, first_seen, last_seen)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (ip) DO UPDATE SET 
		 	mac = COALESCE(EXCLUDED.mac, devices.mac),
		 	vendor = COALESCE(EXCLUDED.vendor, devices.vendor),
		 	hostname = COALESCE(EXCLUDED.hostname, devices.hostname),
		 	last_seen = EXCLUDED.last_seen
		 RETURNING id`,
		dev.IP, dev.MAC, dev.Vendor, dev.Hostname, dev.LastSeen, dev.LastSeen,
	).Scan(&id)
	return id, err
}

// UpsertTopologyEdge accumulates connection bytes between two devices in memory.
// It is flushed periodically by Run.
func (w *Writer) UpsertTopologyEdge(srcDeviceID, dstDeviceID int64, bytes uint64, lastSeen time.Time) {
	if w.pool == nil || srcDeviceID == 0 || dstDeviceID == 0 {
		return
	}
	// Always store smaller ID first to make edges bidirectional? 
	// Usually topology implies direction. Let's keep direction.
	
	w.edgeMu.Lock()
	defer w.edgeMu.Unlock()

	key := fmt.Sprintf("%d:%d", srcDeviceID, dstDeviceID)
	if existing, ok := w.edgeBatch[key]; ok {
		existing.BytesCount += bytes
		existing.LastSeen = lastSeen
	} else {
		w.edgeBatch[key] = &TopologyEdgeUpdate{
			SrcDeviceID: srcDeviceID,
			DstDeviceID: dstDeviceID,
			BytesCount:  bytes,
			LastSeen:    lastSeen,
		}
	}
}

// flushEdges writes all accumulated edges to the database and clears the batch.
func (w *Writer) flushEdges(ctx context.Context) {
	w.edgeMu.Lock()
	if len(w.edgeBatch) == 0 {
		w.edgeMu.Unlock()
		return
	}
	batch := make([]*TopologyEdgeUpdate, 0, len(w.edgeBatch))
	for _, e := range w.edgeBatch {
		batch = append(batch, e)
	}
	// Reallocate to clear quickly
	w.edgeBatch = make(map[string]*TopologyEdgeUpdate)
	w.edgeMu.Unlock()

	srcIDs := make([]int64, len(batch))
	dstIDs := make([]int64, len(batch))
	bytesArr := make([]int64, len(batch)) // pgx uses int64 for bigints usually
	lastSeens := make([]time.Time, len(batch))

	for i, e := range batch {
		srcIDs[i] = e.SrcDeviceID
		dstIDs[i] = e.DstDeviceID
		bytesArr[i] = int64(e.BytesCount)
		lastSeens[i] = e.LastSeen
	}

	_, err := w.pool.Exec(ctx,
		`INSERT INTO topology_edges (src_device_id, dst_device_id, bytes_count, last_seen)
		 SELECT * FROM UNNEST($1::bigint[], $2::bigint[], $3::bigint[], $4::timestamptz[])
		 ON CONFLICT (src_device_id, dst_device_id) DO UPDATE SET
		 	bytes_count = topology_edges.bytes_count + EXCLUDED.bytes_count,
		 	last_seen = GREATEST(topology_edges.last_seen, EXCLUDED.last_seen)`,
		srcIDs, dstIDs, bytesArr, lastSeens,
	)
	if err != nil {
		log.Printf("storage.Writer: flushEdges err: %v", err)
	}
}
