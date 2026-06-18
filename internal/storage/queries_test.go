// Package storage — query method tests.
//
// These tests pin the SQL contract of the read-side methods that the
// HTTP handlers and SSE ticker depend on. sqlmock intercepts the
// *sql.DB that wraps the pgxpool, so we can assert both the exact SQL
// we send and the value we scan back from a single row.
//
// We intentionally use the design doc §3 SQL (not the simplified
// pattern in the plan file) because the DashboardSummary struct has
// four fields and the handler JSON contract must match them 1:1.
package storage

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// Compile-time assertions: *Storage must have the Query* methods that
// the Querier interface demands. Each task adds the corresponding
// entry; once all five ship, the full interface check goes here.
func TestStorage_HasQueryDashboardSummary(t *testing.T) {
	var s *Storage
	_ = s.QueryDashboardSummary
}

func TestStorage_HasQueryTopTalkers(t *testing.T) {
	var s *Storage
	_ = s.QueryTopTalkers
}

func TestStorage_HasQueryProtocolDistribution(t *testing.T) {
	var s *Storage
	_ = s.QueryProtocolDistribution
}

func TestStorage_HasQueryPackets(t *testing.T) {
	var s *Storage
	_ = s.QueryPackets
}

// newMockStorage wires a *Storage around a sqlmock-backed *sql.DB so we
// can exercise the query methods without a real database. The pool and
// healthy fields are left at their zero values because none of the
// current read paths touch them.
func newMockStorage(t *testing.T) (*Storage, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	cleanup := func() { _ = sqlDB.Close() }
	return &Storage{db: sqlDB}, mock, cleanup
}

