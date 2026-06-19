//go:build cgo
// +build cgo

package ml

import (
	"fmt"
	"log"

	ort "github.com/yalue/onnxruntime_go"

	"github.com/viewred/viewred/internal/capture"
	"github.com/viewred/viewred/internal/storage"
)

// Engine runs inference on packets using ONNX Runtime.
type Engine struct {
	initialized bool
	session     *ort.AdvancedSession
}

// NewEngine creates a new ML Engine. 
// Requires sharedLibraryPath (e.g. "onnxruntime.dll") to be valid.
func NewEngine(sharedLibraryPath, modelPath string) (*Engine, error) {
	if sharedLibraryPath != "" {
		ort.SetSharedLibraryPath(sharedLibraryPath)
	}

	err := ort.InitializeEnvironment()
	if err != nil {
		return nil, fmt.Errorf("ml: failed to initialize ONNX runtime: %w", err)
	}

	// This is just a placeholder logic to initialize a session.
	// A real Random Forest or Isolation Forest model expects a specific shape.
	// For V1, we will gracefully handle missing model files and return an "empty" engine
	// if the model path is not provided, allowing the application to run without ML.
	if modelPath == "" {
		log.Println("ml: model path empty, starting engine in disabled mode")
		return &Engine{initialized: false}, nil
	}

	// Example: Create an AdvancedSession. The input/output shapes depend on the trained model.
	// We'll leave it as disabled for the stub until the model is ready.
	return &Engine{initialized: false}, nil
}

// Evaluate runs inference on a packet.
func (e *Engine) Evaluate(pkt capture.PacketEvent) (*storage.Alert, error) {
	if !e.initialized {
		return nil, nil
	}

	// Real V1 implementation would map packet properties to a []float32 tensor,
	// run session.Run(), and extract anomaly score from the output tensor.

	return nil, nil
}

// Close cleans up ONNX runtime resources.
func (e *Engine) Close() {
	if e.session != nil {
		_ = e.session.Destroy()
	}
	_ = ort.DestroyEnvironment()
}
