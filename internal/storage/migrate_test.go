package storage

import "testing"

// TestRunMigrationsNilStorage verifies that RunMigrations rejects a nil
// *Storage with a non-nil error. This is the first defensive check the
// function performs before touching the pool or the filesystem.
func TestRunMigrationsNilStorage(t *testing.T) {
	dir := t.TempDir()
	err := RunMigrations(nil, dir)
	if err == nil {
		t.Fatal("expected error for nil storage, got nil")
	}
}

// TestRunMigrationsNilPool verifies that RunMigrations rejects a *Storage
// whose pool has not been initialized. This guards against callers that
// constructed a Storage literal directly (bypassing New) and then tried
// to run migrations.
func TestRunMigrationsNilPool(t *testing.T) {
	s := &Storage{pool: nil}
	err := RunMigrations(s, t.TempDir())
	if err == nil {
		t.Fatal("expected error for nil pool, got nil")
	}
}

// TestRunMigrationsEmptyDir is a placeholder for the empty-dir check that
// fires after the nil-storage/nil-pool check. It cannot be reached without
// a real pgxpool.Pool (NewWithConfig establishes a real connection), and
// we deliberately avoid using the real database in unit tests. The real
// coverage lives in the e2e path (Task 9) which runs against a live
// TimescaleDB.
func TestRunMigrationsEmptyDir(t *testing.T) {
	t.Skip("integration test — requires real pgxpool.Pool or fake; covered in Task 9 (e2e)")
}
