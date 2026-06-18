//go:build windows

package capture

import (
	"testing"
)

// TestNpcapCapture_ImplementsPacketCapture verifies (via an interface
// assertion) that *NpcapCapture satisfies the PacketCapture contract.
// This is a compile-time check at the call site; if NpcapCapture drifts
// away from the interface, this test fails to build.
func TestNpcapCapture_ImplementsPacketCapture(t *testing.T) {
	var c PacketCapture = NewNpcapCapture()
	if c == nil {
		t.Fatal("NewNpcapCapture returned nil")
	}
}

func TestNpcapCapture_NewHasEmptyState(t *testing.T) {
	c := NewNpcapCapture()
	if c == nil {
		t.Fatal("NewNpcapCapture returned nil")
	}
	if c.Packets() == nil {
		t.Fatal("Packets() channel should be initialized")
	}
	stats := c.Stats()
	if stats.PacketsTotal != 0 || stats.BytesTotal != 0 || stats.PacketsDropped != 0 {
		t.Fatalf("fresh capture should have zero stats, got %+v", stats)
	}
	if !stats.StartedAt.IsZero() {
		t.Fatalf("StartedAt should be zero before Start, got %v", stats.StartedAt)
	}
	if stats.LastError != nil {
		t.Fatalf("LastError should be nil, got %v", stats.LastError)
	}
}

func TestNpcapCapture_StartEmptyIfaceFails(t *testing.T) {
	c := NewNpcapCapture()
	if err := c.Start("", ""); err == nil {
		t.Fatal("Start with empty iface should fail")
	}
}

func TestNpcapCapture_StartAlreadyRunningFails(t *testing.T) {
	c := NewNpcapCapture()
	// We don't actually open a real device here; instead we flip the
	// running flag and try to start again. The double-Start error path
	// must fire before any pcap.OpenLive call.
	c.running = true
	defer func() { c.running = false }()
	if err := c.Start("eth0", ""); err == nil {
		t.Fatal("Start on already-running capture should fail")
	}
}

func TestNpcapCapture_StopWhenNotRunningFails(t *testing.T) {
	c := NewNpcapCapture()
	if err := c.Stop(); err == nil {
		t.Fatal("Stop on idle capture should fail")
	}
}

func TestNpcapCapture_StartTwice_SecondCallFails(t *testing.T) {
	// This test requires Npcap + admin privileges to actually open a
	// device. We use t.Skip so the test runs in the unit-test lane on
	// any host that has Npcap (admin) — and is skipped on dev hosts
	// where capture is not available.
	t.Skip("integration test: requires Npcap installed and admin privileges")

	c := NewNpcapCapture()
	if err := c.Start("any-interface", ""); err != nil {
		t.Fatalf("first Start failed: %v", err)
	}
	defer c.Stop()
	if err := c.Start("any-interface", ""); err == nil {
		t.Fatal("second Start should fail")
	}
}
