# ViewRed V1 — Technical Design

> **Status**: Draft (pending user review)
> **Date**: 2026-06-17
> **Author**: gentle-orchestrator + user (brainstorming session)
> **Topic**: viewred-v1
> **Related docs**:
> - `que es viewred.md` (product definition, v0.2)
> - `design.md` (UI/UX design, v0.1)

---

## 1. Context

ViewRed is a **Network Detection & Response (NDR)** tool with local AI, designed for small teams (20–100 devices) that need visibility into their network traffic without the cost or complexity of enterprise solutions. The differentiator is **AI applied to network analysis**: ML models for anomaly detection + LLM for human-readable explanations, both running 100% locally.

This document captures the technical design agreed during a brainstorming session on 2026-06-17, evolving an earlier product spec written by the user.

---

## 2. Key decisions (changelog of this session)

| Decision | Was | Now | Reason |
|---|---|---|---|
| **Target scale** | 50–500 devices (SMB) | **20–100 devices** (team) | User clarified the team is small, with possible scale-up later |
| **Deployment OS** | Linux server only | **Windows + Linux (cross-platform from day 1)** | User wants Windows local, with portability |
| **Feature scope** | 7 features | **8 features** (+ vulnerability/port scanning) | User wants "monitoring everything" including vuln scanning |
| **Architecture** | (not yet decided) | **A1: Monolith with build tags** | Best fit for 20–100 devices, single binary, simplest deploy |
| **Installer** | (assumed bundled) | **Layered: native installer + first-run wizard** | License/size constraints prohibit bundling everything |
| **AI in V1** | Required from day 1 | **Confirmed: required from V1** | Project justification depends on AI being visible |
| **Capture library** | Linux AF_PACKET | **Cross-platform: Npcap (Win) + AF_PACKET (Linux)** | Required by Windows support |
| **Real-time transport** | SSE + REST | **Confirmed: SSE + REST** | User understood unidirectional trade-off |
| **LLM model** | Qwen 2.5 3B via Ollama | **Confirmed: Qwen 2.5 3B via Ollama** | User has it running locally |

---

## 3. Target & scope

### 3.1 Primary target

- **Users**: small team of 2–10 people who manage a network together.
- **Network**: 20–100 devices, 100 Mbps – 1 Gbps, single location.
- **OS mix**: Windows + Linux on the network, ViewRed runs natively on both.
- **Skill level**: intermediate (knows IP, port, protocol, not data scientist).
- **Out of target for V1**: home/SOHO (too small), enterprise 10 Gbps+ (too big).

### 3.2 V1 features (8 total)

1. **Captura y almacenamiento de paquetes** — pcap rotativo + metadatos en DB.
2. **Dashboard en tiempo real** — KPIs, top talkers, distribución de protocolos.
3. **Filtros y búsqueda** — sobre paquetes históricos y en vivo.
4. **Alertas con detección IA** — reglas + Isolation Forest + Random Forest.
5. **Descubrimiento y topología 2D** — dispositivos + grafo de conexiones.
6. **Asistente IA en lenguaje natural** — Qwen 2.5 3B, bajo demanda.
7. **Alertas y notificaciones** — panel de alertas en UI.
8. **Escaneo de vulnerabilidades y puertos** — nmap subprocess, persistencia en DB.

### 3.3 Out of scope for V1

- Control activo (bloquear dispositivos, filtrar tráfico) — sería otro producto.
- Topología 3D — caro, valor cuestionable.
- Reportes exportables (PDF, CSV) — V2.
- Integración SIEM / Syslog / NetFlow — V2.
- Multi-tenant / alta disponibilidad — V2/V3.
- RBAC granular — V2.
- Auth interna de la app — V2.
- App móvil — V3.
- Actualización automática de firmas de malware — V2.

### 3.4 Timeline

- **1 dev senior solo**: 4–6 meses para V1.
- **Equipo de 2–3 devs**: 2–3 meses.
- **Definición de done**: los 8 features funcionando, TDD discipline, 80% coverage, documentado, instalable en Windows y Linux.

---

## 4. Stack

