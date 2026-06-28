package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/viewred/viewred/internal/config"
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

// ScanDeps holds the dependencies for the scan endpoint.
type ScanDeps struct {
	Querier storage.Querier
	Config  *config.Config
}

// TriggerScan handles POST /api/v1/scan
// For V1, this blocks until the scan completes to simplify the frontend polling.
func TriggerScan(deps ScanDeps) gin.HandlerFunc {
	scanEngine := scanner.NewEngine(deps.Querier, deps.Config.Scanner.NmapPath)
	
	return func(c *gin.Context) {
		var req ScanRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
			return
		}

		if req.Profile == "" {
			req.Profile = "quick"
		}

		start := time.Now()

		// Count vulns before the scan to compute delta.
		vulnsBefore, _ := deps.Querier.GetVulnerabilities(c.Request.Context(), false)
		countBefore := len(vulnsBefore)

		err := scanEngine.Scan(c.Request.Context(), req.IP, req.Profile)
		durationMs := int(time.Since(start).Milliseconds())

		// Persist scan history regardless of success/failure.
		vulnsAfter, _ := deps.Querier.GetVulnerabilities(c.Request.Context(), false)
		status := "COMPLETED"
		if err != nil {
			status = "STOPPED"
		}
		history := storage.ScanHistory{
			Target:       req.IP,
			Profile:      req.Profile,
			DurationMs:   durationMs,
			HostsScanned: 1,
			VulnsFound:   len(vulnsAfter) - countBefore,
			Status:       status,
		}
		if _, saveErr := deps.Querier.SaveScanHistory(c.Request.Context(), history); saveErr != nil {
			// Non-fatal: the scan itself succeeded, just log the issue.
		}

		if err != nil {
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
