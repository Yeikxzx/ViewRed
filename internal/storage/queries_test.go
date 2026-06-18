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
