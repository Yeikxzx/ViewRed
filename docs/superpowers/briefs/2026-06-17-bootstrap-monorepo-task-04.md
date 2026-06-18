# Task 4 Brief: Add YAML config loader (TDD)

**Plan**: `docs/superpowers/plans/2026-06-17-bootstrap-monorepo.md`
**Date**: 2026-06-17
**Project root (work from here)**: `C:\Users\Yeikxz\Desktop\ViewRed`

---

## Context

This is **Task 4 of 12** in the ViewRed bootstrap. Tasks 1, 2, 3 are done:
- Task 1 (`003fd25`): git init + config files
- Task 2 (`75d6b69`): Go module + main.go with --version
- Task 3 (`d32dd82`): Gin router + `/api/v1/health` endpoint (5/5 tests pass)

Your job: add a **YAML config loader** that:
- Returns safe defaults if the file is missing
- Parses user-provided YAML and merges with defaults
- Errors if the YAML is malformed
- Wires into `main.go` so the server uses the config's bind address and port

### Interfaces and decisions from earlier tasks

- **Module path**: `github.com/viewred/viewred` (do not change).
- **`cmd/viewred/main.go`**: has `Version` constant, `--version` flag, and Gin router wiring. You will MODIFY this file in Step 6 to use the config. Keep `Version` and `--version` flag intact.
- **Config struct fields** (defined in Step 4):
  - `Server.BindAddress` (string, default `127.0.0.1`)
  - `Server.Port` (int, default `8080`)
  - `AI.OllamaURL` (string, default `http://localhost:11434`)
  - `AI.OllamaModel` (string, default `qwen2.5:3b`)
  - `AI.MLAnomalyThreshold` (float64, default `0.85`)
  - `AI.LLMTemperature` (float64, default `0.7`)
  - `Scanner.NmapPath` (string, default `""`)
  - `Scanner.MaxConcurrentScans` (int, default `5`)
  - `Scanner.ScanTimeoutSeconds` (int, default `600`)
- **Config path resolution**: env var `VIEWRED_CONFIG` if set; otherwise `os.UserConfigDir()/viewred/config.yaml` (cross-platform: `%APPDATA%\viewred\config.yaml` on Windows, `~/.config/viewred/config.yaml` on Linux).
- **TDD strictly enforced**: 3 tests in `config_test.go`, all must pass.

### Files already in the directory (DO NOT TOUCH except as specified)

- `go.mod`, `go.sum` (will be modified by `go get` in Step 1)
- `cmd/viewred/main.go` (will be REPLACED in Step 6, but KEEP `Version` and `--version` flag)
- `cmd/viewred/main_test.go` (KEEP — tests `Version`)
- `internal/api/...` (KEEP — router and health handler)
- `internal/api/router_test.go` (KEEP)
- `internal/api/handlers/health_test.go` (KEEP)
- All other files: leave alone.

---

## Task: Add YAML config loader (TDD)

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Create: `internal/config/helpers_test.go` (test helper for writing files)
- Modify: `cmd/viewred/main.go` (replace with config-wired version, in Step 6)
- Modify: `go.mod` (yaml.v3 added via `go get`)

- [ ] **Step 1: Add yaml dependency**

```bash
cd C:\Users\Yeikxz\Desktop\ViewRed
go get gopkg.in/yaml.v3@latest
```

Expected: `go.mod` and `go.sum` updated.

- [ ] **Step 2: Write failing tests for config loader**

Create `C:\Users\Yeikxz\Desktop\ViewRed\internal\config\config_test.go`:

```go
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
}

func TestLoadInvalidYAMLReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, writeFile(path, "not: valid: yaml: ["))

	_, err := Load(path)
	assert.Error(t, err)
}
```

Create `C:\Users\Yeikxz\Desktop\ViewRed\internal\config\helpers_test.go`:

```go
package config

import "os"

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
cd C:\Users\Yeikxz\Desktop\ViewRed
go test ./internal/config/...
```

Expected: FAIL with `undefined: Load`, `undefined: Server`, etc.

- [ ] **Step 4: Implement config loader**

Create `C:\Users\Yeikxz\Desktop\ViewRed\internal\config\config.go`:

