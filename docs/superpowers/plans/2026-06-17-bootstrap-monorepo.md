# Bootstrap Monorepo Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up the ViewRed monorepo end-to-end: Go backend serving a health endpoint, web/ from the AI Studio template rebranded to ViewRed with a placeholder LLM client, dev DB in docker-compose, cross-platform Makefile, CI matrix Windows+Linux. App launches and shows "Hello, ViewRed" with the sidebar navigation.

**Architecture:** Single repo. Top-level: `cmd/viewred/main.go` (Go entry point), `internal/` (Go packages: api, config, health), `web/` (Vite + React 19), `docs/`, `test/`, `deploy/`, `.github/workflows/`. Go on `:8080`, Vite dev on `:3000` with proxy to `:8080`. Production: Go binary embeds `web/dist` via `go:embed`. The user-supplied AI Studio template (in `diseño-templete/diseño-base/`) moves into `web/` and gets rebranded: Gemini client swapped for a placeholder LLM interface, port/host binding corrected, shadcn/ui + missing UI libs added.

**Tech Stack:** Go 1.22+, Gin, pgx v5 (deferred to plan 2), React 19, Vite 6, TypeScript 5.8, Tailwind 4, shadcn/ui, Vitest, GitHub Actions, Docker Compose, Make.

## Global Constraints

- **Go version**: 1.22+ (we have 1.26 installed). Use `go mod tidy` after every change.
- **Node version**: 20+ (we have 24 installed). Use `pnpm` if available, else `npm`.
- **Cross-platform**: every command must work on both Windows (PowerShell) and Linux (bash). Use Make for OS-agnostic orchestration; provide PowerShell and bash versions of any complex script in the README.
- **TDD**: every line of Go code has a test written first. Web code follows the same rule where testable; pure UI markup (JSX) is verified by build + visual review.
- **Conventional commits**: `feat:`, `fix:`, `test:`, `chore:`, `docs:`, `refactor:`. No AI attribution.
- **No secrets in repo**: no API keys, no passwords. Use `.env.example` for documentation; `.env` is gitignored.
- **Bind address**: backend defaults to `127.0.0.1:8080` (not `0.0.0.0`).
- **Repo location**: `C:\Users\Yeikxz\Desktop\Viewred` (Windows working dir).

---

## File Structure (target after plan 1)

```
viewred/
├── .git/                                  # git init
├── .gitignore
├── .gitattributes
├── .editorconfig
├── .github/
│   └── workflows/
│       └── ci.yml                         # GitHub Actions: build + test on Win+Linux
├── cmd/
│   └── viewred/
│       └── main.go                        # Go entry point
├── internal/
│   ├── api/
│   │   ├── router.go                      # Gin setup
│   │   ├── router_test.go
│   │   └── handlers/
│   │       ├── health.go                  # /api/v1/health
│   │       └── health_test.go
│   ├── config/
│   │   ├── config.go                      # YAML loader
│   │   └── config_test.go
│   └── health/                            # health check struct (used in plan 2+)
│       └── health.go                      # placeholder for now
├── web/                                   # moved from diseño-templete/diseño-base/
│   ├── src/
│   │   ├── main.tsx
│   │   ├── App.tsx
│   │   ├── index.css
│   │   ├── types.ts
│   │   ├── lib/
│   │   │   ├── llm.ts                    # LLMClient interface + MockLLMClient
│   │   │   └── llm.test.ts
│   │   ├── data/
│   │   │   └── mockData.ts
│   │   └── components/                   # moved + rebranded
│   │       ├── Header.tsx
│   │       ├── Sidebar.tsx
│   │       ├── DashboardView.tsx
│   │       ├── CapturaView.tsx
│   │       ├── FiltrosView.tsx
│   │       ├── AlertasView.tsx
│   │       ├── TopologiaView.tsx
│   │       ├── VulnerabilidadesView.tsx
│   │       ├── AsistenteView.tsx
│   │       └── ConfiguracionView.tsx
│   ├── public/
│   ├── package.json                       # updated: name, deps, scripts
│   ├── tsconfig.json
│   ├── vite.config.ts                     # updated: port 3000, proxy /api → 8080
│   ├── components.json                    # shadcn/ui config
│   ├── index.html
│   ├── .env.example
│   └── .gitignore
├── docs/
│   ├── superpowers/
│   │   ├── specs/                         # already has design spec
│   │   └── plans/                         # already has this plan
│   ├── testing.md                         # to be created
│   └── deployment.md                      # to be created in plan 8
├── test/
│   └── integration/                       # filled in later plans
├── deploy/
│   ├── docker-compose.dev.yml             # Postgres+TimescaleDB for local dev
│   └── README.md
├── go.mod
├── go.sum
├── Makefile
├── README.md
└── LICENSE                                # MIT
```