| Capa | Tecnología | Versión objetivo |
|---|---|---|
| Lenguaje backend | Go | 1.22+ (usa 1.26 que tenés) |
| Captura paquetes | gopacket con driver Npcap (Win) o AF_PACKET (Linux) | latest |
| Framework HTTP | Gin | latest |
| DB driver | pgx v5 (no database/sql) | v5 |
| Base de datos | PostgreSQL + TimescaleDB | PG 15+, TSDB 2.x |
| Frontend | React + TypeScript + Tailwind | React 18+, TS 5+ |
| Real-time | SSE (server-sent events) sobre HTTP | n/a (estándar) |
| ML runtime | ONNX Runtime Go | latest |
| LLM runtime | Ollama (servicio externo local) | 0.30+ (ya instalado) |
| LLM model | Qwen 2.5 3B (ya descargado) | n/a |
| Scanner | nmap CLI como subprocess | 7.9x |
| Build/cross-compile | Go built-in + Makefile | n/a |
| Test runner Go | go test + testify | latest |
| Test runner React | Vitest + Testing Library | latest |
| E2E | Playwright (cuando llegue el momento) | latest |
| Containerización dev | Docker Compose (para tests de integración) | n/a |

---

## 5. Architecture

### 5.1 Approach A1: Monolith with build tags

Single Go binary, platform-specific code isolated via Go build tags. Internal architecture is modular (clean separation of concerns via interfaces), but physically one process.

### 5.2 Diagram

```
┌──────────────────────────────────────────────────────────────┐
│  viewred.exe / viewred (binario único, Admin/root)           │
│                                                              │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐             │
│  │  capture   │  │  scanner   │  │   API      │  ← Gin       │
│  │  (driver)  │  │  (nmap)    │  │  + SSE     │             │
│  └─────┬──────┘  └─────┬──────┘  └─────┬──────┘             │
│        │               │               │                     │
│        └───────────────┴───────────────┘                     │
│                        │                                     │
│              ┌─────────▼──────────┐                         │
│              │  internal bus      │  ← Go channels           │
│              │  (event pipeline)  │                         │
│              └─────────┬──────────┘                         │
│                        │                                     │
│      ┌─────────────────┼─────────────────┐                  │
│      │                 │                 │                  │
│  ┌───▼────┐  ┌─────────▼──────┐  ┌───────▼──────┐         │
│  │  DB    │  │   ML pipeline  │  │  LLM client  │         │
│  │ (pgx + │  │ (ONNX runtime) │  │  (Ollama     │         │
│  │ Timescale) │  IF + RF     │  │   HTTP API)  │         │
│  └────────┘  └───────────────┘  └──────────────┘         │
│                                                              │
│  ┌────────────────────────────────────────────────────┐    │
│  │  web/ (React build, embebido con go:embed)          │    │
│  └────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────┘
              │                              │
              ▼                              ▼
       ┌────────────┐                  ┌──────────┐
       │ PostgreSQL │                  │  Ollama  │
       │+TimescaleDB│                  │ (Qwen3B) │
       └────────────┘                  └──────────┘
```

### 5.3 Directory structure

```
viewred/
├── cmd/
│   └── viewred/
│       └── main.go                  # entry point, lifecycle
├── internal/
│   ├── capture/
│   │   ├── capture.go               # interfaz PacketCapture + PacketEvent
│   │   ├── npcap_windows.go         # //go:build windows
│   │   ├── afpacket_linux.go        # //go:build linux
│   │   ├── mock_capture.go          # para tests de otros packages
│   │   └── testdata/
│   │       ├── sample.pcap
│   │       └── edge_cases.pcap
│   ├── storage/
│   │   ├── storage.go               # pgx pool, queries
│   │   └── migrations/              # SQL migrations (golang-migrate)
│   ├── scanner/
│   │   ├── scanner.go               # interfaz Scanner
│   │   ├── nmap.go                  # impl con os/exec
│   │   └── mock_scanner.go
│   ├── ml/
│   │   ├── pipeline.go              # orquestación
│   │   ├── features.go              # extracción de features
│   │   ├── isolation_forest.go      # inferencia ONNX
│   │   ├── random_forest.go         # clasificación ONNX
│   │   └── assets/
│   │       └── models/              # .onnx files
│   ├── llm/
│   │   ├── client.go                # cliente Ollama
│   │   ├── prompt.go                # system prompts
│   │   └── mock_client.go
│   ├── api/
│   │   ├── router.go                # Gin setup
│   │   ├── handlers/                # REST handlers
│   │   ├── sse/                     # SSE streams
│   │   └── middleware/
│   ├── web/                         # servidor de archivos estáticos
│   ├── config/
│   │   └── config.go                # YAML loader
│   ├── bus/                         # internal event pipeline
│   ├── health/                      # health checks
│   └── logging/                     # slog setup
├── web/                              # React app (frontend)
│   ├── src/
│   ├── public/
│   ├── package.json
│   └── vite.config.ts
├── docs/
│   ├── superpowers/
│   │   └── specs/                   # design specs (este archivo)
│   ├── testing.md                   # manual testing guide
│   └── deployment.md
├── test/
│   ├── integration/
│   │   ├── api_test.go
│   │   ├── llm_test.go
│   │   └── docker-compose.test.yml
│   └── e2e/
│       └── dashboard_test.go
├── go.mod
├── go.sum
├── Makefile                          # build, test, cross-compile
├── README.md
└── .gitignore
```

