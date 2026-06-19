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
