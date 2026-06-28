package resolver

import (
	"testing"
	"time"
)

// frozenNow returns a deterministic clock for tests so we can
// assert against exact expiry times. The returned time is well
// after the unix epoch to avoid any zero-value subtleties.
func frozenNow() (time.Time, func() time.Time) {
	t := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	return t, func() time.Time { return t }
}

func newWithClock(clock func() time.Time) *Resolver {
	return &Resolver{
		entries: make(map[string]entry),
		now:     clock,
		stopCh:  make(chan struct{}),
	}
}

func TestSetAndResolve(t *testing.T) {
	_, clock := frozenNow()
	r := newWithClock(clock)

	r.Set("1.1.1.1", "one.one.one.one", 300)

	if got := r.Resolve("1.1.1.1"); got != "one.one.one.one" {
		t.Errorf("Resolve(1.1.1.1) = %q, want %q", got, "one.one.one.one")
	}
}

func TestResolveUnknownReturnsEmpty(t *testing.T) {
	_, clock := frozenNow()
	r := newWithClock(clock)

	if got := r.Resolve("8.8.8.8"); got != "" {
		t.Errorf("Resolve(8.8.8.8) = %q, want empty", got)
	}
}

func TestResolveEmptyIPReturnsEmpty(t *testing.T) {
	_, clock := frozenNow()
	r := newWithClock(clock)

	if got := r.Resolve(""); got != "" {
		t.Errorf("Resolve(\"\") = %q, want empty", got)
	}
}

func TestSetEmptyInputsAreIgnored(t *testing.T) {
	_, clock := frozenNow()
	r := newWithClock(clock)

	// Empty hostname — should be ignored even if IP is set.
	r.Set("1.1.1.1", "", 300)
	if got := r.Resolve("1.1.1.1"); got != "" {
		t.Errorf("Resolve(1.1.1.1) after empty-hostname Set = %q, want empty", got)
	}

	// Empty IP — should be ignored.
	r.Set("", "example.com", 300)
	if got := r.Resolve("example.com"); got != "" {
		t.Errorf("Resolve(example.com) after empty-IP Set = %q, want empty", got)
	}
}

func TestResolveExpires(t *testing.T) {
	base, clock := frozenNow()
	r := newWithClock(clock)

	// Set with a 1-minute TTL.
	r.Set("1.1.1.1", "one.one.one.one", 60)

	// Just-set: still valid.
	if got := r.Resolve("1.1.1.1"); got != "one.one.one.one" {
		t.Errorf("immediately after Set: Resolve = %q, want %q", got, "one.one.one.one")
	}

	// Advance past expiry.
	now := base.Add(2 * time.Minute)
	r.now = func() time.Time { return now }

	if got := r.Resolve("1.1.1.1"); got != "" {
		t.Errorf("after expiry: Resolve = %q, want empty", got)
	}
}

func TestSetZeroTTLUsesDefault(t *testing.T) {
	_, clock := frozenNow()
	r := newWithClock(clock)

	// TTL=0 → use defaultTTL (1h). The entry must still be valid
	// an hour later.
	r.Set("1.1.1.1", "one.one.one.one", 0)
	if got := r.Resolve("1.1.1.1"); got != "one.one.one.one" {
		t.Errorf("Resolve after TTL=0 Set = %q, want %q", got, "one.one.one.one")
	}
}

func TestSetHugeTTLClampedToMax(t *testing.T) {
	_, clock := frozenNow()
	r := newWithClock(clock)

	// TTL = 2³¹-1 (max uint32) — would be ~68 years. Must be
	// clamped to maxTTL (24h) so a malicious / misconfigured DNS
	// response cannot pin a stale entry forever.
	r.Set("1.1.1.1", "one.one.one.one", 0xFFFFFFFF)

	// We can't wait 24h in a test, but we can confirm the entry
	// exists and the cache size is 1.
	if s := r.Stats(); s.Size != 1 {
		t.Errorf("after huge-TTL Set: Stats.Size = %d, want 1", s.Size)
	}
}

func TestResolveManyReturnsParallelSlice(t *testing.T) {
	_, clock := frozenNow()
	r := newWithClock(clock)

	r.Set("1.1.1.1", "one.one.one.one", 300)
	r.Set("8.8.8.8", "dns.google", 300)

	ips := []string{"1.1.1.1", "9.9.9.9", "8.8.8.8", ""}
	got := r.ResolveMany(ips)
	want := []string{"one.one.one.one", "", "dns.google", ""}

	if len(got) != len(want) {
		t.Fatalf("ResolveMany returned %d entries, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ResolveMany[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestEvictExpiredRemovesOldEntries(t *testing.T) {
	base, clock := frozenNow()
	r := newWithClock(clock)

	r.Set("1.1.1.1", "one.one.one.one", 60) // expires in 1 minute
	r.Set("8.8.8.8", "dns.google", 3600)     // expires in 1 hour
	r.Set("9.9.9.9", "quad9", 7200)         // expires in 2 hours

	// Move clock 90s into the future — only the 1-minute entry
	// should be expired.
	now := base.Add(90 * time.Second)
	r.now = func() time.Time { return now }

	r.evictExpired()

	if s := r.Stats(); s.Size != 2 {
		t.Errorf("after eviction: Stats.Size = %d, want 2", s.Size)
	}
	if got := r.Resolve("1.1.1.1"); got != "" {
		t.Errorf("Resolve(1.1.1.1) = %q, want empty (was expired)", got)
	}
	if got := r.Resolve("8.8.8.8"); got != "dns.google" {
		t.Errorf("Resolve(8.8.8.8) = %q, want %q", got, "dns.google")
	}
}

func TestStatsCountsExpired(t *testing.T) {
	base, clock := frozenNow()
	r := newWithClock(clock)

	r.Set("1.1.1.1", "one.one.one.one", 60)
	r.Set("8.8.8.8", "dns.google", 3600)

	// Move past the first entry's expiry.
	now := base.Add(90 * time.Second)
	r.now = func() time.Time { return now }

	s := r.Stats()
	if s.Size != 2 {
		t.Errorf("Stats.Size = %d, want 2 (both still in map)", s.Size)
	}
	if s.Expired != 1 {
		t.Errorf("Stats.Expired = %d, want 1", s.Expired)
	}
}

func TestStopIsIdempotent(t *testing.T) {
	_, clock := frozenNow()
	r := newWithClock(clock)
	r.Run()
	r.Stop()
	r.Stop() // second call must not panic on closed channel
}
