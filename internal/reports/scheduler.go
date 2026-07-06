package reports

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/viewred/viewred/internal/storage"
)

type AutoReportConfig struct {
	Enabled  bool   `json:"enabled"`
	Interval string `json:"interval"` // "6h", "8h", "24h"
}

// StartScheduler begins a background goroutine that checks the report config
// and triggers automatic generation.
func StartScheduler(ctx context.Context, querier storage.Querier, outputDir string) {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		lastRun := time.Now()

		for {
			select {
			case <-ctx.Done():
				return
			case t := <-ticker.C:
				config := loadConfig(outputDir)
				if !config.Enabled {
					continue
				}

				intervalDuration := parseInterval(config.Interval)
				if intervalDuration == 0 {
					continue
				}

				// Check if we need to run
				if t.Sub(lastRun) >= intervalDuration {
					log.Printf("reports: running scheduled report generation for interval %s", config.Interval)
					
					since := t.Add(-intervalDuration)
					_, err := GenerateReport(ctx, querier, since, t, nil, outputDir)
					if err != nil {
						log.Printf("reports: failed to generate scheduled report: %v", err)
					} else {
						lastRun = t
					}
				}
			}
		}
	}()
}

func loadConfig(outputDir string) AutoReportConfig {
	configPath := filepath.Join(outputDir, "report_config.json")
	config := AutoReportConfig{Enabled: false, Interval: "24h"}
	
	file, err := os.Open(configPath)
	if err == nil {
		defer file.Close()
		bytes, _ := io.ReadAll(file)
		json.Unmarshal(bytes, &config)
	}
	return config
}

func parseInterval(intervalStr string) time.Duration {
	switch intervalStr {
	case "6h":
		return 6 * time.Hour
	case "8h":
		return 8 * time.Hour
	case "24h":
		return 24 * time.Hour
	default:
		d, err := time.ParseDuration(intervalStr)
		if err != nil {
			return 24 * time.Hour
		}
		return d
	}
}
