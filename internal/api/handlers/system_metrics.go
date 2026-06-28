package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

// SystemMetrics represents real CPU, RAM, and uptime.
type SystemMetrics struct {
	CPUPercent float64 `json:"cpu_percent"`
	RAMPercent float64 `json:"ram_percent"`
	RAMUsedMB  uint64  `json:"ram_used_mb"`
	RAMTotalMB uint64  `json:"ram_total_mb"`
	UptimeSecs int64   `json:"uptime_seconds"`
}

// GetSystemMetrics handles GET /api/v1/system/metrics
func GetSystemMetrics(startedAt time.Time) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()

		cpuPct, err := cpu.PercentWithContext(ctx, 0, false)
		if err != nil {
			cpuPct = []float64{0}
		}
		memStats, err := mem.VirtualMemoryWithContext(ctx)
		if err != nil {
			memStats = &mem.VirtualMemoryStat{UsedPercent: 0, Used: 0, Total: 1}
		}

		c.JSON(http.StatusOK, SystemMetrics{
			CPUPercent: round1(cpuPct[0]),
			RAMPercent: round1(memStats.UsedPercent),
			RAMUsedMB:  memStats.Used / 1024 / 1024,
			RAMTotalMB: memStats.Total / 1024 / 1024,
			UptimeSecs: int64(time.Since(startedAt).Seconds()),
		})
	}
}

func round1(v float64) float64 {
	return float64(int(v*10)) / 10
}
