# Exploration: Core API and UI Shell (Plan 3 of 8)

> **Date**: 2026-06-18
> **Status**: Draft (exploration phase)
> **Plan name**: `core-api-and-ui-shell`
> **Base commit**: `296f904` (Plan 2 merged to main)
> **Design spec**: `docs/specs/v1-design.md`

---

## Executive Summary

Plan 3 (`core-api-and-ui-shell`) builds the **bridge between capture/storage and the UI**. After Plans 1-2, the backend can capture packets, write pcap files, and store metadata in TimescaleDB — but none of this data is exposed via API. The frontend has a full visual shell (sidebar, header, all 8 view components) but every piece of data is mocked with random timers and fakes.

Plan 3 adds the HTTP API layer (REST endpoints + SSE streams) and wires the UI to consume real data. After this plan, the dashboard shows live metrics, the packet list queries real stored data, and the architecture supports adding new endpoints in subsequent plans without structural changes.

**The scope must be carefully bounded**: building ALL 19 REST endpoints and 4 SSE streams from the spec in one plan would be ~30+ tasks and overwhelm the plan. This exploration identifies the **minimal viable API surface** that the UI needs to be demonstrably live, defers the rest to their respective plans, and identifies what's needed on the frontend to consume real data.

---

## Current State (After Plans 1-2)

### Backend — what exists

| Package | Status | Key contents |
|---------|--------|-------------|
| `cmd/viewred/main.go` | Complete | Lifeycle: config → storage → capture → pcap writer → storage writer → health → API server → graceful shutdown |
| `internal/api/` | Minimal | Gin router with `NewRouterWithRegistry(health.Registry)`, single route `GET /api/v1/health`, static file serving for `web/dist/` |
| `internal/api/handlers/` | Minimal | Only `health.go` — handler that returns aggregated health report. |
| `internal/capture/` | Complete | `PacketCapture` interface, `NpcapCapture` (Win), `AFPacketCapture` (Linux), `MockCapture`, `PcapWriter` |
| `internal/storage/` | Storage layer only | Pool management, migration runner, batched writer. **No query methods yet.** |
| `internal/health/` | Complete | `Registry`, `Subsystem`, `Report`, `Status` |
| `internal/config/` | Complete | YAML loader with defaults for server, AI, scanner, database |
| `internal/shutdown/` | Complete | Ordered teardown: capture → flush writer → close pcap → drain server → close storage |
| `internal/bus/` | **Does not exist** | Deferred from Plan 2. Not needed for Plan 3 in full form — SSE broadcaster can be standalone. |
| `internal/sse/` | **Does not exist** | New package needed for SSE broadcaster. |

### Frontend — what exists

| Area | Status | Details |
|------|--------|---------|
| Vite project | Ready | React 19, TypeScript 5.8+, Tailwind 4, Vitest, proxy to `:8080` |
| Layout shell | Built | `App.tsx` (state-based routing), `Sidebar.tsx` (collapsible, 8 nav items, system telemetry), `Header.tsx` (search, capture indicator, AI button, config, user avatar) |
| View components | All stubs exist | `DashboardView`, `CapturaView`, `FiltrosView`, `AlertasView`, `TopologiaView`, `VulnerabilidadesView`, `AsistenteView`, `ConfiguracionView` — all use mock data |
| Mock data | `web/src/data/mockData.ts` | Full mock dataset for all views (devices, alerts, vulnerabilities, traffic history, top talkers, protocol distribution) |
| Types | `web/src/types.ts` | `ViewType` (8 values), `KPI`, `NetworkAlert`, `NetworkDevice`, `Vulnerability`, `CapturedPacket`, `ChatMessage`, `ScanHistory` |
| LLM client | Placeholder | `web/src/lib/llm.ts` — fake delay + canned responses |
| State management | Local `useState` | No React Query, no Zustand, no Context Provider |
| Routing | State-based | `currentView: ViewType` + conditional rendering in `App.tsx` — no React Router, no URL-based routing |
| UI library | Custom components | **No shadcn/ui** (despite design spec recommendation). No `sonner`, no `react-hook-form`, no `zod`. |
| SSE | **Not connected** | Dashboard uses `setInterval` to fake real-time updates. No `EventSource` anywhere. |
| Design tokens | Partially applied | Dark theme (`#0a0e14`), Inter + JetBrains Mono fonts, custom scrollbars. Not fully matching the design spec color tokens. |

