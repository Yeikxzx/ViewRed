# Spec: Core API and UI Shell (Plan 3 of 8)

> **Date**: 2026-06-18
> **Plan**: `core-api-and-ui-shell`
> **Base commit**: `296f904` (Plan 2 merged)
> **Context**: Proposal at `docs/superpowers/proposals/2026-06-18-core-api-and-ui-shell.md`. Full design at `docs/superpowers/specs/2026-06-17-viewred-v1-design.md`.
> **User decisions locked**: API scope (7 real + 5 stubs), React Router v7, Zustand, no shadcn/ui (custom), real errors when DB down, SSE only for `/stream/metrics`.

---

## 1. Middleware

### 1.1 CORS

**Intent**: Allow the Vite dev server (`localhost:5173`) to call the API during development. In production the backend serves the built frontend from the same origin, so CORS is only needed for the dev proxy fallback.

**Requirements**:
- CORS middleware MUST allow `Origin: http://localhost:5173` (configurable via `Config.Server.CORSOrigins`).
- Allowed methods MUST include `GET`, `POST`, `OPTIONS`.
- Allowed headers MUST include `Content-Type`, `Authorization`.
- Preflight (`OPTIONS`) requests MUST return `204 No Content` without hitting route handlers.
- Credentials mode MUST be set to `true` for cookie-based dev auth.

**Contract**:
```
NewCORSMiddleware(origins []string) gin.HandlerFunc
```

**Scenarios**:

| Scenario | GIVEN | WHEN | THEN |
|----------|-------|------|------|
| Dev origin allowed | Request from `Origin: http://localhost:5173` to `GET /api/v1/health` | Middleware runs | Response has `Access-Control-Allow-Origin: http://localhost:5173` |
| Preflight handled | `OPTIONS /api/v1/health` with `Origin: http://localhost:5173` | CORS middleware runs | `204 No Content` with CORS headers |
| Unknown origin rejected | Request from `Origin: https://evil.com` | Middleware runs | No `Access-Control-Allow-Origin` header, request proceeds (same-origin behavior) |

---

### 1.2 Request Logging

**Intent**: Every HTTP request produces a structured log line for debugging, auditing, and performance monitoring.

**Requirements**:
- The middleware MUST log: `method`, `path`, `status`, `duration_ms`, `request_id`, `user_agent`, `remote_addr`.
- Duration MUST be measured as wall-clock time between request start and response write.
- Logging MUST use `log/slog` at `INFO` level for successful requests, `WARN` for 4xx, `ERROR` for 5xx.
- Log output MUST be a single line per request (not split across start/end).

**Contract**:
```
RequestLoggingMiddleware() gin.HandlerFunc
```

**Scenarios**:

| Scenario | GIVEN | WHEN | THEN |
|----------|-------|------|------|
| Successful request | `GET /api/v1/health` returns 200 | Request completes | Log line contains `method=GET path=/api/v1/health status=200 duration_ms=<N> level=INFO` |
| Client error | `GET /api/v1/packets/abc` returns 400 | Request completes | Log line at `WARN` level with status=400 |
| Server error | DB down, `GET /api/v1/dashboard/summary` returns 503 | Request completes | Log line at `ERROR` level with status=503 |

---

### 1.3 Request ID

**Intent**: Each request gets a unique traceable ID propagated through the system for correlation across logs.

**Requirements**:
- The middleware MUST generate a `request_id` for every incoming request that does not already carry an `X-Request-Id` header.
- `request_id` MUST be a v4 UUID string.
- The ID MUST be set on the Gin context and on the response header `X-Request-Id`.
- Downstream handlers and logs MUST access it via `c.GetString("request_id")`.

**Contract**:
```
RequestIDMiddleware() gin.HandlerFunc
c.GetString("request_id") → string
```

**Scenarios**:

| Scenario | GIVEN | WHEN | THEN |
|----------|-------|------|------|
| No incoming ID | Request without `X-Request-Id` | Middleware runs | Response has `X-Request-Id` with valid UUID v4 |
| Incoming ID propagated | Request with `X-Request-Id: abc-123` | Middleware runs | Response `X-Request-Id` matches `abc-123` |
| ID accessible in handler | Middleware ran | Handler calls `c.GetString("request_id")` | Returns non-empty string matching response header |

---

## 2. Storage Querier Interface

