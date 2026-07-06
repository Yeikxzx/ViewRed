// Package ml provides ML-based and heuristic network anomaly detection.
//
// The BruteForceDetector counts TCP SYN attempts per (SrcIP, DstPort) for
// SSH and Telnet services over a sliding window, emitting suspicion alerts
// based purely on connection behavior. It does not prove authentication
// failure.
package ml

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/viewred/viewred/internal/capture"
	"github.com/viewred/viewred/internal/storage"
)

// Brute-force thresholds and window configuration.
const (
	bruteForceThreshold         = 10
	bruteForceCriticalThreshold = 50
	bruteForceWindowDuration    = 60 * time.Second
	bruteForceDedupTTL          = 60 * time.Second
)

// targetBruteForcePorts contains the services the detector watches.
var targetBruteForcePorts = map[uint16]bool{22: true, 23: true}

// bruteForceKey identifies a source/destination/service tuple for tracking and dedup.
type bruteForceKey struct {
	SrcIP   string
	DstIP   string
	DstPort uint16
}

// bruteForceWindow holds per-key SYN attempts within the detection window.
type bruteForceWindow struct {
	attempts  []time.Time
	lastSeen  time.Time
	firstSeen time.Time
}

// dedupEntry tracks when an alert was last emitted and at what severity so
// that escalation (e.g. HIGH -> CRITICAL) is not suppressed by dedup.
type dedupEntry struct {
	lastEmitted time.Time
	severity    string
}

// BruteForceDetector applies a behavioral heuristic to detect SSH/Telnet
// brute-force attempts by counting SYNs per (SrcIP, DstIP, DstPort) over time.
// Safe for concurrent use by multiple evaluation workers.
type BruteForceDetector struct {
	mu      sync.Mutex
	windows map[bruteForceKey]*bruteForceWindow
	dedup   map[bruteForceKey]dedupEntry
}

// NewBruteForceDetector creates a new BruteForceDetector.
func NewBruteForceDetector() *BruteForceDetector {
	return &BruteForceDetector{
		windows: make(map[bruteForceKey]*bruteForceWindow),
		dedup:   make(map[bruteForceKey]dedupEntry),
	}
}

// severityRank returns a numeric rank for severity comparison. Higher means
// more severe. Unknown severities are treated as lowest rank.
func severityRank(severity string) int {
	switch severity {
	case "CRITICAL":
		return 2
	case "HIGH":
		return 1
	default:
		return 0
	}
}

// Record adds a TCP SYN packet to the per-(SrcIP, DstPort) sliding window.
// Only TCP SYN packets destined for ports 22 or 23 are tracked.
func (d *BruteForceDetector) Record(pkt capture.PacketEvent) {
	if pkt.Protocol != "TCP" || !pkt.TCPSyn || pkt.TCPAck {
		return
	}
	if !targetBruteForcePorts[pkt.DstPort] {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	now := pkt.Timestamp
	if now.IsZero() {
		now = time.Now()
	}
	cutoff := now.Add(-bruteForceWindowDuration)

	// Garbage-collect stale windows.
	for k, w := range d.windows {
		if w.lastSeen.Before(cutoff) {
			delete(d.windows, k)
		}
	}

	key := bruteForceKey{SrcIP: pkt.SrcIP, DstIP: pkt.DstIP, DstPort: pkt.DstPort}
	w, ok := d.windows[key]
	if !ok {
		w = &bruteForceWindow{firstSeen: now}
		d.windows[key] = w
	}

	// Drop attempts that fell out of the sliding window.
	fresh := w.attempts[:0]
	for _, ts := range w.attempts {
		if !ts.Before(cutoff) {
			fresh = append(fresh, ts)
		}
	}
	w.attempts = fresh

	w.attempts = append(w.attempts, now)
	w.lastSeen = now
}

// Evaluate checks whether the per-(SrcIP, DstPort) count crosses the brute-force
// threshold and returns an alert when it does (subject to dedup). It returns nil
// when the traffic is below threshold or suppressed.
func (d *BruteForceDetector) Evaluate(pkt capture.PacketEvent) *storage.Alert {
	if pkt.Protocol != "TCP" || !pkt.TCPSyn || pkt.TCPAck {
		return nil
	}
	if !targetBruteForcePorts[pkt.DstPort] {
		return nil
	}

	key := bruteForceKey{SrcIP: pkt.SrcIP, DstIP: pkt.DstIP, DstPort: pkt.DstPort}

	d.mu.Lock()
	defer d.mu.Unlock()

	w, ok := d.windows[key]
	if !ok || len(w.attempts) < bruteForceThreshold {
		return nil
	}

	count := len(w.attempts)
	severity := "HIGH"
	score := float32(80.0)
	if count >= bruteForceCriticalThreshold {
		severity = "CRITICAL"
		score = float32(90.0)
	}

	// Dedup: one alert per (SrcIP, DstIP, DstPort) every bruteForceDedupTTL, unless
	// the new alert represents a severity escalation over the last emitted one.
	now := time.Now()
	if entry, seen := d.dedup[key]; seen && now.Sub(entry.lastEmitted) < bruteForceDedupTTL {
		if severityRank(severity) <= severityRank(entry.severity) {
			return nil
		}
	}
	d.dedup[key] = dedupEntry{lastEmitted: now, severity: severity}
	if len(d.dedup) > 1000 {
		for k, v := range d.dedup {
			if now.Sub(v.lastEmitted) > bruteForceDedupTTL {
				delete(d.dedup, k)
			}
		}
	}

	service := serviceNameForPort(pkt.DstPort)
	title := fmt.Sprintf("Suspicious %s Brute Force", service)
	desc := fmt.Sprintf(
		"Repeated %s SYN attempts from %s to %s:%d (%d attempts in %d seconds). "+
			"This is behavioral suspicion only; authentication failure is not proven.",
		service, pkt.SrcIP, pkt.DstIP, pkt.DstPort, count, int(bruteForceWindowDuration.Seconds()),
	)

	ctx := map[string]interface{}{
		"origin":               "bruteforce_detector",
		"method":               "heuristic",
		"service":              service,
		"src_ip":               pkt.SrcIP,
		"dst_ip":               pkt.DstIP,
		"dst_port":             pkt.DstPort,
		"attempt_count":        count,
		"window_seconds":       int(bruteForceWindowDuration.Seconds()),
		"threshold":            bruteForceThreshold,
		"critical_threshold":   bruteForceCriticalThreshold,
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

// serviceNameForPort maps a destination port to its service name for alert text.
func serviceNameForPort(port uint16) string {
	switch port {
	case 22:
		return "SSH"
	case 23:
		return "Telnet"
	default:
		return fmt.Sprintf("port-%d", port)
	}
}
