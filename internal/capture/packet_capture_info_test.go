package capture

import (
	"bytes"
	"testing"
	"time"

	"github.com/google/gopacket"
)

// TestToCaptureInfoMapsFields verifies that ToCaptureInfo preserves the
// fields of a PacketEvent that gopacket needs to write a pcap record:
// timestamp, captured length (the preview size, since we only retain a
// short payload preview per packet in plan 2) and full length.
func TestToCaptureInfoMapsFields(t *testing.T) {
	ts := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	preview := []byte{0xde, 0xad, 0xbe, 0xef, 0xca, 0xfe}
	e := PacketEvent{
		Timestamp:      ts,
		Length:         1500,
		PayloadPreview: preview,
	}

	ci := ToCaptureInfo(e)

	if !ci.Timestamp.Equal(ts) {
		t.Fatalf("Timestamp: got %v, want %v", ci.Timestamp, ts)
	}
	if ci.CaptureLength != len(preview) {
		t.Fatalf("CaptureLength: got %d, want %d", ci.CaptureLength, len(preview))
	}
	if ci.Length != 1500 {
		t.Fatalf("Length: got %d, want 1500", ci.Length)
	}
}

// TestToCaptureInfoEmptyPreview verifies that an empty PayloadPreview maps
// to a zero CaptureLength (so pcapgo writes a record with no captured payload).
func TestToCaptureInfoEmptyPreview(t *testing.T) {
	e := PacketEvent{
		Timestamp: time.Now(),
		Length:    64,
		// PayloadPreview intentionally nil
	}
	ci := ToCaptureInfo(e)
	if ci.CaptureLength != 0 {
		t.Fatalf("CaptureLength: got %d, want 0 for empty preview", ci.CaptureLength)
	}
	if ci.Length != 64 {
		t.Fatalf("Length: got %d, want 64", ci.Length)
	}
}

// TestPayloadBytesReturnsPreview verifies the helper exposes the same bytes
// the writer should persist to the pcap file. In plan 2 we only retain a
// short preview; future plans may reconstruct the full frame.
func TestPayloadBytesReturnsPreview(t *testing.T) {
	preview := []byte{0x01, 0x02, 0x03, 0x04}
	e := PacketEvent{PayloadPreview: preview}

	got := PayloadBytes(e)
	if !bytes.Equal(got, preview) {
		t.Fatalf("PayloadBytes: got %v, want %v", got, preview)
	}
}

// TestPayloadBytesNilIsEmpty asserts the nil-preview case is safe and
// returns nil (not a panic or an empty-but-non-nil slice).
func TestPayloadBytesNilIsEmpty(t *testing.T) {
	e := PacketEvent{}
	got := PayloadBytes(e)
	if got != nil {
		t.Fatalf("PayloadBytes: got %v, want nil", got)
	}
}

// TestToCaptureInfoIsValueType is a compile-time-ish check that the helper
// returns a gopacket.CaptureInfo value (not a pointer), which matches the
// pcapgo.Writer.WritePacket signature. If the return type ever changes to a
// pointer this test will fail to compile.
func TestToCaptureInfoIsValueType(t *testing.T) {
	var _ func(PacketEvent) gopacket.CaptureInfo = ToCaptureInfo
}