**Intent**: Define an interface that the API handlers depend on, so handlers are testable without a real database (mock implementation for unit tests, real implementation in production).

### Interface

```go
type Querier interface {
    QueryDashboardSummary(ctx context.Context, since time.Time) (DashboardSummary, error)
    QueryTopTalkers(ctx context.Context, since time.Time, limit int) ([]TopTalker, error)
    QueryProtocolDistribution(ctx context.Context, since time.Time) ([]ProtocolShare, error)
    QueryPackets(ctx context.Context, filter PacketFilter) ([]PacketEvent, int, error)
    QueryDevices(ctx context.Context, filter DeviceFilter) ([]Device, int, error)
}
```

**Requirements**:
- `Querier` MUST be defined in `internal/storage/querier.go`.
- All methods MUST accept `context.Context` as first parameter for cancellation/deadline propagation.
- Return types MUST be exported structs with JSON tags.
- All methods MUST return an error when the DB is unreachable (wrapping the underlying pgx error).
- `QueryPackets` and `QueryDevices` MUST return `(results, totalCount, error)` for pagination metadata.

### Return types

```go
type DashboardSummary struct {
    PacketsPerSec     float64 `json:"packets_per_sec"`
    BytesPerSec       float64 `json:"bytes_per_sec"`
    ActiveConnections int     `json:"active_connections"`
    DeviceCount       int     `json:"device_count"`
}

type TopTalker struct {
    SrcIP    string  `json:"src_ip"`
    Bytes    int64   `json:"bytes"`
    Packets  int64   `json:"packets"`
}

type ProtocolShare struct {
    Protocol   string  `json:"protocol"`
    Percentage float64 `json:"percentage"`
    Bytes      int64   `json:"bytes"`
}

type PacketFilter struct {
    Since    time.Time `json:"since,omitempty"`
    Until    time.Time `json:"until,omitempty"`
    Protocol string    `json:"protocol,omitempty"`
    SrcIP    string    `json:"src_ip,omitempty"`
    Limit    int       `json:"limit,omitempty"`
    Offset   int       `json:"offset,omitempty"`
}

type DeviceFilter struct {
    Limit  int    `json:"limit,omitempty"`
    Offset int    `json:"offset,omitempty"`
    Status string `json:"status,omitempty"`
}
```

### Query 1: `QueryDashboardSummary`

**SQL skeleton**:
```sql
SELECT
    COUNT(*) / EXTRACT(EPOCH FROM ...) as packets_per_sec,
    COALESCE(SUM(length), 0) / EXTRACT(EPOCH FROM ...) as bytes_per_sec,
    COUNT(DISTINCT CONCAT(src_ip, dst_ip, src_port, dst_port, protocol)) as active_connections,
    COUNT(DISTINCT src_ip) + COUNT(DISTINCT dst_ip) as device_count
FROM packets WHERE time >= $1
```

**Requirements**:
- If no rows exist in the time window, MUST return zero-valued `DashboardSummary` (not an error).
- `since` MUST default to 60 seconds ago if nil/zero.

### Query 2: `QueryTopTalkers`

**SQL skeleton**:
```sql
SELECT src_ip, SUM(bytes) as bytes, COUNT(*) as packets
FROM packets WHERE time >= $1
GROUP BY src_ip
ORDER BY bytes DESC
LIMIT $2
```

**Requirements**:
- `limit` MUST cap at 100 (enforce in code, not just SQL).
- Returns empty slice (not nil) when no data in window.

### Query 3: `QueryProtocolDistribution`

**SQL skeleton**:
```sql
SELECT protocol, SUM(length) as bytes,
       SUM(length) * 100.0 / NULLIF(SUM(SUM(length)) OVER (), 0) as percentage
FROM packets WHERE time >= $1
GROUP BY protocol
ORDER BY bytes DESC
```

**Requirements**:
- Percentages MUST sum to ~100% (floating point rounding allowed).
- Returns empty slice when no data.

### Query 4: `QueryPackets`

**SQL skeleton**:
```sql
SELECT time, src_ip, dst_ip, src_port, dst_port, protocol, length, payload_preview
FROM packets
WHERE time >= $1 AND time <= $2
  AND ($3 = '' OR protocol = $3)
  AND ($4 = '' OR src_ip = $4)
ORDER BY time DESC
LIMIT $5 OFFSET $6
```

