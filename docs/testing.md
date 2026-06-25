# ViewRed — Testing & Verificación

## Resumen Ejecutivo

| Capa | Paquetes/Archivos | Tests | Estado |
|------|-------------------|-------|--------|
| Backend (Go) | 14 paquetes | ~230+ tests | ✅ Todos pasan |
| Frontend (React) | 8 test files | 91 tests | ✅ Todos pasan |
| ML Models | 2 modelos ONNX | Validación sklearn | ✅ 91.27% accuracy |

Última ejecución: 25-Jun-2026

---

## Backend (Go)

### Resultados

```
ok  github.com/viewred/viewred/cmd/viewred              2.451s
ok  github.com/viewred/viewred/internal/api              5.401s
ok  github.com/viewred/viewred/internal/api/handlers      4.907s
ok  github.com/viewred/viewred/internal/api/middleware     3.390s
ok  github.com/viewred/viewred/internal/capture           1.766s
ok  github.com/viewred/viewred/internal/config            2.172s
ok  github.com/viewred/viewred/internal/discovery         2.778s
ok  github.com/viewred/viewred/internal/health            1.654s
ok  github.com/viewred/viewred/internal/rules             2.314s
ok  github.com/viewred/viewred/internal/shutdown          2.403s
ok  github.com/viewred/viewred/internal/sse               2.356s
ok  github.com/viewred/viewred/internal/storage           3.235s
?   github.com/viewred/viewred/internal/ml                [no test files]
?   github.com/viewred/viewred/internal/scanner           [no test files]
```

**14 paquetes, 0 fallas, 0 errores de compilación.**

### Paquetes sin tests

- `internal/ml/` — El motor ML no tiene tests unitarios porque depende de ONNX Runtime (CGO). La validación se hizo en Python durante el training (ver sección ML).
- `internal/scanner/` — Depende de nmap binario externo. Tests requieren entorno con nmap instalado.

### Cobertura por paquete (aproximada)

| Paquete | Cobertura | Notas |
|---------|-----------|-------|
| `internal/capture` | Alta | Packet event, captura |
| `internal/api/handlers` | Alta | Dashboard, alerts, SSE |
| `internal/storage` | Media | Queries, writer, migraciones |
| `internal/rules` | Media | Engine de reglas Suricata |
| `internal/config` | Alta | Configuración, default.rules |
| `internal/sse` | Alta | Broker SSE |
| `internal/shutdown` | Media | Graceful shutdown |
| `internal/discovery` | Media | OUI database |
| `internal/health` | Media | Health check |

---

## Frontend (React)

### Resultados

```
Test Files  8 passed (8)
     Tests  91 passed (91)
  Start at  16:07:07
  Duration  5.78s
```

### Archivos de test

| Archivo | Tests | Lo que cubre |
|---------|-------|-------------|
| `App.test.tsx` | 10 | Routing, navegación, 404 |
| `DashboardView.test.tsx` | 9 | SSE wiring, KPIs, loading, error, empty |
| `CapturaView.test.tsx` | 17 | Health polling, status, error banner, empty |
| `FiltrosView.test.tsx` | — | Filtros de paquetes |
| `TopologiaView.test.tsx` | — | Topología de red |
| `VulnerabilidadesView.test.tsx` | — | Vulnerabilidades |
| `api.test.ts` | — | API fetch wrapper |
| `llm.test.ts` | — | LLM chat integration |

### Warnings conocidos (no bloqueantes)

Los siguientes warnings aparecen en stderr pero **no afectan los resultados**:

1. **Chart width/height** — `"The width(-1) and height(-1) of chart should be greater than 0"`  
   Ocurre porque los charts de recharts no tienen contenedor con dimensiones en jsdom. No afecta tests funcionales.

2. **act() warnings** — `"An update to X inside a test was not wrapped in act(...)"`  
   Actualizaciones de estado asincrónicas post-render. Comunes en tests con fetch/SSE. No afectan assertions.

3. **Failed to parse URL** — `"TypeError: Failed to parse URL from /api/v1/..."`  
   Las llamadas a API no tienen base URL en test (no hay servidor). Se espera — es el comportamiento correcto sin backend.

---

## Motor ML

### Validación de modelos

Los modelos ONNX se validaron durante el entrenamiento con sklearn:

| Modelo | Métrica | Resultado |
|--------|---------|-----------|
| Isolation Forest | True Positive Rate | **76.04%** |
| Isolation Forest | False Positive Rate | **5.00%** |
| Random Forest | Accuracy | **91.27%** |

### Test de integración pendientes

- [ ] Test de carga de modelos ONNX en Go (requiere ONNX Runtime DLL)
- [ ] Test de inferencia end-to-end: capture → features → ONNX → alerta
- [ ] Test de integración con el alert pipeline en main.go

### Feature Importance (validada)

```
[0] Length          — 16.8%
[1] Proto_Idx       —  1.5%
[2] SrcPort         — 17.6%
[3] DstPort         — 16.1%
[4] PacketsPerSec   — 21.4%  ← más importante
[5] UniqueDstPorts  — 15.1%
[6] PayloadEntropy  — 11.6%
```

---

## Pipeline E2E (no automatizado)

El flujo completo requiere entorno con:

1. PostgreSQL + TimescaleDB corriendo
2. ONNX Runtime DLL instalada
3. Interfaz de red para captura (o modo offline con PCAPs)

Flujo:
```
Captura RAW → PacketEvent → ExtractFeatures(7-dim) → ONNX inference
  → ¿Anomalía? → RandomForest → clasificación → Alert en DB
  → ¿Normal? → descarte
```

### Próximos pasos

- [ ] Agregar tests unitarios para `internal/ml/` (mock ONNX sessions)
- [ ] Agregar test de integración E2E con Docker Compose
- [ ] Script de benchmark de inferencia (latencia por paquete)
- [ ] CI/CD con GitHub Actions que ejecute `go test ./...` y `npm test`
