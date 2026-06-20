package storage

import (
	"context"
)

// GetVulnerabilities retrieves vulnerabilities.
func (s *Storage) GetVulnerabilities(ctx context.Context, includeMitigated bool) ([]Vulnerability, error) {
	query := `
		SELECT id, device_ip, cve_id, cvss_score, description, port, mitigated, discovered_at
		FROM vulnerabilities
	`
	if !includeMitigated {
		query += " WHERE mitigated = false"
	}
	query += " ORDER BY id DESC"

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vulns []Vulnerability
	for rows.Next() {
		var v Vulnerability
		if err := rows.Scan(
			&v.ID, &v.DeviceIP, &v.CVEID, &v.CVSSScore, &v.Description,
			&v.Port, &v.Mitigated, &v.DiscoveredAt,
		); err != nil {
			return nil, err
		}
		vulns = append(vulns, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return vulns, nil
}

// SaveVulnerability inserts a new vulnerability.
func (s *Storage) SaveVulnerability(ctx context.Context, v Vulnerability) (int64, error) {
	query := `
		INSERT INTO vulnerabilities (device_ip, cve_id, cvss_score, description, port)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`
	var id int64
	err := s.pool.QueryRow(ctx, query, v.DeviceIP, v.CVEID, v.CVSSScore, v.Description, v.Port).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// MitigateVulnerability marks a vulnerability as mitigated.
func (s *Storage) MitigateVulnerability(ctx context.Context, id int64) error {
	query := `UPDATE vulnerabilities SET mitigated = true WHERE id = $1`
	_, err := s.pool.Exec(ctx, query, id)
	return err
}