**Returns**: `(events, totalCount, error)`. `totalCount` is the count without LIMIT/OFFSET.

**Requirements**:
- `PacketFilter.Limit` MUST default to 50.
- `PacketFilter.Limit` MUST cap at 500.
- `totalCount` MUST be obtained via `SELECT COUNT(*)` with the same WHERE clause.

### Query 5: `QueryDevices`

**SQL skeleton**:
```sql
SELECT id, ip, mac, COALESCE(vendor, ''), COALESCE(hostname, ''),
       first_seen, last_seen, COALESCE(risk_score, 0)
FROM devices
ORDER BY last_seen DESC
LIMIT $1 OFFSET $2
```

**Returns**: `(devices, totalCount, error)`.

### Mock Querier

**Requirements**:
- A `MockQuerier` struct MUST exist for handler tests with overridable function fields (one per method).
- Default implementations MUST return empty/zero data (not nil, not error).
- Tests MUST set function fields to control return values.

### Common error scenarios (all queries)

| Scenario | GIVEN | WHEN | THEN |
|----------|-------|------|------|
| DB unreachable | Connection pool returns error | Query method called | Returns wrapped error, handler maps to 503 |
| Context cancelled | Parent context cancelled mid-query | Query method called | Returns `context.Canceled` error |
| Invalid filter | `limit` = -1 | Query method called | Returns validation error (400 in handler) |
| Empty result set | No rows match filter | Query method called | Returns empty slice, nil error |

---

## 3. REST Endpoints

### 3.1 `GET /api/v1/dashboard/summary`

**Intent**: Return live KPI snapshot for the dashboard header cards.

**Handler contract**:
```
DashboardSummaryHandler(querier storage.Querier) gin.HandlerFunc
```

**Query params**: `since` (ISO 8601, optional, default 60s ago).

**Response**: `200 OK` with `DashboardSummary` JSON body.

**Error cases**:
| Condition | HTTP Status | Body |
|-----------|-------------|------|
| DB unreachable | 503 | `{"error":"service_unavailable","message":"..."}` |
| Invalid `since` format | 400 | `{"error":"bad_request","message":"invalid since format"}` |

**Scenarios**:

| Scenario | GIVEN | WHEN | THEN |
|----------|-------|------|------|
| Happy path | DB has data in last 60s | GET `/api/v1/dashboard/summary` | 200 with `packets_per_sec`, `bytes_per_sec`, `active_connections`, `device_count` |
| Empty data | No packets in last 60s | GET `/api/v1/dashboard/summary` | 200 with all fields zero |
| DB down | Storage pool returns error | GET `/api/v1/dashboard/summary` | 503 with error message (no mock data) |

---

### 3.2 `GET /api/v1/dashboard/top-talkers`

**Intent**: Return top N source IPs by total bytes for the dashboard's Top Talkers widget.

**Handler contract**:
```
TopTalkersHandler(querier storage.Querier) gin.HandlerFunc
```

**Query params**: `since` (ISO 8601, optional, default 60s), `limit` (int, optional, default 10, max 100).

**Response**: `200 OK` with `{"top_talkers": [...]}`.

**Error cases**: Same as summary (503 for DB, 400 for invalid params).

---

### 3.3 `GET /api/v1/dashboard/protocols`

**Intent**: Return protocol distribution percentages for the dashboard donut chart.

**Handler contract**:
```
ProtocolDistributionHandler(querier storage.Querier) gin.HandlerFunc
```

**Query params**: `since` (ISO 8601, optional, default 60s).

**Response**: `200 OK` with `{"protocols": [{"protocol":"TCP","percentage":45.2,"bytes":12345}, ...]}`.

---

### 3.4 `GET /api/v1/packets`

**Intent**: Return paginated, filterable packet list for the packet search view.

**Handler contract**:
```
ListPacketsHandler(querier storage.Querier) gin.HandlerFunc
```

**Query params**: `since`, `until`, `protocol`, `src_ip`, `limit` (default 50, max 500), `offset` (default 0).

**Response**: `200 OK` with:
```json
{
  "packets": [...],
  "total": 1234,
  "limit": 50,
  "offset": 0
}
```

**Scenarios**:

