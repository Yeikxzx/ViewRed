package rules

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/viewred/viewred/internal/capture"
)

func TestParse(t *testing.T) {
	line := `alert tcp any any -> 192.168.1.100 80 (msg:"HTTP Traffic"; sid:1; rev:1;)`
	r, err := Parse(line)
	require.NoError(t, err)

	assert.Equal(t, "alert", r.Action)
	assert.Equal(t, "tcp", r.Protocol)
	assert.Equal(t, "any", r.SrcIP)
	assert.Equal(t, "any", r.SrcPort)
	assert.Equal(t, "192.168.1.100", r.DstIP)
	assert.Equal(t, "80", r.DstPort)
	assert.Equal(t, "HTTP Traffic", r.Options["msg"])
	assert.Equal(t, "1", r.Options["sid"])
}

func TestEngineEvaluate(t *testing.T) {
	e := NewEngine()
	err := e.AddRule(`alert tcp 10.0.0.1 any -> any 22 (msg:"SSH from 10.0.0.1";)`)
	require.NoError(t, err)

	err = e.AddRule(`alert udp any any -> any 53 (msg:"DNS query";)`)
	require.NoError(t, err)

	// Match first rule
	pkt1 := capture.PacketEvent{
		Timestamp: time.Now(),
		SrcIP:     "10.0.0.1",
		DstIP:     "192.168.1.50",
		SrcPort:   12345,
		DstPort:   22,
		Protocol:  "TCP",
		Length:    64,
	}

	matches := e.Evaluate(pkt1)
	require.Len(t, matches, 1)
	assert.Equal(t, "SSH from 10.0.0.1", matches[0].Options["msg"])

	// Match second rule
	pkt2 := capture.PacketEvent{
		Timestamp: time.Now(),
		SrcIP:     "192.168.1.50",
		DstIP:     "8.8.8.8",
		SrcPort:   54321,
		DstPort:   53,
		Protocol:  "UDP",
		Length:    128,
	}

	matches = e.Evaluate(pkt2)
	require.Len(t, matches, 1)
	assert.Equal(t, "DNS query", matches[0].Options["msg"])

	// Match nothing
	pkt3 := capture.PacketEvent{
		Timestamp: time.Now(),
		SrcIP:     "10.0.0.2", // diff src
		DstIP:     "192.168.1.50",
		SrcPort:   12345,
		DstPort:   22,
		Protocol:  "TCP",
		Length:    64,
	}

	matches = e.Evaluate(pkt3)
	require.Len(t, matches, 0)
}

func TestMatch_SYNOnlyMatches(t *testing.T) {
	e := NewEngine()
	err := e.AddRule(`alert tcp any any -> any 22 (msg:"SSH Connection Attempt"; flags:S;)`)
	require.NoError(t, err)

	pkt := capture.PacketEvent{
		Timestamp: time.Now(),
		SrcIP:     "10.0.0.1",
		DstIP:     "192.168.1.50",
		SrcPort:   12345,
		DstPort:   22,
		Protocol:  "TCP",
		TCPSyn:    true,
	}

	matches := e.Evaluate(pkt)
	require.Len(t, matches, 1)
	assert.Equal(t, "SSH Connection Attempt", matches[0].Options["msg"])
}

func TestMatch_NonSYNMismatch(t *testing.T) {
	e := NewEngine()
	err := e.AddRule(`alert tcp any any -> any 22 (msg:"SSH Connection Attempt"; flags:S;)`)
	require.NoError(t, err)

	pkt := capture.PacketEvent{
		Timestamp: time.Now(),
		SrcIP:     "10.0.0.1",
		DstIP:     "192.168.1.50",
		SrcPort:   12345,
		DstPort:   22,
		Protocol:  "TCP",
		TCPSyn:    false,
	}

	matches := e.Evaluate(pkt)
	require.Len(t, matches, 0)
}

