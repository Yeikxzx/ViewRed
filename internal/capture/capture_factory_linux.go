//go:build linux

package capture

// NewCaptureForOS returns the platform-appropriate PacketCapture
// implementation. On Linux we use AF_PACKET sockets via gopacket.
//
// Build-tagged variant for Linux; Windows and other-OS variants live in
// capture_factory_windows.go and capture_factory_other.go respectively.
func NewCaptureForOS() PacketCapture {
	return NewAFPACKETCapture()
}
