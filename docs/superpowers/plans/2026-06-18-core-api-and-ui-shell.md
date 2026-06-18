# Core API and UI Shell Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the bridge between Plan 2's capture/storage pipeline and Plan 1's React UI shell. Add the HTTP API layer (REST endpoints + SSE streams) with middleware, storage query methods, an SSE broker, and wire the frontend to consume real data via React Router, Zustand, and a custom SSE client hook. This is plan 3 of 8.

**Architecture:** Same monolith as Plan 2. New `internal/storage/queries.go` adds read methods behind a `Querier` interface (with `MockQuerier` and `ErrorQuerier` for tests and DB-down). New `internal/sse/` package hosts the broker with a 64-event ring buffer and 30s heartbeat. New `internal/api/middleware/` for CORS, request logging, and request ID. Router gains `NewRouterWithDeps(RouterDeps)` (additive — back-compat preserved). Frontend migrates state-based routing to React Router v7, adds Zustand for shared state, and a `useSSE` hook for live data.

**Tech Stack:** Go 1.25+ (existing), Gin (existing), pgx/v5 (existing), gopacket (existing), `react-router-dom` v7 (new), `zustand` (new). No new Go libraries — uses raw `text/event-stream` and `http.Flusher` for SSE. shadcn/ui is **deferred to Plan 8** — Plan 3 uses custom components only.

## Global Constraints

- **TDD Iron Law**: NO PRODUCTION CODE WITHOUT A FAILING TEST FIRST. Every Go handler, query method, broker method, and frontend hook/component must have a failing test watched before any production code.
- **Mocks only when unavoidable**: sqlmock for query methods (proven Plan 2 pattern), MockQuerier for handler tests, real EventSource mocking in jsdom for frontend.
- **No AI attribution in commits**. No "Co-Authored-By".
- **Conventional commits**: `feat(api):`, `feat(storage):`, `feat(sse):`, `feat(web):`, `fix:`, `test:`, `chore:`, `refactor:`.
- **No secrets in repo**. Dev placeholders only.
- **Bind address**: 127.0.0.1:8080.
- **Cross-platform**: works on Windows + Linux. Build tags where needed (none new in this plan).
- **shadcn/ui DEFERRED to Plan 8**. Plan 3 uses custom components.
- **ErrorQuerier pattern**: when storage is unavailable, main.go injects `ErrorQuerier` (returns 503 for all 5 methods). Removes nil checks from handlers.
- **SSE ring buffer 64 + 30s heartbeat**. Drop oldest on overflow. 3× heartbeat = stale client removal.
- **6-step frontend migration** to avoid breaking the UI mid-migration.
- **Back-compat preserved**: `NewRouter()` and `NewRouterWithRegistry()` keep working; new `NewRouterWithDeps()` is additive.

---

## File Structure (target after plan 3)

```
viewred/
├── internal/
│   ├── api/
│   │   ├── middleware/                        # NEW
│   │   │   ├── cors.go
│   │   │   ├── cors_test.go
│   │   │   ├── logging.go
│   │   │   ├── logging_test.go
│   │   │   ├── request_id.go
│   │   │   └── request_id_test.go
│   │   ├── handlers/
│   │   │   ├── dashboard.go                   # NEW
│   │   │   ├── dashboard_test.go
│   │   │   ├── packets.go                     # NEW
│   │   │   ├── packets_test.go
│   │   │   ├── devices.go                     # NEW
│   │   │   ├── devices_test.go
│   │   │   ├── alerts.go                      # NEW (stub)
│   │   │   ├── alerts_test.go
│   │   │   ├── topology.go                    # NEW (stub)
│   │   │   ├── topology_test.go
│   │   │   ├── vulnerabilities.go             # NEW (stub)
│   │   │   ├── vulnerabilities_test.go
│   │   │   ├── config.go                      # NEW (stub)
│   │   │   ├── config_test.go
│   │   │   ├── sse.go                         # NEW
│   │   │   └── sse_test.go
│   │   ├── router.go                          # MODIFIED: add NewRouterWithDeps
│   │   └── router_test.go                     # MODIFIED: tests for new constructor
│   ├── storage/
│   │   ├── querier.go                         # NEW: interface, MockQuerier, ErrorQuerier
│   │   ├── querier_test.go
│   │   ├── queries.go                         # NEW: 5 query methods
│   │   ├── queries_test.go
│   │   ├── storage.go                         # MODIFIED: implement Querier on Storage
│   │   ├── storage_test.go                    # MODIFIED: add Querier tests
│   │   ├── writer.go                          # UNCHANGED
│   │   ├── migrate.go                         # UNCHANGED
│   │   └── migrations/0001_initial.up.sql     # UNCHANGED
│   ├── sse/                                   # NEW
│   │   ├── broker.go
│   │   ├── broker_test.go
│   │   ├── client.go
│   │   └── event.go
│   ├── capture/                               # UNCHANGED from Plan 2
│   ├── config/                                # UNCHANGED
│   ├── health/                                # UNCHANGED
│   ├── shutdown/
│   │   └── shutdown.go                        # MODIFIED: add broker.Shutdown step
│   ├── api/                                   # (parent dir)
│   └── ...
├── cmd/viewred/
│   └── main.go                                # MODIFIED: wire broker, querier, ticker
├── web/
│   ├── package.json                           # MODIFIED: add react-router-dom, zustand
│   ├── src/
│   │   ├── App.tsx                            # MODIFIED: state-based → Router-based
│   │   ├── main.tsx                           # UNCHANGED
│   │   ├── components/
│   │   │   ├── DashboardView.tsx              # MODIFIED: real SSE data
│   │   │   ├── DashboardView.test.tsx
│   │   │   ├── CapturaView.tsx                # MODIFIED: real health polling
│   │   │   ├── CapturaView.test.tsx
│   │   │   ├── Sidebar.tsx                    # MODIFIED: Link instead of onClick
│   │   │   ├── Sidebar.test.tsx
│   │   │   └── ... (other views UNCHANGED for now)
│   │   ├── lib/
│   │   │   ├── sse.ts                         # NEW: useSSE hook
│   │   │   ├── sse.test.ts
│   │   │   ├── api.ts                         # NEW: fetch wrapper
│   │   │   └── api.test.ts
│   │   └── store/
│   │       ├── sse-store.ts                   # NEW: Zustand store
│   │       └── sse-store.test.ts
└── docs/superpowers/
    ├── plans/2026-06-18-core-api-and-ui-shell.md   # this file
    └── ... (other docs)
```

