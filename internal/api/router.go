package api

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"github.com/viewred/viewred/internal/api/handlers"
)

func NewRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	v1 := r.Group("/api/v1")
	{
		v1.GET("/health", handlers.Health)
	}

	if dist := findWebDist(); dist != "" {
		r.StaticFS("/assets", http.Dir(filepath.Join(dist, "assets")))
		r.StaticFile("/", filepath.Join(dist, "index.html"))
		r.StaticFile("/index.html", filepath.Join(dist, "index.html"))
	} else {
		r.NoRoute(func(c *gin.Context) {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "not_found",
				"message": "Web UI not built. Run `make web-build` or start Vite dev server.",
			})
		})
	}

	return r
}

func findWebDist() string {
	candidates := []string{
		"web/dist",
		"../web/dist",
		"./web/dist",
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	return ""
}
