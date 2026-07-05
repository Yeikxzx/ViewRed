//go:build linux

package capture

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
)

// AFPacketCapture is the Linux implementation of PacketCapture. It uses
// gopacket's pcap subpackage which, on Linux, opens an AF_PACKET socket
// via libpcap. This is the kernel-level capture path — no userspace
// copy beyond what the kernel already does for packet sockets.
//
// Runtime requirements:
//   - libpcap installed (apt: libpcap-dev / rpm: libpcap-devel)
//   - root or CAP_NET_RAW capability
type AFPacketCapture struct {
	iface   string
	filter  string
	handle  *pcap.Handle
	packets chan PacketEvent
	stats   CaptureStats
	stopCh  chan struct{}
	running bool
}

// NewAFPACKETCapture creates a new AFPacketCapture. Capture is not
// started until Start is called.
func NewAFPACKETCapture() *AFPacketCapture {
	return &AFPacketCapture{
		packets: make(chan PacketEvent, 4096),
		stopCh:  make(chan struct{}),
	}
}

// Start opens the device and begins capture in a background goroutine.
// iface is the interface name (e.g. "eth0", "wlan0", "lo"). bpfFilter
// is an optional Berkeley Packet Filter expression; pass "" to capture
// all frames.
func (c *AFPacketCapture) Start(iface string, bpfFilter string) error {
	if c.running {
		return errors.New("afpacket: already running")
	}
	if iface == "" {
		return errors.New("afpacket: empty interface")
	}

	// 65535 snaplen, promiscuous mode, and a long timeout to avoid
	// busy polling. pcap.BlockForever matches the Windows driver.
	handle, err := pcap.OpenLive(iface, 65535, true, pcap.BlockForever)
	if err != nil {
		return fmt.Errorf("afpacket: open %s: %w", iface, err)
	}

	if bpfFilter != "" {
		if err := handle.SetBPFFilter(bpfFilter); err != nil {
			handle.Close()
			return fmt.Errorf("afpacket: set BPF: %w", err)
		}
	}

	c.iface = iface
	c.filter = bpfFilter
	c.handle = handle
	c.stats.StartedAt = time.Now()
	c.stats.LastError = nil // clear stale error from prior run
	c.stats.Running = true
	c.running = true

	go c.readLoop(c.stopCh)
	return nil
}

// readLoop pulls packets from gopacket and emits PacketEvent values
// until Stop is called. It is the only place that touches c.handle.
// The gopacket.Packet → PacketEvent conversion is delegated to
// packetToEvent in packet_event.go so the same logic runs on both
// Windows and Linux.
func (c *AFPacketCapture) readLoop(stopCh <-chan struct{}) {
	src := gopacket.NewPacketSource(c.handle, c.handle.LinkType())
	for {
		select {
		case <-stopCh:
			return
		default:
		}
		packet, err := src.NextPacket()
		if err != nil {
			// If we got an error because Stop() closed the handle, exit immediately
			select {
			case <-stopCh:
				return
			default:
			}
			c.stats.LastError = err
			c.stats.PacketsDropped++
			continue
		}
		c.stats.PacketsTotal++
		c.stats.BytesTotal += uint64(packet.Metadata().CaptureLength)

		event := packetToEvent(packet)
		select {
		case c.packets <- event:
		case <-stopCh:
			return
		default:
			// Channel full: drop the packet rather than block
			// the read loop, which would starve the kernel
			// ring buffer and cause pcap itself to drop.
			c.stats.PacketsDropped++
		}
	}
}

// Stop halts capture, closes the device, and signals the read loop to
// exit. The Packets channel is left open so the caller can drain any
// buffered events; closing it from inside Stop would race with the
// read loop.
func (c *AFPacketCapture) Stop() error {
	if !c.running {
		return errors.New("afpacket: not running")
	}
	c.running = false
	c.stats.Running = false
	close(c.stopCh)
	if c.handle != nil {
		c.handle.Close()
	}
	return nil
}

// Stats returns a snapshot of the current capture state. The struct is
// returned by value, so the caller cannot mutate our counters.
func (c *AFPacketCapture) Stats() CaptureStats { return c.stats }

// Packets returns the receive-only event channel.
func (c *AFPacketCapture) Packets() <-chan PacketEvent { return c.packets }

// Compile-time check: AFPacketCapture implements PacketCapture.
var _ PacketCapture = (*AFPacketCapture)(nil)
