package middleware

import (
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORSConfig controls CORS behavior.
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
}

// NewCORS returns a Gin middleware that sets CORS headers and handles preflight.
func NewCORS(cfg CORSConfig) gin.HandlerFunc {
	originAllowed := func(origin string) bool {
		return slices.Contains(cfg.AllowedOrigins, origin)
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}
		if !originAllowed(origin) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Vary", "Origin")
		if cfg.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		if c.Request.Method == http.MethodOptions {
			if len(cfg.AllowedMethods) > 0 {
				c.Header("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ", "))
			}
			if len(cfg.AllowedHeaders) > 0 {
				c.Header("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ", "))
			}
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
