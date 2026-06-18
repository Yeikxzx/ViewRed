package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestID_GeneratesWhenMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/x", func(c *gin.Context) {
		c.String(200, c.GetString("request_id"))
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Body.String() == "" {
		t.Errorf("expected request id in body, got empty")
	}
	if got := w.Header().Get(RequestIDHeader); got == "" {
		t.Errorf("expected X-Request-Id header, got empty")
	}
	if w.Body.String() != w.Header().Get(RequestIDHeader) {
		t.Errorf("body and header request_id mismatch")
	}
}

func TestRequestID_ReusesInbound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/x", func(c *gin.Context) { c.Status(200) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(RequestIDHeader, "test-12345")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get(RequestIDHeader); got != "test-12345" {
		t.Errorf("expected inbound id, got %q", got)
	}
}
