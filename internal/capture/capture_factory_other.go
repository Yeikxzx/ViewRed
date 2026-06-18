//go:build !windows && !linux

package capture

// NewCaptureForOS returns the platform-appropriate PacketCapture
// implementation. On unsupported OSes we fall back to the in-memory
// MockCapture so the binary can still be started and the HTTP /health
// endpoint exercised on developer machines that do not have a packet
// capture driver installed.
//
// Build-tagged variant for "other" OSes (darwin, freebsd, etc.); Windows
// and Linux variants live in capture_factory_windows.go and
// capture_factory_linux.go respectively.
func NewCaptureForOS() PacketCapture {
	return NewMockCapture()
}
