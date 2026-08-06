# Commit Processing and Author Login Extraction Flow

**Task ID:** cg-1iq59  
**Date:** 2026-08-06  
**Status:** Complete

## Overview

Commitgraph v2 is a redesign of the AI-coding-tool-attribution data platform. The old pipeline was deprecated and torn down on 2026-08-05. The current system is in the design/planning stage with Postgres as the primary datastore.

## Current Architecture State

### 1. Database Schema (Postgres-based)

**Primary Tables:**
- `repos` - repository identity with exclusion tracking
- `users` - developer identity (login, profile_url, avatar_url)
- `email_resolution` - email→login resolution results
- `user_aliases` - login→login alias mapping
- `repo_user_daily_tool` - main rollup table (AI-tool-tagged commits only)
- `corpus_stats` - global scalar totals

**Location:** `/home/coding/commitgraph/migrations/00001_initial_schema.sql`

### 2. Commit Processing Flow

**Current Design (from plan.md):**

The write path has been redesigned from the previous multi-stage architecture:

```
Old Pipeline: clone-worker → stage → compactor merge → filter-worker detect → aggregator rollup
New Pipeline: clone-worker → (extract + compute rollup) → Postgres direct write
```

**Key Components:**
- **clone-worker**: Processes repositories, extracts commits, computes rollup in one pass
- **queue-api**: Work queue coordination (separate service, SQLite-based)
- **Identity resolution**: Email→login resolution via `email_resolution` table
- **Rollup computation**: Per-(user, repo, tool, day) aggregation

### 3. Author Login Extraction Locations

**Primary extraction happens in:**
- **`pkg/rollup/rollup.go`** - Defines `Commit` struct with `AuthorEmail` field
- **`pkg/identity/ingest.go`** - Handles email→login resolution ingestion
- **`pkg/pg/identity.go`** - Database operations for identity resolution

**Commit Data Structure:**
```go
// From pkg/rollup/rollup.go
type Commit struct {
    SHA         string    // Commit SHA
    AuthorEmail string    // Author email (for identity resolution)
    AuthorName  string    // Author name
    CommittedAt time.Time // Commit date (UTC)
    Message     string    // Commit message
    Tools       []string  // Detected AI tools (empty if no AI tool detected)
}
```

**Author Login Resolution Flow:**
```
Commit.AuthorEmail → email_resolution table → users.login → user_id
```

### 4. Database Insert/Upsert Operations

**Identity Resolution (pkg/pg/identity.go):**
- Function: `IngestEmailResolution(ctx context.Context, rows []ResolutionRow)`
- Operation: Bulk INSERT with ON CONFLICT DO UPDATE
- Conflict Rule: Manual source always wins; otherwise newer resolved_at wins
- Location: `/home/coding/commitgraph/pkg/pg/identity.go:88-223`

**User Operations (pkg/pg/users.go):**
- Function: `GetOrInsertUser(ctx context.Context, db Executor, login string)`
- Operation: INSERT INTO users with ON CONFLICT DO NOTHING
- Returns: user_id for the given login
- Location: `/home/coding/commitgraph/pkg/pg/users.go:32-72`

**Rollup Operations:**
- **Note**: The actual rollup insert operations are not yet implemented in pkg/pg/
- Rollup computation logic exists in `pkg/rollup/rollup.go` but database writes are pending
- According to plan, this will use `repo_user_daily_tool` table

### 5. Email Resolution Ingest Path

**Components:**
- **`pkg/identity/ingest.go`**: Core ingest logic with conflict resolution
- **`pkg/client/queueapi/client.go`**: Queue-api client for posting resolutions
- **`containers/login-revalidation-worker/main.go`**: Worker for detecting renamed/deleted GitHub logins

**Ingest Flow:**
```
queue-api client → identity.Ingester → Postgres email_resolution table
```

**Conflict Resolution Rule:**
```sql
ON CONFLICT (email) DO UPDATE
  SET login = excluded.login, source = excluded.source,
      resolved_at = excluded.resolved_at
  WHERE excluded.source = 'manual'
     OR (email_resolution.source <> 'manual'
         AND excluded.resolved_at > email_resolution.resolved_at)
```

### 6. Queue API Integration

**Location:** `/home/coding/commitgraph/containers/queue-api/`
**Schema:** `/home/coding/commitgraph/containers/queue-api/schema.sql`
**Purpose:** Work queue for repository cloning and processing tasks

