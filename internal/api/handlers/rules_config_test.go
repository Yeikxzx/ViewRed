package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/viewred/viewred/internal/storage"
)

func TestGetRules(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("empty list when no rules", func(t *testing.T) {
		mock := &storage.MockQuerier{
			GetCustomRulesFn: func(_ context.Context) ([]storage.CustomRule, error) {
				return nil, nil
			},
		}
		r := gin.New()
		r.GET("/api/v1/rules", GetRules(mock))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/rules", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		var rules []storage.CustomRule
		if err := json.Unmarshal(w.Body.Bytes(), &rules); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(rules) != 0 {
			t.Errorf("expected empty list, got %d items", len(rules))
		}
	})

	t.Run("returns rules list", func(t *testing.T) {
		mock := &storage.MockQuerier{
			GetCustomRulesFn: func(_ context.Context) ([]storage.CustomRule, error) {
				return []storage.CustomRule{
					{ID: 1, RuleText: "alert tcp any any -> any 80", Enabled: true},
				}, nil
			},
		}
		r := gin.New()
		r.GET("/api/v1/rules", GetRules(mock))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/rules", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		var rules []storage.CustomRule
		if err := json.Unmarshal(w.Body.Bytes(), &rules); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(rules) != 1 || rules[0].ID != 1 {
			t.Errorf("expected 1 rule with ID 1, got %+v", rules)
		}
	})

	t.Run("error when store fails", func(t *testing.T) {
		mock := &storage.MockQuerier{
			GetCustomRulesFn: func(_ context.Context) ([]storage.CustomRule, error) {
				return nil, errors.New("store error")
			},
		}
		r := gin.New()
		r.GET("/api/v1/rules", GetRules(mock))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/rules", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestCreateRule(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("bad request - empty body", func(t *testing.T) {
		r := gin.New()
		r.POST("/api/v1/rules", CreateRule(&storage.MockQuerier{}))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/rules", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("creates rule successfully", func(t *testing.T) {
		mock := &storage.MockQuerier{
			SaveCustomRuleFn: func(_ context.Context, ruleText string, enabled bool) (int64, error) {
				return 1, nil
			},
		}
		r := gin.New()
		r.POST("/api/v1/rules", CreateRule(mock))

		w := httptest.NewRecorder()
		body := `{"rule_text":"alert tcp any any -> any 22","enabled":true}`
		req, _ := http.NewRequest("POST", "/api/v1/rules", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
		}
	})

	t.Run("error when store fails", func(t *testing.T) {
		mock := &storage.MockQuerier{
			SaveCustomRuleFn: func(_ context.Context, ruleText string, enabled bool) (int64, error) {
				return 0, errors.New("store error")
			},
		}
		r := gin.New()
		r.POST("/api/v1/rules", CreateRule(mock))

		w := httptest.NewRecorder()
		body := `{"rule_text":"alert tcp any any -> any 80","enabled":true}`
		req, _ := http.NewRequest("POST", "/api/v1/rules", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestDeleteRule(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("bad request - invalid id", func(t *testing.T) {
		r := gin.New()
		r.DELETE("/api/v1/rules/:id", DeleteRule(&storage.MockQuerier{}))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/api/v1/rules/abc", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("deletes rule successfully", func(t *testing.T) {
		mock := &storage.MockQuerier{
			DeleteCustomRuleFn: func(_ context.Context, id int64) error { return nil },
		}
		r := gin.New()
		r.DELETE("/api/v1/rules/:id", DeleteRule(mock))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/api/v1/rules/1", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
		}
	})

	t.Run("error when store fails", func(t *testing.T) {
		mock := &storage.MockQuerier{
			DeleteCustomRuleFn: func(_ context.Context, id int64) error { return errors.New("store error") },
		}
		r := gin.New()
		r.DELETE("/api/v1/rules/:id", DeleteRule(mock))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/api/v1/rules/1", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}
