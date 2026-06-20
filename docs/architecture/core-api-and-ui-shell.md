# Design: Core API and UI Shell (Plan 3 of 8)

> **Date**: 2026-06-18
> **Plan**: `core-api-and-ui-shell`
> **Base commit**: `296f904` (Plan 2 merged)
> **Spec**: `docs/specs/core-api-and-ui-shell.md`
> **Proposal**: `docs/decisions/core-api-and-ui-shell-proposal.md`
> **Target**: 500-800 lines, focused on **interfaces + data flow + TDD approach**, not implementation code.

---

## Quick path

1. **Backend adds**: 3 middleware (CORS, logging, request ID), `storage.Querier` interface + 5 queries, 7 real + 5 stub REST handlers, 1 SSE broker package, 1 live SSE stream (`/stream/metrics`).
2. **Frontend adds**: React Router v7 (URL routing), Zustand store, `useSSE` hook with auto-reconnect, 6-step migration that wires `DashboardView` and `CapturaView` to real data.
3. **Wires up**: `main.go` builds SSE broker + querier + ticker; shutdown adds broker teardown; new `NewRouterWithDeps(RouterDeps)` constructor replaces the per-handler wiring.
4. **TDD discipline**: every task starts with a failing test. **No production code without a RED step first.**

## Why this design

The spec locks in 5 user decisions: bounded API scope (7 + 5 stubs), React Router, Zustand, real errors (no mock fallback), only `/stream/metrics` live. This design focuses on **interfaces that make those decisions concrete and testable**: `Querier` for mockable DB access, `ErrorQuerier` for the "DB down" path, `sse.Broker` for decoupled broadcast, `RouterDeps` for clean DI without breaking the back-compat `NewRouterWithRegistry`.

---

## 1. Architecture overview

### High-level component diagram

```
┌──────────────────────────────────────────────────────────────────────┐
│                            ViewRed process                            │
│                                                                       │
│  ┌─────────────┐    ┌──────────────────────────────────────────┐     │
│  │  capture    │───▶│  internal/bus (deferred to Plan 4)        │     │
│  └─────────────┘    └──────────────────────────────────────────┘     │
│                          │                                            │
│                          ▼                                            │
│                   ┌──────────────┐     ┌─────────────┐                │
│                   │  storage     │     │  storage    │                │
│                   │  .Writer     │     │  .Querier ◀─┼──┐             │
│                   │  (Plan 2)    │     │  (NEW)      │  │             │
│                   └──────┬───────┘     └──────┬──────┘  │             │
│                          │                    │         │             │
│                          ▼                    ▼         │             │
│                   ┌──────────────┐     ┌───────────────────────┐     │
│                   │ TimescaleDB  │     │   internal/api        │     │
│                   │   hypertables│     │  ┌─────────────────┐ │     │
│                   └──────────────┘     │  │   Middleware    │ │     │
│                                        │  │   Recovery      │ │     │
│                                        │  │   CORS          │ │     │
│                                        │  │   RequestID     │ │     │
│                                        │  │   Logging       │ │     │
│                                        │  └────────┬────────┘ │     │
│                                        │           ▼          │     │
│                                        │  ┌─────────────────┐ │     │
│                                        │  │  Handlers       │ │     │
│                                        │  │  dashboard.go   │ │     │
│                                        │  │  packets.go     │ │     │
│                                        │  │  devices.go     │ │     │
│                                        │  │  sse.go         │ │     │
│                                        │  │  stubs (5)      │ │     │
│                                        │  └────────┬────────┘ │     │
│                                        │           ▼          │     │
│                                        │  ┌─────────────────┐ │     │
│                                        │  │  internal/sse    │ │     │
│                                        │  │  Broker          │ │     │
│                                        │  │  (NEW)           │ │     │
│                                        │  └─────────────────┘ │     │
│                                        └───────────┬──────────┘     │
│                                                    │                 │
│                                                    ▼                 │
│                                          ┌─────────────────┐         │
│                                          │  cmd/viewred    │         │
│                                          │  main.go        │         │
│                                          │  ticker 1Hz     │         │
│                                          └─────────────────┘         │
└──────────────────────────────────────────────────────────────────────┘
                                │
                                ▼
                  ┌──────────────────────────┐
                  │  web/ (Vite + React 19)  │
                  │  ┌────────────────────┐  │
                  │  │  BrowserRouter     │  │
                  │  │  Zustand store     │  │
                  │  │  useSSE hook       │  │
                  │  │  DashboardView ◀───┼── EventSource /stream/metrics
                  │  │  CapturaView       │  │
                  │  └────────────────────┘  │
                  └──────────────────────────┘
```

### Component dependencies

| Caller | Callee | When |
|--------|--------|------|
| `main.go` | `sse.NewBroker()` | At startup, before router |
| `main.go` | `storage.NewQuerier(store)` or `storage.NewErrorQuerier()` | After store init |
| `main.go` | `sse.Broker.Run(ctx)` | Goroutine, lifetime of process |
| `main.go` | `api.NewRouterWithDeps(RouterDeps{...})` | At startup |
| `api.NewRouterWithDeps` | `middleware.NewCORS(...)`, `middleware.RequestID()`, `middleware.RequestLogging()` | Router setup |
| Handlers | `storage.Querier` (interface) | Per request |
| `StreamMetricsHandler` | `sse.Broker.Subscribe(ctx, "metrics")` + 1 Hz ticker | Per SSE connection |
| `sse.Broker` | `storage.Querier.QueryDashboardSummary(...)` | Per broadcast (via handler) |
| Frontend `useSSE(topic)` | `EventSource(/api/v1/stream/{topic})` | Component mount |
| Frontend `useSSE(topic)` | `useSSEStore.addMessage(topic, data)` | On each event |