---

### Task 1: CORS middleware

**Files:**
- Create: `internal/api/middleware/cors.go`
- Create: `internal/api/middleware/cors_test.go`

**Interfaces:**
```go
package middleware

// CORSConfig controls CORS behavior.
type CORSConfig struct {
    AllowedOrigins   []string  // e.g., ["http://localhost:5173"] (Vite dev)
    AllowedMethods   []string  // e.g., ["GET", "POST", "PATCH", "DELETE", "OPTIONS"]
    AllowedHeaders   []string  // e.g., ["Content-Type", "X-Request-Id"]
    AllowCredentials bool
}

// NewCORS returns a Gin middleware that sets CORS headers and handles preflight.
func NewCORS(cfg CORSConfig) gin.HandlerFunc
```

- [x] **Step 1: RED (failing test)**

Create `internal/api/middleware/cors_test.go`:
```go
package middleware

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"
)

func TestCORS_SetsHeadersForAllowedOrigin(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.Use(NewCORS(CORSConfig{
        AllowedOrigins: []string{"http://localhost:5173"},
        AllowedMethods: []string{"GET", "POST", "OPTIONS"},
        AllowedHeaders: []string{"Content-Type"},
    }))
    r.GET("/test", func(c *gin.Context) { c.Status(200) })

    req := httptest.NewRequest(http.MethodGet, "/test", nil)
    req.Header.Set("Origin", "http://localhost:5173")
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)

    if w.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d", w.Code)
    }
    if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
        t.Errorf("expected Allow-Origin header, got %q", got)
    }
    if got := w.Header().Get("Vary"); got != "Origin" {
        t.Errorf("expected Vary: Origin header, got %q", got)
    }
}

func TestCORS_RejectsDisallowedOrigin(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.Use(NewCORS(CORSConfig{
        AllowedOrigins: []string{"http://localhost:5173"},
    }))
    r.GET("/test", func(c *gin.Context) { c.Status(200) })

    req := httptest.NewRequest(http.MethodGet, "/test", nil)
    req.Header.Set("Origin", "http://evil.example.com")
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)

    if w.Code != http.StatusForbidden {
        t.Fatalf("expected 403 for disallowed origin, got %d", w.Code)
    }
    if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
        t.Errorf("expected no Allow-Origin for disallowed, got %q", got)
    }
}

func TestCORS_HandlesPreflight(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.Use(NewCORS(CORSConfig{
        AllowedOrigins: []string{"http://localhost:5173"},
        AllowedMethods: []string{"GET", "POST", "OPTIONS"},
        AllowedHeaders: []string{"Content-Type"},
    }))
    r.POST("/test", func(c *gin.Context) { c.Status(200) })

    req := httptest.NewRequest(http.MethodOptions, "/test", nil)
    req.Header.Set("Origin", "http://localhost:5173")
    req.Header.Set("Access-Control-Request-Method", "POST")
    req.Header.Set("Access-Control-Request-Headers", "Content-Type")
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)

    if w.Code != http.StatusNoContent {
        t.Fatalf("expected 204 for preflight, got %d", w.Code)
    }
}
```

