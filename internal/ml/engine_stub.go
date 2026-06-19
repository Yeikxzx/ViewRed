//go:build !cgo
// +build !cgo

package ml

import (
	"log"

	"github.com/viewred/viewred/internal/capture"
	"github.com/viewred/viewred/internal/storage"
)

// Engine runs inference on packets.
// This is a stub for environments compiled without CGO.
type Engine struct {
	initialized bool
}

// NewEngine creates a new ML Engine stub.
func NewEngine(sharedLibraryPath, modelPath string) (*Engine, error) {
	log.Println("ml: built without CGO, ML engine is disabled")
	return &Engine{initialized: false}, nil
}

// Evaluate runs inference on a packet.
func (e *Engine) Evaluate(pkt capture.PacketEvent) (*storage.Alert, error) {
	return nil, nil
}

// Close cleans up resources.
func (e *Engine) Close() {
	// No-op
}
