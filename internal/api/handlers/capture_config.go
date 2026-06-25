package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/gopacket/pcap"

	"github.com/viewred/viewred/internal/config"
)

type NetworkInterface struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Addresses   []string `json:"addresses"`
}

// GetInterfaces handles GET /api/v1/interfaces
func GetInterfaces(c *gin.Context) {
	devs, err := pcap.FindAllDevs()
	if err != nil {
		// pcap unavailable (no Npcap, no permissions). Return an empty
		// list instead of 500 so the frontend can still load config.
		c.JSON(http.StatusOK, []NetworkInterface{})
		return
	}

	var interfaces []NetworkInterface
	for _, d := range devs {
		var addrs []string
		for _, a := range d.Addresses {
			addrs = append(addrs, a.IP.String())
		}
		interfaces = append(interfaces, NetworkInterface{
			Name:        d.Name,
			Description: d.Description,
			Addresses:   addrs,
		})
	}
	c.JSON(http.StatusOK, interfaces)
}

type CaptureConfigDeps struct {
	Config *config.Config
}

// GetCaptureConfig handles GET /api/v1/capture/config
func GetCaptureConfig(deps CaptureConfigDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, deps.Config.Capture)
	}
}

// UpdateCaptureConfig handles POST /api/v1/capture/config
func UpdateCaptureConfig(deps CaptureConfigDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req config.CaptureConfig
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
			return
		}

		deps.Config.Capture = req
		if err := deps.Config.Save(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save configuration: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, deps.Config.Capture)
	}
}
