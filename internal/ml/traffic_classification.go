package ml

import "github.com/viewred/viewred/internal/capture"

var commonClientBoundServicePorts = map[uint16]bool{
	80:   true,
	443:  true,
	8080: true,
	8443: true,
}

func isLikelyClientBoundServiceResponse(pkt capture.PacketEvent) bool {
	return pkt.Protocol == "TCP" &&
		commonClientBoundServicePorts[pkt.SrcPort] &&
		pkt.DstPort >= 1024
}