Run: `go test ./internal/api/middleware/...` — expect build failure: `undefined: NewCORS, CORSConfig`.

- [x] **Step 2: GREEN (minimal implementation)**

Create `internal/api/middleware/cors.go`:
```go
package middleware

import (
    "net/http"
    "slices"
    "strings"

    "github.com/gin-gonic/gin"
)

// CORSConfig controls CORS behavior.
type CORSConfig struct {
    AllowedOrigins   []string
    AllowedMethods   []string
    AllowedHeaders   []string
    AllowCredentials bool
}

// NewCORS returns a Gin middleware that sets CORS headers and handles preflight.
func NewCORS(cfg CORSConfig) gin.HandlerFunc {
    originAllowed := func(origin string) bool {
        return slices.Contains(cfg.AllowedOrigins, origin)
    }
    return func(c *gin.Context) {
        origin := c.GetHeader("Origin")
        if origin == "" {
            c.Next()
            return
        }
        if !originAllowed(origin) {
            c.AbortWithStatus(http.StatusForbidden)
            return
        }
        c.Header("Access-Control-Allow-Origin", origin)
        c.Header("Vary", "Origin")
        if cfg.AllowCredentials {
            c.Header("Access-Control-Allow-Credentials", "true")
        }
        if c.Request.Method == http.MethodOptions {
            if len(cfg.AllowedMethods) > 0 {
                c.Header("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ", "))
            }
            if len(cfg.AllowedHeaders) > 0 {
                c.Header("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ", "))
            }
            c.AbortWithStatus(http.StatusNoContent)
            return
        }
        c.Next()
    }
}
```

Run: `go test ./internal/api/middleware/...` — all 3 tests pass.

- [ ] **Step 3: Verify**
- `go test ./...` — all packages still pass (no existing tests broken)
- `go vet ./...` — clean
- `go build ./...` — clean

- [ ] **Step 4: Commit**
```bash
git add internal/api/middleware/cors.go internal/api/middleware/cors_test.go
git commit -m "feat(api): add CORS middleware with configurable origins"
```

---

### Task 2: Request logging middleware

**Files:**
- Create: `internal/api/middleware/logging.go`
- Create: `internal/api/middleware/logging_test.go`

**Interfaces:**
```go
package middleware

import "github.com/gin-gonic/gin"

// RequestLogging logs each HTTP request with method, path, status, duration, request_id.
// Uses log/slog (stdlib).
func RequestLogging() gin.HandlerFunc
```

- [x] **Step 1: RED (failing test)**

Create `internal/api/middleware/logging_test.go`:
```go
package middleware

import (
    "bytes"
    "log/slog"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"

    "github.com/gin-gonic/gin"
)

func TestRequestLogging_LogsMethodPathStatus(t *testing.T) {
    var buf bytes.Buffer
    old := slog.Default()
    slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
    defer slog.SetDefault(old)

    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.Use(RequestLogging())
    r.GET("/x", func(c *gin.Context) { c.Status(200) })

    req := httptest.NewRequest(http.MethodGet, "/x", nil)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)

    out := buf.String()
    if !strings.Contains(out, "method=GET") {
        t.Errorf("expected method=GET in log, got %q", out)
    }
    if !strings.Contains(out, "path=/x") {
        t.Errorf("expected path=/x in log, got %q", out)
    }
    if !strings.Contains(out, "status=200") {
        t.Errorf("expected status=200 in log, got %q", out)
    }
}
```

Run: `go test ./internal/api/middleware/...` — expect failure: `undefined: RequestLogging`.

- [x] **Step 2: GREEN (minimal implementation)**

Create `internal/api/middleware/logging.go`:
```go
package middleware

import (
    "log/slog"
    "time"

    "github.com/gin-gonic/gin"
)

// RequestLogging logs each HTTP request.
func RequestLogging() gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        c.Next()
        slog.Info("http_request",
            "method", c.Request.Method,
            "path", c.Request.URL.Path,
            "status", c.Writer.Status(),
            "duration_ms", time.Since(start).Milliseconds(),
            "request_id", c.GetString("request_id"),
        )
    }
}
```

Run: `go test ./internal/api/middleware/...` — passes.

- [x] **Step 3: Verify + Commit** (same pattern as Task 1)
```bash
git add internal/api/middleware/logging.go internal/api/middleware/logging_test.go
git commit -m "feat(api): add request logging middleware with slog"
```

---

### Task 3: Request ID middleware

**Files:**
- Create: `internal/api/middleware/request_id.go`
- Create: `internal/api/middleware/request_id_test.go`

**Interfaces:**
```go
package middleware

import "github.com/gin-gonic/gin"

// RequestIDHeader is the canonical header name.
const RequestIDHeader = "X-Request-Id"

// RequestID generates a UUID per request (or reuses the inbound header if present).
func RequestID() gin.HandlerFunc
```