### 5.4 Architecture decisions

| Tema | Decisión | Por qué |
|---|---|---|
| Concurrencia | Goroutines + channels Go para el bus interno. NO IPC en V1. | Un proceso, baja latencia, simple de debuggear. |
| Captura | Interfaz `PacketCapture` con dos impls, elegidas por build tag. | Cross-platform limpio, testeable con mock. |
| Scanner | Subprocess a `nmap` con `os/exec`, parsea XML. | Battle-tested, evita reimplementar SYN scan. |
| Persistencia | pgx v5 + golang-migrate. | Driver más rápido, migraciones reproducibles. |
| ML | ONNX Runtime Go. Modelos en `internal/ml/assets/models/`. | Local, sin nube, latencia <100ms. |
| LLM | Cliente HTTP a Ollama. Ollama corre como servicio separado. | Reusar lo que el usuario ya tiene. |
| Web UI | React build estático embebido con `go:embed`. | Un solo artefacto para deploy. |
| Config | YAML en `os.UserConfigDir()/viewred/config.yaml`. | Cross-platform, editable a mano. |
| Permisos | App corre como Admin (Win) o root (Linux). | Requerido para captura promiscua. |

---

## 6. Components

### 6.1 Capture subsystem (driver + bus)

**Interfaz**:
```go
type PacketCapture interface {
    Start(iface string, bpfFilter string) error
    Stop() error
    Stats() CaptureStats
    Packets() <-chan PacketEvent  // canal de solo lectura
}

type PacketEvent struct {
    Timestamp time.Time
    SrcMAC, DstMAC       string
    SrcIP, DstIP         string
    SrcPort, DstPort     uint16
    Protocol             string  // "TCP", "UDP", "ICMP", "OTHER"
    Length               int
    PayloadPreview       []byte  // truncado a N bytes
}

type CaptureStats struct {
    PacketsTotal   uint64
    PacketsDropped uint64
    BytesTotal     uint64
    StartedAt      time.Time
    LastError      error
}
```

**Implementaciones**:
- `npcap_windows.go` — usa gopacket con handle pcap sobre Npcap.
- `afpacket_linux.go` — usa gopacket con TPacket o AF_PACKET socket.

**Mock** (`mock_capture.go`):
- Reproduce `PacketEvent` grabados desde `testdata/`.
- Permite tests determinísticos sin tráfico real.

### 6.2 Scanner subsystem (nmap subprocess)

**Interfaz**:
```go
type Scanner interface {
    Scan(ctx context.Context, target string, profile ScanProfile) (ScanResult, error)
    Version() (string, error)  // nmap --version
}

type ScanProfile string
const (
    ScanQuick         ScanProfile = "quick"          // top 1000 ports
    ScanFull          ScanProfile = "full"           // 1-65535
    ScanServiceDetect ScanProfile = "service-detect" // -sV
)

type ScanResult struct {
    Target          string
    StartedAt       time.Time
    CompletedAt     time.Time
    Hosts           []HostResult
    Vulnerabilities []Vulnerability
}

type Vulnerability struct {
    CVE         string
    CVSS        float64
    Description string
    Port        uint16
    Service     string
}
```

**Comando ejecutado**:
```
nmap -sV --script=vulners -oX - <target>
```

**Parsing**: `encoding/xml` estándar de Go contra structs.

### 6.3 Storage layer

**Driver**: pgx v5 (pool nativo).

**Migraciones**: directorio `internal/storage/migrations/` con archivos `0001_initial.up.sql`, etc. Usamos `golang-migrate` para aplicarlas.

**Esquema inicial** (TimescaleDB hypertables donde aplique):