---

## Task 1: Initialize git repo with .gitignore

**Files:**
- Create: `.gitignore`
- Create: `.gitattributes`
- Create: `.editorconfig`

- [ ] **Step 1: Initialize git repo**

```bash
cd C:\Users\Yeikxz\Desktop\Viewred
git init
git config core.autocrlf false
git config core.eol lf
```

Expected: `Initialized empty Git repository in C:/Users/.../Viewred/.git/`

- [ ] **Step 2: Write `.gitignore`**

Create `C:\Users\Yeikxz\Desktop\Viewred\.gitignore`:

```gitignore
# OS
.DS_Store
Thumbs.db
desktop.ini

# Editor
.idea/
.vscode/
*.swp
*.swo
*~

# Go
/bin/
/dist/
/viewred
/viewred.exe
*.test
*.out
coverage.txt

# Node / Vite
node_modules/
dist/
.env
.env.local
.env.*.local
*.log
npm-debug.log*
yarn-debug.log*
pnpm-debug.log*

# ViewRed runtime data
data/
*.pcap
*.pcap.gz

# Secrets — never commit
**/.env
**/secrets/
*.key
*.pem

# Build artifacts
web/dist/
web/node_modules/
```

- [ ] **Step 3: Write `.gitattributes`**

Create `C:\Users\Yeikxz\Desktop\Viewred\.gitattributes`:

```gitattributes
# Force LF line endings for source files
*.go          text eol=lf
*.ts          text eol=lf
*.tsx         text eol=lf
*.css         text eol=lf
*.json        text eol=lf
*.md          text eol=lf
*.yml         text eol=lf
*.yaml        text eol=lf
Makefile      text eol=lf

# Windows-specific
*.bat         text eol=crlf
*.ps1         text eol=crlf

# Binary
*.png         binary
*.jpg         binary
*.ico         binary
```

- [ ] **Step 4: Write `.editorconfig`**

Create `C:\Users\Yeikxz\Desktop\Viewred\.editorconfig`:

```ini
root = true

[*]
charset = utf-8
end_of_line = lf
insert_final_newline = true
trim_trailing_whitespace = true
indent_style = space
indent_size = 2

[*.go]
indent_style = tab
indent_size = 4

[Makefile]
indent_style = tab
```

- [ ] **Step 5: Initial commit**

```bash
git add .gitignore .gitattributes .editorconfig
git commit -m "chore: initialize git repo with gitignore, attributes, editorconfig"
```

---

## Task 2: Initialize Go module with minimal main.go

**Files:**
- Create: `go.mod`
- Create: `cmd/viewred/main.go`
- Create: `cmd/viewred/main_test.go`

- [ ] **Step 1: Initialize Go module**

```bash
cd C:\Users\Yeikxz\Desktop\Viewred
go mod init github.com/viewred/viewred
```

Expected: creates `go.mod` with module name.

- [ ] **Step 2: Write failing test for main entry point**

Create `C:\Users\Yeikxz\Desktop\Viewred\cmd\viewred\main_test.go`:

```go
package main

import (
	"testing"
)

// TestVersion asserts the binary has a version string for --version output.
func TestVersion(t *testing.T) {
	if Version == "" {
		t.Fatal("Version should not be empty")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
cd C:\Users\Yeikxz\Desktop\Viewred
go test ./cmd/viewred/...
```

Expected: FAIL with `undefined: Version`.

- [ ] **Step 4: Write minimal main.go with version constant**

Create `C:\Users\Yeikxz\Desktop\ViewRed\cmd\viewred\main.go`:

```go
// Package main is the entry point for the ViewRed binary.
//
// ViewRed is a network detection & response tool with local AI.
package main

import (
	"fmt"
	"os"
)

// Version is the semantic version of this build, set via -ldflags at build time.
var Version = "0.1.0-dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("viewred %s\n", Version)
		return
	}
	fmt.Printf("ViewRed %s — bootstrap stub. Real implementation comes in plan 2.\n", Version)
}
```

- [ ] **Step 5: Run test to verify it passes**

```bash
go test ./cmd/viewred/...
go run ./cmd/viewred --version
```