- [x] **Step 1: RED (failing test)**

```go
func TestRequestID_GeneratesWhenMissing(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.Use(RequestID())
    r.GET("/x", func(c *gin.Context) {
        c.String(200, c.GetString("request_id"))
    })

    req := httptest.NewRequest(http.MethodGet, "/x", nil)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)

    if w.Body.String() == "" {
        t.Errorf("expected request id in body, got empty")
    }
    if got := w.Header().Get(RequestIDHeader); got == "" {
        t.Errorf("expected X-Request-Id header, got empty")
    }
    if w.Body.String() != w.Header().Get(RequestIDHeader) {
        t.Errorf("body and header request_id mismatch")
    }
}

func TestRequestID_ReusesInbound(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.Use(RequestID())
    r.GET("/x", func(c *gin.Context) { c.Status(200) })

    req := httptest.NewRequest(http.MethodGet, "/x", nil)
    req.Header.Set(RequestIDHeader, "test-12345")
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)

    if got := w.Header().Get(RequestIDHeader); got != "test-12345" {
        t.Errorf("expected inbound id, got %q", got)
    }
}
```

- [x] **Step 2: GREEN (minimal implementation)**

```go
package middleware

import (
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
)

const RequestIDHeader = "X-Request-Id"

func RequestID() gin.HandlerFunc {
    return func(c *gin.Context) {
        id := c.GetHeader(RequestIDHeader)
        if id == "" {
            id = uuid.NewString()
        }
        c.Set("request_id", id)
        c.Header(RequestIDHeader, id)
        c.Next()
    }
}
```

NOTE: requires `github.com/google/uuid` — check if it's already a transitive dep from gin; if not, `go get github.com/google/uuid`.

- [x] **Step 3: Verify + Commit**
```bash
git add internal/api/middleware/request_id.go internal/api/middleware/request_id_test.go
git commit -m "feat(api): add X-Request-Id middleware with UUID generation"
```

---

### Task 4: storage.Querier interface

**Files:**
- Create: `internal/storage/querier.go`
- Create: `internal/storage/querier_test.go`

**Interfaces:**
```go
package storage

import (
    "context"
    "time"

    "github.com/viewred/viewred/internal/capture"
)

// DashboardSummary contains KPI metrics for the dashboard.
type DashboardSummary struct {
    PacketsPerSec      float64
    BytesPerSec        float64
    ActiveConnections  int64
    DeviceCount        int64
}

// TopTalker represents a single IP's traffic.
type TopTalker struct {
    IP    string
    Bytes int64
}

// ProtocolShare represents a protocol's share of total traffic.
type ProtocolShare struct {
    Protocol string
    Percent  float64
}

// PacketFilter contains filters for the packet list endpoint.
type PacketFilter struct {
    Since    time.Time
    Until    time.Time
    Protocol string
    SrcIP    string
    DstIP    string
    Limit    int
    Offset   int
}

// DeviceFilter contains filters for the device list endpoint.
type DeviceFilter struct {
    Limit  int
    Offset int
}

// Device is a discovered host.
type Device struct {
    ID         int64
    IP         string
    MAC        string
    Vendor     string
    Hostname   string
    FirstSeen  time.Time
    LastSeen   time.Time
    RiskScore  *float64
}

// Querier is the read interface for storage.
type Querier interface {
    QueryDashboardSummary(ctx context.Context, since time.Time) (DashboardSummary, error)
    QueryTopTalkers(ctx context.Context, since time.Time, limit int) ([]TopTalker, error)
    QueryProtocolDistribution(ctx context.Context, since time.Time) ([]ProtocolShare, error)
    QueryPackets(ctx context.Context, filter PacketFilter) ([]capture.PacketEvent, int, error)
    QueryDevices(ctx context.Context, filter DeviceFilter) ([]Device, int, error)
}

// MockQuerier is a test double with function fields (zero defaults return zero values).
type MockQuerier struct {
    DashboardSummaryFn func(ctx context.Context, since time.Time) (DashboardSummary, error)
    TopTalkersFn       func(ctx context.Context, since time.Time, limit int) ([]TopTalker, error)
    ProtocolDistFn     func(ctx context.Context, since time.Time) ([]ProtocolShare, error)
    PacketsFn          func(ctx context.Context, filter PacketFilter) ([]capture.PacketEvent, int, error)
    DevicesFn          func(ctx context.Context, filter DeviceFilter) ([]Device, int, error)
}

// Compile-time check that MockQuerier implements Querier.
var _ Querier = (*MockQuerier)(nil)

// ErrorQuerier returns errors for all 5 methods (used when DB is unavailable).
type ErrorQuerier struct{ Err error }

// Compile-time check that ErrorQuerier implements Querier.
var _ Querier = (*ErrorQuerier)(nil)
```

- [x] **Step 1: RED (failing test)**