| Scenario | GIVEN | WHEN | THEN |
|----------|-------|------|------|
| Happy path | Packets exist | GET `/api/v1/packets?limit=10&offset=0` | 200 with 10 packets + total count |
| Empty filter | No packets match | GET `/api/v1/packets?protocol=UNKNOWN` | 200 with empty `packets` array, total=0 |
| Invalid limit | limit=-1 | GET `/api/v1/packets?limit=-1` | 400, error message |
| Offset beyond total | offset=999999 | GET `/api/v1/packets?offset=999999` | 200 with empty `packets` array, total=... |

---

### 3.5 `GET /api/v1/packets/:id`

**Intent**: Return a single packet by its database ID.

**Handler contract**:
```
GetPacketHandler(querier storage.Querier) gin.HandlerFunc
```

**Response**: `200 OK` with full `PacketEvent` JSON.

**Error cases**:
| Condition | HTTP Status | Body |
|-----------|-------------|------|
| Not found | 404 | `{"error":"not_found","message":"packet not found"}` |
| Invalid ID (non-numeric) | 400 | `{"error":"bad_request","message":"invalid packet id"}` |

---

### 3.6 `GET /api/v1/devices`

**Intent**: Return paginated list of known devices.

**Handler contract**:
```
ListDevicesHandler(querier storage.Querier) gin.HandlerFunc
```

**Query params**: `limit` (default 50, max 500), `offset` (default 0).

**Response**: `200 OK` with `{"devices": [...], "total": N, "limit": N, "offset": N}`.

---

### 3.7 `GET /api/v1/devices/:id`

**Intent**: Return a single device by primary key.

**Handler contract**:
```
GetDeviceHandler(querier storage.Querier) gin.HandlerFunc
```

**Response**: `200 OK` with full `Device` JSON.

**Error cases**: 404 for not found, 400 for invalid ID.

---

### 3.8 Stub endpoints (5 routes)

**Intent**: Register routes so the frontend never gets 404 for planned-but-not-yet-implemented endpoints.

| Route | Method | Handler name | Response |
|-------|--------|-------------|----------|
| `/api/v1/alerts` | GET | `ListAlertsStub` | `200 {"alerts":[]}` |
| `/api/v1/alerts/:id/ack` | POST | `AckAlertStub` | `200 {"status":"acknowledged"}` |
| `/api/v1/topology` | GET | `GetTopologyStub` | `200 {"nodes":[],"edges":[]}` |
| `/api/v1/vulnerabilities` | GET | `ListVulnerabilitiesStub` | `200 {"vulnerabilities":[]}` |
| `/api/v1/config` | GET | `GetConfigStub` | `200` with current config (read-only, from cfg param) |

**Handler contract**:
```
NewStubHandler(responseBody any) gin.HandlerFunc
```

All stubs MUST return 200 with empty/static content. No DB calls.

---

## 4. SSE Broker

**Intent**: Manage connected SSE clients, allowing topic-based subscriptions with ring buffer backpressure and heartbeat.

### Broker struct

```go
type Broker struct {
    clients   map[string]*client
    mu        sync.RWMutex
    heartbeat time.Duration
    bufSize   int
}

type Event struct {
    Name string      // "metrics", "alert", "topology", "llm"
    Data []byte      // JSON-encoded payload
}

type client struct {
    id      string
    ch      chan Event
    topics  map[string]bool
    lastPing time.Time
}
```

### Requirements

**Lifecycle**:
- `NewBroker(opts ...BrokerOption) *Broker` — creates broker (heartbeat default 30s, buffer 64 events).
- `Run(ctx context.Context)` — starts the heartbeat goroutine that removes stale clients (no recent ping + no buffer space).
- `Shutdown(ctx context.Context)` — closes all client channels with a graceful message.

**Subscription**:
- `Subscribe(ctx context.Context, topics ...string) (<-chan Event, func())` — registers a client buffered channel. Returns the read-only channel + an unregister function (caller MUST call on disconnect).
- Client receives events for ALL matching topics.

**Broadcast**:
- `Broadcast(ctx context.Context, topic string, event Event) error` — sends to all subscribed clients. If a client's buffer is full, drops the oldest event (ring buffer behavior via channel select with default).
- Returns error only if ctx is cancelled (not for individual client drops).

**Heartbeat**:
- Every `heartbeat` interval, broker sends `event: heartbeat\ndata: {}\n\n` to all clients.
- Clients that have not had a channel write in 3 heartbeat intervals are removed.

