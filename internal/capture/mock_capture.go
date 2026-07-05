package capture

import (
	"errors"
	"sync"
	"time"
)

// MockCapture is a test double for PacketCapture. It allows tests to push
// synthetic PacketEvent values into a channel and observe consumer behavior.
type MockCapture struct {
	mu      sync.Mutex
	ch      chan PacketEvent
	stats   CaptureStats
	running bool
	stopCh  chan struct{}
}

// NewMockCapture creates a new MockCapture with a buffered channel.
func NewMockCapture() *MockCapture {
	return &MockCapture{
		ch:     make(chan PacketEvent, 1024),
		stopCh: make(chan struct{}),
	}
}

// Start marks the mock as running. iface and bpfFilter are accepted but
// ignored (the mock doesn't actually capture).
func (m *MockCapture) Start(iface string, bpfFilter string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return errors.New("mock: already running")
	}
	m.running = true
	m.stats.Running = true
	m.stats.StartedAt = time.Now()
	return nil
}

// Stop marks the mock as not running and closes the Packets channel.
func (m *MockCapture) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return errors.New("mock: not running")
	}
	m.running = false
	m.stats.Running = false
	close(m.ch)
	return nil
}

// Stats returns a snapshot.
func (m *MockCapture) Stats() CaptureStats {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stats
}

// Packets returns the receive-only event channel.
func (m *MockCapture) Packets() <-chan PacketEvent { return m.ch }

// PushEvent enqueues an event for consumers. Panics if the channel is full
// or closed (callers should drain the channel in tests).
func (m *MockCapture) PushEvent(e PacketEvent) {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return // drop on the floor if not running
	}
	m.stats.PacketsTotal++
	m.stats.BytesTotal += uint64(e.Length)
	m.mu.Unlock()
	m.ch <- e
}
