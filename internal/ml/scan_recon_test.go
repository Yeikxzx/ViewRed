package ml

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viewred/viewred/internal/capture"
)

func TestScanReconRecordFiltersOnlyTCPSyn(t *testing.T) {
	base := time.Now()
	srcIP := "10.0.0.1"
	dstIP := "10.0.0.2"

	tests := []struct {
		name      string
		pkt       capture.PacketEvent
		wantCount int
	}{
		{
			name:      "TCP SYN is tracked",
			pkt:       makeSynPacket(srcIP, dstIP, 12345, 80, base),
			wantCount: 1,
		},
		{
			name:      "TCP ACK without SYN is ignored",
			pkt:       capture.PacketEvent{SrcIP: srcIP, DstIP: dstIP, Protocol: "TCP", DstPort: 80, Timestamp: base},
			wantCount: 0,
		},
		{
			name:      "TCP SYN ACK response is ignored",
			pkt:       capture.PacketEvent{SrcIP: "104.18.32.47", DstIP: dstIP, SrcPort: 443, DstPort: 50000, Protocol: "TCP", TCPSyn: true, TCPAck: true, Timestamp: base},
			wantCount: 0,
		},
		{
			name:      "UDP traffic is ignored",
			pkt:       capture.PacketEvent{SrcIP: srcIP, DstIP: dstIP, Protocol: "UDP", DstPort: 80, Timestamp: base},
			wantCount: 0,
		},
		{
			name:      "ICMP traffic is ignored",
			pkt:       capture.PacketEvent{SrcIP: srcIP, DstIP: dstIP, Protocol: "ICMP", Timestamp: base},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewScanReconDetector()
			d.Record(tt.pkt)

			d.mu.Lock()
			defer d.mu.Unlock()

			key := scanKey{SrcIP: tt.pkt.SrcIP, DstIP: tt.pkt.DstIP}
			w, ok := d.windows[key]
			if tt.wantCount == 0 {
				assert.False(t, ok, "expected no window for filtered packet")
				return
			}
			require.True(t, ok, "expected a window for tracked packet")
			assert.Equal(t, tt.wantCount, w.synCount)
		})
	}
}

func TestScanReconSynAckResponsesDoNotTriggerAlert(t *testing.T) {
	base := time.Now()
	d := NewScanReconDetector()

	for i := 0; i < 4; i++ {
		pkt := capture.PacketEvent{
			SrcIP:     "104.18.32.47",
			DstIP:     "192.168.100.3",
			SrcPort:   443,
			DstPort:   uint16(50000 + i),
			Protocol:  "TCP",
			TCPSyn:    true,
			TCPAck:    true,
			Timestamp: base.Add(time.Duration(i) * time.Second),
		}
		d.Record(pkt)
	}

	alert := d.Evaluate(capture.PacketEvent{
		SrcIP:     "104.18.32.47",
		DstIP:     "192.168.100.3",
		SrcPort:   443,
		DstPort:   50004,
		Protocol:  "TCP",
		TCPSyn:    true,
		TCPAck:    true,
		Timestamp: base.Add(4 * time.Second),
	})

	assert.Nil(t, alert, "HTTPS SYN+ACK responses must not be classified as port scans")
}

func TestScanReconFourExternalServiceSourcePortsStaySilent(t *testing.T) {
	base := time.Now()
	d := NewScanReconDetector()

	for i := 0; i < 4; i++ {
		pkt := capture.PacketEvent{
			SrcIP:     "85.210.196.11",
			DstIP:     "192.168.100.3",
			SrcPort:   443,
			DstPort:   uint16(50000 + i),
			Protocol:  "TCP",
			TCPSyn:    true,
			Timestamp: base.Add(time.Duration(i) * time.Second),
		}
		d.Record(pkt)
	}

	alert := d.Evaluate(capture.PacketEvent{
		SrcIP:     "85.210.196.11",
		DstIP:     "192.168.100.3",
		SrcPort:   443,
		DstPort:   50004,
		Protocol:  "TCP",
		TCPSyn:    true,
		Timestamp: base.Add(4 * time.Second),
	})

	assert.Nil(t, alert, "4 unique ports from a service source port is too weak for scan/recon")
}

