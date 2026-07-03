package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/viewred/viewred/internal/rules"
	"github.com/viewred/viewred/internal/storage"
)

// GetRules handles GET /api/v1/rules
func GetRules(q storage.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		rules, err := q.GetCustomRules(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if rules == nil {
			rules = []storage.CustomRule{}
		}
		c.JSON(http.StatusOK, rules)
	}
}

type createRuleRequest struct {
	RuleText string `json:"rule_text" binding:"required"`
	Enabled  bool   `json:"enabled"`
}

// CreateRule handles POST /api/v1/rules
func CreateRule(q storage.Querier, eng *rules.Engine) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createRuleRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
			return
		}

		id, err := q.SaveCustomRule(c.Request.Context(), req.RuleText, req.Enabled)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if eng != nil && req.Enabled {
			eng.AddRule(req.RuleText)
		}

		c.JSON(http.StatusCreated, gin.H{
			"id":        id,
			"rule_text": req.RuleText,
			"enabled":   req.Enabled,
		})
	}
}

// DeleteRule handles DELETE /api/v1/rules/:id
func DeleteRule(q storage.Querier, eng *rules.Engine) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID format"})
			return
		}

		// Get SID before deleting to hot-reload the engine
		var sid string
		if eng != nil {
			allRules, _ := q.GetCustomRules(c.Request.Context())
			for _, r := range allRules {
				if r.ID == id {
					parsed, err := rules.Parse(r.RuleText)
					if err == nil && parsed != nil {
						sid = parsed.Options["sid"]
					}
					break
				}
			}
		}

		if err := q.DeleteCustomRule(c.Request.Context(), id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if eng != nil && sid != "" {
			eng.RemoveRule(sid)
		}

		c.Status(http.StatusNoContent)
	}
}
