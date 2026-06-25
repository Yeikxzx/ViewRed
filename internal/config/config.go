// Package config loads ViewRed configuration from a YAML file with safe defaults.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration.
type Config struct {
	SourcePath string         `yaml:"-"` // Not serialized
	Server   ServerConfig   `yaml:"server"`
	Capture  CaptureConfig  `yaml:"capture"`
	AI       AIConfig       `yaml:"ai"`
	Scanner  ScannerConfig  `yaml:"scanner"`
	Database DatabaseConfig `yaml:"database"`
}

type CaptureConfig struct {
	Interface string `yaml:"interface"`
	BPF       string `yaml:"bpf"`
	Snaplen   int    `yaml:"snaplen"`
}

type ServerConfig struct {
	BindAddress string   `yaml:"bind_address"`
	Port        int      `yaml:"port"`
	CORSOrigins []string `yaml:"cors_origins"`
}

type AIConfig struct {
	OllamaURL          string  `yaml:"ollama_url"`
	OllamaModel        string  `yaml:"ollama_model"`
	MLAnomalyThreshold float64 `yaml:"ml_anomaly_threshold"`
	LLMTemperature     float64 `yaml:"llm_temperature"`
}

type ScannerConfig struct {
	NmapPath           string `yaml:"nmap_path"`
	MaxConcurrentScans int    `yaml:"max_concurrent_scans"`
	ScanTimeoutSeconds int    `yaml:"scan_timeout_seconds"`
}

// DatabaseConfig holds the PostgreSQL+TimescaleDB connection settings.
type DatabaseConfig struct {
	URL string `yaml:"url"`
}

func defaults() Config {
	return Config{
		Server: ServerConfig{
			BindAddress: "127.0.0.1",
			Port:        8080,
			// Dev default: permit the Vite dev server so the
			// React UI can reach the API without a CORS error
			// in `make web-dev`. Production deployments
			// override this via server.cors_origins in
			// config.yaml. Leaving the slice empty would skip
			// the CORS middleware entirely (see
			// internal/api/router.go), which is the wrong
			// default for a dev-friendly binary.
			CORSOrigins: []string{"http://localhost:5173", "http://127.0.0.1:3000"},
		},
		Capture: CaptureConfig{
			Interface: "",
			BPF:       "tcp port 80 or tcp port 443",
			Snaplen:   65535,
		},
		AI: AIConfig{
			OllamaURL:          "http://localhost:11434",
			OllamaModel:        "qwen2.5:3b",
			MLAnomalyThreshold: 0.85,
			LLMTemperature:     0.7,
		},
		Scanner: ScannerConfig{
			NmapPath:           "",
			MaxConcurrentScans: 5,
			ScanTimeoutSeconds: 600,
		},
		Database: DatabaseConfig{
			URL: "postgres://viewred:viewred_dev_only@localhost:5432/viewred?sslmode=disable",
		},
	}
}

func Load(path string) (Config, error) {
	cfg := defaults()
	cfg.SourcePath = path

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read config: %w", err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}

	return cfg, nil
}

// Save marshals the config to YAML and writes it to its SourcePath.
func (c *Config) Save() error {
	if c.SourcePath == "" {
		return fmt.Errorf("cannot save: no SourcePath defined")
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	
	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(c.SourcePath), 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	if err := os.WriteFile(c.SourcePath, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