func TestScanReconClientBoundServiceResponsesStaySilentAtThreshold(t *testing.T) {
	base := time.Now()
	d := NewScanReconDetector()

	for i := 0; i < 8; i++ {
		pkt := capture.PacketEvent{
			SrcIP:     "172.64.155.209",
			DstIP:     "192.168.100.3",
			SrcPort:   443,
			DstPort:   uint16(50000 + i),
			Protocol:  "TCP",
			TCPSyn:    true,
			Timestamp: base.Add(time.Duration(i) * time.Second),
		}
		d.Record(pkt)
	}

	alert := d.Evaluate(capture.PacketEvent{
		SrcIP:     "172.64.155.209",
		DstIP:     "192.168.100.3",
		SrcPort:   443,
		DstPort:   50008,
		Protocol:  "TCP",
		TCPSyn:    true,
		Timestamp: base.Add(8 * time.Second),
	})

	assert.Nil(t, alert, "client-bound HTTPS responses should not be classified as scan/recon")
}

func TestScanReconTargetIsolation(t *testing.T) {
	base := time.Now()
	srcIP := "10.0.0.1"

	d := NewScanReconDetector()
	// 3 unique ports to dst1 and 3 unique ports to dst2: neither target crosses the threshold.
	for i := 0; i < 3; i++ {
		d.Record(makeSynPacket(srcIP, "192.168.1.1", uint16(10000+i), uint16(1000+i), base))
		d.Record(makeSynPacket(srcIP, "192.168.1.2", uint16(20000+i), uint16(1000+i), base))
	}

	alert1 := d.Evaluate(makeSynPacket(srcIP, "192.168.1.1", 30000, 1000, base))
	alert2 := d.Evaluate(makeSynPacket(srcIP, "192.168.1.2", 30001, 1000, base))

	assert.Nil(t, alert1, "ports must not be combined across destinations")
	assert.Nil(t, alert2, "ports must not be combined across destinations")
}

func TestScanReconBelowThresholdIsSilent(t *testing.T) {
	base := time.Now()
	srcIP := "10.0.0.1"
	dstIP := "10.0.0.2"

	d := NewScanReconDetector()
	for i := 0; i < 3; i++ {
		d.Record(makeSynPacket(srcIP, dstIP, uint16(10000+i), uint16(1000+i), base))
	}

	alert := d.Evaluate(makeSynPacket(srcIP, dstIP, 20000, 1000, base))
	assert.Nil(t, alert, "3 unique ports must stay below WARNING threshold")
}

func TestScanReconWindowExpiry(t *testing.T) {
	base := time.Now()
	srcIP := "10.0.0.1"
	dstIP := "10.0.0.2"

	d := NewScanReconDetector()
	// 8 old SYNs outside the window.
	for i := 0; i < 8; i++ {
		d.Record(makeSynPacket(srcIP, dstIP, uint16(10000+i), uint16(1000+i), base.Add(-2*scanReconWindowDuration)))
	}
	// 8 fresh SYNs inside the window cross the warning threshold.
	for i := 0; i < 8; i++ {
		d.Record(makeSynPacket(srcIP, dstIP, uint16(20000+i), uint16(1000+i), base))
	}

	alert := d.Evaluate(makeSynPacket(srcIP, dstIP, 30000, 1000, base))
	require.NotNil(t, alert)

	var ctx map[string]interface{}
	require.NoError(t, json.Unmarshal(alert.Context, &ctx))
	assert.Equal(t, float64(8), ctx["port_count"])
	assert.Equal(t, float64(8), ctx["syn_count"])
}

