// Package capture provides the network packet capture subsystem.
//
// The PacketCapture interface is the abstraction over OS-specific packet
// capture mechanisms. Plan 2 ships two implementations: Npcap on Windows
// (via gopacket with pcap handle) and AF_PACKET on Linux (via gopacket with
// AF_PACKET socket). A mock implementation is provided for unit tests.
package capture

import "time"

// PacketEvent is a single captured packet's parsed metadata. The full packet
// bytes are written to a pcap file by PcapWriter; only a short preview is
// retained in memory here.
type PacketEvent struct {
	Timestamp      time.Time
	SrcMAC         string
	DstMAC         string
	SrcIP          string
	DstIP          string
	SrcPort        uint16
	DstPort        uint16
	Protocol       string // "TCP", "UDP", "ICMP", "OTHER"
	Length         int
	PayloadPreview []byte // truncated to N bytes (configurable)
	Hostname       string // populated by DHCP or mDNS
	IsDiscovery    bool   // true if this packet contains discovery info (ARP/DHCP/mDNS)
}

// CaptureStats reports the current state of a PacketCapture implementation.
// Values are monotonic counters; LastError is the last error encountered.
type CaptureStats struct {
	PacketsTotal   uint64
	PacketsDropped uint64
	BytesTotal     uint64
	StartedAt      time.Time
	LastError      error
}

// PacketCapture is the interface every capture driver implements. The caller
// (main.go or a supervisor goroutine) starts the capture, then reads events
// from Packets() and queries stats periodically.
type PacketCapture interface {
	// Start begins capture on the given network interface. An empty
	// interface name means "auto-detect" (driver-specific). bpfFilter is
	// an optional Berkeley Packet Filter expression (empty = capture all).
	Start(iface string, bpfFilter string) error

	// Stop halts capture and closes the Packets() channel.
	Stop() error

	// Stats returns a snapshot of the current capture state.
	Stats() CaptureStats

	// Packets returns a receive-only channel of captured packets. The
	// channel is closed when Stop() is called.
	Packets() <-chan PacketEvent
}
