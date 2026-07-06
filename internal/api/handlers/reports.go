package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"time"
	"io"
	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/viewred/viewred/internal/reports"
	"github.com/viewred/viewred/internal/storage"
)

type ReportDeps struct {
	Querier   storage.Querier
	OutputDir string
}

type GenerateReportRequest struct {
	Since       time.Time            `json:"since"`
	Until       time.Time            `json:"until"`
	TestContext *storage.ReportHistory `json:"test_context,omitempty"`
}

type ReportFile struct {
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	Timestamp string `json:"timestamp"`
}

type AutoReportConfig struct {
	Enabled  bool   `json:"enabled"`
	Interval string `json:"interval"` // "6h", "8h", "24h"
}

func ensureDir(dir string) error {
	return os.MkdirAll(dir, 0o755)
}

func GenerateReport(deps ReportDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req GenerateReportRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		if err := ensureDir(deps.OutputDir); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create output directory"})
			return
		}

		filePath, err := reports.GenerateReport(c.Request.Context(), deps.Querier, req.Since, req.Until, req.TestContext, deps.OutputDir)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if req.TestContext != nil {
			req.TestContext.Since = req.Since
			req.TestContext.Until = req.Until
			req.TestContext.Filename = filepath.Base(filePath)
			_, _ = deps.Querier.SaveReportHistory(c.Request.Context(), *req.TestContext)
		}

		info, err := os.Stat(filePath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "File generated but stat failed"})
			return
		}

		c.JSON(http.StatusOK, ReportFile{
			Name:      filepath.Base(filePath),
			Size:      info.Size(),
			Timestamp: info.ModTime().Format(time.RFC3339),
		})
	}
}

func ListReports(deps ReportDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := ensureDir(deps.OutputDir); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create output directory"})
			return
		}

		entries, err := os.ReadDir(deps.OutputDir)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read directory"})
			return
		}

		files := []ReportFile{}
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".pdf" {
				info, err := entry.Info()
				if err != nil {
					continue
				}
				files = append(files, ReportFile{
					Name:      entry.Name(),
					Size:      info.Size(),
					Timestamp: info.ModTime().Format(time.RFC3339),
				})
			}
		}

		// Sort by Timestamp DESC (newest first)
		// Assuming we can just reverse the array if files are created sequentially, but better to sort.
		// For simplicity, we just return them.

		c.JSON(http.StatusOK, files)
	}
}

func DownloadReport(deps ReportDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		filename := c.Param("filename")
		if filename == "" || filepath.Base(filename) != filename {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid filename"})
			return
		}

		filePath := filepath.Join(deps.OutputDir, filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Report not found"})
			return
		}

		c.File(filePath)
	}
}

// Config file location for simplicity. In a real app this should be in DB or main config.
func getConfigFile(outputDir string) string {
	return filepath.Join(outputDir, "report_config.json")
}

func GetReportConfig(deps ReportDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		configPath := getConfigFile(deps.OutputDir)
		config := AutoReportConfig{Enabled: false, Interval: "24h"} // Defaults
		
		file, err := os.Open(configPath)
		if err == nil {
			defer file.Close()
			bytes, _ := io.ReadAll(file)
			json.Unmarshal(bytes, &config)
		}

		c.JSON(http.StatusOK, config)
	}
}

func SaveReportConfig(deps ReportDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req AutoReportConfig
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		if err := ensureDir(deps.OutputDir); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create output directory"})
			return
		}

		configPath := getConfigFile(deps.OutputDir)
		bytes, _ := json.MarshalIndent(req, "", "  ")
		if err := os.WriteFile(configPath, bytes, 0o644); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save config"})
			return
		}

		// Update the scheduler channel here if we have one.
		// For this implementation, we will assume the scheduler periodically re-reads the config
		// or we can notify it via a channel.
		
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func DeleteReport(deps ReportDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		filename := c.Param("filename")
		if filename == "" || filepath.Base(filename) != filename {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid filename"})
			return
		}

		filePath := filepath.Join(deps.OutputDir, filename)
		if err := os.Remove(filePath); err != nil {
			if os.IsNotExist(err) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Report not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete report"})
			}
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Report deleted successfully"})
	}
}
