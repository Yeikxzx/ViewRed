// Package storage — read-side query methods.
//
// These methods implement the Querier interface on *Storage. They run
// over a *sql.DB (sourced from the same pgxpool as the writer via
// stdlib.OpenDBFromPool) so sqlmock can intercept them in unit tests
// without a real database.
//
// SQL design choices (per docs/superpowers/designs/2026-06-18-core-api-and-ui-shell.md §3):
//   - All four KPI fields are computed in a single SELECT against the
//     packets hypertable so the dashboard ticker needs only one round
//     trip per poll.
//   - PacketsPerSec and BytesPerSec divide by the wall-clock window
//     (NOW() - $1) in seconds. This matches the "rate over a window"
//     semantic and is independent of how many chunks TimescaleDB
//     materialises.
//   - ActiveConnections counts distinct 5-tuples (src/dst IP, src/dst
//     port, protocol) — the spec's CONCAT approach. NULL ports collapse
//     to a shared tuple, which is the right behaviour for ICMP and
//     other port-less protocols.
//   - DeviceCount is the sum of distinct source and destination IPs.
//     A packet between 192.168.1.1 and 192.168.1.2 contributes 2.
package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/viewred/viewred/internal/capture"
)

// errStorageNotInitialized is returned by the read methods when the
// *Storage has no underlying *sql.DB. The most common path to this
// state is calling query methods on a *Storage{} literal (zero value)
// in a test, or main.go handing out the storage object before New
// finishes. Either way, the safe answer is "503 service unavailable",
// not a silent zero-valued success.
var errStorageNotInitialized = errors.New("storage: not initialized")

// qDashboardSummary returns the four KPI columns that drive the
// dashboard header cards. The query is intentionally a single SELECT
// — splitting it into multiple round-trips would multiply load on the
// packets hypertable for no real benefit (TimescaleDB can plan a
// single chunk scan).
//
// The window length is computed in SQL (NOW() - $1) so the rate
// denominator matches the filter predicate exactly. If a caller
// passes a `since` in the future, the denominator is negative and
// the rates will be negative; the spec delegates that edge case to
// the handler, which always passes "now - 60s".
const qDashboardSummary = `
SELECT
    COUNT(*)::float8 / EXTRACT(EPOCH FROM (NOW() - $1)) AS packets_per_sec,
    COALESCE(SUM(length), 0)::float8 / EXTRACT(EPOCH FROM (NOW() - $1)) AS bytes_per_sec,
    COUNT(DISTINCT CONCAT(src_ip, dst_ip, src_port, dst_port, protocol)) AS active_connections,
    (COUNT(DISTINCT src_ip) + COUNT(DISTINCT dst_ip))::bigint AS device_count
FROM packets
WHERE time >= $1
`

// QueryDashboardSummary returns the four KPIs the dashboard renders.
// `since` is the inclusive lower bound of the time window. A zero
// `since` is treated as "no lower bound" by the SQL (time >= epoch
// zero matches every row), which is rarely what callers want — the
// handler is responsible for substituting time.Now().Add(-60s) when
// the request omits ?since=....
//
// The function never returns an error for "no rows" — the SQL
// aggregation always produces one row (zeros when nothing matches),
// which is what the spec mandates.
func (s *Storage) QueryDashboardSummary(ctx context.Context, since time.Time) (DashboardSummary, error) {
	if s.db == nil {
		return DashboardSummary{}, errStorageNotInitialized
	}

	var summary DashboardSummary
	err := s.db.QueryRowContext(ctx, qDashboardSummary, since).Scan(
		&summary.PacketsPerSec,
		&summary.BytesPerSec,
		&summary.ActiveConnections,
		&summary.DeviceCount,
	)
	if err != nil {
		// sql.ErrNoRows is not expected here (the aggregation always
		// returns one row), but if it ever happens, we surface it
		// verbatim rather than masking it with a zero-valued result —
		// the caller should know their SQL is wrong.
		return DashboardSummary{}, err
	}
	return summary, nil
}