### Data flow: REST request

```
HTTP request
  → gin.Recovery()                    (panic → 500)
  → middleware.RequestID()            (set X-Request-Id)
  → middleware.NewCORS(...)()         (set CORS headers, handle preflight)
  → middleware.RequestLogging()       (capture start time, defer log)
  → router → v1.GET("/path", Handler)
  → Handler: parse query → call querier.Method(ctx, ...) → 200/4xx/5xx
  → middleware.RequestLogging logs status, duration, request_id
```

### Data flow: SSE stream

```
Browser opens EventSource("/api/v1/stream/metrics")
  → StreamMetricsHandler(ctx, broker, querier)
    → broker.Subscribe(ctx, "metrics") → (eventsCh, unregisterFn)
    → query storage once → emit initial event
    → ticker.C: each second
        → querier.QueryDashboardSummary(ctx, since=60s ago)
        → if OK:   json.Marshal → emit "event: metrics\ndata: <json>\n\n"
        → if FAIL: emit "event: error\ndata: {\"error\":\"...\"}\n\n"
    → ctx.Done() (client closed) → call unregisterFn → return
  → broker cleans up: removes from map, closes client channel
```

### Data flow: Frontend

```
App mounts
  → main.tsx: <BrowserRouter><App/></BrowserRouter>
  → App: <Routes> matches /dashboard → <DashboardView/>
  → DashboardView: useSSE("metrics")
    → useEffect: opens EventSource("/api/v1/stream/metrics")
    → onmessage: JSON.parse → useSSEStore.addMessage("metrics", data)
    → onerror: setConnectionStatus("disconnected"), schedule reconnect
  → DashboardView: useSSEStore(s => s.messages["metrics"]?.at(-1))
    → re-render with latest metrics
```

---

## 2. File structure (concrete)

### Backend (Go)

```
internal/
├── api/
│   ├── middleware/                     # NEW package
│   │   ├── cors.go                     # NEW: NewCORS(origins []string) gin.HandlerFunc
│   │   ├── cors_test.go                # NEW
│   │   ├── logging.go                  # NEW: RequestLogging() gin.HandlerFunc
│   │   ├── logging_test.go             # NEW
│   │   ├── request_id.go               # NEW: RequestID() gin.HandlerFunc
│   │   └── request_id_test.go          # NEW
│   ├── handlers/
│   │   ├── dashboard.go                # NEW: DashboardSummaryHandler, TopTalkersHandler, ProtocolDistributionHandler
│   │   ├── dashboard_test.go           # NEW
│   │   ├── packets.go                  # NEW: ListPacketsHandler, GetPacketHandler
│   │   ├── packets_test.go             # NEW
│   │   ├── devices.go                  # NEW: ListDevicesHandler, GetDeviceHandler
│   │   ├── devices_test.go             # NEW
│   │   ├── alerts.go                   # NEW: ListAlertsStub, AckAlertStub
│   │   ├── alerts_test.go              # NEW
│   │   ├── topology.go                 # NEW: GetTopologyStub
│   │   ├── topology_test.go            # NEW
│   │   ├── vulnerabilities.go          # NEW: ListVulnerabilitiesStub
│   │   ├── vulnerabilities_test.go     # NEW
│   │   ├── config.go                   # NEW: GetConfigStub
│   │   ├── config_test.go              # NEW
│   │   ├── sse.go                      # NEW: StreamMetricsHandler, 3 stub stream handlers
│   │   ├── sse_test.go                 # NEW
│   │   ├── health.go                   # UNCHANGED (Plan 2)
│   │   └── helpers.go                  # NEW: parseSinceParam, writeJSON, writeError helpers
│   ├── router.go                       # MODIFIED: add NewRouterWithDeps
│   └── router_test.go                  # MODIFIED: add tests for new constructor
├── storage/
│   ├── querier.go                      # NEW: Querier interface + ErrorQuerier + MockQuerier
│   ├── querier_test.go                 # NEW: MockQuerier, ErrorQuerier behavior tests
│   ├── queries.go                      # NEW: DBQuerier with 5 query methods
│   ├── queries_test.go                 # NEW: sqlmock expectations
│   ├── storage.go                      # MODIFIED: add Querier() method returning *DBQuerier
│   └── writer.go                       # UNCHANGED
├── sse/                                # NEW package
│   ├── broker.go                       # NEW: Broker, NewBroker, Run, Shutdown, Subscribe, Broadcast
│   ├── broker_test.go                  # NEW
│   ├── client.go                       # NEW: client struct, channel management
│   ├── client_test.go                  # NEW
│   └── event.go                        # NEW: Event type, Heartbeat event helper
├── shutdown/
│   └── shutdown.go                     # MODIFIED: add Broker interface to Params, call broker.Shutdown
├── config/
│   └── config.go                       # MODIFIED: add Server.CORSOrigins []string
└── (capture/, health/, etc. UNCHANGED)
cmd/viewred/
└── main.go                             # MODIFIED: wire broker, querier, ticker, NewRouterWithDeps
```

### Frontend (TypeScript/React)

