// Package handlers contains HTTP handlers for the ViewRed API.
//
// This file hosts the placeholder (stub) endpoints for features that
// have a stable route + JSON shape but no backing storage or business
// logic yet: alerts, alerts ack, topology, vulnerabilities, and config.
//
// Why static responses? Plan 3 wires the React UI shell to a real API
// surface so the frontend can build against the final route names and
// JSON shapes. Hitting these endpoints today returns the empty/static
// payload the UI should render in the "no data yet" state. Real
// behaviour lands in later plans (alerts in Plan 5, topology in Plan 6,
// vulns in Plan 6, config in Plan 8). All five handlers are
// dependency-free: they do not take a *storage.Querier or any other
// injected service, because the stub has nothing to query.
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// stubVersion is the value reported by ConfigGet as the `version`
// field. Bump it alongside the project's semver. The stub is the
// single source of truth for this constant until a real config
// provider is wired in (Plan 8).
const stubVersion = "0.1.0"





// VulnerabilitiesList handles GET /api/v1/vulnerabilities and returns
// an empty JSON array. Same UI-safety reason as AlertsList: a `null`
// body would break the React `.map(...)` call.
//
// Route: GET /api/v1/vulnerabilities
// Response: 200 with body `[]`.
func VulnerabilitiesList(c *gin.Context) {
	c.JSON(http.StatusOK, []any{})
}

// ConfigGet handles GET /api/v1/config and returns the static build
// metadata the React settings view renders: the viewred version
// (used in the "About" footer) and an empty `features` map (the UI
// will read it to know which feature toggles to show). The features
// key is present (not omitted) so the UI can iterate it safely.
//
// Route: GET /api/v1/config
// Response: 200 with body `{"version": "0.1.0", "features": {}}`.
func ConfigGet(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"version":  stubVersion,
		"features": gin.H{},
	})
}