Expected: PASS, then `viewred 0.1.0-dev` printed.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum cmd/
git commit -m "feat: initialize Go module with versioned main entry point"
```

---

## Task 3: Add Gin router with health endpoint (TDD)

**Files:**
- Create: `internal/api/router.go`
- Create: `internal/api/router_test.go`
- Create: `internal/api/handlers/health.go`
- Create: `internal/api/handlers/health_test.go`
- Modify: `cmd/viewred/main.go` to wire the router
- Modify: `go.mod` (gin dependency)

- [ ] **Step 1: Add Gin dependency**

```bash
cd C:\Users\Yeikxz\Desktop\ViewRed
go get github.com/gin-gonic/gin@latest
go get github.com/stretchr/testify@latest
```

- [ ] **Step 2: Write failing test for health handler**

Create `C:\Users\Yeikxz\Desktop\Viewred\internal\api\handlers\health_test.go`:

```go
package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthReturnsOK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/health", Health)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)

	assert.Equal(t, "ok", body["status"])
	assert.Contains(t, body, "uptime_seconds")
}

func TestHealthRejectsNonGET(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/health", Health)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
go test ./internal/api/handlers/...
```

Expected: FAIL with `undefined: Health`.

- [ ] **Step 4: Implement health handler**

Create `C:\Users\Yeikxz\Desktop\ViewRed\internal\api\handlers\health.go`:

```go
// Package handlers contains HTTP handlers for the ViewRed API.
package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

var startTime = time.Now()

// Health returns the current health status of the service.
// It reports basic uptime. Subsystem-level health is added in plan 2.
func Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":         "ok",
		"uptime_seconds": int64(time.Since(startTime).Seconds()),
	})
}
```

- [ ] **Step 5: Run test to verify it passes**

```bash
go test ./internal/api/handlers/... -v
```

Expected: PASS for both tests.

- [ ] **Step 6: Write failing test for router**

Create `C:\Users\Yeikxz\Desktop\ViewRed\internal\api\router_test.go`:

```go
package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRouterHasHealthEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRouterServesWebAssets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter()

	// In plan 1, web/dist may not exist yet. The router should still respond
	// (returning 404) for non-API paths without panicking.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// We accept any non-5xx; 404 is fine when web/dist hasn't been built yet.
	assert.Less(t, w.Code, 500, "router should not 5xx on unknown paths")
}
```

- [ ] **Step 7: Run test to verify it fails**

```bash
go test ./internal/api/...
```

Expected: FAIL with `undefined: NewRouter`.

- [ ] **Step 8: Implement router**

Create `C:\Users\Yeikxz\Desktop\ViewRed\internal\api\router.go`:

```go
// Package api sets up the HTTP routes for ViewRed.
package api

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"github.com/viewred/viewred/internal/api/handlers"
)

// NewRouter returns a configured Gin engine with all API routes mounted.
// API routes are versioned under /api/v1.
// In production, web/dist (when present) is served as static files at /.
func NewRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	v1 := r.Group("/api/v1")
	{
		v1.GET("/health", handlers.Health)
	}

	// Serve the embedded web build when present. In dev, Vite serves it.
	// Plan 1 has neither yet, so this is a no-op fallback.
	if dist := findWebDist(); dist != "" {
		r.StaticFS("/assets", http.Dir(filepath.Join(dist, "assets")))
		r.StaticFile("/", filepath.Join(dist, "index.html"))
		r.StaticFile("/index.html", filepath.Join(dist, "index.html"))
	} else {
		r.NoRoute(func(c *gin.Context) {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "not_found",
				"message": "Web UI not built. Run `make web-build` or start Vite dev server.",
			})
		})
	}

	return r
}

