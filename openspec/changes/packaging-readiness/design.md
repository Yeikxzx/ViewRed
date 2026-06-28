# Design: Packaging Readiness

## Technical Approach

Change de tipo "preflight": seis puntos bloqueantes detectados por auditoría, todos mecánicos (refactor + sync de tests + docs). Sin features nuevas, sin migraciones, sin estado mutable. Modo TDD activo: cada fix empieza en ROJO (compilación o expectation rota) y termina en VERDE con `go test ./...` y `cd web && npm run test` pasando. Estrategia de entrega: **single PR** (~143 líneas, muy debajo del budget de 400 → `chained PRs recommended: No`, `400-line budget risk: Low`).

Mapeo a los seis specs (R1–R6):
- **R1** (mover archivo): `mv list_interfaces.go cmd/list_interfaces/main.go` + ajustar package.
- **R2** (callsites de `newWireUp`): agregar `nil` faltante en cada call site. La firma actual es `newWireUp(cfg, rg, querier, dnsResolver, capture, geoipSvc)`.
- **R3** (test CORS): actualizar expectation al slice real de 3 orígenes.
- **R4** (`config.example.yaml`): generar desde `config.Defaults()` programáticamente, no a mano.
- **R5** (mock `getScanHistory`): agregar al factory de `vi.mock('./lib/api', …)`.
- **R6** (README): extender tabla de prerequisitos con columna Plataforma.

## Architecture Decisions

### Decision: Orden de aplicación

**Choice**: Aplicar en el orden R1 → R2 → R3 → R4 → R5 → R6. **R1 primero** porque rompe `go build` para todos los paquetes; aplicarlo último obligaría a developers locales a tener el `package main` huérfano. **R4 antes de R5** para que el archivo de config esté commiteado y el parser se pueda testear contra él.

**Rationale**: Cada paso deja el árbol en un estado compilable; no hay un punto medio donde el repo quede roto. Esto minimiza bisect hell si hay que revertir un commit individual.

### Decision: Strategy para R2 (firma de `newWireUp`)

**Opción** | **Tradeoff** | **Decisión**
---|---|---
Agregar `nil` literal en cada call site | Mecánico, explícito, queda claro qué dependencia se está mockeando | ✅ Elegida
Crear helpers `nilResolver()`, `nilGeoip()` | Más limpio pero introduce indirección para algo que se usa solo en tests | ❌
Refactorizar `newWireUp` para aceptar un struct `Deps` | Más elegante, pero cambia la API pública del paquete `main` y queda fuera del scope de "preflight" | ❌

**Rationale**: El scope de este change es dejar el repo empaquetable, no mejorar APIs. Agregar `nil` literal es la diff más chica posible (~30 líneas en 3 archivos) y deja la puerta abierta a un refactor dedicado cuando llegue el change de mocks realistas (out of scope explícito en `proposal.md`).

### Decision: Generación de `config.example.yaml` (R4)

**Opción** | **Tradeoff** | **Decisión**
---|---|---|
Escribir el YAML a mano, copiando los defaults de `config.go` | Riesgo de drift si alguien toca un default y olvida actualizar el ejemplo | ❌
Generar el YAML con un comando `go run ./scripts/gen-config-example` que invoca `config.Defaults()` y `yaml.Marshal` | Garantiza que el ejemplo refleja el código en todo momento; requiere un script nuevo | ❌ (overhead)
Generar el archivo **una vez** leyendo `config.go` + serializar, commiteado al repo, con un test que verifica que sigue siendo parseable por `config.Load` | Cero overhead en build; drift detectable con un test | ✅ Elegida

**Rationale**: El script generador y el archivo vivo son ambos trabajo extra para un change de 143 líneas. El test "config.example.yaml sigue siendo parseable" cubre el 90% del valor (detecta drift en YAML keys) con 5 líneas de test.

### Decision: Estructura del `config.example.yaml`

**Choice**: Reflejar **exactamente** los campos del struct `Config` (server, capture, ai, scanner, database) — no inventar secciones `storage`, `llm`, `cors` que el struct no tiene. La sección "cors" es `server.cors_origins`.

**Rationale**: El spec R4 menciona "server, capture, storage, ai, llm, cors" pero el struct real tiene `Server`, `Capture`, `AI`, `Scanner`, `Database`. Generar secciones que no existen en el código sería peor que no tenerlas: llevaría a usuarios a setear keys que el parser ignora silenciosamente.

## Data Flow

No hay flujo runtime nuevo — es todo estático (refactor + sync). Cada fix sigue el ciclo TDD clásico:

    ROJO (compilación rota / expectation desincronizada)
        └─► GREEN (agregar nil / actualizar expectation / mover archivo)
                └─► REFACTOR (comentarios, opcional)

## File Changes

