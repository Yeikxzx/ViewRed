package storage

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // pgx driver
	_ "github.com/golang-migrate/migrate/v4/source/file"     // file source
)

// RunMigrations applies all up-migrations in migrationsDir to the database
// referenced by the Storage. The migrations directory should contain files
// named "NNNN_name.up.sql" (e.g., "0001_initial.up.sql").
//
// RunMigrations is idempotent: it tracks which migrations have been applied
// in a "schema_migrations" table created by golang-migrate.
//
// It returns a non-nil error if any of the following are true:
//   - s is nil or s.pool is nil
//   - migrationsDir is empty
//   - the pool's connection string is empty
//   - migrate.New fails to bind to the source/database
//   - m.Up fails for any reason other than migrate.ErrNoChange (no-op)
func RunMigrations(s *Storage, migrationsDir string) error {
	if s == nil || s.pool == nil {
		return errors.New("storage: nil storage or pool")
	}
	if migrationsDir == "" {
		return errors.New("storage: empty migrations dir")
	}

	// golang-migrate wants a connection string for the pgx driver.
	// We extract it from the pool's config; if not available, error.
	cfg := s.pool.Config()
	if cfg.ConnString() == "" {
		return errors.New("storage: empty connection string")
	}

	// file:// source URL format
	srcURL := "file://" + migrationsDir

	m, err := migrate.New(srcURL, cfg.ConnString())
	if err != nil {
		return fmt.Errorf("storage: init migrate: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("storage: run migrations: %w", err)
	}

	return nil
}
