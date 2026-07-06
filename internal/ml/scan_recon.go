// Package ml provides ML-based and heuristic network anomaly detection.
//
// The ScanReconDetector tracks unique TCP SYN destination ports per
// (SrcIP, DstIP) over a sliding window to detect port-scan / reconnaissance
// behavior. It is intentionally minimal: TCP SYN only, no payload inspection,
// no ML inference, and no configuration surface.
package ml

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/viewred/viewred/internal/capture"
	"github.com/viewred/viewred/internal/storage"
)

// Scan/recon detection thresholds and window configuration.
const (
	scanReconWindowDuration = 10 * time.Second
	scanReconDedupTTL       = 60 * time.Second
	scanReconMinPackets     = 8

	scanReconWarningThreshold  = 8
	scanReconHighThreshold     = 50
	scanReconCriticalThreshold = 200
)

// scanKey identifies a source/destination pair for tracking and dedup.
type scanKey struct {
	SrcIP string
	DstIP string
}

// scanWindow holds per-(SrcIP, DstIP) TCP SYN state within the detection window.
type scanWindow struct {
	ports     map[uint16]bool
	synCount  int
	firstSeen time.Time
	lastSeen  time.Time
}

// ScanReconDetector applies a behavioral heuristic to detect TCP SYN port scans
// by counting unique destination ports per (SrcIP, DstIP) over time.
// Safe for concurrent use by multiple evaluation workers.
type ScanReconDetector struct {
	mu      sync.Mutex
	windows map[scanKey]*scanWindow
	dedup   map[scanKey]dedupEntry
}

// NewScanReconDetector creates a new ScanReconDetector.
func NewScanReconDetector() *ScanReconDetector {
	return &ScanReconDetector{
		windows: make(map[scanKey]*scanWindow),
		dedup:   make(map[scanKey]dedupEntry),
	}
}

// Record adds a TCP SYN packet to the per-(SrcIP, DstIP) sliding window.
// Only TCP packets with TCPSyn=true are tracked.
func (d *ScanReconDetector) Record(pkt capture.PacketEvent) {
	if pkt.Protocol != "TCP" || !pkt.TCPSyn || pkt.TCPAck {
		return
	}
	if isLikelyClientBoundServiceResponse(pkt) {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	now := pkt.Timestamp
	if now.IsZero() {
		now = time.Now()
	}
	cutoff := now.Add(-scanReconWindowDuration)

	// Garbage-collect stale windows.
	for k, w := range d.windows {
		if w.lastSeen.Before(cutoff) {
			delete(d.windows, k)
		}
	}

	key := scanKey{SrcIP: pkt.SrcIP, DstIP: pkt.DstIP}
	w, ok := d.windows[key]
	if !ok {
		w = &scanWindow{
			ports:     make(map[uint16]bool),
			firstSeen: now,
		}
		d.windows[key] = w
	}

	// If the current window expired, start a fresh one.
	if now.Sub(w.firstSeen) > scanReconWindowDuration {
		w.ports = make(map[uint16]bool)
		w.synCount = 0
		w.firstSeen = now
	}

	w.ports[pkt.DstPort] = true
	w.synCount++
	w.lastSeen = now
}

// Evaluate checks whether the per-(SrcIP, DstIP) unique destination-port count
// crosses a scan/recon threshold and returns an alert when it does (subject to
// dedup). It returns nil when the traffic is below threshold or suppressed.
func (d *ScanReconDetector) Evaluate(pkt capture.PacketEvent) *storage.Alert {
	if pkt.Protocol != "TCP" || !pkt.TCPSyn || pkt.TCPAck {
		return nil
	}
	if isLikelyClientBoundServiceResponse(pkt) {
		return nil
	}

	key := scanKey{SrcIP: pkt.SrcIP, DstIP: pkt.DstIP}

	d.mu.Lock()
	defer d.mu.Unlock()

	w, ok := d.windows[key]
	if !ok || w.synCount < scanReconMinPackets {
		return nil
	}

	uniquePorts := len(w.ports)
	if uniquePorts < scanReconWarningThreshold {
		return nil
	}

	severity := "WARNING"
	score := float32(70.0)
	if uniquePorts >= scanReconCriticalThreshold {
		severity = "CRITICAL"
		score = float32(95.0)
	} else if uniquePorts >= scanReconHighThreshold {
		severity = "HIGH"
		score = float32(85.0)
	}

	// Dedup: one alert per (SrcIP, DstIP) every scanReconDedupTTL, unless
	// the new alert represents a severity escalation over the last emitted one.
	now := time.Now()
	if entry, seen := d.dedup[key]; seen && now.Sub(entry.lastEmitted) < scanReconDedupTTL {
		if severityRank(severity) <= severityRank(entry.severity) {
			return nil
		}
	}
	d.dedup[key] = dedupEntry{lastEmitted: now, severity: severity}
	if len(d.dedup) > 1000 {
		for k, v := range d.dedup {
			if now.Sub(v.lastEmitted) > scanReconDedupTTL {
				delete(d.dedup, k)
			}
		}
	}

	windowSeconds := int(w.lastSeen.Sub(w.firstSeen).Seconds())
	if windowSeconds < 1 {
		windowSeconds = 1
	}

	dstPorts := make([]uint16, 0, len(w.ports))
	for port := range w.ports {
		dstPorts = append(dstPorts, port)
	}
	sort.Slice(dstPorts, func(i, j int) bool { return dstPorts[i] < dstPorts[j] })

	title := "Suspicious TCP SYN Scan/Recon"
	desc := fmt.Sprintf(
		"TCP SYN scan detected from %s to %s: %d unique destination ports seen in %d seconds.",
		pkt.SrcIP, pkt.DstIP, uniquePorts, windowSeconds,
	)

	ctx := map[string]interface{}{
		"origin":               "scan_recon_detector",
		"method":               "heuristic",
		"src_ip":               pkt.SrcIP,
		"dst_ip":               pkt.DstIP,
		"dst_ports":            dstPorts,
		"port_count":           uniquePorts,
		"syn_count":            w.synCount,
		"window_seconds":       windowSeconds,
		"severity_decision":    severity,
		"behavioral_suspicion": true,
	}
	ctxJSON, _ := json.Marshal(ctx)

	return &storage.Alert{
		Time:        time.Now().UTC(),
		Severity:    severity,
		Title:       title,
		Description: &desc,
		Source:      fmt.Sprintf("%s:%d", pkt.SrcIP, pkt.SrcPort),
		Score:       score,
		Context:     ctxJSON,
	}
}
