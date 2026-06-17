package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server  ServerConfig  `yaml:"server"`
	AI      AIConfig      `yaml:"ai"`
	Scanner ScannerConfig `yaml:"scanner"`
}

type ServerConfig struct {
	BindAddress string `yaml:"bind_address"`
	Port        int    `yaml:"port"`
}

type AIConfig struct {
	OllamaURL           string  `yaml:"ollama_url"`
	OllamaModel         string  `yaml:"ollama_model"`
	MLAnomalyThreshold  float64 `yaml:"ml_anomaly_threshold"`
	LLMTemperature      float64 `yaml:"llm_temperature"`
}

type ScannerConfig struct {
	NmapPath           string `yaml:"nmap_path"`
	MaxConcurrentScans int    `yaml:"max_concurrent_scans"`
	ScanTimeoutSeconds int    `yaml:"scan_timeout_seconds"`
}

func defaults() Config {
	return Config{
		Server: ServerConfig{
			BindAddress: "127.0.0.1",
			Port:        8080,
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