// Happy-path: the SQL returns the four KPI columns and the method
// must scan them into the DashboardSummary struct fields without
// losing precision (PacketsPerSec/BytesPerSec are float64).
func TestQueryDashboardSummary_ReturnsKPIs(t *testing.T) {
	s, mock, cleanup := newMockStorage(t)
	defer cleanup()

	rows := sqlmock.NewRows([]string{
		"packets_per_sec", "bytes_per_sec", "active_connections", "device_count",
	}).AddRow(1234.5, 4.2e6, int64(234), int64(47))

	mock.ExpectQuery(qDashboardSummary).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(rows)

	got, err := s.QueryDashboardSummary(context.Background(), time.Now().Add(-60*time.Second))
	if err != nil {
		t.Fatalf("QueryDashboardSummary: unexpected error: %v", err)
	}

	if got.PacketsPerSec != 1234.5 {
		t.Errorf("PacketsPerSec: got %v, want 1234.5", got.PacketsPerSec)
	}
	if got.BytesPerSec != 4.2e6 {
		t.Errorf("BytesPerSec: got %v, want 4.2e6", got.BytesPerSec)
	}
	if got.ActiveConnections != 234 {
		t.Errorf("ActiveConnections: got %d, want 234", got.ActiveConnections)
	}
	if got.DeviceCount != 47 {
		t.Errorf("DeviceCount: got %d, want 47", got.DeviceCount)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

// Empty result set: SQL returns one row with all-zero values, the
// method must return a zero-valued DashboardSummary, not an error.
// The spec explicitly says: "If no rows exist in the time window, MUST
// return zero-valued DashboardSummary (not an error)."
func TestQueryDashboardSummary_EmptyResultReturnsZeros(t *testing.T) {
	s, mock, cleanup := newMockStorage(t)
	defer cleanup()

	rows := sqlmock.NewRows([]string{
		"packets_per_sec", "bytes_per_sec", "active_connections", "device_count",
	}).AddRow(0.0, 0.0, int64(0), int64(0))

	mock.ExpectQuery(qDashboardSummary).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(rows)

	got, err := s.QueryDashboardSummary(context.Background(), time.Now().Add(-60*time.Second))
	if err != nil {
		t.Fatalf("QueryDashboardSummary: unexpected error on empty result: %v", err)
	}

	if got.PacketsPerSec != 0 || got.BytesPerSec != 0 {
		t.Errorf("expected zero rates, got packets_per_sec=%v bytes_per_sec=%v",
			got.PacketsPerSec, got.BytesPerSec)
	}
	if got.ActiveConnections != 0 || got.DeviceCount != 0 {
		t.Errorf("expected zero counts, got active=%d device=%d",
			got.ActiveConnections, got.DeviceCount)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

// DB error: a database failure (network, cancelled context, etc.)
// must surface as a non-nil error so the handler can map it to a
// 503 response. The spec is explicit that ErrorQuerier handles
// "DB unreachable at startup", but a mid-run error must also
// propagate.
func TestQueryDashboardSummary_PropagatesDBError(t *testing.T) {
	s, mock, cleanup := newMockStorage(t)
	defer cleanup()

	sentinel := errors.New("connection reset by peer")
	mock.ExpectQuery(qDashboardSummary).
		WithArgs(sqlmock.AnyArg()).
		WillReturnError(sentinel)

	_, err := s.QueryDashboardSummary(context.Background(), time.Now().Add(-60*time.Second))
	if err == nil {
		t.Fatal("expected error from QueryDashboardSummary, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected wrapped sentinel error, got %v", err)
	}
}

// Defensive: if a caller manages to construct a *Storage with a nil
// db (e.g. by calling New with an empty DSN before this commit, or by
// hand in a test), the query method must not panic. A nil-db check
// keeps the contract tight: an uninitialised *Storage never silently
// returns zero data.
func TestQueryDashboardSummary_NilDBReturnsError(t *testing.T) {
	s := &Storage{} // pool and db both nil
	_, err := s.QueryDashboardSummary(context.Background(), time.Now())
	if err == nil {
		t.Fatal("expected error when db is nil, got nil")
	}
}

// SQL contract: the SQL we send must include the WHERE clause that
// filters by `time >= $1`. This pins the partition pruning that
// TimescaleDB relies on for the packets hypertable. If a future
// refactor drops the filter, the dashboard would do a full-table scan
// on every poll and the test would fail.
func TestQueryDashboardSummary_SQLHasTimeFilter(t *testing.T) {
	const want = "WHERE time >= $1"
	if !strings.Contains(qDashboardSummary, want) {
		t.Errorf("dashboard summary SQL must contain %q\nGot:\n%s", want, qDashboardSummary)
	}
}

// ---------------------------------------------------------------------------
// QueryTopTalkers
//
// Aggregates bytes per IP, counting each IP once whether it appears as
// src or dst, and returns the top N rows ordered by bytes DESC. The
// contract is:
//
//   - The result slice is empty (not nil) when the time window has no
//     rows.
//   - The limit parameter is clamped to [1, 100] before being sent to
//     SQL so a malicious or buggy caller cannot ask for an unbounded
//     result set.
//   - The SQL filters by `time >= $1` so TimescaleDB can prune chunks.
//   - DB errors propagate so the HTTP handler can map them to 503.
// ---------------------------------------------------------------------------

// Happy path: the SQL returns three rows ordered by bytes (we trust
// the ORDER BY in the SQL; the test just asserts we scan them in the
// order the SQL produced them). The method must convert those rows
// into TopTalker values without losing the int64 byte counts.
func TestQueryTopTalkers_ReturnsTopN(t *testing.T) {
	s, mock, cleanup := newMockStorage(t)
	defer cleanup()

	rows := sqlmock.NewRows([]string{"ip", "bytes"}).
		AddRow("10.0.0.1", int64(1_000_000)).
		AddRow("10.0.0.2", int64(500_000)).
		AddRow("10.0.0.3", int64(250_000))

	mock.ExpectQuery(qTopTalkers).
		WithArgs(sqlmock.AnyArg(), 10).
		WillReturnRows(rows)

	got, err := s.QueryTopTalkers(context.Background(), time.Now().Add(-60*time.Second), 10)
	if err != nil {
		t.Fatalf("QueryTopTalkers: unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 talkers, got %d (%+v)", len(got), got)
	}
	wantIP := []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}
	wantBytes := []int64{1_000_000, 500_000, 250_000}
	for i, w := range wantIP {
		if got[i].IP != w {
			t.Errorf("talker[%d].IP: got %q, want %q", i, got[i].IP, w)
		}
		if got[i].Bytes != wantBytes[i] {
			t.Errorf("talker[%d].Bytes: got %d, want %d", i, got[i].Bytes, wantBytes[i])
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

// Empty result: SQL returns zero rows. The method MUST return a
// non-nil, zero-length slice so JSON marshalling produces "[]" not
// "null". A nil slice would force every handler to special-case the
// empty response.
func TestQueryTopTalkers_EmptyResultReturnsEmptySlice(t *testing.T) {
	s, mock, cleanup := newMockStorage(t)
	defer cleanup()

	rows := sqlmock.NewRows([]string{"ip", "bytes"}) // no AddRow

	mock.ExpectQuery(qTopTalkers).
		WithArgs(sqlmock.AnyArg(), 10).
		WillReturnRows(rows)

	got, err := s.QueryTopTalkers(context.Background(), time.Now().Add(-60*time.Second), 10)
	if err != nil {
		t.Fatalf("QueryTopTalkers on empty result: unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(got) != 0 {
		t.Errorf("expected len 0, got %d (%+v)", len(got), got)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

// DB error: a database failure (network, cancelled context, etc.)
// must surface as a non-nil error so the handler can map it to 503.
// This is the same mid-run error contract as QueryDashboardSummary.
func TestQueryTopTalkers_PropagatesDBError(t *testing.T) {
	s, mock, cleanup := newMockStorage(t)
	defer cleanup()

	sentinel := errors.New("connection reset by peer")
	mock.ExpectQuery(qTopTalkers).
		WithArgs(sqlmock.AnyArg(), 10).
		WillReturnError(sentinel)

	_, err := s.QueryTopTalkers(context.Background(), time.Now().Add(-60*time.Second), 10)
	if err == nil {
		t.Fatal("expected error from QueryTopTalkers, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected wrapped sentinel error, got %v", err)
	}
}

// Defensive: a *Storage{} literal (zero value, no db) must not panic.
// The nil-db guard returns the same error as QueryDashboardSummary so
// handlers can treat them uniformly.
func TestQueryTopTalkers_NilDBReturnsError(t *testing.T) {
	s := &Storage{} // pool and db both nil
	_, err := s.QueryTopTalkers(context.Background(), time.Now(), 10)
	if err == nil {
		t.Fatal("expected error when db is nil, got nil")
	}
}

// Limit cap: a caller asking for 500 rows must be clamped to 100
// before the SQL is sent. The cap is enforced in code (not just in
// SQL) so a buggy handler cannot request an unbounded result set —
// the storage layer is the last line of defence.
func TestQueryTopTalkers_CapsLimitAt100(t *testing.T) {
	s, mock, cleanup := newMockStorage(t)
	defer cleanup()

	rows := sqlmock.NewRows([]string{"ip", "bytes"})

	mock.ExpectQuery(qTopTalkers).
		WithArgs(sqlmock.AnyArg(), 100). // caller asked for 500, we send 100
		WillReturnRows(rows)

	if _, err := s.QueryTopTalkers(context.Background(), time.Now().Add(-60*time.Second), 500); err != nil {
		t.Fatalf("QueryTopTalkers: unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

// Limit floor: a caller asking for 0 or negative rows must get a
// sensible default (10). The SQL is still called once with the
// clamped limit so the dashboard never silently shows no data because
// the handler passed a bogus limit.
func TestQueryTopTalkers_NonPositiveLimitDefaultsTo10(t *testing.T) {
	s, mock, cleanup := newMockStorage(t)
	defer cleanup()

	rows := sqlmock.NewRows([]string{"ip", "bytes"})

	mock.ExpectQuery(qTopTalkers).
		WithArgs(sqlmock.AnyArg(), 10).
		WillReturnRows(rows)

	if _, err := s.QueryTopTalkers(context.Background(), time.Now().Add(-60*time.Second), 0); err != nil {
		t.Fatalf("QueryTopTalkers(0): unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

// SQL contract: the SQL must include the WHERE clause that filters
// by `time >= $1`. As with QueryDashboardSummary, this pins the
// partition pruning that TimescaleDB relies on. A refactor that
// drops the filter would do a full hypertable scan on every poll.
func TestQueryTopTalkers_SQLHasTimeFilter(t *testing.T) {
	const want = "WHERE time >= $1"
	if !strings.Contains(qTopTalkers, want) {
		t.Errorf("top talkers SQL must contain %q\nGot:\n%s", want, qTopTalkers)
	}
}

// ---------------------------------------------------------------------------
// QueryProtocolDistribution
//
// Aggregates bytes per protocol in the time window and returns each
// protocol's percentage share of the total. The contract is:
//
//   - The result slice is empty (not nil) when the time window has no
//     rows, so the dashboard donut chart renders an empty ring rather
//     than crashing on a nil dereference.
//   - The percentage is a float64 in [0, 100]. The full set of
//     percentages sums to ~100 (rounding tolerated).
//   - The SQL groups by protocol and filters by `time >= $1` so
//     TimescaleDB can prune chunks.
//   - DB errors propagate so the HTTP handler can map them to 503.
// ---------------------------------------------------------------------------

// Happy path: the SQL returns three rows (TCP, UDP, ICMP) with their
// respective shares of the total bytes. The method must scan them
// into ProtocolShare values preserving the float64 precision and the
// protocol-name column. We do not assert a specific order here — the
// SQL's ORDER BY SUM(length) DESC is verified separately.
func TestQueryProtocolDistribution_ReturnsDistribution(t *testing.T) {
	s, mock, cleanup := newMockStorage(t)
	defer cleanup()

	rows := sqlmock.NewRows([]string{"protocol", "percent"}).
		AddRow("TCP", 65.5).
		AddRow("UDP", 30.0).
		AddRow("ICMP", 4.5)

	mock.ExpectQuery(qProtocolDistribution).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(rows)

	got, err := s.QueryProtocolDistribution(context.Background(), time.Now().Add(-60*time.Second))
	if err != nil {
		t.Fatalf("QueryProtocolDistribution: unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 protocol shares, got %d (%+v)", len(got), got)
	}

	want := map[string]float64{"TCP": 65.5, "UDP": 30.0, "ICMP": 4.5}
	for _, ps := range got {
		w, ok := want[ps.Protocol]
		if !ok {
			t.Errorf("unexpected protocol in result: %q", ps.Protocol)
			continue
		}
		if ps.Percent != w {
			t.Errorf("protocol %q: got percent %v, want %v", ps.Protocol, ps.Percent, w)
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

// Empty result: SQL returns zero rows. The method MUST return a
// non-nil, zero-length slice so the JSON encoder emits "[]" not
// "null" — a nil slice would force the handler to special-case the
// empty state and the donut chart client to handle "null" responses.
func TestQueryProtocolDistribution_EmptyResultReturnsEmptySlice(t *testing.T) {
	s, mock, cleanup := newMockStorage(t)
	defer cleanup()

	rows := sqlmock.NewRows([]string{"protocol", "percent"}) // no AddRow

	mock.ExpectQuery(qProtocolDistribution).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(rows)

	got, err := s.QueryProtocolDistribution(context.Background(), time.Now().Add(-60*time.Second))
	if err != nil {
		t.Fatalf("QueryProtocolDistribution on empty result: unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(got) != 0 {
		t.Errorf("expected len 0, got %d (%+v)", len(got), got)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

// DB error: a database failure (network, cancelled context, etc.)
// must surface as a non-nil error so the handler can map it to 503.
// This is the same mid-run error contract as QueryDashboardSummary
// and QueryTopTalkers.
func TestQueryProtocolDistribution_PropagatesDBError(t *testing.T) {
	s, mock, cleanup := newMockStorage(t)
	defer cleanup()

	sentinel := errors.New("connection reset by peer")
	mock.ExpectQuery(qProtocolDistribution).
		WithArgs(sqlmock.AnyArg()).
		WillReturnError(sentinel)

	_, err := s.QueryProtocolDistribution(context.Background(), time.Now().Add(-60*time.Second))
	if err == nil {
		t.Fatal("expected error from QueryProtocolDistribution, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected wrapped sentinel error, got %v", err)
	}
}

// Defensive: a *Storage{} literal (zero value, no db) must not panic.
// The nil-db guard returns the same error as QueryDashboardSummary
// and QueryTopTalkers so handlers can treat them uniformly.
func TestQueryProtocolDistribution_NilDBReturnsError(t *testing.T) {
	s := &Storage{} // pool and db both nil
	_, err := s.QueryProtocolDistribution(context.Background(), time.Now())
	if err == nil {
		t.Fatal("expected error when db is nil, got nil")
	}
}

// SQL contract: the SQL must include the WHERE clause that filters
// by `time >= $1`. As with the other read methods, this pins the
// partition pruning that TimescaleDB relies on. A refactor that
// drops the filter would do a full hypertable scan on every poll.
func TestQueryProtocolDistribution_SQLHasTimeFilter(t *testing.T) {
	const want = "WHERE time >= $1"
	if !strings.Contains(qProtocolDistribution, want) {
		t.Errorf("protocol distribution SQL must contain %q\nGot:\n%s", want, qProtocolDistribution)
	}
}

// SQL contract: the SQL must group by protocol. Without the GROUP BY
// the response would collapse to a single row (the total of all
// protocols), which is not what the donut chart expects.
func TestQueryProtocolDistribution_SQLHasGroupByProtocol(t *testing.T) {
	const want = "GROUP BY protocol"
	if !strings.Contains(qProtocolDistribution, want) {
		t.Errorf("protocol distribution SQL must contain %q\nGot:\n%s", want, qProtocolDistribution)
	}
}

// ---------------------------------------------------------------------------
// QueryPackets
//
// Filters packets by time range (since/until), protocol, src_ip, dst_ip and
// returns a paginated slice ordered by time DESC, plus the total count of
// matching rows for pagination metadata. The contract is:
//
//   - Empty result returns a non-nil, zero-length slice and totalCount=0 so
//     JSON encodes "[]" and the UI renders an empty state without nil-deref.
//   - Limit defaults to 50 when zero/negative and caps at 500. The cap is
//     enforced in code (not just in SQL) so a buggy handler cannot ask for
//     an unbounded result set against the packets hypertable.
//   - totalCount is computed via a separate SELECT COUNT(*) with the same
//     WHERE clause, so the UI can render page N of M consistently even when
//     LIMIT/OFFSET shrinks the visible slice.
//   - DB errors propagate so the HTTP handler can map them to 503.
//   - The packets table does not store MAC addresses, so SrcMAC/DstMAC are
//     returned as empty strings (the handler/UI must already tolerate this
//     for back-compat with Packets that are tunnel-encapsulated and lack
//     L2 info).
// ---------------------------------------------------------------------------

// Happy path: SQL returns two packet rows and the COUNT(*) returns 42. The
// method must scan the rows into PacketEvent values (without MAC, since the
// schema doesn't store MACs) and return totalCount separately so the UI
// can render page 1 of (42/Limit).
func TestQueryPackets_ReturnsEventsAndCount(t *testing.T) {
	s, mock, cleanup := newMockStorage(t)
	defer cleanup()

	now := time.Now()
	eventRows := sqlmock.NewRows([]string{
		"time", "src_ip", "dst_ip", "src_port", "dst_port", "protocol", "length", "payload_preview",
	}).
		AddRow(now, "10.0.0.1", "10.0.0.2", int32(443), int32(51234), "TCP", int(1500), []byte{0xde, 0xad, 0xbe, 0xef}).
		AddRow(now.Add(-1*time.Second), "10.0.0.3", "10.0.0.4", int32(80), int32(62000), "TCP", int(800), []byte("HTTP"))

	mock.ExpectQuery(qPackets).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), 50, 0).
		WillReturnRows(eventRows)
	mock.ExpectQuery(qPacketsCount).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(42)))

	filter := PacketFilter{
		Since:  now.Add(-1 * time.Hour),
		Until:  now,
		Limit:  50,
		Offset: 0,
	}
	got, total, err := s.QueryPackets(context.Background(), filter)
	if err != nil {
		t.Fatalf("QueryPackets: unexpected error: %v", err)
	}
	if total != 42 {
		t.Errorf("totalCount: got %d, want 42", total)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 packets, got %d (%+v)", len(got), got)
	}
	if got[0].SrcIP != "10.0.0.1" || got[0].DstIP != "10.0.0.2" {
		t.Errorf("packet[0] IPs: got src=%q dst=%q, want 10.0.0.1/10.0.0.2", got[0].SrcIP, got[0].DstIP)
	}
	if got[0].Protocol != "TCP" || got[0].Length != 1500 {
		t.Errorf("packet[0] proto/length: got %q/%d, want TCP/1500", got[0].Protocol, got[0].Length)
	}
	if got[0].SrcPort != 443 || got[0].DstPort != 51234 {
		t.Errorf("packet[0] ports: got %d/%d, want 443/51234", got[0].SrcPort, got[0].DstPort)
	}
	if string(got[0].PayloadPreview) != string([]byte{0xde, 0xad, 0xbe, 0xef}) {
		t.Errorf("packet[0] payload: got %x, want deadbeef", got[0].PayloadPreview)
	}
	// MAC columns don't exist in the schema; the scanner must return empty strings.
	if got[0].SrcMAC != "" || got[0].DstMAC != "" {
		t.Errorf("packet[0] MAC: got src=%q dst=%q, want empty (schema has no MAC columns)", got[0].SrcMAC, got[0].DstMAC)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

// Empty result: SQL returns zero rows. The method MUST return a non-nil
// empty slice and totalCount=0 so the JSON encoder emits "[]" and the UI
// renders an empty state without nil-deref.
func TestQueryPackets_EmptyResultReturnsEmptySlice(t *testing.T) {
	s, mock, cleanup := newMockStorage(t)
	defer cleanup()

	mock.ExpectQuery(qPackets).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), 50, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"time", "src_ip", "dst_ip", "src_port", "dst_port", "protocol", "length", "payload_preview",
		})) // no AddRow

	mock.ExpectQuery(qPacketsCount).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))

	got, total, err := s.QueryPackets(context.Background(), PacketFilter{})
	if err != nil {
		t.Fatalf("QueryPackets on empty result: unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(got) != 0 {
		t.Errorf("expected len 0, got %d (%+v)", len(got), got)
	}
	if total != 0 {
		t.Errorf("expected totalCount 0, got %d", total)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

// DB error on the SELECT: the data query must surface a non-nil error so
// the handler can map it to 503. The COUNT query must not run.
func TestQueryPackets_PropagatesDBErrorOnSelect(t *testing.T) {
	s, mock, cleanup := newMockStorage(t)
	defer cleanup()

	sentinel := errors.New("connection reset by peer")
	mock.ExpectQuery(qPackets).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), 50, 0).
		WillReturnError(sentinel)

	_, _, err := s.QueryPackets(context.Background(), PacketFilter{})
	if err == nil {
		t.Fatal("expected error from QueryPackets, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected wrapped sentinel error, got %v", err)
	}
}

// Defensive: a *Storage{} literal (zero value, no db) must not panic.
// The nil-db guard returns the same error as the other query methods so
// handlers can treat them uniformly.
func TestQueryPackets_NilDBReturnsError(t *testing.T) {
	s := &Storage{} // pool and db both nil
	_, _, err := s.QueryPackets(context.Background(), PacketFilter{})
	if err == nil {
		t.Fatal("expected error when db is nil, got nil")
	}
}

// Limit cap: a caller asking for 5000 rows must be clamped to 500 before
// the SQL is sent. The cap is enforced in code (not just in SQL) so a
// buggy handler cannot request an unbounded result set against the
// packets hypertable.
func TestQueryPackets_CapsLimitAt500(t *testing.T) {
	s, mock, cleanup := newMockStorage(t)
	defer cleanup()

	filter := PacketFilter{
		Since: time.Now().Add(-1 * time.Hour),
		Until: time.Now(),
		Limit: 5000,
	}

	eventRows := sqlmock.NewRows([]string{
		"time", "src_ip", "dst_ip", "src_port", "dst_port", "protocol", "length", "payload_preview",
	})
	mock.ExpectQuery(qPackets).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), 500, 0). // caller asked for 5000, we send 500
		WillReturnRows(eventRows)
	mock.ExpectQuery(qPacketsCount).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))

	if _, _, err := s.QueryPackets(context.Background(), filter); err != nil {
		t.Fatalf("QueryPackets: unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

// Limit floor: a caller passing 0 or negative must get the spec default
// (50). The SQL is still called with the clamped limit so the table
// never silently shows no data because the handler passed a bogus limit.
func TestQueryPackets_NonPositiveLimitDefaultsTo50(t *testing.T) {
	s, mock, cleanup := newMockStorage(t)
	defer cleanup()

	eventRows := sqlmock.NewRows([]string{
		"time", "src_ip", "dst_ip", "src_port", "dst_port", "protocol", "length", "payload_preview",
	})
	mock.ExpectQuery(qPackets).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), 50, 0).
		WillReturnRows(eventRows)
	mock.ExpectQuery(qPacketsCount).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))

	if _, _, err := s.QueryPackets(context.Background(), PacketFilter{Limit: 0}); err != nil {
		t.Fatalf("QueryPackets(0): unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

// Filters propagate to SQL: the protocol, src_ip, and dst_ip arguments
// must reach the database (not be silently dropped). This pins the
// filter-by-equals semantics that the spec requires for the UI's
// filterable packet list.
func TestQueryPackets_PassesAllFiltersToSQL(t *testing.T) {
	s, mock, cleanup := newMockStorage(t)
	defer cleanup()

	now := time.Now()
	since := now.Add(-1 * time.Hour)
	until := now

	eventRows := sqlmock.NewRows([]string{
		"time", "src_ip", "dst_ip", "src_port", "dst_port", "protocol", "length", "payload_preview",
	})
	mock.ExpectQuery(qPackets).
		WithArgs(since, until, "TCP", "10.0.0.5", "10.0.0.6", 50, 0).
		WillReturnRows(eventRows)
	mock.ExpectQuery(qPacketsCount).
		WithArgs(since, until, "TCP", "10.0.0.5", "10.0.0.6").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))

	filter := PacketFilter{
		Since:    since,
		Until:    until,
		Protocol: "TCP",
		SrcIP:    "10.0.0.5",
		DstIP:    "10.0.0.6",
	}
	if _, _, err := s.QueryPackets(context.Background(), filter); err != nil {
		t.Fatalf("QueryPackets: unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

// Pagination: offset must be forwarded to the SQL so the UI can fetch
// page N by passing offset = N*limit. Without this, the packet list
// page control is broken.
func TestQueryPackets_OffsetPropagatesToSQL(t *testing.T) {
	s, mock, cleanup := newMockStorage(t)
	defer cleanup()

	eventRows := sqlmock.NewRows([]string{
		"time", "src_ip", "dst_ip", "src_port", "dst_port", "protocol", "length", "payload_preview",
	})
	mock.ExpectQuery(qPackets).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), 25, 100).
		WillReturnRows(eventRows)
	mock.ExpectQuery(qPacketsCount).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(125)))

	if _, total, err := s.QueryPackets(context.Background(), PacketFilter{Limit: 25, Offset: 100}); err != nil {
		t.Fatalf("QueryPackets: unexpected error: %v", err)
	} else if total != 125 {
		t.Errorf("totalCount: got %d, want 125", total)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

// SQL contract: the data query must include the WHERE clause that
// filters by `time >= $1 AND time <= $2`. As with the other read
// methods, this pins the partition pruning that TimescaleDB relies on.
func TestQueryPackets_SQLHasTimeRangeFilter(t *testing.T) {
	if !strings.Contains(qPackets, "time >= $1") {
		t.Errorf("packets SQL must contain time >= $1\nGot:\n%s", qPackets)
	}
	if !strings.Contains(qPackets, "time <= $2") {
		t.Errorf("packets SQL must contain time <= $2\nGot:\n%s", qPackets)
	}
}

// SQL contract: the data query must ORDER BY time DESC so the most
// recent packets appear first in the UI list. Without this clause the
// result order is undefined (Postgres-specific) and the UI's
// "newest first" rendering is broken.
func TestQueryPackets_SQLOrdersByTimeDesc(t *testing.T) {
	if !strings.Contains(qPackets, "ORDER BY time DESC") {
		t.Errorf("packets SQL must contain ORDER BY time DESC\nGot:\n%s", qPackets)
	}
}

// SQL contract: the data query must include LIMIT $N OFFSET $M for
// pagination. Without these clauses the entire hypertable would be
// scanned and returned on every request, which is catastrophic for
// performance.
func TestQueryPackets_SQLHasLimitOffset(t *testing.T) {
	if !strings.Contains(qPackets, "LIMIT $") {
		t.Errorf("packets SQL must contain LIMIT $\nGot:\n%s", qPackets)
	}
	if !strings.Contains(qPackets, "OFFSET $") {
		t.Errorf("packets SQL must contain OFFSET $\nGot:\n%s", qPackets)
	}
}

// SQL contract: the data query must filter by both src_ip and dst_ip,
// and the count query must mirror the same filter so totalCount matches
// the filtered result set (not the unfiltered hypertable).
func TestQueryPackets_SQLFiltersBySrcAndDstIP(t *testing.T) {
	if !strings.Contains(qPackets, "src_ip") {
		t.Errorf("packets SQL must reference src_ip\nGot:\n%s", qPackets)
	}
	if !strings.Contains(qPackets, "dst_ip") {
		t.Errorf("packets SQL must reference dst_ip\nGot:\n%s", qPackets)
	}
	if !strings.Contains(qPacketsCount, "src_ip") {
		t.Errorf("packets count SQL must reference src_ip\nGot:\n%s", qPacketsCount)
	}
	if !strings.Contains(qPacketsCount, "dst_ip") {
		t.Errorf("packets count SQL must reference dst_ip\nGot:\n%s", qPacketsCount)
	}
}
