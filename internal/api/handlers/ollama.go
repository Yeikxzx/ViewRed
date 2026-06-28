package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/viewred/viewred/internal/config"
)

// OllamaTestDeps holds the dependencies for the Ollama test endpoint.
type OllamaTestDeps struct {
	Config *config.Config
}

// OllamaTest returns a handler that probes the configured Ollama
// endpoint and reports whether the configured model is available. The
// test makes a short-lived HTTP request to the Ollama /api/tags
// endpoint (which is public and does not require auth) and parses
// the response for the requested model.
//
// Response shape:
//   200: { status, model, available, ollama_url }
//   503: { status, error, ollama_url }
func OllamaTest(deps OllamaTestDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ollamaURL := deps.Config.AI.OllamaURL
		if ollamaURL == "" {
			ollamaURL = "http://localhost:11434"
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		url := ollamaURL + "/api/tags"
		req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":     "unreachable",
				"error":      err.Error(),
				"ollama_url": ollamaURL,
			})
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":     "error",
				"error":      fmt.Sprintf("Ollama returned HTTP %d", resp.StatusCode),
				"ollama_url": ollamaURL,
			})
			return
		}

		var tagsResp struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":     "error",
				"error":      "Failed to parse Ollama response: " + err.Error(),
				"ollama_url": ollamaURL,
			})
			return
		}

		wanted := deps.Config.AI.OllamaModel
		available := false
		var found string
		for _, m := range tagsResp.Models {
			if m.Name == wanted {
				available = true
				found = m.Name
				break
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"status":     "ok",
			"model":      wanted,
			"found":      found,
			"available":  available,
			"ollama_url": ollamaURL,
		})
	}
}