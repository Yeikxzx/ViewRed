package storage

import (
	"context"
	"time"
)

// ReportHistory represents a persisted context for a generated report.
type ReportHistory struct {
	ID             int64     `json:"id"`
	GeneratedAt    time.Time `json:"generated_at"`
	Since          time.Time `json:"since"`
	Until          time.Time `json:"until"`
	Filename       string    `json:"filename"`
	ScenarioName   string    `json:"scenario_name"`
	VictimIP       string    `json:"victim_ip"`
	AttackerIP     string    `json:"attacker_ip"`
	ExpectedResult string    `json:"expected_result"`
	Notes          string    `json:"notes"`
}

// TrafficPoint represents a single data point in the traffic timeline.
type TrafficPoint struct {
	Time    time.Time `json:"time"`
	Packets int64     `json:"packets"`
	Bytes   int64     `json:"bytes"`
}

// TopConversation represents a top pair of communicating IPs and ports.
type TopConversation struct {
	SrcIP    string `json:"src_ip"`
	DstIP    string `json:"dst_ip"`
	DstPort  int    `json:"dst_port"`
	Protocol string `json:"protocol"`
	Bytes    int64  `json:"bytes"`
	Packets  int64  `json:"packets"`
}

func (s *Storage) SaveReportHistory(ctx context.Context, r ReportHistory) (int64, error) {
	if s.db == nil {
		return 0, errStorageNotInitialized
	}

	query := `
		INSERT INTO reports_history 
		(since, until, filename, scenario_name, victim_ip, attacker_ip, expected_result, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`
	var id int64
	err := s.db.QueryRowContext(ctx, query,
		r.Since, r.Until, r.Filename, r.ScenarioName, r.VictimIP, r.AttackerIP, r.ExpectedResult, r.Notes,
	).Scan(&id)
	return id, err
}

func (s *Storage) QueryTrafficTimeline(ctx context.Context, since, until time.Time) ([]TrafficPoint, error) {
	if s.db == nil {
		return nil, errStorageNotInitialized
	}

	query := `
		SELECT 
			date_trunc('minute', time) AS minute_time,
			COUNT(*)::bigint AS packets,
			COALESCE(SUM(length), 0)::bigint AS bytes
		FROM packets
		WHERE time >= $1 AND time <= $2
		GROUP BY minute_time
		ORDER BY minute_time ASC
	`
	rows, err := s.db.QueryContext(ctx, query, since, until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := make([]TrafficPoint, 0)
	for rows.Next() {
		var p TrafficPoint
		if err := rows.Scan(&p.Time, &p.Packets, &p.Bytes); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return points, nil
}

func (s *Storage) QueryTopConversations(ctx context.Context, since, until time.Time, limit int) ([]TopConversation, error) {
	if s.db == nil {
		return nil, errStorageNotInitialized
	}

	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT 
			host(src_ip) AS src_ip, 
			host(dst_ip) AS dst_ip, 
			dst_port, 
			protocol,
			SUM(length)::bigint AS bytes,
			COUNT(*)::bigint AS packets
		FROM packets
		WHERE time >= $1 AND time <= $2
		GROUP BY src_ip, dst_ip, dst_port, protocol
		ORDER BY bytes DESC
		LIMIT $3
	`
	rows, err := s.db.QueryContext(ctx, query, since, until, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	convs := make([]TopConversation, 0)
	for rows.Next() {
		var c TopConversation
		if err := rows.Scan(&c.SrcIP, &c.DstIP, &c.DstPort, &c.Protocol, &c.Bytes, &c.Packets); err != nil {
			return nil, err
		}
		convs = append(convs, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return convs, nil
}
