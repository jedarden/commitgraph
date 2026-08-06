# Current Resolution Write Path to queue-api SQLite

**Date:** 2026-08-06  
**Bead:** cg-jqoc7  
**Task:** Document the current code that writes resolved email-github pairs to queue-api SQLite

## Overview

The current resolution write path flows from the **login-revalidation-worker** (in the new commitgraph system) to **queue-api** (from the deprecated commitgraph-deprecated system) where resolved pairs are stored in SQLite.

## Data Flow Architecture

```
login-revalidation-worker (new system)
    ↓
    HTTP POST /email-resolution/resolve
    ↓
queue-api HTTP handler (deprecated system)
    ↓
queue-api storage layer (deprecated system) 
    ↓
SQLite email_resolution table (deprecated system)
```

## 1. Login Revalidation Worker Entry Point

**File:** `/home/coding/commitgraph/containers/login-revalidation-worker/main.go`

**Key Function:** `updateEmailResolution()` (lines 384-425)

```go
func updateEmailResolution(ctx context.Context, cfg *Config, email, newLogin string) error {
    // POST to queue-api endpoint
    row := ResolutionRow{
        Email:       email,
        GithubLogin: newLogin,
        Provider:    "github",
        WorkerID:    cfg.WorkerID,
        ResolvedAt:  time.Now().Format(time.RFC3339),
    }
    
    // POST to cfg.QueueAPIURL + "/email-resolution/resolve"
    // Uses optional Bearer token from QUEUE_API_INTERNAL_TOKEN
    // Returns status 200 on success
}
```

**Data Written:**
- `email` - The author email being resolved
- `github_login` - The resolved GitHub username (lowercase)
- `provider` - Always "github" currently
- `worker_id` - Worker identifier (hostname by default)
- `resolved_at` - ISO 8601 timestamp of resolution

**Trigger:** Called when login revalidation detects a GitHub account rename (status="renamed" in line 264-275)

## 2. Queue-API HTTP Handler

**File:** `/home/coding/commitgraph-deprecated/containers/queue-api/internal/server/email_resolution.go`

**Key Function:** `handleEmailResolutionResolve()` (lines 158-206)

```go
func (s *Server) handleEmailResolutionResolve(w http.ResponseWriter, r *http.Request) {
    // Validate request: method, worker_id, email
    // Normalize github_login to lowercase
    // Handle NULL for unresolvable cases
    // Call s.db.ResolveEmailResolution()
    
    // Returns: status "resolved" | "unresolvable" | "already_resolved"
}
```

**Request Structure:**
```go
type emailResolutionResolveRequest struct {
    Provider    string  // "github"
    WorkerID    string  // Worker identifier
    Email       string  // Author email
    GitHubLogin *string // NULL for unresolvable, non-empty string for resolved
}
```

## 3. Queue-API Storage Layer

**File:** `/home/coding/commitgraph-deprecated/containers/queue-api/internal/storage/email_resolution.go`

**Key Function:** `ResolveEmailResolution()` (lines 261-329)

```go
func (db *DB) ResolveEmailResolution(ctx context.Context, provider, email, workerID, login string, unresolvable bool) (string, error) {
    // Begin transaction
    // UPDATE email_resolution SET:
    //   github_login = ?1 (login or NULL)
    //   status = ?2 ("resolved" or "unresolvable") 
    //   attempted_at = datetime('now')
    //   is_alias_candidate = ?3 (0 or 1)
    //   claimed_by = NULL
    //   claimed_at = NULL
    //   lease_expires_at = NULL
    // WHERE author_email = ?4 AND provider = ?5 AND claimed_by = ?6 AND attempted_at IS NULL
    
    // Commit transaction
    // Return status or error (ErrAlreadyResolved, ErrClaimConflict)
}
```

**Lease Validation:** Only the worker holding the current lease can resolve. This prevents race conditions where expired workers might clobber newer resolutions.

**Idempotency:** Returns `ErrAlreadyResolved` if the email is already terminal (attempted_at IS NOT NULL), making the operation safe to retry.

## 4. SQLite Database Schema

**File:** `/home/coding/commitgraph-deprecated/containers/queue-api/schema.sql`

**Table:** `email_resolution` (lines 195-214)

