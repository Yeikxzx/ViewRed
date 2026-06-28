package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/viewred/viewred/internal/geoip"
)

// GeoIPDeps holds the GeoIP service dependency.
type GeoIPDeps struct {
	GeoIP *geoip.Service
}

// GeoIPNetworkLocation handles GET /api/v1/geoip.
// Returns the geographic location of the network's public IP so the
// frontend can center a real-world map on the network's approximate
// location and place local devices as points near it.
func GeoIPNetworkLocation(deps GeoIPDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		loc, err := deps.GeoIP.NetworkLocation()
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, loc)
	}
}

// GeoIPLookup handles GET /api/v1/geoip/:ip.
// Returns the geographic location for a specific IP address.
func GeoIPLookup(deps GeoIPDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.Param("ip")
		loc, err := deps.GeoIP.Lookup(ip)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, loc)
	}
}