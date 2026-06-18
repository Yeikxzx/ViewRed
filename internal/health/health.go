// Package health provides a registry of subsystem health checks. It is used
// by the /api/v1/health endpoint to report overall system status.
package health

import (
	"context"
	"regexp"
	"sync"
	"time"
)

// Status is the high-level health of a subsystem or the whole system.
type Status string

const (
	StatusOK       Status = "ok"
	StatusDegraded Status = "degraded"
	StatusDown     Status = "down"
)

// Subsystem is one component's health entry in a health Report.
type Subsystem struct {
	Name   string `json:"name"`
	Status Status `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// Report is the full health snapshot returned by the API.
type Report struct {
	Status        Status      `json:"status"`
	UptimeSeconds int64       `json:"uptime_seconds"`
	Subsystems    []Subsystem `json:"subsystems"`
}

// checkFunc is a user-supplied function that probes a subsystem.
type checkFunc func(ctx context.Context) error

// Registry holds a set of named subsystem health checks.
type Registry struct {
	mu        sync.RWMutex
	checks    map[string]checkFunc
	startedAt time.Time
}

// New creates an empty health Registry.
func New() *Registry {
	return &Registry{
		checks:    make(map[string]checkFunc),
		startedAt: time.Now(),
	}
}

// Add registers a health check for a named subsystem. The check function
// should return nil for healthy, or an error describing the problem.
func (r *Registry) Add(name string, check checkFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.checks[name] = check
}

// Check runs all registered checks and returns an aggregated Report.
// Subsystems are returned in registration order.
//
// Error details are sanitized via SanitizeDetail before being placed
// in the report — this prevents accidental leakage of credentials that
// may be embedded in error messages (e.g. the pgxpool ping error
// includes the DSN, which contains the database password).
func (r *Registry) Check(ctx context.Context) Report {
	r.mu.RLock()
	defer r.mu.RUnlock()

	subs := make([]Subsystem, 0, len(r.checks))
	allOK := true
	anyDown := false
	for name, check := range r.checks {
		sub := Subsystem{Name: name, Status: StatusOK}
		if err := check(ctx); err != nil {
			sub.Status = StatusDown
			sub.Detail = SanitizeDetail(err.Error())
			anyDown = true
			allOK = false
		}
		subs = append(subs, sub)
	}

	status := StatusOK
	switch {
	case allOK:
		status = StatusOK
	case anyDown && len(r.checks) > 0:
		// If all are down → Down. If some are down → Degraded.
		allDown := true
		for _, s := range subs {
			if s.Status != StatusDown {
				allDown = false
				break
			}
		}
		if allDown {
			status = StatusDown
		} else {
			status = StatusDegraded
		}
	}

	return Report{
		Status:        status,
		UptimeSeconds: int64(time.Since(r.startedAt).Seconds()),
		Subsystems:    subs,
	}
}

// Healthy returns true if the last Check returned StatusOK.
func (r *Registry) Healthy() bool {
	// Synchronous check (small cost).
	report := r.Check(context.Background())
	return report.Status == StatusOK
}

// urlUserinfo matches the userinfo component of a URL
// (scheme://user:pass@host/...). The scheme part is preserved so the
// log remains useful (you can still tell that a Postgres DSN was
// redacted, not a generic error). The user:pass part is replaced with
// "***:***".
var urlUserinfo = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.\-]*://)([^@\s/]+)@`)

// SanitizeDetail redacts credentials from URLs embedded in error
// messages. The pgxpool Ping error, for example, embeds the full DSN
// (which contains the database password) into its message. Exposing
// that to the /api/v1/health endpoint would leak the password to any
// caller.
//
// The redaction preserves the scheme and host so the error remains
// useful for debugging:
//
//	postgres://user:secret@host:5432/db
//	                  ↓
//	postgres://***:***@host:5432/db
//
// The redaction is applied to every URL-shaped substring in the input,
// so an error containing multiple URLs is handled correctly.
func SanitizeDetail(s string) string {
	if s == "" {
		return s
	}
	return urlUserinfo.ReplaceAllString(s, `${1}***:***@`)
}
