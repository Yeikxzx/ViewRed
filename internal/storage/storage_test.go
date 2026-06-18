package storage

import (
	"context"
	"testing"
	"time"
)

func TestNewValidatesDSN(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Empty DSN should fail at parse time.
	_, err := New(ctx, "")
	if err == nil {
		t.Fatal("expected error for empty DSN, got nil")
	}
}

func TestStorageExposesPool(t *testing.T) {
	// sqlmock doesn't provide a real pgxpool, so we test that Pool() returns
	// nil safely when the storage is in a failed state.
	// For a positive test, see TestStoragePing which uses New with a mocked DSN.
	s := &Storage{pool: nil}
	if s.Pool() != nil {
		t.Fatal("expected nil pool from uninitialized Storage")
	}
}

func TestStorageHealthyReflectsState(t *testing.T) {
	s := &Storage{}
	if s.Healthy() {
		t.Fatal("default Storage should be unhealthy")
	}
	s.healthy = true
	if !s.Healthy() {
		t.Fatal("Storage with healthy=true should be healthy")
	}
}
