# Identity Ingest Endpoint Interface Exploration

## Task Completion Summary

Explored the "identity ingest endpoint" referenced in the plan and discovered it is **not an HTTP endpoint** but rather an internal Go package interface for bulk email→login resolution upserts.

## Interface Type

**Internal Go package interface** - NOT an HTTP endpoint.

- **Package**: `github.com/jedarden/commitgraph/pkg/identity`
- **Main file**: `pkg/identity/ingest.go`
- **PostgreSQL implementation**: `pkg/pg/identity.go`
- **Access**: Internal cluster only, never exposed on public or authenticated-user-facing surfaces

## Required Parameters

The `ResolutionRow` struct defines all required fields:

```go
type ResolutionRow struct {
    Email      string    // Email address (primary key, required)
    Login      string    // Resolved GitHub login (required)
    Source     Source    // Source of resolution: "live", "seed", or "manual" (required)
    ResolvedAt time.Time // When this resolution was made (required)
}
```

### Field Details

| Field | Type | Validation | Notes |
|-------|------|------------|-------|
| `Email` | string | Must be non-empty | Primary key for upsert |
| `Login` | string | Must be non-empty | GitHub username |
| `Source` | Source | Must be one of: `SourceLive`, `SourceSeed`, `SourceManual` | Provenance tracking |
| `ResolvedAt` | time.Time | Must be non-zero | Timestamp of resolution |

### Source Values

- **`SourceLive`** ("live"): Resolved by live enrichment worker
- **`SourceSeed`** ("seed"): From claude-leaderboard frozen cache  
- **`SourceManual`** ("manual"): Hand-curated by operator

## Authentication/Authorization

**Cluster-internal only** - No HTTP authentication mechanism:

- Called by internal services: `user-enrichment-worker`, migration scripts
- Never exposed on public or user-facing surfaces
- Access controlled by Kubernetes network policies and cluster boundaries
- PostgreSQL credentials via `ExternallySecret` / `ClusterSecret`

## Method Signature

```go
// IngestResolution performs a bulk upsert of email resolution rows
func (i *Ingester) IngestResolution(ctx context.Context, rows []ResolutionRow) error
```

## Expected Response Format

### Success
- Returns `nil` error
- Operation is idempotent - safe to retry

### Failure
- Returns error with descriptive message
- Validation errors: `"row <idx>: <validation error>"`
- Database errors: `"bulk upsert failed: <underlying error>"`

## Error Handling Patterns

### Validation Errors (First-Fail Fast)

All rows are validated **before** any database operation:

```go
row 0: email cannot be empty
row 5: login cannot be empty  
row 12: invalid source "bogus" (must be live, seed, or manual)
row 23: resolved_at cannot be zero
```

### Database Errors

Database errors are wrapped with context:

```go
bulk upsert failed: connection refused
bulk upsert failed: duplicate key value violates unique constraint
```

### Conflict Resolution

The `ON CONFLICT` rule handles duplicates automatically:

```sql
ON CONFLICT (email) DO UPDATE
  SET login = excluded.login, 
      source = excluded.source,
      resolved_at = excluded.resolved_at
  WHERE excluded.source = 'manual'
     OR (email_resolution.source <> 'manual'
         AND excluded.resolved_at > email_resolution.resolved_at)
```

**Conflict rules**:
1. `source='manual'` always wins (overwrites any existing row)
2. Non-manual sources win only if existing row is also non-manual AND has older `resolved_at`
3. Otherwise existing row is preserved (new row silently skipped)

## Minimal Working Example

```go
package main

import (
    "context"
    "log"
    "time"

    _ "github.com/lib/pq"
    "github.com/jedarden/commitgraph/pkg/identity"
    "github.com/jedarden/commitgraph/pkg/pg"
)

func main() {
    // 1. Connect to PostgreSQL (cluster-internal)
    db, err := sql.Open("postgres", "host=postgres port=5432 dbname=commitgraph user=app password=*** sslmode=require")
    if err != nil {
        log.Fatalf("Failed to connect: %v", err)
    }
    defer db.Close()

    // 2. Create ingester
    ingester := pg.NewIdentityIngester(db)

    // 3. Prepare resolution rows
    rows := []identity.ResolutionRow{
        {
            Email:      "user1@example.com",
            Login:      "user1",
            Source:     identity.SourceSeed,
            ResolvedAt: time.Date(2024, 8, 1, 12, 0, 0, 0, time.UTC),
        },
        {
            Email:      "user2@example.com", 
            Login:      "user2",
            Source:     identity.SourceManual,
            ResolvedAt: time.Now().UTC(),
        },
    }

    // 4. Ingest (with context for timeout/cancellation)
    ctx := context.Background()
    if err := ingester.IngestResolution(ctx, rows); err != nil {
        log.Fatalf("Ingest failed: %v", err)
    }

    log.Println("Ingest completed successfully")
}
```

