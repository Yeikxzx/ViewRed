// Package sse provides an in-process broker for Server-Sent Events.
//
// The Broker is the central hub: Subscribe returns a channel that an
// HTTP handler streams to the wire, and Broadcast fans an Event out to
// every subscriber. A 64-event ring buffer per client keeps slow
// consumers from blocking the broker; a 30-second heartbeat detects
// dead connections even when the consumer's context has not yet been
// cancelled (e.g. network drop without TCP RST).
package sse

// Event is a single SSE message.
//
// Name maps to the "event:" SSE field; Data maps to the "data:" field.
// Topic is the broker's routing key. When non-empty, only clients
// whose topics list contains it receive the event. When empty, every
// connected client receives the event (true broadcast).
type Event struct {
	Name  string
	Data  []byte
	Topic string
}
