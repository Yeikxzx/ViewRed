package ml

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viewred/viewred/internal/capture"
)

// makePacket is a small helper that returns a PacketEvent with the supplied
// fields. It keeps table-driven tests readable and avoids repeating the
// protocol/port boilerplate.
func makePacket(srcIP, dstIP string, srcPort, dstPort uint16, length int, ts time.Time, tcpSyn bool) capture.PacketEvent {
	return capture.PacketEvent{
		SrcIP:     srcIP,
		DstIP:     dstIP,
		SrcPort:   srcPort,
		DstPort:   dstPort,
		Protocol:  "TCP",
		Length:    length,
		Timestamp: ts,
		TCPSyn:    tcpSyn,
	}
}

func TestFloodRecordAccumulatesPerSourceWindow(t *testing.T) {
	tests := []struct {
		name      string
		packets   []capture.PacketEvent
		wantPacks int
		wantBytes int64
		wantPorts int
	}{
		{
			name: "single packet keeps one count",
			packets: []capture.PacketEvent{
				makePacket("10.0.0.1", "10.0.0.2", 1234, 80, 100, time.Now(), false),
			},
			wantPacks: 1,
			wantBytes: 100,
			wantPorts: 1,
		},
		{
			name: "multiple packets to same destination accumulate",
			packets: []capture.PacketEvent{
				makePacket("10.0.0.1", "10.0.0.2", 1234, 80, 100, time.Now(), false),
				makePacket("10.0.0.1", "10.0.0.2", 1234, 80, 200, time.Now(), false),
				makePacket("10.0.0.1", "10.0.0.2", 1234, 80, 300, time.Now(), false),
			},
			wantPacks: 3,
			wantBytes: 600,
			wantPorts: 1,
		},
		{
			name: "unique destination ports are tracked",
			packets: []capture.PacketEvent{
				makePacket("10.0.0.1", "10.0.0.2", 1234, 80, 100, time.Now(), false),
				makePacket("10.0.0.1", "10.0.0.2", 1234, 443, 100, time.Now(), false),
				makePacket("10.0.0.1", "10.0.0.2", 1234, 8080, 100, time.Now(), false),
			},
			wantPacks: 3,
			wantBytes: 300,
			wantPorts: 3,
		},
		{
			name: "different sources have independent windows",
			packets: []capture.PacketEvent{
				makePacket("10.0.0.1", "10.0.0.2", 1234, 80, 100, time.Now(), false),
				makePacket("10.0.0.3", "10.0.0.2", 1234, 80, 400, time.Now(), false),
			},
			wantPacks: 1,
			wantBytes: 100,
			wantPorts: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fd := NewFloodDetector()
			for _, pkt := range tt.packets {
				fd.Record(pkt)
			}

			fd.mu.Lock()
			defer fd.mu.Unlock()

			w, ok := fd.windows["10.0.0.1"]
			require.True(t, ok, "expected a window for 10.0.0.1")
			assert.Equal(t, tt.wantPacks, w.packets)
			assert.Equal(t, tt.wantBytes, w.bytes)
			assert.Equal(t, tt.wantPorts, len(w.dstPorts))
		})
	}
}

