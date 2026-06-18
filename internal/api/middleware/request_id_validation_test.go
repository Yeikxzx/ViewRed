package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestRequestID_RejectsCRLFInjection is the regression test for
// Plan 3 review finding W1 (CWE-117: log injection via
// X-Request-Id). The middleware used to echo the inbound header
// verbatim, so an attacker could send a header like
//   X-Request-Id: legit\r\n[ERROR] arbitrary log line
// to inject a fake log line on the next request the operator
// greps for. The fix discards any inbound value that contains
// CR or LF and generates a fresh UUID instead.
func TestRequestID_RejectsCRLFInjection(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name        string
		inbound     string
		mustNotEcho bool
	}{
		{
			name:        "CR injection",
			inbound:     "legit\r[ERROR] arbitrary",
			mustNotEcho: true,
		},
		{
			name:        "LF injection",
			inbound:     "legit\n[ERROR] arbitrary",
			mustNotEcho: true,
		},
		{
			name:        "CRLF injection",
			inbound:     "legit\r\n[ERROR] arbitrary",
			mustNotEcho: true,
		},
		{
			name:        "NUL byte (control char family)",
			inbound:     "legit\x00hidden",
			mustNotEcho: true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.Use(RequestID())
			r.GET("/x", func(c *gin.Context) { c.Status(200) })

			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			req.Header.Set(RequestIDHeader, tt.inbound)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			got := w.Header().Get(RequestIDHeader)
			if got == "" {
				t.Fatalf("expected a generated request id, got empty header")
			}
			if got == tt.inbound {
				t.Fatalf("CRLF/control-char id must be rejected, got echoed value %q", got)
			}
			// Sanity: no CR or LF in the response header value.
			if strings.ContainsAny(got, "\r\n\x00") {
				t.Fatalf("generated id must not contain CR/LF/NUL, got %q", got)
			}
		})
	}
}

// TestRequestID_RejectsOverlongInbound covers the length cap. A
// 200-character inbound id is rejected and a fresh UUID is
// generated in its place. The cap is 128 chars: long enough for
// UUIDs (36), ULIDs (26), and short correlation tokens, short
// enough to bound memory and log-line growth.
func TestRequestID_RejectsOverlongInbound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(RequestID())
	r.GET("/x", func(c *gin.Context) { c.Status(200) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	long := strings.Repeat("a", 200)
	req.Header.Set(RequestIDHeader, long)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	got := w.Header().Get(RequestIDHeader)
	if got == long {
		t.Fatalf("overlong inbound id must be rejected, got echoed value of length %d", len(got))
	}
	if len(got) > 128 {
		t.Fatalf("generated id must respect the 128-char cap, got length %d", len(got))
	}
	if got == "" {
		t.Fatalf("expected a generated request id, got empty header")
	}
}

// TestRequestID_AcceptsShortReasonableInbound pins the happy
// path: short ASCII ids (the kind an operator or upstream proxy
// would send) are echoed verbatim. This guards against
// over-rejection: a fix that discards every inbound id would
// break log correlation across services.
func TestRequestID_AcceptsShortReasonableInbound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []string{
		"abc-123",
		"req-2026-06-18-001",
		"550e8400-e29b-41d4-a716-446655440000",
		strings.Repeat("a", 128), // exactly at the cap
	}
	for _, inbound := range cases {
		t.Run(inbound, func(t *testing.T) {
			r := gin.New()
			r.Use(RequestID())
			r.GET("/x", func(c *gin.Context) { c.Status(200) })

			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			req.Header.Set(RequestIDHeader, inbound)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if got := w.Header().Get(RequestIDHeader); got != inbound {
				t.Fatalf("expected echo of %q, got %q", inbound, got)
			}
		})
	}
}