**Backpressure**:
- Each client has a buffered channel of `bufSize` (default 64).
- When `Broadcast` sends and channel is full, use `select` with `default` to drop the event.
- Client goroutine reads from channel and writes to HTTP `http.Flusher`.

**Scenarios**:

| Scenario | GIVEN | WHEN | THEN |
|----------|-------|------|------|
| Client subscribes and receives | Client calls `Subscribe("metrics")` | Broker broadcasts metrics event | Client channel receives the event |
| Slow client drops | Client buffer is full (64 events), broker broadcasts event #65 | Broadcast sends | Oldest event is dropped, client still connected |
| Client disconnects | Client receives unregister func from Subscribe | Unregister called | Client removed from broker, goroutine cleaned up |
| Heartbeat sent | Broker running with 30s heartbeat | 30s elapses | All clients receive heartbeat event |
| Stale client removed | Client not consuming events for >90s | Heartbeat goroutine runs | Client unregistered, goroutine released |

---

## 5. SSE Stream: `/api/v1/stream/metrics`

**Intent**: Provide a 1 Hz real-time stream of dashboard KPIs for the frontend's live dashboard.

**Handler contract**:
```
StreamMetricsHandler(broker *sse.Broker, querier storage.Querier) gin.HandlerFunc
```

**Protocol**:
- Response MUST set headers: `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`.
- Client MUST use `http.Flusher` to flush each event.
- Each event format: `event: metrics\ndata: {...}\n\n`

**Ticker**: 1 Hz (1 second interval). Uses `time.Ticker`.

**Initial event**: On connect, immediately query storage and emit a snapshot before starting the ticker.

**Event emission**:
1. Query storage for `DashboardSummary` via `QueryDashboardSummary(ctx, 60s ago)`.
2. If query succeeds → serialize to JSON → emit `event: metrics\ndata: <json>\n\n`.
3. If query fails → emit `event: error\ndata: {"error":"...","recoverable":true}\n\n` and continue ticker (do NOT disconnect).

**Disconnect**:
- When client closes connection (request context done), call the unregister function from broker subscription.
- The handler goroutine exits cleanly.

**Scenarios**:

| Scenario | GIVEN | WHEN | THEN |
|----------|-------|------|------|
| Client connects | Client opens SSE connection | Handler runs | Receives initial event with current metrics, then 1 Hz stream |
| Storage query fails mid-stream | DB becomes unavailable, next tick query fails | Ticker fires | Emits error event, continues next tick |
| Client disconnects | Client closes connection | Request context cancelled | Handler calls unregister, goroutine exits |
| Client full buffer | Client not reading events for >10s | Buffer fills to 64 | Broker drops oldest events, client stays connected until heartbeat fires stale removal |

---

## 6. Router Refactor

**Intent**: Add a `NewRouterWithDeps` constructor that accepts all service dependencies, replacing the pattern where each handler is wired individually.

### Contract

```go
type RouterDeps struct {
    Health  *health.Registry
    Querier storage.Querier
    Broker  *sse.Broker
    Config  *config.Config
}

func NewRouterWithDeps(deps RouterDeps) *gin.Engine
```

**Requirements**:
- `NewRouterWithDeps` MUST apply middleware in this order: `gin.Recovery()` → CORS → Request ID → Request Logging → routes.
- Static file serving (`web/dist/`) MUST be preserved from the existing `NewRouterWithRegistry` pattern.
- `NewRouterWithRegistry(rg *health.Registry)` MUST be preserved as a back-compat alias (creates a RouterDeps with only Health set, leaves Querier/Broker/Config as nil, and handlers check for nil to return 501).
- `NewRouter()` MUST be preserved as a back-compat alias that calls `NewRouterWithRegistry(health.New())`.

### Route registration order

```
/api/v1/
├── health                  (GET)
├── dashboard/summary       (GET)
├── dashboard/top-talkers   (GET)
├── dashboard/protocols     (GET)
├── packets                 (GET)
├── packets/:id             (GET)
├── devices                 (GET)
├── devices/:id             (GET)
├── alerts                  (GET, stub)
├── alerts/:id/ack          (POST, stub)
├── topology                (GET, stub)
├── vulnerabilities         (GET, stub)
├── config                  (GET, stub)
├── stream/metrics          (GET, SSE)
├── stream/alerts           (GET, SSE, stub, no broadcast)
├── stream/topology         (GET, SSE, stub, no broadcast)
└── stream/llm              (GET, SSE, stub, no broadcast)
```

