# Proposal: Core API and UI Shell (Plan 3 of 8)

> **Date**: 2026-06-18
> **Status**: Draft (proposal phase)
> **Plan name**: `core-api-and-ui-shell`
> **Base commit**: `296f904` (Plan 2 merged to main)
> **Exploration report**: `docs/superpowers/explorations/2026-06-18-core-api-and-ui-shell.md`
> **Design spec**: `docs/superpowers/specs/2026-06-17-viewred-v1-design.md`

---

## Intent

Bridge the capture/storage pipeline (Plan 2) and the React UI shell (Plan 1) by adding the HTTP API layer (REST + SSE) and wiring the frontend to consume real data. After this plan, the dashboard shows live metrics from storage, the packet list queries real data, and the architecture supports adding new endpoints in subsequent plans without structural changes.

This plan does **not** build all 8 V1 features. It establishes the **pattern and skeleton** that subsequent plans (4-7) extend with their domain-specific endpoints and the LLM/scanner integrations.

---

## Scope

### In scope (this plan delivers)

**Backend:**
- API middleware: CORS, request logging, request ID
- `storage.Querier` interface + 5 query implementations (dashboard summary, top talkers, protocol distribution, paginated packets, devices)
- REST endpoints: 7 real (dashboard 3 + packets 2 + devices 2) + 5 stub routes (alerts, topology, vulnerabilities, config) so the UI has stable URLs
- SSE broker package (`internal/sse/`) with client registration, ring buffer, heartbeat
- SSE `/api/v1/stream/metrics` endpoint (1 Hz ticker, polls storage)
- Router refactor: `NewRouterWithDeps(RouterDeps)` extending the existing `NewRouterWithRegistry` pattern
- `cmd/viewred/main.go` wiring: SSE broker + metrics ticker injected into router

**Frontend:**
- React Router v7 for URL-based routing (`/dashboard`, `/captura`, etc.)
- Zustand store for SSE data and shared state
- SSE client with auto-reconnect + exponential backoff
- shadcn/ui initialization (Card, Badge, Table, Progress, Toast)
- `DashboardView` wired to real SSE data (replaces `setInterval` fakes)
- `CapturaView` wired to real capture status
- Loading / error / empty states across views

### Out of scope (deferred to other plans)

| Feature | Deferred to | Reason |
|---------|-------------|--------|
| Alert endpoints with real data | Plan 4 | Depends on ML engine |
| Topology endpoints + device discovery | Plan 5 | Needs ARP/mDNS scanner |
| LLM chat endpoints | Plan 6 | Needs real Ollama client |
| Vulnerability/scan endpoints | Plan 7 | Needs nmap integration |
| Config write (PATCH) | Plan 8 | Low priority, can wait |
| `internal/bus` (event pipeline) | Plan 4 | SSE broker + channels sufficient for now; bus needed when ML aggregator requires pub/sub |
| E2E tests (Playwright) | Plan 8 | Needs real DB + capture stack |
| Topology / alert / LLM SSE streams | Plans 4, 5, 6 | Routes registered as stubs now, populated later |

### Why this scope is bounded

Building all 19 REST endpoints and 4 SSE streams from the spec in one plan would be ~30+ tasks, overwhelm review cycles, and violate the "1 PR per plan" workflow. The minimal viable API surface that makes the UI demonstrably live is **7 real endpoints + 5 stub routes + 1 live SSE stream**. Subsequent plans add their endpoints in the same pattern.

---

## Approach

### Backend

1. **Storage query layer** is the foundation. Add `internal/storage/queries.go` with a `Querier` interface and 5 implementations. The interface is mocked in handler tests so handlers are testable without a real DB.
2. **SSE broker** is a new `internal/sse/` package with `Broker` (client registry + ring buffer + heartbeat) and a `Subscribe(ctx, topics) (<-chan Event, func())` API. Backpressure: drop oldest events if client buffer is full.
3. **Handlers** consume the `Querier` interface. Tier-1 handlers return real data; tier-2 handlers (stubs) return `200 OK` with empty arrays or `501 Not Implemented` for the methods that need a POST body.
4. **Router** gains a `NewRouterWithDeps(RouterDeps)` constructor that wires middleware, real handlers, stub handlers, and SSE endpoints. The existing `NewRouterWithRegistry` and `NewRouter()` (back-compat) stay.
5. **main.go** builds the SSE broker, starts the metrics ticker goroutine, and passes everything into `NewRouterWithDeps`.