func TestFloodEvaluateThresholds(t *testing.T) {
	base := time.Now()

	tests := []struct {
		name      string
		packets   []capture.PacketEvent
		wantAlert bool
		wantScore float32
	}{
		{
			name: "normal browsing stays below thresholds",
			packets: func() []capture.PacketEvent {
				var pkts []capture.PacketEvent
				for i := 0; i < 100; i++ {
					pkts = append(pkts, makePacket("10.0.0.1", "10.0.0.2", 1234, 80, 64, base, false))
				}
				return pkts
			}(),
			wantAlert: false,
		},
		{
			name: "exactly 1000 pps does not alert",
			packets: func() []capture.PacketEvent {
				var pkts []capture.PacketEvent
				for i := 0; i < 1000; i++ {
					pkts = append(pkts, makePacket("10.0.0.1", "10.0.0.2", 1234, 80, 64, base, false))
				}
				return pkts
			}(),
			wantAlert: false,
		},
		{
			name: "sustained packets trigger pps threshold",
			packets: func() []capture.PacketEvent {
				var pkts []capture.PacketEvent
				for i := 0; i < 3001; i++ {
					pkts = append(pkts, makePacket("10.0.0.1", "10.0.0.2", 1234, 80, 64, base.Add(time.Duration(i)*time.Millisecond), false))
				}
				return pkts
			}(),
			wantAlert: true,
			wantScore: 65.0,
		},
		{
			name: "high throughput triggers bps threshold",
			packets: func() []capture.PacketEvent {
				var pkts []capture.PacketEvent
				for i := 0; i < 10; i++ {
					// 10 MB per packet * 10 over ~3s remains above 25 MB/s.
					pkts = append(pkts, makePacket("10.0.0.1", "10.0.0.2", 1234, 80, 10<<20, base.Add(time.Duration(i)*334*time.Millisecond), false))
				}
				return pkts
			}(),
			wantAlert: true,
			wantScore: 65.0,
		},
		{
			name: "unique destination ports trigger threshold",
			packets: func() []capture.PacketEvent {
				var pkts []capture.PacketEvent
				for i := 0; i < 31; i++ {
					pkts = append(pkts, makePacket("10.0.0.1", "10.0.0.2", 1234, uint16(1000+i), 64, base, false))
				}
				return pkts
			}(),
			wantAlert: true,
			wantScore: 65.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fd := NewFloodDetector()
			for _, pkt := range tt.packets {
				fd.Record(pkt)
			}

			alert := fd.Evaluate(makePacket("10.0.0.1", "10.0.0.2", 1234, 80, 64, base, false))
			if !tt.wantAlert {
				assert.Nil(t, alert)
				return
			}

			require.NotNil(t, alert)
			assert.Equal(t, tt.wantScore, alert.Score)
			assert.Equal(t, "CRITICAL", alert.Severity)
		})
	}
}

func TestFloodShortBurstDoesNotTriggerRateAlert(t *testing.T) {
	base := time.Now()
	fd := NewFloodDetector()

	for i := 0; i < 20; i++ {
		fd.Record(makePacket("172.64.155.209", "192.168.100.3", 443, 80, 1<<20, base.Add(time.Duration(i)*time.Millisecond), false))
	}

	alert := fd.Evaluate(makePacket("172.64.155.209", "192.168.100.3", 443, 80, 64, base.Add(20*time.Millisecond), false))
	assert.Nil(t, alert, "short response bursts must not be inflated into flood-rate alerts")
}

func TestFloodSustainedHTTPSServiceResponseDoesNotTriggerRateAlert(t *testing.T) {
	base := time.Now()
	fd := NewFloodDetector()

	for i := 0; i < 3001; i++ {
		fd.Record(makePacket("172.64.155.209", "192.168.100.3", 443, 50000, 64, base.Add(time.Duration(i)*time.Millisecond), false))
	}

	alert := fd.Evaluate(makePacket("172.64.155.209", "192.168.100.3", 443, 50000, 64, base.Add(3*time.Second), false))
	assert.Nil(t, alert, "client-bound HTTPS responses from source port 443 should not be classified as flood")
}

func TestFloodDedupBySourceOnly(t *testing.T) {
	base := time.Now()
	srcIP := "10.0.0.1"
	dst1 := "192.168.1.1"
	dst2 := "192.168.1.2"

	fd := NewFloodDetector()

	// Seed enough traffic to guarantee a flood alert.
	for i := 0; i < 3001; i++ {
		fd.Record(makePacket(srcIP, dst1, 1234, 80, 64, base.Add(time.Duration(i)*time.Millisecond), false))
	}

	first := fd.Evaluate(makePacket(srcIP, dst1, 1234, 80, 64, base, false))
	require.NotNil(t, first, "first evaluation should emit an alert")

	// Same source, different destination — should be suppressed by source-only dedup.
	second := fd.Evaluate(makePacket(srcIP, dst2, 1234, 443, 64, base, false))
	assert.Nil(t, second, "same source with different destination should be deduplicated")

	// Different source should still be eligible.
	otherSrc := "10.0.0.2"
	for i := 0; i < 3001; i++ {
		fd.Record(makePacket(otherSrc, dst1, 1234, 80, 64, base.Add(time.Duration(i)*time.Millisecond), false))
	}
	other := fd.Evaluate(makePacket(otherSrc, dst1, 1234, 80, 64, base, false))
	assert.NotNil(t, other, "different source should emit its own alert")
}

