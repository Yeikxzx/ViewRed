# Change: Packaging Readiness

## Intent
ViewRed es una herramienta local-first de NDR (Network Detection & Response) que está cerca de su primera distribución cerrada para pentesting interno. Una auditoría reciente encontró seis bloqueantes que impiden empaquetar y correr la suite de tests en limpio: un archivo `package main` huérfano en la raíz que rompe `go build`, tests desactualizados respecto a la firma de `newWireUp`, un test de defaults de CORS desincronizado, un mock de Vitest incompleto, ausencia de `config.example.yaml` y un `README.md` con prerequisitos incompletos. Este change resuelve esos seis puntos sin agregar features, para dejar el repositorio en estado compilable, testeable y presentable a un pentester.

## Scope

**Mover**
- `list_interfaces.go` → `cmd/list_interfaces/main.go`

**Modificar (Go tests)**
- `cmd/viewred/main_cors_test.go`
- `cmd/viewred/main_shutdown_test.go`
- `cmd/viewred/main_wire_test.go`
- `internal/config/*_test.go` (test de defaults CORS)

**Modificar (frontend)**
- `web/src/__tests__/App.test.tsx` (mock factory)

**Crear**
- `config.example.yaml` (raíz del repo)

**Actualizar (docs)**
- `README.md` (sección de prerequisitos)

## Approach

**1. Mover `list_interfaces.go`.** La causa raíz es estructural: dos archivos declaran `package main` en la raíz y choca con `cmd/viewred/main.go`. Crear `cmd/list_interfaces/main.go`, mover el contenido tal cual y borrar el original. Verificar con `go build ./...` antes y después.

**2. Sincronizar call sites de `newWireUp` en tests.** La firma evolucionó a `newWireUp(cfg, deps, resolver, geoip, ...)`. Agregar `nil, nil` en cada call site hasta que `go test ./cmd/viewred/...` compile. Es mecánico: TDD dice rojo (compila-falla) → verde (agregar nils) → refactor.

**3. Sincronizar test de defaults CORS.** El test asume 1 origin; el código real ahora define 3. Ajustar el expectation al valor real (verificar primero qué origins carga `internal/config.Defaults()`). No tocar el código de defaults.

**4. Crear `config.example.yaml`.** Generar desde los defaults reales del código (no a mano) para evitar drift. Comentarios en español por sección: server, capture, storage, ai, llm, cors. Sin placeholders.

**5. Completar mock `getScanHistory`.** El factory en `App.test.tsx` no exporta `getScanHistory`, por eso el test del componente que la consume falla con "is not a function". Agregar `getScanHistory: vi.fn(() => Promise.resolve([]))` al mock.

**6. Actualizar prerequisitos del README.** Agregar a la lista: Npcap (Windows), libpcap-dev (Linux), nmap en PATH, `CGO_ENABLED=1`, ONNX runtime (versión pineada), pnpm como package manager. Mantener compat matrix.

## Risks

| Riesgo | Likelihood | Impacto | Mitigación |
|--------|-----------|---------|------------|
| Mover `list_interfaces.go` rompe imports no obvios | Baja | Build roto | Correr `go build ./...` antes de borrar el original |
| `nil, nil` en `newWireUp` enmascara bugs reales de wiring | Media | Tests verdes, runtime rojo | Mocks reales quedan para un change posterior dedicado |
| `config.example.yaml` se desincroniza con el código | Media | Usuarios configuran mal | Generar el ejemplo desde `config.Defaults()` programáticamente |
| Test CORS fragil ante futuros cambios de defaults | Baja | Rompe con cada cambio | Documentar en el test qué origins se esperan y por qué |
| Mock de `getScanHistory` esconde bug de API real | Baja | Bug pasa a prod | Mocks solo para unit tests; los integration tests mockean a nivel HTTP |
| Versiones pineadas (ONNX, Npcap) quedan stale | Media | Instalación falla | Agregar compat matrix + fecha de última verificación |

**Rollback plan**: change 100% tests/docs/refactor mecánico, sin estado mutable ni migraciones. `git revert <sha>` alcanza si algo falla post-merge.

## Effort Estimate

| Item | Líneas |
|------|--------|
| Mover `list_interfaces.go` | ~5 |
| Fix call sites `newWireUp` (3 archivos × ~10 sitios) | ~30 |
| Fix test CORS defaults | ~5 |
| Crear `config.example.yaml` | ~80 |
| Fix mock `getScanHistory` | ~3 |
| Actualizar README | ~20 |
| **Total** | **~143 líneas** |

Muy por debajo de 400 → estrategia **C4 (single PR)** confirmada. Modo TDD activo: `go test ./...` para Go, `cd web && npm run test` para frontend.

## Out of Scope

- Empaquetado real (binarios, instaladores, Docker, goreleaser): change posterior.
- Mocks realistas de `*resolver.Resolver` y `*geoip.Service`: requiere diseño adicional.
- Fix de toolchain real (CGO/ONNX en Windows): el README documenta, la fix es otro change.
- Renombrar o refactorizar archivos de test de `internal/config`: solo se ajusta el expectation.
- Cualquier feature nueva (UI, captura, AI): este change es exclusivamente de "listo para empaquetar".
- CI/CD pipeline: queda para el change de release engineering.
