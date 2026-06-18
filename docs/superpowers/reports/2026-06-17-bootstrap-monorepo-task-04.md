# Task 4 Report: YAML config loader (TDD)

**Date**: 2026-06-17
**Commit**: `d9a4ac2`

## What was implemented

- `internal/config/config.go` — Config struct with Server, AI, Scanner subsections + `Load()` reading YAML with safe defaults on missing file, merge on present file, error on malformed YAML
- `internal/config/config_test.go` — 3 tests (defaults on missing, parse from file, error on invalid YAML)
- `internal/config/helpers_test.go` — `writeFile` helper
- `cmd/viewred/main.go` — replaced with config-wired version: uses `VIEWRED_CONFIG` env var or `os.UserConfigDir()/viewred/config.yaml`, then calls `config.Load()` and binds to `cfg.Server.BindAddress:cfg.Server.Port`
- `go.mod` — promoted `gopkg.in/yaml.v3` from indirect to direct dependency

## TDD evidence

**RED** (before implementation):
```
internal\config\config_test.go:12:14: undefined: Load
internal\config\config_test.go:33:14: undefined: Load
internal\config\config_test.go:46:12: undefined: Load
FAIL	github.com/viewred/viewred/internal/config [build failed]
```

**GREEN** (after implementation):
```
=== RUN   TestLoadDefaultsWhenFileMissing
--- PASS: TestLoadDefaultsWhenFileMissing (0.00s)
=== RUN   TestLoadFromYAMLFile
--- PASS: TestLoadFromYAMLFile (0.01s)
=== RUN   TestLoadInvalidYAMLReturnsError
--- PASS: TestLoadInvalidYAMLReturnsError (0.01s)
PASS
ok  	github.com/viewred/viewred/internal/config	1.345s
```

**Full suite** — all 4 packages pass:
```
ok  	github.com/viewred/viewred/cmd/viewred
ok  	github.com/viewred/viewred/internal/api
ok  	github.com/viewred/viewred/internal/api/handlers
ok  	github.com/viewred/viewred/internal/config
```

## Verification

- `go run ./cmd/viewred --version` → `viewred 0.1.0-dev`
- `go run ./cmd/viewred` → starts on `http://127.0.0.1:8080`

## Files created/modified

| File | Action |
|------|--------|
| `internal/config/config.go` | created |
| `internal/config/config_test.go` | created |
| `internal/config/helpers_test.go` | created |
| `cmd/viewred/main.go` | modified (config-wired) |
| `go.mod` | modified (yaml.v3 direct) |

## Commit

```
d9a4ac2 feat: add YAML config loader with safe defaults
```

## Concerns

None.