```sql
-- Paquetes: hypertable particionada por tiempo
CREATE TABLE packets (
    time TIMESTAMPTZ NOT NULL,
    src_ip INET NOT NULL,
    dst_ip INET NOT NULL,
    src_port INTEGER,
    dst_port INTEGER,
    protocol TEXT NOT NULL,
    length INTEGER NOT NULL,
    payload_preview BYTEA
);
SELECT create_hypertable('packets', 'time');

-- Flows: agregaciones 5-tuple por ventana
CREATE TABLE flows (
    time TIMESTAMPTZ NOT NULL,
    src_ip INET NOT NULL,
    dst_ip INET NOT NULL,
    src_port INTEGER,
    dst_port INTEGER,
    protocol TEXT NOT NULL,
    packets_count BIGINT NOT NULL,
    bytes_count BIGINT NOT NULL
);
SELECT create_hypertable('flows', 'time');

-- Devices
CREATE TABLE devices (
    id BIGSERIAL PRIMARY KEY,
    ip INET UNIQUE NOT NULL,
    mac MACADDR UNIQUE,
    vendor TEXT,
    hostname TEXT,
    first_seen TIMESTAMPTZ NOT NULL,
    last_seen TIMESTAMPTZ NOT NULL,
    risk_score REAL
);

-- Topology edges
CREATE TABLE topology_edges (
    id BIGSERIAL PRIMARY KEY,
    src_device_id BIGINT REFERENCES devices(id),
    dst_device_id BIGINT REFERENCES devices(id),
    bytes_count BIGINT NOT NULL,
    last_seen TIMESTAMPTZ NOT NULL,
    UNIQUE(src_device_id, dst_device_id)
);

-- Alerts
CREATE TABLE alerts (
    id BIGSERIAL PRIMARY KEY,
    time TIMESTAMPTZ NOT NULL,
    severity TEXT NOT NULL,        -- 'info', 'warning', 'critical'
    source TEXT NOT NULL,          -- 'ml', 'rule', 'anomaly'
    score REAL NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    context JSONB,
    acknowledged_at TIMESTAMPTZ
);

-- Vulnerabilities
CREATE TABLE vulnerabilities (
    id BIGSERIAL PRIMARY KEY,
    device_id BIGINT REFERENCES devices(id),
    cve TEXT,
    cvss REAL,
    description TEXT,
    port INTEGER,
    service TEXT,
    discovered_at TIMESTAMPTZ NOT NULL,
    scan_id UUID
);

-- LLM conversations
CREATE TABLE llm_conversations (
    id UUID PRIMARY KEY,
    title TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE llm_messages (
    id BIGSERIAL PRIMARY KEY,
    conversation_id UUID REFERENCES llm_conversations(id),
    role TEXT NOT NULL,           -- 'user' | 'assistant' | 'system'
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);
```

**Retención**:
- `packets`: default 7 días, configurable.
- `flows`: default 30 días.
- `alerts`: sin expiry en V1 (el admin las borra o archiva).
- TimescaleDB compression en particiones con más de 7 días.

### 6.4 API surface

**REST endpoints** (Gin):

| Método | Path | Descripción |
|---|---|---|
| GET | `/api/v1/health` | Health check (ver §10) |
| GET | `/api/v1/dashboard/summary` | KPIs actuales |
| GET | `/api/v1/dashboard/top-talkers` | Top N por bytes |
| GET | `/api/v1/dashboard/protocols` | Distribución de protocolos |
| GET | `/api/v1/packets` | Lista paginada, filtros query params |
| GET | `/api/v1/packets/{id}` | Detalle de un paquete |
| POST | `/api/v1/filters` | Crear filtro guardado |
| GET | `/api/v1/filters` | Listar filtros guardados |
| GET | `/api/v1/alerts` | Listar alertas, filtros y paginación |
| POST | `/api/v1/alerts/{id}/ack` | Acknowledge |
| GET | `/api/v1/topology` | Nodos y edges del grafo |
| GET | `/api/v1/devices` | Listar devices descubiertos |
| GET | `/api/v1/devices/{id}` | Detalle de un device |
| POST | `/api/v1/scan` | Disparar escaneo |
| GET | `/api/v1/scans` | Historial de escaneos |
| GET | `/api/v1/vulnerabilities` | Listar vulnerabilidades |
| POST | `/api/v1/llm/chat` | Enviar mensaje al LLM |
| GET | `/api/v1/llm/conversations` | Listar conversaciones |
| GET | `/api/v1/config` | Leer config actual |
| PATCH | `/api/v1/config` | Actualizar config |

