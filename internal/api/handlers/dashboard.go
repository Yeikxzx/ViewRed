// Package handlers contains HTTP handlers for the ViewRed API.
//
// The dashboard handlers expose the three read-only KPI endpoints that
// back the React dashboard view: /summary (windowed throughput and
// device/connection counts), /top-talkers (heaviest IPs in the window),
// and /protocols (protocol share of traffic). All three take a
// storage.Querier at construction time so they can be wired against the
// real *storage.Storage in production and against storage.MockQuerier /
// storage.ErrorQuerier in tests. When the injected querier returns an
// error, the handler maps it to 503 so callers can distinguish "no
// data" (200, empty body) from "storage down" (503).
package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/viewred/viewred/internal/storage"
)

// dashboardDefaultWindow is the lookback applied when the caller does
// not supply a `since` query parameter. One hour matches the React
// dashboard's default filter and keeps the SQL aggregates cheap.
const dashboardDefaultWindow = 1 * time.Hour

// dashboardMaxTopTalkers caps the `limit` query parameter on
// /top-talkers so a malicious or buggy client cannot ask for millions
// of rows. 100 comfortably fits the bar chart in the UI.
const dashboardMaxTopTalkers = 100

// dashboardDefaultTopTalkers is the page size used when the caller
// omits `limit`. 10 fits the top-talkers card without scrolling.
const dashboardDefaultTopTalkers = 10

// sinceParam is the canonical query parameter name for the lookback
// window. Exposed as a const so tests and the router can reference the
// same string.
const sinceParam = "since"

// DashboardSummary returns the windowed KPI summary for the dashboard.
//
// Query parameters:
//
//	since: optional RFC3339 timestamp. Defaults to "one hour ago".
//
// On success: 200 with a JSON body containing packets_per_sec,
// bytes_per_sec, active_connections, device_count, and the resolved
// `since` echo. On querier error: 503 with an `error` field.
func DashboardSummary(q storage.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		since, err := parseSince(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		summary, qerr := q.QueryDashboardSummary(c.Request.Context(), since)
		if qerr != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":  "storage unavailable",
				"detail": qerr.Error(),
				"since":  since,
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"packets_per_sec":    summary.PacketsPerSec,
			"bytes_per_sec":      summary.BytesPerSec,
			"active_connections": summary.ActiveConnections,
			"device_count":       summary.DeviceCount,
			"since":              since,
		})
	}
}

// TopTalkers returns the heaviest IPs in the lookback window.
//
// Query parameters:
//
//	since: optional RFC3339 timestamp. Defaults to "one hour ago".
//	limit: optional 1..dashboardMaxTopTalkers. Defaults to
//	       dashboardDefaultTopTalkers. Values outside the range fall
//	       back to the default rather than 400-ing the request, so a
//	       bad client cannot DoS the dashboard with garbage.
//
// On success: 200 with `{"talkers": [...], "since": "..."}`. On querier
// error: 503.
func TopTalkers(q storage.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		since, err := parseSince(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		limit := parseTopTalkerLimit(c)

		talkers, qerr := q.QueryTopTalkers(c.Request.Context(), since, limit)
		if qerr != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":  "storage unavailable",
				"detail": qerr.Error(),
				"since":  since,
			})
			return
		}

		// Map to a snake_case JSON shape. Doing the translation here
		// (rather than relying on struct tags on storage.TopTalker)
		// keeps the storage package free of HTTP-layer concerns.
		out := make([]gin.H, 0, len(talkers))
		for _, t := range talkers {
			out = append(out, gin.H{
				"src_ip":  t.SrcIP,
				"bytes":   t.Bytes,
				"packets": t.Packets,
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"top_talkers": out,
			"since":       since,
		})
	}
}

// ProtocolDistribution returns the protocol share of traffic in the
// lookback window.
//
// Query parameters:
//
//	since: optional RFC3339 timestamp. Defaults to "one hour ago".
//
// On success: 200 with `{"protocols": [...], "since": "..."}`. On
// querier error: 503.
func ProtocolDistribution(q storage.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		since, err := parseSince(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		shares, qerr := q.QueryProtocolDistribution(c.Request.Context(), since)
		if qerr != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":  "storage unavailable",
				"detail": qerr.Error(),
				"since":  since,
			})
			return
		}

		out := make([]gin.H, 0, len(shares))
		for _, s := range shares {
			out = append(out, gin.H{
				"protocol":   s.Protocol,
				"percentage": s.Percentage,
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"protocols": out,
			"since":     since,
		})
	}
}

// parseSince resolves the `since` query parameter into a time.Time. An
// empty value means "use the default window". A non-empty value that
// cannot be parsed as RFC3339 returns an error so the handler can map
// it to 400. The default (one hour ago) is computed inside the handler
// — not at construction time — so long-lived handlers still report a
// rolling window instead of a fixed point in the past.
func parseSince(c *gin.Context) (time.Time, error) {
	raw := c.Query(sinceParam)
	if raw == "" {
		return time.Now().Add(-dashboardDefaultWindow), nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, errors.New("since: must be RFC3339 (e.g. 2026-06-18T10:00:00Z)")
	}
	return t, nil
}

// parseTopTalkerLimit resolves the `limit` query parameter for the
// top-talkers endpoint. Missing, non-numeric, or out-of-range values
// fall back to the default rather than producing a 400, so a noisy
// client cannot break the dashboard with a bad URL.
func parseTopTalkerLimit(c *gin.Context) int {
	raw := c.Query("limit")
	if raw == "" {
		return dashboardDefaultTopTalkers
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return dashboardDefaultTopTalkers
	}
	if n < 1 || n > dashboardMaxTopTalkers {
		return dashboardDefaultTopTalkers
	}
	return n
}
