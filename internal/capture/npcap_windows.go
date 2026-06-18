//go:build windows

package capture

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
)

// NpcapCapture is the Windows implementation of PacketCapture, using the
// Npcap driver (a Windows-compatible pcap implementation backed by
// libpcap's API surface).
//
// Runtime requirements:
//   - Npcap installed: https://npcap.com/
//   - admin privileges (Npcap loads a kernel driver)
type NpcapCapture struct {
	iface   string
	filter  string
	handle  *pcap.Handle
	packets chan PacketEvent
	stats   CaptureStats
	stopCh  chan struct{}
	running bool
}

// NewNpcapCapture creates a new NpcapCapture. Capture is not started
// until Start is called.
func NewNpcapCapture() *NpcapCapture {
	return &NpcapCapture{
		packets: make(chan PacketEvent, 4096),
		stopCh:  make(chan struct{}),
	}
}

// Start opens the device and begins capture in a background goroutine.
// iface is the Npcap device name (e.g. "\\Device\\NPF_{...}" or a
// friendly name like "Ethernet"). bpfFilter is an optional Berkeley
// Packet Filter expression; pass "" to capture all frames.
func (c *NpcapCapture) Start(iface string, bpfFilter string) error {
	if c.running {
		return errors.New("npcap: already running")
	}
	if iface == "" {
		return errors.New("npcap: empty interface")
	}

	handle, err := pcap.OpenLive(iface, 65535, true, pcap.BlockForever)
	if err != nil {
		return fmt.Errorf("npcap: open %s: %w", iface, err)
	}

	if bpfFilter != "" {
		if err := handle.SetBPFFilter(bpfFilter); err != nil {
			handle.Close()
			return fmt.Errorf("npcap: set BPF: %w", err)
		}
	}

	c.iface = iface
	c.filter = bpfFilter
	c.handle = handle
	c.stats.StartedAt = time.Now()
	c.running = true

	go c.readLoop()
	return nil
}

// readLoop pulls packets from gopacket and emits PacketEvent values
// until Stop is called. It is the only place that touches c.handle.
func (c *NpcapCapture) readLoop() {
	src := gopacket.NewPacketSource(c.handle, c.handle.LinkType())
	for {
		select {
		case <-c.stopCh:
			return
		default:
		}
		packet, err := src.NextPacket()
		if err != nil {
			c.stats.LastError = err
			c.stats.PacketsDropped++
			continue
		}
		c.stats.PacketsTotal++
		c.stats.BytesTotal += uint64(packet.Metadata().CaptureLength)

		event := packetToEvent(packet)
		select {
		case c.packets <- event:
		case <-c.stopCh:
			return
		default:
			// Channel full: drop the packet rather than block
			// the read loop, which would starve the kernel
			// ring buffer and cause Npcap itself to drop.
			c.stats.PacketsDropped++
		}
	}
}

// Stop halts capture, closes the device, and signals the read loop to
// exit. The Packets channel is left open so the caller can drain any
// buffered events; closing it from inside Stop would race with the
// read loop.
func (c *NpcapCapture) Stop() error {
	if !c.running {
		return errors.New("npcap: not running")
	}
	c.running = false
	close(c.stopCh)
	if c.handle != nil {
		c.handle.Close()
	}
	return nil
}

// Stats returns a snapshot of the current capture state. The struct is
// returned by value, so the caller cannot mutate our counters.
func (c *NpcapCapture) Stats() CaptureStats { return c.stats }

// Packets returns the receive-only event channel.
func (c *NpcapCapture) Packets() <-chan PacketEvent { return c.packets }

// Compile-time check: NpcapCapture implements PacketCapture.
var _ PacketCapture = (*NpcapCapture)(nil)
