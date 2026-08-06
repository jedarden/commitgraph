# Migration tool choice: goose for CNPG Postgres forward-only migrations

## The question

Which migration runner should commitgraph v2 use for its CNPG Postgres cluster? The pipeline needs a mechanism to version and apply schema changes, both locally (against dev Postgres) and in Phase 0 cluster rollout. No migration precedent exists in this codebase — the closest prior art, `iad-ci/queue-db`, is cited only for CNPG resource shape, not tooling.

## The answer: pressly/goose

**goose** is a lightweight migration tool with excellent CLI support for Kubernetes jobs and a Go library for embedding. It uses numbered `.sql` files with a `goose_db_version` tracking table and supports forward-only migrations.

## Why goose over alternatives

- **CLI-first:** `goose up` runs from any container in Kubernetes — init containers, jobs, or sidecars. No custom Go binary needed.
- **Postgres-native:** First-class Postgres driver with connection pooling and proper transaction handling.
- **Tracking table built-in:** Creates `goose_db_version` automatically with `(version_id, is_applied)` columns — partial applies are detectable by `is_applied = false`.
- **Simple file format:** `00001_name.sql` with `-- +goose Up` and `-- +goose Down` sections (even if we never use Down).
- **Idempotent by default:** Re-running `goose up` against an already-migrated database is a no-op — it checks the tracking table and skips applied versions.
- **Go-embeddable:** If the pipeline ever needs to run migrations from within a Go binary, goose has a `github.com/pressly/goose/v3` library.

## Alternatives considered and rejected

- **golang-migrate/migrate:** Also excellent, but goose's simpler CLI and file naming won out. Both would have worked.
- **rubenv/sql-migrate:** Go library only, no built-in CLI. Would require writing a custom wrapper for Kubernetes jobs — unnecessary complexity.
- **Raw psql with hand-rolled tracking:** Reinvents the wheel. Migration runners are a solved problem — the tracking table logic is subtle (partial applies, transaction boundaries, locking).

## How it's invoked

### Local development

```bash
# Install goose CLI
go install github.com/pressly/goose/v3/cmd/goose@latest

# Run migrations against local Postgres
export PGHOST=localhost
export PGPORT=5432
export PGDATABASE=commitgraph
export PGUSER=commitgraph
export PGPASSWORD=...

goose postgres "user=$PGUSER password=$PGPASSWORD host=$PGHOST port=$PGPORT dbname=$PGDATABASE sslmode=disable" up
```

### Kubernetes (Phase 0 rollout)

From an init container or Job in the CNPG cluster's namespace:

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
        image: ghcr.io/pressly/goose:latest  # or custom image with goose binary
        command:
        - goose
        - postgres
        - "user=$(POSTGRES_USER) password=$(POSTGRES_PASSWORD) host=$(POSTGRES_HOST) port=$(POSTGRES_PORT) dbname=$(POSTGRES_DB) sslmode=require"
        - up
        env:
        - name: POSTGRES_HOST
          value: commitgraph-rw.default.svc  # CNPG service
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

CNPG creates `-rw` (read-write) and `-ro` (read-only) services. Migrations must target `-rw`.

## File convention

Migrations live in `migrations/` with numeric prefixes:

```
migrations/
├── 00001_initial_schema.sql          # Existing 001_initial_schema.sql → 00001
├── 00002_add_corpus_stats.sql       # Future migration
└── ...
```

Each file uses goose's directive syntax:

```sql
-- +goose Up
CREATE TABLE repos (...);
CREATE INDEX ON email_resolution (login);
-- ... rest of schema DDL

-- +goose Down
-- Not used for forward-only migrations, but syntax required
DROP TABLE IF EXISTS repos CASCADE;
```

goose automatically:
- Wraps each Up block in a transaction
- Inserts a row into `goose_db_version` on success
- Skips already-applied versions on re-run

## Safety and idempotency

- **Re-running is safe:** `goose up` checks `goose_db_version` and only runs new migrations.
- **Partial applies are detectable:** If migration N fails mid-transaction, `goose_db_version` shows `is_applied = false` for version N. Re-running `goose up` retries only N.
- **Transactionally applied:** Each migration file runs in a single transaction. Any failure rolls back the entire migration file, leaving the database in a consistent state.
