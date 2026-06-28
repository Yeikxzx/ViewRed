package storage

import (
	"context"
)

// SaveScanHistory inserts a scan history record and returns its ID.
func (s *Storage) SaveScanHistory(ctx context.Context, h ScanHistory) (int64, error) {
	if s.db == nil {
		return 0, errStorageNotInitialized
	}

	var id int64
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO scan_history (target, profile, duration_ms, hosts_scanned, vulns_found, status)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id`,
		h.Target, h.Profile, h.DurationMs, h.HostsScanned, h.VulnsFound, h.Status,
	).Scan(&id)
	return id, err
}

// GetScanHistory returns the most recent scan history records, ordered by
// started_at descending.
func (s *Storage) GetScanHistory(ctx context.Context, limit int) ([]ScanHistory, error) {
	if s.db == nil {
		return nil, errStorageNotInitialized
	}

	if limit <= 0 || limit > 100 {
		limit = 20
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, target, profile, duration_ms, hosts_scanned, vulns_found, status, started_at
		 FROM scan_history
		 ORDER BY started_at DESC
		 LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []ScanHistory
	for rows.Next() {
		var h ScanHistory
		if err := rows.Scan(&h.ID, &h.Target, &h.Profile, &h.DurationMs, &h.HostsScanned, &h.VulnsFound, &h.Status, &h.StartedAt); err != nil {
			return nil, err
		}
		history = append(history, h)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if history == nil {
		history = []ScanHistory{}
	}
	return history, nil
}
