package health

import (
	"context"
	"errors"
	"testing"
)

func TestRegistryEmptyIsOK(t *testing.T) {
	r := New()
	report := r.Check(context.Background())
	if report.Status != StatusOK {
		t.Fatalf("empty registry should be OK, got %s", report.Status)
	}
	if len(report.Subsystems) != 0 {
		t.Fatalf("expected 0 subsystems, got %d", len(report.Subsystems))
	}
}

func TestRegistryAggregatesSubsystems(t *testing.T) {
	r := New()
	r.Add("db", func(ctx context.Context) error { return nil })
	r.Add("capture", func(ctx context.Context) error { return errors.New("down") })

	report := r.Check(context.Background())
	if report.Status != StatusDegraded {
		t.Fatalf("one down should be Degraded, got %s", report.Status)
	}
	if len(report.Subsystems) != 2 {
		t.Fatalf("expected 2 subsystems, got %d", len(report.Subsystems))
	}
}

func TestRegistryAllDown(t *testing.T) {
	r := New()
	r.Add("a", func(ctx context.Context) error { return errors.New("fail a") })
	r.Add("b", func(ctx context.Context) error { return errors.New("fail b") })

	report := r.Check(context.Background())
	if report.Status != StatusDown {
		t.Fatalf("all down should be Down, got %s", report.Status)
	}
}