```go
func TestMockQuerier_ImplementsQuerier(t *testing.T) {
    var _ Querier = (*MockQuerier)(nil)
    var _ Querier = (*ErrorQuerier)(nil)
}

func TestErrorQuerier_ReturnsErrOnAllMethods(t *testing.T) {
    sentinel := errors.New("db down")
    q := &ErrorQuerier{Err: sentinel}
    ctx := context.Background()
    since := time.Now()

    if _, err := q.QueryDashboardSummary(ctx, since); !errors.Is(err, sentinel) {
        t.Errorf("QueryDashboardSummary: expected %v, got %v", sentinel, err)
    }
    if _, err := q.QueryTopTalkers(ctx, since, 10); !errors.Is(err, sentinel) {
        t.Errorf("QueryTopTalkers: expected %v, got %v", sentinel, err)
    }
    if _, err := q.QueryProtocolDistribution(ctx, since); !errors.Is(err, sentinel) {
        t.Errorf("QueryProtocolDistribution: expected %v, got %v", sentinel, err)
    }
    if _, _, err := q.QueryPackets(ctx, PacketFilter{}); !errors.Is(err, sentinel) {
        t.Errorf("QueryPackets: expected %v, got %v", sentinel, err)
    }
    if _, _, err := q.QueryDevices(ctx, DeviceFilter{}); !errors.Is(err, sentinel) {
        t.Errorf("QueryDevices: expected %v, got %v", sentinel, err)
    }
}

func TestMockQuerier_DefaultsToZeroValues(t *testing.T) {
    q := &MockQuerier{}
    ctx := context.Background()
    since := time.Now()

    summary, err := q.QueryDashboardSummary(ctx, since)
    if err != nil { t.Fatalf("unexpected err: %v", err) }
    if summary.PacketsPerSec != 0 || summary.BytesPerSec != 0 {
        t.Errorf("expected zero summary, got %+v", summary)
    }
}

func TestMockQuerier_DelegatesToFunctionFields(t *testing.T) {
    expected := DashboardSummary{PacketsPerSec: 42.0}
    q := &MockQuerier{
        DashboardSummaryFn: func(ctx context.Context, since time.Time) (DashboardSummary, error) {
            return expected, nil
        },
    }
    got, err := q.QueryDashboardSummary(context.Background(), time.Now())
    if err != nil { t.Fatalf("unexpected err: %v", err) }
    if got.PacketsPerSec != 42.0 { t.Errorf("expected 42.0, got %v", got.PacketsPerSec) }
}
```

- [x] **Step 2: GREEN (minimal implementation)**

Implement MockQuerier (5 method delegations) and ErrorQuerier (5 method error returns). Code is mechanical; see the patterns above.

- [x] **Step 3: Verify + Commit**
```bash
git add internal/storage/querier.go internal/storage/querier_test.go
git commit -m "feat(storage): add Querier interface with MockQuerier and ErrorQuerier"
```

---

### Tasks 5-9: Storage query methods

Five query methods, one task each. They follow a consistent pattern:
1. Define the SQL in a const at the top of the function
2. Write a failing test with go-sqlmock expectations
3. Implement the query
4. Verify

Example pattern (apply to all 5):

#### Task 5: QueryDashboardSummary

**Files:**
- Modify: `internal/storage/queries.go` (new file)
- Modify: `internal/storage/queries_test.go` (new file)

```go
// queries.go
const qDashboardSummary = `
SELECT
    COALESCE(COUNT(*), 0) AS packet_count,
    COALESCE(SUM(length), 0) AS byte_count,
    COALESCE(COUNT(DISTINCT src_ip), 0) AS device_count
FROM packets
WHERE time >= $1
`

func (s *Storage) QueryDashboardSummary(ctx context.Context, since time.Time) (DashboardSummary, error) {
    // implementation: query, scan into DashboardSummary, return
}
```

Test uses sqlmock:
```go
func TestQueryDashboardSummary_ReturnsKPIs(t *testing.T) {
    db, mock, _ := sqlmock.New()
    defer db.Close()
    // set up mock expectation
    mock.ExpectQuery(qDashboardSummary).
        WithArgs(sqlmock.AnyArg()).
        WillReturnRows(sqlmock.NewRows([]string{"packet_count", "byte_count", "device_count"}).
            AddRow(1000, 500000, 25))

    // call method, assert result
}
```

Commit: `feat(storage): add QueryDashboardSummary with sqlmock test`

**Tasks 6-9 follow the same pattern** for `QueryTopTalkers`, `QueryProtocolDistribution`, `QueryPackets`, `QueryDevices`. Use the design doc §3 for exact SQL and return types.

---

### Tasks 10-12: Dashboard, Packet, Device handlers

Three task groups. Each handler:
- Takes a `Querier` parameter
- Parses query params
- Calls querier
- Returns JSON
- Handles 503 (when ErrorQuerier is injected, errors propagate naturally)

