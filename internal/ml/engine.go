//go:build cgo
// +build cgo

package ml

import (
	"fmt"
	"log"
	"path/filepath"
	"time"

	ort "github.com/yalue/onnxruntime_go"

	"github.com/viewred/viewred/internal/capture"
	"github.com/viewred/viewred/internal/storage"
)

// Engine runs inference on packets using ONNX Runtime.
type Engine struct {
	initialized bool
	isoSession  *ort.DynamicAdvancedSession
	rfSession   *ort.DynamicAdvancedSession
}

// NewEngine creates a new ML Engine.
func NewEngine(sharedLibraryPath, modelDir string) (*Engine, error) {
	if sharedLibraryPath != "" {
		ort.SetSharedLibraryPath(sharedLibraryPath)
	}

	err := ort.InitializeEnvironment()
	if err != nil {
		log.Printf("ml: ONNX runtime init failed (maybe dll missing?): %v. Starting in DISABLED mode.", err)
		return &Engine{initialized: false}, nil
	}

	if modelDir == "" {
		log.Println("ml: model directory empty, starting engine in disabled mode")
		return &Engine{initialized: false}, nil
	}

	isoPath := filepath.Join(modelDir, "isolation_forest.onnx")
	rfPath := filepath.Join(modelDir, "random_forest.onnx")

	// Create Isolation Forest session
	isoSession, err := ort.NewDynamicAdvancedSession(isoPath,
		[]string{"float_input"},
		[]string{"label", "scores"},
		nil,
	)
	if err != nil {
		log.Printf("ml: failed to load isolation forest from %s: %v. Starting in disabled mode.", isoPath, err)
		return &Engine{initialized: false}, nil
	}

	// Create Random Forest session
	rfSession, err := ort.NewDynamicAdvancedSession(rfPath,
		[]string{"float_input"},
		[]string{"output_label"},
		nil,
	)
	if err != nil {
		log.Printf("ml: failed to load random forest from %s: %v. Starting in disabled mode.", rfPath, err)
		return &Engine{initialized: false}, nil
	}

	log.Println("ml: ONNX engine successfully initialized with pre-trained models")
	return &Engine{
		initialized: true,
		isoSession:  isoSession,
		rfSession:   rfSession,
	}, nil
}

// Evaluate runs inference on a packet.
func (e *Engine) Evaluate(pkt capture.PacketEvent) (*storage.Alert, error) {
	if !e.initialized {
		return nil, nil
	}

	features := ExtractFeatures(pkt)

	inputShape := ort.NewShape(1, 3)
	inputTensor, err := ort.NewTensor(inputShape, features)
	if err != nil {
		return nil, fmt.Errorf("ml: failed to create input tensor: %w", err)
	}
	defer inputTensor.Destroy()

	// 1. Isolation Forest for Anomaly Detection
	isoLabelTensor, err := ort.NewEmptyTensor[int64](ort.NewShape(1))
	if err != nil {
		return nil, err
	}
	defer isoLabelTensor.Destroy()

	isoScoresTensor, err := ort.NewEmptyTensor[float32](ort.NewShape(1, 1))
	if err != nil {
		return nil, err
	}
	defer isoScoresTensor.Destroy()

	err = e.isoSession.Run(
		[]ort.ArbitraryTensor{inputTensor},
		[]ort.ArbitraryTensor{isoLabelTensor, isoScoresTensor},
	)
	if err != nil {
		return nil, fmt.Errorf("ml: isolation forest inference failed: %w", err)
	}

	isoLabelData := isoLabelTensor.GetData()
	if len(isoLabelData) == 0 {
		return nil, nil
	}

	// isolation forest label: -1 for anomaly, 1 for normal
	if isoLabelData[0] != -1 {
		return nil, nil // Not anomalous
	}

	// 2. It's anomalous! Run Random Forest to classify
	rfLabelTensor, err := ort.NewEmptyTensor[int64](ort.NewShape(1))
	if err != nil {
		return nil, err
	}
	defer rfLabelTensor.Destroy()

	err = e.rfSession.Run(
		[]ort.ArbitraryTensor{inputTensor},
		[]ort.ArbitraryTensor{rfLabelTensor},
	)
	if err != nil {
		return nil, fmt.Errorf("ml: random forest inference failed: %w", err)
	}

	rfLabelData := rfLabelTensor.GetData()
	if len(rfLabelData) == 0 {
		return nil, nil
	}

	classLabel := rfLabelData[0]
	
	category := "Generic Anomaly"
	descriptionStr := fmt.Sprintf("Anomalous packet detected from %s to %s (Len: %d).", pkt.SrcIP, pkt.DstIP, pkt.Length)
	severity := "WARNING"

	switch classLabel {
	case 0:
		category = "Unusual Web Traffic"
		descriptionStr += " Random Forest classified as web traffic with abnormal characteristics."
	case 1:
		category = "DNS Anomaly"
		descriptionStr += " Possible DNS tunnel or unusually large query."
		severity = "HIGH"
	case 2:
		category = "Suspicious Port Traffic"
		descriptionStr += " Communication to a non-standard high port detected by RF."
		severity = "CRITICAL"
	case 3:
		category = "ICMP Anomaly"
		descriptionStr += " Anomalous ICMP/ping pattern (possible sweep)."
		severity = "HIGH"
	}

	alert := &storage.Alert{
		Time:        time.Now().UTC(),
		Severity:    severity,
		Title:       category,
		Description: &descriptionStr,
		Source:      fmt.Sprintf("%s:%d", pkt.SrcIP, pkt.SrcPort),
		Score:       85.0, // Confidence score placeholder
		Context:     []byte(fmt.Sprintf(`{"origin":"ml_engine","model":"isolation_forest+random_forest"}`)),
	}

	return alert, nil
}

// Close cleans up ONNX runtime resources.
func (e *Engine) Close() {
	if e.isoSession != nil {
		_ = e.isoSession.Destroy()
	}
	if e.rfSession != nil {
		_ = e.rfSession.Destroy()
	}
	_ = ort.DestroyEnvironment()
}
