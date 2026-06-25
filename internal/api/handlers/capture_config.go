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
		if err := h.Capturer.Stop(); err != nil {
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