func TestScanReconThresholdTiers(t *testing.T) {
	base := time.Now()
	srcIP := "10.0.0.1"
	dstIP := "10.0.0.2"

	tests := []struct {
		name         string
		portCount    int
		wantAlert    bool
		wantSeverity string
		wantScore    float32
		wantTitle    string
	}{
		{
			name:      "3 unique ports stays silent",
			portCount: 3,
			wantAlert: false,
		},
		{
			name:      "4 unique ports stays silent",
			portCount: 4,
			wantAlert: false,
		},
		{
			name:         "8 unique ports triggers WARNING",
			portCount:    8,
			wantAlert:    true,
			wantSeverity: "WARNING",
			wantScore:    70.0,
			wantTitle:    "Suspicious TCP SYN Scan/Recon",
		},
		{
			name:         "50 unique ports triggers HIGH",
			portCount:    50,
			wantAlert:    true,
			wantSeverity: "HIGH",
			wantScore:    85.0,
			wantTitle:    "Suspicious TCP SYN Scan/Recon",
		},
		{
			name:         "200 unique ports triggers CRITICAL",
			portCount:    200,
			wantAlert:    true,
			wantSeverity: "CRITICAL",
			wantScore:    95.0,
			wantTitle:    "Suspicious TCP SYN Scan/Recon",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewScanReconDetector()
			for i := 0; i < tt.portCount; i++ {
				d.Record(makeSynPacket(srcIP, dstIP, uint16(10000+i), uint16(1000+i), base))
			}

			alert := d.Evaluate(makeSynPacket(srcIP, dstIP, 20000, 1000, base))
			if !tt.wantAlert {
				assert.Nil(t, alert)
				return
			}

			require.NotNil(t, alert)
			assert.Equal(t, tt.wantSeverity, alert.Severity)
			assert.Equal(t, tt.wantScore, alert.Score)
			assert.Equal(t, tt.wantTitle, alert.Title)

			var ctx map[string]interface{}
			require.NoError(t, json.Unmarshal(alert.Context, &ctx))
			assert.Equal(t, "scan_recon_detector", ctx["origin"])
			assert.Equal(t, "heuristic", ctx["method"])
			assert.Equal(t, srcIP, ctx["src_ip"])
			assert.Equal(t, dstIP, ctx["dst_ip"])
			assert.Equal(t, float64(tt.portCount), ctx["port_count"])
			assert.Equal(t, float64(tt.portCount), ctx["syn_count"])
			assert.Equal(t, true, ctx["behavioral_suspicion"])
			assert.Equal(t, tt.wantSeverity, ctx["severity_decision"])
			assert.Equal(t, tt.wantTitle, alert.Title)

			ports, ok := ctx["dst_ports"].([]interface{})
			require.True(t, ok)
			assert.Len(t, ports, tt.portCount)
		})
	}
}

func TestScanReconDedup(t *testing.T) {
	base := time.Now()
	srcIP := "10.0.0.1"
	dstIP := "10.0.0.2"

	d := NewScanReconDetector()
	for i := 0; i < 8; i++ {
		d.Record(makeSynPacket(srcIP, dstIP, uint16(10000+i), uint16(1000+i), base))
	}

	first := d.Evaluate(makeSynPacket(srcIP, dstIP, 20000, 1000, base))
	require.NotNil(t, first, "first evaluation should emit an alert")

	second := d.Evaluate(makeSynPacket(srcIP, dstIP, 20001, 1000, base))
	assert.Nil(t, second, "alert should be suppressed within dedup TTL")

	// Different (SrcIP, DstIP) should still alert.
	otherSrc := "10.0.0.2"
	for i := 0; i < 8; i++ {
		d.Record(makeSynPacket(otherSrc, dstIP, uint16(10000+i), uint16(1000+i), base))
	}
	other := d.Evaluate(makeSynPacket(otherSrc, dstIP, 30000, 1000, base))
	assert.NotNil(t, other, "different source should emit its own alert")
}

func TestScanReconDedupExpiresAfterTTL(t *testing.T) {
	base := time.Now()
	srcIP := "10.0.0.1"
	dstIP := "10.0.0.2"

	d := NewScanReconDetector()
	for i := 0; i < 8; i++ {
		d.Record(makeSynPacket(srcIP, dstIP, uint16(10000+i), uint16(1000+i), base))
	}

	first := d.Evaluate(makeSynPacket(srcIP, dstIP, 20000, 1000, base))
	require.NotNil(t, first)

	// Simulate that the dedup entry was recorded just before the TTL expired.
	d.mu.Lock()
	d.dedup[scanKey{SrcIP: srcIP, DstIP: dstIP}] = dedupEntry{
		lastEmitted: time.Now().Add(-scanReconDedupTTL).Add(-1 * time.Second),
		severity:    "WARNING",
	}
	d.mu.Unlock()

	second := d.Evaluate(makeSynPacket(srcIP, dstIP, 20001, 1000, base))
	assert.NotNil(t, second, "alert should fire again after dedup TTL expires")
}

