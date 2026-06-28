// Package handlers — small debug endpoint for the DNS resolver.
//
// The endpoint returns the resolver's current size and a small
// sample of the cache. It exists so operators can verify the
// capture pipeline is actually feeding DNS answers into the
// resolver without having to inspect the process from outside.
//
// The endpoint is mounted under /api/v1/dns/cache and is read-only.
// It is safe to expose: the response contains only IP→hostname
// mappings that the resolver has already learned from the local
// network's own DNS traffic, which is exactly the data the UI
// already surfaces in the packets list.
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/viewred/viewred/internal/resolver"
)

// DnsCacheGet returns a Gin handler that serves
// `GET /api/v1/dns/cache` with a snapshot of the resolver's
// state: total entries, expired count, and a small sample of
// the live mappings for debugging.
//
// The sample is capped at sampleLimit entries; the full cache is
// not dumped because the real cache can be large and most
// debugging is satisfied by "is anything there?". The full dump
// is left to a follow-up if an operator actually needs it.
func DnsCacheGet(r *resolver.Resolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		if r == nil {
			c.JSON(http.StatusOK, gin.H{
				"resolver": "disabled",
				"stats":    gin.H{},
			})
			return
		}
		stats := r.Stats()
		sample := r.Sample(sampleLimit)
		c.JSON(http.StatusOK, gin.H{
			"resolver": "enabled",
			"stats": gin.H{
				"size":    stats.Size,
				"expired": stats.Expired,
			},
			"sample": sample,
		})
	}
}

// sampleLimit caps how many entries the debug endpoint returns.
// A small number keeps the response cheap and is enough to
// confirm the cache is non-empty.
const sampleLimit = 20