### Frontend

1. **Sequencing matters** to avoid breaking the UI mid-migration. Order:
   1. Add Zustand + SSE client (no routing change, no data wiring yet)
   2. Add React Router (no content change — wrap existing views in `<Route>`)
   3. Wire `DashboardView` to real SSE data via the store
   4. Wire `CapturaView` to real capture status
   5. Add loading/error/empty states
   6. (skipped — shadcn/ui deferred to Plan 8)
2. **SSE client** is a small wrapper around the native `EventSource` API: auto-reconnect with exponential backoff, exposes `useSSE(topic)` React hook.
3. **Zustand store** holds SSE messages + connection status; components subscribe with selectors.

### User-confirmed key decisions

These 5 decisions are locked in for this plan:

| # | Decision | Choice |
|---|----------|--------|
| 1 | API scope | 7 real endpoints + 5 stub routes (NOT all 19) |
| 2 | Frontend routing | React Router v7 (URL-based, not state-based) |
| 3 | UI library | **Deferred to Plan 8** (installer phase). Plan 3 uses custom components. |
| 4 | Fallback mode | Real errors when DB unavailable (no fake generated data) — forces honest loading/error/empty states |
| 5 | SSE scope | Only `/stream/metrics` live now; other SSE routes registered as stubs |

---

## TDD Discipline

This plan applies the **Iron Law** of superpowers TDD strictly during implementation:

> **NO PRODUCTION CODE WITHOUT A FAILING TEST FIRST**

Applied as:
- **Every Go handler**: failing test using `httptest` + gin test mode first, watch it fail, then minimal implementation
- **Every query method**: failing test using `go-sqlmock` first, watch it fail, then minimal SQL + Go code
- **SSE broker**: failing tests for client registration, broadcast, heartbeat, disconnect handling
- **Frontend components**: failing test with Vitest + Testing Library first, watch it fail, then implementation
- **Mocks only when unavoidable**: SSE ticker uses `time.NewTicker` abstraction for testability
- **Regression tests for any bug found** during development — test reproduces the bug BEFORE the fix

The previous Plan 2 demonstrated this discipline with 16 commits of RED → GREEN → REFACTOR. Plan 3 follows the same pattern with no exceptions.

**Implementation-phase prompt will include** `test-driven-development` skill path and explicitly require implementers to show the RED output in their return.

---

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| **Scope creep** — adding "just one more endpoint" | 25+ tasks, plan becomes unreviewable | Strict scope boundaries in this proposal. Per-task review will reject out-of-scope work. |
| **No real TimescaleDB in dev** — query methods can't integration-test | SQL bugs may slip through | Unit tests with `go-sqlmock` (proven pattern from Plan 2). Real DB integration test deferred to CI. |
| **SSE concurrency bugs** — goroutine leaks from slow clients | Service degrades over time | Ring buffer per client (spec §7.5), context cancellation on disconnect, heartbeat to detect dead connections, tested with `httptest`. |
| **CORS misconfig in dev** — frontend fails silently | Demo broken | CORS middleware tested with `httptest`; Vite dev proxy is already configured in `web/vite.config.ts`. |
| **Frontend change fatigue** — too many simultaneous changes | Bug-prone migration | Strict sequencing: Zustand+SSE → Router → Wire views → Loading states → shadcn/ui. Per-step verification. |
| **subagent smoke test timeouts** on Windows | Plan 14 reviewer cancellation pattern | Orchestrator runs smoke tests inline with port-based cleanup; subagents only run unit tests. |

---

## Success Criteria

Observable, testable criteria for "Plan 3 is done":