```
web/
├── package.json                        # MODIFIED: add react-router-dom, zustand
├── vite.config.ts                      # UNCHANGED (proxy already in place)
├── tsconfig.json                       # UNCHANGED
├── src/
│   ├── main.tsx                        # MODIFIED: wrap App in BrowserRouter
│   ├── App.tsx                         # MODIFIED: state-based → Router-based
│   ├── App.test.tsx                    # NEW
│   ├── types.ts                        # MODIFIED: remove ViewType, add RoutePath
│   ├── components/
│   │   ├── DashboardView.tsx           # MODIFIED: real SSE data via useSSE
│   │   ├── DashboardView.test.tsx      # NEW
│   │   ├── CapturaView.tsx             # MODIFIED: real health polling
│   │   ├── CapturaView.test.tsx        # NEW
│   │   ├── Sidebar.tsx                 # MODIFIED: <button> → <Link to=...>
│   │   ├── Sidebar.test.tsx            # NEW
│   │   ├── Header.tsx                  # MODIFIED: setView → useNavigate
│   │   ├── NotFoundView.tsx            # NEW: 404 page
│   │   └── (FiltrosView, AlertasView, TopologiaView, VulnerabilidadesView,
│   │      AsistenteView, ConfiguracionView: UNCHANGED in Plan 3)
│   ├── lib/
│   │   ├── sse.ts                      # NEW: useSSE(topic) hook
│   │   ├── sse.test.ts                 # NEW
│   │   ├── api.ts                      # NEW: fetch wrapper with error handling
│   │   ├── api.test.ts                 # NEW
│   │   ├── llm.ts                      # UNCHANGED
│   │   └── llm.test.ts                 # UNCHANGED
│   └── store/
│       ├── sse-store.ts                # NEW: Zustand store
│       └── sse-store.test.ts           # NEW
```

**Files NOT touched in Plan 3** (deferred):
- `internal/bus/` — still not created. SSE metrics polls storage; no bus needed.
- `internal/scanner/`, `internal/ml/`, `internal/llm/` — Plan 5/4/6.
- `web/src/components/{FiltrosView,AlertasView,TopologiaView,VulnerabilidadesView,AsistenteView,ConfiguracionView}.tsx` — Plan 3 keeps them in router but they still show mock data (UI navigation works, data wiring comes in later plans).

---

## 3. Interface definitions

> **Convention**: type definitions and signatures only. No function bodies. JSON tags are part of the contract (frontend reads them).

### 3.1 Go: `storage.Querier`

```go
// File: internal/storage/querier.go
package storage

import (
    "context"
    "time"
)

// Querier is the read-side interface for the storage layer. API handlers
// depend on this interface (not on *Storage directly) so handler tests can
// use MockQuerier or ErrorQuerier without a real DB.
type Querier interface {
    QueryDashboardSummary(ctx context.Context, since time.Time) (DashboardSummary, error)
    QueryTopTalkers(ctx context.Context, since time.Time, limit int) ([]TopTalker, error)
    QueryProtocolDistribution(ctx context.Context, since time.Time) ([]ProtocolShare, error)
    QueryPackets(ctx context.Context, filter PacketFilter) ([]PacketEvent, int, error)
    QueryDevices(ctx context.Context, filter DeviceFilter) ([]Device, int, error)
}

// Return types (all exported, JSON-tagged for handler response shapes)

type DashboardSummary struct {
    PacketsPerSec     float64 `json:"packets_per_sec"`
    BytesPerSec       float64 `json:"bytes_per_sec"`
    ActiveConnections int     `json:"active_connections"`
    DeviceCount       int     `json:"device_count"`
}

type TopTalker struct {
    SrcIP   string `json:"src_ip"`
    Bytes   int64  `json:"bytes"`
    Packets int64  `json:"packets"`
}

type ProtocolShare struct {
    Protocol   string  `json:"protocol"`
    Percentage float64 `json:"percentage"`
    Bytes      int64  `json:"bytes"`
}

// PacketFilter holds the query parameters for QueryPackets.
type PacketFilter struct {
    Since    time.Time
    Until    time.Time
    Protocol string
    SrcIP    string
    Limit    int
    Offset   int
}

// DeviceFilter holds the query parameters for QueryDevices.
type DeviceFilter struct {
    Limit  int
    Offset int
    Status string
}

// Device represents a discovered network host.
type Device struct {
    ID         int64     `json:"id"`
    IP         string    `json:"ip"`
    MAC        string    `json:"mac"`
    Vendor     string    `json:"vendor"`
    Hostname   string    `json:"hostname"`
    FirstSeen  time.Time `json:"first_seen"`
    LastSeen   time.Time `json:"last_seen"`
    RiskScore  float64   `json:"risk_score"`
}

// ErrorQuerier returns a service-unavailable error for every method.
// Used by main.go when storage init failed (DB unreachable at startup).
type ErrorQuerier struct {
    Err error // typically: errors.New("storage unavailable")
}

func (q *ErrorQuerier) QueryDashboardSummary(ctx context.Context, since time.Time) (DashboardSummary, error)
func (q *ErrorQuerier) QueryTopTalkers(ctx context.Context, since time.Time, limit int) ([]TopTalker, error)
func (q *ErrorQuerier) QueryProtocolDistribution(ctx context.Context, since time.Time) ([]ProtocolShare, error)
func (q *ErrorQuerier) QueryPackets(ctx context.Context, filter PacketFilter) ([]PacketEvent, int, error)
func (q *ErrorQuerier) QueryDevices(ctx context.Context, filter DeviceFilter) ([]Device, int, error)

// MockQuerier has overridable function fields, one per Querier method.
// Defaults return zero/empty data with no error.
type MockQuerier struct {
    SummaryFn       func(ctx context.Context, since time.Time) (DashboardSummary, error)
    TopTalkersFn    func(ctx context.Context, since time.Time, limit int) ([]TopTalker, error)
    ProtocolDistFn  func(ctx context.Context, since time.Time) ([]ProtocolShare, error)
    PacketsFn       func(ctx context.Context, filter PacketFilter) ([]PacketEvent, int, error)
    DevicesFn       func(ctx context.Context, filter DeviceFilter) ([]Device, int, error)
}

// All 5 Query* methods, each with a default body that returns the zero value.
```

### 3.2 Go: `storage.queries` (DBQuerier)

