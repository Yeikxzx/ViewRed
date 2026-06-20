package ml

import (
	"math"
	"strings"

	"github.com/viewred/viewred/internal/capture"
)

const (
	// entropySampleSize is the number of bytes used for payload entropy.
	// Truncating avoids wasting CPU on large payloads — the first 256
	// bytes are enough to distinguish text from binary or encrypted data.
	entropySampleSize = 256

	// numFeatures is the length of the feature vector. Every consumer
	// (engine.go tensor shape, train_models.py dimension) must match.
	numFeatures = 7
)

// FeatureIndex provides named constants for each position in the feature
// vector so the training script can reference them by name.
type FeatureIndex int

const (
	FILength          FeatureIndex = 0 // Packet length in bytes
	FIProtocol        FeatureIndex = 1 // 0=TCP, 1=UDP, 2=ICMP, 3=OTHER
	FISrcPort         FeatureIndex = 2 // Source port
	FIDstPort         FeatureIndex = 3 // Destination port
	FIPacketsPerSec   FeatureIndex = 4 // Packet rate for this src IP
	FIUniqueDstPorts  FeatureIndex = 5 // Unique dst ports for this src IP
	FIPayloadEntropy  FeatureIndex = 6 // Shannon entropy of payload
)

// ExtractFeatures converts a packet event and its current window context
// into a float32 slice for ONNX inference.
//
// Feature vector (7 elements):
//
//	[0] Length — packet size in bytes
//	[1] Proto_Idx — protocol index (0=TCP, 1=UDP, 2=ICMP, 3=OTHER)
//	[2] SrcPort — source port (0 if none)
//	[3] DstPort — destination port (0 if none)
//	[4] PacketsPerSec — packet rate from src IP over last 60s
//	[5] UniqueDstPorts — distinct dst ports contacted by src IP in window
//	[6] PayloadEntropy — Shannon entropy of first 256 payload bytes (0 if empty)
func ExtractFeatures(pkt capture.PacketEvent, ws *WindowStats) []float32 {
	features := make([]float32, numFeatures)

	features[FIProtocol] = protoIdx(pkt.Protocol)
	features[FISrcPort] = float32(pkt.SrcPort)
	features[FIDstPort] = float32(pkt.DstPort)
	features[FILength] = float32(pkt.Length)

	if ws != nil {
		features[FIPacketsPerSec] = float32(ws.PacketsPerSec(pkt.SrcIP))
		features[FIUniqueDstPorts] = float32(ws.UniqueDstPorts(pkt.SrcIP))
	}

	features[FIPayloadEntropy] = float32(PayloadEntropy(pkt.PayloadPreview))

	return features
}

// protoIdx maps a protocol string to a float32 index.
func protoIdx(proto string) float32 {
	switch strings.ToUpper(proto) {
	case "TCP":
		return 0
	case "UDP":
		return 1
	case "ICMP", "ICMPV4", "ICMPV6":
		return 2
	default:
		return 3
	}
}

// PayloadEntropy computes the Shannon entropy of the first N bytes of a
// payload. High entropy (~7-8) suggests encrypted or compressed data; low
// entropy (~1-4) suggests plaintext or structured data; zero means no
// payload. This is useful for detecting tunnelling or data exfiltration.
func PayloadEntropy(data []byte) float64 {
	n := len(data)
	if n == 0 {
		return 0
	}
	if n > entropySampleSize {
		n = entropySampleSize
		data = data[:n]
	}

	var freq [256]int
	for _, b := range data {
		freq[b]++
	}

	var entropy float64
	for _, count := range freq {
		if count == 0 {
			continue
		}
		p := float64(count) / float64(n)
		entropy -= p * math.Log2(p)
	}
	return entropy
}
