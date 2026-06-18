# Task 9 Brief: Add docker-compose for dev DB

**Plan**: `docs/superpowers/plans/2026-06-17-bootstrap-monorepo.md`
**Date**: 2026-06-17
**Project root (work from here)**: `C:\Users\Yeikxz\Desktop\ViewRed`

---

## Context

This is **Task 9 of 12** in the ViewRed bootstrap. Tasks 1-8 are done (Go backend, web/, Makefile).

Your job: create a `docker-compose.dev.yml` for the local dev database (PostgreSQL + TimescaleDB) plus a `README.md` for the `deploy/` directory. These are static files — no code, no tests, no processes to run.

### Interfaces and decisions from earlier tasks

- **Backend** will connect to PostgreSQL with this connection string: `postgres://viewred:viewred_dev_only@localhost:5432/viewred`. The docker-compose must match.
- **PostgreSQL 15+ with TimescaleDB 2.x** is the target.
- **User does NOT have Docker on this machine** (per sdd-init R2). The docker-compose file is for future use when the user installs Docker, OR for users who already have it. Don't try to run docker in this task.
- **No TDD** — these are config files.
- **Cross-platform note**: docker-compose is cross-platform. The user just doesn't have Docker installed.

### Files already in the directory (DO NOT TOUCH)

- All existing files — KEEP
- You are only creating files under `deploy/`

---

## Task: Add docker-compose for dev DB

**Files:**
- Create: `deploy/docker-compose.dev.yml`
- Create: `deploy/README.md`

- [ ] **Step 1: Write docker-compose for dev DB**

Create `C:\Users\Yeikxz\Desktop\ViewRed\deploy\docker-compose.dev.yml`:

```yaml
# Local dev database for ViewRed.
# Usage: docker compose -f deploy/docker-compose.dev.yml up -d
# Note: requires Docker. If you don't have Docker, install PostgreSQL+TimescaleDB
# natively and create a database called "viewred" with user "viewred".

services:
  postgres:
    image: timescale/timescaledb:latest-pg15
    container_name: viewred-postgres
    environment:
      POSTGRES_DB: viewred
      POSTGRES_USER: viewred
      POSTGRES_PASSWORD: viewred_dev_only
    ports:
      - "5432:5432"
    volumes:
      - viewred-pgdata:/home/postgres/pgdata/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U viewred"]
      interval: 5s
      timeout: 5s
      retries: 5

volumes:
  viewred-pgdata:
```

- [ ] **Step 2: Write deploy/README.md**

Create `C:\Users\Yeikxz\Desktop\ViewRed\deploy\README.md`:

````markdown
# Deploy

This directory contains deployment artifacts.

## `docker-compose.dev.yml`

Local development database (PostgreSQL + TimescaleDB).

Start it:
```bash
docker compose -f deploy/docker-compose.dev.yml up -d
```

Stop it:
```bash
docker compose -f deploy/docker-compose.dev.yml down
```

Reset data (delete the volume):
```bash
docker compose -f deploy/docker-compose.dev.yml down -v
```

Connection string for the Go app:
```
postgres://viewred:viewred_dev_only@localhost:5432/viewred
```

If you don't have Docker, install PostgreSQL 15+ and TimescaleDB 2.x natively,
create a database and user matching the connection string above.
````

- [ ] **Step 3: Commit**

```bash
cd C:\Users\Yeikxz\Desktop\ViewRed
git add deploy/
git status
git commit -m "chore: add docker-compose for local dev database"
```

---

## Verification

Before reporting back, verify:

1. `deploy/docker-compose.dev.yml` exists with the brief content (PostgreSQL 15 + TimescaleDB, healthcheck, volume).
2. `deploy/README.md` exists with the brief content (start/stop/reset commands, connection string, native install alternative).
3. Single commit on `main` with the expected message.

**Do NOT try to run `docker compose` — the user doesn't have Docker installed, and the file is for future use.**

---

## What to report

Write your full report to: `C:\Users\Yeikxz\Desktop\ViewRed\docs\superpowers\reports\2026-06-17-bootstrap-monorepo-task-09.md`

Include:
- What you implemented (file paths, byte sizes)
- `git status` before and after the commit
- Commit SHA + subject
- Any concerns

Then reply with ONLY (under 15 lines):

- **Status:** DONE | DONE_WITH_CONCERNS | BLOCKED | NEEDS_CONTEXT
- Commits created (short SHA + subject)
- One-line summary
- Your concerns, if any
- The report file path

## When you're stuck

If anything unusual, **stop and ask** with NEEDS_CONTEXT.