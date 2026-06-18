# Plan 3 e2e verification report

**Date:** 2026-06-18
**Change:** core-api-and-ui-shell
**Status:** PASS

## Test suite

| Layer | Command | Result |
| --- | --- | --- |
| Go | `go test ./...` | 10/10 packages OK |
| Go vet | `go vet ./...` | clean |
| Web | `cd web && npm run test` | 91/91 tests passed (8 files) |
| Web lint | `cd web && npm run lint` (tsc --noEmit) | clean |
| Web build | `cd web && npm run build` | clean (chunk-size info only) |

## Binary build

`go build -o ./bin/viewred.exe ./cmd/viewred` — OK, ~14 MB.

## End-to-end smoke (binary on 127.0.0.1:8080)

| Endpoint | Expected | Result |
| --- | --- | --- |
| `GET /api/v1/health` | 200 | **200** + `X-Request-Id` header present |
| `GET /api/v1/dashboard/summary` | 503 (no DB → ErrorQuerier) | **503 ServiceUnavailable** + `{"error":"storage unavailable"}` |
| `GET /api/v1/stream/metrics` | SSE headers | **text/event-stream** + Connection + X-Accel-Buffering + X-Request-Id |
| `GET /api/v1/alerts` | 200 `[]` | **200** `[]` |
| `GET /api/v1/topology` | 200 `{nodes:[],edges:[]}` | **200** `{"edges":[],"nodes":[]}` |
| `GET /api/v1/config` | 200 `{version, features}` | **200** `{"features":{},"version":"0.1.0"}` |

Cleanup: process killed by `Stop-Process -Id <PID>` via `Get-NetTCPConnection -LocalPort 8080`.

## Plan 3 task summary (26 tasks)

All 26 tasks complete. See `apply-progress` engram topic
`sdd/core-api-and-ui-shell/apply-progress` for per-task commits.

## Whole-branch review

Fresh-context sdd-verify review of the full 27-commit Plan 3 diff (base
`296f904`, HEAD before fixes `5762d0a`) found 4 CRITICAL and 1 WARNING
integration bugs. Verdict: **BLOCK-MERGE**.

### Critical findings (all fixed)

| # | Bug | Fix commit | What changed |
|---|-----|------------|--------------|
| C1 | `DashboardSummary` had no JSON tags → SSE emitted PascalCase → frontend (snake_case) rendered empty dashboard | `04fd5d2` | Added `json:"..."` snake_case tags to `DashboardSummary`, `TopTalker`, `ProtocolShare`, `Device` |
| C2 | Both main ticker AND per-request handler ticker broadcast on the `metrics` topic → 2 frames/sec, duplicated JSON | `cd95ba4` | Removed per-request ticker from `StreamMetricsHandler`; handler is now a pure relay |
| C3 | `shutdown.Params.Broker` was empty; explicit `defer` shut broker AFTER storage | `570f228` | `Broker: wire.Broker` passed to `shutdown.Params`; redundant defer removed |
| C4 | CORS never wired in production: config had no `CORSOrigins`, main_wire passed empty `CORSConfig` | `de5c154` | Added `CORSOrigins []string` to `config.Server` with `["http://localhost:5173"]` dev default; wired in main_wire |
| W1 | `RequestID` accepted inbound `X-Request-Id` verbatim → CRLF/log injection | `c4799fc` | Validate inbound: reject CR/LF/NUL, cap at 128 chars, fall back to UUID |

### Post-fix re-verification

| Check | Result |
| --- | --- |
| `go test ./...` | 10/10 packages OK |
| `cd web && npm run test` | 91/91 tests passed (8 files) |
| `go vet ./...` | clean |
| `cd web && npm run lint` | clean |
| Binary rebuild | OK |
| `OPTIONS /api/v1/dashboard/summary` with `Origin: http://localhost:5173` | **204** + `Access-Control-Allow-Origin: http://localhost:5173` + `Vary: Origin` |
| Same OPTIONS with `Origin: http://evil.example.com` | **403** (CORS rejects) |
| `GET /api/v1/health` with `X-Request-Id: good-id-123` | echoed back |
| `GET /api/v1/stream/metrics` headers | `text/event-stream` + `X-Accel-Buffering: no` + `X-Request-Id` |
| SSE body | heartbeats only (no metrics events because no DB; unit tests cover wire format + no-double-broadcast) |

## Next step

Plan 3 is **DONE**. All 26 tasks complete, 5 review fixes applied, full suite green, binary runs, CORS + RequestID validated. Subsequent plans:
- Plan 4: ml-and-alerts
- Plan 5: topology-and-discovery
- Plan 6: llm-assistant
- Plan 7: scanner-and-vulns
- Plan 8: installer-and-polish
