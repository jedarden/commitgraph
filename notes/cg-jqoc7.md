# Resolution Write Path Documentation (cg-jqoc7)

## Overview
This document traces the complete data flow for email→GitHub login resolution writes from the worker layer through to the SQLite persistence layer in queue-api.

## Current System Architecture

**IMPORTANT:** The current system writes to **SQLite** in queue-api, **NOT PostgreSQL**. The `email_resolution` table lives at `/data/queue.db` in the queue-api PVC in the `ord-devimprint` cluster.

## Complete Write Path Flow

### 1. Entry Point: Login Revalidation Worker
**Location:** `/home/coding/commitgraph/containers/login-revalidation-worker/main.go`

**Trigger:** When GitHub API detects a user login has been renamed (case "renamed" in `processRow`)

**Data flow:**
- Line 268: `updateEmailResolution(ctx, cfg, row.Email, *newLogin)` is called
- Lines 387-393: Constructs `ResolutionRow` struct
- Lines 400-424: Makes HTTP POST to queue-api endpoint

**Request format:**
```go
row := ResolutionRow{
    Email:       email,
    GithubLogin: newLogin,
    Provider:    "github",
    WorkerID:    cfg.WorkerID,
    ResolvedAt:  time.Now().Format(time.RFC3339),
}
```

### 2. HTTP Endpoint: Queue API
**Location:** `/home/coding/commitgraph-deprecated/containers/queue-api/internal/server/email_resolution.go`

**Endpoint:** `POST /email-resolution/resolve`

**Handler:** `handleEmailResolutionResolve()` (lines 158-206)

**Data flow:**
- Lines 163-178: Validates request (method, JSON, required fields)
- Lines 180-191: Normalizes login (lowercase) and handles unresolvable case
- Line 193: Calls `s.db.ResolveEmailResolution()` with transaction context
- Lines 194-205: Maps storage errors to HTTP status codes

**Request validation:**
- `worker_id` must not be empty
- `email` must not be empty
- `github_login` must be non-empty (use `null` for unresolvable)
- Login is normalized to lowercase at ingestion (line 190)

**Response codes:**
- 200 OK: Resolution recorded or already resolved (idempotent)
- 409 Conflict: Lease not held (expired and reclaimed, or resolved by another)
- 400 Bad Request: Invalid request body
- 500 Internal Server Error: Database operation failed

### 3. Storage Layer: SQLite Transaction
**Location:** `/home/coding/commitgraph-deprecated/containers/queue-api/internal/storage/email_resolution.go`

**Function:** `ResolveEmailResolution()` (lines 261-329)

**Data flow:**
- Lines 275-279: Begins SQLite transaction
- Lines 281-294: Executes UPDATE with lease validation
- Lines 298-304: Commits transaction on success
- Lines 308-328: Distinguishes error types for proper HTTP mapping

**Key lease validation (line 293):**
```sql
WHERE author_email = ?4
  AND provider     = ?5
  AND claimed_by   = ?6    -- Must match current worker
  AND attempted_at IS NULL  -- Must not be already resolved
```

### 4. Database Schema: SQLite Table
**Location:** `/home/coding/commitgraph-deprecated/containers/queue-api/schema.sql`

**Table:** `email_resolution` (lines 195-214)

**Database path:** `/data/queue.db` in queue-api PVC
**PVC name:** `queue-api-data`
**Cluster:** `ord-devimprint`
**Current pod:** `queue-api-c5894c469-p9rhr`

**Schema:**
```sql
CREATE TABLE IF NOT EXISTS email_resolution (
    author_email       TEXT    PRIMARY KEY,       -- Email address (key)
    github_login       TEXT,                      -- Resolved login; NULL = unresolvable
    provider           TEXT    NOT NULL DEFAULT 'github',
    status             TEXT    NOT NULL DEFAULT 'pending'
                      CHECK (status IN ('pending','claimed','resolved','unresolvable')),
    priority           INTEGER NOT NULL DEFAULT 0,       -- AI-tool commit count
    is_alias_candidate INTEGER NOT NULL DEFAULT 0,       -- 1 = flagged for alias review
    claimed_by         TEXT,                             -- Worker holding lease
    claimed_at         TEXT,
    lease_expires_at   TEXT,                             -- Lease expiration time
    attempted_at       TEXT,                             -- Set on resolve → terminal
    created_at         TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at         TEXT    NOT NULL DEFAULT (datetime('now'))
);
```

**Write operation (from storage/email_resolution.go lines 281-294):**
```sql
UPDATE email_resolution
   SET github_login       = ?1,              -- Resolved login or NULL
       status             = ?2,              -- 'resolved' or 'unresolvable'
       attempted_at       = datetime('now'), -- Marks row as terminal
       is_alias_candidate = ?3,              -- 1 for unresolvable (negative cache)
       claimed_by         = NULL,            -- Clear lease fields
       claimed_at         = NULL,
       lease_expires_at   = NULL,
       updated_at         = datetime('now')
 WHERE author_email = ?4
   AND provider     = ?5
   AND claimed_by   = ?6                    -- Lease validation
   AND attempted_at IS NULL
```

