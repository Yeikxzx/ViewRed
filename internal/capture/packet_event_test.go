package capture

import (
	"net"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// buildPacket constructs a synthetic gopacket.Packet for testing the
// packetToEvent conversion helper. linkType lets us pick Ethernet or
// raw-IPv4 (loopback) framing.
//
// gopacket.NewPacket does NOT populate Metadata() — that is set by
// PacketSource.NextPacket in production. We mimic that here so the
// resulting Packet behaves the same way packets from a live source do.
func buildPacket(t *testing.T, linkType layers.LinkType, payload []byte) gopacket.Packet {
	t.Helper()

	// Ethernet header: src/dst MAC + EtherType IPv4
	eth := &layers.Ethernet{
		SrcMAC:       net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0x01},
		DstMAC:       net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0x02},
		EthernetType: layers.EthernetTypeIPv4,
	}
	ip := &layers.IPv4{
		Version:  4,
		IHL:      5,
		SrcIP:    net.ParseIP("10.0.0.1"),
		DstIP:    net.ParseIP("10.0.0.2"),
		Protocol: layers.IPProtocolTCP,
		TTL:      64,
	}
	tcp := &layers.TCP{
		SrcPort: 443,
		DstPort: 50000,
	}
	// TCP/UDP need to know their network layer for checksum computation.
	tcp.SetNetworkLayerForChecksum(ip)

	// Serialize layers bottom-up. gopacket's SerializeBuffers is the
	// canonical way to compute checksums and lengths.
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}
	if err := gopacket.SerializeLayers(buf, opts, eth, ip, tcp, gopacket.Payload(payload)); err != nil {
		t.Fatalf("SerializeLayers: %v", err)
	}
	pkt := gopacket.NewPacket(buf.Bytes(), linkType, gopacket.NoCopy)
	pkt.Metadata().Timestamp = time.Now()
	pkt.Metadata().CaptureLength = len(buf.Bytes())
	pkt.Metadata().Length = len(buf.Bytes())
	return pkt
}

func TestPacketToEvent_EthernetIPv4TCP(t *testing.T) {
	pkt := buildPacket(t, layers.LinkTypeEthernet, []byte("hello"))

	ev := packetToEvent(pkt)

	if ev.SrcMAC != "aa:bb:cc:dd:ee:01" {
		t.Fatalf("SrcMAC: got %q", ev.SrcMAC)
	}
	if ev.DstMAC != "aa:bb:cc:dd:ee:02" {
		t.Fatalf("DstMAC: got %q", ev.DstMAC)
	}
	if ev.SrcIP != "10.0.0.1" {
		t.Fatalf("SrcIP: got %q", ev.SrcIP)
	}
	if ev.DstIP != "10.0.0.2" {
		t.Fatalf("DstIP: got %q", ev.DstIP)
	}
	if ev.SrcPort != 443 {
		t.Fatalf("SrcPort: got %d", ev.SrcPort)
	}
	if ev.DstPort != 50000 {
		t.Fatalf("DstPort: got %d", ev.DstPort)
	}
	if ev.Protocol != "TCP" {
		t.Fatalf("Protocol: got %q, want TCP", ev.Protocol)
	}
	if ev.Length == 0 {
		t.Fatalf("Length should be > 0, got %d", ev.Length)
	}
	if string(ev.PayloadPreview) != "hello" {
		t.Fatalf("PayloadPreview: got %q, want %q", ev.PayloadPreview, "hello")
	}
	if ev.Timestamp.IsZero() {
		t.Fatalf("Timestamp should be non-zero")
	}
}

func TestPacketToEvent_UDP(t *testing.T) {
	eth := &layers.Ethernet{
		SrcMAC:       net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0x01},
		DstMAC:       net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0x02},
		EthernetType: layers.EthernetTypeIPv4,
	}
	ip := &layers.IPv4{
		Version:  4,
		IHL:      5,
		SrcIP:    net.ParseIP("192.168.1.10"),
		DstIP:    net.ParseIP("8.8.8.8"),
		Protocol: layers.IPProtocolUDP,
		TTL:      64,
	}
	udp := &layers.UDP{SrcPort: 53, DstPort: 33333}
	udp.SetNetworkLayerForChecksum(ip)
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}
	if err := gopacket.SerializeLayers(buf, opts, eth, ip, udp, gopacket.Payload([]byte("dns-query"))); err != nil {
		t.Fatalf("SerializeLayers: %v", err)
	}
	pkt := gopacket.NewPacket(buf.Bytes(), layers.LinkTypeEthernet, gopacket.NoCopy)
	pkt.Metadata().Timestamp = time.Now()
	pkt.Metadata().CaptureLength = len(buf.Bytes())
	pkt.Metadata().Length = len(buf.Bytes())

	ev := packetToEvent(pkt)
	if ev.Protocol != "UDP" {
		t.Fatalf("Protocol: got %q, want UDP", ev.Protocol)
	}
	if ev.SrcPort != 53 {
		t.Fatalf("SrcPort: got %d, want 53", ev.SrcPort)
	}
	if ev.DstPort != 33333 {
		t.Fatalf("DstPort: got %d, want 33333", ev.DstPort)
	}
}