**Scenarios**:

| Scenario | GIVEN | WHEN | THEN |
|----------|-------|------|------|
| Full deps | All deps populated | `NewRouterWithDeps(fullDeps)` called | All 7 real + 5 stub + 4 SSE routes registered |
| Back-compat | Only health registry | `NewRouterWithRegistry(rg)` called | Only health route + static files registered, 501 for others |
| Middleware order | Any request | Request hits router | Recovery → CORS → Request ID → Logging → Handler |

---

## 7. main.go Changes

**Intent**: Wire the new SSE broker, metrics ticker, and storage querier into the router.

### Requirements

**Pre-existing** (from Plan 2, unchanged):
- Config loading
- Storage init (may be nil)
- Capture init
- Pcap writer
- Storage writer
- Health registry

**New additions**:
1. Create SSE broker: `broker := sse.NewBroker()` before health registry.
2. Start broker lifecycle: `go broker.Run(ctx)` in a goroutine.
3. Create storage querier (only if store != nil): `var querier storage.Querier = &storage.DBQuerier{Pool: store.Pool()}`. If store is nil, querier MUST be set to a `storage.ErrorQuerier` that returns 503 for all methods.
4. Create `NewRouterWithDeps` with all deps instead of `NewRouterWithRegistry`.
5. Broker shutdown: add broker to `shutdown.Params` (extend the struct or add a new interface).

**Scenarios**:

| Scenario | GIVEN | WHEN | THEN |
|----------|-------|------|------|
| DB available | store != nil | main.go runs | Querier uses real DB, metrics stream hits storage |
| DB unavailable | store == nil | main.go runs | ErrorQuerier used, all dashboard endpoints return 503 |
| Broker shutdown | SIGINT received | shutdown.Shutdown called | Broker.Shutdown sends "server shutting down" to all SSE clients |

---

## 8. Frontend: React Router Setup

**Intent**: Replace state-based routing (`useState<ViewType>`) with URL-based routing using React Router v7.

### Route structure

| URL Path | Component | Nav name |
|----------|-----------|----------|
| `/` | Redirect to `/dashboard` | — |
| `/dashboard` | `DashboardView` | Dashboard |
| `/captura` | `CapturaView` | Captura |
| `/filtros` | `FiltrosView` | Filtros |
| `/alertas` | `AlertasView` | Alertas |
| `/topologia` | `TopologiaView` | Topología |
| `/vulnerabilidades` | `VulnerabilidadesView` | Vulnerabilidades |
| `/asistente` | `AsistenteView` | Asistente |
| `/configuracion` | `ConfiguracionView` | Configuración |
| `*` (fallback) | 404 page | — |

### Requirements
- Use `BrowserRouter` from `react-router-dom` v7.
- The `<Sidebar>` navigation MUST use `<Link>` instead of `onClick`/`setView`.
- The `<Header>` component that has view navigation MUST also use `<Link>`.
- Existing view component props (alerts, devices, etc.) MUST be lifted to a shared context or passed via Router children.
- A `<Navigate to="/dashboard" replace />` MUST handle the root `/` route.
- A catch-all route MUST render a simple "404 — Page Not Found" component.

### Migration guide (no breaking intermediate states)
1. Install `react-router-dom`.
2. Wrap `<App>` in `<BrowserRouter>` in `main.tsx`.
3. Define routes using `<Routes>` and `<Route>` replacing conditional `<DashboardView>` etc.
4. Replace `setView` calls with `useNavigate()` and `<Link>`.
5. Delete `ViewType` type if no longer referenced, or keep for backward compat.

**Scenarios**:

| Scenario | GIVEN | WHEN | THEN |
|----------|-------|------|------|
| Navigate to dashboard | User visits `/dashboard` | App renders | DashboardView is rendered |
| Redirect root | User visits `/` | Route matches | Automatically redirects to `/dashboard` |
| Navigate with sidebar | User clicks "Alertas" link in sidebar | `<Link to="/alertas">` clicked | URL changes to `/alertas`, AlertasView renders |
| 404 page | User visits `/nonexistent` | No route matches | "404 — Page Not Found" renders |
| Browser back | User navigates to `/captura` then clicks back | Browser back button pressed | Returns to previous URL and view |

