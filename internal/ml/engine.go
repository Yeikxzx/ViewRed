//go:build cgo
// +build cgo

package ml

import (
	"fmt"
	"log"
	"path/filepath"
	"sync"
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
	windowStats *WindowStats
	dedup       map[string]time.Time
	dedupMu     sync.Mutex
	dedupTTL    time.Duration
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
		windowStats: NewWindowStats(),
		dedup:       make(map[string]time.Time),
		dedupTTL:    5 * time.Minute,
	}, nil
}

// Evaluate runs inference on a packet.
func (e *Engine) Evaluate(pkt capture.PacketEvent) (*storage.Alert, error) {
	if !e.initialized {
		return nil, nil
	}

	// Record the packet in the sliding window first so the window stats
	// reflect this packet's contribution to the source IP's behavior.
	e.windowStats.Record(pkt.SrcIP, pkt.DstIP, pkt.DstPort, pkt.Length, pkt.Timestamp)

	features := ExtractFeatures(pkt, e.windowStats)

	inputShape := ort.NewShape(1, numFeatures)
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
		category = "DoS/DDoS Flood"
		descriptionStr += " Possible DoS/DDoS flood attack detected."
		severity = "CRITICAL"
	case 1:
		category = "Recon / Scanning"
		descriptionStr += " Possible reconnaissance or port scanning activity."
		severity = "WARNING"
	case 2:
		category = "Web Attack"
		descriptionStr += " Possible web attack (SQLi, XSS, Command Injection)."
		severity = "HIGH"
	case 3:
		category = "Brute Force"
		descriptionStr += " Possible brute force attack (SSH/Telnet)."
		severity = "HIGH"
	case 4:
		category = "MITM"
		descriptionStr += " Possible man-in-the-middle attack (ARP spoofing, impersonation)."
		severity = "CRITICAL"
	case 5:
		category = "Malware (Mirai)"
		descriptionStr += " Possible Mirai botnet activity detected."
		severity = "CRITICAL"
	}

	fingerprint := fmt.Sprintf("%s|%s|%s|%d", category, pkt.SrcIP, pkt.DstIP, pkt.DstPort)
	e.dedupMu.Lock()
	if e.dedup == nil {
		e.dedup = make(map[string]time.Time)
	}

	// GC check
	if len(e.dedup) > 1000 {
		now := time.Now()
		for k, v := range e.dedup {
			if now.Sub(v) > e.dedupTTL {
				delete(e.dedup, k)
			}
		}
	}

	lastSeen, ok := e.dedup[fingerprint]
	if ok && time.Since(lastSeen) < e.dedupTTL {
		e.dedupMu.Unlock()
		return nil, nil // Deduplicated
	}
	e.dedup[fingerprint] = time.Now()
	e.dedupMu.Unlock()

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

// Close cleans up ONNX runtime resources and the window tracker.
func (e *Engine) Close() {
	if e.windowStats != nil {
		e.windowStats.Close()
	}
	if e.isoSession != nil {
		_ = e.isoSession.Destroy()
	}
	if e.rfSession != nil {
		_ = e.rfSession.Destroy()
	}
	_ = ort.DestroyEnvironment()
}
