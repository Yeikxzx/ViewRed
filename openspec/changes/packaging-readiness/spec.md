# Spec: Packaging Readiness

> Delta sobre el estado actual del repositorio. Six items bloqueantes detectados en auditoría pre-empaquetado.

---

## R1: Mover `list_interfaces.go` a `cmd/list_interfaces/main.go`

**Given** que `list_interfaces.go` existe en la raíz del repo con `package main`
**When** se ejecuta `go build ./...`
**Then** falla porque dos paquetes `main` compiten (`list_interfaces.go` y `cmd/viewred/main.go`)

### Acceptance criteria
- [ ] `cmd/list_interfaces/main.go` existe con el contenido exacto del `list_interfaces.go` original
- [ ] `list_interfaces.go` en la raíz fue eliminado
- [ ] `go build ./cmd/list_interfaces/...` compila sin error
- [ ] `go build ./...` compila sin error (no hay más conflicts de `package main`)
- [ ] `go vet ./cmd/list_interfaces/...` no reporta issues nuevos

---

## R2: Sincronizar callsites de `newWireUp` en tests de `cmd/viewred`

**Given** que `newWireUp` ahora requiere 6 parámetros: `(cfg, rg, querier, dnsResolver, capture, geoipSvc)`
**When** se ejecuta `go test ./cmd/viewred/...`
**Then** los 3 archivos de test (`main_cors_test.go`, `main_shutdown_test.go`, `main_wire_test.go`) fallan con error de compilación porque pasan 5 parámetros

### Acceptance criteria
- [ ] `main_cors_test.go`: todos los calls a `newWireUp` pasan 6 args (agregar `nil` para `geoipSvc`)
- [ ] `main_shutdown_test.go`: todos los calls a `newWireUp` pasan 6 args
- [ ] `main_wire_test.go`: todos los calls a `newWireUp` y `newWireUpWithInterval` pasan la cantidad correcta de args
- [ ] `go test ./cmd/viewred/...` compila y corre (los tests pueden fallar por otras razones, pero no por error de compilación)

---

## R3: Sincronizar test de defaults CORS en `internal/config`

**Given** que `TestLoadDefaultsIncludeLocalhost5173CORSOrigin` en `internal/config/config_cors_test.go` espera `[]string{"http://localhost:5173"}`
**When** los defaults reales ahora incluyen 3 orígenes
**Then** el test falla porque el expectation no coincide con la realidad

### Acceptance criteria
- [ ] Se verifica el valor real de `config.Defaults().Server.CORSOrigins`
- [ ] El expectation del test se actualiza al slice real (sin modificar el código de defaults)
- [ ] `go test ./internal/config/... -run TestLoadDefaultsIncludeLocalhost5173CORSOrigin` pasa
- [ ] Se agrega un comentario en el test documentando qué orígenes se esperan y por qué existen (dev conveniences)

---

## R4: Crear `config.example.yaml` en la raíz

**Given** que no existe `config.example.yaml`
**When** un usuario nuevo clona el repo
**Then** no tiene una referencia de configuración para copiar y adaptar

### Acceptance criteria
- [ ] `config.example.yaml` existe en la raíz del repo
- [ ] Contiene todas las secciones: `server`, `capture`, `storage`, `ai`, `llm`, `cors`
- [ ] Los valores reflejan los defaults reales del código (generado desde `config.Defaults()` o extraído del código, no escrito a mano)
- [ ] Los comentarios están en español
- [ ] No contiene valores sensibles ni placeholders
- [ ] `config.example.yaml` pasa como entrada válida del parser de config (cargable sin error)

---

## R5: Completar mock de `getScanHistory` en `App.test.tsx`

**Given** que el `vi.mock('./lib/api')` en `web/src/App.test.tsx` no incluye `getScanHistory`
**When** un componente hijo consume `getScanHistory` desde `./lib/api`
**Then** el test falla con "getScanHistory is not a function"

### Acceptance criteria
- [ ] El factory de `vi.mock('./lib/api')` exporta `getScanHistory: vi.fn(() => Promise.resolve([]))`
- [ ] `cd web && npm run test` pasa sin errores relacionados a `getScanHistory`
- [ ] Ningún otro mock de `./lib/api` está incompleto (verificar cobertura de funciones exportadas)

---

## R6: Actualizar prerequisitos en `README.md`

**Given** que la sección "Prerrequisitos" del `README.md` lista Go, Node.js, pnpm, Docker y PostgreSQL
**When** un usuario en Windows o Linux intenta compilar o testear
**Then** faltan dependencias críticas (Npcap, libpcap-dev, nmap, CGO, ONNX) que no están documentadas

### Acceptance criteria
- [ ] La tabla de prerrequisitos incluye:
  - Npcap (Windows) con link de descarga
  - libpcap-dev (Linux, Ubuntu/Debian) o equivalente (libpcap-devel en RHEL)
  - nmap en PATH
  - `CGO_ENABLED=1` (nota sobre toolchain)
  - ONNX Runtime con versión pineada y fecha de última verificación
  - pnpm 9+ (ya existe, mantener)
- [ ] Se agrega una columna "Plataforma" indicando si es requerido en Windows, Linux o ambas
- [ ] Se agrega nota sobre compatibilidad cross-platform (lo que funciona y lo que no)

---

## Scenarios

### S1: Build completo post-move
- **Setup**: `list_interfaces.go` movido a `cmd/list_interfaces/main.go`, archivo raíz eliminado
- **Input**: `go build ./...`
- **Expected**: Exit 0, sin errores de `package main` duplicado

### S2: Tests Go post-fix
- **Setup**: Callsites de `newWireUp` actualizados, test CORS sincronizado
- **Input**: `go test ./cmd/viewred/... ./internal/config/...`
- **Expected**: Compila sin errores; los tests que pasaban antes siguen pasando (regresión cero)

### S3: Frontend tests post-fix
- **Setup**: Mock de `getScanHistory` agregado en `App.test.tsx`
- **Input**: `cd web && npm run test`
- **Expected**: Todos los tests de routing pasan, sin errores de "is not a function"

### S4: Config example cargable
- **Setup**: `config.example.yaml` creado en la raíz
- **Input**: `go run ./cmd/viewred --config config.example.yaml` (o test que parsea el archivo)
- **Expected**: El parser carga el archivo sin errores de sintaxis ni validación

### S5: README completo para onboarding
- **Setup**: README actualizado con todos los prerequisitos
- **Input**: Lectura de la sección "Prerrequisitos" por un desarrollador nuevo
- **Expected**: Toda dependencia necesaria para `go build ./...` y `cd web && pnpm test` está documentada con versión, plataforma y propósito