---

## 9. Frontend: Zustand Store

**Intent**: Shared state store for SSE data and connection status, consumed by components via selectors.

### Store structure

```typescript
interface SSEStore {
  messages: Record<string, unknown[]>;  // topic → events
  connectionStatus: 'connecting' | 'connected' | 'disconnected' | 'reconnecting';
  lastError: string | null;
  captureStatus: 'running' | 'paused' | 'unknown';

  // Actions
  addMessage: (topic: string, data: unknown) => void;
  setConnectionStatus: (status: SSEStore['connectionStatus']) => void;
  clearMessages: (topic?: string) => void;
  setCaptureStatus: (status: SSEStore['captureStatus']) => void;
}
```

**Requirements**:
- `messages` is grouped by topic string (e.g., `"metrics"`, `"alerts"`).
- Each topic's array keeps at most 100 entries (ring buffer).
- `addMessage` appends to the topic array, trimming to 100.
- Store is exported as a singleton: `export const useSSEStore = create<SSEStore>(...)`.

**Scenarios**:

| Scenario | GIVEN | WHEN | THEN |
|----------|-------|------|------|
| Add message | Store has empty messages | `addMessage("metrics", {...})` called | `messages["metrics"]` has 1 entry |
| Trim buffer | Topic has 100 entries | `addMessage("metrics", data)` called | Array trimmed to 100, oldest removed |
| Connection status | Status is "connecting" | `setConnectionStatus("connected")` called | `connectionStatus` updates to "connected" |
| Clear topic | Topic "metrics" has 10 entries | `clearMessages("metrics")` called | `messages["metrics"]` is empty |

---

## 10. Frontend: SSE Client Hook

**Intent**: A React hook that manages an `EventSource` connection with auto-reconnect and exponential backoff.

### Hook signature

```typescript
function useSSE(topic: string): {
  lastEvent: unknown | null;
  connectionStatus: 'connecting' | 'connected' | 'disconnected' | 'reconnecting';
  error: string | null;
}
```

### Requirements
- Connects to `/api/v1/stream/{topic}` when the component mounts.
- On receiving an event, calls `addMessage(topic, parsedData)` on the Zustand store.
- On connection close (non-200 or just close), starts reconnect with exponential backoff: 1s, 2s, 4s, 8s, 16s, max 30s.
- On unmount, closes the EventSource and resets the backoff timer.
- Jitter: add +/- 500ms random to each backoff interval.
- Parses SSE event data as JSON. If parse fails, logs to console and skips.
- Reports connection status changes to the store via `setConnectionStatus`.
- Does NOT reconnect on explicit `componentWillUnmount` (clean unmount).

**Scenarios**:

| Scenario | GIVEN | WHEN | THEN |
|----------|-------|------|------|
| Connect | Component mounts with `useSSE("metrics")` | Hook runs | Creates EventSource to `/api/v1/stream/metrics` |
| Receive event | Server sends `event: metrics\ndata: {"packets_per_sec": 123}\n\n` | EventSource fires `onmessage` | Store updated via `addMessage("metrics", data)` |
| Disconnect & reconnect | Connection drops (server restart) | EventSource fires `onerror` | Backoff timer starts at 1s, reconnects after 1s |
| Exponential backoff | Reconnect fails repeatedly | 4th failure | Timer at 8s, then 16s, caps at 30s |
| Unmount cleanup | Component unmounting | Hook cleanup runs | EventSource closed, no further reconnects |

---

## 11. Frontend: Component Changes

### 11.1 DashboardView

**Intent**: Subscribe to `/api/v1/stream/metrics` and render live KPI data from the Zustand store instead of `setInterval` fakes.

**Requirements**:
- Remove the `useEffect` with `setInterval` that generates fake traffic data.
- Use `useSSE("metrics")` to receive real-time KPI updates.
- Read the latest `metrics` event from `useSSEStore(s => s.messages.metrics?.at(-1))`.
- Pass `packetsPerSec`, `bytesPerSec`, `activeConnections`, `deviceCount` to the KPI cards.
- Keep the traffic chart using local state that accumulates the last 60 data points from SSE events.
- Keep the Top Talkers section using `useSSE("metrics")` (broker includes top talkers in the event payload).
- Keep the Protocol Distribution section using `useSSE("metrics")`.
- Keep the Recent Alerts section as-is (mock data until Plan 4).