**SSE streams**:

| Path | Qué emite | Frecuencia |
|---|---|---|
| `/api/v1/stream/metrics` | KPIs + top talkers | 1 Hz |
| `/api/v1/stream/alerts` | Alertas nuevas | push al evento |
| `/api/v1/stream/topology` | Cambios en devices/edges | push al evento |
| `/api/v1/stream/llm` | Tokens del LLM en stream | per-token |

**Versionado**: `/api/v1/...` desde el día 1.

### 6.5 AI subsystem

**ML pipeline**:
- **Anomaly detection**: Isolation Forest en ONNX. Features por ventana de 60s: bytes/s, packets/s, new_flows, syn_ratio, entropy_dst_ports. Output: score 0-1. Threshold configurable, default 0.85.
- **Classifier**: Random Forest pre-entrenado. Clasifica flows como: web, video, voip, p2p, suspicious, other. Output: label + confidence.
- **Flujo**: PacketEvent → aggregator (5-tuple, 60s window) → features → modelo → score → si score > threshold, generar alert.

**LLM client**:
```go
type LLMClient interface {
    Chat(ctx context.Context, messages []Message, opts ChatOptions) (<-chan string, error)
    IsAvailable(ctx context.Context) bool
}
```
- HTTP a Ollama `http://localhost:11434/api/chat` con `stream=true`.
- Configurable: URL, model, temperature, system prompt.
- Validación de output cuando se pide JSON (re-ask hasta 2 veces, después error).
- Streaming opcional vía canal interno → SSE del backend.

### 6.6 Web serving

- Build de React (`pnpm build`) → carpeta `web/dist/`.
- En `main.go`: `//go:embed web/dist`, servido por Gin en `/`.
- **Dev hot-reload**: en modo dev, Vite sirve en `:5173` y el backend configura CORS para `localhost:5173`.
- **Producción**: el backend sirve todo, no hay CORS, no hay Vite corriendo.

### 6.7 Config & lifecycle

**Config** (YAML):
```yaml
server:
  bind_address: "127.0.0.1"
  port: 8080

capture:
  interface: "auto"          # auto = detecta, o nombre explícito
  bpf_filter: ""             # vacío = todo
  snaplen: 65535
  promiscuous: true

storage:
  connection_string: "postgres://viewred:password@localhost:5432/viewred"
  retention_days: 7
  retention_flows_days: 30

ai:
  ollama_url: "http://localhost:11434"
  ollama_model: "qwen2.5:3b"
  ml_anomaly_threshold: 0.85
  llm_temperature: 0.7
  llm_system_prompt: "..."

scanner:
  nmap_path: ""              # vacío = usar PATH
  max_concurrent_scans: 5
  scan_timeout_seconds: 600
```

**Lifecycle**:
```
Startup:
  1. Cargar config (crear default si no existe)
  2. Setup logging
  3. Health checks: DB, Ollama, nmap, permisos
  4. Si algo crítico falta → exit con mensaje claro
  5. Conectar DB, correr migraciones pendientes
  6. Cargar modelos ONNX
  7. Iniciar goroutines: capture, aggregator, ml, scanner runner, sse broadcaster
  8. Arrancar API server
  9. Marcar "ready"

Shutdown (SIGINT/SIGTERM):
  1. Stop accepting new requests (API)
  2. Stop capture (5s timeout, force kill después)
  3. Flush DB writes (10s timeout)
  4. Close SSE clients con "server shutting down"
  5. Close DB pool
  6. Exit 0
```

---

## 7. Data flow

### 7.1 Main flow: packet → DB → UI

```
Red
  │
  ▼
[capture driver]
  │
  │ PacketEvent
  ▼
[internal bus]
  │
  ├──► [DB writer]        (goroutine, batched inserts de 500 packets o 1s)
  │         │
  │         ▼
  │     PostgreSQL/TimescaleDB
  │
  ├──► [Aggregator]       (goroutine, ventanas 60s)
  │         │
  │         ▼
  │     features (bytes/s, new flows, entropy)
  │         │
  │         ▼
  │     [ML sampler]      (muestreo 1:10)
  │         │
  │         ▼
  │     ONNX inference (Isolation Forest + Random Forest)
  │         │
  │         ▼
  │     score + label
  │         │
  │         ▼
  │     [Alert engine]   (umbral + reglas)
  │         │
  │         ▼
  │     INSERT alerts
  │
  └──► [SSE broadcaster]
            │
            ▼
        SSE clients (UI)
```

