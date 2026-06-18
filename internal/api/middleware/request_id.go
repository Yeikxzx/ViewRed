package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestIDHeader is the canonical header name.
const RequestIDHeader = "X-Request-Id"

// requestIDMaxLen caps the inbound X-Request-Id value. 128 is
// comfortably larger than a UUID (36) or a ULID (26) and small
// enough to bound the memory and log-line growth an attacker
// could cause with a multi-megabyte header.
const requestIDMaxLen = 128

// RequestID generates a UUID per request (or reuses the inbound
// header if present and valid).
//
// The inbound value is rejected when any of these hold:
//   - it is empty
//   - it contains a CR, LF, or NUL byte (CWE-117 log-injection
//     risk: an attacker can split a log line and inject a fake
//     "ERROR ..." entry that operators may later grep for)
//   - it exceeds requestIDMaxLen characters
//
// In any of those cases the middleware generates a fresh UUID
// instead. The validation is intentionally cheap and
// allocation-free for the happy path.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := sanitizeRequestID(c.GetHeader(RequestIDHeader))
		if id == "" {
			id = uuid.NewString()
		}
		c.Set("request_id", id)
		c.Header(RequestIDHeader, id)
		c.Next()
	}
}

// sanitizeRequestID returns the inbound id when it is safe to
// echo, or the empty string when it must be rejected. The
// caller treats the empty return as "generate a new UUID".
//
// The check is conservative: any byte in the printable-ASCII
// danger set (CR/LF/NUL) is a hard reject. The length cap is
// 128 chars, which is well above the longest expected
// correlation id (UUID v4 = 36 chars).
func sanitizeRequestID(in string) string {
	if in == "" {
		return ""
	}
	if len(in) > requestIDMaxLen {
		return ""
	}
	// strings.ContainsAny scans the whole string; for the
	// happy path (a short UUID) the scan is a handful of
	// bytes and never allocates.
	if strings.ContainsAny(in, "\r\n\x00") {
		return ""
	}
	return in
}
