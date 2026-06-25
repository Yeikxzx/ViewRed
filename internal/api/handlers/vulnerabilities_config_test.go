package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/viewred/viewred/internal/storage"
)

func TestGetVulnerabilities(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("returns empty list when no vulnerabilities", func(t *testing.T) {
		mock := &storage.MockQuerier{
			GetVulnerabilitiesFn: func(_ context.Context, includeMitigated bool) ([]storage.Vulnerability, error) {
				return nil, nil
			},
		}
		r := gin.New()
		r.GET("/api/v1/vulnerabilities", GetVulnerabilities(mock))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/vulnerabilities", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("returns vulnerabilities", func(t *testing.T) {
		mock := &storage.MockQuerier{
			GetVulnerabilitiesFn: func(_ context.Context, includeMitigated bool) ([]storage.Vulnerability, error) {
				cve := "CVE-2024-1234"
				return []storage.Vulnerability{{ID: 1, CVEID: &cve}}, nil
			},
		}
		r := gin.New()
		r.GET("/api/v1/vulnerabilities", GetVulnerabilities(mock))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/vulnerabilities", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if !strings.Contains(w.Body.String(), "CVE-2024-1234") {
			t.Errorf("expected CVE in response, got: %s", w.Body.String())
		}
	})

	t.Run("error when store fails", func(t *testing.T) {
		mock := &storage.MockQuerier{
			GetVulnerabilitiesFn: func(_ context.Context, includeMitigated bool) ([]storage.Vulnerability, error) {
				return nil, errors.New("store error")
			},
		}
		r := gin.New()
		r.GET("/api/v1/vulnerabilities", GetVulnerabilities(mock))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/vulnerabilities", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestMitigateVulnerability(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("bad request - invalid id", func(t *testing.T) {
		r := gin.New()
		r.POST("/api/v1/vulnerabilities/:id/mitigate", MitigateVulnerability(&storage.MockQuerier{}))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/vulnerabilities/abc/mitigate", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("mitigates successfully", func(t *testing.T) {
		mock := &storage.MockQuerier{
			MitigateVulnerabilityFn: func(_ context.Context, id int64) error { return nil },
		}
		r := gin.New()
		r.POST("/api/v1/vulnerabilities/:id/mitigate", MitigateVulnerability(mock))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/vulnerabilities/1/mitigate", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
		}
	})

	t.Run("error when store fails", func(t *testing.T) {
		mock := &storage.MockQuerier{
			MitigateVulnerabilityFn: func(_ context.Context, id int64) error { return errors.New("store error") },
		}
		r := gin.New()
		r.POST("/api/v1/vulnerabilities/:id/mitigate", MitigateVulnerability(mock))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/vulnerabilities/1/mitigate", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}
