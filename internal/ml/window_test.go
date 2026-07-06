package ml

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultWindowDurations(t *testing.T) {
	assert.Equal(t, 5*time.Second, defaultWindowDuration)
	assert.Equal(t, 30*time.Second, defaultCleanupInterval)
}

func TestPacketsPerSecUsesFiveSecondWindow(t *testing.T) {
	ws := NewWindowStats()
	defer ws.Close()

	now := time.Now()
	srcIP := "10.0.0.1"
	dstIP := "10.0.0.2"

	// Stale traffic outside the window must not affect the rate.
	ws.Record(srcIP, dstIP, 443, 100, now.Add(-10*time.Second))

	// Ten fresh packets spaced 200 ms apart over 1.8 seconds, safely inside the
	// active 5-second window.
	for i := 0; i < 10; i++ {
		ws.Record(srcIP, dstIP, 80, 100, now.Add(-3*time.Second+time.Duration(i)*200*time.Millisecond))
	}

	// Elapsed from first to last is 1.8 seconds.
	rate := ws.PacketsPerSec(srcIP)
	assert.InDelta(t, 10.0/1.8, rate, 0.01)
}

func TestUniqueDstPortsExcludesExpiredTraffic(t *testing.T) {
	ws := NewWindowStats()
	defer ws.Close()

	now := time.Now()
	srcIP := "10.0.0.1"
	dstIP := "10.0.0.2"

	// Two expired destination ports.
	ws.Record(srcIP, dstIP, 80, 100, now.Add(-10*time.Second))
	ws.Record(srcIP, dstIP, 443, 100, now.Add(-10*time.Second))

	// One fresh destination port.
	ws.Record(srcIP, dstIP, 53, 100, now.Add(-1*time.Second))

	assert.Equal(t, 1, ws.UniqueDstPorts(srcIP))
	assert.Equal(t, 1, ws.UniqueDstIPs(srcIP))
}

func TestEvictStaleRemovesExpiredSourceIPs(t *testing.T) {
	ws := NewWindowStats()
	defer ws.Close()

	now := time.Now()
	srcIP := "10.0.0.1"
	dstIP := "10.0.0.2"

	ws.Record(srcIP, dstIP, 80, 100, now.Add(-10*time.Second))

	ws.evictStale()

	assert.Equal(t, 0.0, ws.PacketsPerSec(srcIP))
	assert.Equal(t, 0, ws.UniqueDstPorts(srcIP))
	assert.Equal(t, 0, ws.UniqueDstIPs(srcIP))
	assert.Equal(t, 0.0, ws.BytesPerSec(srcIP))
}

func TestColdAndEmptyWindowsAreDeterministic(t *testing.T) {
	ws := NewWindowStats()
	defer ws.Close()

	assert.Equal(t, 0.0, ws.PacketsPerSec("10.0.0.1"))
	assert.Equal(t, 0, ws.UniqueDstPorts("10.0.0.1"))
	assert.Equal(t, 0, ws.UniqueDstIPs("10.0.0.1"))
	assert.Equal(t, 0.0, ws.BytesPerSec("10.0.0.1"))

	// A single recorded packet is the cold-window special case: PPS is 1.0.
	ws.Record("10.0.0.1", "10.0.0.2", 80, 100, time.Now())
	assert.Equal(t, 1.0, ws.PacketsPerSec("10.0.0.1"))
	assert.Equal(t, 1, ws.UniqueDstPorts("10.0.0.1"))
}

func TestWindowStatsTrimsPartiallyExpiredSamples(t *testing.T) {
	ws := NewWindowStats()
	defer ws.Close()

	now := time.Now()
	srcIP := "10.0.0.1"
	dstIP := "10.0.0.2"

	// One expired port followed by one fresh port; total bytes differ so we
	// can also verify byte aggregates are rebuilt after partial trimming.
	ws.Record(srcIP, dstIP, 443, 200, now.Add(-10*time.Second))
	ws.Record(srcIP, dstIP, 80, 100, now.Add(-1*time.Second))

	assert.Equal(t, 1, ws.UniqueDstPorts(srcIP))
	assert.Equal(t, 1, ws.UniqueDstIPs(srcIP))
	assert.Equal(t, int64(100), ws.byIP[srcIP].totalBytes)
}

func TestTrimRetainsExactCutoffSample(t *testing.T) {
	cutoff := time.Unix(1000, 0)
	w := &ipWindow{
		packets: []packetSample{
			{ts: cutoff.Add(-time.Second), dstPort: 80, length: 100},
			{ts: cutoff, dstPort: 443, length: 200},
			{ts: cutoff.Add(time.Second), dstPort: 53, length: 300},
		},
		dstPorts:   map[uint16]struct{}{80: {}, 443: {}, 53: {}},
		dstIPs:     map[string]struct{}{"10.0.0.2": {}, "10.0.0.3": {}, "10.0.0.4": {}},
		totalBytes: 600,
	}

	remaining := w.trim(cutoff)

	assert.Equal(t, 2, remaining)
	assert.Equal(t, 2, len(w.packets))
	assert.Equal(t, int64(500), w.totalBytes)
	assert.NotContains(t, w.dstPorts, uint16(80))
	assert.Contains(t, w.dstPorts, uint16(443))
	assert.Contains(t, w.dstPorts, uint16(53))
}

func TestQueryMethodsEvictStalePacketsWithoutRecord(t *testing.T) {
	ws := NewWindowStats()
	defer ws.Close()

	srcIP := "10.0.0.1"
	dstIP := "10.0.0.2"

	// Record one packet that is fresh now but will become stale before the
	// next query, without any further Record call.
	ws.Record(srcIP, dstIP, 80, 100, time.Now().Add(-4*time.Second))

	// Wait long enough for the packet to fall outside the 5-second window.
	time.Sleep(1500 * time.Millisecond)

	// Without any new Record call or cleanup cycle, query methods must
	// trim the stale packet at query time and return zero.
	assert.Equal(t, 0.0, ws.PacketsPerSec(srcIP))
	assert.Equal(t, 0, ws.UniqueDstPorts(srcIP))
	assert.Equal(t, 0, ws.UniqueDstIPs(srcIP))
	assert.Equal(t, 0.0, ws.BytesPerSec(srcIP))
}
