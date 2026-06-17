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