// QueryTopTalkers — design constraints (see plan §Task 6 and
// docs/superpowers/designs/2026-06-18-core-api-and-ui-shell.md §3):
//
//   - Aggregate by IP whether it appears as src or dst. A packet from
//     A to B contributes `length` bytes to A's bucket AND to B's
//     bucket; the bar chart then shows the noisiest devices on the
//     local network regardless of traffic direction.
//   - The result slice must be empty (not nil) when no rows match, so
//     the JSON encoder emits "[]" — handlers and JS clients don't
//     have to special-case nil.
//   - limit is clamped to [1, 100] before reaching SQL. The cap is
//     applied in code (not just SQL) so a buggy handler cannot ask
//     for an unbounded result set; storage is the last line of
//     defence against runaway queries against the packets hypertable.
//   - The query filters by `time >= $1` so TimescaleDB can prune
//     chunks. Dropping the predicate would do a full hypertable scan
//     on every poll.
//
// Defaults: limit <= 0 → 10. This is the same default the handler
// applies when ?limit= is missing, so the two layers agree.
const defaultTopTalkersLimit = 10
const maxTopTalkersLimit = 100

// qTopTalkers aggregates bytes per IP across both src and dst columns
// and returns the top N rows. The CTE form makes the dual projection
// obvious to a reader; an inline UNION ALL in the FROM clause is
// functionally equivalent but harder to scan.
//
// The inner SELECTs both carry the same `time >= $1` predicate so
// TimescaleDB can prune chunks inside each arm. Pulling the filter
// out of the CTE body would re-introduce a full-hypertable scan.
const qTopTalkers = `
WITH combined AS (
    SELECT src_ip AS ip, length AS bytes, 1 AS pkts
    FROM packets
    WHERE time >= $1
    UNION ALL
    SELECT dst_ip AS ip, length AS bytes, 1 AS pkts
    FROM packets
    WHERE time >= $1
)
SELECT ip, SUM(bytes)::bigint AS bytes, SUM(pkts)::bigint AS packets
FROM combined
GROUP BY ip
ORDER BY bytes DESC
LIMIT $2
`

