package handlers

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

type PcapFile struct {
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	Timestamp string `json:"timestamp"`
}

func ListPcaps(pcapDir string) gin.HandlerFunc {
	return func(c *gin.Context) {
		entries, err := os.ReadDir(pcapDir)
		if err != nil {
			if os.IsNotExist(err) {
				c.JSON(http.StatusOK, []PcapFile{})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read directory"})
			return
		}

		// Always return an empty array, not null
		files := []PcapFile{}
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".pcap" {
				info, err := entry.Info()
				if err != nil {
					continue
				}
				files = append(files, PcapFile{
					Name:      entry.Name(),
					Size:      info.Size(),
					Timestamp: info.ModTime().Format("2006-01-02T15:04:05Z07:00"),
				})
			}
		}
		
		// Optional: Sort files by Timestamp descending?
		// For simplicity, we just return them. Client can sort.
		c.JSON(http.StatusOK, files)
	}
}

func DeletePcap(pcapDir string) gin.HandlerFunc {
	return func(c *gin.Context) {
		filename := c.Param("filename")
		if filename == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Filename required"})
			return
		}

		// Prevent directory traversal
		if filepath.Base(filename) != filename {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid filename"})
			return
		}

		path := filepath.Join(pcapDir, filename)
		err := os.Remove(path)
		if err != nil {
			if os.IsNotExist(err) {
				c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete file"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func DeleteAllPcaps(pcapDir string) gin.HandlerFunc {
	return func(c *gin.Context) {
		entries, err := os.ReadDir(pcapDir)
		if err != nil {
			if os.IsNotExist(err) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read directory"})
			return
		}

		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".pcap" {
				os.Remove(filepath.Join(pcapDir, entry.Name()))
			}
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}
