// Package ml provides ML-based and heuristic network anomaly detection.
//
// The FloodDetector uses simple statistical thresholds to detect flood-style
// attacks (DoS/DDoS) without relying on ONNX model inference. It runs
// alongside the ML engine and the rules engine in the evaluation pipeline.
package ml

import (
	"fmt"
	"sync"
	"time"

	"github.com/viewred/viewred/internal/capture"
	"github.com/viewred/viewred/internal/storage"
)

// Flood thresholds — tweak based on your network baseline.
const (
	floodPPSThreshold         = 1000     // packets/sec from a single source IP
	floodBPSThreshold         = 25 << 20 // 25 MB/s from a single source IP
	floodUniquePortsThreshold = 30       // distinct dst ports from one source IP in window
	floodWindowDuration       = 30 * time.Second
	floodMinRateDuration      = 3 * time.Second
	floodDedupTTL             = 60 * time.Second
)

// srcWindow holds per-IP counters within the detection window.
type srcWindow struct {
	packets        int
	bytes          int64
	dstPorts       map[uint16]struct{} // all destination ports, retained for state visibility
	nonSynDstPorts map[uint16]struct{} // unique-port flood signal (excludes TCP SYN scan traffic)
	lastSeen       time.Time
	firstSeen      time.Time
}

// FloodDetector applies statistical heuristics to identify flood attacks.
// Safe for concurrent use by multiple evaluation workers.
type FloodDetector struct {
	mu      sync.Mutex
	windows map[string]*srcWindow
	dedup   map[string]time.Time
}

// NewFloodDetector creates a new FloodDetector.
func NewFloodDetector() *FloodDetector {
	return &FloodDetector{
		windows: make(map[string]*srcWindow),
		dedup:   make(map[string]time.Time),
	}
}

// Record adds a packet to the source IP's sliding window.
func (fd *FloodDetector) Record(pkt capture.PacketEvent) {
	if isLikelyClientBoundServiceResponse(pkt) {
		return
	}

	fd.mu.Lock()
	defer fd.mu.Unlock()

	now := pkt.Timestamp
	cutoff := now.Add(-floodWindowDuration)

	// Garbage-collect stale entries
	for ip, w := range fd.windows {
		if w.lastSeen.Before(cutoff) {
			delete(fd.windows, ip)
		}
	}

	w, ok := fd.windows[pkt.SrcIP]
	if !ok {
		w = &srcWindow{
			dstPorts:       make(map[uint16]struct{}),
			nonSynDstPorts: make(map[uint16]struct{}),
			firstSeen:      now,
		}
		fd.windows[pkt.SrcIP] = w
	}

	// If the window expired, reset
	if now.Sub(w.firstSeen) > floodWindowDuration {
		w.packets = 0
		w.bytes = 0
		w.dstPorts = make(map[uint16]struct{})
		w.nonSynDstPorts = make(map[uint16]struct{})
		w.firstSeen = now
	}

	w.packets++
	w.bytes += int64(pkt.Length)
	w.dstPorts[pkt.DstPort] = struct{}{}
	// Exclude TCP SYN packets and DNS server responses (source port 53) from
	// the unique-port flood signal; scan/recon detection owns TCP SYN
	// port-fanout classification, and DNS responses to many ephemeral ports
	// are normal resolver behavior (e.g. Tailscale MagicDNS).
	if (pkt.Protocol != "TCP" || !pkt.TCPSyn) && pkt.SrcPort != 53 {
		w.nonSynDstPorts[pkt.DstPort] = struct{}{}
	}
	w.lastSeen = now
}

// Evaluate checks if the given source IP exceeds any flood threshold.
// Returns a non-nil Alert when a flood is detected (subject to dedup).
func (fd *FloodDetector) Evaluate(pkt capture.PacketEvent) *storage.Alert {
	fd.mu.Lock()

	w, ok := fd.windows[pkt.SrcIP]
	if !ok || w.packets < 10 {
		// Not enough data to judge
		fd.mu.Unlock()
		return nil
	}

	elapsed := w.lastSeen.Sub(w.firstSeen).Seconds()
	if elapsed <= 0 {
		elapsed = 1
	}

	pps := float64(w.packets) / elapsed
	bps := float64(w.bytes) / elapsed
	uniquePorts := len(w.nonSynDstPorts)

	var reasons []string
	severity := "WARNING"

	if elapsed >= floodMinRateDuration.Seconds() {
		if pps > floodPPSThreshold {
			reasons = append(reasons, fmt.Sprintf("%.0f pkt/s (umbral: %d)", pps, floodPPSThreshold))
			severity = "CRITICAL"
		}
		if bps > floodBPSThreshold {
			reasons = append(reasons, fmt.Sprintf("%.1f MB/s (umbral: %d MB/s)", bps/1024/1024, floodBPSThreshold/1024/1024))
			severity = "CRITICAL"
		}
	}
	if uniquePorts > floodUniquePortsThreshold {
		reasons = append(reasons, fmt.Sprintf("%d puertos destino distintos (umbral: %d)", uniquePorts, floodUniquePortsThreshold))
		severity = "CRITICAL"
	}

	fd.mu.Unlock()

	if len(reasons) == 0 {
		return nil
	}

	// Dedup: one alert per source IP every floodDedupTTL
	fp := fmt.Sprintf("FLOOD|%s", pkt.SrcIP)
	fd.mu.Lock()
	if last, seen := fd.dedup[fp]; seen && time.Since(last) < floodDedupTTL {
		fd.mu.Unlock()
		return nil
	}
	fd.dedup[fp] = time.Now()
	if len(fd.dedup) > 1000 {
		now := time.Now()
		for k, v := range fd.dedup {
			if now.Sub(v) > floodDedupTTL {
				delete(fd.dedup, k)
			}
		}
	}
	fd.mu.Unlock()

	desc := fmt.Sprintf("Posible inundación desde %s hacia %s: %s", pkt.SrcIP, pkt.DstIP, joinReasons(reasons))
	title := "🚨 Posible inundación (Flood DoS/DDoS)"

	return &storage.Alert{
		Time:        time.Now().UTC(),
		Severity:    severity,
		Title:       title,
		Description: &desc,
		Source:      fmt.Sprintf("%s:%d", pkt.SrcIP, pkt.SrcPort),
		Score:       65.0,
		Context:     []byte(`{"origin":"flood_detector","method":"heuristic"}`),
	}
}

func joinReasons(parts []string) string {
	s := ""
	for i, p := range parts {
		if i > 0 {
			s += "; "
		}
		s += p
	}
	return s
}
