# ViewRed — Decisiones de Arquitectura

## Stack confirmado V1

- **Captura**: Go + gopacket + AF_PACKET (Linux, kernel ≥4.18) / Npcap (Windows)
- **Backend**: Go + Gin
- **DB**: PostgreSQL + TimescaleDB
- **UI**: React 18 + TypeScript + Tailwind CSS
- **Real-time**: SSE + REST (no WebSockets)
- **ML**: Isolation Forest + Random Forest en ONNX (local, CPU)
- **LLM**: Qwen 2.5 3B vía Ollama (local, multilingüe)
- **Deploy**: bind 127.0.0.1, acceso admin vía SSH tunnel

## Patrones clave

### Constructor Injection (no DI frameworks)
- Handlers reciben `storage.Querier` como parámetro
- Storage implementa `Querier`; `MockQuerier` / `ErrorQuerier` para tests
- Sin nil checks: `ErrorQuerier` devuelve 503 para todos los métodos

### Router
- `NewRouterWithDeps(RouterDeps)` — constructor principal
- `NewRouter()` / `NewRouterWithRegistry()` — back-compat preservados
- Middleware order: Recovery → RequestID → CORS → RequestLogging → routes

### SSE Broker
- Topic-based pub/sub con ring buffer de 64 eventos por cliente
- Heartbeat cada 30s, stale client removal tras 3 heartbeats perdidos
- `sse.Event{Topic, Name, Data}`
- Broadcast no bloqueante

### Shutdown ordenado
```
capture.Stop → close packets → close evalPackets → pcap.Close
→ storage.Writer.Flush → http.Server.Shutdown → broker.Shutdown
→ storage.Close
```

### Frontend
- React Router v7 con `BrowserRouter`
- Zustand para estado compartido SSE
- `useSSE` hook con auto-reconnect y exponential backoff (1s → 30s max)
- ViewState pattern: loading | error | empty | data

## Out of V1 (explícito)
Topología 3D, control activo (bloquear), RBAC granular, reportes exportables,
SIEM/Syslog/NetFlow, multi-tenant/HA, app móvil, auth interna.
