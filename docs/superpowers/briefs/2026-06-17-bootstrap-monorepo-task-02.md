# Task 2 Brief: Initialize Go module with minimal main.go

**Plan**: `docs/superpowers/plans/2026-06-17-bootstrap-monorepo.md`
**Date**: 2026-06-17
**Project root (work from here)**: `C:\Users\Yeikxz\Desktop\ViewRed`

---

## Context

This is **Task 2 of 12** in the ViewRed bootstrap. Task 1 just initialized git (commit `003fd25`). Your job is to add a Go module with a minimal main entry point that supports `--version` and a placeholder `main()` body. Subsequent tasks will add the Gin router (Task 3), config loader (Task 4), etc.

### Interfaces and decisions from earlier tasks

- **No earlier code dependencies** — Task 1 only created config files (.gitignore, .gitattributes, .editorconfig). This is the first code in the repo.
- **Module path**: `github.com/viewred/viewred` (set in Step 1, do not change).
- **Go version**: 1.22+ (user has 1.26 installed).
- **Cross-platform**: every command must work on both Windows and Linux. The `go` toolchain handles this for us; no special handling needed beyond what the brief specifies.

### Files already in the directory (DO NOT TOUCH in this task)

- `.gitignore`, `.gitattributes`, `.editorconfig` (from Task 1, committed)
- `que es viewred.md`, `design.md` (will be tracked in later commits)
- `docs/superpowers/specs/`, `docs/superpowers/plans/`, `docs/superpowers/briefs/`, `docs/superpowers/reports/`, `docs/superpowers/progress/` (your work artifacts)
- `diseño-templete/` (will be moved in Task 5)

---

## Task: Initialize Go module with minimal main.go

**Files:**
- Create: `go.mod`
- Create: `cmd/viewred/main.go`
- Create: `cmd/viewred/main_test.go`

- [ ] **Step 1: Initialize Go module**

```bash
cd C:\Users\Yeikxz\Desktop\Viewred
go mod init github.com/viewred/viewred
```

Expected: creates `go.mod` with module name. If `go.sum` is also created, leave it.

- [ ] **Step 2: Write failing test for main entry point**

Create `C:\Users\Yeikxz\Desktop\ViewRed\cmd\viewred\main_test.go`:

```go
package main

import (
	"testing"
)

// TestVersion asserts the binary has a version string for --version output.
func TestVersion(t *testing.T) {
	if Version == "" {
		t.Fatal("Version should not be empty")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
cd C:\Users\Yeikxz\Desktop\ViewRed
go test ./cmd/viewred/...
```

Expected: FAIL with `undefined: Version` (Go compiler error since `Version` doesn't exist yet).

- [ ] **Step 4: Write minimal main.go with version constant**

Create `C:\Users\Yeikxz\Desktop\ViewRed\cmd\viewred\main.go`:

```go
// Package main is the entry point for the ViewRed binary.
//
// ViewRed is a network detection & response tool with local AI.
package main

import (
	"fmt"
	"os"
)

// Version is the semantic version of this build, set via -ldflags at build time.
var Version = "0.1.0-dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("viewred %s\n", Version)
		return
	}
	fmt.Printf("ViewRed %s — bootstrap stub. Real implementation comes in plan 2.\n", Version)
}
```

- [ ] **Step 5: Run test to verify it passes**

```bash
cd C:\Users\Yeikxz\Desktop\ViewRed
go test ./cmd/viewred/...
go run ./cmd/viewred --version
```

Expected: test PASS, then `viewred 0.1.0-dev` printed.

- [ ] **Step 6: Commit**

```bash
cd C:\Users\Yeikxz\Desktop\ViewRed
git add go.mod go.sum cmd/
git commit -m "feat: initialize Go module with versioned main entry point"
```

(If `go.sum` doesn't exist, omit it from the add — `git add go.mod cmd/` is fine.)

---

## Verification

Before reporting back, verify:

1. `go.mod` exists with module `github.com/viewred/viewred`.
2. `cmd/viewred/main.go` and `cmd/viewred/main_test.go` exist with the exact content from the brief.
3. `go test ./cmd/viewred/...` passes.
4. `go run ./cmd/viewred --version` prints `viewred 0.1.0-dev`.
5. The commit was made with the expected message.

---

## What to report

Write your full report to: `C:\Users\Yeikxz\Desktop\ViewRed\docs\superpowers\reports\2026-06-17-bootstrap-monorepo-task-02.md`

Include:
- What you implemented
- TDD evidence: RED (failing test output before main.go) and GREEN (passing test + run output)
- Files created
- Commit SHA + subject
- Any concerns

Then reply with ONLY (under 15 lines):

- **Status:** DONE | DONE_WITH_CONCERNS | BLOCKED | NEEDS_CONTEXT
- Commits created (short SHA + subject)
- One-line test summary (e.g. "TestVersion passes, go run --version prints expected output")
- Your concerns, if any
- The report file path

## When you're stuck or unsure

If `go mod init` fails, or Go isn't on PATH, or anything unexpected, **stop and ask** with NEEDS_CONTEXT. Don't guess.