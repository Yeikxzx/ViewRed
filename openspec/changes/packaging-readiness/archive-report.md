# Archive Report: Packaging Readiness

**Archived on:** 2026-06-28
**Status:** ✅ Completed — all 6 blocker items resolved, all tests passing

---

## Summary

Change de tipo "preflight" para dejar el repositorio de ViewRed en estado compilable, testeable y presentable antes de la primera distribución cerrada para pentesting interno. Los seis bloqueantes detectados en la auditoría se resolvieron sin agregar features nuevas.

| R# | Requerimiento | Estado | Evidencia |
|----|--------------|--------|-----------|
| R1 | Mover `list_interfaces.go` → `cmd/list_interfaces/main.go` | ✅ | `go build ./...` compila sin error de `package main` duplicado |
| R2 | Sincronizar callsites de `newWireUp` en tests de `cmd/viewred` | ✅ | 11 calls actualizados con 6to arg `nil` (o reordenados en `newWireUpWithInterval`) |
| R3 | Sincronizar test de defaults CORS en `internal/config` | ✅ | Expectation actualizado a 3 orígenes; comentario documentado |
| R4 | Crear `config.example.yaml` en la raíz | ✅ | 57 líneas, 6 secciones, comentarios en español, cargable sin error |
| R5 | Completar mock de `getScanHistory` en `App.test.tsx` | ✅ | `vi.fn(() => Promise.resolve([]))` agregado al factory |
| R6 | Actualizar prerequisitos en `README.md` | ✅ | Tabla con columna Plataforma; Npcap, libpcap-dev, nmap, CGO, ONNX documentados |

---

## Files Changed

### Created
| Archivo | Descripción |
|---------|-------------|
| `cmd/list_interfaces/main.go` | Contenido idéntico al `list_interfaces.go` original. `package main` con `func main()` que lista interfaces de red vía `gopacket/pcap`. |
| `config.example.yaml` | Config de ejemplo con secciones `server`, `capture`, `ai`, `scanner`, `database`. Comentarios en español. Refleja defaults reales del código. |

### Deleted
| Archivo | Descripción |
|---------|-------------|
| `list_interfaces.go` (raíz) | Movido a `cmd/list_interfaces/main.go`. Elimina el conflicto de `package main` duplicado. |

### Modified
| Archivo | Cambio |
|---------|--------|
| `cmd/viewred/main_cors_test.go` | 2 calls a `newWireUp` — agregado `nil` como 6to argumento (`geoipSvc`) |
| `cmd/viewred/main_shutdown_test.go` | 2 calls a `newWireUp` — agregado `nil` como 6to argumento |
| `cmd/viewred/main_wire_test.go` | 5 calls a `newWireUp` (+ `nil`) + 3 calls a `newWireUpWithInterval` (insertado `nil` para `dnsResolver` antes de `interval`) |
| `internal/config/config_cors_test.go` | Expectation de CORS actualizado de 1 a 3 orígenes + comentario documentando dev conveniences |
| `web/src/App.test.tsx` | `getScanHistory: vi.fn(() => Promise.resolve([]))` agregado al factory de `vi.mock('./lib/api', ...)` |
| `README.md` | Tabla de Prerrequisitos: columna "Plataforma" agregada; filas para Npcap, libpcap-dev, nmap, CGO_ENABLED=1, ONNX Runtime |

---

## Test Results Snapshot

### Go Build & Vet
```
$ go build ./...       → Exit 0 (sin errores)
$ go vet ./cmd/list_interfaces/...  → Exit 0 (sin issues)
```

### Go Tests
```
$ go test ./cmd/viewred/... ./internal/config/... -count=1
ok  github.com/viewred/viewred/cmd/viewred      0.496s
ok  github.com/viewred/viewred/internal/config   0.397s
```

### Frontend Tests (Vitest)
```
Test Files  8 passed (8)
     Tests 91 passed (91)
  Duration 4.92s
```

Las advertencias de `act(...)` en los tests de React son preexistentes y no corresponden a este change.

---

## What's Left for Future Changes

| Item | Change Propuesto | Dependencia |
|------|-----------------|-------------|
| Empaquetado real (binarios, instaladores, Docker, goreleaser) | `release-engineering` | Ninguna |
| Mocks realistas de `*resolver.Resolver` y `*geoip.Service` | `realistic-mocks` | Ninguna |
| Fix de toolchain CGO/ONNX en Windows | `toolchain-setup` | README actualizado (este change) |
| CI/CD pipeline | `release-engineering` | `release-engineering` |
| Renombrar/refactorizar `newWireUp` para struct `Deps` | `refactor-wireup` | `realistic-mocks` |

---

## Final State of the Project

- **Compilación:** `go build ./...` sin errores en Go backend
- **Backend tests:** 2 packages, ambos OK
- **Frontend tests:** 8 test files, 91 tests, todos pasando
- **Configuración:** `config.example.yaml` presente y cargable
- **Documentación:** README con prerequisitos completos incluyendo Npcap, libpcap-dev, nmap, CGO, ONNX
- **Estructura:** Sin archivos `package main` huérfanos en la raíz
- **Mock coverage:** `getScanHistory` cubierto en el factory de `vi.mock('./lib/api', ...)`

El repositorio queda en un estado donde cualquier developer con Go 1.25+, Node.js 22+, pnpm 9+, Npcap/libpcap, nmap en PATH y ONNX Runtime 1.19.2 puede clonar, compilar y testear sin blockers.
