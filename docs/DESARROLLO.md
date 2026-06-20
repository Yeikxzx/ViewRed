# ViewRed — Documento de Desarrollo

> **Última actualización**: 2026-06-20
> **Branch activo**: `feat/plan4-ml-and-alerts` (21 commits desde `main`)
> **HEAD**: `7079555`

---

## Índice

1. [Resumen del proyecto](#1-resumen-del-proyecto)
2. [Stack tecnológico](#2-stack-tecnológico)
3. [Arquitectura](#3-arquitectura)
4. [Estructura del proyecto](#4-estructura-del-proyecto)
5. [Plan de desarrollo (8 planes)](#5-plan-de-desarrollo-8-planes)
6. [Estado actual — Branch feat/plan4-ml-and-alerts](#6-estado-actual--branch-featplan4-ml-and-alerts)
7. [Reglas de desarrollo](#7-reglas-de-desarrollo)
8. [Issues conocidos](#8-issues-conocidos)
9. [Próximos pasos](#9-próximos-pasos)
10. [Apéndice: commits](#10-apéndice-commits)

---

## 1. Resumen del proyecto

ViewRed es un **monitor de red local** con detección de anomalías basada en IA. Corre en `127.0.0.1:8080` con acceso vía SSH tunnel. Captura paquetes en vivo, almacena metadata en PostgreSQL + TimescaleDB, muestra un dashboard React con datos en tiempo real vía SSE, y usa:

- **ML local**: Isolation Forest + Random Forest en ONNX Runtime para detección de anomalías
- **LLM local**: Qwen 2.5 3B vía Ollama para asistente en lenguaje natural
- **Reglas**: Suricata-style para detección basada en firmas

### Caso de uso principal

Administrador de red hace SSH a su servidor, forwardea el puerto 8080, y monitorea tráfico, dispositivos, alertas y topología desde el browser sin depender de servicios cloud.

---

## 2. Stack tecnológico

| Componente | Tecnología | Versión |
|---|---|---|
| Lenguaje backend | Go | 1.26.3 |
| HTTP framework | Gin | v1.12 |
| DB driver | pgx/v5 + database/sql | — |
| Base de datos | PostgreSQL + TimescaleDB | — |
| Captura de paquetes | gopacket (Npcap Windows / AF_PACKET Linux) | — |
| ML Runtime | ONNX Runtime (github.com/yalue/onnxruntime_go) | v1.16 |
| Modelos ML | Isolation Forest + Random Forest (exportados a ONNX via skl2onnx) | — |
| LLM | Qwen 2.5 3B vía Ollama (http://localhost:11434) | Ollama 0.30.9 |
| Frontend | React 18 + TypeScript | — |
| Bundler | Vite | 6.x |
| CSS | Tailwind CSS | 4.x |
| Estado global | Zustand | v5 |
| Routing | React Router | v7 |
| Charting | Recharts | v3 |
| Testing Go | go test + testify + sqlmock | — |
| Testing frontend | Vitest + Testing Library + jsdom | — |
| Migraciones DB | golang-migrate | v4 |

### Host de desarrollo

Windows 11. Sin Docker, sin WSL. Tooling disponible:
- Go 1.26.3, Node 24.16.0, npm 11.13.0, pnpm 9.12.0
- Git 2.54.0, Ollama 0.30.9 (qwen2.5:3b cargado)
- winget, Chocolatey, Scoop

---

## 3. Arquitectura

### 3.1 Patrones generales

| Patrón | Implementación |
|---|---|
| Inyección de dependencias | Constructor injection (sin frameworks) |
| Querier interface | `storage.Querier` → `*Storage` / `MockQuerier` / `ErrorQuerier` |
| Test doubles | `MockQuerier` con function fields, `ErrorQuerier` para 503 |
| SSE Pub/Sub | Broker con topic-based routing, ring buffer 64, heartbeat 30s |
| Shutdown ordenado | `shutdown.Shutdown(params)` con interfaces pequeñas |

### 3.2 Router

Tres constructores, back-compat preservado:

```go
NewRouter()                                   → solo health + static
NewRouterWithRegistry(rg *health.Registry)    → health custom + static
NewRouterWithDeps(deps RouterDeps)            → full router con todo
```

**Middleware order**: `Recovery → RequestID → [CORS] → RequestLogging → routes`

Las assets estáticas se montan **antes** de los middleware globales para evitar que CORS bloquee scripts.

### 3.3 SSE Broker

- `internal/sse/broker.go` — event loop con register/unregister/broadcast/heartbeat channels
- `internal/sse/client.go` — ring buffer de 64 eventos, drop del más viejo en overflow
- `internal/sse/event.go` — `Event{Topic, Name, Data}`
- Heartbeat cada 30s, stale client removal tras 3 heartbeats perdidos
- `sse.Event.Topic` para routing: clients se suscriben a topics, events sin topic son broadcast global

### 3.4 Shutdown ordenado

```
1. capture.Stop()
2. close(packets channel)     → drena writers
3. close(evalPackets channel) → drena ML workers
4. pcapWriter.Close()
5. storage.Writer.Flush()     → último batch con contexto fresco
6. http.Server.Shutdown()
7. broker.Shutdown()          → evento graceful a clients
8. storage.Close()
```

### 3.5 Frontend

| Componente | Propósito |
|---|---|
| `BrowserRouter` + `Routes` | React Router v7 con SPA fallback |
| `useSSEStore` (Zustand) | Estado global: messages, connectionStatus, captureStatus, lastError |
| `useSSE(topic)` | Hook con EventSource nativo, auto-reconnect (1s→30s, exponential backoff + jitter) |
| `ViewState<T>` | Pattern: `loading` \| `error` \| `empty` \| `data` |
| `api.ts` | Fetch wrapper con tipos backend: getTopTalkers, getProtocols, getRecentAlerts, etc. |
| `BackendLLMClient` | Cliente chat con SSE streaming vía `/api/v1/chat` |

### 3.6 ML Pipeline

```
PacketEvent → WindowStats.Record() → ExtractFeatures(pkt, ws) → [7 features]
  → ONNX tensor (1, 7)
    → Isolation Forest: label == -1? → Anomalía → Random Forest → clasificación (0-3)
    → label != -1? → Tráfico normal, descartado
```

**Feature vector** (7 dimensiones):

| # | Feature | Rango | Descripción |
|---|---------|-------|-------------|
| 0 | Length | 0-65535 | Tamaño del paquete |
| 1 | Proto_Idx | 0-3 | 0=TCP, 1=UDP, 2=ICMP, 3=OTHER |
| 2 | SrcPort | 0-65535 | Puerto origen |
| 3 | DstPort | 0-65535 | Puerto destino |
| 4 | PacketsPerSec | 0-N | Tasa de paquetes para esta src IP (ventana 60s) |
| 5 | UniqueDstPorts | 0-N | Puertos destino únicos contactados por esta src IP |
| 6 | PayloadEntropy | 0-8 | Shannon entropy de primeros 256 bytes |

**WindowStats**: sliding window de 60s por IP origen, thread-safe con mutex,
cleanup automático cada 5 minutos.

### 3.7 ML Build Constraints

- `engine.go` → `//go:build cgo` (requiere ONNX Runtime C library)
- `engine_stub.go` → `//go:build !cgo` (no-op para builds sin CGO)
- `features.go` + `window.go` → sin build tag (compilan siempre)

---

## 4. Estructura del proyecto

```
viewred/
├── cmd/viewred/
│   ├── main.go           ← Entry point, wiring, shutdown
│   └── main_wire.go      ← wireUp: broker + ticker + router
├── internal/
│   ├── api/
│   │   ├── router.go     ← NewRouter, NewRouterWithDeps
│   │   ├── handlers/     ← Dashboard, Packets, Devices, Alerts, Topology,
│   │   │                    LLM Chat, Rules Config, Vulnerabilities, SSE, Stubs
│   │   └── middleware/   ← CORS, RequestLogging, RequestID
│   ├── capture/          ← PacketCapture interface, PacketEvent, PcapWriter
│   ├── config/           ← YAML config loader, defaults
│   ├── discovery/        ← ARP/DHCP/mDNS discovery engine, OUI vendor lookup
│   ├── health/           ← Health registry with subsystem checks
│   ├── ml/               ← ONNX engine, features, WindowStats
│   ├── rules/            ← Suricata-style rule parser and engine
│   ├── scanner/          ← nmap wrapper for vulnerability scanning
│   ├── shutdown/         ← Ordered teardown orchestrator
│   ├── sse/              ← Broker, client, event
│   └── storage/          ← Querier interface, Storage, Writer, Migrations
├── web/
│   ├── src/
│   │   ├── App.tsx       ← Router, layout, SSE alerts, chat, state
│   │   ├── components/   ← DashboardView, CapturaView, Sidebar, Header,
│   │   │                    FiltrosView, AlertasView, TopologiaView,
│   │   │                    VulnerabilidadesView, AsistenteView, ConfiguracionView
│   │   ├── store/        ← sse-store.ts (Zustand)
│   │   ├── lib/          ← sse.ts (useSSE), api.ts, llm.ts
│   │   └── types/        ← TypeScript interfaces
│   └── package.json
├── models/               ← ONNX models (isolation_forest.onnx, random_forest.onnx)
├── scripts/              ← train_models.py, build.ps1
├── docs/
│   ├── DESARROLLO.md     ← Este documento
│   ├── context/          ← Dump de contexto completo (11 archivos)
│   └── superpowers/      ← SDD artifacts: planes, specs, designs, briefs, reports
└── openspec/             ← SDD artifact store
```

---

## 5. Plan de desarrollo (8 planes)

### Plan 1: Bootstrap ✅ (mergeado a main)

Setup inicial del monorepo: `go.mod`, `web/package.json`, Vite + Tailwind,
Makefile, docker-compose para dev DB, GitHub Actions CI, README.

### Plan 2: Capture and Storage ✅ (mergeado a main, commit `296f904`)

- Captura de paquetes con gopacket (Npcap Windows / AF_PACKET Linux)
- Almacenamiento en TimescaleDB via pgx
- Health checks con registry extensible
- Shutdown ordenado con `internal/shutdown/`
- **4 critical fixes** post whole-branch review

### Plan 3: Core API and UI Shell ✅ (mergeado a main, HEAD `8d08305`)

**26 tareas + 5 review fixes = 32 commits.**

Lo implementado:
- **Middleware**: CORS (configurable), Request Logging (slog), Request ID (UUID v4)
- **Storage Querier**: Interface + MockQuerier + ErrorQuerier + 5 queries
- **Handlers REST**: dashboard, top-talkers, protocols, packets, devices
- **SSE Broker**: topic-based pub/sub, ring buffer 64, heartbeat 30s
- **Router**: `NewRouterWithDeps(RouterDeps)` preservando back-compat
- **Frontend**: React Router v7, Zustand store, useSSE hook, ViewState pattern
- **Shutdown**: SSE broker integrado en shutdown ordenado

### Plan 4: ML and Alerts 🚧 EN PROGRESO (branch feat/plan4-ml-and-alerts)

- ✅ Rules engine (Suricata-style parser + evaluator)
- ✅ ML engine (ONNX Runtime, Isolation Forest + Random Forest)
- ✅ Feature extraction (7 features, WindowStats)
- ✅ Alert CRUD handlers + DB migrations
- ✅ Alertas en tiempo real vía SSE
- ❌ ML pipeline integrado con storage writer y SSE broker en main.go
- ❌ Tests para ml/ package
- ❌ Entrenamiento con datasets reales (CIC-IoT-2023 recomendado)
- ⚠️ Alert descriptions en inglés (corregido)

### Plan 5: Topology and Discovery 🚧 PARCIAL (branch feat/plan4-ml-and-alerts)

- ✅ Discovery engine (ARP/DHCP/mDNS)
- ✅ OUI vendor lookup con download automático
- ✅ Topology queries (dispositivos + edges)
- ✅ TopologiaView con React Flow
- ❌ Tests para discovery.Engine

### Plan 6: LLM Assistant 🚧 PARCIAL (branch feat/plan4-ml-and-alerts)

- ✅ LLM Chat Handler con proxy a Ollama + SSE streaming
- ✅ BackendLLMClient en frontend
- ✅ Persistencia de conversaciones en DB
- ❌ Tests de integración con Ollama

### Plan 7: Scanner and Vulns 🚧 PARCIAL (branch feat/plan4-ml-and-alerts)

- ✅ Scanner engine con nmap wrapper
- ✅ Vulnerabilities CRUD + DB migrations
- ❌ Fallback para sistemas sin nmap
- ❌ Tests para scanner/

### Plan 8: Installer and Polish ❌ No iniciado

- Windows installer
- shadcn/ui migration (deferido de Plan 3)
- UX polish final

---

## 6. Estado actual — Branch feat/plan4-ml-and-alerts

### 6.1 Commits (21 desde main)

```
7079555 chore: add onnxruntime.dll and oui.txt to gitignore, update context docs
90cd7b5 feat(cmd): point ML engine to models directory for ONNX model loading
73de228 feat(scanner): add nmap-based vulnerability scanning engine
34ff4c4 feat(cmd): update SSE event name and add default rules config
aaa7fcd feat(web): wire dashboard to real API, add chat streaming, toast notifications
7ac9899 feat(api): add LLM chat, rules CRUD, alerts handlers and update router
a521d71 feat(storage): add port/payload_search packet filters, migrations, and new query methods
b7ba0a0 docs(context): add complete project context dump for disaster recovery
d4be4fc feat(ml): expand feature vector to 7 dimensions with sliding window stats
de67698 fix(ml): restore engine_stub.go for non-CGO builds and add cgo build tag
bf8a3c4 refactor(api): remove unused VulnerabilitiesHandler struct
03ca531 fix(discovery): add 30s timeout to OUI database HTTP download
402fe33 fix(cmd): guard store.GetCustomRules against nil when DB unavailable
8a4e157 feat(topology): implement plan 5 discovery engine and UI
8d735b6 fix(eval): prevent goroutine leak in evalPackets and optimize Suricata rules port parsing
525997a docs(plan3): add handover document for next agent
8d08305 docs(plan3): append review-fixes and re-verification to e2e report          ← main HEAD
... (más commits de Plan 3)
```

### 6.2 Tests

| Suite | Resultado |
|---|---|
| `go test ./...` | ✅ 14/14 packages |
| `go vet ./...` | ✅ Limpio |
| `go build ./...` | ✅ Compila |
| Frontend (vitest) | ✅ 8 files, 91 tests |
| TypeScript (tsc --noEmit) | ✅ Sin errores |

### 6.3 Bugs corregidos en verificación

| ID | Severidad | Qué | Fix |
|---|---|---|---|
| C1 | 🔴 CRITICAL | `store.GetCustomRules(ctx)` sin nil guard → panic si DB no disponible | `if store != nil {` |
| C2 | 🔴 CRITICAL | `engine_stub.go` eliminado, `engine.go` sin build tag cgo → build roto sin CGO | Restauré stub + tag |
| H1 | 🟠 HIGH | `http.Get()` sin timeout en OUI download → startup puede colgar | `http.Client{Timeout: 30s}` |
| H2 | 🟠 HIGH | `VulnerabilitiesHandler` struct dead code, scanner creado por request | Eliminé dead code |
| H3 | 🟠 HIGH | Alert descriptions en español (viola convención inglés en código) | Traducidas a inglés |

### 6.4 Paquetes sin tests

| Package | Tests | Prioridad |
|---|---|---|
| `internal/ml` | 0 tests | 🔴 Alta — feature engineering sin cobertura |
| `internal/scanner` | 0 tests | 🔴 Alta — dependencia externa (nmap) |
| `internal/discovery` | 2 tests (OUI only) | 🟡 Media — Engine sin test |
| `internal/rules` | 2 tests (básicos) | 🟡 Media — Parse + Evaluate |

---

## 7. Reglas de desarrollo

### 7.1 TDD Iron Law

**NO PRODUCTION CODE WITHOUT A FAILING TEST FIRST.**
RED → GREEN → REFACTOR para cada tarea Go/TypeScript.

`strict_tdd: true` configurado en SDD init.

### 7.2 Conventional Commits

```
feat:     Nueva funcionalidad
fix:      Corrección de bug
refactor: Cambio que no agrega funcionalidad ni corrige bug
test:     Tests nuevos o actualizados
docs:     Documentación
chore:    Tooling, config, CI
```

Scope opcional: `feat(api):`, `fix(web):`, etc.

**Sin AI attribution**: No `Co-Authored-By:`, no "Generated by".

**Work-unit commits**: Un cambio lógico por commit. Size exception:
incluir `(size exception: NNN lines)` cuando >400 líneas.

### 7.3 Code style

- **Inglés** en código, identificadores, funciones, comentarios, UI copy, errores
- **Español rioplatense** solo en conversación con el usuario
- Cross-platform: Windows + Linux
- Sin `panic` en producción (usar `gin.Recovery()`)
- Sin DI frameworks — constructor injection

### 7.4 SDD Workflow

Para cada nuevo plan:

```
explore → propose → spec → design → tasks → apply → verify → archive
```

Cada fase es un sub-agente. Apply es un sub-agente por tarea.
Whole-branch review obligatorio después de cada plan (catcha integration bugs).

---

## 8. Issues conocidos

### 8.1 Del handover de Plan 3 (no corregidos)

| ID | Severidad | Descripción | Archivo |
|---|---|---|---|
| W2 | MEDIUM | `ProtocolShare` no tiene campo `Bytes` (spec lo requiere, frontend usa `percentage`) | `querier.go` |
| W4 | LOW | SSE no emite `event: error` con `recoverable: true` cuando falla query | `sse.go` |
| W5 | LOW | SSE no emite snapshot inicial al conectar | `sse.go` |
| W6 | LOW | `Broker.Subscribe` race raro si ctx se cancela mid-call | `broker.go` |
| W7 | LOW | Defers redundantes en main.go | `main.go` |
| S1 | LOW | Constante `"metrics"` duplicada | `sse.go` + `main_wire.go` |
| S2 | LOW | `Broker.Shutdown` no emite evento graceful | `broker.go` |
| S4 | LOW | `act()` warnings en tests frontend | CapturaView, App |

### 8.2 Issues del branch actual

| ID | Severidad | Descripción | Archivo |
|---|---|---|---|
| — | INFO | Router middleware order: spec dice Recovery→CORS→RequestID, código hace Recovery→RequestID→CORS | `router.go` |
| — | INFO | `scanner.Engine` requiere nmap binary — sin fallback en Windows | `scanner/nmap.go` |
| — | INFO | Score de alertas ML hardcodeado a 85.0 | `ml/engine.go` |
| — | INFO | LLM Chat guarda mensaje assistant en goroutine sin error handling | `handlers/llm_chat.go` |

### 8.3 Datasets recomendados para reentrenar ML

Prioridad:

1. **CIC-IoT-2023** — https://www.unb.ca/cic/datasets/iotdataset-2023.html (IoT real + ataques, 2023)
2. **Edge-IIoTset 2022** — https://ieee-dataport.org/open-access/edge-iiotset (IoT/IIoT, 14 ataques)
3. **CIC-IDS2017** — https://www.unb.ca/cic/datasets/ids-2017.html (gold standard, 80+ features)
4. **TON_IoT** — https://research.unsw.edu.au/projects/toniot-datasets (sensores + telemetría)
5. **CIC-Darknet2020** — https://www.unb.ca/cic/datasets/darknet2020.html (túneles/anonymización)

---

## 9. Próximos pasos

### Prioridad 1: Completar integración de ML en main.go

El ML Engine (`Evaluate`) genera alerts pero no están wireadas al `storage.Writer`
ni al SSE broker en `main.go`. El worker pool las evalúa pero se pierden.

### Prioridad 2: SDD Artifacts para Planes 4-6

No existen documentos SDD (explore → propose → spec → design → tasks)
para los planes actuales. Usar `sdd-new` para formalizar.

### Prioridad 3: Tests faltantes

- `internal/ml/` — tests para ExtractFeatures, WindowStats, PayloadEntropy
- `internal/scanner/` — tests sin nmap (mock)
- `internal/discovery/engine.go` — tests para Process, getDeviceID
- `internal/rules/` — tests para casos edge del parser

### Prioridad 4: Feature engineering adicional

- Agregar TCP flags al feature vector (requiere capturarlos en PacketEvent)
- Flow-based features (duración de conexión, bytes por flow)
- Feature scaling antes de ONNX inference

### Prioridad 5: Limpieza

- Decidir entre `package-lock.json` (npm) y `pnpm-lock.yaml` (legacy)
- Probar CI build con `CGO_ENABLED=0` (engine_stub.go debe compilar)
- Decidir merge strategy: squash merge a main o mantener branch

---

## 10. Apéndice: commits

### Commits en feat/plan4-ml-and-alerts (desde main)

```
7079555 chore: add onnxruntime.dll and oui.txt to gitignore, update context docs
90cd7b5 feat(cmd): point ML engine to models directory for ONNX model loading
73de228 feat(scanner): add nmap-based vulnerability scanning engine
34ff4c4 feat(cmd): update SSE event name and add default rules config
aaa7fcd feat(web): wire dashboard to real API, add chat streaming, toast notifications
7ac9899 feat(api): add LLM chat, rules CRUD, alerts handlers and update router
a521d71 feat(storage): add port/payload_search packet filters, migrations, and new query methods
b7ba0a0 docs(context): add complete project context dump for disaster recovery
d4be4fc feat(ml): expand feature vector to 7 dimensions with sliding window stats
de67698 fix(ml): restore engine_stub.go for non-CGO builds and add cgo build tag
bf8a3c4 refactor(api): remove unused VulnerabilitiesHandler struct
03ca531 fix(discovery): add 30s timeout to OUI database HTTP download
402fe33 fix(cmd): guard store.GetCustomRules against nil when DB unavailable
8a4e157 feat(topology): implement plan 5 discovery engine and UI
8d735b6 fix(eval): prevent goroutine leak in evalPackets and optimize Suricata rules port parsing
525997a docs(plan3): add handover document for next agent
```

### Commits en main (Planes 1-3)

```
8d08305 docs(plan3): append review-fixes and re-verification to e2e report
c4799fc fix(api): validate inbound X-Request-Id against CRLF and length
de5c154 fix(api): wire CORS middleware with localhost:5173 default in dev
570f228 fix(cmd): pass Broker to shutdown.Params; remove redundant defer
cd95ba4 fix(api): remove per-request ticker from StreamMetricsHandler (relay only)
04fd5d2 fix(storage): add JSON tags to Querier result structs (snake_case wire format)
5762d0a docs(plan3): add e2e verification report for core-api-and-ui-shell
7b0c3f7 feat(web): add ViewState<T> pattern and loading/error/empty states
92842f9 feat(web): wire CapturaView to real health endpoint polling
075c8b5 feat(web): wire DashboardView to real SSE metrics stream
9feb182 refactor(web): use React Router NavLink in Sidebar navigation
fe728ae refactor(web): migrate App.tsx from state-based to React Router v7
afec310 fix(web): resolve useSSE hook test timeouts
ae5f3ab feat(web): add zustand store for SSE and capture status
ee3b3bc feat(shutdown): integrate SSE broker into ordered teardown
f88e4fd feat(cmd): wire SSE broker and Querier into main.go lifecycle
8bc0099 feat(api): add NewRouterWithDeps constructor preserving back-compat
... (más commits de Plan 2 y Plan 1)
```

---

## 11. Cómo correr el proyecto en desarrollo

### Requisitos

Ver `docs/SETUP.md` para la instalación completa desde cero.

### 1. Base de datos (opcional — el proyecto funciona sin DB)

Con Docker:
```powershell
docker compose -f deploy/docker-compose.dev.yml up -d
```

Sin Docker: el binario arranca igual, los endpoints devuelven 503
cuando la DB no está disponible.

### 2. Backend (Go)

```powershell
# Desde la raíz del proyecto
go run ./cmd/viewred
# Escucha en http://127.0.0.1:8080
```

Variables de entorno:

| Variable | Default | Descripción |
|---|---|---|
| `VIEWRED_CONFIG` | `%APPDATA%/viewred/config.yaml` | Ruta al archivo de config |
| `VIEWRED_INTERFACE` | auto-detect | Interfaz de red para captura |
| `VIEWRED_PCAP_DIR` | `%TEMP%/viewred-pcap` | Directorio para archivos pcap |

### 3. Frontend (Vite dev server)

```powershell
# En otra terminal:
cd web
pnpm dev
# Escucha en http://127.0.0.1:3000
```

El frontend en dev se conecta al backend en `localhost:8080` vía CORS.
La config de CORS ya permite `localhost:5173` (Vite default) y
`localhost:3000` (puerto custom del package.json).

### 4. Tests

```powershell
# Todos los tests
go test ./...
cd web && pnpm test

# Backend + frontend separados
go test ./...
cd web && pnpm vitest run

# Type checking
cd web && pnpm tsc --noEmit
```

### 5. Build de producción

```powershell
# Backend
go build -o viewred.exe ./cmd/viewred

# Frontend (genera web/dist/)
cd web && pnpm build
```

El frontend buildeado se sirve automáticamente desde el binario Go
(lo monta en `/` via `mountWebAssets`).

---

> Este documento se actualiza manualmente. Para regenerar el contexto
> completo automáticamente, ejecutar el agente con `docs/context/00-README.md`.