## Real-World Usage Examples

### 1. Seed from claude-leaderboard (`cmd/seed-email-resolution/main.go`)

Reads 349,425 frozen pairs from SQLite and ingests with `source='seed'`:

```go
rows, err := seedDB.Query("SELECT author_email, github_login, resolved_at FROM author_login_cache")
// ... parse rows ...
allRows = append(allRows, identity.ResolutionRow{
    Email:      email,
    Login:      login, 
    Source:     identity.SourceSeed,
    ResolvedAt: resolvedAt,
})

// Batch ingest (1000 rows per batch)
ingester := pg.NewIdentityIngester(postgresDB)
for i := 0; i < len(allRows); i += *batchSize {
    batch := allRows[i:end]
    if err := ingester.IngestEmailResolution(ctx, batch); err != nil {
        log.Fatalf("Failed to ingest batch %d: %v", i, err)
    }
}
```

### 2. Live enrichment worker

As GitHub API returns email→login mappings during commit enrichment:

```go
row := identity.ResolutionRow{
    Email:      commit.AuthorEmail,
    Login:      githubLogin,
    Source:     identity.SourceLive,
    ResolvedAt: time.Now().UTC(),
}
ingester.IngestResolution(ctx, []identity.ResolutionRow{row})
```

### 3. Manual alias curation (`cmd/load-admin-aliases/main.go`)

Operator-curated mappings from GitOps ConfigMap:

```go
// Read from declarative-config/k8s/ord-devimprint/commitgraph/admin-alias-configmap.yml
row := identity.ResolutionRow{
    Email:      "old-email@example.com",
    Login:      "canonical-login",
    Source:     identity.SourceManual,
    ResolvedAt: time.Now().UTC(),
}
```

## Architecture Summary

```
┌─────────────────────────────────────────────────────────────────┐
│                    Cluster Internal Only                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                   │
│  ┌──────────────────┐         ┌──────────────────┐              │
│  │ Live Enrichment  │         │  Seed Scripts    │              │
│  │     Worker       │         │  (one-off)       │              │
│  └────────┬─────────┘         └────────┬─────────┘              │
│           │                            │                         │
│           └──────────┬─────────────────┘                         │
│                      ▼                                           │
│           ┌────────────────────┐                                │
│           │  pkg/identity      │                                │
│           │  Ingester          │                                │
│           │  IngestResolution()│                               │
│           └────────┬───────────┘                                │
│                    │                                             │
│                    ▼                                             │
│           ┌────────────────────┐                                │
│           │  PostgreSQL        │                                │
│           │  email_resolution  │                                │
│           └────────────────────┘                                │
│                                                                   │
└─────────────────────────────────────────────────────────────────┘
```

## Key Implementation Notes

1. **Bulk efficiency**: Uses PostgreSQL `UNNEST` for single-round-trip bulk inserts
2. **Idempotent**: Safe to retry - `ON CONFLICT` rule handles duplicates
3. **Validation-first**: All rows validated before any database writes
4. **Partial success NOT supported**: Any error fails entire batch
5. **Context support**: `context.Context` for timeout/cancellation
6. **No HTTP**: Direct Go function call - no REST/HTTP endpoint exists

## Testing

Unit tests in `pkg/identity/ingest_test.go` and `pkg/pg/identity_test.go` cover:
- Empty batch handling
- Validation errors for all fields  
- All three source types
- Database error propagation
- Bulk batch efficiency (1000+ rows)
- SQL query correctness (ON CONFLICT rule)

---

**Conclusion**: The "identity ingest endpoint" is an internal Go package interface, not an HTTP service. It provides controlled bulk upsert of email→login resolutions with conflict resolution based on source provenance and timestamp freshness.
