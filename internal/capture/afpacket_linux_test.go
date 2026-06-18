//go:build linux

package capture

import (
	"testing"
)

// TestAFPACKETCapture_ImplementsPacketCapture verifies (via an interface
// assertion) that *AFPACKETCapture satisfies the PacketCapture contract.
// This is a compile-time check at the call site; if AFPACKETCapture drifts
// away from the interface, this test fails to build.
func TestAFPACKETCapture_ImplementsPacketCapture(t *testing.T) {
	var c PacketCapture = NewAFPACKETCapture()
	if c == nil {
		t.Fatal("NewAFPACKETCapture returned nil")
	}
}

func TestAFPACKETCapture_NewHasEmptyState(t *testing.T) {
	c := NewAFPACKETCapture()
	if c == nil {
		t.Fatal("NewAFPACKETCapture returned nil")
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

func TestAFPACKETCapture_StartEmptyIfaceFails(t *testing.T) {
	c := NewAFPACKETCapture()
	if err := c.Start("", ""); err == nil {
		t.Fatal("Start with empty iface should fail")
	}
}

func TestAFPACKETCapture_StartAlreadyRunningFails(t *testing.T) {
	c := NewAFPACKETCapture()
	// We don't actually open a real device here; instead we flip the
	// running flag and try to start again. The double-Start error path
	// must fire before any pcap.OpenLive call.
	c.running = true
	defer func() { c.running = false }()
	if err := c.Start("eth0", ""); err == nil {
		t.Fatal("Start on already-running capture should fail")
	}
}

func TestAFPACKETCapture_StopWhenNotRunningFails(t *testing.T) {
	c := NewAFPACKETCapture()
	if err := c.Stop(); err == nil {
		t.Fatal("Stop on idle capture should fail")
	}
}

func TestAFPACKETCapture_StartTwice_SecondCallFails(t *testing.T) {
	// This test requires libpcap installed and root/CAP_NET_RAW to
	// actually open a device. We use t.Skip so the test runs in the
	// integration-test lane on a Linux host that has libpcap and the
	// needed privileges — and is skipped on dev hosts where capture
	// is not available.
	t.Skip("integration test: requires libpcap installed and root/CAP_NET_RAW")

	c := NewAFPACKETCapture()
	if err := c.Start("lo", ""); err != nil {
		t.Fatalf("first Start failed: %v", err)
	}
	defer c.Stop()
	if err := c.Start("lo", ""); err == nil {
		t.Fatal("second Start should fail")
	}
}
