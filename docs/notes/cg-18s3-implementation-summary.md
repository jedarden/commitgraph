# cg-18s3: Migration runner implementation summary

## Task completed

All acceptance criteria for cg-18s3 have been met.

## What was done

### 1. Migration tool chosen: goose (pressly/goose v3)

**Rationale**: goose is a lightweight, Postgres-native migration tool with:
- CLI-first design for Kubernetes jobs and local development
- Built-in tracking table (`goose_db_version`) for idempotency
- Simple numbered `.sql` file format with `-- +goose Up`/`-- +goose Down` directives
- Go library embedding support if needed

Full rationale documented in `docs/notes/cg-18s3-migration-tool-choice.md`

### 2. Initial schema as migration 0001

**File**: `migrations/00001_initial_schema.sql`

Contains all six tables from the plan's Postgres schema:
- `repos` - repository identity with exclusion tracking
- `users` - developer identity (no counter columns)
- `email_resolution` - email→login resolution results
- `user_aliases` - login→login alias mapping
- `repo_user_daily_tool` - main rollup (AI-tool-tagged commits only)
- `corpus_stats` - global scalar totals

Uses `IF NOT EXISTS` for safety and includes a Down migration (not used for forward-only but required by goose syntax).

### 3. Idempotency guaranteed

goose's `goose_db_version` tracking table ensures:
- Re-running `goose up` against an already-migrated database is a no-op
- Only new migrations are applied
- Partial applies are detectable (`is_applied = false`)

### 4. Local and deployment invocation documented

**Local development**:
```bash
goose postgres "user=$PGUSER password=$PGPASSWORD host=$PGHOST port=$PGPORT dbname=$PGDATABASE sslmode=disable" up
```

**Kubernetes deployment**: Example Job manifest in `migrations/example-job.yaml` for Phase 0 rollout.

### 5. Documentation in docs/notes/

- `docs/notes/cg-18s3-migration-tool-choice.md` - Full rationale and usage
- `migrations/README.md` - Developer quick reference

## Files modified/created

1. `migrations/00001_initial_schema.sql` - Initial schema (pre-existing)
2. `migrations/example-job.yaml` - Kubernetes Job reference (created)
3. `migrations/README.md` - Quick reference (pre-existing)
4. `docs/notes/cg-18s3-migration-tool-choice.md` - Full rationale (pre-existing)
5. `migrations/001_initial_schema.sql` - Duplicate removed (cleanup)

## Next steps

Phase 0 should:
1. Provision the CNPG cluster with `instances: 1` (or `3` for HA if decided)
2. Apply the initial schema: `goose postgres ... up`
3. Verify the tracking table: `SELECT * FROM goose_db_version;`
4. Rehearse backup/restore from `barmanObjectStore` before any real traffic

## Verification

To verify migrations work:
```bash
# Install goose
go install github.com/pressly/goose/v3/cmd/goose@latest

# Run migrations
cd /home/coding/commitgraph/migrations
goose postgres "user=commitgraph password=... host=localhost port=5432 dbname=commitgraph sslmode=disable" up

# Check status
goose postgres "user=commitgraph password=... host=localhost port=5432 dbname=commitgraph sslmode=disable" status

# Run again (should be no-op)
goose postgres "user=commitgraph password=... host=localhost port=5432 dbname=commitgraph sslmode=disable" up
```
