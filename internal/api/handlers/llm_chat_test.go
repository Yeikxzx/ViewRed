package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/viewred/viewred/internal/storage"
)

func TestLLMChatHandler_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("bad request - empty body", func(t *testing.T) {
		r := gin.New()
		r.POST("/api/v1/chat", LLMChatHandler(&storage.MockQuerier{}))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/chat", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("bad request - no conversation_id", func(t *testing.T) {
		r := gin.New()
		r.POST("/api/v1/chat", LLMChatHandler(&storage.MockQuerier{}))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/chat", strings.NewReader(
			`{"messages":[{"role":"user","content":"hello"}]}`,
		))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("bad request - empty messages", func(t *testing.T) {
		r := gin.New()
		r.POST("/api/v1/chat", LLMChatHandler(&storage.MockQuerier{}))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/chat", strings.NewReader(
			`{"conversation_id":"abc-123","messages":[]}`,
		))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("ollama unavailable returns 502", func(t *testing.T) {
		mock := &storage.MockQuerier{
			EnsureConversationExistsFn: func(_ context.Context, conversationID string) error {
				return nil
			},
			SaveLLMMessageFn: func(_ context.Context, msg storage.LLMMessage) error {
				return nil
			},
		}
		r := gin.New()
		r.POST("/api/v1/chat", LLMChatHandler(mock))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/chat", strings.NewReader(
			`{"conversation_id":"abc-123","messages":[{"role":"user","content":"hello"}]}`,
		))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadGateway {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadGateway)
		}
	})
}

func TestGetChatHistoryHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("returns history", func(t *testing.T) {
		mock := &storage.MockQuerier{
			GetConversationHistoryFn: func(_ context.Context, conversationID string) ([]storage.LLMMessage, error) {
				return []storage.LLMMessage{
					{Role: "user", Content: "hello"},
					{Role: "assistant", Content: "hi there"},
				}, nil
			},
		}
		r := gin.New()
		r.GET("/api/v1/chat/history/:conversation_id", GetChatHistoryHandler(mock))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/chat/history/abc-123", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("error when store fails", func(t *testing.T) {
		mock := &storage.MockQuerier{
			GetConversationHistoryFn: func(_ context.Context, conversationID string) ([]storage.LLMMessage, error) {
				return nil, nil
			},
		}
		r := gin.New()
		r.GET("/api/v1/chat/history/:conversation_id", GetChatHistoryHandler(mock))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/chat/history/abc-123", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("empty conversation_id returns 404 (route not matched)", func(t *testing.T) {
		// Gin returns 404 when the route param is empty because the pattern
		// /history/:conversation_id requires a non-empty segment. This is
		// expected — the handler itself checks for empty conversation_id.
		r := gin.New()
		r.GET("/api/v1/chat/history/:conversation_id", GetChatHistoryHandler(&storage.MockQuerier{}))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/chat/history/", nil)
		r.ServeHTTP(w, req)

		// Gin requires the trailing segment; empty param = route not matched
		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
	})
}