**Queue API Schema:**
```sql
CREATE TABLE repo_queue (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  provider        TEXT NOT NULL,
  repo_full_name  TEXT NOT NULL,
  kind            TEXT NOT NULL DEFAULT 'normal-clone',
  status          TEXT NOT NULL DEFAULT 'pending',
  ...
);
```

### 7. Rollup Computation Logic

**Location:** `/home/coding/commitgraph/pkg/rollup/rollup.go`

**Function:** `ComputeRollup(commits []Commit, repoID int64, bounds QuarantineBounds) []RollupRow`

**Process:**
1. Group commits by (user_email, tool, day)
2. Skip commits with no AI tools detected
3. Apply date quarantine filter (exclude out-of-range dates)
4. Normalize committed_at to day (UTC midnight)
5. Create rollup entry for each detected tool
6. Return aggregated rollup rows

**Rollup Row Structure:**
```go
type RollupRow struct {
    UserEmail string    // Author email (resolved to user_id later)
    RepoID    int64     // Repository ID
    Tool      string    // AI tool name
    Day       time.Time // Day (UTC, midnight)
    Count     int       // Number of commits
}
```

## Key Findings

### 1. **Status: Design/Planning Stage**
- The README explicitly states: "Design/planning stage — no application code yet"
- The old pipeline was torn down on 2026-08-05
- This is a redesign, not an iteration of the old system

### 2. **Author Login Extraction is Email-Based**
- Author logins are not directly extracted from commits
- Instead, author emails from commits are resolved to logins via `email_resolution` table
- Resolution happens through identity resolution ingest pipeline

### 3. **No Direct Commit Storage in Postgres**
- Postgres does NOT store raw commit data
- Raw commits are stored in ARMOR (encrypted Parquet format)
- Postgres only stores aggregated rollup data
- Total commit counts are maintained in `corpus_stats` table

### 4. **Missing Implementation**
The following components are designed but not yet implemented:
- Actual clone-worker that processes git repositories
- Direct rollup insert operations to `repo_user_daily_tool` table
- Git commit extraction and author email parsing
- Integration between rollup computation and Postgres writes

### 5. **Identity Resolution is Separate Pipeline**
- Email→login resolution is handled separately from commit processing
- Uses queue-api for work queue coordination
- Supports three sources: 'live', 'seed', 'manual'
- Implements conflict resolution with precedence rules

## Processing Flow Summary

```
1. Repository Discovery
   ↓
2. Queue API (repo_queue table)
   ↓
3. Clone-worker (not yet implemented)
   - Clone repository
   - Extract commit history
   - Parse commit.AuthorEmail
   - Detect AI tool footprints
   ↓
4. Rollup Computation (pkg/rollup/rollup.go)
   - Group by (user_email, tool, day)
   - Apply date quarantine
   ↓
5. Identity Resolution
   - author_email → email_resolution table → users.login → user_id
   ↓
6. Postgres Write (not yet implemented)
   - repo_user_daily_tool insert
   - users table upsert
   - corpus_stats update
```

## File Locations Summary

**Database Operations:**
- `/home/coding/commitgraph/pkg/pg/identity.go` - Email resolution operations
- `/home/coding/commitgraph/pkg/pg/users.go` - User operations
- `/home/coding/commitgraph/pkg/pg/repo.go` - Repository operations
- `/home/coding/commitgraph/pkg/pg/user_aliases.go` - Alias operations

**Core Logic:**
- `/home/coding/commitgraph/pkg/rollup/rollup.go` - Rollup computation
- `/home/coding/commitgraph/pkg/identity/ingest.go` - Identity ingest logic
- `/home/coding/commitgraph/pkg/client/queueapi/client.go` - Queue API client

**Schema:**
- `/home/coding/commitgraph/migrations/00001_initial_schema.sql` - Main database schema
- `/home/coding/commitgraph/containers/queue-api/schema.sql` - Queue API schema

**Documentation:**
- `/home/coding/commitgraph/docs/plan/plan.md` - Complete architecture plan
- `/home/coding/commitgraph/README.md` - Project overview

## Conclusion

The commitgraph v2 system has a well-designed architecture for commit processing and author login extraction, but it is currently in the design/planning stage. The key insight is that author logins are not directly extracted from commits but are resolved through a separate email→login resolution pipeline. The actual implementation of commit processing and rollup writes to Postgres is pending development.