// Package config loads ViewRed configuration from a YAML file with safe defaults.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration.
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	AI       AIConfig       `yaml:"ai"`
	Scanner  ScannerConfig  `yaml:"scanner"`
	Database DatabaseConfig `yaml:"database"`
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
			CORSOrigins: []string{"http://localhost:5173"},
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
			URL: "postgres://viewred:viewred_dev_only@localhost:5432/viewred",
		},
	}
}

func Load(path string) (Config, error) {
	cfg := defaults()

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
