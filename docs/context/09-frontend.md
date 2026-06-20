# ViewRed — Frontend

## Stack

- React 18 + TypeScript
- Vite 6
- Tailwind CSS
- React Router v7
- Zustand v5
- Vitest + Testing Library + jsdom

## Rutas

| URL | Componente | Estado |
|-----|-----------|--------|
| `/` | Redirect → `/dashboard` | ✅ |
| `/dashboard` | DashboardView (SSE real) | ✅ |
| `/captura` | CapturaView (health polling) | ✅ |
| `/filtros` | FiltrosView (packets search) | 🚧 Actualizado |
| `/alertas` | AlertasView | 🚧 Actualizado |
| `/topologia` | TopologiaView (React Flow 2D) | 🚧 Actualizado |
| `/vulnerabilidades` | VulnerabilidadesView | 🚧 Actualizado |
| `/asistente` | AsistenteView (LLM chat) | 🚧 Actualizado |
| `/configuracion` | ConfiguracionView | 🚧 Actualizado |
| `*` | NotFoundView (404) | ✅ |

## Estado global (Zustand)

`useSSEStore` — store único para toda la app:
- `messages: Map<string, unknown>` — último mensaje por topic SSE
- `connectionStatus: 'connecting' | 'connected' | 'disconnected' | 'reconnecting'`
- `captureStatus: 'running' | 'paused' | 'unknown'`
- `lastError: string | null`

## useSSE Hook

`web/src/lib/sse.ts`:
- Conecta a `/api/v1/stream/{topic}`
- Auto-reconnect con exponential backoff (1s → 2s → 4s → 8s → 16s → 30s max)
- Jitter: ±500ms aleatorio
- Actualiza zustand store en cada mensaje
- Cleanup en unmount

## API Client

`web/src/lib/api.ts` (untracked):
- `API_BASE = '/api/v1'`
- Funciones: getTopTalkers, getProtocols, getRecentAlerts, getPackets,
  acknowledgeAlert, getVulnerabilities, triggerScan, mitigateVulnerability
- Mapeo de tipos backend → frontend

## ViewState Pattern

```
type ViewState<T> =
  | { status: 'loading' }
  | { status: 'error'; message: string; retry: () => void }
  | { status: 'empty'; message: string; action?: { ... } }
  | { status: 'data'; data: T }
```

Aplicado en DashboardView y CapturaView.

## Tests

8 test files, 91 tests. Todos pasan.
- `act()` warnings conocidos (S4 del handover) — cosmético, no funcional.
