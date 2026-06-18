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
