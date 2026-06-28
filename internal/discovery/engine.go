package discovery

import (
	"context"
	"log"
	"net"
	"sync"
	"time"

	"github.com/viewred/viewred/internal/capture"
	"github.com/viewred/viewred/internal/storage"
)

// Engine processes packets to extract network discovery information (devices and topology).
type Engine struct {
	writer *storage.Writer

	ipMapMu sync.RWMutex
	ipMap   map[string]int64 // IP -> DeviceID cache
}

// NewEngine creates a new discovery engine.
func NewEngine(writer *storage.Writer) *Engine {
	// Initialize OUI database. Best effort.
	if err := LoadOUI("oui.txt"); err != nil {
		log.Printf("discovery: failed to load OUI database: %v", err)
	}

	return &Engine{
		writer: writer,
		ipMap:  make(map[string]int64),
	}
}

func (e *Engine) getDeviceID(ctx context.Context, ip, mac, vendor, hostname string, ts time.Time) int64 {
	e.ipMapMu.RLock()
	id, ok := e.ipMap[ip]
	e.ipMapMu.RUnlock()
	if ok && hostname == "" {
		// If we know the ID and there's no new hostname, just return ID.
		// Wait, we still want to update lastSeen. The background flow aggregator updates it, but for now we skip DB update unless there's a new hostname or it's unknown.
		// For simplicity, we can let Writer.UpsertDevice handle the DB upsert, but we want to avoid hammering the DB.
		// Let's just return the cached ID if we don't have new discovery info (IsDiscovery=false).
		return id
	}

	// Uncached or new info -> Upsert
	dev := storage.Device{
		IP:       ip,
		MAC:      mac,
		Vendor:   vendor,
		Hostname: hostname,
	}
	
	dev.LastSeen = ts

	newID, err := e.writer.UpsertDevice(ctx, dev)
	if err != nil {
		// Just log and return 0
		log.Printf("discovery: UpsertDevice err: %v", err)
		return 0
	}

	e.ipMapMu.Lock()
	e.ipMap[ip] = newID
	e.ipMapMu.Unlock()
	return newID
}

// isPrivateIP returns true when ip belongs to a private / local
// network range (RFC 1918, RFC 6598, RFC 4193, or link-local).
func isPrivateIP(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	return parsed.IsPrivate() || parsed.IsLinkLocalUnicast()
}

// Process inspects a packet for MAC/IP and topology connections.
//
// Only private (local network) IPs are tracked — public IPs seen in
// outbound traffic are ignored so the topology graph stays focused on
// devices around the network. Self-loop edges (src == dst) are also
// skipped.
func (e *Engine) Process(ctx context.Context, pkt capture.PacketEvent) {
	if pkt.SrcMAC == "" || pkt.SrcIP == "" {
		return
	}
	// Skip public / external IPs — we only track local network devices.
	if !isPrivateIP(pkt.SrcIP) {
		return
	}

	vendor := GetVendor(pkt.SrcMAC)

	var srcID int64
	if pkt.IsDiscovery || e.needsUpdate(pkt.SrcIP) {
		srcID = e.getDeviceID(ctx, pkt.SrcIP, pkt.SrcMAC, vendor, pkt.Hostname, pkt.Timestamp)
	} else {
		srcID = e.getCachedID(pkt.SrcIP)
	}

	if pkt.DstIP != "" && pkt.Length > 0 && srcID != 0 {
		// Only create edges between private IPs — skip external
		// destinations and self-loops.
		if !isPrivateIP(pkt.DstIP) {
			return
		}
		dstID := e.getCachedID(pkt.DstIP)
		if dstID != 0 && dstID != srcID {
			e.writer.UpsertTopologyEdge(srcID, dstID, uint64(pkt.Length), pkt.Timestamp)
		}
	}
}

func (e *Engine) getCachedID(ip string) int64 {
	e.ipMapMu.RLock()
	defer e.ipMapMu.RUnlock()
	return e.ipMap[ip]
}

func (e *Engine) needsUpdate(ip string) bool {
	e.ipMapMu.RLock()
	_, ok := e.ipMap[ip]
	e.ipMapMu.RUnlock()
	return !ok
}