1. **API works**: `curl http://127.0.0.1:8080/api/v1/dashboard/summary` returns `200 OK` with a valid JSON body when DB is up; `503 Service Unavailable` when DB is down
2. **API error states**: `curl /api/v1/dashboard/summary` with DB down returns an error response, NOT mock data
3. **SSE works**: `curl -N http://127.0.0.1:8080/api/v1/stream/metrics` produces a stream of `event: metrics\ndata: {...}\n\n` messages at 1 Hz
4. **Stub routes exist**: `curl /api/v1/alerts` returns `200 OK` with `[]` (not `404`)
5. **Frontend routes work**: navigating to `http://127.0.0.1:5173/dashboard` (Vite dev) or `http://127.0.0.1:8080/dashboard` (built) shows the dashboard
6. **Dashboard live data**: dashboard shows real metrics from `/stream/metrics` (KPIs update without page refresh)
7. **Loading states**: with DB down, dashboard shows an error/empty state (not "always works" mock)
8. **TDD proof**: every Go commit's history shows RED output → GREEN output. The final commit count includes regression tests for any bugs found.
9. **Build clean**: `go test ./...` 9/9 packages pass, `go vet ./...` clean, `pnpm build` OK
10. **Smoke test**: backend binary starts, `/api/v1/health` returns 200, `/api/v1/dashboard/summary` returns either 200 (DB up) or 503 (DB down), frontend builds and embeds

---

## Dependencies

### From Plan 2 (consumed, not re-implemented)

- `cmd/viewred/main.go` lifecycle (config → storage → capture → writers → API → shutdown)
- `internal/storage/storage.go` (pool, ping, close)
- `internal/storage/migrations/0001_initial.up.sql` (TimescaleDB hypertables)
- `internal/capture/capture.go` (PacketCapture interface, PacketEvent, CaptureStats)
- `internal/health/health.go` (Registry, Report)
- `internal/shutdown/shutdown.go` (ordered teardown)
- `internal/api/router.go` (NewRouterWithRegistry pattern to extend)

### External libraries (new in this plan)

Backend (Go):
- `github.com/gin-gonic/gin/cors` — CORS middleware (Gin has it built in)
- (no new DB driver — uses pgx from Plan 2)
- (no new SSE library — uses raw `text/event-stream` + `http.Flusher`)

Frontend (TypeScript):
- `react-router-dom` v7 — URL routing
- `zustand` — state management
- No React Query, no SWR — Zustand + SSE + direct fetch is sufficient for V1
- shadcn/ui dependencies (`clsx`, `tailwind-merge`, `class-variance-authority`, sonner, lucide-react) deferred to Plan 8

### Repository / tooling
- `npx shadcn@latest init` (one-time, user confirms in cli)
- No new build tooling

---

## Open Questions

These should be resolved before the spec/design phase begins:

1. **Frontend embedding strategy**: Plan 1 already serves `web/dist/` as static files. Plan 3 doesn't change this, but should we `go:embed` the dist for single-binary distribution? (Defer to Plan 8 — installer phase.)
2. **SSE auth**: V1 has no auth (per spec §3.3). SSE endpoints are open. Acceptable for V1.
3. **Metrics ticker interval**: Spec doesn't specify; explore recommends 1 Hz. Confirm.
4. **Plan name**: keep `core-api-and-ui-shell` (user's original) or rename to `core-api-sse-and-ui-wiring` (more accurate per explore). Keep the original unless user objects.

---

## Recommendation

**Proceed to `sdd-spec`** with this scope. The proposal locks in the 5 user-confirmed decisions and the bounded scope. The next phase (`sdd-spec`) will write detailed requirements with scenarios for each endpoint, query method, SSE stream, and frontend route — preserving the TDD Iron Law for the implementation phase.

Estimated implementation: **15-21 tasks across 8-10 work units** (per the explore report's effort breakdown). Medium complexity.

Plan name retained as `core-api-and-ui-shell` per the user's original framing. (The explore's rename suggestion to `core-api-sse-and-ui-wiring` is documented but not adopted.)