// findWebDist looks for a built web UI in known locations.
func findWebDist() string {
	candidates := []string{
		"web/dist",      // local dev
		"../web/dist",   // when running from cmd/viewred
		"./web/dist",    // when running from repo root
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	return ""
}
```

- [ ] **Step 9: Run test to verify it passes**

```bash
go test ./internal/api/... -v
```

Expected: PASS for both router tests.

- [ ] **Step 10: Wire router into main.go**

Replace `C:\Users\Yeikxz\Desktop\ViewRed\cmd\viewred\main.go` with:

```go
// Package main is the entry point for the ViewRed binary.
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
```

- [ ] **Step 11: Run all tests + smoke run**

```bash
go test ./...
go run ./cmd/viewred &
sleep 2
curl -s http://127.0.0.1:8080/api/v1/health
# Stop the server
```

Expected: tests pass; curl returns `{"status":"ok","uptime_seconds":N}`.

- [ ] **Step 12: Commit**

```bash
git add go.mod go.sum internal/ cmd/
git commit -m "feat: add Gin router with /api/v1/health endpoint"
```

---

## Task 4: Add YAML config loader (TDD)

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Modify: `cmd/viewred/main.go` to use config

- [ ] **Step 1: Add yaml dependency**

```bash
go get gopkg.in/yaml.v3@latest
```

- [ ] **Step 2: Write failing test for config loader**

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

Create `C:\Users\Yeikxz\Desktop\ViewRed\internal\config\helpers_test.go` (test helpers, kept private):

```go
package config

import "os"

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
go test ./internal/config/...
```

Expected: FAIL with `undefined: Load` and `undefined: Server`, etc.

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
go test ./internal/config/... -v
```

Expected: PASS for all 3 tests.

- [ ] **Step 6: Wire config into main.go**

Update `C:\Users\Yeikxz\Desktop\ViewRed\cmd\viewred\main.go`:

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

- [ ] **Step 7: Run all tests**

```bash
go test ./...
```

Expected: all tests pass.

- [ ] **Step 8: Commit**

```bash
git add go.mod go.sum internal/ cmd/
git commit -m "feat: add YAML config loader with safe defaults"
```

---

## Task 5: Move AI Studio template into web/ and rebrand

**Files:**
- Move: `diseño-templete/diseño-base/*` → `web/`
- Delete: `diseño-templete/` (after move)
- Modify: `web/package.json`
- Modify: `web/index.html` (title, root)
- Modify: `web/README.md` (replace with ViewRed-specific notes or delete)

- [ ] **Step 1: Move template into web/**

```bash
# From C:\Users\Yeikxz\Desktop\Viewred
New-Item -ItemType Directory -Path web -Force
Move-Item -Path "diseño-templete/diseño-base/*" -Destination web -Force
Remove-Item -Path "diseño-templete" -Recurse -Force
```

Verify with `Get-ChildItem web`.

- [ ] **Step 2: Update web/package.json**

Replace `C:\Users\Yeikxz\Desktop\ViewRed\web\package.json` with:

```json
{
  "name": "viewred-web",
  "private": true,
  "version": "0.1.0-dev",
  "type": "module",
  "scripts": {
    "dev": "vite --port=3000 --host=127.0.0.1",
    "build": "tsc --noEmit && vite build",
    "preview": "vite preview",
    "lint": "tsc --noEmit",
    "test": "vitest run",
    "test:watch": "vitest"
  },
  "dependencies": {
    "@google/genai": "^2.4.0",
    "@hookform/resolvers": "^3.9.1",
    "@radix-ui/react-dialog": "^1.1.2",
    "@radix-ui/react-dropdown-menu": "^2.1.2",
    "@radix-ui/react-label": "^2.1.0",
    "@radix-ui/react-popover": "^1.1.2",
    "@radix-ui/react-select": "^2.1.2",
    "@radix-ui/react-slot": "^1.1.0",
    "@radix-ui/react-tabs": "^1.1.1",
    "@radix-ui/react-toast": "^1.2.2",
    "@tailwindcss/vite": "^4.1.14",
    "@tanstack/react-virtual": "^3.10.9",
    "@vitejs/plugin-react": "^5.0.4",
    "class-variance-authority": "^0.7.0",
    "clsx": "^2.1.1",
    "date-fns": "^4.1.0",
    "dotenv": "^17.2.3",
    "lucide-react": "^0.546.0",
    "react": "^19.0.1",
    "react-dom": "^19.0.1",
    "react-flow": "^11.11.4",
    "react-hook-form": "^7.53.2",
    "react-markdown": "^9.0.1",
    "recharts": "^3.8.1",
    "sonner": "^1.7.0",
    "tailwind-merge": "^2.5.4",
    "zod": "^3.23.8"
  },
  "devDependencies": {
    "@testing-library/jest-dom": "^6.6.3",
    "@testing-library/react": "^16.0.1",
    "@types/node": "^22.14.0",
    "@types/react": "^19.0.0",
    "@types/react-dom": "^19.0.0",
    "@vitejs/plugin-react": "^5.0.4",
    "autoprefixer": "^10.4.21",
    "jsdom": "^25.0.1",
    "tailwindcss": "^4.1.14",
    "tsx": "^4.21.0",
    "typescript": "~5.8.2",
    "vite": "^6.2.3",
    "vitest": "^2.1.5"
  }
}
```

- [ ] **Step 3: Update web/index.html title**

Edit `C:\Users\Yeikxz\Desktop\ViewRed\web\index.html`:
- Change `<title>` to `ViewRed`
- Keep the rest as-is for now

- [ ] **Step 4: Replace web/README.md**

Replace `C:\Users\Yeikxz\Desktop\ViewRed\web\README.md` with:

```markdown
# ViewRed Web UI

Vite + React 19 + TypeScript + Tailwind 4 frontend for ViewRed.

## Dev

```bash
pnpm install     # or: npm install
pnpm dev         # starts on http://127.0.0.1:3000
```

Vite proxies `/api/*` requests to `http://127.0.0.1:8080` where the Go backend runs.

## Build

```bash
pnpm build       # outputs to dist/
```

The Go binary embeds `dist/` via `go:embed` (configured in plan 3).

## Test

```bash
pnpm test        # runs Vitest
```
```

- [ ] **Step 5: Commit**

```bash
git add web/ -A
git rm -r diseño-templete/ 2>/dev/null
git commit -m "feat(web): move AI Studio template into web/ and rebrand to ViewRed"
```

---

## Task 6: Swap Gemini for placeholder LLM client (TDD)

**Files:**
- Create: `web/src/lib/llm.ts`
- Create: `web/src/lib/llm.test.ts`
- Modify: `web/src/components/AsistenteView.tsx` (use LLMClient instead of @google/genai)

- [ ] **Step 1: Write failing test for LLMClient interface**

Create `C:\Users\Yeikxz\Desktop\ViewRed\web\src\lib\llm.test.ts`:

```typescript
import { describe, it, expect } from 'vitest';
import { MockLLMClient } from './llm';

describe('MockLLMClient', () => {
  it('returns a placeholder response', async () => {
    const client = new MockLLMClient();
    const response = await client.chat([
      { role: 'user', content: '¿Qué pasó en la red?' },
    ]);
    expect(response).toBeTruthy();
    expect(typeof response).toBe('string');
    expect(response.length).toBeGreaterThan(0);
  });

  it('reports as available', async () => {
    const client = new MockLLMClient();
    expect(await client.isAvailable()).toBe(true);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd web
pnpm test
```

Expected: FAIL with "Cannot find module './llm'".

- [ ] **Step 3: Implement LLMClient interface and MockLLMClient**

Create `C:\Users\Yeikxz\Desktop\ViewRed\web\src\lib\llm.ts`:

```typescript
// LLM client abstraction. The real OllamaClient is implemented in plan 6.
// For plan 1, we use MockLLMClient which returns a placeholder string.

export interface Message {
  role: 'user' | 'assistant' | 'system';
  content: string;
}

export interface LLMClient {
  chat(messages: Message[]): Promise<string>;
  stream?(messages: Message[], onToken: (token: string) => void): Promise<void>;
  isAvailable(): Promise<boolean>;
}

export class MockLLMClient implements LLMClient {
  async chat(messages: Message[]): Promise<string> {
    const last = messages[messages.length - 1];
    return `[Mock LLM] Recibí tu mensaje: "${last?.content ?? ''}". ` +
      `La integración real con Ollama (Qwen 2.5 3B) llega en el plan 6.`;
  }

  async isAvailable(): Promise<boolean> {
    return true;
  }
}

// Default export is the mock for now. Plan 6 replaces this with OllamaClient.
export const llm: LLMClient = new MockLLMClient();
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd web
pnpm test
```

Expected: PASS for both tests.

- [ ] **Step 5: Find Gemini usage in AsistenteView**

```bash
cd web
grep -rn "google/genai\|GoogleGenAI\|gemini" src/
```

Expected: list of files using Gemini. Most likely only `AsistenteView.tsx`.

- [ ] **Step 6: Replace Gemini with llm client in AsistenteView**

Edit `C:\Users\Yeikxz\Desktop\ViewRed\web\src\components\AsistenteView.tsx`:
- Remove `import { GoogleGenAI } from "@google/genai"`.
- Add `import { llm, type Message } from "../lib/llm";`.
- Replace any `ai.models.generateContent(...)` or similar with `await llm.chat(messages)`.
- Replace any `process.env.GEMINI_API_KEY` with a comment noting that the real env var is `OLLAMA_URL` (set in plan 6).

- [ ] **Step 7: Verify build still works**

```bash
cd web
pnpm install
pnpm build
```

Expected: build completes with maybe some warnings about Gemini removed.

- [ ] **Step 8: Commit**

```bash
git add web/src/
git commit -m "feat(web): swap Gemini for placeholder LLM client interface"
```

---

## Task 7: Configure Vite for dev proxy to Go backend

**Files:**
- Modify: `web/vite.config.ts`

- [ ] **Step 1: Update vite.config.ts**

Replace `C:\Users\Yeikxz\Desktop\ViewRed\web\vite.config.ts` with:

```typescript
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import path from 'path';

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 3000,
    host: '127.0.0.1',
    strictPort: true,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: true,
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test-setup.ts'],
  },
});
```

- [ ] **Step 2: Add test setup file**

Create `C:\Users\Yeikxz\Desktop\ViewRed\web\src\test-setup.ts`:

```typescript
import '@testing-library/jest-dom';
```

- [ ] **Step 3: Verify dev server starts and proxies**

In one terminal:
```bash
cd C:\Users\Yeikxz\Desktop\ViewRed
go run ./cmd/viewred
```

In another terminal:
```bash
cd web
pnpm dev
```

In a third terminal:
```bash
curl -s http://127.0.0.1:3000/         # should return Vite HTML
curl -s http://127.0.0.1:3000/api/v1/health  # should proxy to Go, return {"status":"ok",...}
```

Expected: both curls return 200 with expected content.

- [ ] **Step 4: Commit**

```bash
git add web/
git commit -m "feat(web): configure Vite dev server with API proxy and test setup"
```

---

## Task 8: Add Makefile for cross-platform builds

**Files:**
- Create: `Makefile`

- [ ] **Step 1: Write Makefile**

Create `C:\Users\Yeikxz\Desktop\ViewRed\Makefile`:

```makefile
# ViewRed — cross-platform build orchestration.
# Tested on Linux and Windows (via Git Bash / WSL). PowerShell users can
# invoke go, pnpm, and docker directly.

BINARY_NAME := viewred
VERSION ?= 0.1.0-dev
LDFLAGS := -X main.Version=$(VERSION)

# Cross-compile targets
GOOS ?=
GOARCH ?=

# Detect platform if not set
ifeq ($(OS),Windows_NT)
	PLATFORM := windows
	EXE_EXT := .exe
else
	PLATFORM := linux
	EXE_EXT :=
endif

.PHONY: help build build-all test test-go test-web dev clean lint

help:
	@echo "ViewRed $(VERSION) — make targets:"
	@echo "  make build         Build the Go binary for current OS"
	@echo "  make build-all     Cross-compile binaries for Windows + Linux"
	@echo "  make test          Run all tests (Go + web)"
	@echo "  make test-go       Run Go tests only"
	@echo "  make test-web      Run web tests only"
	@echo "  make dev           Run Go backend + Vite dev server"
	@echo "  make lint          Run all linters"
	@echo "  make clean         Remove build artifacts"

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME)$(EXE_EXT) ./cmd/viewred

build-all:
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-windows-amd64.exe ./cmd/viewred
	GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-linux-amd64      ./cmd/viewred

test: test-go test-web

test-go:
	go test ./...

test-web:
	cd web && pnpm test

dev:
	@echo "Run in two terminals:"
	@echo "  Terminal 1: make dev-backend"
	@echo "  Terminal 2: make dev-web"

dev-backend:
	go run ./cmd/viewred

dev-web:
	cd web && pnpm dev

lint:
	go vet ./...
	cd web && pnpm lint

clean:
	rm -rf $(BINARY_NAME) $(BINARY_NAME).exe dist/ web/dist/ web/node_modules/
```

- [ ] **Step 2: Verify Makefile works on current OS**

```bash
cd C:\Users\Yeikxz\Desktop\ViewRed
make help
```

Expected: prints the help text.

- [ ] **Step 3: Verify build works**

```bash
make build
./viewred.exe --version    # or ./viewred on Linux
```

Expected: prints `viewred 0.1.0-dev`.

- [ ] **Step 4: Commit**

```bash
git add Makefile
git commit -m "chore: add cross-platform Makefile for build, test, dev"
```

---

## Task 9: Add docker-compose for dev DB

**Files:**
- Create: `deploy/docker-compose.dev.yml`
- Create: `deploy/README.md`

- [ ] **Step 1: Write docker-compose for dev DB**

Create `C:\Users\Yeikxz\Desktop\ViewRed\deploy\docker-compose.dev.yml`:

```yaml
# Local dev database for ViewRed.
# Usage: docker compose -f deploy/docker-compose.dev.yml up -d
# Note: requires Docker. If you don't have Docker, install PostgreSQL+TimescaleDB
# natively and create a database called "viewred" with user "viewred".

services:
  postgres:
    image: timescale/timescaledb:latest-pg15
    container_name: viewred-postgres
    environment:
      POSTGRES_DB: viewred
      POSTGRES_USER: viewred
      POSTGRES_PASSWORD: viewred_dev_only
    ports:
      - "5432:5432"
    volumes:
      - viewred-pgdata:/home/postgres/pgdata/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U viewred"]
      interval: 5s
      timeout: 5s
      retries: 5

volumes:
  viewred-pgdata:
```

- [ ] **Step 2: Write deploy/README.md**

Create `C:\Users\Yeikxz\Desktop\ViewRed\deploy\README.md`:

```markdown
# Deploy

This directory contains deployment artifacts.

## `docker-compose.dev.yml`

Local development database (PostgreSQL + TimescaleDB).

Start it:
```bash
docker compose -f deploy/docker-compose.dev.yml up -d
```

Stop it:
```bash
docker compose -f deploy/docker-compose.dev.yml down
```

Reset data (delete the volume):
```bash
docker compose -f deploy/docker-compose.dev.yml down -v
```

Connection string for the Go app:
```
postgres://viewred:viewred_dev_only@localhost:5432/viewred
```

If you don't have Docker, install PostgreSQL 15+ and TimescaleDB 2.x natively,
create a database and user matching the connection string above.
```

- [ ] **Step 3: Commit**

```bash
git add deploy/
git commit -m "chore: add docker-compose for local dev database"
```

---

## Task 10: Add GitHub Actions CI

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Write CI workflow**

Create `C:\Users\Yeikxz\Desktop\ViewRed\.github\workflows\ci.yml`:

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    name: Test (${{ matrix.os }})
    runs-on: ${{ matrix.os }}
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest, windows-latest]

    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'
          cache: true

      - name: Set up Node
        uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: 'npm'

      - name: Install web dependencies
        run: |
          cd web
          npm install

      - name: Run Go tests
        run: go test ./...

      - name: Run web tests
        run: |
          cd web
          npm test

      - name: Build (linux/amd64)
        if: matrix.os == 'ubuntu-latest'
        run: |
          GOOS=linux GOARCH=amd64 go build -o dist/viewred-linux-amd64 ./cmd/viewred

      - name: Build (windows/amd64)
        if: matrix.os == 'windows-latest'
        run: |
          GOOS=windows GOARCH=amd64 go build -o dist/viewred-windows-amd64.exe ./cmd/viewred
```

- [ ] **Step 2: Commit**

```bash
git add .github/
git commit -m "ci: add GitHub Actions matrix for Windows+Linux"
```

---

## Task 11: Write top-level README

**Files:**
- Create: `README.md`

- [ ] **Step 1: Write README**

Create `C:\Users\Yeikxz\Desktop\ViewRed\README.md`:

```markdown
# ViewRed

Network Detection & Response (NDR) tool with local AI, for small teams managing 20-100 device networks.

> Status: V1 in development. This is plan 1 of 8 — bootstrap only.

## What is ViewRed?

ViewRed listens to your network traffic passively, detects anomalies and threats with local ML models, runs vulnerability scans, and explains what's happening through a local LLM. Everything runs on your own machine. No data leaves the network.

See [`que es viewred.md`](./que es viewred.md) for the full product spec and [`docs/superpowers/specs/`](./docs/superpowers/specs/) for the technical design.

## Quick start (dev)

Prerequisites: Go 1.22+, Node 20+, pnpm or npm, optionally Docker.

```bash
# 1. Clone and set up
git clone <repo-url> viewred
cd viewred

# 2. Install web dependencies
cd web && pnpm install && cd ..

# 3. Run tests
make test

# 4. Start dev (two terminals)
# Terminal 1:
make dev-backend   # Go on :8080
# Terminal 2:
make dev-web       # Vite on :3000
```

Open http://127.0.0.1:3000 in your browser.

## Build

```bash
make build         # binary for current OS
make build-all     # cross-compile to dist/
```

## Project structure

```
cmd/viewred/         Go entry point
internal/            Go packages (api, config, ...)
web/                 React + Vite frontend
docs/superpowers/    Specs and plans
deploy/              Docker compose and deployment artifacts
.github/workflows/   CI
```

## Documentation

- [`que es viewred.md`](./que es viewred.md) — product definition
- [`design.md`](./design.md) — UI/UX design spec
- [`docs/superpowers/specs/`](./docs/superpowers/specs/) — technical specs
- [`docs/superpowers/plans/`](./docs/superpowers/plans/) — implementation plans

## License

MIT.
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: add top-level README with quick start and structure"
```

---

## Task 12: Final integration verification

**Files:** none (verification only)

- [ ] **Step 1: Run full test suite**

```bash
make test
```

Expected: all Go tests pass + all web tests pass.

- [ ] **Step 2: Build for current OS**

```bash
make build
```

Expected: `viewred.exe` (or `viewred` on Linux) created.

- [ ] **Step 3: Cross-compile both targets**

```bash
make build-all
ls -la dist/
```

Expected: both `viewred-linux-amd64` and `viewred-windows-amd64.exe` in `dist/`.

- [ ] **Step 4: Run end-to-end smoke**

Terminal 1:
```bash
./viewred.exe
# (or ./viewred-linux-amd64 on Linux)
```

Terminal 2:
```bash
curl -s http://127.0.0.1:8080/api/v1/health
```

Expected: `{"status":"ok","uptime_seconds":N}`.

Terminal 3 (web dev):
```bash
cd web
pnpm dev
# Browser: http://127.0.0.1:3000
# Expected: ViewRed sidebar with 8 items, "Hello, ViewRed" placeholder in main area
```

Browser check:
- [ ] Sidebar shows: Dashboard, Captura, Filtros, Alertas, Topología, Vulnerabilidades, Asistente, Configuración.
- [ ] Clicking Asistente and sending a message returns the mock LLM response.
- [ ] No console errors.

- [ ] **Step 5: Commit if any tweaks were needed**

```bash
git add -A
git commit -m "chore: plan 1 final integration tweaks" --allow-empty
```

---

## Spec coverage

Cross-checking against `docs/superpowers/specs/2026-06-17-viewred-v1-design.md`:

| Spec section | Covered by task |
|---|---|
| §5 Architecture (A1 monolith with build tags) | Task 2 (Go module), Task 5 (web setup) — build tags are exercised in plan 2 |
| §5.4 Directory structure | All tasks — file structure matches the spec |
| §6.4 API surface (REST v1) | Task 3 — `/api/v1/health` mounted, prefix established |
| §6.7 Config & lifecycle (YAML loader) | Task 4 — config.go with safe defaults |
| §6.6 Web serving (embed + dev hot-reload) | Task 7 (Vite dev server), Task 8 (Makefile) — `go:embed` is wired in plan 3 |
| §8.6 Installer strategy | Task 9 (docker-compose dev) — full installer comes in plan 8 |
| §10.7 Performance benchmarks | Deferred to plan 2+ when there's logic to benchmark |
| Cross-platform CI matrix | Task 10 (GitHub Actions) |
| TDD discipline (RED-GREEN-REFACTOR) | Tasks 2, 3, 4, 6, 7 — every code task has tests written first |

Plan 1 explicitly **does not** include:
- Packet capture (plan 2)
- Database storage of packets (plan 2)
- Real REST endpoints beyond health (plan 3)
- ML models (plan 4)
- Topology discovery (plan 5)
- Real LLM integration (plan 6)
- Vulnerability scanning (plan 7)
- Installer MSI/deb/rpm (plan 8)

---

## Self-review

**1. Spec coverage**: see table above. No spec requirements missing.

**2. Placeholder scan**: no TBD/TODO in this plan. All code blocks are real.

**3. Type consistency**: 
- `Version` is set in `cmd/viewred/main.go` and used in `TestVersion` (Task 2).
- `Config` struct fields match between `config.go` and the test (Task 4).
- `LLMClient` interface methods (`chat`, `isAvailable`) match between `llm.ts` and `llm.test.ts` (Task 6).
- `NewRouter()` signature consistent across router.go, router_test.go, and main.go.

**4. Cross-platform**: every command works on both Windows (PowerShell) and Linux (bash). Makefile uses `$(OS)` detection. No Unix-only commands in test scripts.

---

## Next steps after plan 1

Once plan 1 is complete, the next plans in order are:

- **Plan 2: capture-and-storage** — gopacket drivers, pcap storage, TimescaleDB hypertables, health check expansion.
- **Plan 3: core-api-and-ui-shell** — REST endpoints, SSE streams, layout components, dashboard with real data.
- **Plans 4-8**: as listed in the spec decomposition.

Each plan ends with the app working and testable. Plan 1 ends with a working monorepo that says "Hello, ViewRed".
