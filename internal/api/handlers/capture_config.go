package handlers

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/google/gopacket/pcap"

	"github.com/viewred/viewred/internal/capture"
	"github.com/viewred/viewred/internal/config"
	"github.com/viewred/viewred/internal/sse"
)

// CaptureHandle groups the capture lifecycle objects that main.go owns
// so the capture restart handler can stop and start capture without
// requiring a full backend restart.
type CaptureHandle struct {
	Capturer    capture.PacketCapture
	Packets     chan capture.PacketEvent
	EvalPackets chan capture.PacketEvent
	Broker      *sse.Broker
}

type NetworkInterface struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Addresses   []string `json:"addresses"`
}

// GetInterfaces handles GET /api/v1/interfaces
func GetInterfaces(c *gin.Context) {
	devs, err := pcap.FindAllDevs()
	if err != nil {
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
	Config  *config.Config
	Capture *CaptureHandle
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

// RestartCapture handles POST /api/v1/capture/restart.
// Stops the current capture and starts a new one with the saved config,
// without requiring a full backend restart.
func RestartCapture(deps CaptureConfigDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := deps.Capture
		if h == nil || h.Capturer == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Capture not available"})
			return
		}

		cfg := deps.Config.Capture

		// Stop the old capture. This closes the Packets() channel and
		// causes the existing bridge goroutine in main.go to exit.
		if err := h.Capturer.Stop(); err != nil && err.Error() != "npcap: not running" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to stop capture: " + err.Error()})
			return
		}

		// Start the new capture with the saved interface and BPF filter.
		iface := cfg.Interface
		if iface == "" {
			iface = os.Getenv("VIEWRED_INTERFACE")
		}
		if err := h.Capturer.Start(iface, cfg.BPF); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start capture: " + err.Error()})
			return
		}

		// Spawn a new bridge goroutine for the fresh Packets() channel.
		go func() {
			for e := range h.Capturer.Packets() {
				select {
				case h.Packets <- e:
				default:
				}
				select {
				case h.EvalPackets <- e:
				default:
				}
			}
		}()

		// Broadcast the updated capture status to the frontend.
		if statusJSON, err := json.Marshal(map[string]string{"status": "running"}); err == nil {
			h.Broker.Broadcast(sse.Event{
				Name:  "capture-status",
				Topic: "capture-status",
				Data:  statusJSON,
			})
		}

		c.JSON(http.StatusOK, gin.H{"message": "Capture restarted successfully with interface " + iface})
	}
}

// StoragePolicy represents the storage policy configuration
// returned by the GET /api/v1/capture/storage-policy endpoint
// and accepted by the POST variant.
type StoragePolicy struct {
	MaxPcapSizeMB  int `json:"max_pcap_size_mb"`
	MaxPcapAgeHours int `json:"max_pcap_age_hours"`
}

// GetStoragePolicy handles GET /api/v1/capture/storage-policy.
// Returns the current pcap rotation and retention settings from config.
func GetStoragePolicy(deps CaptureConfigDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := deps.Config.Capture
		c.JSON(http.StatusOK, StoragePolicy{
			MaxPcapSizeMB:   cfg.MaxPcapSizeMB,
			MaxPcapAgeHours: cfg.MaxPcapAgeHours,
		})
	}
}

// UpdateStoragePolicy handles POST /api/v1/capture/storage-policy.
// Updates the pcap rotation and retention settings in the config file.
// The new values take effect on the next capture restart.
func UpdateStoragePolicy(deps CaptureConfigDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req StoragePolicy
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
			return
		}

		if req.MaxPcapSizeMB < 1 {
			req.MaxPcapSizeMB = 1
		}
		if req.MaxPcapAgeHours < 1 {
			req.MaxPcapAgeHours = 1
		}

		deps.Config.Capture.MaxPcapSizeMB = req.MaxPcapSizeMB
		deps.Config.Capture.MaxPcapAgeHours = req.MaxPcapAgeHours
		if err := deps.Config.Save(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save configuration: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, StoragePolicy{
			MaxPcapSizeMB:   deps.Config.Capture.MaxPcapSizeMB,
			MaxPcapAgeHours: deps.Config.Capture.MaxPcapAgeHours,
		})
	}
}

// StopCapture handles POST /api/v1/capture/stop.
// Stops capture and leaves it stopped — no restart.
func StopCapture(deps CaptureConfigDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := deps.Capture
		if h == nil || h.Capturer == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Capture not available"})
			return
		}

		if err := h.Capturer.Stop(); err != nil {
			if err.Error() == "npcap: not running" || err.Error() == "afpacket: not running" {
				c.JSON(http.StatusOK, gin.H{"message": "Capture already stopped"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to stop capture: " + err.Error()})
			return
		}

		// Broadcast paused status.
		if statusJSON, err := json.Marshal(map[string]string{"status": "paused"}); err == nil {
			h.Broker.Broadcast(sse.Event{
				Name:  "capture-status",
				Topic: "capture-status",
				Data:  statusJSON,
			})
		}

		c.JSON(http.StatusOK, gin.H{"message": "Capture stopped"})
	}
}
