package ml

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viewred/viewred/internal/capture"
)

// makeSynPacket returns a TCP SYN PacketEvent for brute-force tests.
func makeSynPacket(srcIP, dstIP string, srcPort, dstPort uint16, ts time.Time) capture.PacketEvent {
	return makePacket(srcIP, dstIP, srcPort, dstPort, 64, ts, true)
}

func TestBruteForceRecordFiltersOnlyTargetPorts(t *testing.T) {
	base := time.Now()

	tests := []struct {
		name      string
		pkt       capture.PacketEvent
		wantCount int
	}{
		{
			name:      "SSH SYN port 22 is tracked",
			pkt:       makeSynPacket("10.0.0.1", "10.0.0.2", 12345, 22, base),
			wantCount: 1,
		},
		{
			name:      "Telnet SYN port 23 is tracked",
			pkt:       makeSynPacket("10.0.0.1", "10.0.0.2", 12345, 23, base),
			wantCount: 1,
		},
		{
			name:      "HTTP SYN port 80 is ignored",
			pkt:       makeSynPacket("10.0.0.1", "10.0.0.2", 12345, 80, base),
			wantCount: 0,
		},
		{
			name:      "non-TCP traffic is ignored",
			pkt:       capture.PacketEvent{SrcIP: "10.0.0.1", DstIP: "10.0.0.2", Protocol: "UDP", DstPort: 23, Timestamp: base},
			wantCount: 0,
		},
		{
			name:      "TCP ACK without SYN is ignored",
			pkt:       capture.PacketEvent{SrcIP: "10.0.0.1", DstIP: "10.0.0.2", Protocol: "TCP", DstPort: 22, Timestamp: base},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewBruteForceDetector()
			d.Record(tt.pkt)

			d.mu.Lock()
			defer d.mu.Unlock()

			key := bruteForceKey{SrcIP: tt.pkt.SrcIP, DstIP: tt.pkt.DstIP, DstPort: tt.pkt.DstPort}
			w, ok := d.windows[key]
			if tt.wantCount == 0 {
				assert.False(t, ok, "expected no window for filtered packet")
				return
			}
			require.True(t, ok, "expected a window for tracked packet")
			assert.Len(t, w.attempts, tt.wantCount)
		})
	}
}

func TestBruteForceThresholds(t *testing.T) {
	base := time.Now()
	srcIP := "10.0.0.1"
	dstIP := "10.0.0.2"

	tests := []struct {
		name         string
		port         uint16
		count        int
		wantAlert    bool
		wantSeverity string
		wantScore    float32
		wantTitle    string
	}{
		{
			name:      "9 SSH SYNs stays below threshold",
			port:      22,
			count:     9,
			wantAlert: false,
		},
		{
			name:         "10 SSH SYNs triggers HIGH",
			port:         22,
			count:        10,
			wantAlert:    true,
			wantSeverity: "HIGH",
			wantScore:    80.0,
			wantTitle:    "Suspicious SSH Brute Force",
		},
		{
			name:         "50 Telnet SYNs triggers CRITICAL",
			port:         23,
			count:        50,
			wantAlert:    true,
			wantSeverity: "CRITICAL",
			wantScore:    90.0,
			wantTitle:    "Suspicious Telnet Brute Force",
		},
		{
			name:         "60 SSH SYNs stays CRITICAL",
			port:         22,
			count:        60,
			wantAlert:    true,
			wantSeverity: "CRITICAL",
			wantScore:    90.0,
			wantTitle:    "Suspicious SSH Brute Force",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewBruteForceDetector()
			for i := 0; i < tt.count; i++ {
				d.Record(makeSynPacket(srcIP, dstIP, uint16(10000+i), tt.port, base))
			}

			alert := d.Evaluate(makeSynPacket(srcIP, dstIP, 20000, tt.port, base))
			if !tt.wantAlert {
				assert.Nil(t, alert)
				return
			}

			require.NotNil(t, alert)
			assert.Equal(t, tt.wantSeverity, alert.Severity)
			assert.Equal(t, tt.wantScore, alert.Score)
			assert.Equal(t, tt.wantTitle, alert.Title)
		})
	}
}

