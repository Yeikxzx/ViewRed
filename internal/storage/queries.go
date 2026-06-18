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
	"errors"
	"time"
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
    SELECT src_ip AS ip, length AS bytes
    FROM packets
    WHERE time >= $1
    UNION ALL
    SELECT dst_ip AS ip, length AS bytes
    FROM packets
    WHERE time >= $1
)
SELECT ip, SUM(bytes)::bigint AS bytes
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
		if err := rows.Scan(&t.IP, &t.Bytes); err != nil {
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
       (SUM(length) * 100.0 / NULLIF(SUM(SUM(length)) OVER (), 0))::float8 AS percent
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
		if err := rows.Scan(&ps.Protocol, &ps.Percent); err != nil {
			return nil, err
		}
		shares = append(shares, ps)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return shares, nil
}