| Archivo | Acción | Descripción |
|---|---|---|
| `list_interfaces.go` (raíz) | **Delete** | Movido a `cmd/list_interfaces/main.go`. Borrar después de verificar `go build ./...`. |
| `cmd/list_interfaces/main.go` | **Create** | Contenido idéntico al original. `package main`, import de `gopacket/pcap`, `func main()` con `pcap.FindAllDevs()`. |
| `cmd/viewred/main_cors_test.go` | Modify | 2 calls a `newWireUp`: agregar 6to `nil` (`geoipSvc`). |
| `cmd/viewred/main_shutdown_test.go` | Modify | 2 calls a `newWireUp`: agregar 6to `nil`. |
| `cmd/viewred/main_wire_test.go` | Modify | 5 calls a `newWireUp` (agregar 6to `nil`) + 3 calls a `newWireUpWithInterval` (insertar `nil` para `dnsResolver` antes del `interval`). |
| `internal/config/config_cors_test.go` | Modify | Reemplazar `[]string{"http://localhost:5173"}` por `[]string{"http://localhost:5173", "http://127.0.0.1:5173", "http://127.0.0.1:3000"}`. Agregar comentario documentando que son dev conveniences. |
| `config.example.yaml` (raíz) | **Create** | Una sección por struct field. Comentarios en español. |
| `web/src/App.test.tsx` | Modify | Agregar `getScanHistory: vi.fn(() => Promise.resolve([]))` al factory de `vi.mock('./lib/api', …)`. **Nota**: el archivo real está en `web/src/App.test.tsx`, no `web/src/__tests__/App.test.tsx`. |
| `README.md` | Modify | Tabla de Prerrequisitos: agregar columna "Plataforma"; filas para Npcap (Win), libpcap-dev (Linux), nmap (PATH), `CGO_ENABLED=1`, ONNX runtime (versión pineada), pnpm 9+ (ya estaba, mantener). |
| `internal/config/config_example_test.go` (opcional) | **Create** | Verifica que `config.Load("config.example.yaml")` parsea sin error. Cubre R4. |

**Totales**: 2 create, 5 modify, 1 delete, 1 test opcional.

## Interfaces / Contracts

Firma de `newWireUp` (de `main_wire.go:104`) — referencia para los fixes de R2:

```go
func newWireUp(
    cfg *config.Config,
    rg *health.Registry,
    querier storage.Querier,
    dnsResolver *resolver.Resolver,
    capture *handlers.CaptureHandle,
    geoipSvc *geoip.Service,
) *wireUp
```

Y `newWireUpWithInterval` (`main_wire.go:112`):

```go
func newWireUpWithInterval(
    cfg *config.Config,
    rg *health.Registry,
    querier storage.Querier,
    dnsResolver *resolver.Resolver,
    interval time.Duration,
    capture *handlers.CaptureHandle,
    geoipSvc *geoip.Service,
) *wireUp
```

Estructura objetivo de `config.example.yaml` (esqueleto, comentarios en español en el archivo real):

```yaml
server:
  bind_address: "127.0.0.1"
  port: 8080
  cors_origins:
    - "http://localhost:5173"
    - "http://127.0.0.1:5173"
    - "http://127.0.0.1:3000"
capture:
  interface: ""
  bpf: "tcp port 80 or tcp port 443"
  snaplen: 65535
  max_pcap_size_mb: 500
  max_pcap_age_hours: 168
  pcap_dir: "data/pcaps"
ai:
  ollama_url: "http://localhost:11434"
  ollama_model: "qwen2.5:3b"
  ml_anomaly_threshold: 0.85
  llm_temperature: 0.7
scanner:
  nmap_path: ""
  max_concurrent_scans: 5
  scan_timeout_seconds: 600
database:
  url: "postgres://viewred:viewred_dev_only@localhost:5432/viewred?sslmode=disable"
```

## Testing Strategy

| Capa | Qué se testea | Cómo |
|---|---|---|
| Build Go | `go build ./...` sin `package main` duplicado | Shell, post-move. |
| Tests Go | `go test ./...` (cmd/viewred, internal/config) | RED: errores de compilación por args faltantes / expectation CORS rota. GREEN: agregar nils + actualizar expectation. |
| Tests React | `cd web && npm run test` | RED: `getScanHistory is not a function`. GREEN: agregar al factory mock. |
| Config example | `config.Load("config.example.yaml")` parsea sin error | Test nuevo en `internal/config/` (opcional pero barato). |
| Documental | `README.md` menciona Npcap, libpcap-dev, nmap, CGO, ONNX, pnpm | Code review. |

## Migration / Rollout

No requiere migration. No hay estado mutable, no hay feature flag, no hay datos a transformar. Si algo falla post-merge: `git revert <sha>` alcanza. Los tests que ya pasaban antes siguen pasando (regresión cero) — esto está garantizado por la naturaleza mecánica del change.

## Open Questions

- **Test `config_example_test.go`**: incluir (5 líneas, cubre drift futuro) o saltear. Recomendación: incluir.
- **Path real de App.test.tsx**: está en `web/src/App.test.tsx`, no `web/src/__tests__/App.test.tsx` como dice el spec. Documentado para que el implementador no busque en el lugar equivocado.
- **No hay preguntas bloqueantes.**
