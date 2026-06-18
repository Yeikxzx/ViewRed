// Package storage manages the PostgreSQL+TimescaleDB persistence layer.
package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Storage owns a pgx connection pool and tracks its health.
type Storage struct {
	pool    *pgxpool.Pool
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

	return &Storage{pool: pool, healthy: true}, nil
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
}

// Healthy returns the last-known health state.
func (s *Storage) Healthy() bool { return s.healthy }