```sql
CREATE TABLE IF NOT EXISTS email_resolution (
    author_email       TEXT    PRIMARY KEY,
    github_login       TEXT,                              -- NULL ⇒ provable non-match
    provider           TEXT    NOT NULL DEFAULT 'github',
    status             TEXT    NOT NULL DEFAULT 'pending'
                       CHECK (status IN ('pending','claimed','resolved','unresolvable')),
    priority           INTEGER NOT NULL DEFAULT 0,       -- AI-tool commit count
    is_alias_candidate INTEGER NOT NULL DEFAULT 0,       -- 1 for negative result flagging
    claimed_by         TEXT,                             -- worker holding lease
    claimed_at         TEXT,
    lease_expires_at   TEXT,                             -- past ⇒ reclaimable
    attempted_at       TEXT,                             -- set on resolve ⇒ terminal
    created_at         TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at         TEXT    NOT NULL DEFAULT (datetime('now'))
);

-- Index for claim operations (highest priority, oldest first)
CREATE INDEX idx_email_resolution_claim
    ON email_resolution (provider, status, attempted_at, priority DESC, created_at);
```

**Fields Written on Resolution:**
- `github_login` - Resolved login or NULL (negative cache)
- `status` - Set to "resolved" or "unresolvable"
- `attempted_at` - Set to current timestamp (makes row terminal)
- `is_alias_candidate` - Set to 1 for unresolvable results
- `claimed_by` - Cleared to NULL
- `claimed_at` - Cleared to NULL  
- `lease_expires_at` - Cleared to NULL
- `updated_at` - Set to current timestamp

## Queue Claim Interaction Points

**Claim Path (for context):**

1. **User-enrichment-worker** claims rows via `POST /email-resolution/claim`
2. **Queue-api** `ClaimEmailResolution()` (lines 130-248 in storage layer)
3. **Reclaims expired leases** first (crashed worker recovery)
4. **Claims highest priority rows** using: `WHERE provider='github' AND status='pending' AND attempted_at IS NULL ORDER BY priority DESC, created_at ASC LIMIT ?`
5. **Sets lease fields:** claimed_by, claimed_at, lease_expires_at

**Resolve Path (this document's focus):**

1. **Worker** calls `POST /email-resolution/resolve` with the resolution result
2. **Queue-api** validates the worker still holds the lease (`claimed_by = workerID`)
3. **Writes terminal state** and clears lease fields in single transaction
4. **Row becomes cached** and is never re-claimed (attempted_at IS NOT NULL)

## Database Connection Management

**File:** `/home/coding/commitgraph-deprecated/containers/queue-api/internal/storage/db.go`

**Connection Architecture:**
- **Writer pool** - Single connection (`SetMaxOpenConns(1)`) for all mutations
- **Reader pool** - 4 connections for read-only operations (exports, stats)
- **WAL mode** - Enables concurrent readers alongside single writer
- **Busy timeout** - 5 seconds for retrying under contention

**Pragmas Applied:**
```sql
PRAGMA journal_mode=WAL          -- Write-Ahead Logging for concurrent readers
PRAGMA foreign_keys=ON           -- Referential integrity
PRAGMA synchronous=NORMAL        -- Durable WAL commits without full fsync
PRAGMA busy_timeout=5000         -- Retry contention for 5 seconds
```

## Key Invariants

1. **One GitHub API call per email** - `attempted_at` makes rows terminal
2. **Lease validation on resolve** - Only holding worker can resolve
3. **Priority-based claim ordering** - Highest AI-commit-value first
4. **Negative caching** - `github_login=NULL` stores provable non-matches
5. **Idempotent operations** - Safe to retry upsert/resolve/claim

## Migration Context

**Current System Status:**
- This write path is from the **deprecated** commitgraph system
- New commitgraph system is moving to PostgreSQL directly  
- The login-revalidation-worker is the **only remaining user** of this legacy queue-api write path
- Queue-api SQLite is being replaced by PostgreSQL `email_resolution` table with conflict resolution rules

**Replacement Implementation:**
- See `/home/coding/commitgraph/pkg/pg/identity.go` for PostgreSQL implementation
- Uses bulk upsert with ON CONFLICT rules for conflict resolution
- No separate claim/lease mechanism - direct writes with source-based priority

## References

- **Login Revalidation Worker:** `/home/coding/commitgraph/containers/login-revalidation-worker/main.go`
- **Queue-api Server:** `/home/coding/commitgraph-deprecated/containers/queue-api/internal/server/email_resolution.go`
- **Queue-api Storage:** `/home/coding/commitgraph-deprecated/containers/queue-api/internal/storage/email_resolution.go`
- **Database Schema:** `/home/coding/commitgraph-deprecated/containers/queue-api/schema.sql`
- **Connection Management:** `/home/coding/commitgraph-deprecated/containers/queue-api/internal/storage/db.go`