func TestMatch_SYNACKMismatch(t *testing.T) {
	e := NewEngine()
	err := e.AddRule(`alert tcp any any -> any 22 (msg:"SSH Connection Attempt"; flags:S;)`)
	require.NoError(t, err)

	pkt := capture.PacketEvent{
		Timestamp: time.Now(),
		SrcIP:     "192.168.1.50",
		DstIP:     "10.0.0.1",
		SrcPort:   22,
		DstPort:   12345,
		Protocol:  "TCP",
		TCPSyn:    true,
		TCPAck:    true,
	}

	matches := e.Evaluate(pkt)
	require.Len(t, matches, 0)
}

func TestMatch_PortGate(t *testing.T) {
	e := NewEngine()
	err := e.AddRule(`alert tcp any any -> any 22 (msg:"SSH Connection Attempt"; flags:S;)`)
	require.NoError(t, err)

	pkt := capture.PacketEvent{
		Timestamp: time.Now(),
		SrcIP:     "10.0.0.1",
		DstIP:     "192.168.1.50",
		SrcPort:   12345,
		DstPort:   2222,
		Protocol:  "TCP",
		TCPSyn:    true,
	}

	matches := e.Evaluate(pkt)
	require.Len(t, matches, 0)
}

func TestMatch_UnsupportedFlagSafety(t *testing.T) {
	e := NewEngine()
	err := e.AddRule(`alert tcp any any -> any 22 (msg:"SSH with ACK flag"; flags:A;)`)
	require.NoError(t, err)

	pkt := capture.PacketEvent{
		Timestamp: time.Now(),
		SrcIP:     "10.0.0.1",
		DstIP:     "192.168.1.50",
		SrcPort:   12345,
		DstPort:   22,
		Protocol:  "TCP",
		TCPSyn:    true,
	}

	matches := e.Evaluate(pkt)
	require.Len(t, matches, 0)
}

func TestDedupTTL_DefaultsToOneMinute(t *testing.T) {
	e := NewEngine()
	assert.Equal(t, time.Minute, e.dedupTTL)
}

func TestDedupTTL_IsConfigurable(t *testing.T) {
	e := NewEngine()
	e.SetDedupTTL(30 * time.Second)
	assert.Equal(t, 30*time.Second, e.dedupTTL)

	e.SetDedupTTL(0)
	assert.Equal(t, time.Minute, e.dedupTTL)
}

func TestDedup_SuppressesDuplicateWithinCooldown(t *testing.T) {
	e := NewEngine()
	err := e.AddRule(`alert tcp any any -> any 22 (msg:"SSH Connection Attempt"; flags:S;)`)
	require.NoError(t, err)

	pkt := capture.PacketEvent{
		Timestamp: time.Now(),
		SrcIP:     "10.0.0.1",
		DstIP:     "192.168.1.50",
		SrcPort:   12345,
		DstPort:   22,
		Protocol:  "TCP",
		TCPSyn:    true,
	}

	// First occurrence must fire.
	matches := e.Evaluate(pkt)
	require.Len(t, matches, 1)

	// Same key within the cooldown must be suppressed.
	matches = e.Evaluate(pkt)
	assert.Len(t, matches, 0)

	// A different destination port is a different dedup key and must fire.
	pkt.DstPort = 2222
	matches = e.Evaluate(pkt)
	assert.Len(t, matches, 0) // port 22 rule does not match port 2222
}

func TestDedup_AllowsReappearanceAfterCooldown(t *testing.T) {
	e := NewEngine()
	err := e.AddRule(`alert tcp any any -> any 22 (msg:"SSH Connection Attempt"; flags:S;)`)
	require.NoError(t, err)

	pkt := capture.PacketEvent{
		Timestamp: time.Now(),
		SrcIP:     "10.0.0.1",
		DstIP:     "192.168.1.50",
		SrcPort:   12345,
		DstPort:   22,
		Protocol:  "TCP",
		TCPSyn:    true,
	}

	matches := e.Evaluate(pkt)
	require.Len(t, matches, 1)

	// Simulate the cooldown having elapsed by backdating the stored dedup entry.
	fp := "SSH Connection Attempt|10.0.0.1|192.168.1.50|22"
	e.dedupMu.Lock()
	e.dedup[fp] = time.Now().Add(-61 * time.Second)
	e.dedupMu.Unlock()

	matches = e.Evaluate(pkt)
	assert.Len(t, matches, 1)
}
