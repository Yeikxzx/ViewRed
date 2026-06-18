package capture

import (
	"testing"
	"time"
)

func TestPacketEventHasExpectedFields(t *testing.T) {
	e := PacketEvent{
		Timestamp:      time.Now(),
		SrcMAC:         "aa:bb:cc:dd:ee:ff",
		DstMAC:         "11:22:33:44:55:66",
		SrcIP:          "192.168.1.10",
		DstIP:          "8.8.8.8",
		SrcPort:        443,
		DstPort:        12345,
		Protocol:       "TCP",
		Length:         1500,
		PayloadPreview: []byte{0xde, 0xad, 0xbe, 0xef},
	}
	if e.SrcIP != "192.168.1.10" {
		t.Fatalf("SrcIP mismatch: got %q", e.SrcIP)
	}
	if e.Protocol != "TCP" {
		t.Fatalf("Protocol mismatch: got %q", e.Protocol)
	}
	if len(e.PayloadPreview) != 4 {
		t.Fatalf("PayloadPreview length: got %d", len(e.PayloadPreview))
	}
}

func TestCaptureStatsZeroValue(t *testing.T) {
	s := CaptureStats{}
	if s.PacketsTotal != 0 || s.PacketsDropped != 0 || s.BytesTotal != 0 {
		t.Fatal("zero value should be all zeros")
	}
	if s.LastError != nil {
		t.Fatalf("LastError should be nil, got %v", s.LastError)
	}
}
