package sse

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"
)

// waitFor polls cond until it returns true or timeout elapses. Fails
// the test on timeout. Tests use it to synchronize with the broker's
// Run loop without fixed sleeps.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("waitFor: timeout after %v", timeout)
}

// startTestBroker creates a broker with default settings and starts
// its Run loop in a goroutine. The returned cancel stops the loop.
func startTestBroker(t *testing.T) (*Broker, context.CancelFunc) {
	t.Helper()
	b := newBroker(64, 30*time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	go b.Run(ctx)
	return b, cancel
}

// A client subscribed to a topic receives a broadcast event for that
// topic, with Name and Data preserved verbatim.
func TestBroker_SubscribeAndReceive(t *testing.T) {
	b, cancel := startTestBroker(t)
	defer cancel()

	subCtx, subCancel := context.WithCancel(context.Background())
	defer subCancel()
	ch, _ := b.Subscribe(subCtx, []string{"metrics"})

	waitFor(t, time.Second, func() bool { return b.ActiveClientCount() == 1 })

	b.Broadcast(Event{Name: "tick", Data: []byte("hello"), Topic: "metrics"})

	select {
	case e := <-ch:
		if e.Name != "tick" {
			t.Errorf("expected name=tick, got %q", e.Name)
		}
		if string(e.Data) != "hello" {
			t.Errorf("expected data=hello, got %q", string(e.Data))
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

// An event with an empty topic reaches every client, regardless of
// the topics they subscribed to.
func TestBroker_BroadcastsToAllWhenTopicEmpty(t *testing.T) {
	b, cancel := startTestBroker(t)
	defer cancel()

	ch1, _ := b.Subscribe(context.Background(), []string{"metrics"})
	ch2, _ := b.Subscribe(context.Background(), []string{"alerts"})

	waitFor(t, time.Second, func() bool { return b.ActiveClientCount() == 2 })

	b.Broadcast(Event{Name: "global", Data: []byte("hi")})

	for i, ch := range []<-chan Event{ch1, ch2} {
		select {
		case e := <-ch:
			if e.Name != "global" {
				t.Errorf("client %d: expected name=global, got %q", i, e.Name)
			}
		case <-time.After(time.Second):
			t.Fatalf("client %d: timeout", i)
		}
	}
}

// A slow consumer's buffer caps at 64 events; the oldest are dropped
// to make room for new ones.
func TestBroker_RingBufferOverflow(t *testing.T) {
	b, cancel := startTestBroker(t)
	defer cancel()

	subCtx, subCancel := context.WithCancel(context.Background())
	defer subCancel()
	ch, _ := b.Subscribe(subCtx, []string{"metrics"})

	waitFor(t, time.Second, func() bool { return b.ActiveClientCount() == 1 })

	for i := 0; i < 100; i++ {
		b.Broadcast(Event{Name: "tick", Data: []byte(strconv.Itoa(i)), Topic: "metrics"})
	}

	// Wait for the broker to process all 100 broadcasts. With one
	// client and small payloads this is well under 50ms, but the
	// test only cares about the final buffer state, not the timing.
	waitFor(t, time.Second, func() bool { return len(ch) == 64 })
	time.Sleep(50 * time.Millisecond)

	received := make([]Event, 0, 100)
drain:
	for {
		select {
		case e := <-ch:
			received = append(received, e)
		default:
			break drain
		}
	}
	if len(received) != 64 {
		t.Errorf("expected 64 events in buffer, got %d", len(received))
	}
	if len(received) > 0 {
		if string(received[0].Data) != "36" {
			t.Errorf("expected first data=36 (oldest dropped), got %q", string(received[0].Data))
		}
		if string(received[len(received)-1].Data) != "99" {
			t.Errorf("expected last data=99, got %q", string(received[len(received)-1].Data))
		}
	}
}

// Calling the unsubscribe function removes the client and closes its
// channel.
func TestBroker_Unsubscribe(t *testing.T) {
	b, cancel := startTestBroker(t)
	defer cancel()

	ch, unsubscribe := b.Subscribe(context.Background(), []string{"metrics"})

	waitFor(t, time.Second, func() bool { return b.ActiveClientCount() == 1 })

	unsubscribe()

	waitFor(t, time.Second, func() bool { return b.ActiveClientCount() == 0 })

	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected channel to be closed after unsubscribe")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for channel close")
	}
}

// A client that never drains its channel is eventually removed by the
// broker. The consumer's context stays alive; only the broker's
// consecutive-drops heuristic decides the client is dead.
func TestBroker_StaleClientRemoval(t *testing.T) {
	b := newBroker(64, 10*time.Millisecond) // short heartbeat for fast test
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.Run(ctx)

	subCtx, subCancel := context.WithCancel(context.Background())
	defer subCancel()
	ch, _ := b.Subscribe(subCtx, []string{"metrics"})

	waitFor(t, time.Second, func() bool { return b.ActiveClientCount() == 1 })

	// Don't read from ch. Heartbeats will fill the buffer, then start
	// dropping. After 3 consecutive heartbeats with drops, the broker
	// removes the client.
	waitFor(t, 2*time.Second, func() bool { return b.ActiveClientCount() == 0 })

	// The channel has 64 buffered heartbeats AND is closed. Drain
	// until the close signal arrives.
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return // success: channel closed
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for channel close")
		}
	}
}

// Many concurrent subscribers and broadcasters must not race or panic.
func TestBroker_ConcurrentSubscribeAndBroadcast(t *testing.T) {
	b, cancel := startTestBroker(t)
	defer cancel()

	var wg sync.WaitGroup
	nSubs := 10
	nBroadcasts := 50

	for i := 0; i < nSubs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			subCtx, subCancel := context.WithCancel(context.Background())
			defer subCancel()
			ch, _ := b.Subscribe(subCtx, []string{"metrics"})
			// Drain a few events to keep the buffer from filling, then
			// exit. The point of this test is concurrency, not
			// delivery semantics.
			for j := 0; j < 5; j++ {
				select {
				case <-ch:
				case <-time.After(100 * time.Millisecond):
					return
				}
			}
		}()
	}

	for j := 0; j < nBroadcasts; j++ {
		wg.Add(1)
		go func(j int) {
			defer wg.Done()
			b.Broadcast(Event{Name: "tick", Data: []byte(strconv.Itoa(j)), Topic: "metrics"})
		}(j)
	}

	wg.Wait()
	// No panic, no race. Test passes.
}

// Shutdown closes every client channel and drops every client from
// the active set.
func TestBroker_ShutdownClosesAllClients(t *testing.T) {
	b, cancel := startTestBroker(t)
	defer cancel()

	ch1, _ := b.Subscribe(context.Background(), []string{"a"})
	ch2, _ := b.Subscribe(context.Background(), []string{"b"})

	waitFor(t, time.Second, func() bool { return b.ActiveClientCount() == 2 })

	if err := b.Shutdown(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i, ch := range []<-chan Event{ch1, ch2} {
		select {
		case _, ok := <-ch:
			if ok {
				t.Errorf("client %d: expected channel to be closed", i)
			}
		case <-time.After(time.Second):
			t.Errorf("client %d: timeout", i)
		}
	}

	if got := b.ActiveClientCount(); got != 0 {
		t.Errorf("expected 0 clients after shutdown, got %d", got)
	}
}