#### Task 10: Dashboard handlers

**Files:**
- Create: `internal/api/handlers/dashboard.go` ✅
- Create: `internal/api/handlers/dashboard_test.go` ✅

Three handlers in one file: `DashboardSummary`, `TopTalkers`, `ProtocolDistribution`. Each takes `storage.Querier` and returns JSON.

Test pattern (one for each):
```go
func TestDashboardSummary_ReturnsKPIs(t *testing.T) {
    q := &storage.MockQuerier{
        DashboardSummaryFn: func(ctx context.Context, since time.Time) (storage.DashboardSummary, error) {
            return storage.DashboardSummary{PacketsPerSec: 1234.5, BytesPerSec: 4.2e6, ActiveConnections: 234, DeviceCount: 47}, nil
        },
    }
    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.GET("/api/v1/dashboard/summary", DashboardSummary(q))

    req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/summary", nil)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)

    if w.Code != 200 { t.Fatalf("expected 200, got %d", w.Code) }
    var body map[string]any
    json.Unmarshal(w.Body.Bytes(), &body)
    if body["packets_per_sec"] != 1234.5 { t.Errorf("got %v", body["packets_per_sec"]) }
}

func TestDashboardSummary_Returns503OnError(t *testing.T) {
    q := &storage.ErrorQuerier{Err: errors.New("db down")}
    // ... assert 503
}
```

Commit: `feat(api): add dashboard handlers (summary, top-talkers, protocols)`

**Tasks 11-12 follow the same pattern** for `packets.go` and `devices.go`.

---

### Task 13: Stub handlers (5 endpoints)

**Files:**
- Create: `internal/api/handlers/stubs.go` (single file with all 5 stubs)
- Create: `internal/api/handlers/stubs_test.go`

Five stub handlers registered as routes:
- `GET /api/v1/alerts` → `[]`
- `POST /api/v1/alerts/:id/ack` → `{"acknowledged": false}`
- `GET /api/v1/topology` → `{"nodes": [], "edges": []}`
- `GET /api/v1/vulnerabilities` → `[]`
- `GET /api/v1/config` → empty config object

Test: each handler returns 200 with the expected empty shape. Commit: `feat(api): add stub handlers for alerts, topology, vulns, config`.

---

### Task 14: SSE broker

**Files:**
- Create: `internal/sse/broker.go`
- Create: `internal/sse/broker_test.go`
- Create: `internal/sse/client.go`
- Create: `internal/sse/event.go`

**Interfaces:**
```go
package sse

// Event is a single SSE message.
type Event struct {
    Name string  // event: <name>
    Data []byte  // data: <data>
}

// Broker manages connected SSE clients.
type Broker struct {
    mu          sync.RWMutex
    clients     map[*client]struct{}
    bufferSize  int
    heartbeat   time.Duration
    register    chan *client
    unregister  chan *client
    broadcast   chan Event
}

// NewBroker creates a broker with sensible defaults (buffer 64, heartbeat 30s).
func NewBroker() *Broker

// Run starts the broker's event loop. Cancel ctx to stop.
func (b *Broker) Run(ctx context.Context)

// Subscribe registers a new SSE client subscribed to the given topics.
// Returns a channel of events for this client and an unsubscribe func.
func (b *Broker) Subscribe(ctx context.Context, topics []string) (<-chan Event, func())

// Broadcast sends an event to all clients subscribed to the event's topic (or all if topic is empty).
func (b *Broker) Broadcast(e Event)

// Shutdown closes all client channels and stops the broker.
func (b *Broker) Shutdown(ctx context.Context) error

// ActiveClientCount returns the current number of connected clients.
func (b *Broker) ActiveClientCount() int
```

Tests (table-driven):
- `TestBroker_SubscribeAndReceive`: subscribe, broadcast, assert received
- `TestBroker_RingBufferOverflow`: subscribe with full buffer, broadcast, assert oldest dropped
- `TestBroker_Unsubscribe`: subscribe, unsubscribe, broadcast, assert not received
- `TestBroker_StaleClientRemoval`: client context cancelled, broker removes after 3× heartbeat
- `TestBroker_ConcurrentSubscribeAndBroadcast`: run with -race, no panics

Commit: `feat(sse): add broker with ring buffer and heartbeat`

---

### Task 15: SSE handler (/stream/metrics + stubs)

**Files:**
- Create: `internal/api/handlers/sse.go`
- Create: `internal/api/handlers/sse_test.go`

