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

// DNSAnswer is one A or AAAA record observed in a DNS response.
// The capture layer extracts these from every DNS packet and the
// resolver package (internal/resolver) consumes them to build a
// process-local IP→hostname cache. We carry the answer on the
// PacketEvent itself rather than calling into the resolver
// directly so the capture pipeline stays single-purpose and the
// resolver is testable in isolation.
type DNSAnswer struct {
	// IP is the resolved address, in dotted-quad ("1.2.3.4") or
	// canonical IPv6 form. Always set; the capture skips empty
	// values rather than emitting "ip"="" entries.
	IP string
	// Hostname is the canonical name the client asked for. For
	// CNAME chains the resolver would normally walk to the
	// final A record's name, but we use the question's name
	// when present so a YouTube request shows up as
	// "youtube.com" rather than "youtube.com.cdn" or
	// "googlevideo.com". Falls back to the answer's own name
	// when the response carries no question.
	Hostname string
	// TTL is the DNS answer's time-to-live in seconds. The
	// resolver clamps this to its own max; the raw value is
	// preserved here for tests and observability.
	TTL uint32
}

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
		e.TCPSyn = tcpL.SYN
		e.TCPAck = tcpL.ACK
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

	// Extract Discovery Hostnames (DHCP / mDNS) and DNS A/AAAA
	// answers for the IP→hostname cache. The same DNS layer is
	// the source for both: DHCP packets embed the hostname in
	// their options, mDNS answers carry PTR records, and
	// regular DNS responses carry the A/AAAA records that the
	// resolver package needs.
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

		// mDNS PTR (port 5353) — a device announcing its own
		// hostname. We surface this on the PacketEvent as a
		// discovery hostname (existing behaviour) and ignore
		// it as a service-resolution candidate: a PTR for
		// "my-laptop.local" doesn't tell us what the laptop
		// is talking to.
		if e.DstPort == 5353 {
			for _, answer := range dnsL.Answers {
				if answer.Type == layers.DNSTypePTR {
					e.Hostname = string(answer.PTR)
					e.IsDiscovery = true
					break
				}
			}
		}

		// Regular DNS responses (QR=1) — extract every A/AAAA
		// record so the resolver can map the IP to the queried
		// hostname. We use the question's name as the canonical
		// name because a CNAME chain would otherwise point the
		// final A record at the CNAME target ("cdn.example.com")
		// rather than the user-visible name ("example.com").
		if dnsL.QR && len(dnsL.Answers) > 0 {
			var canonical string
			if len(dnsL.Questions) > 0 {
				canonical = string(dnsL.Questions[0].Name)
			}
			for _, ans := range dnsL.Answers {
				if ans.Type != layers.DNSTypeA && ans.Type != layers.DNSTypeAAAA {
					continue
				}
				ip := ans.IP.String()
				name := canonical
				if name == "" {
					name = string(ans.Name)
				}
				if ip != "" && name != "" {
					e.DNSAnswers = append(e.DNSAnswers, DNSAnswer{
						IP:       ip,
						Hostname: name,
						TTL:      ans.TTL,
					})
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
