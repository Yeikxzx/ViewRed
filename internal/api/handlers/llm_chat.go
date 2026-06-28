package handlers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

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

// systemPrompt defines the AI assistant's role, capabilities, and knowledge
// about ViewRed. It is prepended to every Ollama request as a system message
// so the model has context regardless of which client connects.
const systemPrompt = `Eres un asistente de ciberseguridad especializado en ViewRed, un sistema de monitoreo y detección de redes IIoT (Industrial Internet of Things).

## Tu Rol
- Asistente de SecOps para operadores de seguridad de redes industriales
- Respondés en español rioplatense, con tono profesional pero accesible
- Tus respuestas son concisas y técnicamente precisas
- **Siempre respondés basándote en los datos reales que se te proporcionan abajo en "## Datos de tu red en tiempo real". Si la info no está ahí, decís que no tenés esos datos en lugar de inventarlos.**

## ViewRed — Stack
- Backend: Go, PostgreSQL + TimescaleDB, Gin router, SSE broker
- Frontend: React 19, TypeScript, Zustand, Recharts
- Captura: Npcap (Windows) / AF_PACKET (Linux)

## Motor ML — Pipeline
1. Captura de paquetes → PacketEvent
2. Extracción de 7 features: [Length, Proto_Idx, SrcPort, DstPort, PacketsPerSec, UniqueDstPorts, PayloadEntropy]
3. Isolation Forest (anomaly detection): si detecta anomalía →
4. Random Forest (clasificación): 6 clases de ataque:
   - 0: DoS/DDoS Flood — severidad CRITICAL
   - 1: Recon / Scanning — severidad WARNING
   - 2: Web Attack (SQLi, XSS, Command Injection) — severidad HIGH
   - 3: Brute Force (SSH/Telnet) — severidad HIGH
   - 4: MITM (ARP spoofing, impersonación) — severidad CRITICAL
   - 5: Malware (Mirai) — severidad CRITICAL

## Alertas
- Severidades: INFO, WARNING, HIGH, CRITICAL
- Origen: ML Engine (91.27% accuracy) o Rules Engine (Suricata)
- Las alertas ML se generan por paquete; las de reglas por firma

## Capacidades
- Explicar alertas de seguridad en lenguaje natural
- Recomendar mitigaciones para CVEs y ataques detectados
- Describir patrones de tráfico anómalo
- Ayudar con configuración de reglas de detección
- NO ejecutás comandos ni modificás configuraciones — solo asesorás`;

// buildNetworkContext queries the database and returns a plain-text summary
// of the current network state (vulnerabilities, devices, alerts) that is
// appended to the system prompt so the model responds with real data.
func buildNetworkContext(ctx context.Context, q storage.Querier) string {
	var sb strings.Builder
	now := time.Now()
	// Usamos formato sin segundos para favorecer el cacheo del prompt (KV cache de Ollama)
	sb.WriteString(fmt.Sprintf("\n\n## Datos de tu red en tiempo real (actualizado: %s)\n", now.Format("2006-01-02 15:04")))

	// --- Vulnerabilidades ---
	vulns, err := q.GetVulnerabilities(ctx, false)
	if err == nil && len(vulns) > 0 {
		sb.WriteString(fmt.Sprintf("\n### Vulnerabilidades activas (%d encontradas)\n", len(vulns)))
		count := 20
		if len(vulns) < count {
			count = len(vulns)
		}
		for _, v := range vulns[:count] {
			score := "N/A"
			if v.CVSSScore != nil {
				score = fmt.Sprintf("%.1f", *v.CVSSScore)
			}
			cve := "N/A"
			if v.CVEID != nil {
				cve = *v.CVEID
			}
			port := "N/A"
			if v.Port != nil {
				port = fmt.Sprintf("%d", *v.Port)
			}
			sb.WriteString(fmt.Sprintf("- IP: %s | Puerto: %s | %s | CVSS: %s | %s\n",
				v.DeviceIP, port, cve, score, truncateText(v.Description, 100)))
		}
	} else {
		sb.WriteString("\n### Vulnerabilidades\nNo se detectaron vulnerabilidades activas.\n")
	}

	// --- Dispositivos ---
	devices, totalDevices, err := q.QueryDevices(ctx, storage.DeviceFilter{Limit: 30, Offset: 0})
	if err == nil && len(devices) > 0 {
		sb.WriteString(fmt.Sprintf("\n### Dispositivos en la red (%d en total, listando un máximo de %d)\n", totalDevices, len(devices)))
		for _, d := range devices {
			host := d.Hostname
			if host == "" {
				host = "sin hostname"
			}
			sb.WriteString(fmt.Sprintf("- IP: %s | MAC: %s | Vendor: %s | Hostname: %s | Ultima vez: %s\n",
				d.IP, d.MAC, d.Vendor, host, d.LastSeen.Format("15:04")))
		}
	}

	// --- Alertas recientes (última hora) ---
	// We filter via AlertFilter with a limit; exact time filtering is
	// handled by the alerts query (it returns most recent first).
	alerts, totalAlerts, err := q.QueryAlerts(ctx, storage.AlertFilter{Limit: 15, Offset: 0})
	if err == nil && len(alerts) > 0 {
		sb.WriteString(fmt.Sprintf("\n### Alertas recientes (%d en total, mostrando %d)\n", totalAlerts, len(alerts)))
		for _, a := range alerts {
			desc := ""
			if a.Description != nil {
				desc = *a.Description
			}
			sb.WriteString(fmt.Sprintf("- [%s] %s | Origen: %s | Score: %.0f | %s\n",
				a.Severity, a.Title, a.Source, a.Score, truncateText(desc, 120)))
		}
	} else {
		sb.WriteString("\n### Alertas\nNo hay alertas activas en este momento.\n")
	}

	return sb.String()
}

func truncateText(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
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

		// Construct payload for Ollama.
		// Build a system prompt with real-time network context so the
		// model responds with actual data instead of inventing it.
		networkCtx := buildNetworkContext(c.Request.Context(), q)
		fullSystemPrompt := systemPrompt + networkCtx

		// Trim the conversation history to the last N messages so the
		// prompt stays within the model's context window. The user
		// always gets their final question; older context is dropped.
		const maxHistoryMessages = 20
		historyMsgs := req.Messages
		if len(historyMsgs) > maxHistoryMessages {
			historyMsgs = historyMsgs[len(historyMsgs)-maxHistoryMessages:]
		}

		ollamaMessages := make([]struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}, 0, len(historyMsgs)+1)
		ollamaMessages = append(ollamaMessages, struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{Role: "system", Content: fullSystemPrompt})
		ollamaMessages = append(ollamaMessages, historyMsgs...)

		ollamaReq := OllamaRequest{
			Model:    "qwen2.5:3b",
			Messages: ollamaMessages,
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
