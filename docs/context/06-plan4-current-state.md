# ViewRed — Estado Actual del Branch feat/plan4-ml-and-alerts

## Qué incluye

El branch `feat/plan4-ml-and-alerts` tiene código de **Plan 4** (ML, alerts, rules),
**Plan 5** (discovery, topology) y **Plan 6** (LLM chat) **mezclado**.
No hay artifacts SDD para ninguno de estos planes.

## Commiteado (7 commits desde main)

### Fixes de verificación (este agente)
1. `402fe33` — fix(cmd): nil guard en store.GetCustomRules (CRITICAL)
2. `03ca531` — fix(discovery): timeout HTTP en OUI download (HIGH)
3. `bf8a3c4` — refactor(api): dead code removal (VulnerabilitiesHandler)
4. `de67698` — fix(ml): restore engine_stub.go + cgo build tag (CRITICAL, build roto)

### Feature work
5. `8d735b6` — fix(eval): goroutine leak en evalPackets + Suricata port parsing
6. `8a4e157` — feat(topology): discovery engine + UI
7. `d4be4fc` — feat(ml): feature vector 7-d + WindowStats + modelos retrained

## No commiteado (28 archivos modificados + 14 untracked)

### Backend
- `internal/api/handlers/` — alerts real handler, llm_chat, rules_config, packets payload_search
- `internal/api/router.go` — nuevas rutas para alerts, rules, scan, chat
- `internal/storage/` — queries para alerts, topology, chat, rules, vulns; migrations 0002-0003
- `internal/scanner/nmap.go` — scanner engine (requiere nmap binary)
- `internal/config/` — default.rules file

### Frontend
- `web/src/App.tsx` + `App.test.tsx` — nuevas rutas y vistas
- `web/src/components/` — ConfiguracionView, FiltrosView, TopologiaView, VulnerabilidadesView, DashboardView actualizados
- `web/src/lib/api.ts` — fetch wrapper con tipos backend
- `web/src/lib/llm.ts` + test — integración con API Ollama

## Lo que falta

- SDD artifacts para Plan 4+
- Integración completa de ML engine con alerts en main.go
- shadcn/ui (deferido a Plan 8)
- Pruebas E2E de las nuevas funcionalidades
