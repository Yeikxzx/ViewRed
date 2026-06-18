# Task 1 Brief: Initialize git repo with .gitignore

**Plan**: `docs/superpowers/plans/2026-06-17-bootstrap-monorepo.md`
**Date**: 2026-06-17
**Project root (work from here)**: `C:\Users\Yeikxz\Desktop\ViewRed`
**Module path** (set in Task 2, not this one): `github.com/viewred/viewred`

---

## Context

This is the **first task** of the ViewRed bootstrap. The project is brand new — it has no git repo yet. Your job is to initialize git and set up the basic repo hygiene (`.gitignore`, `.gitattributes`, `.editorconfig`).

### Files already in the directory (DO NOT TOUCH in this task)

The project already has these files (created in earlier sessions, not by you):

- `.atl/` — skill registry cache (leave alone)
- `que es viewred.md` — product spec
- `design.md` — UI spec
- `docs/superpowers/specs/2026-06-17-viewred-v1-design.md` — technical design
- `docs/superpowers/plans/2026-06-17-bootstrap-monorepo.md` — this plan
- `diseño-templete/diseño-base/` — AI Studio template (will be moved in Task 5; don't touch it here)

These will be committed in later tasks. **In Task 1, only commit the three config files you create.** Running `git status` after your commit should show those files as untracked — that's expected and will be cleaned up in subsequent tasks.

### Cross-platform note

- You are on Windows. Use **Git Bash** for git commands, or PowerShell.
- The user is on Windows; subsequent tasks (including Go module init in Task 2) will work cross-platform.
- All file paths in this plan use Windows backslashes (`C:\Users\...`). Git itself uses forward slashes internally.

---

## Task: Initialize git repo with .gitignore

**Files:**
- Create: `.gitignore`
- Create: `.gitattributes`
- Create: `.editorconfig`

- [ ] **Step 1: Initialize git repo**

```bash
cd C:\Users\Yeikxz\Desktop\ViewRed
git init
git config core.autocrlf false
git config core.eol lf
```

Expected: `Initialized empty Git repository in C:/Users/.../Viewred/.git/`

- [ ] **Step 2: Write `.gitignore`**

Create `C:\Users\Yeikxz\Desktop\ViewRed\.gitignore`:

```gitignore
# OS
.DS_Store
Thumbs.db
desktop.ini

# Editor
.idea/
.vscode/
*.swp
*.swo
*~

# Go
/bin/
/dist/
/viewred
/viewred.exe
*.test
*.out
coverage.txt

# Node / Vite
node_modules/
dist/
.env
.env.local
.env.*.local
*.log
npm-debug.log*
yarn-debug.log*
pnpm-debug.log*

# ViewRed runtime data
data/
*.pcap
*.pcap.gz

# Secrets — never commit
**/.env
**/secrets/
*.key
*.pem

# Build artifacts
web/dist/
web/node_modules/
```

- [ ] **Step 3: Write `.gitattributes`**

Create `C:\Users\Yeikxz\Desktop\ViewRed\.gitattributes`:

```gitattributes
# Force LF line endings for source files
*.go          text eol=lf
*.ts          text eol=lf
*.tsx         text eol=lf
*.css         text eol=lf
*.json        text eol=lf
*.md          text eol=lf
*.yml         text eol=lf
*.yaml        text eol=lf
Makefile      text eol=lf

# Windows-specific
*.bat         text eol=crlf
*.ps1         text eol=crlf

# Binary
*.png         binary
*.jpg         binary
*.ico         binary
```

- [ ] **Step 4: Write `.editorconfig`**

Create `C:\Users\Yeikxz\Desktop\ViewRed\.editorconfig`:

```ini
root = true

[*]
charset = utf-8
end_of_line = lf
insert_final_newline = true
trim_trailing_whitespace = true
indent_style = space
indent_size = 2

[*.go]
indent_style = tab
indent_size = 4

[Makefile]
indent_style = tab
```

- [ ] **Step 5: Initial commit**

```bash
git add .gitignore .gitattributes .editorconfig
git commit -m "chore: initialize git repo with gitignore, attributes, editorconfig"
```

Expected: commit succeeds with the three files. After commit, `git status` shows a clean working tree (the other pre-existing files like `que es viewred.md` will appear as untracked, which is expected for this task — they get tracked in later tasks).

---

## Verification

Before reporting back, verify:

1. `.git/` directory exists in project root.
2. `git log --oneline` shows one commit with message starting with `chore: initialize git repo...`.
3. `git status` lists the other pre-existing files as untracked (this is expected).
4. All three files exist and contain the expected content.

---

## What to report

Write your full report to: `docs/superpowers/reports/2026-06-17-bootstrap-monorepo-task-01.md`

Report contents:
- What you implemented.
- Files created (full paths).
- Commit SHA + subject.
- Any issues or concerns (e.g. untracked files that surprised you).

Then reply with ONLY (under 15 lines):

- **Status:** DONE | DONE_WITH_CONCERNS | BLOCKED | NEEDS_CONTEXT
- Commits created (short SHA + subject)
- One-line test summary (this task has no tests, say "no tests for this task")
- Your concerns, if any
- The report file path