**Loading state**: When `connectionStatus === "connecting"`, show skeleton loading for KPI cards (gray pulsing rectangles matching card dimensions).

**Error state**: When `connectionStatus === "disconnected"` and `lastError !== null`, show a toast-like banner: "Conexión SSE perdida. Reintentando...".

**Empty state**: When messages exist but all KPI values are zero, show "No hay tráfico en los últimos 60s" on the dashboard.

### 11.2 CapturaView

**Intent**: Fetch capture status from the API instead of using local `useState` fakes.

**Requirements**:
- Add a `useEffect` that calls `GET /api/v1/capture/status` (or `GET /api/v1/health` and reads the capture subsystem) every 5 seconds.
- Replace local `packetsTotal`, `rate`, `capturedBytes` state with values from the health endpoint.
- The pause/resume button MUST call `POST /api/v1/capture/toggle` in future plans; for Plan 3 it can remain a local toggle with a TODO comment.
- Remove `useEffect` that fakes counter increments.

**Loading state**: Skeleton counters while API call is in-flight.

### 11.3 App.tsx

**Intent**: Refactor from state-based routing to React Router.

**Requirements**:
- Remove `useState<ViewType>` and `setView` state.
- Replace with `<Routes>` and `<Route>` components.
- Lift shared state (alerts, devices, vulnerabilities) to the App level or use Zustand.
- Pass `useNavigate` instead of `setView` callbacks to children.
- Wrap the layout (Sidebar + Header + main) in a Layout component that renders `<Outlet />`.
- Keep the `handleAcknowledgeAlert`, `handleMitigateVuln`, chat handlers — they modify local state, which is fine until these features have real API endpoints.

### 11.4 Sidebar.tsx

**Intent**: No logic change; routes use `<Link>` instead of `onClick`.

**Requirements**:
- Replace `<div onClick={() => setView('dashboard')}>` with `<Link to="/dashboard">`.
- Keep the active state highlight using `useLocation().pathname`.
- Keep the collapsed toggle, alert badge, telemetry footer unchanged.

---

## 12. Loading / Error / Empty States (Pattern)

**Intent**: Every view that consumes API data MUST handle three states gracefully: loading, error, empty.

**Pattern** (applied to DashboardView and CapturaView; other views follow as they get real data in later plans):

```typescript
type ViewState<T> =
  | { status: 'loading' }
  | { status: 'error'; message: string; retry: () => void }
  | { status: 'empty'; message: string; action?: { label: string; onClick: () => void } }
  | { status: 'data'; data: T };
```

**Requirements**:
- **Loading**: Render skeleton UI (CSS-only, no animation library). Gray placeholder rectangles matching the shape of the data they replace.
- **Error**: Render a container with the error message and a "Reintentar" (Retry) button that re-fetches/reconnects.
- **Empty**: Render a container with an explanatory message and optional CTA (e.g., "Start capture to see data").
- Error states MUST NOT render partial or stale data underneath the error message.
- Transitions between states SHOULD be smooth: loading → data (no flash); data → error (error overlay keeps data visible in background).

**Scenarios**:

| Scenario | GIVEN | WHEN | THEN |
|----------|-------|------|------|
| Loading | DashboardView mounts, SSE not yet connected | View renders | 4 skeleton KPI cards visible |
| Error with retry | SSE connection fails permanently | User clicks "Reintentar" | SSE reconnection attempted |
| Empty data | DashboardView connected to SSE, all KPI values zero | View renders | Empty state message shown (data still passes through) |
| Loading → data | SSE connects and first event arrives | View re-renders | Skeletons replaced with real KPI values |

---

## Open Questions (resolved)

| Question | Resolution |
|----------|-----------|
| Metrics ticker interval | 1 Hz, confirmed by user decision #5 |
| SS: keep original plan name | `core-api-and-ui-shell`, confirmed by proposal |
| ErrorQuerier vs nil check | Use `ErrorQuerier` that returns 503 for all methods when store is nil |
| SSE stream/stub routes: how many? | 4 registered: `/metrics` (live), `/alerts`, `/topology`, `/llm` (stubs — channels exist but never receive events) |
| Browser vs Hash router | BrowserRouter with fallback handling; production `go:embed` handles SPA fallback via Gin NoRoute |
