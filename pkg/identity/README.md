# Identity Ingest Path

This package implements the single bulk-upsert path that all writers use to insert email→login resolutions into the `email_resolution` table, ensuring consistent conflict resolution per [plan.md](../../../docs/plan/plan.md).

## Overview

The ingest path applies a deterministic conflict resolution rule when inserting or updating email resolutions:

- **Manual source always wins** - A manually curated resolution is never overwritten
- **Otherwise, newer wins** - The resolution with the newer `resolved_at` timestamp wins
- **Provenance is auditable** - Every row carries its `source` (`live`/`seed`/`manual`)

## Writers

All three writers go through this single ingest path:

1. **Live enrichment worker** (`source='live'`) - Resolves emails via GitHub API during live operation
2. **Claude-leaderboard seed** (`source='seed'`) - Bulk import of 349,425 frozen pairs from claude-leaderboard
3. **Manual curation** (`source='manual'`) - Operator-authored aliases

## Conflict Resolution Rule

The PostgreSQL `ON CONFLICT` clause implements the rule exactly as specified in plan.md:

```sql
ON CONFLICT (email) DO UPDATE
  SET login = excluded.login, source = excluded.source, resolved_at = excluded.resolved_at
  WHERE excluded.source = 'manual'
     OR (email_resolution.source <> 'manual'
         AND excluded.resolved_at > email_resolution.resolved_at)
```

### Semantics

| Incoming Source | Existing Source | Result |
|-----------------|-----------------|--------|
| `manual` | any | Incoming **wins** (manual always overwrites) |
| any | `manual` | Existing **wins** (manual is never overwritten) |
| `live`/`seed` | `live`/`seed`, newer | Existing **wins** (older incoming is skipped) |
| `live`/`seed` | `live`/`seed`, older | Incoming **wins** (newer resolution) |

Rows that lose the conflict check are **silently skipped** (upsert semantics, not batch failure).

## Usage

### Basic Example

```go
import (
    "context"
    "time"
    
    "github.com/jedarden/commitgraph/pkg/identity"
    "github.com/jedarden/commitgraph/pkg/pg"
)

func main() {
    // Initialize database connection (example with database/sql)
    db, _ := sql.Open("postgres", "...")
    
    // Create PostgreSQL ingester
    ingester := pg.NewIdentityIngester(db)
    
    // Create identity ingester
    identityIngester := identity.NewIngester(ingester)
    
    // Prepare batch of resolutions
    rows := []identity.ResolutionRow{
        {
            Email:      "user@example.com",
            Login:      "userlogin",
            Source:     identity.SourceLive,
            ResolvedAt: time.Now().UTC(),
        },
        // ... more rows
    }
    
    // Ingest batch
    err := identityIngester.IngestResolution(context.Background(), rows)
    if err != nil {
        // Handle error
    }
}
```

### With Transaction

```go
func ingestWithTx(db *sql.DB, rows []identity.ResolutionRow) error {
    tx, _ := db.Begin()
    defer tx.Rollback()
    
    ingester := pg.NewIdentityIngester(tx)
    identityIngester := identity.NewIngester(ingester)
    
    if err := identityIngester.IngestResolution(context.Background(), rows); err != nil {
        return err
    }
    
    return tx.Commit()
}
```

### Large Batch (349K+ rows)

The implementation uses PostgreSQL's `UNNEST` with array parameters for efficient bulk insert:

```go
// This handles 349,425 rows from claude-leaderboard seed efficiently
// Single round-trip, no per-row overhead
rows := loadClaudeLeaderboardSeed() // 349,425 rows
err := identityIngester.IngestResolution(ctx, rows)
```

## Schema

The `email_resolution` table (from [migrations/001_initial_schema.sql](../../../migrations/001_initial_schema.sql)):

```sql
CREATE TABLE email_resolution (
  email       TEXT PRIMARY KEY,
  login       TEXT NOT NULL,
  source      TEXT NOT NULL,          -- 'live' | 'seed' | 'manual'
  resolved_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX ON email_resolution (login);
```

## Validation

The ingest path validates all rows before database insertion:

- `email` cannot be empty
- `login` cannot be empty
- `source` must be one of: `live`, `seed`, `manual`
- `resolved_at` cannot be zero

Validation failures return an error with the row index: `"row 42: email cannot be empty"`.

## Design Rationale

### Why a single path?

The plan specifies this as the single path all writers use because:

1. **Consistent conflict resolution** - No writer can bypass the rule
2. **Auditable provenance** - Every row's source is recorded
3. **Idempotent operation** - Safe to retry (e.g., during migration)

### Why bulk operation?

The claude-leaderboard seed alone is 349,425 rows. Row-at-a-time inserts would require 349,425 round trips. The bulk implementation uses a single SQL statement with `UNNEST` array parameters.

### Why `UNNEST` instead of `VALUES`?

PostgreSQL's `UNNEST` with typed array parameters (`::text[]`, `::timestamptz[]`) is:

- **Efficient** - Single query execution plan
- **Type-safe** - Parameter typing prevents injection
- **Scalable** - Handles thousands of rows per batch

## Implementation Details

### PostgreSQL Implementation

`pkg/pg/identity.go` implements the `identity.DB` interface for PostgreSQL using `database/sql`:

```go
type DB interface {
    IngestEmailResolution(ctx context.Context, rows []identity.ResolutionRow) error
}
```

The concrete implementation uses `UNNEST` to expand array parameters into rows:

```sql
INSERT INTO email_resolution (email, login, source, resolved_at)
SELECT unnest($1::text[]),
       unnest($2::text[]),
       unnest($3::text[]),
       unnest($4::timestamptz[])
```

### Mock Testing

Both packages include comprehensive tests using mock implementations:

- `pkg/identity/ingest_test.go` - Tests validation logic
- `pkg/pg/identity_test.go` - Tests SQL generation and conflict rule

Run tests:

```bash
go test ./pkg/identity/... -v
go test ./pkg/pg/... -v
```

## References

- [plan.md - Identity ingest endpoint](../../../docs/plan/plan.md#identity-ingest-endpoint)
- [plan.md - Postgres schema](../../../docs/plan/plan.md#postgres-schema)
- [migrations/001_initial_schema.sql](../../../migrations/001_initial_schema.sql)
