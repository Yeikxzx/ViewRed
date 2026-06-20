// Package handlers contains HTTP handlers for the ViewRed API.
//
// The packets handler serves GET /api/v1/packets: a paginated,
// filterable list of captured packet events for the React packet
// search view. It accepts a time window (since/until),
// protocol/src_ip/dst_ip filters, and a paginated limit/offset, and
// returns the matching rows plus a total count for the "page N of M"
// footer. The handler is purely a translation layer between the HTTP
// request and the storage.Querier contract — it owns query-string
// parsing, RFC3339 validation, pagination clamping, and the JSON
// shape, but never runs SQL.
//
// Query-param defaults: since=1h ago, limit=100, offset=0. The
// since/limit defaults are handler-level (not storage-level) so a
// future SSE ticker or CLI caller can still use the storage layer's
// own defaults (50, no since) without inheriting the HTTP defaults.
package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/viewred/viewred/internal/capture"
	"github.com/viewred/viewred/internal/storage"
)

// packetsDefaultLimit is the page size used when the caller omits
// `limit`. 100 matches the React packet search view's default page
// size and gives the operator a useful window of recent traffic in a
// single round trip.
const packetsDefaultLimit = 100

// packetsMaxLimit caps the `limit` query parameter. 500 matches the
// storage layer's cap (maxPacketsLimit in queries.go) so the handler
// and the SQL agree on the upper bound.
const packetsMaxLimit = 500

// packetsDefaultOffset is the offset used when the caller omits
// `offset`. Zero is the natural choice (start of the result set).
const packetsDefaultOffset = 0

// PacketsList returns a Gin handler that serves
// `GET /api/v1/packets`.
//
// Query parameters:
//
//	since:    optional RFC3339 timestamp. Defaults to "one hour ago".
//	until:    optional RFC3339 timestamp. Defaults to "no upper bound".
//	protocol: optional. Matches the protocol column verbatim (TCP/UDP/ICMP/OTHER).
//	src_ip:   optional. Matches the source IP column verbatim.
//	dst_ip:   optional. Matches the destination IP column verbatim.
//	limit:    optional integer in [1, packetsMaxLimit]. Defaults to
//	          packetsDefaultLimit. Negative or non-numeric values 400;
//	          values above the cap are clamped to packetsMaxLimit.
//	offset:   optional non-negative integer. Defaults to 0.
//
// On success: 200 with `{"items": [...], "total": N}`. On querier
// error: 503 with `{"error": "service_unavailable", "message": "..."}`.
// On invalid query params: 400 with `{"error": "bad_request", "message": "..."}`.
//
// The items slice is the raw capture.PacketEvent rows in storage order
// (time DESC, newest first). The handler maps each event to a
// snake-case JSON shape so the HTTP contract is decoupled from the
// capture package's field names.
func PacketsList(q storage.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		filter, err := parsePacketsFilter(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "bad_request",
				"message": err.Error(),
			})
			return
		}

		events, total, qerr := q.QueryPackets(c.Request.Context(), filter)
		if qerr != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":   "service_unavailable",
				"message": qerr.Error(),
			})
			return
		}

		// Map to a snake_case JSON shape. Doing the translation here
		// (rather than relying on struct tags on capture.PacketEvent)
		// keeps the capture package free of HTTP-layer concerns. The
		// payload preview is a raw byte slice; JSON encodes it as
		// base64, which the React client can decode for display.
		items := make([]gin.H, 0, len(events))
		for _, ev := range events {
			items = append(items, packetEventJSON(ev))
		}

		c.JSON(http.StatusOK, gin.H{
			"packets": items,
			"total":   total,
			"limit":   filter.Limit,
			"offset":  filter.Offset,
		})
	}
}

// parsePacketsFilter resolves every documented query parameter into a
// storage.PacketFilter. Invalid values return an error so the handler
// can map them to 400; valid-but-clamped values (e.g. limit > max)
// are silently clamped because the alternative — 400 — punishes the
// client for a benign mistake.
//
// The function reuses the dashboard's parseSince helper for the
// `since` field so the two endpoints agree on the default-window and
// the RFC3339 error message.
func parsePacketsFilter(c *gin.Context) (storage.PacketFilter, error) {
	var f storage.PacketFilter

	// since — default to 1h ago via the shared dashboard helper.
	since, err := parseSince(c)
	if err != nil {
		return f, err
	}
	f.Since = since

	// until — optional. Empty string means "no upper bound"; the SQL
	// treats an unset Until the same as a zero-value time.Time
	// (time >= epoch zero), which the WHERE clause collapses into
	// "match all rows" via the same pattern as the empty filter
	// columns.
	if raw := c.Query("until"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return f, errors.New("until: must be RFC3339 (e.g. 2026-06-18T11:00:00Z)")
		}
		f.Until = t
	}

	// String filters — passed verbatim. The SQL's
	// ($N = '' OR col = $N) pattern treats an empty string as
	// "wildcard", so an absent filter is indistinguishable from
	// an explicitly empty one.
	f.Protocol = c.Query("protocol")
	f.SrcIP = c.Query("src_ip")
	f.DstIP = c.Query("dst_ip")
	f.PayloadSearch = c.Query("payload_search")

	// Port filter — passed as integer. A missing or invalid port
	// falls back to 0 which acts as a wildcard in the SQL.
	if raw := c.Query("port"); raw != "" {
		if p, err := strconv.Atoi(raw); err == nil && p > 0 && p <= 65535 {
			f.Port = p
		}
	}

	// limit — must be a positive integer; values above the cap are
	// clamped to packetsMaxLimit. The storage layer applies its own
	// clamp as a final guard, but doing it here too means the value
	// the handler reports to the JSON response (if echoed) matches
	// what storage actually queried.
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return f, errors.New("limit: must be a positive integer")
		}
		if n > packetsMaxLimit {
			n = packetsMaxLimit
		}
		f.Limit = n
	} else {
		f.Limit = packetsDefaultLimit
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
		f.Offset = packetsDefaultOffset
	}

	return f, nil
}

// packetEventJSON renders a single capture.PacketEvent as a
// snake-case JSON object. The capture.PacketEvent type lives in the
// capture package, which is intentionally HTTP-agnostic — adding
// `json:` tags there would couple the capture code to the API's
// shape, and a future internal consumer of PacketEvent would carry
// those tags around. Mapping at the handler boundary is the cleaner
// split.
func packetEventJSON(ev capture.PacketEvent) gin.H {
	return gin.H{
		"time":            ev.Timestamp,
		"src_ip":          ev.SrcIP,
		"dst_ip":          ev.DstIP,
		"src_port":        ev.SrcPort,
		"dst_port":        ev.DstPort,
		"protocol":        ev.Protocol,
		"length":          ev.Length,
		"payload_preview": ev.PayloadPreview,
	}
}