```go
// Package config loads ViewRed configuration from a YAML file with safe defaults.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration.
type Config struct {
	Server  ServerConfig  `yaml:"server"`
	AI      AIConfig      `yaml:"ai"`
	Scanner ScannerConfig `yaml:"scanner"`
}

// ServerConfig controls the HTTP server.
type ServerConfig struct {
	BindAddress string `yaml:"bind_address"`
	Port        int    `yaml:"port"`
}

// AIConfig controls the LLM and ML subsystems.
type AIConfig struct {
	OllamaURL    string  `yaml:"ollama_url"`
	OllamaModel  string  `yaml:"ollama_model"`
	MLAnomalyThreshold float64 `yaml:"ml_anomaly_threshold"`
	LLMTemperature float64 `yaml:"llm_temperature"`
}

// ScannerConfig controls the nmap-based vulnerability scanner.
type ScannerConfig struct {
	NmapPath            string `yaml:"nmap_path"`
	MaxConcurrentScans  int    `yaml:"max_concurrent_scans"`
	ScanTimeoutSeconds  int    `yaml:"scan_timeout_seconds"`
}

// defaults returns a Config with safe defaults.
func defaults() Config {
	return Config{
		Server: ServerConfig{
			BindAddress: "127.0.0.1",
			Port:        8080,
		},
		AI: AIConfig{
			OllamaURL:           "http://localhost:11434",
			OllamaModel:         "qwen2.5:3b",
			MLAnomalyThreshold:  0.85,
			LLMTemperature:      0.7,
		},
		Scanner: ScannerConfig{
			NmapPath:           "", // empty means use $PATH
			MaxConcurrentScans: 5,
			ScanTimeoutSeconds: 600,
		},
	}
}

// Load reads the YAML config file at path. If the file does not exist,
// returns safe defaults. If the file exists but is malformed, returns an error.
func Load(path string) (Config, error) {
	cfg := defaults()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // missing file = defaults, no error
		}
		return cfg, fmt.Errorf("read config: %w", err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}

	return cfg, nil
}
```

- [ ] **Step 5: Run test to verify it passes**

```bash
cd C:\Users\Yeikxz\Desktop\ViewRed
go test ./internal/config/... -v
```

Expected: PASS for all 3 tests (`TestLoadDefaultsWhenFileMissing`, `TestLoadFromYAMLFile`, `TestLoadInvalidYAMLReturnsError`).

- [ ] **Step 6: Wire config into main.go**

Replace `C:\Users\Yeikxz\Desktop\ViewRed\cmd\viewred\main.go` with:

```go
// Package main is the entry point for the ViewRed binary.
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
		// Cross-platform default config location.
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
```

Note: This replacement **keeps** the `Version` constant and `--version` flag. `TestVersion` in `cmd/viewred/main_test.go` will still pass.

- [ ] **Step 7: Run all tests**

```bash
cd C:\Users\Yeikxz\Desktop\ViewRed
go test ./...
```

Expected: all tests pass (8/8 across 4 packages: cmd/viewred, internal/api, internal/api/handlers, internal/config).

- [ ] **Step 8: Commit**

```bash
cd C:\Users\Yeikxz\Desktop\ViewRed
git add go.mod go.sum internal/ cmd/
git commit -m "feat: add YAML config loader with safe defaults"
```

---

## Verification

Before reporting back, verify:

1. `go test ./...` passes all tests across all packages.
2. `go run ./cmd/viewred --version` still prints `viewred 0.1.0-dev`.
3. `go run ./cmd/viewred` starts and serves on `http://127.0.0.1:8080` (the default config).
4. The 4 new files exist: `internal/config/config.go`, `internal/config/config_test.go`, `internal/config/helpers_test.go`, plus the modified `cmd/viewred/main.go`.
5. Single commit on `main` with the expected message.

---

## What to report

Write your full report to: `C:\Users\Yeikxz\Desktop\ViewRed\docs\superpowers\reports\2026-06-17-bootstrap-monorepo-task-04.md`

Include:
- What you implemented
- TDD evidence: RED (compile errors) and GREEN (3/3 tests pass)
- Files created/modified
- Commit SHA + subject
- Any concerns

Then reply with ONLY (under 15 lines):

- **Status:** DONE | DONE_WITH_CONCERNS | BLOCKED | NEEDS_CONTEXT
- Commits created (short SHA + subject)
- One-line test summary (e.g. "8/8 tests pass, --version still works")
- Your concerns, if any
- The report file path

## When you're stuck

If `go get` fails, tests don't behave as expected, or anything unusual, **stop and ask** with NEEDS_CONTEXT. Don't guess.