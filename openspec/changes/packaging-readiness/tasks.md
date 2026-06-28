# Tasks: Packaging Readiness

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~143 |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | single PR |
| Delivery strategy | exception-ok |
| Chain strategy | size-exception |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | Fix Go build + tests + packaging docs | PR 1 | single PR (~143 lines), base: main |

## Phase 1: Build Infrastructure

- [x] 1.1 Create `cmd/list_interfaces/main.go` with content from root `list_interfaces.go` (same `package main` + code)
- [x] 1.2 Delete root `list_interfaces.go`
- [x] 1.3 Verify: `go build ./...` compiles without `package main` conflict error

## Phase 2: Test Compilation Fixes

- [x] 2.1 `cmd/viewred/main_cors_test.go` — add `nil` 6th arg (`geoipSvc`) to each `newWireUp(cfg, ...)` call
- [x] 2.2 `cmd/viewred/main_shutdown_test.go` — add `nil` 6th arg to each `newWireUp(cfg, ...)` call
- [x] 2.3 `cmd/viewred/main_wire_test.go` — add missing args to all `newWireUp` and `newWireUpWithInterval` calls
- [x] 2.4 Verify: `go test ./cmd/viewred/...` compiles (tests may still fail on CORS expectation)

## Phase 3: Test Corrections

- [x] 3.1 Inspect `config.Defaults().Server.CORSOrigins` to find actual 3-origin value
- [x] 3.2 `internal/config/config_cors_test.go` — update expected CORS origins to real value; add comment documenting why 3 origins exist (dev conveniences)
- [x] 3.3 `web/src/__tests__/App.test.tsx` — add `getScanHistory: vi.fn(() => Promise.resolve([]))` to `vi.mock('./lib/api')` factory; verify no other exported functions are missing from mock
- [x] 3.4 Verify: `go test ./internal/config/... -run TestLoadDefaultsIncludeLocalhost5173CORSOrigin` passes
- [x] 3.5 Verify: `cd web && npm run test` passes without errors

## Phase 4: Packaging Artifacts

- [x] 4.1 Create `config.example.yaml` at repo root with sections (server, capture, storage, ai, llm, cors) from real code defaults; comments in Spanish; no placeholders
- [x] 4.2 Verify: `config.example.yaml` loads without parse error via Go config parser
- [x] 4.3 `README.md` — update prerequisites: add Npcap (Win), libpcap-dev (Linux), nmap in PATH, `CGO_ENABLED=1`, ONNX Runtime (pinned + last-verified date); add Platform column; keep pnpm 9+
- [x] 4.4 Final verification: `go build ./...` && `go test ./...` && `cd web && npm run test` all pass