### What Plan 2 explicitly deferred (per its self-review)

> "Plan 2 does not include: real REST endpoints for packet list (plan 3), SSE broadcaster (plan 3), top talkers / aggregations (plan 4)"

---

## Detailed Scope Analysis

### What Plan 3 SHOULD Cover

The principle: Plan 3 delivers the **skeleton that all subsequent plans extend**. Every endpoint added in Plans 4-8 should follow a pattern established in Plan 3.

#### 1. API Middleware (new: `internal/api/middleware/`)

| Middleware | Priority | Notes |
|-----------|----------|-------|
| CORS | **Required** | Needed for Vite dev proxy to work. Gin has a built-in `cors` package or a simple custom middleware. |
| Request logging | **Required** | Structured logging per request (method, path, status, duration, request_id). Using `slog` + Gin middleware. |
| Request ID | **Nice** | X-Request-Id header propagation. Low effort, high debug value. |
| Recovery | Already exists | `gin.Recovery()` is already in the router. |

#### 2. Storage Query Layer (add to `internal/storage/`)

The storage package has `New`, `Ping`, `Close`, `RunMigrations`, `NewWriter`/`Run` — but **no query methods**. Plan 3 needs:

| Query | Purpose | SQL complexity |
|-------|---------|----------------|
| `QueryPackets(ctx, filter PacketFilter) ([]PacketEvent, error)` | Paginated packet list with time range, protocol, IP filters | Medium — multiple WHERE clauses + LIMIT/OFFSET |
| `QueryDashboardSummary(ctx, since time.Time) (DashboardSummary, error)` | KPIs: packets/s, bytes/s, active connections, device count | Medium — GROUP BY + COUNT DISTINCT |
| `QueryTopTalkers(ctx, since time.Time, limit int) ([]TopTalker, error)` | Top N IPs by bytes | Low — GROUP BY src_ip ORDER BY SUM(bytes) DESC |
| `QueryProtocolDistribution(ctx, since time.Time) ([]ProtocolShare, error)` | Protocol percentages | Low — GROUP BY protocol |
| `QueryFlows(ctx, filter FlowFilter) ([]Flow, error)` | 5-tuple flow aggregations | Medium — uses flows hypertable |

**Each query method must have test** following Plan 2's TDD pattern. Since we don't have a real DB in the dev environment, queries should use `go-sqlmock` for unit tests (with plan to integration-test in CI).

#### 3. REST Endpoints (add to `internal/api/handlers/`)

**Tier 1 — MVP endpoints (core to making the UI demoable):**

| Method | Path | Handler | Data source | Priority |
|--------|------|---------|-------------|----------|
| `GET` | `/api/v1/dashboard/summary` | `DashboardSummary` | Storage query or mock if DB unavailable | **Required** |
| `GET` | `/api/v1/dashboard/top-talkers` | `TopTalkers` | Storage query or mock | **Required** |
| `GET` | `/api/v1/dashboard/protocols` | `ProtocolDistribution` | Storage query or mock | **Required** |
| `GET` | `/api/v1/packets` | `ListPackets` | Storage query with pagination | **Required** |
| `GET` | `/api/v1/packets/{id}` | `GetPacket` | Storage query by ID | **Nice** |
| `GET` | `/api/v1/devices` | `ListDevices` | Storage query | **Required** (for topology) |
| `GET` | `/api/v1/devices/{id}` | `GetDevice` | Storage query | **Nice** |

**Tier 2 — Stub endpoints (register route, return placeholder/empty data):**

