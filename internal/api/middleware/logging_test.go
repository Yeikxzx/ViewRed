package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestLogging_LogsMethodPathStatus(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(old)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestLogging())
	r.GET("/x", func(c *gin.Context) { c.Status(200) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	out := buf.String()
	if !strings.Contains(out, "method=GET") {
		t.Errorf("expected method=GET in log, got %q", out)
	}
	if !strings.Contains(out, "path=/x") {
		t.Errorf("expected path=/x in log, got %q", out)
	}
	if !strings.Contains(out, "status=200") {
		t.Errorf("expected status=200 in log, got %q", out)
	}
}