func TestBruteForceServiceSeparation(t *testing.T) {
	base := time.Now()
	srcIP := "10.0.0.1"
	dstIP := "10.0.0.2"

	d := NewBruteForceDetector()
	for i := 0; i < 9; i++ {
		d.Record(makeSynPacket(srcIP, dstIP, uint16(10000+i), 22, base))
		d.Record(makeSynPacket(srcIP, dstIP, uint16(20000+i), 23, base))
	}

	alert := d.Evaluate(makeSynPacket(srcIP, dstIP, 30000, 22, base))
	assert.Nil(t, alert, "ports must not be combined")
}

func TestBruteForceTargetIsolation(t *testing.T) {
	base := time.Now()
	srcIP := "10.0.0.1"

	d := NewBruteForceDetector()
	for i := 0; i < 9; i++ {
		d.Record(makeSynPacket(srcIP, "10.0.0.2", uint16(10000+i), 22, base))
		d.Record(makeSynPacket(srcIP, "10.0.0.3", uint16(20000+i), 22, base))
	}

	alert1 := d.Evaluate(makeSynPacket(srcIP, "10.0.0.2", 30000, 22, base))
	alert2 := d.Evaluate(makeSynPacket(srcIP, "10.0.0.3", 30001, 22, base))

	assert.Nil(t, alert1, "attempts must not be combined across destinations")
	assert.Nil(t, alert2, "attempts must not be combined across destinations")
}

func TestBruteForceWindowExpiry(t *testing.T) {
	base := time.Now()
	srcIP := "10.0.0.1"
	dstIP := "10.0.0.2"

	d := NewBruteForceDetector()
	// 9 old attempts outside the window.
	for i := 0; i < 9; i++ {
		d.Record(makeSynPacket(srcIP, dstIP, uint16(10000+i), 22, base.Add(-2*bruteForceWindowDuration)))
	}
	// 10 fresh attempts inside the window.
	for i := 0; i < 10; i++ {
		d.Record(makeSynPacket(srcIP, dstIP, uint16(20000+i), 22, base))
	}

	alert := d.Evaluate(makeSynPacket(srcIP, dstIP, 30000, 22, base))
	require.NotNil(t, alert)

	var ctx map[string]interface{}
	require.NoError(t, json.Unmarshal(alert.Context, &ctx))
	assert.Equal(t, float64(10), ctx["attempt_count"])
}

func TestBruteForceDedup(t *testing.T) {
	base := time.Now()
	srcIP := "10.0.0.1"
	dstIP := "10.0.0.2"

	d := NewBruteForceDetector()
	for i := 0; i < 15; i++ {
		d.Record(makeSynPacket(srcIP, dstIP, uint16(10000+i), 22, base))
	}

	first := d.Evaluate(makeSynPacket(srcIP, dstIP, 20000, 22, base))
	require.NotNil(t, first, "first evaluation should emit an alert")

	second := d.Evaluate(makeSynPacket(srcIP, dstIP, 20001, 22, base))
	assert.Nil(t, second, "alert should be suppressed within dedup TTL")

	// Different (SrcIP, DstPort) should still alert.
	otherKey := makeSynPacket("10.0.0.2", dstIP, 10000, 22, base)
	for i := 0; i < 15; i++ {
		d.Record(makeSynPacket("10.0.0.2", dstIP, uint16(10000+i), 22, base))
	}
	other := d.Evaluate(otherKey)
	assert.NotNil(t, other, "different source should emit its own alert")
}

func TestBruteForceDedupExpiresAfterTTL(t *testing.T) {
	base := time.Now()
	srcIP := "10.0.0.1"
	dstIP := "10.0.0.2"

	d := NewBruteForceDetector()
	for i := 0; i < 15; i++ {
		d.Record(makeSynPacket(srcIP, dstIP, uint16(10000+i), 22, base))
	}

	first := d.Evaluate(makeSynPacket(srcIP, dstIP, 20000, 22, base))
	require.NotNil(t, first)

	// Simulate that the dedup entry was recorded just before the TTL expired.
	d.mu.Lock()
	d.dedup[bruteForceKey{SrcIP: srcIP, DstIP: dstIP, DstPort: 22}] = dedupEntry{
		lastEmitted: time.Now().Add(-bruteForceDedupTTL).Add(-1 * time.Second),
		severity:    "HIGH",
	}
	d.mu.Unlock()

	second := d.Evaluate(makeSynPacket(srcIP, dstIP, 20001, 22, base))
	assert.NotNil(t, second, "alert should fire again after dedup TTL expires")
}