```go
// File: internal/storage/queries.go
package storage

// DBQuerier is the real Postgres-backed implementation of Querier. It holds
// a *pgxpool.Pool and runs SQL. All five methods map 1:1 to spec §2 queries.
type DBQuerier struct {
    pool *pgxpool.Pool
}

func NewDBQuerier(pool *pgxpool.Pool) *DBQuerier
// + 5 Query* methods implementing Querier (signatures match the interface).

// Compile-time check: *DBQuerier satisfies Querier.
var _ Querier = (*DBQuerier)(nil)
```

> Implementation hint (not code): each query uses `pool.Query(ctx, sql, args...)`, scans rows into the typed struct, and returns `[]Type`, `nil` for empty results (not `nil` slice — handlers wrap with JSON serializer).

### 3.3 Go: `sse.Broker` and friends

```go
// File: internal/sse/event.go
package sse

// Event is a single SSE message. Data is already JSON-encoded.
type Event struct {
    Name string // "metrics" | "alert" | "topology" | "llm" | "heartbeat" | "error"
    Data []byte // JSON payload
}

// Heartbeat returns the canonical heartbeat event: `event: heartbeat\ndata: {}\n\n`.
func Heartbeat() Event
```

```go
// File: internal/sse/broker.go
package sse

import (
    "context"
    "sync"
    "time"
)

// BrokerOption configures NewBroker.
type BrokerOption func(*Broker)

func WithHeartbeat(d time.Duration) BrokerOption  // default 30s
func WithBufferSize(n int) BrokerOption           // default 64

// Broker manages SSE client subscriptions. Safe for concurrent use.
type Broker struct {
    mu        sync.RWMutex
    clients   map[string]*client   // client ID → client
    heartbeat time.Duration
    bufSize   int

    // Lifecycle signaling (internal)
    doneCh    chan struct{}
}

// NewBroker constructs a broker. The broker is not yet running; call Run
// in a goroutine to start the heartbeat / stale-removal loop.
func NewBroker(opts ...BrokerOption) *Broker

// Run starts the heartbeat goroutine. It exits when ctx is cancelled.
// Idempotent on multiple calls (subsequent calls are no-ops).
func (b *Broker) Run(ctx context.Context)

// Subscribe registers a new client subscribed to the given topics.
// Returns a receive-only channel of events and an unregister function
// that the caller MUST call on disconnect. The buffer is per-client
// (broker.bufSize); if the buffer is full, Broadcast drops the event
// for that client.
func (b *Broker) Subscribe(ctx context.Context, topics ...string) (<-chan Event, func())

// Broadcast sends an event to all subscribed clients whose topic set
// intersects with `topic`. Drops for full buffers are silent (logged
// at DEBUG). Returns nil unless ctx is cancelled.
func (b *Broker) Broadcast(ctx context.Context, topic string, event Event) error

// Shutdown closes all client channels with a graceful "server shutting
// down" event. Idempotent.
func (b *Broker) Shutdown(ctx context.Context) error

// ActiveClientCount returns the number of currently registered clients.
// Useful for tests and the /api/v1/health endpoint.
func (b *Broker) ActiveClientCount() int
```

```go
// File: internal/sse/client.go
package sse

// client is the internal subscription record. Not exported.
type client struct {
    id       string
    ch       chan Event
    topics   map[string]bool
    lastPing time.Time
}
```

### 3.4 Go: `api.RouterDeps`

```go
// File: internal/api/router.go (additions)
package api

import (
    "github.com/viewred/viewred/internal/config"
    "github.com/viewred/viewred/internal/health"
    "github.com/viewred/viewred/internal/sse"
    "github.com/viewred/viewred/internal/storage"
)

type RouterDeps struct {
    Health  *health.Registry
    Querier storage.Querier
    Broker  *sse.Broker
    Config  *config.Config
}

// NewRouterWithDeps builds a router with all real routes + middleware.
// If a dep is nil, the corresponding routes return 501 Not Implemented
// (graceful degradation for partial wiring).
func NewRouterWithDeps(deps RouterDeps) *gin.Engine
```

> The `Querier` field is an **interface** (`storage.Querier`), not a `*DBQuerier` concrete type. This is what makes `ErrorQuerier` and `MockQuerier` work transparently.

### 3.5 Go: handler signatures

```go
// File: internal/api/handlers/dashboard.go
package handlers

import (
    "github.com/viewred/viewred/internal/storage"
    "github.com/gin-gonic/gin"
)

func DashboardSummaryHandler(q storage.Querier) gin.HandlerFunc
func TopTalkersHandler(q storage.Querier) gin.HandlerFunc
func ProtocolDistributionHandler(q storage.Querier) gin.HandlerFunc

// File: internal/api/handlers/packets.go
func ListPacketsHandler(q storage.Querier) gin.HandlerFunc
func GetPacketHandler(q storage.Querier) gin.HandlerFunc

// File: internal/api/handlers/devices.go
func ListDevicesHandler(q storage.Querier) gin.HandlerFunc
func GetDeviceHandler(q storage.Querier) gin.HandlerFunc

// File: internal/api/handlers/stubs.go (one file, 5 stub handlers)
func NewStubHandler(responseBody any) gin.HandlerFunc
// Used by: ListAlertsStub, AckAlertStub, GetTopologyStub, ListVulnerabilitiesStub, GetConfigStub

// File: internal/api/handlers/sse.go
func StreamMetricsHandler(broker *sse.Broker, q storage.Querier) gin.HandlerFunc
func StreamStubHandler(broker *sse.Broker) gin.HandlerFunc  // for /stream/alerts, /stream/topology, /stream/llm
```

### 3.6 Go: middleware signatures

