package handlers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/viewred/viewred/internal/storage"
)

// ChatRequest represents the JSON body expected from the frontend.
type ChatRequest struct {
	ConversationID string `json:"conversation_id"`
	Messages       []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

// OllamaRequest represents the JSON payload sent to local Ollama.
type OllamaRequest struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	Stream bool `json:"stream"`
}

// LLMChatHandler proxies the chat request to a local Ollama instance
// and streams the response back to the client using Server-Sent Events (SSE).
// It also persists the user's message and the assistant's final response to the database.
func LLMChatHandler(q storage.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ChatRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		if req.ConversationID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "conversation_id is required"})
			return
		}

		if len(req.Messages) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "messages cannot be empty"})
			return
		}

		// Save user message to DB
		userMsg := req.Messages[len(req.Messages)-1]
		if userMsg.Role == "user" {
			// Ensure conversation exists first (idempotent)
			_ = q.EnsureConversationExists(c.Request.Context(), req.ConversationID)
			
			err := q.SaveLLMMessage(c.Request.Context(), storage.LLMMessage{
				ConversationID: req.ConversationID,
				Role:           userMsg.Role,
				Content:        userMsg.Content,
			})
			if err != nil {
				// Log error but continue to get response
				fmt.Printf("Failed to save user message: %v\n", err)
			}
		}

		// Construct payload for Ollama
		ollamaReq := OllamaRequest{
			Model:    "qwen2.5:3b",
			Messages: req.Messages,
			Stream:   true,
		}
		
		bodyBytes, _ := json.Marshal(ollamaReq)
		
		// Make request to local Ollama API
		resp, err := http.Post("http://localhost:11434/api/chat", "application/json", bytes.NewReader(bodyBytes))
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to connect to Ollama"})
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			c.JSON(http.StatusBadGateway, gin.H{"error": "Ollama returned error status"})
			return
		}

		// Set SSE headers
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		// Flush headers so client starts receiving
		c.Writer.Flush()

		// Read stream
		scanner := bufio.NewScanner(resp.Body)
		var assistantFullContent string
		
		for scanner.Scan() {
			line := scanner.Text()
			
			// Parse JSON to extract content and check if done
			var chunk struct {
				Model   string `json:"model"`
				Message struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"message"`
				Done bool `json:"done"`
			}
			
			if err := json.Unmarshal([]byte(line), &chunk); err == nil {
				assistantFullContent += chunk.Message.Content
			}

			// SSE format: data: <json>\n\n
			_, writeErr := c.Writer.Write([]byte("data: " + line + "\n\n"))
			if writeErr != nil {
				// Client disconnected
				break
			}
			c.Writer.Flush()
			
			if chunk.Done {
				break
			}
		}

		// Save the assistant's final accumulated response to the DB
		if assistantFullContent != "" {
			// Do this in background context in case request was cancelled
			go func(convID, content string) {
				_ = q.SaveLLMMessage(context.Background(), storage.LLMMessage{
					ConversationID: convID,
					Role:           "assistant",
					Content:        content,
				})
			}(req.ConversationID, assistantFullContent)
		}
	}
}

// GetChatHistoryHandler retrieves the conversation history for a given UUID.
func GetChatHistoryHandler(q storage.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		conversationID := c.Param("conversation_id")
		if conversationID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "conversation_id is required"})
			return
		}

		msgs, err := q.GetConversationHistory(c.Request.Context(), conversationID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve history"})
			return
		}

		c.JSON(http.StatusOK, msgs)
	}
}
