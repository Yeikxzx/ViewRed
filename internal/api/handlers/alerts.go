package handlers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/viewred/viewred/internal/storage"
)

// AlertsList returns a handler that paginates through alerts.
func AlertsList(q storage.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit := 100
		if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
			limit = l
		}
		if limit > 1000 {
			limit = 1000
		}

		offset := 0
		if o, err := strconv.Atoi(c.Query("offset")); err == nil && o >= 0 {
			offset = o
		}

		filter := storage.AlertFilter{
			Limit:  limit,
			Offset: offset,
		}

		alerts, total, err := q.QueryAlerts(c.Request.Context(), filter)
		if err != nil {
			log.Printf("handlers.AlertsList: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"alerts": alerts,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		})
	}
}

// AlertAcknowledge returns a handler that marks an alert as acknowledged.
func AlertAcknowledge(q storage.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid alert id"})
			return
		}

		err = q.AcknowledgeAlert(c.Request.Context(), id)
		if err != nil {
			log.Printf("handlers.AlertAcknowledge: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}