## Data Structure & Fields Written

### Input Fields (from login-revalidation-worker)
- `email` (string): Author email address
- `github_login` (string): Resolved GitHub username
- `provider` (string): Always "github" for current implementation
- `worker_id` (string): Worker instance identifier (default: hostname)
- `resolved_at` (string): ISO 8601 timestamp

### Stored Fields (in SQLite)
- `author_email` (TEXT, PRIMARY KEY): Email address
- `github_login` (TEXT): Resolved login or NULL for unresolvable
- `provider` (TEXT): Identity provider (defaults to "github")
- `status` (TEXT): 'resolved' | 'unresolvable' | 'already_resolved'
- `priority` (INTEGER): AI commit count (from original upsert)
- `is_alias_candidate` (INTEGER): 1 for unresolvable (flagged for review)
- `claimed_by` (TEXT): Worker ID (cleared on resolve)
- `claimed_at` (TEXT): Lease claim time (cleared on resolve)
- `lease_expires_at` (TEXT): Lease expiration (cleared on resolve)
- `attempted_at` (TEXT): Resolution time (marks row terminal)
- `created_at` (TEXT): Row creation time
- `updated_at` (TEXT): Last modification time

## Queue Claim/Lease Interaction Points

### Claim Process
**Function:** `ClaimEmailResolution()` (storage/email_resolution.go:130-248)

**Flow:**
1. Reclaims expired leases (lines 149-158)
2. Selects pending rows (lines 161-198)
3. Updates to 'claimed' status (lines 207-242)

**Claim order:** `WHERE provider=? AND status='pending' AND attempted_at IS NULL ORDER BY priority DESC, created_at ASC`

**Lease TTL:** Configured via `QUEUE_API_LEASE_TTL` environment variable

### Resolve Process
**Function:** `ResolveEmailResolution()` (storage/email_resolution.go:261-329)

**Lease validation:** Only the worker holding the current lease may resolve (line 293: `AND claimed_by = ?6`)

**Conflict handling:**
- `ErrAlreadyResolved`: Row already terminal (idempotent success)
- `ErrClaimConflict`: Lease expired and reclaimed by another worker

**Terminal states:** Once `attempted_at` is set (lines 285, positive OR negative), the row is never reclaimed or re-resolved.

## Alternative Write Paths

### Seed Path (Historical)
**Location:** `/home/coding/commitgraph/cmd/seed-email-resolution/main.go`

**Purpose:** Bulk load from claude-leaderboard's frozen `author_login_cache` table

**Destination:** This path also writes to **PostgreSQL** (`email_resolution` table in the commitgraph database), NOT queue-api SQLite.

**Key difference:** Uses `identity.NewIngester()` and `IngestResolution()` for PostgreSQL bulk upsert with ON CONFLICT resolution rules.

### Identity Ingest Endpoints (Future)
The deprecated queue-api code also exposes:
- `POST /email-resolution/upsert` — Enqueue emails for resolution
- `POST /email-resolution/claim` — Lease batch of emails for resolution
- `GET /email-resolution/export` — Bulk read of current resolutions

## Critical Distinction: SQLite vs PostgreSQL

**Current resolution writes go to SQLite in queue-api:**
- Database: `/data/queue.db` in queue-api PVC
- Table: `email_resolution`
- Purpose: Worker coordination, lease management, negative caching

**PostgreSQL `email_resolution` table is different:**
- Database: `commitgraph` PostgreSQL database
- Purpose: Canonical identity resolution store with conflict resolution
- Uses different schema with ON CONFLICT rules for source precedence

These are **two separate systems** that happen to share a table name.

## Access & Verification

**Queue pod:** `queue-api-c5894c469-p9rhr` (namespace `commitgraph`)
**Cluster:** `ord-devimprint`
**Read-only proxy:** `http://kubectl-proxy-ord-devimprint:8001`
**Admin kubeconfig:** `/home/coding/.kube/ord-devimprint-admin.kubeconfig` (expires ~3 days)

**Verification query (once admin access restored):**
```bash
kubectl --kubeconfig=~/.kube/ord-devimprint-admin.kubeconfig exec -n commitgraph queue-api-c5894c469-p9rhr -- \
  sqlite3 /data/queue.db "SELECT COUNT(*) FROM email_resolution WHERE status='resolved';"
```

## References

- Worker code: `/home/coding/commitgraph/containers/login-revalidation-worker/main.go:383-425`
- HTTP handler: `/home/coding/commitgraph-deprecated/containers/queue-api/internal/server/email_resolution.go:158-206`
- Storage layer: `/home/coding/commitgraph-deprecated/containers/queue-api/internal/storage/email_resolution.go:261-329`
- Database schema: `/home/coding/commitgraph-deprecated/containers/queue-api/schema.sql:195-214`

---

**Generated:** 2026-08-06  
**Bead:** cg-jqoc7 (Locate and document current resolution write path)