**Throttling**:
- DB writer: batch de 500 packets o 1s.
- Aggregator: ventana fija de 60s.
- ML sampler: 1 de cada N (configurable, default 10).
- SSE: 1 update/seg por componente en UI.

### 7.2 Alert flow

```
[Alert engine] → INSERT en `alerts` → SSE push a clientes suscritos
                                              │
                                              ▼
                                        UI: nueva card al tope
                                            (animación de entrada)
```

Cada alerta incluye: severity, source (ml/rule/anomaly), score, title, description, context JSONB.

### 7.3 Scan flow

```
[UI: botón "Escanear red"] 
       │
       ▼
POST /api/v1/scan { target, profile }
       │
       ▼
[Scanner scheduler]   (cola, rate limit N hosts concurrentes)
       │
       ▼
[Nmap runner]
   $ nmap -sV --script=vulners -oX - <target>
       │
       ▼
[XML parser]
       │
       ▼
INSERT en `vulnerabilities` + `devices`
       │
       ▼
UI: refresh en panel
```

**Perfiles**: `quick`, `full`, `service-detect`. **Rate limit**: 5 concurrentes (config).

### 7.4 LLM chat flow

```
[UI: chat input]
       │
       ▼
POST /api/v1/llm/chat { message, conversation_id }
       │
       ▼
[LLM client] construye contexto:
  - system prompt fijo
  - resumen alertas recientes
  - top traffic actual
  - devices activos
  - historial conversación
       │
       ▼
Ollama /api/chat (stream=true)
       │
       │  token
       ▼
SSE /api/v1/stream/llm
       │
       ▼
UI renderiza tokens incremental
       │
       ▼
(al final) Persistir turno en `llm_messages`
```

- **Memoria**: dentro de la conversación sí. Entre conversaciones no en V1.
- **Timeout**: 30s por respuesta, después error + opción de reintentar.

### 7.5 Backpressure matrix

| Punto | Qué pasa si se tensa | Mitigación |
|---|---|---|
| Captura más rápida que DB | Channel buffer lleno | Drop circular + contador en stats |
| ML inferencia lenta | Acumula features | Worker pool, descartando features viejas |
| SSE clients lentos | Buffers crecen | Ring buffer por cliente, drop oldest |
| Ollama caído | Timeouts | Circuit breaker, UI muestra "IA no disponible" |
| nmap no instalado | Subprocess error | Status al startup, scan deshabilitado en UI |

---

## 8. Cross-platform strategy

### 8.1 Build matrix

| Output | Target | Cómo |
|---|---|---|
| `viewred-windows-amd64.exe` | Windows 10/11, Server | `GOOS=windows GOARCH=amd64 go build` |
| `viewred-linux-amd64` | Ubuntu 22.04+, Debian 12+, RHEL 9+ | `GOOS=linux GOARCH=amd64 go build` |

**Makefile targets**:
- `make build` — build del OS actual.
- `make build-all` — cross-compile ambos.
- `make test` — tests del OS actual.
- `make dev` — arranca backend + Vite con hot-reload.
- `make lint` — linters.

### 8.2 Build tags

Convención de sufijos de Go:
- `*_windows.go` → solo Windows.
- `*_linux.go` → solo Linux.
- `*_test.go` → archivo de test (en ambos OS).

Ejemplo: `internal/capture/npcap_windows.go` se compila solo en Windows automáticamente.

### 8.3 Paths y config

Usar SIEMPRE `path/filepath` y helpers del stdlib:

