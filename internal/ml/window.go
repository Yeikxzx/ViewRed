package ml

import (
	"sync"
	"time"
)

const (
	// defaultWindowDuration is how far back we track per-IP stats.
	defaultWindowDuration = 5 * time.Second

	// defaultCleanupInterval is how often stale IP entries are purged.
	defaultCleanupInterval = 30 * time.Second
)

// packetSample keeps the metadata needed to maintain correct aggregate
// state when the sliding window is partially trimmed.
type packetSample struct {
	ts       time.Time
	dstIP    string
	dstPort  uint16
	length   int
}

// ipWindow holds rolling counters for a single source IP over a time window.
type ipWindow struct {
	packets    []packetSample
	dstPorts   map[uint16]struct{}
	dstIPs     map[string]struct{}
	totalBytes int64
}

// WindowStats tracks per-source-IP traffic stats over a sliding time window.
// Safe for concurrent use by multiple evaluation workers.
type WindowStats struct {
	mu        sync.Mutex
	byIP      map[string]*ipWindow
	window    time.Duration
	stopClean chan struct{}
}

// NewWindowStats creates a rolling window tracker. The cleaner goroutine
// removes stale IPs periodically; call Close to stop it.
func NewWindowStats() *WindowStats {
	ws := &WindowStats{
		byIP:      make(map[string]*ipWindow),
		window:    defaultWindowDuration,
		stopClean: make(chan struct{}),
	}
	go ws.cleanupLoop()
	return ws
}

// cleanupLoop runs periodically to evict IPs with no recent activity.
func (ws *WindowStats) cleanupLoop() {
	ticker := time.NewTicker(defaultCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			ws.evictStale()
		case <-ws.stopClean:
			return
		}
	}
}

// Close stops the background cleanup goroutine.
func (ws *WindowStats) Close() {
	close(ws.stopClean)
}

// evictStale removes IPs whose last packet is outside the window.
func (ws *WindowStats) evictStale() {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.trimAllLocked(time.Now().Add(-ws.window))
}

// trimAllLocked drops samples older than cutoff from every IP window and
// removes IPs that no longer have any packets. Caller must hold ws.mu.
func (ws *WindowStats) trimAllLocked(cutoff time.Time) {
	for ip, w := range ws.byIP {
		w.trim(cutoff)
		if len(w.packets) == 0 {
			delete(ws.byIP, ip)
		}
	}
}

// trim drops samples older than cutoff and rebuilds aggregate state from the
// remaining samples so expired packets do not contribute to port/IP/byte counts.
func (w *ipWindow) trim(cutoff time.Time) int {
	first := len(w.packets)
	for i, s := range w.packets {
		if !s.ts.Before(cutoff) {
			first = i
			break
		}
	}
	if first == len(w.packets) {
		// All are stale
		w.packets = w.packets[:0]
		w.dstPorts = make(map[uint16]struct{})
		w.dstIPs = make(map[string]struct{})
		w.totalBytes = 0
		return 0
	}
	if first > 0 {
		w.packets = w.packets[first:]
		w.rebuildAggregates()
	}
	return len(w.packets)
}

// rebuildAggregates recomputes dstPorts, dstIPs and totalBytes from the
// current packet slice. It is called after partial trimming.
func (w *ipWindow) rebuildAggregates() {
	w.dstPorts = make(map[uint16]struct{})
	w.dstIPs = make(map[string]struct{})
	w.totalBytes = 0
	for _, s := range w.packets {
		w.dstPorts[s.dstPort] = struct{}{}
		w.dstIPs[s.dstIP] = struct{}{}
		w.totalBytes += int64(s.length)
	}
}

// Record adds a packet event to the window for its source IP.
func (ws *WindowStats) Record(ip string, dstIP string, dstPort uint16, length int, ts time.Time) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	w, ok := ws.byIP[ip]
	if !ok {
		w = &ipWindow{
			dstPorts: make(map[uint16]struct{}),
			dstIPs:   make(map[string]struct{}),
		}
		ws.byIP[ip] = w
	}

	s := packetSample{ts: ts, dstIP: dstIP, dstPort: dstPort, length: length}
	w.packets = append(w.packets, s)
	w.dstPorts[dstPort] = struct{}{}
	w.dstIPs[dstIP] = struct{}{}
	w.totalBytes += int64(length)

	// Trim old entries to keep the slice bounded
	cutoff := time.Now().Add(-ws.window)
	w.trim(cutoff)
}

// PacketsPerSec returns the packet rate for the given source IP
// within the current window. Returns 0 if the IP has no recent data.
func (ws *WindowStats) PacketsPerSec(ip string) float64 {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	ws.trimAllLocked(time.Now().Add(-ws.window))

	w, ok := ws.byIP[ip]
	if !ok || len(w.packets) == 0 {
		return 0
	}

	n := len(w.packets)
	if n < 2 {
		if n == 0 {
			return 0
		}
		return 1.0
	}
	elapsed := w.packets[n-1].ts.Sub(w.packets[0].ts).Seconds()
	if elapsed <= 0 {
		return float64(n)
	}
	return float64(n) / elapsed
}

// UniqueDstPorts returns the number of distinct destination ports
// contacted by the given source IP in the current window.
func (ws *WindowStats) UniqueDstPorts(ip string) int {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	ws.trimAllLocked(time.Now().Add(-ws.window))

	w, ok := ws.byIP[ip]
	if !ok {
		return 0
	}
	return len(w.dstPorts)
}

// UniqueDstIPs returns the number of distinct destination IPs
// contacted by the given source IP in the current window.
func (ws *WindowStats) UniqueDstIPs(ip string) int {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	ws.trimAllLocked(time.Now().Add(-ws.window))

	w, ok := ws.byIP[ip]
	if !ok {
		return 0
	}
	return len(w.dstIPs)
}

// BytesPerSec returns the byte rate for the given source IP.
func (ws *WindowStats) BytesPerSec(ip string) float64 {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	ws.trimAllLocked(time.Now().Add(-ws.window))

	w, ok := ws.byIP[ip]
	if !ok || len(w.packets) == 0 {
		return 0
	}

	n := len(w.packets)
	elapsed := w.packets[n-1].ts.Sub(w.packets[0].ts).Seconds()
	if elapsed <= 0 {
		return float64(w.totalBytes)
	}
	return float64(w.totalBytes) / elapsed
}