```go
// File: internal/api/middleware/cors.go
package middleware

// NewCORS builds a CORS middleware that allows the given origins.
// Empty origins means "same-origin only".
func NewCORS(origins []string) gin.HandlerFunc

// File: internal/api/middleware/logging.go
package middleware

// RequestLogging logs one structured line per request (slog INFO/WARN/ERROR).
// Must run AFTER RequestIDMiddleware so log lines carry the request_id.
func RequestLogging() gin.HandlerFunc

// File: internal/api/middleware/request_id.go
package middleware

// RequestID generates/propagates X-Request-Id. Stores it on the context
// under the key "request_id" and sets it on the response header.
func RequestID() gin.HandlerFunc

// ContextKey is exported for handlers that want to retrieve the ID.
const ContextKey = "request_id"
```

### 3.7 Go: shutdown extension

```go
// File: internal/shutdown/shutdown.go (additions)

// Broker is the subset of *sse.Broker that Shutdown needs.
type Broker interface {
    Shutdown(ctx context.Context) error
}

// Add Broker field to Params.
type Params struct {
    // ... existing fields ...
    Broker Broker
}

// Add Shutdown call in the order:
//   ... existing 1-6 ...
//   7. Server.Shutdown (drain in-flight HTTP requests).
//   8. Broker.Shutdown (notify SSE clients with "server shutting down").
//   9. Storage.Close (pool last).
```

### 3.8 Frontend: TypeScript types

```typescript
// File: web/src/lib/sse.ts (signature)
export type ConnectionStatus = 'connecting' | 'connected' | 'disconnected' | 'reconnecting';

export interface UseSSEResult<T = unknown> {
  lastEvent: T | null;
  connectionStatus: ConnectionStatus;
  error: string | null;
  reconnect: () => void;  // manual retry
}

export function useSSE<T = unknown>(topic: string): UseSSEResult<T>;
```

```typescript
// File: web/src/store/sse-store.ts
import { create } from 'zustand';

export type ConnectionStatus = 'connecting' | 'connected' | 'disconnected' | 'reconnecting';
export type CaptureStatus = 'running' | 'paused' | 'unknown';

export interface SSEStoreState {
  messages: Record<string, unknown[]>;
  connectionStatus: ConnectionStatus;
  lastError: string | null;
  captureStatus: CaptureStatus;

  addMessage: (topic: string, data: unknown) => void;
  setConnectionStatus: (status: ConnectionStatus) => void;
  setLastError: (err: string | null) => void;
  clearMessages: (topic?: string) => void;
  setCaptureStatus: (status: CaptureStatus) => void;
}

export const useSSEStore: import('zustand').UseBoundStore<import('zustand').StoreApi<SSEStoreState>>;
```

```typescript
// File: web/src/lib/api.ts (signature)
export interface ApiError {
  status: number;
  code: string;
  message: string;
}

export async function apiGet<T>(path: string, signal?: AbortSignal): Promise<T>;
export async function apiPost<T>(path: string, body?: unknown, signal?: AbortSignal): Promise<T>;
```

```typescript
// File: web/src/types.ts (additions)
export type RoutePath =
  | '/'
  | '/dashboard'
  | '/captura'
  | '/filtros'
  | '/alertas'
  | '/topologia'
  | '/vulnerabilidades'
  | '/asistente'
  | '/configuracion';

// ViewType is removed once App.tsx no longer references it.
```

```typescript
// File: web/src/components/NotFoundView.tsx
export default function NotFoundView(): JSX.Element;
// Renders: "404 — Page Not Found" with a <Link to="/dashboard">Volver al inicio</Link>
```

---

## 4. Key architectural decisions (justified)

| Decision | Choice | Alternatives | Rationale |
|---|---|---|---|
| **`Querier` interface** | `internal/storage/querier.go` | Concrete `*DBQuerier` only; sqlmock-only | Handlers depend on interface, not impl. Tests use `MockQuerier` (function-field mocks, easy to compose). DB-level sqlmock tests verify SQL separately. |
| **`ErrorQuerier`** | Returns 503 for every method | `nil` checks at every handler; panic-on-missing-dep | Centralizes the "DB down" path. `main.go` swaps one line. Handlers stay clean — same error path as real failures. **Aligned with user decision #4 (real errors, no mock fallback).** |
| **`MockQuerier` with function fields** | `SummaryFn func(...) (..., error)` per method | `testify/mock`; sqlmock for everything | Function fields are pure-Go, no dependency, easy to read. sqlmock still used in `queries_test.go` for real SQL. **Per Plan 2's tested pattern.** |
| **SSE ring buffer size: 64** | Per-client `chan Event` with cap 64 | Larger (256+); unbuffered | 64 × ~1 KB/event ≈ 64 KB/client. With typical 1 Hz metrics, that's ~1 min headroom. Drops the oldest event if client stalls; the heartbeat removes truly dead clients at 3× interval. |
| **SSE heartbeat: 30 s** | Periodic ping to all clients | 60 s; no heartbeat (detect only on write) | 30 s detects a half-dead TCP connection within ~90 s (3 missed pings). Cheap (1 line every 30 s per client). Spec §4. |
| **Metrics ticker in `main.go`**, not in broker | `time.NewTicker(1 * time.Second)` started in main | Broker owns its own ticker | Keeps broker passive (subscription/broadcast only). Easier to test ticker isolation. Easier to swap tickers in tests (`WithTicker` option on the handler). |
| **`RouterDeps` with all deps** | Single struct passed to `NewRouterWithDeps` | Per-handler constructor injection | Mirrors the existing `NewRouterWithRegistry` pattern. One constructor builds the full router. Nil-checked for back-compat with `NewRouter` and `NewRouterWithRegistry`. |
| **`NewRouterWithDeps` back-compat** | Old constructors stay; new one is additive | Break the old API | Zero risk to existing tests (Plan 2 router tests still pass). main.go switches at the end. |
| **React Router v7 `BrowserRouter`** | `BrowserRouter` over `HashRouter` | `HashRouter`; state-based | Vite dev proxy + Go static file serving already handle SPA fallback. HashRouter is a workaround for servers that don't. Plan 8 will `go:embed` + use a NoRoute handler. |
| **Zustand without persistence** | Plain `create(set => ...)` | Zustand `persist` middleware | SSE data is ephemeral. Persistence would re-show stale metrics on reload. Loading skeleton handles the brief gap. |
| **shadcn/ui deferred to Plan 8** | Keep custom components for Plan 3 | Install now | User decision #3. Avoids 1 task that needs user-interactive `shadcn init`; preserves focus on the API bridge. |
| **`internal/bus` NOT introduced** | SSE broker + channels sufficient | Generic pub/sub bus now | Plan 4 (ML aggregator) is when bus becomes necessary. YAGNI: don't introduce abstraction until two+ subsystems need it. |
| **SSE stub routes for `/alerts`, `/topology`, `/llm`** | Registered, no events emitted | Omit until Plan 4/5/6 | Frontend can `useSSE("alerts")` early without 404. Same pattern as REST stubs. |