func TestScanReconEscalationInProductionCadence(t *testing.T) {
	base := time.Now()
	srcIP := "10.0.0.1"
	dstIP := "10.0.0.2"

	d := NewScanReconDetector()

	for i := 1; i <= 55; i++ {
		pkt := makeSynPacket(srcIP, dstIP, uint16(10000+i), uint16(1000+i), base)
		d.Record(pkt)
		alert := d.Evaluate(pkt)

		switch i {
		case 8:
			require.NotNil(t, alert, "8th unique port must emit WARNING")
			assert.Equal(t, "WARNING", alert.Severity)
			assert.Equal(t, float32(70.0), alert.Score)
		case 50:
			require.NotNil(t, alert, "50th unique port must escalate to HIGH despite dedup from WARNING")
			assert.Equal(t, "HIGH", alert.Severity)
			assert.Equal(t, float32(85.0), alert.Score)
		default:
			// Only expected alert points are 8 and 50; everything else should be
			// suppressed (after 8) or below threshold (before 8).
			if i > 8 {
				assert.Nil(t, alert, "unique port %d must be suppressed by dedup", i)
			} else {
				assert.Nil(t, alert, "unique port %d is below threshold", i)
			}
		}
	}
}

func TestScanReconEvidenceContext(t *testing.T) {
	base := time.Now()
	srcIP := "10.0.0.1"
	dstIP := "10.0.0.2"

	d := NewScanReconDetector()
	for i := 0; i < 8; i++ {
		d.Record(makeSynPacket(srcIP, dstIP, uint16(10000+i), uint16(1000+i), base))
	}

	alert := d.Evaluate(makeSynPacket(srcIP, dstIP, 20000, 1000, base))
	require.NotNil(t, alert)
	require.NotNil(t, alert.Description)
	assert.Contains(t, *alert.Description, "TCP SYN scan detected")
	assert.Contains(t, *alert.Description, dstIP)
	assert.Equal(t, fmt.Sprintf("%s:%d", srcIP, 20000), alert.Source)

	var ctx map[string]interface{}
	require.NoError(t, json.Unmarshal(alert.Context, &ctx))
	assert.Equal(t, "scan_recon_detector", ctx["origin"])
	assert.Equal(t, "heuristic", ctx["method"])
	assert.Equal(t, srcIP, ctx["src_ip"])
	assert.Equal(t, dstIP, ctx["dst_ip"])
	assert.Equal(t, float64(8), ctx["port_count"])
	assert.Equal(t, float64(8), ctx["syn_count"])
	assert.Equal(t, "WARNING", ctx["severity_decision"])
	assert.Equal(t, true, ctx["behavioral_suspicion"])

	ports, ok := ctx["dst_ports"].([]interface{})
	require.True(t, ok)
	assert.Len(t, ports, 8)
}

func TestScanReconConcurrentAccess(t *testing.T) {
	d := NewScanReconDetector()
	base := time.Now()
	srcIP := "10.0.0.1"
	dst := "10.0.0.2"

	var wg sync.WaitGroup
	workers := 4
	iterations := 250

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				pkt := makeSynPacket(srcIP, dst, uint16(1000+worker), uint16(1000+i), base)
				d.Record(pkt)
			}
		}(w)
	}

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				pkt := makeSynPacket(srcIP, dst, uint16(1000+worker), uint16(1000+i), base)
				_ = d.Evaluate(pkt)
			}
		}(w)
	}

	wg.Wait()

	d.mu.Lock()
	defer d.mu.Unlock()
	w, ok := d.windows[scanKey{SrcIP: srcIP, DstIP: dst}]
	require.True(t, ok)
	assert.Equal(t, workers*iterations, w.synCount)
}
