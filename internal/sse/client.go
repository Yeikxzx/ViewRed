package sse

import (
	"context"
	"sync"
	"time"
)

// client is one SSE consumer's state inside the broker.
//
// The channel acts as a 64-slot ring buffer: the broker pushes events
// into it, the consumer (the HTTP SSE handler) reads from it. When the
// channel is full, the broker drops the OLDEST event to make room —
// preserving the most recent data, which is what live metrics need. A
// stale 30-second-old packet count is more harmful to a dashboard than
// a missing one.
//
// consecutiveDrops counts heartbeats that had to drop an old event to
// fit the new one. Three consecutive drops marks the client as stale:
// the consumer cannot possibly be reading, so the connection is
// presumed dead even if the consumer's context has not been cancelled.
type client struct {
	topics []string
	ch     chan Event
	ctx    context.Context

	mu               sync.Mutex
	lastActive       time.Time
	consecutiveDrops int
}

// newClient creates a client with the given subscription. bufferSize
// sets the channel capacity (the ring buffer depth). lastActive is
// initialised to the current time so a freshly-subscribed client is
// never immediately considered stale.
func newClient(ctx context.Context, topics []string, bufferSize int) *client {
	return &client{
		topics:     topics,
		ch:         make(chan Event, bufferSize),
		ctx:        ctx,
		lastActive: time.Now(),
	}
}

// subscribedTo reports whether the client wants events for the given
// topic. An empty topic matches every client (broadcast).
func (c *client) subscribedTo(topic string) bool {
	if topic == "" {
		return true
	}
	for _, t := range c.topics {
		if t == topic {
			return true
		}
	}
	return false
}

// tryDeliver pushes e into the client's channel. If the channel is
// full, it drops the OLDEST event first to make room. Returns true
// when the event was delivered without dropping (a signal that the
// consumer is keeping up); returns false when a drop was required.
//
// The split return value is what lets sendHeartbeats distinguish
// "consumer is healthy" from "consumer is slow / dead" without a
// separate ACK mechanism: a clean delivery means the channel had
// room, which means the consumer recently read.
func (c *client) tryDeliver(e Event) bool {
	select {
	case c.ch <- e:
		c.mu.Lock()
		c.lastActive = time.Now()
		c.consecutiveDrops = 0
		c.mu.Unlock()
		return true
	default:
		// Channel full — drop the oldest event. This is non-blocking:
		// if another goroutine already drained the channel between
		// the failed send and here, the receive is a no-op, which is
		// still safe (we just delivered without a drop).
		select {
		case <-c.ch:
		default:
		}
		// Try to send again. With the drop above, the channel has at
		// least one slot, but another producer may have raced in.
		select {
		case c.ch <- e:
			c.mu.Lock()
			c.consecutiveDrops++
			c.mu.Unlock()
			return false
		default:
			c.mu.Lock()
			c.consecutiveDrops++
			c.mu.Unlock()
			return false
		}
	}
}