func TestBruteForceEscalationInProductionCadence(t *testing.T) {
	base := time.Now()
	srcIP := "10.0.0.1"
	dstIP := "10.0.0.2"

	d := NewBruteForceDetector()

	for i := 1; i <= 55; i++ {
		pkt := makeSynPacket(srcIP, dstIP, uint16(10000+i), 22, base)
		d.Record(pkt)
		alert := d.Evaluate(pkt)

		switch i {
		case 10:
			require.NotNil(t, alert, "10th SYN must emit HIGH")
			assert.Equal(t, "HIGH", alert.Severity)
			assert.Equal(t, float32(80.0), alert.Score)
		case 50:
			require.NotNil(t, alert, "50th SYN must escalate to CRITICAL despite dedup from HIGH")
			assert.Equal(t, "CRITICAL", alert.Severity)
			assert.Equal(t, float32(90.0), alert.Score)
		default:
			// Only expected alert points are 10 and 50; everything else should be
			// suppressed (after 10) or below threshold (before 10).
			if i > 10 {
				assert.Nil(t, alert, "SYN %d must be suppressed by dedup", i)
			} else {
				assert.Nil(t, alert, "SYN %d is below threshold", i)
			}
		}
	}
}

func TestBruteForceEvidenceContext(t *testing.T) {
	base := time.Now()
	srcIP := "10.0.0.1"
	dstIP := "10.0.0.2"

	d := NewBruteForceDetector()
	for i := 0; i < 12; i++ {
		d.Record(makeSynPacket(srcIP, dstIP, uint16(10000+i), 22, base))
	}

	alert := d.Evaluate(makeSynPacket(srcIP, dstIP, 20000, 22, base))
	require.NotNil(t, alert)
	require.NotNil(t, alert.Description)
	assert.Contains(t, *alert.Description, "authentication failure is not proven")
	assert.Contains(t, *alert.Description, "SSH")
	assert.Equal(t, "10.0.0.1:20000", alert.Source)

	var ctx map[string]interface{}
	require.NoError(t, json.Unmarshal(alert.Context, &ctx))
	assert.Equal(t, "bruteforce_detector", ctx["origin"])
	assert.Equal(t, "heuristic", ctx["method"])
	assert.Equal(t, "SSH", ctx["service"])
	assert.Equal(t, srcIP, ctx["src_ip"])
	assert.Equal(t, dstIP, ctx["dst_ip"])
	assert.Equal(t, float64(22), ctx["dst_port"])
	assert.Equal(t, float64(12), ctx["attempt_count"])
	assert.Equal(t, float64(60), ctx["window_seconds"])
	assert.Equal(t, float64(bruteForceThreshold), ctx["threshold"])
	assert.Equal(t, float64(bruteForceCriticalThreshold), ctx["critical_threshold"])
	assert.Equal(t, "HIGH", ctx["severity_decision"])
	assert.Equal(t, true, ctx["behavioral_suspicion"])
}

func TestBruteForceConcurrentAccess(t *testing.T) {
	d := NewBruteForceDetector()
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
				pkt := makeSynPacket(srcIP, dst, uint16(1000+worker), 22, base)
				d.Record(pkt)
			}
		}(w)
	}

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				pkt := makeSynPacket(srcIP, dst, uint16(1000+worker), 22, base)
				_ = d.Evaluate(pkt)
			}
		}(w)
	}

	wg.Wait()

	d.mu.Lock()
	defer d.mu.Unlock()
	w, ok := d.windows[bruteForceKey{SrcIP: srcIP, DstIP: dst, DstPort: 22}]
	require.True(t, ok)
	assert.Equal(t, workers*iterations, len(w.attempts))
}
