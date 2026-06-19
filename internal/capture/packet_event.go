package capture

import (
	"net"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// payloadPreviewBytes is the maximum number of payload bytes retained in
// PacketEvent.PayloadPreview. We keep this small so the event stays
// cheap to allocate even at line rate.
const payloadPreviewBytes = 64

// packetToEvent converts a gopacket.Packet to our canonical PacketEvent
// struct. It is platform-agnostic so the same conversion is used by the
// Npcap (Windows) and AF_PACKET (Linux) drivers.
//
// The caller is expected to have a fully-decoded packet whose Metadata()
// is populated (e.g. one returned by gopacket.PacketSource.NextPacket in
// the production path). If Metadata() is empty, the resulting PacketEvent
// will have a zero Timestamp and Length — this is a deliberate no-op, not
// an error, so the same code works in tests that hand-construct packets.
func packetToEvent(p gopacket.Packet) PacketEvent {
	e := PacketEvent{
		Timestamp: p.Metadata().Timestamp,
		Length:    int(p.Metadata().CaptureLength),
	}

	if eth := p.Layer(layers.LayerTypeEthernet); eth != nil {
		ethL := eth.(*layers.Ethernet)
		e.SrcMAC = ethL.SrcMAC.String()
		e.DstMAC = ethL.DstMAC.String()
	}
	if ip4 := p.Layer(layers.LayerTypeIPv4); ip4 != nil {
		ip4L := ip4.(*layers.IPv4)
		e.SrcIP = ip4L.SrcIP.String()
		e.DstIP = ip4L.DstIP.String()
	} else if arpLayer := p.Layer(layers.LayerTypeARP); arpLayer != nil {
		arpL := arpLayer.(*layers.ARP)
		e.SrcIP = net.IP(arpL.SourceProtAddress).String()
		e.DstIP = net.IP(arpL.DstProtAddress).String()
		e.IsDiscovery = true
		e.Protocol = "ARP"
	}
	if tcp := p.Layer(layers.LayerTypeTCP); tcp != nil {
		tcpL := tcp.(*layers.TCP)
		e.SrcPort = uint16(tcpL.SrcPort)
		e.DstPort = uint16(tcpL.DstPort)
		e.Protocol = "TCP"
	}
	if udp := p.Layer(layers.LayerTypeUDP); udp != nil {
		udpL := udp.(*layers.UDP)
		e.SrcPort = uint16(udpL.SrcPort)
		e.DstPort = uint16(udpL.DstPort)
		e.Protocol = "UDP"
	}
	if e.Protocol == "" {
		if p.Layer(layers.LayerTypeICMPv4) != nil {
			e.Protocol = "ICMP"
		} else {
			e.Protocol = "OTHER"
		}
	}

	// Extract Discovery Hostnames (DHCP / mDNS)
	if dhcpLayer := p.Layer(layers.LayerTypeDHCPv4); dhcpLayer != nil {
		dhcpL := dhcpLayer.(*layers.DHCPv4)
		for _, opt := range dhcpL.Options {
			if opt.Type == layers.DHCPOptHostname {
				e.Hostname = string(opt.Data)
				e.IsDiscovery = true
				break
			}
		}
	} else if dnsLayer := p.Layer(layers.LayerTypeDNS); dnsLayer != nil {
		dnsL := dnsLayer.(*layers.DNS)
		if e.DstPort == 5353 { // mDNS
			for _, answer := range dnsL.Answers {
				if answer.Type == layers.DNSTypePTR {
					e.Hostname = string(answer.PTR)
					e.IsDiscovery = true
					break
				}
			}
		}
	}

	// Preview the first payloadPreviewBytes bytes of the application
	// layer (HTTP, DNS, TLS, etc.). If the packet has no application
	// layer, the preview is nil.
	if payload := p.ApplicationLayer(); payload != nil {
		data := payload.Payload()
		if len(data) > payloadPreviewBytes {
			data = data[:payloadPreviewBytes]
		}
		e.PayloadPreview = data
	}

	return e
}