func TestFloodDedupExpiresAfterTTL(t *testing.T) {
	base := time.Now()
	srcIP := "10.0.0.1"
	dst := "192.168.1.1"

	fd := NewFloodDetector()
	for i := 0; i < 3001; i++ {
		fd.Record(makePacket(srcIP, dst, 1234, 80, 64, base.Add(time.Duration(i)*time.Millisecond), false))
	}

	first := fd.Evaluate(makePacket(srcIP, dst, 1234, 80, 64, base, false))
	require.NotNil(t, first)

	// Simulate that the dedup entry was recorded just before the TTL expired.
	fd.mu.Lock()
	fd.dedup["FLOOD|"+srcIP] = time.Now().Add(-floodDedupTTL).Add(-1 * time.Second)
	fd.mu.Unlock()

	second := fd.Evaluate(makePacket(srcIP, dst, 1234, 80, 64, base, false))
	assert.NotNil(t, second, "alert should fire again after dedup TTL expires")
}

func TestFloodConcurrentAccess(t *testing.T) {
	fd := NewFloodDetector()
	base := time.Now()
	srcIP := "10.0.0.1"
	dst := "192.168.1.1"

	var wg sync.WaitGroup
	workers := 4
	iterations := 250

	// Concurrent Record calls from multiple workers.
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				pkt := makePacket(srcIP, dst, uint16(1000+worker), uint16(80+i%10), 64, base, false)
				fd.Record(pkt)
			}
		}(w)
	}

	// Concurrent Evaluate calls.
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				pkt := makePacket(srcIP, dst, uint16(1000+worker), uint16(80+i%10), 64, base, false)
				_ = fd.Evaluate(pkt)
			}
		}(w)
	}

	wg.Wait()

	fd.mu.Lock()
	defer fd.mu.Unlock()
	w, ok := fd.windows[srcIP]
	require.True(t, ok)
	assert.Equal(t, workers*iterations, w.packets)
}

func TestFloodSynScanDoesNotTriggerUniquePortAlert(t *testing.T) {
	base := time.Now()
	srcIP := "10.0.0.1"
	dstIP := "192.168.1.1"

	fd := NewFloodDetector()
	for i := 0; i < 31; i++ {
		fd.Record(makePacket(srcIP, dstIP, 1234, uint16(1000+i), 64, base, true))
	}

	alert := fd.Evaluate(makePacket(srcIP, dstIP, 1234, 80, 64, base, true))
	assert.Nil(t, alert, "TCP SYN scan ports must not trigger flood unique-port alert")
}

func TestFloodNonSynTcpStillTriggersUniquePortAlert(t *testing.T) {
	base := time.Now()
	srcIP := "10.0.0.1"
	dstIP := "192.168.1.1"

	fd := NewFloodDetector()
	for i := 0; i < 31; i++ {
		fd.Record(makePacket(srcIP, dstIP, 1234, uint16(1000+i), 64, base, false))
	}

	alert := fd.Evaluate(makePacket(srcIP, dstIP, 1234, 80, 64, base, false))
	require.NotNil(t, alert, "non-SYN TCP unique-port flood should still alert")
	assert.Equal(t, "CRITICAL", alert.Severity)
	assert.Contains(t, *alert.Description, "puertos destino distintos")
}

func TestFloodUdpMultiPortFloodStillTriggersAlert(t *testing.T) {
	base := time.Now()
	srcIP := "10.0.0.1"
	dstIP := "192.168.1.1"

	fd := NewFloodDetector()
	for i := 0; i < 31; i++ {
		fd.Record(capture.PacketEvent{
			SrcIP:     srcIP,
			DstIP:     dstIP,
			SrcPort:   1234,
			DstPort:   uint16(1000 + i),
			Protocol:  "UDP",
			Length:    64,
			Timestamp: base,
		})
	}

	alert := fd.Evaluate(capture.PacketEvent{
		SrcIP:     srcIP,
		DstIP:     dstIP,
		SrcPort:   1234,
		DstPort:   80,
		Protocol:  "UDP",
		Length:    64,
		Timestamp: base,
	})
	require.NotNil(t, alert, "UDP unique-port flood should still alert")
	assert.Equal(t, "CRITICAL", alert.Severity)
}

