package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viewred/viewred/internal/capture"
)

// Compile-time interface assertions double as runtime checks when go test
// builds the test binary. If a future change makes MockQuerier or
// ErrorQuerier stop satisfying Querier, the test binary fails to compile
// — which is the whole point of the assertion.
func TestMockQuerier_ImplementsQuerier(t *testing.T) {
	var _ Querier = (*MockQuerier)(nil)
	var _ Querier = (*ErrorQuerier)(nil)
}

// ErrorQuerier must surface the same error from every Querier method so
// handlers can rely on a single error path. We use errors.Is so wrapped
// errors from future refactors still match.
func TestErrorQuerier_ReturnsErrOnAllMethods(t *testing.T) {
	sentinel := errors.New("db down")
	q := &ErrorQuerier{Err: sentinel}
	ctx := context.Background()
	since := time.Now()

	if _, err := q.QueryDashboardSummary(ctx, since); !errors.Is(err, sentinel) {
		t.Errorf("QueryDashboardSummary: expected %v, got %v", sentinel, err)
	}
	if _, err := q.QueryTopTalkers(ctx, since, 10); !errors.Is(err, sentinel) {
		t.Errorf("QueryTopTalkers: expected %v, got %v", sentinel, err)
	}
	if _, err := q.QueryProtocolDistribution(ctx, since); !errors.Is(err, sentinel) {
		t.Errorf("QueryProtocolDistribution: expected %v, got %v", sentinel, err)
	}
	if _, _, err := q.QueryPackets(ctx, PacketFilter{}); !errors.Is(err, sentinel) {
		t.Errorf("QueryPackets: expected %v, got %v", sentinel, err)
	}
	if _, _, err := q.QueryDevices(ctx, DeviceFilter{}); !errors.Is(err, sentinel) {
		t.Errorf("QueryDevices: expected %v, got %v", sentinel, err)
	}
}

// ErrorQuerier with a nil Err must still return a non-nil error so
// handlers never silently swallow DB-down conditions.
func TestErrorQuerier_NilErrStillReturnsError(t *testing.T) {
	q := &ErrorQuerier{}
	ctx := context.Background()
	since := time.Now()

	if _, err := q.QueryDashboardSummary(ctx, since); err == nil {
		t.Error("expected non-nil error from ErrorQuerier with nil Err, got nil")
	}
}

// A zero-value MockQuerier (no function fields set) must return zero
// values for all result types without panicking. This is the contract
// that lets tests use &MockQuerier{} as a no-op stand-in.
func TestMockQuerier_DefaultsToZeroValues(t *testing.T) {
	q := &MockQuerier{}
	ctx := context.Background()
	since := time.Now()

	t.Run("QueryDashboardSummary", func(t *testing.T) {
		summary, err := q.QueryDashboardSummary(ctx, since)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if summary.PacketsPerSec != 0 || summary.BytesPerSec != 0 {
			t.Errorf("expected zero summary, got %+v", summary)
		}
		if summary.ActiveConnections != 0 || summary.DeviceCount != 0 {
			t.Errorf("expected zero counts, got %+v", summary)
		}
	})

	t.Run("QueryTopTalkers", func(t *testing.T) {
		got, err := q.QueryTopTalkers(ctx, since, 10)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if got != nil {
			t.Errorf("expected nil top talkers, got %v", got)
		}
	})

	t.Run("QueryProtocolDistribution", func(t *testing.T) {
		got, err := q.QueryProtocolDistribution(ctx, since)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if got != nil {
			t.Errorf("expected nil protocol shares, got %v", got)
		}
	})

	t.Run("QueryPackets", func(t *testing.T) {
		packets, total, err := q.QueryPackets(ctx, PacketFilter{})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if packets != nil {
			t.Errorf("expected nil packets, got %v", packets)
		}
		if total != 0 {
			t.Errorf("expected zero total, got %d", total)
		}
	})

	t.Run("QueryDevices", func(t *testing.T) {
		devices, total, err := q.QueryDevices(ctx, DeviceFilter{})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if devices != nil {
			t.Errorf("expected nil devices, got %v", devices)
		}
		if total != 0 {
			t.Errorf("expected zero total, got %d", total)
		}
	})
}

// When a function field is set, MockQuerier must delegate the call to it
// and propagate the (value, error) pair verbatim.
func TestMockQuerier_DelegatesToFunctionFields(t *testing.T) {
	t.Run("QueryDashboardSummary", func(t *testing.T) {
		expected := DashboardSummary{PacketsPerSec: 42.0, BytesPerSec: 100, ActiveConnections: 5, DeviceCount: 3}
		q := &MockQuerier{
			DashboardSummaryFn: func(ctx context.Context, since time.Time) (DashboardSummary, error) {
				return expected, nil
			},
		}
		got, err := q.QueryDashboardSummary(context.Background(), time.Now())
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if got != expected {
			t.Errorf("expected %+v, got %+v", expected, got)
		}
	})

	t.Run("QueryTopTalkers", func(t *testing.T) {
		expected := []TopTalker{{IP: "10.0.0.1", Bytes: 1024}, {IP: "10.0.0.2", Bytes: 2048}}
		q := &MockQuerier{
			TopTalkersFn: func(ctx context.Context, since time.Time, limit int) ([]TopTalker, error) {
				return expected, nil
			},
		}
		got, err := q.QueryTopTalkers(context.Background(), time.Now(), 5)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("expected 2 talkers, got %d", len(got))
		}
	})

	t.Run("QueryProtocolDistribution", func(t *testing.T) {
		expected := []ProtocolShare{{Protocol: "TCP", Percent: 70.0}, {Protocol: "UDP", Percent: 30.0}}
		q := &MockQuerier{
			ProtocolDistFn: func(ctx context.Context, since time.Time) ([]ProtocolShare, error) {
				return expected, nil
			},
		}
		got, err := q.QueryProtocolDistribution(context.Background(), time.Now())
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("expected 2 shares, got %d", len(got))
		}
	})

	t.Run("QueryPackets", func(t *testing.T) {
		sentinel := errors.New("query fail")
		q := &MockQuerier{
			PacketsFn: func(ctx context.Context, filter PacketFilter) ([]capture.PacketEvent, int, error) {
				return nil, 0, sentinel
			},
		}
		_, _, err := q.QueryPackets(context.Background(), PacketFilter{})
		if !errors.Is(err, sentinel) {
			t.Errorf("expected %v, got %v", sentinel, err)
		}
	})

	t.Run("QueryDevices", func(t *testing.T) {
		sentinel := errors.New("device fail")
		q := &MockQuerier{
			DevicesFn: func(ctx context.Context, filter DeviceFilter) ([]Device, int, error) {
				return nil, 0, sentinel
			},
		}
		_, _, err := q.QueryDevices(context.Background(), DeviceFilter{})
		if !errors.Is(err, sentinel) {
			t.Errorf("expected %v, got %v", sentinel, err)
		}
	})
}
