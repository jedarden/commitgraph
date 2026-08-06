# Postgres Migrations

This directory contains **goose** migrations for the commitgraph v2 CNPG Postgres cluster.

## What is goose?

goose is a lightweight migration tool with CLI support for Kubernetes jobs and Go library embedding. See `docs/notes/cg-18s3-migration-tool-choice.md` for the full rationale.

## File format

Migrations are numbered SQL files with goose directives:

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS repos (...);

-- +goose Down
DROP TABLE IF EXISTS repos;
```

- `00001_initial_schema.sql` - Initial schema (repos, users, email_resolution, user_aliases, repo_user_daily_tool, corpus_stats)
- `00002_*.sql` - Future migrations (not yet created)

## Non-migration files

Files that are **NOT** managed by goose:

- `invariant_2_no_out_of_range_days.sql` - Data invariant check for validation/auditing

## Running migrations locally

### Install goose CLI

```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
```

Or use the Docker image:

```bash
docker pull ghcr.io/pressly/goose:latest
```

### Run migrations

```bash
# Set Postgres connection parameters
export PGHOST=localhost
export PGPORT=5432
export PGDATABASE=commitgraph
export PGUSER=commitgraph
export PGPASSWORD=your_password

# Run all pending migrations
goose postgres "user=$PGUSER password=$PGPASSWORD host=$PGHOST port=$PGPORT dbname=$PGDATABASE sslmode=disable" up

# Check migration status
goose postgres "user=$PGUSER password=$PGPASSWORD host=$PGHOST port=$PGPORT dbname=$PGDATABASE sslmode=disable" status
```

### Reset database (WARNING: destructive)

```bash
# This runs the Down migrations and then Up again - USE WITH CAUTION
goose postgres "..." reset
```

## Running migrations in Kubernetes

From an init container or Job in the CNPG cluster namespace:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: commitgraph-migrate
spec:
  template:
    spec:
      containers:
      - name: goose
        image: ghcr.io/pressly/goose:latest
        command:
        - goose
        - postgres
        - "user=$(POSTGRES_USER) password=$(POSTGRES_PASSWORD) host=$(POSTGRES_HOST) port=$(POSTGRES_PORT) dbname=$(POSTGRES_DB) sslmode=require"
        - up
        env:
        - name: POSTGRES_HOST
          value: commitgraph-rw.default.svc  # CNPG read-write service
        - name: POSTGRES_USER
          valueFrom:
            secretKeyRef:
              name: commitgraph-app
              key: username
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: commitgraph-app
              key: password
        - name: POSTGRES_DB
          value: commitgraph
        - name: POSTGRES_PORT
          value: "5432"
      restartPolicy: Never
```

**Important:** Use the `-rw` (read-write) service for migrations. The `-ro` service is for read-only workloads.

## Migration safety

- **Re-running is safe:** `goose up` checks `goose_db_version` and skips applied versions
- **Transactions:** Each migration runs in a single transaction — any failure rolls back completely
- **Partial applies detectable:** If migration N fails, `goose_db_version` shows `is_applied = false` for version N
- **Idempotent DDL:** All CREATE statements use `IF NOT EXISTS` for extra safety

## The tracking table

goose automatically creates `goose_db_version`:

```sql
CREATE TABLE goose_db_version (
    id INTEGER PRIMARY KEY,
    version_id BIGINT NOT NULL,
    is_applied BOOLEAN NOT NULL,
    tstamp TIMESTAMP NULL
);
```

After migration 00001 runs successfully:

| id | version_id | is_applied | tstamp |
|----|------------|------------|--------|
| 1  | 1          | true       | now    |

## Development workflow

1. Create a new migration file: `00002_descriptive_name.sql`
2. Add `-- +goose Up` and `-- +goose Down` sections
3. Test locally: `goose postgres "..." up`
4. Verify: Connect to Postgres and check schema
5. Commit: The file is now part of the tracked migration history

## See also

- `docs/notes/cg-18s3-migration-tool-choice.md` - Why goose and how to use it
- `docs/plan/plan.md` - Full commitgraph v2 architecture plan
