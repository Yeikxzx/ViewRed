package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/viewred/viewred/internal/api"
	"github.com/viewred/viewred/internal/config"
)

var Version = "0.1.0-dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("viewred %s\n", Version)
		return
	}

	cfgPath := os.Getenv("VIEWRED_CONFIG")
	if cfgPath == "" {
		if dir, err := os.UserConfigDir(); err == nil {
			cfgPath = filepath.Join(dir, "viewred", "config.yaml")
		}
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.BindAddress, cfg.Server.Port)
	router := api.NewRouter()
	log.Printf("ViewRed %s listening on http://%s", Version, addr)

	if err := router.Run(addr); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