---

## 5. Middleware ordering

**Order**: `gin.Recovery()` → `RequestID()` → `CORS(origins)` → `RequestLogging()` → routes.

| Position | Why |
|---|---|
| 1. **Recovery** | Catches panics anywhere downstream. Converts to 500 + logs. **First** so any later middleware panic also gets recovered. |
| 2. **RequestID** | Generates / propagates `X-Request-Id` and stores it on the context. **Second** so CORS + Logging can include it in their output. |
| 3. **CORS** | Sets `Access-Control-*` headers. Handles `OPTIONS` preflight (returns 204, short-circuits). **Third** so the response headers are set even for CORS-rejected requests that proceed to handlers. |
| 4. **RequestLogging** | Captures start time, defers log line with final status + duration. **Last** so it observes the final status (including 4xx/5xx produced by handlers). |

**Skipped**: `gin.Logger()` — replaced by our `RequestLogging` middleware (slog, structured, level-based).

---

## 6. Shutdown integration

**Updated order in `internal/shutdown/shutdown.go`** (additions bolded):

```
1. Capturer.Stop()         (stop capture — no new packets)
2. close(packets)          (signal writers to exit)
3. <-Writer.Done()         (wait for storage writer)
4. PcapWriter.Close()      (release file)
5. Server.Shutdown(ctx)    (drain in-flight HTTP)
6. Broker.Shutdown(ctx)    ← NEW: send "server shutting down" to SSE clients
7. Storage.Close()         (pool last)
```

