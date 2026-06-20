package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/viewred/viewred/internal/scanner"
	"github.com/viewred/viewred/internal/storage"
)

// GetVulnerabilities handles GET /api/v1/vulnerabilities
func GetVulnerabilities(q storage.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		includeMitigated := c.Query("include_mitigated") == "true"
		vulns, err := q.GetVulnerabilities(c.Request.Context(), includeMitigated)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if vulns == nil {
			vulns = []storage.Vulnerability{}
		}
		c.JSON(http.StatusOK, vulns)
	}
}

type ScanRequest struct {
	IP      string `json:"ip" binding:"required"`
	Profile string `json:"profile"`
}

// TriggerScan handles POST /api/v1/scan
// For V1, this blocks until the scan completes to simplify the frontend polling.
func TriggerScan(q storage.Querier) gin.HandlerFunc {
	scanEngine := scanner.NewEngine(q)
	
	return func(c *gin.Context) {
		var req ScanRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
			return
		}

		if req.Profile == "" {
			req.Profile = "quick"
		}

		// Blocking scan execution for V1 simplicity
		err := scanEngine.Scan(c.Request.Context(), req.IP, req.Profile)
		if err != nil {
			// Instead of a full 500, we can return 500 but note that nmap failed.
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Scan failed", "details": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Scan completed successfully"})
	}
}

// MitigateVulnerability handles POST /api/v1/vulnerabilities/:id/mitigate
func MitigateVulnerability(q storage.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID format"})
			return
		}

		if err := q.MitigateVulnerability(c.Request.Context(), id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.Status(http.StatusNoContent)
	}
}
