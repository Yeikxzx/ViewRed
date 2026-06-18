package health

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// TestRegistrySanitizesDetail is the regression test for IMPORTANT #2
// (sanitize health handler's Subsystem.Detail). The pgxpool ping error
// can include the DSN, which contains the database password. The fix
// is to redact userinfo (the `user:pass@` part of any URL) from the
// Detail field before exposing it in the report.
//
// This test registers a check that returns an error containing a URL
// with credentials, then asserts that the Detail in the report has
// the userinfo redacted.
func TestRegistrySanitizesDetail(t *testing.T) {
	r := New()
	const dsn = "postgres://dbuser:supersecret@db.example.com:5432/viewred"
	r.Add("storage", func(ctx context.Context) error {
		return errors.New("dial tcp 10.0.0.1:5432: connect: " + dsn)
	})

	report := r.Check(context.Background())

	if len(report.Subsystems) != 1 {
		t.Fatalf("expected 1 subsystem, got %d", len(report.Subsystems))
	}
	detail := report.Subsystems[0].Detail

	if strings.Contains(detail, "supersecret") {
		t.Errorf("Detail leaked password: %q", detail)
	}
	if strings.Contains(detail, "dbuser") {
		t.Errorf("Detail leaked username: %q", detail)
	}
	// The redacted form should be present.
	if !strings.Contains(detail, "***:***@") {
		t.Errorf("expected redacted userinfo in Detail, got %q", detail)
	}
}

// TestRegistrySanitizesMultipleURLs verifies that the sanitizer
// handles multiple URLs in the same error message.
func TestRegistrySanitizesMultipleURLs(t *testing.T) {
	r := New()
	r.Add("storage", func(ctx context.Context) error {
		return errors.New("failed connecting postgres://u1:p1@h1/db and redis://u2:p2@h2:6379")
	})

	report := r.Check(context.Background())
	detail := report.Subsystems[0].Detail

	for _, secret := range []string{"u1", "p1", "u2", "p2"} {
		if strings.Contains(detail, secret) {
			t.Errorf("Detail leaked %q: %q", secret, detail)
		}
	}
}

// TestRegistryPreservesNonURLDetails verifies that error messages
// without URLs are passed through unchanged.
func TestRegistryPreservesNonURLDetails(t *testing.T) {
	r := New()
	r.Add("capture", func(ctx context.Context) error {
		return errors.New("interface eth0 is down")
	})

	report := r.Check(context.Background())
	detail := report.Subsystems[0].Detail

	if detail != "interface eth0 is down" {
		t.Errorf("expected unchanged detail, got %q", detail)
	}
}