**Interfaces:**
```go
package handlers

import (
    "context"
    "time"

    "github.com/viewred/viewred/internal/sse"
    "github.com/viewred/viewred/internal/storage"
)

// StreamMetricsHandler streams dashboard metrics at 1 Hz.
type StreamMetricsHandlerDeps struct {
    Broker  *sse.Broker
    Querier storage.Querier
}

func StreamMetricsHandler(deps StreamMetricsHandlerDeps) gin.HandlerFunc

// StreamStubHandler is a placeholder for alerts/topology/llm streams.
type StreamStubHandlerDeps struct {
    Broker  *sse.Broker
    Topic   string
}

func StreamStubHandler(deps StreamStubHandlerDeps) gin.HandlerFunc
```

Test: real broker, MockQuerier, assert SSE event format `event: metrics\ndata: {...}\n\n` at ~1 Hz.

Commit: `feat(api): add SSE metrics stream and stub handlers for alerts/topology/llm`

---

### Task 16: Router refactor (NewRouterWithDeps)

**Files:**
- Modify: `internal/api/router.go`
- Modify: `internal/api/router_test.go`

Add `NewRouterWithDeps(deps RouterDeps) *gin.Engine`:
```go
type RouterDeps struct {
    Health   *health.Registry
    Querier  storage.Querier
    Broker   *sse.Broker
    Config   *config.Config
    CORSConfig middleware.CORSConfig  // optional
}

func NewRouterWithDeps(deps RouterDeps) *gin.Engine {
    // Recovery → RequestID → CORS → RequestLogging → routes
    // Routes: /health, /dashboard/*, /packets*, /devices*, /alerts*, /topology, /vulnerabilities, /config, /stream/*
}
```

Preserve `NewRouter()` and `NewRouterWithRegistry(registry)` for back-compat (they construct minimal deps).

Test: real handlers with MockQuerier, assert routes are registered and respond.

Commit: `feat(api): add NewRouterWithDeps constructor preserving back-compat`

---

### Task 17: main.go wire-up

**Files:**
- Modify: `cmd/viewred/main.go`

Changes:
- Create SSE broker
- Run broker in goroutine
- If storage init succeeds: use `*Storage` as Querier
- If storage init fails: use `&storage.ErrorQuerier{Err: ...}`
- Create metrics ticker (1 Hz) goroutine that queries dashboard + broadcasts
- Pass broker, querier, config to `NewRouterWithDeps`
- Defer broker.Shutdown in the right order

Commit: `feat(cmd): wire SSE broker and Querier into main.go lifecycle`

---

### Task 18: Shutdown integration

**Files:**
- Modify: `internal/shutdown/shutdown.go`
- Modify: `internal/shutdown/shutdown_test.go`

Add `Broker` to shutdown.Params with `Shutdown(ctx) error` method. Insert broker.Shutdown after server.Shutdown, before store.Close.

Test: table-driven with mock subsystems + broker, assert order via spy.

Commit: `feat(shutdown): integrate SSE broker into ordered teardown`

---

### Task 19: Frontend deps + Zustand store

**Files:**
- Modify: `web/package.json` (add `react-router-dom` v7, `zustand`)
- Create: `web/src/store/sse-store.ts`
- Create: `web/src/store/sse-store.test.ts`

```typescript
// sse-store.ts
import { create } from 'zustand'

export type ConnectionStatus = 'connecting' | 'connected' | 'disconnected' | 'reconnecting'

export interface SSEStoreState {
  messages: Map<string, unknown>  // topic -> last message
  connectionStatus: ConnectionStatus
  captureStatus: 'running' | 'paused' | 'unknown'
  lastError: string | null
  addMessage: (topic: string, data: unknown) => void
  setConnectionStatus: (status: ConnectionStatus) => void
  setCaptureStatus: (status: 'running' | 'paused' | 'unknown') => void
  setLastError: (err: string | null) => void
  clearMessages: () => void
}

export const useSSEStore = create<SSEStoreState>((set) => ({
  messages: new Map(),
  connectionStatus: 'disconnected',
  captureStatus: 'unknown',
  lastError: null,
  addMessage: (topic, data) => set((s) => {
    const next = new Map(s.messages)
    next.set(topic, data)
    return { messages: next }
  }),
  setConnectionStatus: (status) => set({ connectionStatus: status }),
  setCaptureStatus: (status) => set({ captureStatus: status }),
  setLastError: (err) => set({ lastError: err }),
  clearMessages: () => set({ messages: new Map() }),
}))
```

Tests with Vitest: assert state updates, immutability, etc.

Commit: `feat(web): add zustand store for SSE and capture status`

---

### Task 20: useSSE hook

**Files:**
- Create: `web/src/lib/sse.ts`
- Create: `web/src/lib/sse.test.ts`

```typescript
export interface UseSSEResult<T> {
  lastEvent: T | null
  connectionStatus: ConnectionStatus
  error: string | null
  reconnect: () => void
}

export function useSSE<T = unknown>(topic: string): UseSSEResult<T> {
  // native EventSource with auto-reconnect (exponential backoff: 1s, 2s, 4s, 8s, max 30s)
  // updates zustand store
  // cleanup on unmount
}
```