func TestFloodMixedSynAndNonSynCountsOnlyNonSynPorts(t *testing.T) {
	base := time.Now()
	srcIP := "10.0.0.1"
	dstIP := "192.168.1.1"

	fd := NewFloodDetector()
	// SYN traffic to 25 unique ports must not count toward flood unique-port threshold.
	for i := 0; i < 25; i++ {
		fd.Record(makePacket(srcIP, dstIP, 1234, uint16(1000+i), 64, base, true))
	}
	// Non-SYN traffic to 5 unique ports stays below the 30-port threshold.
	for i := 0; i < 5; i++ {
		fd.Record(makePacket(srcIP, dstIP, 1234, uint16(2000+i), 64, base, false))
	}

	fd.mu.Lock()
	w := fd.windows[srcIP]
	assert.Equal(t, 30, len(w.dstPorts), "all ports should still be tracked in dstPorts")
	assert.Equal(t, 5, len(w.nonSynDstPorts), "only non-SYN ports should count for flood signal")
	fd.mu.Unlock()

	alert := fd.Evaluate(makePacket(srcIP, dstIP, 1234, 80, 64, base, false))
	assert.Nil(t, alert, "mixed traffic below non-SYN threshold must not alert")
}

func TestFloodWindowResetClearsBothPortMaps(t *testing.T) {
	srcIP := "10.0.0.1"
	dstIP := "192.168.1.1"

	fd := NewFloodDetector()
	first := time.Now()
	for i := 0; i < 5; i++ {
		fd.Record(makePacket(srcIP, dstIP, 1234, uint16(1000+i), 64, first, false))
	}

	// Advance past the flood window duration to force a reset.
	afterExpiry := first.Add(floodWindowDuration + time.Second)
	fd.Record(makePacket(srcIP, dstIP, 1234, 80, 64, afterExpiry, false))

	fd.mu.Lock()
	defer fd.mu.Unlock()
	w := fd.windows[srcIP]
	assert.Equal(t, 1, w.packets)
	assert.Equal(t, 1, len(w.dstPorts))
	assert.Equal(t, 1, len(w.nonSynDstPorts))
}

func TestFloodAndScanReconDetectorsClassifySynScanIndependently(t *testing.T) {
	base := time.Now()
	srcIP := "10.0.0.1"
	dstIP := "192.168.1.1"

	floodDetector := NewFloodDetector()
	scanReconDetector := NewScanReconDetector()

	// Simulate production order from cmd/viewred/main.go.
	var lastPkt capture.PacketEvent
	for i := 0; i < 31; i++ {
		pkt := makePacket(srcIP, dstIP, 1234, uint16(1000+i), 64, base, true)
		floodDetector.Record(pkt)
		scanReconDetector.Record(pkt)
		lastPkt = pkt
	}

	floodAlert := floodDetector.Evaluate(lastPkt)
	scanAlert := scanReconDetector.Evaluate(lastPkt)

	assert.Nil(t, floodAlert, "SYN scan fan-out must not satisfy flood unique-port alert")
	require.NotNil(t, scanAlert, "SYN scan fan-out must still be classified by scan/recon detector")
	assert.Contains(t, *scanAlert.Description, "TCP SYN scan detected")
}

func TestFloodDnsSrcPort53DoesNotTriggerUniquePortAlert(t *testing.T) {
	base := time.Now()
	srcIP := "100.100.100.100"
	dstIP := "10.0.0.1"

	fd := NewFloodDetector()
	for i := 0; i < 31; i++ {
		fd.Record(capture.PacketEvent{
			SrcIP:     srcIP,
			DstIP:     dstIP,
			SrcPort:   53,
			DstPort:   uint16(1000 + i),
			Protocol:  "UDP",
			Length:    64,
			Timestamp: base,
		})
	}

	alert := fd.Evaluate(capture.PacketEvent{
		SrcIP:     srcIP,
		DstIP:     dstIP,
		SrcPort:   53,
		DstPort:   80,
		Protocol:  "UDP",
		Length:    64,
		Timestamp: base,
	})
	assert.Nil(t, alert, "DNS source-port 53 responses to many ephemeral ports must not trigger unique-port flood alert")
}