| Recurso | Windows | Linux |
|---|---|---|
| Config | `%APPDATA%\viewred\config.yaml` | `~/.config/viewred/config.yaml` |
| DB data | `%APPDATA%\viewred\data\` | `~/.local/share/viewred/data/` |
| Cache | `%LOCALAPPDATA%\viewred\cache\` | `~/.cache/viewred/` |
| Logs | `%APPDATA%\viewred\logs\` | `~/.local/share/viewred/logs/` |

### 8.4 Permisos

| OS | Requerido | Detección | Acción si falta |
|---|---|---|---|
| Windows | Admin (UAC elevated) | `windows.IsCurrentProcessElevated()` | Mensaje: "Ejecutar como Administrador" |
| Linux | root o `CAP_NET_RAW` | `os.Geteuid() == 0` | Mensaje: "Ejecutar como root o con capabilities" |

### 8.5 External dependencies: detección al startup

```
✓ PostgreSQL+TimescaleDB   reachable at localhost:5432
✓ Ollama                   reachable at http://localhost:11434
✓ Nmap                     nmap 7.94 found at /usr/bin/nmap
✓ Npcap (Windows)          Npcap driver version 1.78 detected
✓ Permission level         running as Administrator / root
```

Si falta algo, mensaje accionable con link de descarga o comando sugerido.

### 8.6 Installer strategy (layered)

- **Capa 1**: installer nativo (MSI / deb / rpm). Tamaño ~30 MB. Instala binario + service/daemon wrapper.
- **Capa 2**: first-run wizard en la app. Detecta deps, ofrece instalarlas, configura conexión.
- **Capa 3**: docker-compose alternativo (`.yml` provisto). Para usuarios con Docker.
- **Capa 4**: detección continua. Si algo se cae, lo muestra en UI.

**Lo que NO se bundlea**: PostgreSQL, Ollama, modelos LLM, Npcap. Por licencia, tamaño o mantenimiento.

### 8.7 Testing por OS

- Unit tests del código común: ambos OS.
- Tests OS-específicos: build tag explícito.
- Integration: docker-compose con Postgres+TimescaleDB.
- Manual: `tcpreplay` para reproducir capturas, documentado en `docs/testing.md`.

---

## 9. Error handling

### 9.1 Filosofía

> El subsistema de captura NUNCA debe crashear la app. La UI siempre responde. El usuario se entera de los problemas.

| Nivel | Quién lo ve | Cómo se comunica |
|---|---|---|
| DEBUG | Devs | Solo en logs |
| INFO | Operador curioso | Panel de logs en UI |
| WARN | Admin | Toast no bloqueante + log |
| ERROR | Admin | Banner persistente + log + acción |
| CRITICAL | Admin | Modal bloqueante + requiere acción |

### 9.2 Por subsistema

| Subsistema | Error | Respuesta |
|---|---|---|
| **Captura** | Interface cae | Reconnect cada 5s con backoff, banner en UI |
| | Permiso perdido | Parar captura, modal con instrucciones |
| | Packet drops | Contador en stats, surface en dashboard |
| | Driver error | Retry con backoff exponencial (max 30s) |
| **DB** | Conexión perdida | Buffer en memoria (cap 10k), reconnect, drop oldest si lleno |
| | Query falló | Log ERROR, 500 al cliente |
| | Disco lleno | CRITICAL, parar captura, banner con instrucción |
| | Migración falló al startup | Exit con código 1, mensaje claro |
| **AI** | Ollama caído | Circuit breaker después de 3 fallos, UI muestra "IA no disponible" |
| | Modelo no descargado | Ofrecer `ollama pull` automático |
| | ONNX runtime fail al cargar | ML deshabilitado, captura sigue, banner |
| | Inference timeout | Descartar resultado, WARN |
| | Output malformado | Re-ask hasta 2 veces, después error |
| **Scanner** | nmap no encontrado | Status, banner, scan deshabilitado en UI |
| | Scan timeout | Kill subprocess, ERROR, devolver error a UI |
| | Parse XML falló | Log con XML raw, mensaje en UI |
| **API/SSE** | Cliente HTTP inválido (4xx) | Mensaje claro, sin filtrar internals |
| | Error interno (5xx) | Log con request_id, mensaje genérico al cliente |
| | SSE client desconectó | Cleanup automático |
| | SSE client lento | Ring buffer, drop oldest |
| **UI** | API call fail | Toast, "Reintentar" |
| | SSE desconectado | Auto-reconnect con backoff, indicador en header |
| | Failed initial load | Página de error, diagnóstico + "Reintentar" |

### 9.3 Graceful shutdown

```
SIGINT/SIGTERM
  → cancel context
  → stop capture (5s timeout, force kill)
  → flush DB writes (10s timeout)
  → close SSE clients con "server shutting down"
  → close DB pool
  → exit 0
