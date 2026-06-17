package main

import (
	"fmt"
	"log"
	"os"

	"github.com/viewred/viewred/internal/api"
)

var Version = "0.1.0-dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("viewred %s\n", Version)
		return
	}

	addr := os.Getenv("VIEWRED_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}

	router := api.NewRouter()
	log.Printf("ViewRed %s listening on http://%s", Version, addr)

	if err := router.Run(addr); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
