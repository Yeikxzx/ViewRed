package capture

import "github.com/google/gopacket"

// ToCaptureInfo converts a PacketEvent to the gopacket.CaptureInfo value
// that pcapgo.Writer.WritePacket expects.
//
// In plan 2 we only retain a short PayloadPreview (up to payloadPreviewBytes
// bytes of the application layer), so CaptureLength is the length of the
// preview. Length is the original on-wire frame length reported by the
// capture driver. Future plans may reconstruct the full frame from the
// parsed metadata and update CaptureLength to match.
//
// The conversion is intentionally cheap: it copies the timestamp and two
// ints, no allocations. This matters because it runs in the per-packet
// hot path when the main goroutine pipes events to the pcap writer.
func ToCaptureInfo(e PacketEvent) gopacket.CaptureInfo {
	return gopacket.CaptureInfo{
		Timestamp:      e.Timestamp,
		CaptureLength:  len(e.PayloadPreview),
		Length:         e.Length,
	}
}

// PayloadBytes returns the bytes to write to the pcap file for a given
// PacketEvent. In plan 2 this is the retained PayloadPreview; a future
// plan may rebuild the full Ethernet frame from the parsed metadata
// (MACs, IPs, ports, transport header) and return that instead.
//
// The returned slice is the same backing array as PacketEvent.PayloadPreview
// — callers should not mutate it after handing it to the writer.
func PayloadBytes(e PacketEvent) []byte {
	return e.PayloadPreview
}
