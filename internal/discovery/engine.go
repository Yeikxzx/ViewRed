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
	macMap  map[string]int64 // MAC -> DeviceID cache for gateway resolution
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
		macMap: make(map[string]int64),
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
	if isPrivateIP(ip) && mac != "" {
		e.macMap[mac] = newID
	}
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
// All IPs are tracked as nodes (both local and external) so the user
// has full visibility of every device their network contacts. Edges
// are only created when at least one endpoint is a local IP, to avoid
// saturating the topology graph with external↔external connections.
// Self-loop edges (src == dst) are also skipped.
func (e *Engine) Process(ctx context.Context, pkt capture.PacketEvent) {
	if pkt.SrcMAC == "" || pkt.SrcIP == "" {
		return
	}

	srcMAC := pkt.SrcMAC
	srcVendor := GetVendor(srcMAC)
	// External IPs do not have a meaningful local MAC (it's just the gateway's MAC).
	if !isPrivateIP(pkt.SrcIP) {
		srcMAC = ""
		srcVendor = ""
	}

	var srcID int64
	if pkt.IsDiscovery || e.needsUpdate(pkt.SrcIP) {
		srcID = e.getDeviceID(ctx, pkt.SrcIP, srcMAC, srcVendor, pkt.Hostname, pkt.Timestamp)
	} else {
		srcID = e.getCachedID(pkt.SrcIP)
	}

	if pkt.DstIP != "" && pkt.Length > 0 && srcID != 0 {
		// Skip edges between two external IPs — keep the graph
		// focused on what your local network is talking to.
		if !isPrivateIP(pkt.SrcIP) && !isPrivateIP(pkt.DstIP) {
			return
		}

		var dstID int64
		if e.needsUpdate(pkt.DstIP) {
			dstMAC := pkt.DstMAC
			dstVendor := GetVendor(dstMAC)
			if !isPrivateIP(pkt.DstIP) {
				dstMAC = ""
				dstVendor = ""
			}
			dstID = e.getDeviceID(ctx, pkt.DstIP, dstMAC, dstVendor, "", pkt.Timestamp)
		} else {
			dstID = e.getCachedID(pkt.DstIP)
		}

		if dstID != 0 && dstID != srcID {
			// If traffic is going from Local to External, route it through the gateway (DstMAC)
			if isPrivateIP(pkt.SrcIP) && !isPrivateIP(pkt.DstIP) && pkt.DstMAC != "" {
				e.ipMapMu.RLock()
				gwID, ok := e.macMap[pkt.DstMAC]
				e.ipMapMu.RUnlock()
				
				if ok && gwID != srcID && gwID != dstID {
					e.writer.UpsertTopologyEdge(srcID, gwID, uint64(pkt.Length), pkt.Timestamp)
					e.writer.UpsertTopologyEdge(gwID, dstID, uint64(pkt.Length), pkt.Timestamp)
					return
				}
			}

			// If traffic is coming from External to Local, route it through the gateway (SrcMAC)
			if !isPrivateIP(pkt.SrcIP) && isPrivateIP(pkt.DstIP) && pkt.SrcMAC != "" {
				e.ipMapMu.RLock()
				gwID, ok := e.macMap[pkt.SrcMAC]
				e.ipMapMu.RUnlock()
				
				if ok && gwID != srcID && gwID != dstID {
					e.writer.UpsertTopologyEdge(srcID, gwID, uint64(pkt.Length), pkt.Timestamp)
					e.writer.UpsertTopologyEdge(gwID, dstID, uint64(pkt.Length), pkt.Timestamp)
					return
				}
			}

			// Normal direct edge
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
