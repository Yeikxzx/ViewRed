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

## Commits locales (8 fixes + 5 features + 1 docs = 14 commits desde main)

```
90cd7b5 feat(cmd): point ML engine to models directory for ONNX model loading
73de228 feat(scanner): add nmap-based vulnerability scanning engine
34ff4c4 feat(cmd): update SSE event name and add default rules config
aaa7fcd feat(web): wire dashboard to real API, add chat streaming, toast notifications
7ac9899 feat(api): add LLM chat, rules CRUD, alerts handlers and update router
a521d71 feat(storage): add port/payload_search packet filters, migrations, and new query methods
b7ba0a0 docs(context): add complete project context dump for disaster recovery
d4be4fc feat(ml): expand feature vector to 7 dimensions with sliding window stats
de67698 fix(ml): restore engine_stub.go for non-CGO builds and add cgo build tag
bf8a3c4 refactor(api): remove unused VulnerabilitiesHandler struct
03ca531 fix(discovery): add 30s timeout to OUI database HTTP download
402fe33 fix(cmd): guard store.GetCustomRules against nil when DB unavailable
8a4e157 feat(topology): implement plan 5 discovery engine and UI
8d735b6 fix(eval): prevent goroutine leak in evalPackets and optimize Suricata rules port parsing
```

## Queda sin trackear (ignorado por .gitignore o generado)
```
onnxruntime.dll   — binario ONNX Runtime (11MB, ignorado por .gitignore)
oui.txt           — OUI database descargada en runtime (ignorado por .gitignore)
```