```

### 9.4 Logging

- `log/slog` (stdlib) structured logging.
- Output: archivo rotativo + stderr. JSON en prod, text en dev.
- Rotación: diaria o 100 MB, retención 14 días.
- Cada línea: timestamp, level, subsistema, request_id, mensaje, contexto.

### 9.5 Health check endpoint

`GET /api/v1/health`:
```json
{
  "status": "ok",
  "uptime_seconds": 1234,
  "subsystems": {
    "capture": { "status": "ok", "packets_total": 12345, "drops": 0 },
    "database": { "status": "ok", "latency_ms": 2 },
    "ml": { "status": "ok", "last_inference_at": "..." },
    "llm": { "status": "ok", "model": "qwen2.5:3b" },
    "scanner": { "status": "ok", "nmap_version": "7.94" }
  }
}
```

---

## 10. Testing strategy

### 10.1 Disciplina: TDD (RED-GREEN-REFACTOR)

Cada línea de código de producción tiene un test escrito antes. No negociable, alineado con la skill `test-driven-development` de Superpowers.

```
1. Escribir test que falla (RED)
2. Verificar que falla por la razón correcta
3. Escribir código mínimo para pasar (GREEN)
4. Refactorizar manteniendo tests verdes (REFACTOR)
5. Commit atómico
```

### 10.2 Pirámide

```
        ┌───────────────────┐
        │   E2E (pocos)     │  flujo completo
        ├───────────────────┤
        │  Integración      │  DB real, deps mockeadas
        │  (algunos)        │
        ├───────────────────┤
        │   Unit (muchos)   │  lógica pura, mocks
        └───────────────────┘
```

### 10.3 Por capa

| Capa | Tipo | Velocidad |
|---|---|---|
| Parsers (packet, nmap XML, YAML) | Unit | <1s |
| Capture interface + lógica común | Unit | <1s |
| Capture driver (Npcap/AF_PACKET) | Unit + Integration | <5s |
| Storage layer | Integration (DB real) | <30s |
| ML pipeline | Unit (modelo dummy) | <5s |
| LLM client | Unit (mock Ollama) | <5s |
| Scanner wrapper | Unit (mock exec) | <5s |
| API REST | Integration (httptest) | <30s |
| SSE streams | Integration | <10s |
| Web UI críticos | Component (Vitest) | <30s |
| E2E happy path | E2E (Playwright) | <2min |

### 10.4 Coverage targets

| Capa | Target |
|---|---|
| Lógica de negocio core | 85% |
| Código de seguridad | 100% |
| API REST handlers | 80% |
| LLM client + scanner | 75% |
| Web UI componentes | 60% |
| **Global** | **80%** mínimo |

### 10.5 Naming

`TestSubject_Scenario_ExpectedBehavior`:
- `TestPacketParser_MalformedInput_ReturnsError`
- `TestAlertEngine_ScoreAboveThreshold_CreatesAlert`
- `TestNpcapDriver_PermissionDenied_StartsWithoutCrashing`

### 10.6 CI

- PRs: unit + integration con DB Docker. Falla si coverage < target.
- Main: todo + e2e nightly.
- Benchmarks: semanales, detección de regresiones con `benchstat`.

### 10.7 Performance benchmarks

```go
func BenchmarkCapture_PacketParsing(b *testing.B) { ... }   // 1 Mpps
func BenchmarkStorage_BatchInsert(b *testing.B) { ... }      // 10k pkts/s
func BenchmarkML_InferenceLatency(b *testing.B) { ... }      // <100ms
```

Falla CI si performance cae >20% vs baseline.

---

## 11. Open questions (to address during planning, not blocking)

- **First thing to build**: recomendación es `bootstrap-monorepo` (estructura del proyecto, CI básico, primer test smoke). TBD en `writing-plans`.
- **Reposicionamiento de `que es viewred.md`**: ese doc es la definición de producto, este spec es el diseño técnico. Ambos viven. No hay que borrar nada.
- **Git init**: el proyecto no es repo todavía. Recomendación: `git init` antes de empezar a tirar código, conventional commits.
- **Conventional commits**: mensajes estandarizados (feat, fix, test, refactor, docs).
- **CHANGELOG**: a partir del primer cambio.

---

## 12. Changelog

- **v0.1 (2026-06-17, borrador inicial)**: Primera versión del diseño técnico consolidada de la sesión de brainstorming. Target 20-100 devices, cross-platform Windows+Linux, 8 features, arquitectura A1 monolith con build tags, TDD-first, layered installer.
