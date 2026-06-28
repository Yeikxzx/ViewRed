package storage

import (
	"context"
	"net"
)

// QueryTopology fetches all devices and the most recent topology edges.
func (s *Storage) QueryTopology(ctx context.Context) ([]Device, []TopologyEdge, error) {
	if s.db == nil {
		return nil, nil, errStorageNotInitialized
	}

	// 1. Fetch all known devices
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, ip::text, mac, vendor, hostname, first_seen, last_seen, risk_score
		 FROM devices
		 ORDER BY id ASC`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var devices []Device
	for rows.Next() {
		var d Device
		// ip is inet in postgres, pgx handles scanning into string if we cast or let sql driver do it, but database/sql can scan it directly into string
		if err := rows.Scan(&d.ID, &d.IP, &d.MAC, &d.Vendor, &d.Hostname, &d.FirstSeen, &d.LastSeen, &d.RiskScore); err != nil {
			return nil, nil, err
		}
		devices = append(devices, d)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	// Filter out public / external IPs so the topology only shows
	// local network devices. The discovery engine now skips public
	// IPs too, but this query-side filter cleans up any that were
	// inserted before the engine fix.
	filtered := devices[:0]
	for _, d := range devices {
		ip, _, err := net.ParseCIDR(d.IP)
		if err == nil && (ip.IsPrivate() || ip.IsLinkLocalUnicast()) {
			filtered = append(filtered, d)
		}
	}
	devices = filtered

	// 2. Fetch edges
	edgeRows, err := s.db.QueryContext(ctx,
		`SELECT id, src_device_id, dst_device_id, bytes_count, last_seen
		 FROM topology_edges
		 ORDER BY bytes_count DESC`)
	if err != nil {
		return nil, nil, err
	}
	defer edgeRows.Close()

	var edges []TopologyEdge
	for edgeRows.Next() {
		var e TopologyEdge
		if err := edgeRows.Scan(&e.ID, &e.SrcDeviceID, &e.DstDeviceID, &e.BytesCount, &e.LastSeen); err != nil {
			return nil, nil, err
		}
		edges = append(edges, e)
	}
	if err := edgeRows.Err(); err != nil {
		return nil, nil, err
	}

	// Filter out edges whose src or dst device was removed by the
	// private-IP filter above, plus any self-loop edges.
	validIDs := make(map[int64]struct{}, len(devices))
	for _, d := range devices {
		validIDs[d.ID] = struct{}{}
	}
	filteredEdges := edges[:0]
	for _, e := range edges {
		_, srcOK := validIDs[e.SrcDeviceID]
		_, dstOK := validIDs[e.DstDeviceID]
		if srcOK && dstOK && e.SrcDeviceID != e.DstDeviceID {
			filteredEdges = append(filteredEdges, e)
		}
	}
	edges = filteredEdges

	return devices, edges, nil
}
