//go:build windows

package capture

// NewCaptureForOS returns the platform-appropriate PacketCapture
// implementation. On Windows we use the Npcap driver, which is backed
// by libpcap's API surface.
//
// Build-tagged variant for Windows; Linux and other-OS variants live in
// capture_factory_linux.go and capture_factory_other.go respectively.
func NewCaptureForOS() PacketCapture {
	return NewNpcapCapture()
}