// QueryTopTalkers returns the top-N IPs by total bytes (src or dst)
// observed in the window [since, now). The result is ordered by bytes
// descending. An empty window produces an empty (non-nil) slice and a
// nil error, per the spec.
func (s *Storage) QueryTopTalkers(ctx context.Context, since time.Time, limit int) ([]TopTalker, error) {
	if s.db == nil {
		return nil, errStorageNotInitialized
	}

	// Clamp limit in code. The handler also defaults/clips, but doing
	// it here means a future caller (e.g. a CLI tool, an SSE ticker's
	// internal sampling) cannot bypass the cap.
	if limit <= 0 {
		limit = defaultTopTalkersLimit
	}
	if limit > maxTopTalkersLimit {
		limit = maxTopTalkersLimit
	}

	rows, err := s.db.QueryContext(ctx, qTopTalkers, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Pre-allocate an empty slice (not nil) so JSON encodes "[]" when
	// the time window has no rows. Handlers can pass the result
	// straight to the JSON encoder.
	talkers := make([]TopTalker, 0)
	for rows.Next() {
		var t TopTalker
		if err := rows.Scan(&t.SrcIP, &t.Bytes, &t.Packets); err != nil {
			return nil, err
		}
		talkers = append(talkers, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return talkers, nil
}

// QueryProtocolDistribution — design constraints (see plan §Task 7
// and docs/superpowers/designs/2026-06-18-core-api-and-ui-shell.md §3):
//
//   - Aggregate bytes per protocol and return each protocol's
//     percentage share of the total. The donut chart in the dashboard
//     renders this directly.
//   - The percentage is computed in SQL with a window-function total
//     (SUM(SUM(length)) OVER ()) so it stays accurate as the row set
//     changes. The arithmetic is done once in the database rather
//     than in Go to avoid an N+1 round-trip (one COUNT query plus N
//     per-protocol queries would defeat the purpose of having SQL).
//   - NULLIF guards the divisor: when the time window is empty the
//     inner SUM is 0 and the divisor becomes NULL, so the percentage
//     is NULL too. The handler/GROUP BY still emits zero rows in that
//     case (no rows to group), so the guard is belt-and-braces for
//     the edge case where SQL reports one row with SUM=0.
//   - The result slice is empty (not nil) when no rows match so the
//     JSON encoder emits "[]" and the dashboard client doesn't have
//     to special-case null.
//   - The query filters by `time >= $1` so TimescaleDB can prune
//     chunks. Dropping the predicate would do a full hypertable scan
//     on every poll.
const qProtocolDistribution = `
SELECT protocol,
       (SUM(length) * 100.0 / NULLIF(SUM(SUM(length)) OVER (), 0))::float8 AS percentage
FROM packets
WHERE time >= $1
GROUP BY protocol
ORDER BY SUM(length) DESC
`

// QueryProtocolDistribution returns the share of total bytes each
// protocol contributes in the window [since, now). The shares are
// sorted by bytes descending so the donut chart can render the
// largest slice first. An empty window produces an empty (non-nil)
// slice and a nil error, per the spec.
func (s *Storage) QueryProtocolDistribution(ctx context.Context, since time.Time) ([]ProtocolShare, error) {
	if s.db == nil {
		return nil, errStorageNotInitialized
	}

	rows, err := s.db.QueryContext(ctx, qProtocolDistribution, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Pre-allocate an empty slice (not nil) so JSON encodes "[]" when
	// the time window has no rows. The dashboard client can then
	// render an empty ring without a nil-dereference check.
	shares := make([]ProtocolShare, 0)
	for rows.Next() {
		var ps ProtocolShare
		if err := rows.Scan(&ps.Protocol, &ps.Percentage); err != nil {
			return nil, err
		}
		shares = append(shares, ps)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return shares, nil
}

// QueryPackets — design constraints (see plan §Task 8 and
// docs/superpowers/specs/2026-06-18-core-api-and-ui-shell.md §2.4):
//
//   - Filter by time range [Since, Until], Protocol, SrcIP, DstIP. Empty
//     filter strings are "match anything"; the ($N = ” OR col = $N)
//     pattern lets the SQL plan with a single prepared statement
//     rather than one per filter combination, which is the right
//     trade-off because pgx caches the plan anyway and the planner
//     pushes the equality check down to a single chunk scan.
//   - Filter by `time >= $1 AND time <= $2` so TimescaleDB can prune
//     chunks. Dropping the bounds would do a full hypertable scan on
//     every request and is the #1 way to take the dashboard down.
//   - ORDER BY time DESC so the most recent packets appear first in
//     the UI list (the spec's "newest first" semantic). Without an
//     ORDER BY, Postgres is free to return rows in any order and the
//     UI's stable scrollback is broken.
//   - LIMIT $N OFFSET $M for pagination. Limit is clamped in code to
//     [1, 500] before reaching SQL so a buggy handler cannot ask for
//     an unbounded result set against the packets hypertable.
//   - totalCount is a separate SELECT COUNT(*) with the same WHERE
//     clause, NOT a windowed COUNT() OVER (). The windowed form is
//     tempting but it forces the planner to materialise the whole
//     filtered set before applying LIMIT — a single COUNT(*) over
//     the same predicate uses a chunk-pruned index-only scan and
//     stays sub-millisecond.
//   - The packets schema does not store MAC addresses, so SrcMAC /
//     DstMAC are returned as empty strings. The UI must already
//     tolerate this for tunnel-encapsulated traffic that lacks L2
//     info; we don't fabricate MACs that the database never had.
//
// Defaults: limit <= 0 → 50. This is the spec-mandated default
// (MUST default to 50 per the spec) and is the same default the
// handler applies when ?limit= is missing, so the two layers agree.
const defaultPacketsLimit = 50
const maxPacketsLimit = 500

// qPackets selects the filtered, paginated packet rows for the UI list.
// Argument order: $1 since, $2 until, $3 protocol, $4 src_ip, $5 dst_ip,
// $6 limit, $7 offset. The ($N = ” OR col = $N) guards let an empty
// filter string act as a wildcard, which is the spec's "any" semantic
// for an unfiltered column.
const qPackets = `
SELECT time, src_ip, dst_ip, src_port, dst_port, protocol, length, payload_preview
FROM packets
WHERE time >= $1 AND time <= $2
  AND ($3 = '' OR protocol = $3)
  AND ($4 = '' OR src_ip = $4)
  AND ($5 = '' OR dst_ip = $5)
  AND ($6 = 0 OR src_port = $6 OR dst_port = $6)
  AND ($7 = '' OR encode(payload_preview, 'escape') ILIKE '%' || $7 || '%')
ORDER BY time DESC
LIMIT $8 OFFSET $9
`

// qPacketsCount mirrors the WHERE clause of qPackets but emits a
// single integer — the total number of rows that match the filter,
// ignoring LIMIT/OFFSET. The handler/UI use this to render "page N of
// M" controls.
const qPacketsCount = `
SELECT COUNT(*)
FROM packets
WHERE time >= $1 AND time <= $2
  AND ($3 = '' OR protocol = $3)
  AND ($4 = '' OR src_ip = $4)
  AND ($5 = '' OR dst_ip = $5)
  AND ($6 = 0 OR src_port = $6 OR dst_port = $6)
  AND ($7 = '' OR encode(payload_preview, 'escape') ILIKE '%' || $7 || '%')
`

// QueryPackets returns a paginated, filtered slice of packet events
// plus the total count of matching rows. The result is ordered by
// time DESC (newest first). An empty result produces a non-nil empty
// slice and totalCount=0, per the spec.
func (s *Storage) QueryPackets(ctx context.Context, filter PacketFilter) ([]capture.PacketEvent, int, error) {
	if s.db == nil {
		return nil, 0, errStorageNotInitialized
	}

	// Clamp limit in code. The handler also defaults/clips, but doing
	// it here means a future caller (e.g. a CLI tool, an SSE ticker's
	// internal sampling) cannot bypass the cap and accidentally ask
	// the packets hypertable for an unbounded result set.
	limit := filter.Limit
	if limit <= 0 {
		limit = defaultPacketsLimit
	}
	if limit > maxPacketsLimit {
		limit = maxPacketsLimit
	}

	// data query — the filtered, paginated slice. The protocol/src_ip/
	// dst_ip filters are passed verbatim; the SQL uses the
	// ($N = '' OR col = $N) pattern to interpret an empty string as
	// "no filter" for that column.
	dataRows, err := s.db.QueryContext(ctx, qPackets,
		filter.Since,
		filter.Until,
		filter.Protocol,
		filter.SrcIP,
		filter.DstIP,
		filter.Port,
		filter.PayloadSearch,
		limit,
		filter.Offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer dataRows.Close()

	// Pre-allocate a non-nil empty slice so JSON encodes "[]" when no
	// rows match. The packet list UI renders an empty state without
	// nil-deref checks.
	events := make([]capture.PacketEvent, 0)
	for dataRows.Next() {
		var ev capture.PacketEvent
		if err := dataRows.Scan(
			&ev.Timestamp,
			&ev.SrcIP,
			&ev.DstIP,
			&ev.SrcPort,
			&ev.DstPort,
			&ev.Protocol,
			&ev.Length,
			&ev.PayloadPreview,
		); err != nil {
			return nil, 0, err
		}
		// The packets table does not store MAC addresses; leave the
		// L2 fields empty so the UI can render "<no L2>" consistently.
		events = append(events, ev)
	}
	if err := dataRows.Err(); err != nil {
		return nil, 0, err
	}

	// count query — the total number of rows that match the same
	// filter, ignoring LIMIT/OFFSET. Run after the data query so a
	// mid-run DB error on the count doesn't leave the caller
	// wondering whether the (possibly empty) data slice is "all
	// there is" or "truncated by a failure".
	var total int
	if err := s.db.QueryRowContext(ctx, qPacketsCount,
		filter.Since,
		filter.Until,
		filter.Protocol,
		filter.SrcIP,
		filter.DstIP,
		filter.Port,
		filter.PayloadSearch,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	return events, total, nil
}

// QueryDevices — design constraints (see plan §Task 9 and
// docs/superpowers/specs/2026-06-18-core-api-and-ui-shell.md §2 Query 5):
//
//   - Returns a paginated slice of discovered devices ordered by
//     `last_seen DESC` so the most recently active devices appear first
//     in the UI list. Without an explicit ORDER BY, Postgres is free to
//     return rows in any order and the device list's stable scrollback
//     is broken.
//   - Nullable columns (mac, vendor, hostname, risk_score) are scanned
//     via sql.Null* wrappers and translated to zero-value Go strings or
//     a nil *float64 so the JSON contract matches the design doc §3
//     (Device.RiskScore is *float64 because not every device has been
//     scored yet).
//   - totalCount is a separate SELECT COUNT(*) (no LIMIT/OFFSET applied)
//     so the UI can render "page N of M" controls even when
//     LIMIT/OFFSET shrinks the visible slice. The windowed form
//     (COUNT() OVER ()) is tempting but forces the planner to
//     materialise the full filtered set before applying LIMIT.
//   - LIMIT $N OFFSET $M for pagination. Limit is clamped in code to
//     [1, 500] before reaching SQL so a buggy handler cannot ask for
//     an unbounded result set against the devices table.
//
// Defaults: limit <= 0 → 50. This is the spec-mandated pagination
// default and matches QueryPackets's default so the two list endpoints
// behave consistently for the UI.
const defaultDevicesLimit = 50
const maxDevicesLimit = 500

// qDevices selects the paginated device rows for the UI list. Argument
// order: $1 limit, $2 offset. The COALESCE wrapping the nullable
// text columns converts SQL NULL into empty strings so the scanner can
// land them directly into Go string fields; the risk_score COALESCE
// means the column is never NULL on the wire even when the underlying
// value is null, but we still scan it via sql.NullFloat64 in code so
// the design doc's *float64 contract (NULL → nil) survives a future
// refactor that drops the COALESCE.
const qDevices = `
SELECT id, ip, mac, COALESCE(vendor, ''), COALESCE(hostname, ''),
       first_seen, last_seen, risk_score
FROM devices
ORDER BY last_seen DESC
LIMIT $1 OFFSET $2
`

// qDevicesCount returns the total number of device rows in the table,
// ignoring LIMIT/OFFSET. The handler/UI use this to render "page N of
// M" controls. The same WHERE-less form would still be correct if a
// future task adds filter columns (vendor, hostname, etc.) — but
// filters land as a separate parameter, not a new task, so the
// function stays single-purpose for now.
const qDevicesCount = `
SELECT COUNT(*)
FROM devices
`

// QueryDevices returns a paginated slice of discovered devices plus
// the total count of rows in the table. The result is ordered by
// `last_seen DESC` (most recently active first). An empty result
// produces a non-nil empty slice and totalCount=0, per the spec.
func (s *Storage) QueryDevices(ctx context.Context, filter DeviceFilter) ([]Device, int, error) {
	if s.db == nil {
		return nil, 0, errStorageNotInitialized
	}

	// Clamp limit in code. The handler also defaults/clips, but doing
	// it here means a future caller (e.g. a CLI tool, a future
	// exports endpoint) cannot bypass the cap and accidentally ask
	// the devices table for an unbounded result set.
	limit := filter.Limit
	if limit <= 0 {
		limit = defaultDevicesLimit
	}
	if limit > maxDevicesLimit {
		limit = maxDevicesLimit
	}

	// data query — the paginated slice, ordered by last_seen DESC.
	// The nullability of mac/vendor/hostname/risk_score is handled
	// per-column in the Scan call below; the COALESCE in the SQL is
	// belt-and-braces (vendor/hostname land as "" without it, but
	// risk_score stays NULL and must be handled in code).
	dataRows, err := s.db.QueryContext(ctx, qDevices, limit, filter.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer dataRows.Close()

	// Pre-allocate a non-nil empty slice so JSON encodes "[]" when
	// the devices table is empty. The device list UI renders an
	// empty state without nil-deref checks.
	devices := make([]Device, 0)
	for dataRows.Next() {
		var (
			d         Device
			mac       sql.NullString
			vendor    sql.NullString
			hostname  sql.NullString
			riskScore sql.NullFloat64
		)
		if err := dataRows.Scan(
			&d.ID,
			&d.IP,
			&mac,
			&vendor,
			&hostname,
			&d.FirstSeen,
			&d.LastSeen,
			&riskScore,
		); err != nil {
			return nil, 0, err
		}
		// Translate NULL columns to zero-value Go strings / a nil
		// *float64. The spec mandates *float64 for RiskScore so
		// "device not yet scored" is distinguishable from "device
		// scored 0.0" in the JSON response.
		if mac.Valid {
			d.MAC = mac.String
		}
		if vendor.Valid {
			d.Vendor = vendor.String
		}
		if hostname.Valid {
			d.Hostname = hostname.String
		}
		if riskScore.Valid {
			v := riskScore.Float64
			d.RiskScore = &v
		}
		devices = append(devices, d)
	}
	if err := dataRows.Err(); err != nil {
		return nil, 0, err
	}

	// count query — the total number of rows in the devices table,
	// ignoring LIMIT/OFFSET. Run after the data query so a mid-run
	// DB error on the count doesn't leave the caller wondering
	// whether the (possibly empty) data slice is "all there is" or
	// "truncated by a failure".
	var total int
	if err := s.db.QueryRowContext(ctx, qDevicesCount).Scan(&total); err != nil {
		return nil, 0, err
	}

	return devices, total, nil
}

// QueryAlerts returns a paginated list of alerts.
func (s *Storage) QueryAlerts(ctx context.Context, filter AlertFilter) ([]Alert, int, error) {
	if filter.Limit <= 0 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	countQuery := `SELECT COUNT(*) FROM alerts`
	var count int
	if err := s.db.QueryRowContext(ctx, countQuery).Scan(&count); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, time, severity, source, score, title, description, context, acknowledged_at
		FROM alerts
		ORDER BY time DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := s.db.QueryContext(ctx, query, filter.Limit, filter.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var alerts []Alert
	for rows.Next() {
		var a Alert
		if err := rows.Scan(
			&a.ID, &a.Time, &a.Severity, &a.Source, &a.Score, &a.Title,
			&a.Description, &a.Context, &a.AcknowledgedAt,
		); err != nil {
			return nil, 0, err
		}
		alerts = append(alerts, a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if alerts == nil {
		alerts = []Alert{}
	}

	return alerts, count, nil
}

// AcknowledgeAlert marks an alert as acknowledged by setting acknowledged_at to NOW().
func (s *Storage) AcknowledgeAlert(ctx context.Context, id int64) error {
	query := `UPDATE alerts SET acknowledged_at = NOW() WHERE id = $1 AND acknowledged_at IS NULL`
	res, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		// Just to emulate a "not found or already acked", but we can just return nil.
		// A REST API might want a 404/409, so maybe return a custom error?
		// For now we'll just return nil to keep it idempotent.
	}
	return nil
}


// Compile-time check: *Storage satisfies the Querier interface. If a
// future task adds a method to Querier without implementing it on
// *Storage, the build fails here, not in a downstream handler test.
// The same check is repeated inside querier.go for the test doubles
// (MockQuerier, ErrorQuerier); keeping a copy here pins the production
// type at the layer where the methods are defined.
var _ Querier = (*Storage)(nil)
