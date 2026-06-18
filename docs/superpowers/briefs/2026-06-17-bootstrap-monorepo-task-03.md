# Task 3 Brief: Add Gin router with health endpoint (TDD)

**Plan**: `docs/superpowers/plans/2026-06-17-bootstrap-monorepo.md`
**Date**: 2026-06-17
**Project root (work from here)**: `C:\Users\Yeikxz\Desktop\ViewRed`

---

## Context

This is **Task 3 of 12** in the ViewRed bootstrap. Tasks 1 and 2 are done:
- Task 1 (`003fd25`): git init + .gitignore/.gitattributes/.editorconfig
- Task 2 (`75d6b69`): Go module `github.com/viewred/viewred` + `cmd/viewred/main.go` with `--version` flag + TestVersion

Your job: add the **Gin HTTP framework** and a **`/api/v1/health` endpoint** with full TDD discipline. The router also handles a fallback for non-API paths (returns 404 JSON when web/dist is not built, which is the case in plan 1).

### Interfaces and decisions from earlier tasks

- **Module path**: `github.com/viewred/viewred` (do not change).
- **Existing `cmd/viewred/main.go`**: Has `Version` constant and `--version` flag handling. You will REPLACE this file in Step 10 to wire the router in. Keep the `Version` constant and `--version` flag.
- **Bind address**: `127.0.0.1:8080` (per spec, default in main.go).
- **API prefix**: `/api/v1` (everything versioned under this).
- **TDD strictly enforced**: this task has 4 distinct pieces of code, each with a test-first cycle.

### Files already in the directory (DO NOT TOUCH except as specified)

- `go.mod`, `go.sum` (will be modified by `go get` in Step 1)
- `cmd/viewred/main.go` (will be REPLACED in Step 10)
- `cmd/viewred/main_test.go` (KEEP — it tests `Version` which is still used in the new main.go)
- `que es viewred.md`, `design.md`, `docs/...`, `diseño-templete/` (leave alone, will be tracked later)
- `.gitignore`, `.gitattributes`, `.editorconfig` (do not touch)

---

## Task: Add Gin router with health endpoint (TDD)

**Files:**
- Create: `internal/api/router.go`
- Create: `internal/api/router_test.go`
- Create: `internal/api/handlers/health.go`
- Create: `internal/api/handlers/health_test.go`
- Modify: `cmd/viewred/main.go` (replace with new version that uses the router)
- Modify: `go.mod` (Gin + testify added via `go get`)

### Cycle 1: Health handler

- [ ] **Step 1: Add Gin dependency**

```bash
cd C:\Users\Yeikxz\Desktop\ViewRed
go get github.com/gin-gonic/gin@latest
go get github.com/stretchr/testify@latest
```

Expected: `go.mod` and `go.sum` updated.

- [ ] **Step 2: Write failing test for health handler**

Create `C:\Users\Yeikxz\Desktop\ViewRed\internal\api\handlers\health_test.go`:

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
cd C:\Users\Yeikxz\Desktop\ViewRed
go test ./internal/api/handlers/...
```

Expected: FAIL with `undefined: Health` (compile error).

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
cd C:\Users\Yeikxz\Desktop\ViewRed
go test ./internal/api/handlers/... -v
```

Expected: PASS for both `TestHealthReturnsOK` and `TestHealthRejectsNonGET`.

### Cycle 2: Router

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
cd C:\Users\Yeikxz\Desktop\ViewRed
go test ./internal/api/...
```

Expected: FAIL with `undefined: NewRouter` (compile error).

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
cd C:\Users\Yeikxz\Desktop\ViewRed
go test ./internal/api/... -v
```

Expected: PASS for both router tests.

### Cycle 3: Wire it up

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

Note: This replacement **keeps** the `Version` constant and `--version` flag. Do NOT remove them. `TestVersion` in `cmd/viewred/main_test.go` will still pass.

- [ ] **Step 11: Run all tests + smoke run**

```bash
cd C:\Users\Yeikxz\Desktop\ViewRed
go test ./...
go run ./cmd/viewred &
SERVER_PID=$!
sleep 2
curl -s http://127.0.0.1:8080/api/v1/health
echo ""
# On Windows PowerShell, the process handling is different. Either:
#   - Use `Start-Process -NoNewWindow` and `Stop-Process`, OR
#   - Run `go run` in a separate terminal, curl from this one
# Kill the server
kill $SERVER_PID 2>/dev/null
# On Windows: `Stop-Process -Id $SERVER_PID` if you used Start-Process
```

On Windows specifically (PowerShell), use this alternative for the smoke test:

```powershell
cd C:\Users\Yeikxz\Desktop\ViewRed
go test ./...
Start-Process -FilePath "go" -ArgumentList "run","./cmd/viewred" -NoNewWindow
Start-Sleep -Seconds 2
$response = Invoke-RestMethod -Uri "http://127.0.0.1:8080/api/v1/health" -Method Get
$response
Stop-Process -Name "viewred" -Force -ErrorAction SilentlyContinue
Stop-Process -Name "go" -Force -ErrorAction SilentlyContinue
```

Expected: tests pass; the response is an object with `status: "ok"` and `uptime_seconds: <number>`.

- [ ] **Step 12: Commit**

```bash
cd C:\Users\Yeikxz\Desktop\ViewRed
git add go.mod go.sum internal/ cmd/
git commit -m "feat: add Gin router with /api/v1/health endpoint"
```

---

## Verification

Before reporting back, verify:

1. `go test ./...` passes (all tests across all packages).
2. `go run ./cmd/viewred` starts and serves `/api/v1/health` with status 200 and JSON body `{"status":"ok","uptime_seconds":N}`.
3. The 4 new files exist: `internal/api/router.go`, `internal/api/router_test.go`, `internal/api/handlers/health.go`, `internal/api/handlers/health_test.go`.
4. `cmd/viewred/main.go` was updated (router wired in), but `Version` and `--version` flag still work.
5. Single commit on `main` with the expected message.

---

## What to report

Write your full report to: `C:\Users\Yeikxz\Desktop\ViewRed\docs\superpowers\reports\2026-06-17-bootstrap-monorepo-task-03.md`

Include:
- What you implemented
- TDD evidence: 3 RED/GREEN cycles (Cycle 1 for health handler, Cycle 2 for router, Cycle 3 is wiring — no separate RED for that one)
- Files created/modified
- Commit SHA + subject
- Smoke run output (the actual `curl` or `Invoke-RestMethod` response)
- Any concerns

Then reply with ONLY (under 15 lines):

- **Status:** DONE | DONE_WITH_CONCERNS | BLOCKED | NEEDS_CONTEXT
- Commits created (short SHA + subject)
- One-line test summary (e.g. "4/4 tests pass, /api/v1/health returns 200 OK")
- Your concerns, if any
- The report file path

## When you're stuck

If `go get` fails, tests don't behave as expected, or anything unusual, **stop and ask** with NEEDS_CONTEXT. Don't guess.