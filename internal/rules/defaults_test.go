package rules

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/viewred/viewred/internal/capture"
)

func TestDefaultRules(t *testing.T) {
	defaults := DefaultRuleStrings()
	require.Len(t, defaults, 2)

	ssh, err := Parse(defaults[0])
	require.NoError(t, err)
	assert.Equal(t, "22", ssh.DstPort)
	assert.Equal(t, "SSH Connection Attempt", ssh.Options["msg"])
	assert.Equal(t, "INFO", ssh.Options["severity"])
	assert.Equal(t, "S", ssh.Options["flags"])

	telnet, err := Parse(defaults[1])
	require.NoError(t, err)
	assert.Equal(t, "23", telnet.DstPort)
	assert.Equal(t, "Insecure Telnet Traffic", telnet.Options["msg"])
	assert.Equal(t, "WARNING", telnet.Options["severity"])
	assert.Equal(t, "S", telnet.Options["flags"])
}

func TestDefaultRules_Evaluate(t *testing.T) {
	e := NewEngine()
	for _, rule := range DefaultRuleStrings() {
		require.NoError(t, e.AddRule(rule))
	}

	sshSyn := capture.PacketEvent{
		Timestamp: time.Now(),
		SrcIP:     "10.0.0.1",
		DstIP:     "192.168.1.50",
		SrcPort:   12345,
		DstPort:   22,
		Protocol:  "TCP",
		TCPSyn:    true,
	}
	matches := e.Evaluate(sshSyn)
	require.Len(t, matches, 1)
	assert.Equal(t, "SSH Connection Attempt", matches[0].Options["msg"])
	assert.Equal(t, "INFO", matches[0].Options["severity"])

	sshAck := capture.PacketEvent{
		Timestamp: time.Now(),
		SrcIP:     "10.0.0.1",
		DstIP:     "192.168.1.50",
		SrcPort:   12345,
		DstPort:   22,
		Protocol:  "TCP",
		TCPSyn:    false,
	}
	matches = e.Evaluate(sshAck)
	assert.Len(t, matches, 0)

	telnetSyn := capture.PacketEvent{
		Timestamp: time.Now(),
		SrcIP:     "10.0.0.1",
		DstIP:     "192.168.1.50",
		SrcPort:   12345,
		DstPort:   23,
		Protocol:  "TCP",
		TCPSyn:    true,
	}
	matches = e.Evaluate(telnetSyn)
	require.Len(t, matches, 1)
	assert.Equal(t, "Insecure Telnet Traffic", matches[0].Options["msg"])
	assert.Equal(t, "WARNING", matches[0].Options["severity"])

	telnetAck := capture.PacketEvent{
		Timestamp: time.Now(),
		SrcIP:     "10.0.0.1",
		DstIP:     "192.168.1.50",
		SrcPort:   12345,
		DstPort:   23,
		Protocol:  "TCP",
		TCPSyn:    false,
	}
	matches = e.Evaluate(telnetAck)
	assert.Len(t, matches, 0)
}
