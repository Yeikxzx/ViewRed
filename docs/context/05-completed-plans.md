# ViewRed — Planes Completados (mergeados a main)

## Plan 1: Bootstrap

Monorepo Go + React. Setup inicial con `go.mod`, `web/package.json`, Vite, Tailwind, Makefile, docker-compose para dev DB, GitHub Actions CI, README.

## Plan 2: Capture and Storage (commit `296f904`)

Captura de paquetes con gopacket, almacenamiento en TimescaleDB via pgx, health checks, shutdown ordenado. 15 tareas + 4 critical fixes post whole-branch review.

### Fixes críticos de Plan 2
- Health registry no estaba wireado a la API
- Shutdown defers en orden incorrecto (store se cerraba antes que el writer)
- Goroutine de pcap bridge leak en shutdown
- Último batch de storage dropeado en shutdown (por contexto cancelado)

Se introdujo `internal/shutdown/` con `Writer.Done()` para coordinación.

## Plan 3: Core API and UI Shell (HEAD `8d08305`)

Bridge entre Plan 2 y la UI React. 26 tasks + 5 review fixes = 32 commits.

### Lo implementado
- **Middleware**: CORS (configurable), Request Logging (slog), Request ID (UUID v4)
- **Storage Querier**: Interface + MockQuerier + ErrorQuerier + 5 queries (dashboard, top-talkers, protocols, packets, devices)
- **Handlers REST**: dashboard/summary, top-talkers, protocols, packets, devices, alerts, topology, vulnerabilities, config, chat, rules CRUD
- **SSE Broker**: topic-based pub/sub, ring buffer 64, heartbeat 30s, stale removal
- **Router**: `NewRouterWithDeps(RouterDeps)` preservando back-compat
- **Frontend**: React Router v7, Zustand store, useSSE hook, DashboardView con SSE real, CapturaView con health polling, ViewState pattern (loading/error/empty)
- **Shutdown**: SSE broker integrado en shutdown ordenado

### Fixes críticos de Plan 3 (whole-branch review)
- C1: Structs sin JSON tags → json.Marshal producía PascalCase, frontend esperaba snake_case
- C2: `broadcastTick` y SSE handler hacían doble broadcast
- C3: Shutdown defers en orden incorrecto (broker después de store)
- C4: CORS no wireado en main.go
- W1: CRLF injection en X-Request-Id (validación agregada)
