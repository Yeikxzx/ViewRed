# ViewRed — Estado del Repo

## Ramas

| Rama | Descripción |
|------|-------------|
| `main` | Planes 1-3 mergeados. HEAD = `8d08305` |
| `feat/plan4-ml-and-alerts` | **ACTUAL**. Plan 4 + 5 + 6 parcial |
| `feat/plan2-capture-and-storage` | Histórica (Plan 2, se puede borrar) |
| `hotfix/plan3-warnings` | Branch para hotfixes de Plan 3 (vacía) |

## Commits en feat/plan4-ml-and-alerts (desde main)

```
d4be4fc feat(ml): expand feature vector to 7 dimensions with sliding window stats
de67698 fix(ml): restore engine_stub.go for non-CGO builds and add cgo build tag
bf8a3c4 refactor(api): remove unused VulnerabilitiesHandler struct
03ca531 fix(discovery): add 30s timeout to OUI database HTTP download
402fe33 fix(cmd): guard store.GetCustomRules against nil when DB unavailable
8a4e157 feat(topology): implement plan 5 discovery engine and UI
8d735b6 fix(eval): prevent goroutine leak in evalPackets and optimize Suricata rules port parsing
```

## Cambios sin commitear (del otro agente)

Quedan **28 archivos modificados** del otro agente sin commitear:

```
cmd/viewred/main_wire.go
go.mod / go.sum
internal/api/handlers/packets.go, sse.go, stubs.go, stubs_test.go
internal/api/router.go
internal/config/config.go, config_test.go
internal/storage/migrate.go, querier.go, queries.go, queries_test.go
web/package-lock.json, package.json
web/src/App.test.tsx, App.tsx
web/src/components/ConfiguracionView.tsx, DashboardView.*, FiltrosView.tsx, TopologiaView.tsx, VulnerabilidadesView.tsx
web/src/lib/llm.test.ts, llm.ts
web/src/test-setup.ts
```

**Archivos untracked** (nuevos):
```
fix_tests.py
internal/api/handlers/llm_chat.go, rules_config.go
internal/config/default.rules
internal/scanner/
internal/storage/migrations/0002_*, 0003_*
internal/storage/queries_chat.go, queries_rules.go, queries_vulnerabilities.go
models/ (isolation_forest.onnx, random_forest.onnx)
onnxruntime.dll
scripts/build.ps1, train_models.py
list_interfaces.go, oui.txt
web/src/lib/api.ts
```
