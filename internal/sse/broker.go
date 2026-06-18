package sse

import (
	"context"
	"sync"
	"time"
)

const (
	// defaultBufferSize is the per-client ring buffer depth. Keeps a
	// slow consumer from blocking the broker.
	defaultBufferSize = 64

	// defaultHeartbeat is the production heartbeat interval. Three
	// consecutive dropped heartbeats (= 90s) mark a client as stale.
	defaultHeartbeat = 30 * time.Second

	// heartbeatEventName is the SSE event name the broker sends on
	// every tick. Consumers should treat it as keep-alive; the data
	// payload is empty.
	heartbeatEventName = "heartbeat"

	// staleHeartbeatLimit is the consecutive-drops threshold for
	// declaring a client dead. Three gives one heartbeat of grace
	// (network blip, GC pause) before removing the client.
	staleHeartbeatLimit = 3

	// channelBufferSize is the depth of the broker's internal
	// channels (broadcast, unregister). 64 lets main.go push a burst
	// of metrics without blocking the producer goroutine.
	channelBufferSize = 64
)

// Broker is the central pub/sub for SSE clients.
//
// Concurrency model: a single Run loop owns the clients map and
// processes every state change (register, unregister, broadcast,
// heartbeat) via channels. Public methods only send to those channels,
// so they are safe to call from any goroutine without further locking.
//
// The heartbeat tick fires every `heartbeat` duration. On each tick,
// the broker tries to deliver a heartbeat event to every client.
// Clients that had to drop events for staleHeartbeatLimit consecutive
// heartbeats are removed: they are presumed dead (network gone,
// consumer stuck) even if the consumer's context has not yet been
// cancelled.
type Broker struct {
	mu         sync.RWMutex
	clients    map[*client]struct{}
	bufferSize int
	heartbeat  time.Duration

	register   chan *client
	unregister chan *client
	broadcast  chan Event
	done       chan struct{}

	shutdownOnce sync.Once
}

// NewBroker creates a broker with the production defaults: 64-event
// ring buffer per client, 30-second heartbeat.
func NewBroker() *Broker {
	return newBroker(defaultBufferSize, defaultHeartbeat)
}

// newBroker is the package-private constructor used by tests to inject
// a shorter heartbeat (so the stale-removal test can run in
// milliseconds).
func newBroker(bufferSize int, heartbeat time.Duration) *Broker {
	return &Broker{
		clients:    make(map[*client]struct{}),
		bufferSize: bufferSize,
		heartbeat:  heartbeat,
		register:   make(chan *client),
		unregister: make(chan *client, channelBufferSize),
		broadcast:  make(chan Event, channelBufferSize),
		done:       make(chan struct{}),
	}
}

// Run is the broker's event loop. Cancel ctx to stop; alternatively,
// call Shutdown. The loop processes all state changes (register /
// unregister / broadcast / heartbeat) on a single goroutine, so the
// clients map needs no external synchronisation beyond the mutex that
// guards the snapshot the heartbeat and broadcast paths read.
func (b *Broker) Run(ctx context.Context) {
	ticker := time.NewTicker(b.heartbeat)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-b.done:
			return
		case c := <-b.register:
			b.mu.Lock()
			b.clients[c] = struct{}{}
			b.mu.Unlock()
		case c := <-b.unregister:
			b.removeClient(c)
		case e := <-b.broadcast:
			b.deliver(e)
		case <-ticker.C:
			b.sendHeartbeats()
		}
	}
}

// deliver fans an event out to every subscribed client. Clients whose
// subscription does not match the event's topic are skipped.
func (b *Broker) deliver(e Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for c := range b.clients {
		if !c.subscribedTo(e.Topic) {
			continue
		}
		c.tryDeliver(e)
	}
}

// sendHeartbeats ticks once per heartbeat interval. Successful
// (non-dropping) deliveries leave the per-client drop counter alone
// (tryDeliver already reset it to 0). Three consecutive heartbeats
// that had to drop a stale event mark the client as stale and trigger
// removal.
func (b *Broker) sendHeartbeats() {
	var stale []*client
	b.mu.RLock()
	for c := range b.clients {
		delivered := c.tryDeliver(Event{Name: heartbeatEventName})
		if delivered {
			continue
		}
		c.mu.Lock()
		c.consecutiveDrops++
		if c.consecutiveDrops >= staleHeartbeatLimit {
			stale = append(stale, c)
		}
		c.mu.Unlock()
	}
	b.mu.RUnlock()

	for _, c := range stale {
		b.removeClient(c)
	}
}

// removeClient evicts c from the map and closes its channel. Safe to
// call multiple times for the same client — duplicates are no-ops,
// which keeps the explicit-unsubscribe and the auto-cancel-ctx
// goroutine from racing into a double-close panic.
func (b *Broker) removeClient(c *client) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.clients[c]; ok {
		delete(b.clients, c)
		close(c.ch)
	}
}

// Subscribe registers a new SSE client subscribed to the given topics.
// The returned channel delivers events for this client; the returned
// function unsubscribes (also called automatically when ctx is
// cancelled).
//
// Topics is the list of routing keys the client wants. An empty list
// means the client receives every broadcast event (events with an
// empty topic).
func (b *Broker) Subscribe(ctx context.Context, topics []string) (<-chan Event, func()) {
	c := newClient(ctx, topics, b.bufferSize)
	select {
	case b.register <- c:
	case <-ctx.Done():
	case <-b.done:
	}
	// Watch the consumer's context: when the SSE handler disconnects,
	// the framework cancels the request context, and we clean up
	// without the consumer having to call the unsubscribe func.
	go func() {
		select {
		case <-ctx.Done():
		case <-b.done:
			return
		}
		select {
		case b.unregister <- c:
		case <-b.done:
		}
	}()
	return c.ch, func() {
		select {
		case b.unregister <- c:
		case <-b.done:
		}
	}
}

// Broadcast sends an event to every client subscribed to the event's
// topic, or to every client if the event's topic is empty.
//
// Broadcast is non-blocking: if the broker's internal broadcast
// channel is full, the call waits until the Run loop drains it. With
// the default 64-deep buffer and a single-consumer Run loop this is
// effectively immediate, but callers that push a flood of events from
// a tight loop should be aware.
func (b *Broker) Broadcast(e Event) {
	select {
	case b.broadcast <- e:
	case <-b.done:
	}
}

// Shutdown closes every client channel and stops the broker's loop. It
// is safe to call multiple times. After Shutdown, the broker no longer
// accepts subscriptions or broadcasts (the public methods become
// no-ops via the done-channel select).
//
// Shutdown does not wait for the Run goroutine to return; the caller
// cancels the context passed to Run to wait for that.
func (b *Broker) Shutdown(ctx context.Context) error {
	b.shutdownOnce.Do(func() {
		close(b.done)
		b.mu.Lock()
		for c := range b.clients {
			close(c.ch)
			delete(b.clients, c)
		}
		b.mu.Unlock()
	})
	return nil
}

// ActiveClientCount returns the number of currently connected clients.
func (b *Broker) ActiveClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}