| Method | Path | Reason to stub |
|--------|------|----------------|
| `GET` | `/api/v1/alerts` | UI needs the route to exist for async loading (can show "no data yet" state) |
| `POST` | `/api/v1/alerts/{id}/ack` | Prevents 404s when the UI tries to ack (Plan 4 makes it real) |
| `GET` | `/api/v1/topology` | UI needs the route (Plan 5 makes it real) |
| `GET` | `/api/v1/vulnerabilities` | UI needs the route (Plan 7 makes it real) |
| `GET` | `/api/v1/config` | UI needs it for the config view (Plan 8 can add PATCH) |

**Tier 3 — Defer entirely:**

| Method | Path | Deferred to |
|--------|------|-------------|
| `POST` | `/api/v1/filters` | Plan 3+ or Plan 4 |
| `GET` | `/api/v1/filters` | Plan 3+ or Plan 4 |
| `POST` | `/api/v1/scan` | Plan 7 |
| `GET` | `/api/v1/scans` | Plan 7 |
| `POST` | `/api/v1/llm/chat` | Plan 6 |
| `GET` | `/api/v1/llm/conversations` | Plan 6 |
| `PATCH` | `/api/v1/config` | Plan 8 |

#### 4. SSE Infrastructure (new: `internal/sse/`)

| Component | Description | Priority |
|-----------|-------------|----------|
| `Broker` struct | Manages connected SSE clients (register, unregister, broadcast) | **Required** |
| Client heartbeat | Ping every 30s to detect dead connections | **Required** |
| Ring buffer per client | Drop oldest events if client is slow (per spec §7.5) | **Required** |

**SSE stream spec (§6.4):**

| Path | Emits | Frequency | Plan |
|------|-------|-----------|------|
| `GET /api/v1/stream/metrics` | KPIs + top talkers | 1 Hz | **Plan 3 — Required** |
| `GET /api/v1/stream/alerts` | New alerts | Push on event | Plan 4 |
| `GET /api/v1/stream/topology` | Device/edge changes | Push on event | Plan 5 |
| `GET /api/v1/stream/llm` | LLM tokens | Per-token | Plan 6 |

For Plan 3, only the `/stream/metrics` endpoint is built and populated. The others are registered as empty stubs.

The metrics stream should:
- Tick every 1 second
- Query the dashboard summary + top talkers from storage
- If DB is unavailable, return mock/generated data to keep the UI alive during development
- Emit as SSE `event: metrics\ndata: {...}\n\n`

#### 5. UI Changes

| Change | Description | Effort |
|--------|-------------|--------|
| Add React Router v7 | `react-router-dom`, define routes for each view | Low-Medium |
| Add Zustand | Lightweight store for SSE data, auth status, capture status | Low |
| Add SSE client | `EventSource` wrapper with auto-reconnect + backoff | Low |
| Wire Dashboard to real data | Replace `setInterval` fakes with SSE subscription | Medium |
| Wire Packet list to API | Replace mock data with `fetch` from `/api/v1/packets` | Medium |
| Add shadcn/ui (optional) | `npx shadcn@latest init`, then use for common components | Medium |
| Loading/error states | Replace "always works" mock data with real states | Medium |
| Captcha toggle wiring | Connect capture status button to API | Low |

