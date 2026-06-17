package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadDefaultsWhenFileMissing(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1", cfg.Server.BindAddress)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "http://localhost:11434", cfg.AI.OllamaURL)
	assert.Equal(t, "qwen2.5:3b", cfg.AI.OllamaModel)
}

func TestLoadFromYAMLFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yaml := `
server:
  bind_address: 0.0.0.0
  port: 9000
ai:
  ollama_url: http://gpu-box.lan:11434
  ollama_model: qwen2.5:7b
`
	require.NoError(t, writeFile(path, yaml))

	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0", cfg.Server.BindAddress)
	assert.Equal(t, 9000, cfg.Server.Port)
	assert.Equal(t, "http://gpu-box.lan:11434", cfg.AI.OllamaURL)
	assert.Equal(t, "qwen2.5:7b", cfg.AI.OllamaModel)
	assert.Equal(t, 5, cfg.Scanner.MaxConcurrentScans)
	assert.Equal(t, 0.85, cfg.AI.MLAnomalyThreshold)
	assert.Equal(t, 600, cfg.Scanner.ScanTimeoutSeconds)
}

func TestLoadInvalidYAMLReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, writeFile(path, "not: valid: yaml: ["))

	_, err := Load(path)
	assert.Error(t, err)
}
