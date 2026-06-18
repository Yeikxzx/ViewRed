// Package handlers contains HTTP handlers for the ViewRed API.
//
// The devices handler serves GET /api/v1/devices: a paginated list of
// discovered hosts for the React device table view. It accepts the
// limit/offset query parameters, builds a storage.DeviceFilter, and
// returns the matching rows plus a total count for the "page N of M"
// footer. The handler is purely a translation layer between the HTTP
// request and the storage.Querier contract — it owns query-string
// parsing, pagination clamping, and the JSON shape, but never runs SQL.
//
// Query-param defaults: limit=100, offset=0. The 100 default matches
// the React device table's default page size; the storage layer's own
// default is 50, so a buggy handler that relied on storage defaults
// would silently halve the visible device set.
package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/viewred/viewred/internal/storage"
)

// devicesDefaultLimit is the page size used when the caller omits
// `limit`. 100 matches the React device table's default page size
// and gives the operator a useful window of recent devices in a
// single round trip.
const devicesDefaultLimit = 100

// devicesMaxLimit caps the `limit` query parameter. 500 matches the
// storage layer's cap (maxDevicesLimit in queries.go) so the handler
// and the SQL agree on the upper bound.
const devicesMaxLimit = 500

// devicesDefaultOffset is the offset used when the caller omits
// `offset`. Zero is the natural choice (start of the result set).
const devicesDefaultOffset = 0

// DevicesList returns a Gin handler that serves
// `GET /api/v1/devices`.
//
// Query parameters:
//
//	limit:  optional integer in [1, devicesMaxLimit]. Defaults to
//	        devicesDefaultLimit. Negative or non-numeric values 400;
//	        values above the cap are clamped to devicesMaxLimit.
//	offset: optional non-negative integer. Defaults to 0.
//
// On success: 200 with `{"items": [...], "total": N}`. On querier
// error: 503 with `{"error": "service_unavailable", "message": "..."}`.
// On invalid query params: 400 with `{"error": "bad_request", "message": "..."}`.
//
// The items slice is the raw storage.Device rows in storage order
// (last_seen DESC, most recently active first). The handler maps each
// device to a snake-case JSON shape so the HTTP contract is decoupled
// from the storage package's field names. The RiskScore field is
// *float64 on the wire: a nil pointer serialises as JSON `null`,
// distinguishing "not yet scored" from "scored 0.0".
func DevicesList(q storage.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		filter, err := parseDevicesFilter(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "bad_request",
				"message": err.Error(),
			})
			return
		}

		devices, total, qerr := q.QueryDevices(c.Request.Context(), filter)
		if qerr != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":   "service_unavailable",
				"message": qerr.Error(),
			})
			return
		}

		// Map to a snake_case JSON shape. Doing the translation here
		// (rather than relying on struct tags on storage.Device)
		// keeps the storage package free of HTTP-layer concerns.
		items := make([]gin.H, 0, len(devices))
		for _, d := range devices {
			items = append(items, deviceJSON(d))
		}

		c.JSON(http.StatusOK, gin.H{
			"items": items,
			"total": total,
		})
	}
}

// parseDevicesFilter resolves every documented query parameter into a
// storage.DeviceFilter. Invalid values return an error so the handler
// can map them to 400; valid-but-clamped values (e.g. limit > max) are
// silently clamped because the alternative — 400 — punishes the
// client for a benign mistake.
//
// The function is intentionally minimal: devices have no time
// window or text filters yet (those land with vendor/hostname search
// in a future task), so the only fields to parse are the pagination
// knobs.
func parseDevicesFilter(c *gin.Context) (storage.DeviceFilter, error) {
	var f storage.DeviceFilter

	// limit — must be a positive integer; values above the cap are
	// clamped to devicesMaxLimit. The storage layer applies its own
	// clamp as a final guard, but doing it here too means the value
	// the handler reports to the JSON response (if echoed) matches
	// what storage actually queried.
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return f, errors.New("limit: must be a positive integer")
		}
		if n > devicesMaxLimit {
			n = devicesMaxLimit
		}
		f.Limit = n
	} else {
		f.Limit = devicesDefaultLimit
	}

	// offset — must be a non-negative integer. There is no upper
	// bound on offset (the SQL handles large offsets via
	// chunk-pruned scans); we only reject negatives.
	if raw := c.Query("offset"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			return f, errors.New("offset: must be a non-negative integer")
		}
		f.Offset = n
	} else {
		f.Offset = devicesDefaultOffset
	}

	return f, nil
}

// deviceJSON renders a single storage.Device as a snake-case JSON
// object. The storage.Device type lives in the storage package, which
// is intentionally HTTP-agnostic — adding `json:` tags there would
// couple the storage code to the API's shape, and a future internal
// consumer of Device (e.g. an exports CLI) would carry those tags
// around. Mapping at the handler boundary is the cleaner split.
//
// The RiskScore *float64 is passed through directly: a nil pointer
// encodes as JSON `null`, which the React device table renders as
// "—" (not yet scored), distinct from a 0.0 score.
func deviceJSON(d storage.Device) gin.H {
	return gin.H{
		"id":         d.ID,
		"ip":         d.IP,
		"mac":        d.MAC,
		"vendor":     d.Vendor,
		"hostname":   d.Hostname,
		"first_seen": d.FirstSeen,
		"last_seen":  d.LastSeen,
		"risk_score": d.RiskScore,
	}
}