Test: mock EventSource, assert state updates on message, cleanup on unmount, reconnect on close.

Commit: `feat(web): add useSSE hook with auto-reconnect`

---

### Task 21: React Router setup

**Files:**
- Modify: `web/src/App.tsx` (state-based → Router-based)
- Modify: `web/src/App.test.tsx` (or create if missing)

Replace `currentView: ViewType` state with `<BrowserRouter>` and `<Routes>`:
```tsx
<BrowserRouter>
  <Routes>
    <Route path="/" element={<Navigate to="/dashboard" replace />} />
    <Route path="/dashboard" element={<DashboardView />} />
    <Route path="/captura" element={<CapturaView />} />
    <Route path="/filtros" element={<FiltrosView />} />
    <Route path="/alertas" element={<AlertasView />} />
    <Route path="/topologia" element={<TopologiaView />} />
    <Route path="/vulnerabilidades" element={<VulnerabilidadesView />} />
    <Route path="/asistente" element={<AsistenteView />} />
    <Route path="/configuracion" element={<ConfiguracionView />} />
    <Route path="*" element={<NotFoundView />} />
  </Routes>
</BrowserRouter>
```

Test: use `MemoryRouter` in tests, assert view matches URL.

Commit: `refactor(web): migrate App.tsx from state-based to React Router v7`

---

### Task 22: Sidebar Link migration

**Files:**
- Modify: `web/src/components/Sidebar.tsx`
- Modify: `web/src/components/Sidebar.test.tsx`

Replace `onClick={() => setCurrentView(...)}` with `<Link to="/dashboard">`.

Test: render, click a link, assert URL change (use MemoryRouter).

Commit: `refactor(web): use React Router Link in Sidebar navigation`

---

### Task 23: DashboardView SSE wiring

**Files:**
- Modify: `web/src/components/DashboardView.tsx`
- Modify: `web/src/components/DashboardView.test.tsx`

Replace `setInterval` fakes with `useSSE('/api/v1/stream/metrics')`. Render KPIs from `useSSEStore`.

Test: render with mocked useSSE returning sample metrics, assert DOM contains the values.

Commit: `feat(web): wire DashboardView to real SSE metrics stream`

---

### Task 24: CapturaView health polling

**Files:**
- Modify: `web/src/components/CapturaView.tsx`
- Modify: `web/src/components/CapturaView.test.tsx`

Replace mock capture counters with polling `/api/v1/health` every 2s. Show real status.

Test: mock fetch, render, assert status display.

Commit: `feat(web): wire CapturaView to real health endpoint`

---

### Task 25: Loading/error/empty states

**Files:**
- Modify: `web/src/components/DashboardView.tsx` (add states)
- Modify: `web/src/components/DashboardView.test.tsx`

Add three states:
- **Loading**: skeleton with animated bars
- **Error**: error icon + retry button
- **Empty**: placeholder + CTA

Test: render with `connectionStatus='disconnected'`, assert error state. Render with `messages` empty, assert loading state.

Commit: `feat(web): add loading/error/empty states to DashboardView`

---

### Task 26: e2e verification

**Files:**
- Create: `docs/superpowers/plans/2026-06-18-core-api-and-ui-shell.md` — append "End-to-End Verification Results" section
- Modify: `docs/superpowers/progress/` — update ledger

Run:
- `go test ./...` — 9/9 packages pass (added `internal/sse`, possibly `internal/api/middleware`)
- `pnpm test` — all frontend tests pass
- `pnpm build` — production build OK
- `go build -o viewred.exe ./cmd/viewred` — binary builds
- Start binary, curl `/api/v1/health`, `/api/v1/dashboard/summary` (expect 200 or 503 depending on DB), `/api/v1/stream/metrics` (verify event format)
- Start Vite dev, navigate to `/dashboard`, verify it loads

Smoke test with port-based cleanup (orchestrator does this, not subagent).

Commit: `docs(plan): mark Plan 3 core-api-and-ui-shell complete with e2e verification`

---

## Notes for Implementers

- **TDD discipline is mandatory**. Every Go/TS file's commit must show RED output before GREEN. The implementer's return must include `tdd_trace` with the RED output.
- **Smoke tests run in the orchestrator, not in subagents**. Do NOT start the viewred binary in your task — the orchestrator does that with port-based cleanup.
- **Cross-platform**: PowerShell on Windows, but use `workdir` parameter, not `cd`. Use `$env:GOOS = 'linux'` for cross-compile.
- **No AI attribution in commits**. Conventional commits only.
- **If a test passes immediately, you're testing existing behavior — fix the test.**
- **If a test errors (not fails), there's a syntax/setup issue — fix the test setup, not the assertion.**

## End-to-End Verification Results (filled in by Task 26)

(To be populated by the final verification step.)
