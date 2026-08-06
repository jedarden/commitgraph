# SQLite Resolution Write Locations (cg-m6dkr)

## Finding Summary

**IMPORTANT CORRECTION**: The commitgraph system uses **PostgreSQL**, not SQLite, for the `email_resolution` table. The task description mentioned SQLite, but all writes in this codebase go to PostgreSQL.

## Database Schema

**Table**: `email_resolution`  
**Location**: `migrations/00001_initial_schema.sql:31-36`

```sql
CREATE TABLE IF NOT EXISTS email_resolution (
  email       TEXT PRIMARY KEY,
  login       TEXT NOT NULL,
  source      TEXT NOT NULL,          -- 'live' | 'seed' | 'manual'
  resolved_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS email_resolution_login_idx ON email_resolution (login);
```

**Columns**:
- `email` - Email address (primary key)
- `login` - Resolved GitHub login
- `source` - Provenance: 'live' (enrichment worker), 'seed' (claude-leaderboard), 'manual' (operator)
- `resolved_at` - Timestamp when this resolution was made

---

## Write Location #1: Core PostgreSQL Insert/Upsert

**File**: `pkg/pg/identity.go:91-138`  
**Function**: `IdentityIngester.IngestEmailResolution(ctx, rows)`

**SQL Operation**: Bulk INSERT with ON CONFLICT DO UPDATE

```go
INSERT INTO email_resolution (email, login, source, resolved_at)
SELECT unnest($1::text[]),
       unnest($2::text[]),
       unnest($3::text[]),
       unnest($4::timestamptz[])
ON CONFLICT (email) DO UPDATE
  SET login = excluded.login,
      source = excluded.source,
      resolved_at = excluded.resolved_at
  WHERE excluded.source = 'manual'
     OR (email_resolution.source <> 'manual'
         AND excluded.resolved_at > email_resolution.resolved_at)
```

**Conflict Resolution Rule**:
1. Manual source always wins (overwrites any existing row)
2. Non-manual sources win only if existing row is also non-manual AND new resolved_at is newer
3. Otherwise, existing row is preserved

---

## Write Location #2: High-Level Validation Layer

**File**: `pkg/identity/ingest.go:94-108`  
**Function**: `Ingester.IngestResolution(ctx, rows)`

**Flow**:
1. Validates all rows (email/login non-empty, valid source, resolved_at not zero)
2. Delegates to database implementation (`IngestEmailResolution`)

---

## Entry Points that Trigger Writes

### 1. Live Enrichment Worker

**File**: `containers/user-enrichment-worker/worker.py:139-158`  
**Function**: `record_resolution(email, login)`

**Flow**:
1. Worker claims batch from queue-api
2. Resolves email → GitHub login via GitHub API
3. POST to `{INGEST_URL}/identity/ingest` with source='live'
4. Endpoint calls `Ingester.IngestResolution()` → PostgreSQL INSERT

**Source**: 'live'  
**Trigger**: New emails discovered during commit processing

---

### 2. Login Revalidation Worker

**File**: `containers/login-revalidation-worker/main.go:384-425`  
**Function**: `updateEmailResolution(ctx, cfg, email, newLogin)`

**Flow**:
1. Worker samples rows from `email_revalidation` table
2. Checks login liveness via GitHub API
3. On rename detected: POST to `{QUEUE_API_URL}/email-resolution/resolve`
4. Endpoint writes to email_resolution with source='live'

**Source**: 'live'  
**Trigger**: GitHub login rename detected

---

### 3. Seed Script: claude-leaderboard import

**File**: `cmd/seed-email-resolution/main.go` (reads from SQLite backup)  
**Also**: `cmd/seed-author-login-cache/main.go`

**Flow**:
1. Reads 349K+ (email, login, resolved_at) triples from claude-leaderboard SQLite
2. Converts to `ResolutionRow` with source='seed'
3. Calls `Ingester.IngestResolution()` in batches
4. PostgreSQL bulk INSERT with conflict resolution

**Source**: 'seed'  
**Trigger**: One-time manual migration from frozen claude-leaderboard cache

---

## Data Flow Diagram

```
┌─────────────────────────────────────┐
│  Live Resolution Path               │
├─────────────────────────────────────┤
│ user-enrichment-worker             │
│   ├─ claim batch from queue-api    │
│   ├─ GitHub API lookup              │
│   └─ POST /identity/ingest          │
│        ↓                            │
│    identity.Ingester                │
│        ↓                            │
│    pg.IdentityIngester              │
│        ↓                            │
│    PostgreSQL INSERT/UPDATE         │
└─────────────────────────────────────┘

┌─────────────────────────────────────┐
│  Revalidation Path                 │
├─────────────────────────────────────┤
│ login-revalidation-worker          │
│   ├─ sample email_revalidation      │
│   ├─ GitHub API check               │
│   ├─ detect rename                  │
│   └─ POST /email-resolution/resolve │
│        ↓                            │
│    (via queue-api endpoint)         │
│        ↓                            │
│    PostgreSQL INSERT/UPDATE         │
└─────────────────────────────────────┘

┌─────────────────────────────────────┐
│  Seed Path (one-time)               │
├─────────────────────────────────────┤
│ seed-email-resolution               │
│   ├─ read claude-leaderboard SQLite │
│   └─ identity.Ingester              │
│        ↓                            │
│    PostgreSQL INSERT/UPDATE         │
└─────────────────────────────────────┘
```

---

## Key Files

| File | Purpose |
|------|---------|
| `pkg/pg/identity.go:91-138` | Core PostgreSQL INSERT/UPDATE logic |
| `pkg/identity/ingest.go:94-108` | High-level validation + dispatch |
| `containers/user-enrichment-worker/worker.py:139-158` | Live resolution entry point |
| `containers/login-revalidation-worker/main.go:384-425` | Revalidation update path |
| `migrations/00001_initial_schema.sql:31-36` | Table schema definition |

---

## Acceptance Criteria

- [x] All SQLite write locations for resolution data are identified  
  *(Note: System uses PostgreSQL, not SQLite)*
- [x] Current flow is documented (trigger point → write function)
- [x] Table schema and columns are noted