**Decision needed**: React Router vs keep state-based routing. Keeping state-based is simpler and avoids a dependency. The design spec doesn't mandate React Router. However, URL-based routing is important for:
- Deep-linking to specific views (e.g., topología#device-42)
- Browser back/forward navigation
- Future features like sharing alert links

My recommendation: **add React Router**, but keep the migration small — wrap the existing views in `<Route>` components without changing their internals. The path structure would be `/dashboard`, `/captura`, `/filtros`, `/alertas`, `/topologia`, `/vulnerabilidades`, `/asistente`, `/configuracion`.

#### 6. Integration with Plans 1-2

| Integration point | What exists | What Plan 3 builds |
|-------------------|-------------|-------------------|
| `internal/api/router.go` | `GET /api/v1/health` only | Adds all Tier 1-2 routes, middleware, SSE endpoint |
| `internal/api/handlers/` | `health.go` only | Adds `dashboard.go`, `packets.go`, `devices.go`, stub handlers, SSE handler |
| `internal/storage/` | No query methods | Adds `queries.go` with `QueryPackets`, `QueryDashboardSummary`, `QueryTopTalkers`, `QueryProtocolDistribution`, `QueryDevices` |
| `internal/config/` | Config struct | May need `CaptureConfig.Interface` added if not already present |
| `cmd/viewred/main.go` | Runs router + subsystems | Injects SSE broker into router, wires metrics ticker |
| `web/src/App.tsx` | State-based routing | Replaces with React Router, adds SSE connection |
| `web/src/components/DashboardView.tsx` | Fake timers + mock data | Subscribes to SSE stream for real metrics |
| `web/vite.config.ts` | Proxy to `:8080` | No changes needed |

### What Plan 3 Explicitly DEFERS

| Feature | Rationale | Plan |
|---------|-----------|------|
| Real ML alert engine | Needs ONNX models + aggregator | Plan 4 |
| Alert endpoints with real data | Depends on alert engine | Plan 4 |
| Topology endpoints | Needs device discovery (ARP, mDNS) | Plan 5 |
| Device discovery logic | New subsystem | Plan 5 |
| LLM chat integration | Needs real Ollama client | Plan 6 |
| Scan endpoints | Needs nmap integration | Plan 7 |
| Vulnerability endpoints | Depends on scan | Plan 7 |
| Config write (PATCH) | Low priority, can wait | Plan 8 |
| Internal event bus (`internal/bus/`) | SSE broker + channels are sufficient for now; bus is needed when multiple subsystems need pub/sub | Plan 4+ |
| Performance benchmarks | Lacks real data to benchmark against | Plan 4+ |
| E2E tests (Playwright) | Need a real DB + capture to be meaningful | Plan 8 |

### The `internal/bus` Decision

The design spec §5.3 shows an `internal/bus` (event pipeline) between capture and all consumers. The spec diagram lines suggest capture → bus → [DB writer, aggregator, ML, SSE].

Plan 2 deferred the bus, using direct channels instead (capture → packets channel → [pcap writer, storage writer]).

**Question: Should Plan 3 introduce `internal/bus`?**

Arguments for "yes, now":
- Cleaner architecture from the start
- Adding it later means refactoring all existing consumers
- The SSE broadcaster could be a bus subscriber

Arguments for "defer":
- Adding the bus is a refactoring task with no direct user-visible benefit
- The SSE broadcaster can read from its own ticker without needing the bus
- Plan 4 (ML aggregator) actually NEEDS the bus — that's the natural time to introduce it
- YAGNI — we don't yet know what shape the bus needs to be

**Recommendation**: Defer the bus to Plan 4 (ML & alerts). For Plan 3, the SSE metrics stream reads from storage (polling) rather than from a live packet channel. This is simpler and sufficient for V1.

---

## Key Technical Decisions for the User

| Decision | Options | Recommendation |
|----------|---------|----------------|
| **API scope**: How many endpoints in Plan 3? | (a) All 19 for completeness, (b) Minimal viable set (dashboard + packets + devices + stubs), (c) Only dashboard | **(b)** — build dashboard + packets + devices + stub the rest |
| **Storage queries**: Use `go-sqlmock` or struct mocks? | (a) sqlmock for all, (b) Use interface-based mocks with `storage.Querier` interface | **(b)** — define a `Querier` interface so handlers can be tested without sqlmock; interface allows both real and mock implementations |
| **SSE broadcaster**: Poll storage or receive from channel? | (a) Poll storage every 1s, (b) Receive live from capture channel via bus | **(a)** for Plan 3 metrics; bus-based push for Plan 4 alerts |
| **UI routing**: React Router or state-based? | (a) Add React Router, (b) Keep state-based | **(a)** — URL-based routing is a V1 requirement for usability |
| **UI state management**: Zustand or keep local? | (a) Zustand, (b) React Query, (c) Keep `useState` | **(a)** — Zustand is lightweight, sufficient for SSE data store, recommended by design spec |
| **UI library**: shadcn/ui or custom? | (a) Install shadcn/ui now, (b) Keep custom, (c) Partial (just a few components) | **(a)** — shadcn/ui provides the table, toast, progress, modal, badge, card components the design spec calls for |
| **Bus**: Introduce `internal/bus` now or defer? | (a) Now, (b) Defer to Plan 4 | **(b)** — defer to Plan 4 when the ML aggregator needs it |
| **Fallback data**: When DB is unavailable, return errors or mock data? | (a) Return errors (the real dev experience), (b) Return generated data for demo mode | **(b)** — a "demo mode" with generated data keeps the UI developable without TimescaleDB |
| **TDD strictness**: Apply Iron Law for all new code? | (a) Yes, per design spec §10.1, (b) Relax for handlers | **(a)** — the user explicitly confirmed TDD discipline. NO PRODUCTION CODE WITHOUT A FAILING TEST FIRST. |

---

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| **Scope creep**: Building all REST endpoints in one plan | 25+ tasks, plan takes too long, review fatigue | Strict scope boundaries documented above. Review gate: "does this endpoint depend on a feature NOT yet built?" If yes, stub it. |
| **No real DB in dev**: Query methods can't be integration-tested | Queries may have SQL syntax errors | Unit test with `go-sqlmock`. SQL can be verified when CI has TimescaleDB. |
| **SSE complexity**: Managing many concurrent clients | Goroutine leaks, connection handling bugs | `internal/sse/` package with clean lifecycle, tested with `httptest` + `context` cancellation |
| **Frontend change fatigue**: Changing routing, state, AND components at once | Too many simultaneous changes, bug-prone | Sequence: (1) Add Zustand + SSE client (no routing change), (2) Add React Router (no content change), (3) Wire views to real data |
| **TDD + mocking for SSE**: Testing real-time streams | Tests are fragile or non-deterministic | Use channel-based SSE broker that can be injected with fake data in tests. Test with timeouts and `httptest.Server`. |

---

## Integration Points with Plans 1-2

### How the router changes

Current `router.go`:
```go
r := gin.New()
r.Use(gin.Recovery())
v1 := r.Group("/api/v1")
v1.GET("/health", handlers.Health(rg))
// + static file serving
```

Planned `router.go`:
```go
r := gin.New()
r.Use(gin.Recovery())
r.Use(middleware.RequestLogging())
r.Use(middleware.CORS(cfg))

v1 := r.Group("/api/v1")
v1.GET("/health", handlers.Health(rg))
v1.GET("/dashboard/summary", handlers.DashboardSummary(q))
v1.GET("/dashboard/top-talkers", handlers.TopTalkers(q))
v1.GET("/dashboard/protocols", handlers.ProtocolDistribution(q))
v1.GET("/packets", handlers.ListPackets(q))
v1.GET("/packets/:id", handlers.GetPacket(q))
v1.GET("/devices", handlers.ListDevices(q))
v1.GET("/devices/:id", handlers.GetDevice(q))
v1.GET("/alerts", handlers.ListAlerts(q))       // stub
v1.POST("/alerts/:id/ack", handlers.AckAlert(q)) // stub
v1.GET("/topology", handlers.GetTopology(q))      // stub
v1.GET("/vulnerabilities", handlers.ListVulnerabilities(q)) // stub
v1.GET("/config", handlers.GetConfig(cfg))        // stub

v1.GET("/stream/metrics", handlers.StreamMetrics(broker))
v1.GET("/stream/alerts", handlers.StreamAlerts(broker))
v1.GET("/stream/topology", handlers.StreamTopology(broker))

// + static file serving
```

### How `main.go` changes

Current: creates router with health registry.
Planned: also creates SSE broker, starts metrics ticker goroutine, passes querier + broker + config to router.

### How the frontend changes

Current `App.tsx`: state-based routing, no data fetching.
Planned: React Router, global Zustand store for SSE data, `useSSE` hook in DashboardView.

### What `NewRouter` signature becomes

```go
// NewRouterWithDeps builds a router with all dependencies injected.
func NewRouterWithDeps(deps RouterDeps) *gin.Engine
```

Where `RouterDeps` includes:
```go
type RouterDeps struct {
    Health  *health.Registry
    Querier storage.Querier
    Broker  *sse.Broker
    Config  *config.Config
}
```

This follows the established `NewRouterWithRegistry` pattern.

---

## TDD Requirements for Plan 3

Per the design spec (§10.1) and the user's explicit instruction:

### The Iron Law: NO PRODUCTION CODE WITHOUT A FAILING TEST FIRST

1. **Every Go handler**: Write a failing test using `httptest` + gin test mode → handler must be testable without a real DB (use `storage.Querier` interface mock)
2. **Every query method**: Write a failing test using `go-sqlmock` → test returns expected rows, handles empty results, handles errors
3. **SSE broker**: Write tests for client registration, unregistration, broadcast → use `httptest.ResponseRecorder` with `context` cancellation
4. **Frontend changes**: Write component tests with Vitest + Testing Library → test that DashboardView renders KPIs from SSE data, test that SSE client reconnects on disconnect
5. **Mock only when unavoidable**: SSE stream tests may need mocked time (ticker); use `time.NewTicker` abstraction for testability
6. **Regression tests**: Any bug found during development gets a test that reproduces it BEFORE the fix

### Test patterns

```go
// Handler test (storage mocked)
func TestDashboardSummary_ReturnsKPIs(t *testing.T) {
    t.Parallel()
    mockQuerier := &MockQuerier{
        Summary: storage.DashboardSummary{
            PacketsPerSec: 1234.5,
            BytesPerSec:   4.2e6,
            ActiveConnections: 234,
            DeviceCount:    47,
        }
    }
    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.GET("/api/v1/dashboard/summary", handlers.DashboardSummary(mockQuerier))
    
    req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/summary", nil)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    
    require.Equal(t, http.StatusOK, w.Code)
    var body map[string]any
    json.Unmarshal(w.Body.Bytes(), &body)
    assert.Equal(t, 1234.5, body["packets_per_sec"])
}
```

```go
// SSE broker test
func TestBroker_BroadcastsToSubscribers(t *testing.T) {
    b := sse.NewBroker()
    w := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodGet, "/stream/metrics", nil)
    
    ctx, cancel := context.WithCancel(req.Context())
    defer cancel()
    req = req.WithContext(ctx)
    
    go b.ServeHTTP(w, req)
    
    // Wait for client to register
    time.Sleep(50 * time.Millisecond)
    b.Broadcast("metrics", []byte(`{"packets_per_sec": 1234}`))
    time.Sleep(50 * time.Millisecond)
    
    cancel()
    assert.Contains(t, w.Body.String(), "packets_per_sec")
}
```

---

## Effort Breakdown

### Backend tasks

| Task | Files | Est. effort |
|------|-------|-------------|
| **Task 1**: Add middleware (CORS, logging, request ID) | `internal/api/middleware/cors.go`, `internal/api/middleware/logging.go` + tests | Medium |
| **Task 2**: Define `storage.Querier` interface | `internal/storage/querier.go` + test | Low |
| **Task 3**: Implement `QueryDashboardSummary` | `internal/storage/queries.go` + test | Medium |
| **Task 4**: Implement `QueryTopTalkers` | `internal/storage/queries.go` + test | Medium |
| **Task 5**: Implement `QueryProtocolDistribution` | `internal/storage/queries.go` + test | Low |
| **Task 6**: Implement `QueryPackets` (paginated) | `internal/storage/queries.go` + test | Medium |
| **Task 7**: Implement `QueryDevices` | `internal/storage/queries.go` + test | Low |
| **Task 8**: Dashboard handlers (summary, top-talkers, protocols) | `internal/api/handlers/dashboard.go` + test | Medium |
| **Task 9**: Packet handlers (list, detail) | `internal/api/handlers/packets.go` + test | Medium |
| **Task 10**: Device handlers (list, detail) | `internal/api/handlers/devices.go` + test | Low |
| **Task 11**: Stub handlers (alerts, topology, vulns, config) | `internal/api/handlers/alerts.go`, `topology.go`, `vulnerabilities.go`, `config.go` + tests | Low |
| **Task 12**: SSE broker package | `internal/sse/broker.go` + test | Medium |
| **Task 13**: SSE metrics handler + ticker | `internal/api/handlers/sse.go` + test | Medium |
| **Task 14**: Wire everything into router + main.go | `internal/api/router.go`, `cmd/viewred/main.go` | Medium |

### Frontend tasks

| Task | Files | Est. effort |
|------|-------|-------------|
| **Task 15**: Add Zustand + SSE client library | `web/package.json`, `web/src/lib/sse.ts`, `web/src/store/sse-store.ts` + tests | Medium |
| **Task 16**: Add React Router + define routes | `web/package.json`, `web/src/App.tsx` (refactor) + tests | Medium |
| **Task 17**: Wire DashboardView to real SSE data | `web/src/components/DashboardView.tsx` (rewrite data layer) + tests | Medium |
| **Task 18**: Wire CapturaView to real capture status | `web/src/components/CapturaView.tsx` + tests | Low |
| **Task 19**: Add shadcn/ui (optional) | `npx shadcn@latest init`, add Card, Badge, Table, Progress, Toast | Low |
| **Task 20**: Loading/error/empty states across views | All views | Low |
| **Task 21**: Final integration: backend + frontend end-to-end | None (verification only) | Low |

### Total estimate: **15-21 tasks** across **8-10 work units**
- Backend: ~10 tasks (L-M complexity)
- Frontend: ~5-7 tasks (L-M complexity)
- Integration: ~1 task
- **Complexity**: Medium (many moving parts, but each individually is straightforward)
- **Risk areas**: SSE testing (time-dependent), CORS in dev mode, DB query correctness without real DB

---

## Recommendation

**Proceed to `sdd-propose`** with the scope defined above.

The plan name `core-api-and-ui-shell` is appropriate. The scope should be strictly bounded to:
1. Middleware (CORS, logging)
2. Storage query interface + 5 query implementations (dashboard, top talkers, protocols, packets, devices)
3. REST endpoints for dashboard, packets, devices + stubs for alerts, topology, vulns, config
4. SSE broker + metrics stream
5. UI routing (React Router), state (Zustand), SSE client
6. Wire DashboardView to real SSE data
7. Wire packet list to API
8. Loading/error/empty states

The `internal/bus` should NOT be introduced in Plan 3 — defer to Plan 4 when the ML aggregator needs it.

The plan should follow the TDD Iron Law strictly. Each query method, handler, and SSE component must have a failing test before production code.

**One change to consider**: rename the plan to `core-api-sse-and-ui-wiring` to more accurately reflect that this is NOT building all endpoints, but rather the core bridge and the wiring pattern that subsequent plans extend. The current name `core-api-and-ui-shell` is acceptable.

---

## Open Questions for the User

1. **API scope confirmation**: Dashboard + packets + devices + stubs for alerts/topology/vulns/config. Is this the right scope, or do you want more (or fewer) endpoints in this plan?

2. **React Router vs state-based**: The design spec doesn't explicitly require React Router. Do you want URL-based routing, or keep the simpler state-based approach for V1?

3. **shadcn/ui**: The design spec recommends it, but it's not installed. Do you want to add it in Plan 3, or keep pure custom components for V1?

4. **Fallback mode**: When the DB is unavailable, should the API return errors (forcing the UI to show error states) or return generated/mock data so the UI is always demoable?

5. **SSE only for metrics**: Plan 3 builds only `/stream/metrics`. Alerts, topology, and LLM streams are registered as routes but won't emit events until their respective plans. OK?

6. **TDD with go-sqlmock**: The query methods will use `go-sqlmock` (same pattern as Plan 2). Real DB integration testing will happen when CI has a TimescaleDB. Acceptable?

7. **Plan rename**: `core-api-sse-and-ui-wiring` instead of `core-api-and-ui-shell` — more accurate. Rename or keep?