func TestFloodUdpMultiPortFloodStillTriggersAlert_Non53SrcPort(t *testing.T) {
	base := time.Now()
	srcIP := "10.0.0.1"
	dstIP := "192.168.1.1"

	fd := NewFloodDetector()
	for i := 0; i < 31; i++ {
		fd.Record(capture.PacketEvent{
			SrcIP:     srcIP,
			DstIP:     dstIP,
			SrcPort:   1234,
			DstPort:   uint16(1000 + i),
			Protocol:  "UDP",
			Length:    64,
			Timestamp: base,
		})
	}

	alert := fd.Evaluate(capture.PacketEvent{
		SrcIP:     srcIP,
		DstIP:     dstIP,
		SrcPort:   1234,
		DstPort:   80,
		Protocol:  "UDP",
		Length:    64,
		Timestamp: base,
	})
	require.NotNil(t, alert, "UDP unique-port flood from non-53 source should still alert")
	assert.Equal(t, "CRITICAL", alert.Severity)
	assert.Contains(t, *alert.Description, "puertos destino distintos")
}

func TestFloodVolumetricPpsFromSrcPort53StillAlerts(t *testing.T) {
	base := time.Now()
	srcIP := "100.100.100.100"
	dstIP := "10.0.0.1"

	fd := NewFloodDetector()
	for i := 0; i < 3001; i++ {
		fd.Record(capture.PacketEvent{
			SrcIP:     srcIP,
			DstIP:     dstIP,
			SrcPort:   53,
			DstPort:   80,
			Protocol:  "UDP",
			Length:    64,
			Timestamp: base.Add(time.Duration(i) * time.Millisecond),
		})
	}

	alert := fd.Evaluate(capture.PacketEvent{
		SrcIP:     srcIP,
		DstIP:     dstIP,
		SrcPort:   53,
		DstPort:   80,
		Protocol:  "UDP",
		Length:    64,
		Timestamp: base,
	})
	require.NotNil(t, alert, "PPS flood from source port 53 should still alert")
	assert.Contains(t, *alert.Description, "pkt/s")
}

func TestFloodVolumetricBpsFromSrcPort53StillAlerts(t *testing.T) {
	base := time.Now()
	srcIP := "100.100.100.100"
	dstIP := "10.0.0.1"

	fd := NewFloodDetector()
	for i := 0; i < 10; i++ {
		fd.Record(capture.PacketEvent{
			SrcIP:     srcIP,
			DstIP:     dstIP,
			SrcPort:   53,
			DstPort:   80,
			Protocol:  "UDP",
			Length:    10 << 20,
			Timestamp: base.Add(time.Duration(i) * 334 * time.Millisecond),
		})
	}

	alert := fd.Evaluate(capture.PacketEvent{
		SrcIP:     srcIP,
		DstIP:     dstIP,
		SrcPort:   53,
		DstPort:   80,
		Protocol:  "UDP",
		Length:    64,
		Timestamp: base,
	})
	require.NotNil(t, alert, "BPS flood from source port 53 should still alert")
	assert.Contains(t, *alert.Description, "MB/s")
}

func TestFloodDnsSrcPort53StillTrackedInDstPortsAndCounters(t *testing.T) {
	base := time.Now()
	srcIP := "100.100.100.100"
	dstIP := "10.0.0.1"

	fd := NewFloodDetector()
	for i := 0; i < 5; i++ {
		fd.Record(capture.PacketEvent{
			SrcIP:     srcIP,
			DstIP:     dstIP,
			SrcPort:   53,
			DstPort:   uint16(1000 + i),
			Protocol:  "UDP",
			Length:    100,
			Timestamp: base,
		})
	}

	fd.mu.Lock()
	w := fd.windows[srcIP]
	assert.Equal(t, 5, w.packets, "packets should count source-port 53 traffic")
	assert.Equal(t, int64(500), w.bytes, "bytes should count source-port 53 traffic")
	assert.Equal(t, 5, len(w.dstPorts), "dstPorts should retain source-port 53 visibility")
	assert.Equal(t, 0, len(w.nonSynDstPorts), "nonSynDstPorts should exclude source-port 53 traffic")
	fd.mu.Unlock()
}
