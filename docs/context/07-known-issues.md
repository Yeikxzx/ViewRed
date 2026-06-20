# ViewRed — Issues Conocidos

## Del handover de Plan 3 (no corregidos)

| ID | Severidad | Qué | Dónde |
|----|-----------|-----|-------|
| W2 | MEDIUM | `TopTalker` tiene `{IP, Bytes}` pero spec quiere `{SrcIP, Bytes, Packets}`. `ProtocolShare` tiene `{Protocol, Percent}` pero spec quiere `{Protocol, Percentage, Bytes}` | `internal/storage/querier.go` |
| W4 | LOW | SSE no emite `event: error` con `recoverable: true` cuando falla query | `internal/api/handlers/sse.go` |
| W5 | LOW | SSE handler no emite snapshot inicial al conectar | `internal/api/handlers/sse.go` |
| W6 | LOW | `Broker.Subscribe` puede devolver canal que nunca entrega si ctx cancela mid-call | `internal/sse/broker.go` |
| W7 | LOW | Múltiples `defer`s en main.go + `shutdown.Shutdown` overlapping; redundant closures | `cmd/viewred/main.go` |
| S1 | LOW | Constante `"metrics"` duplicada en `sse.go` y `main_wire.go` | ambos |
| S2 | LOW | `Broker.Shutdown` no emite evento graceful antes de cerrar | `internal/sse/broker.go` |
| S4 | LOW | `act()` warnings en tests frontend | `CapturaView.test.tsx`, `App.test.tsx` |
| S5 | PROCESS | E2E no curó body SSE para verificar metrics frame | — |

## Issues del branch actual

| ID | Severidad | Qué | Dónde |
|----|-----------|-----|-------|
| — | INFO | Router middleware order: spec dice `Recovery→CORS→RequestID`, código hace `Recovery→RequestID→CORS` | `internal/api/router.go` |
| — | INFO | `scanner/nmap.go` requiere nmap binary (no existe en Windows sin install manual) | `internal/scanner/nmap.go` |
| — | INFO | `fix_tests.py` sin trackear ni ejecutar (el otro agente lo dejó) | raíz |
| — | INFO | `onnxruntime.dll` (11MB) + models (1.2MB) commiteados — decidir si incluir en repo | raíz |
| — | INFO | `models/` files podrían ir en `.gitignore` y generarse con script | `scripts/train_models.py` |
| — | INFO | Score de alertas ML hardcodeado a 85.0 en `engine.go` | `internal/ml/engine.go` |
| — | INFO | LLM Chat Handler guarda mensaje assistant en goroutine sin error handling | `internal/api/handlers/llm_chat.go` |
