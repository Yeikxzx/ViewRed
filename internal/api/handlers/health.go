// Package handlers contains HTTP handlers for the ViewRed API.
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/viewred/viewred/internal/health"
)

// Health returns the current aggregated health report as JSON.
// The report includes the overall status, uptime, and a list of subsystem
// statuses. The HTTP status code reflects the overall status (200 for OK,
// 200 for Degraded, 503 for Down) — clients should check the body's "status"
// field for the precise value.
func Health(rg *health.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		report := rg.Check(c.Request.Context())
		status := http.StatusOK
		if report.Status == health.StatusDown {
			status = http.StatusServiceUnavailable
		}
		c.JSON(status, report)
	}
}