**Why after server drain?**
- Step 5 stops accepting new HTTP requests, including new SSE connections.
- Step 6 then closes existing SSE channels with a graceful event — clients see the message and reconnect (or fail) on their own.
- Step 7 (storage close) is last so the broker can still query it during its own Shutdown (though for Plan 3, broker.Shutdown doesn't query).

**Adapter**: `main.go` adds `Broker: broker` to `shutdown.Params`.

---

## 7. main.go wire-up (diff)

```go
// File: cmd/viewred/main.go — diff from Plan 2

 import (
     ...
+    "github.com/viewred/viewred/internal/sse"
+    "github.com/viewred/viewred/internal/api/handlers"  // only if main.go references handler types directly
 )

 // After health registry (line ~162), ADD:
+// SSE broker — created first so the router can hold a reference.
+broker := sse.NewBroker()
+go broker.Run(ctx)
+defer broker.Shutdown(context.Background())  // panic-safety net; explicit call below

 // Replace querier construction (NEW):
+var querier storage.Querier
+if store != nil {
+    querier = storage.NewDBQuerier(store.Pool())
+} else {
+    log.Printf("storage not initialized, using ErrorQuerier (all queries will 503)")
+    querier = &storage.ErrorQuerier{Err: errors.New("storage unavailable")}
+}

 // Replace router construction (MODIFIED):
-router := api.NewRouterWithRegistry(rg)
+router := api.NewRouterWithDeps(api.RouterDeps{
+    Health:  rg,
+    Querier: querier,
+    Broker:  broker,
+    Config:  cfg,
+})

 // Add to shutdown.Params (MODIFIED):
 if err := shutdown.Shutdown(shutdown.Params{
     ...
+    Broker:     broker,
 }); err != nil {
     log.Printf("shutdown: %v", err)
 }
```

> The 1 Hz metrics ticker lives **inside** `StreamMetricsHandler`, not in `main.go`. Each SSE connection owns its own ticker (created on connect, stopped on disconnect). The 1 Hz spec requirement is per-stream, not per-process.

---

## 8. Frontend migration sequence

**6 ordered steps** (re-emphasized from spec §11). Each step is a separate task in `sdd-tasks` with its own verification + rollback.

| # | Step | Files | Verification | Rollback |
|---|------|-------|--------------|----------|
| 1 | **Add Zustand + SSE client library** (no routing change yet) | `package.json` (deps), `web/src/lib/sse.ts`, `web/src/store/sse-store.ts` | `pnpm test` passes; no view changes; existing app still works | `git revert <commit>`; remove `react-router-dom`/`zustand` from deps |
| 2 | **Add React Router** (wrap views in `<Routes>`, no content change) | `main.tsx` (wrap `<App>` in `<BrowserRouter>`), `App.tsx` (Routes/Route) | `pnpm test` passes; `pnpm dev`; clicking sidebar still shows the right view (now via URL); navigating to `/dashboard` shows DashboardView | Revert `App.tsx` and `main.tsx`; the deps stay but no code references them |
| 3 | **Wire `DashboardView` to real SSE data** | `DashboardView.tsx`, `DashboardView.test.tsx` | `pnpm test` passes; dashboard shows real KPIs updating (with backend running); error/loading states appear when backend down | Revert `DashboardView.tsx`; mock data returns |
| 4 | **Wire `CapturaView` to real health polling** | `CapturaView.tsx`, `CapturaView.test.tsx` | `pnpm test` passes; counters update from `/api/v1/health` every 5s | Revert `CapturaView.tsx`; mock timers return |
| 5 | **Add loading/error/empty states** | `DashboardView.tsx`, `CapturaView.tsx`, new `LoadingState`, `ErrorState`, `EmptyState` components | With backend off, dashboard shows clear "Conexión SSE perdida" message; with backend on but no traffic, "No hay tráfico" message | Revert state-handling blocks; keep data wiring |
| 6 | (skipped — shadcn/ui deferred to Plan 8) | — | — | — |

> **Why this order**: each step ends in a state where the app is **as functional as before** (or more). There's no step where the UI is partially broken. Steps 3-4 can run in parallel if different subagents (different files), but sequencing via a single orchestrator is safer.

---

## 9. Frontend data flow (detail)

### Initial mount

```
main.tsx renders <BrowserRouter><App/></BrowserRouter>
  → App renders <Routes>
  → URL "/dashboard" matches <Route path="/dashboard" element={<DashboardView/>} />
  → DashboardView mounts
  → useSSE("metrics") runs its useEffect
    → new EventSource("/api/v1/stream/metrics")
    → onopen → setConnectionStatus("connected")
    → onmessage → JSON.parse → useSSEStore.addMessage("metrics", data)
  → useSSEStore selector re-renders DashboardView with last event
```

### Disconnect + reconnect

```
Backend stops responding (or SSE handler errors)
  → EventSource.onerror fires
  → useSSE schedules reconnect with exponential backoff (1s, 2s, 4s, 8s, 16s, 30s cap)
  → Each retry: jitter ±500ms added to delay
  → setConnectionStatus("reconnecting")
  → After successful reconnection: setConnectionStatus("connected")
```

### Unmount cleanup

```
DashboardView unmounts (user navigates away)
  → useSSE useEffect cleanup
  → EventSource.close()
  → reconnect timer cleared
  → connectionStatus remains "connected" (other components may still use it)
```

---

## 10. Risk mitigations (concrete)

| Risk | Mitigation | Verification |
|---|---|---|
| **Scope creep** (one more endpoint) | Per-task review rejects anything not in §3 of the spec; "out of scope" rule from proposal §"Out of scope" | Each PR diff is checked against the 12 routes in the spec |
| **No real TimescaleDB in dev** | `go-sqlmock` for `queries_test.go`; `MockQuerier` for handler tests; `ErrorQuerier` for "DB down" tests | `go test ./...` passes; `sqlmock.NewExpectationsMet` succeeds |
| **SSE concurrency bugs** (goroutine leaks, deadlocks) | `Broker.Run` is one goroutine; per-client goroutine on `Subscribe` (in handler); `unregisterFn` always called in defer | `broker.ActiveClientCount()` test: subscribe 100, unsubscribe 50, expect 50. Run with `-race` flag. |
| **Slow client drops** | Ring buffer (cap 64) + select-default on send; test with a client that never reads | Test: subscribe, never read, broadcast 100 events, expect no panic, broker log shows 36 drops |
| **CORS misconfig in dev** | CORS test verifies `localhost:5173` allowed, `evil.com` rejected; Vite proxy already configured for `/api` | `cors_test.go` covers 3 scenarios; `vite.config.ts` already proxies `/api` → `:8080` |
| **Frontend change fatigue** | Strict 6-step migration with verification at each step | Each step ends with a runnable state; no half-migrated app |
| **SSE auth (no auth in V1)** | All endpoints open in dev; production is single-user, single-host. Note in `config.go` for V2 to add `AuthConfig`. | No mitigation in Plan 3; documented as deferred to V2 |
| **subagent smoke test timeouts on Windows** | Orchestrator runs smoke tests inline (port-based cleanup); subagents only run unit tests | Orchestrator verification step before "Plan 3 done" |

---

## 11. Task decomposition (preview, NOT the full tasks)

> The next phase (`sdd-tasks`) will detail each task with code-level steps. This is the high-level shape: 25 tasks across 4 layers.

### Backend (15 tasks)

| ID | Name | Depends on | Purpose |
|---|---|---|---|
| T1 | Middleware: CORS, logging, request ID | — | 3 middleware + tests |
| T2 | `storage.Querier` interface + `ErrorQuerier` + `MockQuerier` | — | Interface + test mocks |
| T3 | `QueryDashboardSummary` | T2 | One query method + sqlmock test |
| T4 | `QueryTopTalkers` | T2 | One query method + sqlmock test |
| T5 | `QueryProtocolDistribution` | T2 | One query method + sqlmock test |
| T6 | `QueryPackets` (paginated) | T2 | One query method + sqlmock test |
| T7 | `QueryDevices` | T2 | One query method + sqlmock test |
| T8 | Dashboard handlers (3) | T2, T3, T4, T5 | 3 handlers + MockQuerier tests |
| T9 | Packet handlers (2) | T2, T6 | 2 handlers + MockQuerier tests |
| T10 | Device handlers (2) | T2, T7 | 2 handlers + MockQuerier tests |
| T11 | Stub handlers (5) | router | One file, 5 stub handlers + tests |
| T12 | SSE broker (`broker.go` + `client.go` + `event.go`) | — | 3 files + tests |
| T13 | SSE handlers (`sse.go`: `/stream/metrics` live + 3 stubs) | T12, T3, T4 | 1 file + tests |
| T14 | Router refactor (`NewRouterWithDeps`) | T1, T8, T9, T10, T11, T13 | Modify `router.go` + tests |
| T15 | `main.go` wire-up | T14, T12 | Modify `main.go`; verify startup |

### Integration (1 task)

| ID | Name | Depends on | Purpose |
|---|---|---|---|
| T16 | Shutdown integration (broker.Shutdown in sequence) | T12, T15 | Modify `internal/shutdown/shutdown.go` + tests |

### Frontend (8 tasks)

| ID | Name | Depends on | Purpose |
|---|---|---|---|
| T17 | Frontend deps (`react-router-dom`, `zustand`) | — | Modify `package.json` + lockfile |
| T18 | Zustand store (`sse-store.ts`) | T17 | 1 file + tests |
| T19 | `useSSE` hook + `api.ts` helpers | T17, T18 | 2 files + tests |
| T20 | `App.tsx` refactor (state → Router) | T17 | 1 file + tests |
| T21 | `Sidebar.tsx` Link migration | T20 | 1 file + tests |
| T22 | `DashboardView` SSE wiring | T18, T19 | 1 file + tests |
| T23 | `CapturaView` health polling | — | 1 file + tests |
| T24 | Loading / error / empty states | T22 | Components + tests |

### Verification (1 task)

| ID | Name | Depends on | Purpose |
|---|---|---|---|
| T25 | e2e verification (orchestrator runs) | all | curl + browser smoke + go test ./... + pnpm test + pnpm build |

**Dependency graph summary**: T1, T2, T12, T17 have no deps and can start in parallel. T14 and T20 are the merge points (router + App). Frontend and backend can run largely in parallel after their respective foundations. T25 is the final orchestrator check.

---

## 12. TDD integration (Iron Law)

> **NO PRODUCTION CODE WITHOUT A FAILING TEST FIRST.**
> Every implementer must show the RED output in their return. No exceptions.

### Per-component test patterns

| Component | Test pattern | Example test name |
|---|---|---|
| **Middleware** | `httptest.NewRequest` + `httptest.NewRecorder` + Gin test mode | `TestCORS_AllowsDevOrigin_SetsHeader` |
| **Querier interface** | Plain `MockQuerier` with function fields; assert return | `TestMockQuerier_DefaultSummary_ReturnsZeroValue` |
| **`ErrorQuerier`** | Assert all methods return `(zero, err)`; test multiple methods in one test for compactness | `TestErrorQuerier_AllMethods_ReturnError` |
| **Query methods (DBQuerier)** | `sqlmock.New()` + `pgx/v5/stdlib` bridge; expect SQL string + return rows | `TestQueryDashboardSummary_PacketsPerSec_ScansFloat` |
| **Handlers** | `MockQuerier` + `httptest` + Gin test mode; assert status, body shape | `TestDashboardSummaryHandler_ValidRequest_ReturnsKPIs` |
| **Handler error path** | `MockQuerier` returning an error; assert 503 | `TestDashboardSummaryHandler_QueryFails_Returns503` |
| **Stub handlers** | Plain `httptest`; assert 200 + body | `TestListAlertsStub_ReturnsEmptyArray` |
| **SSE broker lifecycle** | Channel-based; subscribe, broadcast, unregister; assert events received | `TestBroker_BroadcastsToSubscribedClient` |
| **SSE broker backpressure** | Subscribe, fill buffer, broadcast past cap; assert no panic, drops logged | `TestBroker_SlowClient_OldestDropped` |
| **SSE broker disconnect** | Subscribe, call unregister, broadcast; assert no delivery | `TestBroker_AfterUnregister_NoDelivery` |
| **SSE broker stale removal** | Subscribe with stopped client, wait 3× heartbeat; assert removed | `TestBroker_StaleClient_Removed` |
| **SSE handler** | Real broker + MockQuerier; `httptest`; parse `event:` and `data:` lines | `TestStreamMetricsHandler_EmitsInitialEvent` |
| **Frontend `useSSE` hook** | Mock `EventSource`; assert state updates | `useSSE.test.ts: receives event → store updated` |
| **Frontend component** | `render` with Zustand store provider; assert DOM | `DashboardView.test.tsx: shows KPI from store` |
| **Frontend router** | `MemoryRouter` in test; assert view matches URL | `App.test.tsx: /dashboard renders DashboardView` |
| **Shutdown** | Table test with mock subsystems + broker; assert order via spy | `TestShutdown_Order_BrokerAfterServer` |

### Process for every task

1. **RED**: write the failing test. Show the failure output in the implementer's return.
2. **GREEN**: minimum production code to pass. Show the passing output.
3. **REFACTOR**: clean up. All tests must still pass.
4. **Commit**: one commit per task. Conventional commit message.

### What implementers must include in their return

- File paths created/modified
- The RED output (before)
- The GREEN output (after)
- Any decisions made (with rationale)
- Any blockers or open questions

---

## Open questions

None. All 5 user decisions are locked in the proposal; spec is finalized; existing code patterns are clear from Plan 2.

---

## Checklist for sdd-tasks phase

- [ ] Each of the 25 tasks has a TDD plan with the failing test written first
- [ ] Each task is a single conventional commit
- [ ] No task depends on a future task in the same plan
- [ ] T25 (e2e verification) runs all 12 spec scenarios
- [ ] Back-compat: `NewRouter()` and `NewRouterWithRegistry()` still work without changes to call sites (Plan 2 tests still pass)
- [ ] No shadcn/ui dependency added in `package.json` (deferred to Plan 8)
- [ ] No `internal/bus/` package created (deferred to Plan 4)

---

## Next step

`/sdd-tasks` — turn this design into 25 atomic tasks, each with its failing test, implementation steps, and commit message.
