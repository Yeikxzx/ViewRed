// Package storage manages the PostgreSQL+TimescaleDB persistence layer.
package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

// Storage owns a pgx connection pool and tracks its health.
//
// The pool is the write path (used by the Writer for batched inserts)
// and the source of *sql.DB for the read path: queries in queries.go
// use the database/sql interface so sqlmock can mock them in unit
// tests, while production reads go through stdlib.OpenDBFromPool which
// shares the same underlying connections as the pool.
type Storage struct {
	pool    *pgxpool.Pool
	db      *sql.DB
	healthy bool
}

// New parses the DSN and establishes a connection pool. It pings the database
// to verify connectivity before returning.
func New(ctx context.Context, dsn string) (*Storage, error) {
	if dsn == "" {
		return nil, errors.New("storage: empty DSN")
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("storage: parse DSN: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("storage: connect: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("storage: ping: %w", err)
	}

	return &Storage{pool: pool, db: stdlib.OpenDBFromPool(pool), healthy: true}, nil
}

// Pool returns the underlying pgx pool. May be nil if New failed.
func (s *Storage) Pool() *pgxpool.Pool { return s.pool }

// Ping verifies the database is reachable.
func (s *Storage) Ping(ctx context.Context) error {
	if s.pool == nil {
		return errors.New("storage: pool not initialized")
	}
	return s.pool.Ping(ctx)
}

// Close releases the connection pool.
func (s *Storage) Close() {
	if s.pool != nil {
		s.pool.Close()
		s.healthy = false
	}
	if s.db != nil {
		// OpenDBFromPool docs note that closing the *sql.DB does NOT
		// close the underlying *pgxpool.Pool, so Close on the pool
		// above is the canonical shutdown. We still close the *sql.DB
		// to release any driver-internal state.
		_ = s.db.Close()
	}
}

// Healthy returns the last-known health state.
func (s *Storage) Healthy() bool { return s.healthy }
