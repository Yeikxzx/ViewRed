package capture

import (
	"strings"

	"github.com/google/gopacket/pcap"
)

// isVirtualInterface returns true when the interface name or description
// indicates it is a virtual adapter that is unlikely to carry real user
// traffic (Hyper-V, Wi-Fi Direct, loopback, etc.).
func isVirtualInterface(name, description string) bool {
	lower := strings.ToLower(name + " " + description)
	for _, keyword := range []string{
		"hyper-v",
		"virtual",
		"loopback",
		"wi-fi direct",
	} {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

// AutoDetectInterface returns the most likely physical network interface.
// It filters out virtual adapters (Hyper-V, Wi-Fi Direct, loopback) and
// prefers the first remaining interface that has at least one assigned IP
// address. Falls back to the raw pcap.FindAllDevs() order when every
// interface is virtual.
//
// The empty string is returned when pcap.FindAllDevs fails entirely.
func AutoDetectInterface() string {
	devs, err := pcap.FindAllDevs()
	if err != nil || len(devs) == 0 {
		return ""
	}

	// First pass: collect physical (non-virtual) interfaces.
	var physical []pcap.Interface
	for _, d := range devs {
		if !isVirtualInterface(d.Name, d.Description) {
			physical = append(physical, d)
		}
	}

	// If every interface is virtual, fall back to the original
	// auto-detect behaviour so the user can at least pick manually.
	if len(physical) == 0 {
		physical = devs
	}

	// Prefer the first physical interface with an assigned IP address.
	for _, d := range physical {
		if len(d.Addresses) > 0 {
			return d.Name
		}
	}

	// No IP addresses on any physical interface — return the first one.
	return physical[0].Name
}