func TestPacketToEvent_ICMP(t *testing.T) {
	// Build an Ethernet/IPv4/ICMP packet.
	eth := &layers.Ethernet{
		SrcMAC:       net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x66},
		DstMAC:       net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
		EthernetType: layers.EthernetTypeIPv4,
	}
	ip := &layers.IPv4{
		Version:  4,
		IHL:      5,
		SrcIP:    net.ParseIP("10.0.0.1"),
		DstIP:    net.ParseIP("10.0.0.2"),
		Protocol: layers.IPProtocolICMPv4,
		TTL:      64,
	}
	icmp := &layers.ICMPv4{
		TypeCode: layers.CreateICMPv4TypeCode(layers.ICMPv4TypeEchoRequest, 0),
	}
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}
	if err := gopacket.SerializeLayers(buf, opts, eth, ip, icmp); err != nil {
		t.Fatalf("SerializeLayers: %v", err)
	}
	pkt := gopacket.NewPacket(buf.Bytes(), layers.LinkTypeEthernet, gopacket.NoCopy)
	pkt.Metadata().Timestamp = time.Now()
	pkt.Metadata().CaptureLength = len(buf.Bytes())
	pkt.Metadata().Length = len(buf.Bytes())

	ev := packetToEvent(pkt)
	if ev.Protocol != "ICMP" {
		t.Fatalf("Protocol: got %q, want ICMP", ev.Protocol)
	}
}

func TestPacketToEvent_OtherProtocol(t *testing.T) {
	// An Ethernet frame with no L3 should yield Protocol "OTHER".
	eth := &layers.Ethernet{
		SrcMAC:       net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0x01},
		DstMAC:       net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0x02},
		EthernetType: layers.EthernetTypeARP,
	}
	arp := &layers.ARP{
		AddrType:          layers.LinkTypeEthernet,
		Protocol:          layers.EthernetTypeIPv4,
		HwAddressSize:     6,
		ProtAddressSize:   4,
		Operation:         1, // request
		SourceHwAddress:   []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0x01},
		SourceProtAddress: []byte{10, 0, 0, 1},
		DstHwAddress:      []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0x02},
		DstProtAddress:    []byte{10, 0, 0, 2},
	}
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}
	if err := gopacket.SerializeLayers(buf, opts, eth, arp); err != nil {
		t.Fatalf("SerializeLayers: %v", err)
	}
	pkt := gopacket.NewPacket(buf.Bytes(), layers.LinkTypeEthernet, gopacket.NoCopy)
	pkt.Metadata().Timestamp = time.Now()
	pkt.Metadata().CaptureLength = len(buf.Bytes())
	pkt.Metadata().Length = len(buf.Bytes())

	ev := packetToEvent(pkt)
	if ev.Protocol != "OTHER" {
		t.Fatalf("Protocol: got %q, want OTHER", ev.Protocol)
	}
	if ev.SrcIP != "" || ev.DstIP != "" {
		t.Fatalf("IP fields should be empty, got SrcIP=%q DstIP=%q", ev.SrcIP, ev.DstIP)
	}
}

func TestPacketToEvent_PayloadPreviewTruncates(t *testing.T) {
	big := make([]byte, 200)
	for i := range big {
		big[i] = byte(i % 256)
	}
	pkt := buildPacket(t, layers.LinkTypeEthernet, big)

	ev := packetToEvent(pkt)
	const previewLen = 64
	if len(ev.PayloadPreview) != previewLen {
		t.Fatalf("PayloadPreview length: got %d, want %d", len(ev.PayloadPreview), previewLen)
	}
	for i, b := range ev.PayloadPreview {
		if b != byte(i%256) {
			t.Fatalf("PayloadPreview[%d]: got %d, want %d", i, b, i%256)
			break
		}
	}
}

func TestPacketToEvent_TimestampPreserved(t *testing.T) {
	pkt := buildPacket(t, layers.LinkTypeEthernet, []byte("x"))
	ts := pkt.Metadata().Timestamp
	if ts.IsZero() {
		t.Fatal("packet timestamp should be non-zero from buildPacket")
	}
	ev := packetToEvent(pkt)
	if !ev.Timestamp.Equal(ts) {
		t.Fatalf("Timestamp: got %v, want %v", ev.Timestamp, ts)
	}
	if ev.Length != pkt.Metadata().CaptureLength {
		t.Fatalf("Length: got %d, want %d", ev.Length, pkt.Metadata().CaptureLength)
	}
}

func TestPacketToEvent_NoApplicationLayer(t *testing.T) {
	// Build a SYN packet (no payload).
	eth := &layers.Ethernet{
		SrcMAC:       net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0x01},
		DstMAC:       net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0x02},
		EthernetType: layers.EthernetTypeIPv4,
	}
	ip := &layers.IPv4{
		Version:  4,
		IHL:      5,
		SrcIP:    net.ParseIP("10.0.0.1"),
		DstIP:    net.ParseIP("10.0.0.2"),
		Protocol: layers.IPProtocolTCP,
		TTL:      64,
	}
	tcp := &layers.TCP{SrcPort: 1234, DstPort: 80, SYN: true}
	tcp.SetNetworkLayerForChecksum(ip)
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}
	if err := gopacket.SerializeLayers(buf, opts, eth, ip, tcp); err != nil {
		t.Fatalf("SerializeLayers: %v", err)
	}
	pkt := gopacket.NewPacket(buf.Bytes(), layers.LinkTypeEthernet, gopacket.NoCopy)
	pkt.Metadata().Timestamp = time.Now()
	pkt.Metadata().CaptureLength = len(buf.Bytes())
	pkt.Metadata().Length = len(buf.Bytes())

	ev := packetToEvent(pkt)
	if len(ev.PayloadPreview) != 0 {
		t.Fatalf("PayloadPreview should be empty for SYN, got %d bytes", len(ev.PayloadPreview))
	}
	if ev.Timestamp.IsZero() {
		t.Fatal("Timestamp should be non-zero")
	}
}
