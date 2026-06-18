package capture

import (
	"testing"
	"time"
)

func TestMockCaptureStartStop(t *testing.T) {
	m := NewMockCapture()
	if err := m.Start("eth0", ""); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := m.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestMockCapturePushesEvents(t *testing.T) {
	m := NewMockCapture()
	if err := m.Start("eth0", ""); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer m.Stop()

	m.PushEvent(PacketEvent{
		Timestamp: time.Now(),
		SrcIP:     "10.0.0.1",
		DstIP:     "10.0.0.2",
		Protocol:  "TCP",
		Length:    100,
	})

	// Drain the channel (non-blocking via select).
	select {
	case e := <-m.Packets():
		if e.SrcIP != "10.0.0.1" {
			t.Fatalf("got SrcIP %q, want %q", e.SrcIP, "10.0.0.1")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("no event received within 100ms")
	}
}

func TestMockCaptureStats(t *testing.T) {
	m := NewMockCapture()
	_ = m.Start("eth0", "")
	defer m.Stop()
	m.PushEvent(PacketEvent{Timestamp: time.Now(), Length: 100})
	m.PushEvent(PacketEvent{Timestamp: time.Now(), Length: 200})

	// Drain 2 events
	<-m.Packets()
	<-m.Packets()

	stats := m.Stats()
	if stats.PacketsTotal != 2 {
		t.Fatalf("got PacketsTotal %d, want 2", stats.PacketsTotal)
	}
	if stats.BytesTotal != 300 {
		t.Fatalf("got BytesTotal %d, want 300", stats.BytesTotal)
	}
}
