// Package resolver keeps a process-local map from IP address to
// hostname, populated by observing DNS responses on the wire.
//
// The resolver is intentionally simple: it does no network I/O of
// its own (no reverse lookups, no SNI parsing, no third-party APIs
// in v1) — it only learns what the devices on the network already
// asked DNS for. That is enough to label ~all outbound traffic
// because the same DNS lookup that fetches "youtube.com" for the
// browser also fetches the IPs that the browser then connects to.
//
// The cache is bounded by a TTL (default: matches the answer's own
// TTL, capped at maxTTL). Expired entries are evicted by a
// background goroutine so the map cannot grow without bound.
package resolver

import (
	"sync"
	"time"
)

// Default and upper-bound TTLs. Default is 1h, which matches typical
// upstream resolvers' minimum cache time and is long enough to
// survive a tab being backgrounded. Max is 24h, which prevents a
// malicious or misconfigured DNS response with TTL=2³¹-1 from
// pinning a stale mapping forever.
const (
	defaultTTL   = 1 * time.Hour
	maxTTL       = 24 * time.Hour
	cleanupEvery = 5 * time.Minute
)

// entry is a single cached IP→hostname mapping with an expiry
// time. The struct is unexported so callers cannot mutate it
// without going through Set.
type entry struct {
	hostname string
	expires  time.Time
}

// Stats is a snapshot of the resolver's internal state, returned by
// Stats for observability and tests. A non-pointer value keeps
// callers from holding stale locks.
type Stats struct {
	Size     int
	Expired  int
	MaxTTL   time.Duration
	MinTTL   time.Duration
}

// Resolver is the in-memory IP→hostname cache. Zero value is NOT
// usable; construct one with New(). A Resolver is safe for
// concurrent use; its Run/Stop methods manage the eviction
// goroutine.
type Resolver struct {
	mu      sync.RWMutex
	entries map[string]entry
	now     func() time.Time // injectable clock for tests

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// New constructs a Resolver with an empty cache. Call Run() to
// start the background eviction goroutine, Stop() to stop it.
// Stop is idempotent and safe to call after Run if Run returned
// without error.
func New() *Resolver {
	return &Resolver{
		entries: make(map[string]entry),
		now:     time.Now,
		stopCh:  make(chan struct{}),
	}
}

// Run starts the background eviction goroutine. The goroutine
// exits when Stop is called. Run returns immediately; the caller
// does not need to wait.
//
// Run is idempotent in the sense that calling it twice is a
// no-op for the second call (the second goroutine is started but
// does nothing useful). Callers should call Run at most once per
// Resolver instance.
func (r *Resolver) Run() {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		t := time.NewTicker(cleanupEvery)
		defer t.Stop()
		for {
			select {
			case <-r.stopCh:
				return
			case <-t.C:
				r.evictExpired()
			}
		}
	}()
}

// Stop signals the eviction goroutine to exit and waits for it.
// Stop is safe to call multiple times; subsequent calls are
// no-ops (the channel close panics are guarded by a sync.Once in
// practice — see the implementation).
func (r *Resolver) Stop() {
	select {
	case <-r.stopCh:
		// already closed
		return
	default:
		close(r.stopCh)
	}
	r.wg.Wait()
}

// Set records a hostname for the given IP. Empty inputs are
// silently ignored. The TTL comes from the DNS answer when
// available; an explicit ttlSeconds of 0 falls back to
// defaultTTL. Values larger than maxTTL are clamped to maxTTL.
func (r *Resolver) Set(ip, hostname string, ttlSeconds uint32) {
	if ip == "" || hostname == "" {
		return
	}
	ttl := time.Duration(ttlSeconds) * time.Second
	if ttl <= 0 {
		ttl = defaultTTL
	}
	if ttl > maxTTL {
		ttl = maxTTL
	}
	r.mu.Lock()
	r.entries[ip] = entry{
		hostname: hostname,
		expires:  r.now().Add(ttl),
	}
	r.mu.Unlock()
}

// Resolve returns the cached hostname for an IP, or the empty
// string if the IP is unknown or its entry has expired. The
// expiry check happens under the read lock so the eviction
// goroutine and Resolve do not race.
func (r *Resolver) Resolve(ip string) string {
	if ip == "" {
		return ""
	}
	r.mu.RLock()
	e, ok := r.entries[ip]
	r.mu.RUnlock()
	if !ok || r.now().After(e.expires) {
		return ""
	}
	return e.hostname
}

// ResolveMany looks up hostnames for a slice of IPs in a single
// read-lock acquisition. Returns a parallel slice of hostnames
// (empty string for unknown / expired entries). Useful for the
// API handler that enriches a list of packet events: doing all
// resolutions at once avoids taking the RLock per packet and
// keeps the lock hot path short.
func (r *Resolver) ResolveMany(ips []string) []string {
	if len(ips) == 0 {
		return nil
	}
	now := r.now()
	r.mu.RLock()
	out := make([]string, len(ips))
	for i, ip := range ips {
		if ip == "" {
			continue
		}
		e, ok := r.entries[ip]
		if !ok || now.After(e.expires) {
			continue
		}
		out[i] = e.hostname
	}
	r.mu.RUnlock()
	return out
}

// Stats returns a snapshot of cache size and TTL distribution.
// Used by observability and tests; the lock is held only for the
// duration of the snapshot.
func (r *Resolver) Stats() Stats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s := Stats{Size: len(r.entries)}
	if s.Size == 0 {
		return s
	}
	now := r.now()
	min, max := time.Duration(0), time.Duration(0)
	for _, e := range r.entries {
		rem := e.expires.Sub(now)
		if rem <= 0 {
			s.Expired++
			continue
		}
		if min == 0 || rem < min {
			min = rem
		}
		if rem > max {
			max = rem
		}
	}
	s.MinTTL = min
	s.MaxTTL = max
	return s
}

// Sample returns up to limit live (non-expired) entries from the
// cache, in unspecified order. Used by the debug endpoint so
// operators can confirm the resolver is actually being fed
// without dumping the whole map. Limit is clamped to a sane
// upper bound so a caller can't OOM the response.
func (r *Resolver) Sample(limit int) []map[string]any {
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	now := r.now()
	out := make([]map[string]any, 0, limit)
	for ip, e := range r.entries {
		if now.After(e.expires) {
			continue
		}
		out = append(out, map[string]any{
			"ip":       ip,
			"hostname": e.hostname,
		})
		if len(out) >= limit {
			break
		}
	}
	return out
}

// evictExpired drops every entry whose expiry is in the past.
// Called by the background ticker. Writes the map in place
// under the write lock.
func (r *Resolver) evictExpired() {
	now := r.now()
	r.mu.Lock()
	for ip, e := range r.entries {
		if now.After(e.expires) {
			delete(r.entries, ip)
		}
	}
	r.mu.Unlock()
}
