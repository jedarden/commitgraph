# Commit Processing and Author Login Extraction Flow

**Task ID:** cg-1iq59
**Date:** 2026-08-06
**Status:** Exploration complete

## Overview

This document describes the complete flow of commit processing and author login extraction in the commitgraph v2 architecture, from repository scanning to database insertion.

## Architecture Summary

The commitgraph v2 system processes commits through a multi-stage pipeline:

1. **Repository Scanning** (clone-worker) - Extract raw commit data
2. **AI Tool Detection** (shared/detection.py) - Tag commits with AI tools
3. **Rollup Computation** (pkg/rollup/rollup.go) - Aggregate by user/repo/tool/day
4. **Identity Resolution** (email_resolution table) - Resolve emails to logins
5. **Database Insertion** (repo_user_daily_tool table) - Store aggregated results

## Commit Data Structure

### Raw Commit Schema

Commits are extracted from git repositories with the following structure:

```go
// pkg/rollup/rollup.go:56-64
type Commit struct {
    SHA         string    // Commit SHA
    AuthorEmail string    // Author email (for identity resolution)
    AuthorName  string    // Author name
    CommittedAt time.Time // Commit date (UTC)
    Message     string    // Commit message
    Tools       []string  // Detected AI tools (empty if no AI tool detected)
}
```

### Corpus Parquet Schema

The corpus stores commits in Parquet format with schema:
- `provider` (e.g., 'github')
- `repo_full_name` (e.g., 'owner/repo')
- `sha` (commit SHA)
- `author_name` (commit author name)
- `author_email` (commit author email) ← **Key field for identity resolution**
- `committed_at` (commit timestamp)
- `message` (commit message text)

**Location:** `migration/migrate_corpus.py:357-365`

## Processing Flow

### Stage 1: Repository Scanning (clone-worker)

**Purpose:** Extract raw commit data from git repositories

**Implementation:** (from plan.md lines 228-248)
1. Warm-start from stored snapshot before falling back to full clone
2. Walk full history, extract `(sha, author_name, author_email, committed_at, message)`
3. Run `shared/detection.py` inline per commit
4. Compute `(user, repo, tool, day, count)` rollup rows for AI-tool-tagged commits only
5. In one pass: (a) upsert rollup to Postgres, (b) write Parquet artifact to ARMOR

**Key Fields Extracted:**
- `author_email` - Primary identifier for user identity
- `author_name` - Secondary identifier (not used for identity resolution)
- `committed_at` - Used for day-level aggregation and quarantine filtering

### Stage 2: AI Tool Detection

**Purpose:** Identify which commits were created with AI coding tools

**Implementation:** `containers/clone-worker/detection.py`

**Detection Function:**
```python
# containers/clone-worker/detection.py:395
def detect_tools_for_commit(
    author_email: str,
    author_name: str,
    commit_message: str
) -> Set[str]
```

**Signal Tiers Checked:**
1. Co-Authored-By trailer emails
2. Author emails (bot-authored commits)
3. Author name patterns
4. Body text patterns

**Catalog Coverage:** 15+ tools across 4 signal tiers (Claude Code, Cursor, Aider, Copilot, etc.)

### Stage 3: Rollup Computation

**Purpose:** Aggregate AI-tagged commits by (user, repo, tool, day)

**Implementation:** `pkg/rollup/rollup.go:77-141`

**Process:**
1. Group commits by `(user_email, tool, day)`
2. Skip commits with no AI tools detected
3. Apply date quarantine filter (exclude commits outside [2005-01-01, today+1])
4. Count commits per aggregation key

**Rollup Row Structure:**
```go
// pkg/rollup/rollup.go:66-75
type RollupRow struct {
    UserEmail string    // Author email (resolved to user_id later)
    RepoID    int64     // Repository ID
    Tool      string    // AI tool name
    Day       time.Time // Day (UTC, midnight)
    Count     int       // Number of commits
    // InsertTime is set by database DEFAULT transaction_timestamp()
}
```

**Key Processing Logic:**
```go
// pkg/rollup/rollup.go:113
key := commit.AuthorEmail + "|" + tool + "|" + day.Format(time.RFC3339)
```

### Stage 4: Identity Resolution

**Purpose:** Resolve author emails to GitHub logins

**Implementation:** Two-stage resolution process

#### Stage 4a: Email Resolution (email_resolution table)

**Purpose:** Cache email → login resolutions permanently

**Schema:** `migrations/00001_initial_schema.sql:30-37`
```sql
CREATE TABLE IF NOT EXISTS email_resolution (
  email       TEXT PRIMARY KEY,
  login       TEXT NOT NULL,
  source      TEXT NOT NULL,  -- 'live' | 'seed' | 'manual'
  resolved_at TIMESTAMPTZ NOT NULL
);
```

**Conflict Resolution Rule:** (from `pkg/pg/identity.go:162-176`)
- Manual source always wins (overwrites any existing row)
- Non-manual sources win only if existing row is also non-manual AND newer resolved_at
- Otherwise preserve existing row

**Implementation:** `pkg/pg/identity.go:94-234`

#### Stage 4b: User Aliases (user_aliases table)

**Purpose:** Merge multiple logins that represent the same person

**Schema:** `migrations/00001_initial_schema.sql:39-44`
```sql
CREATE TABLE IF NOT EXISTS user_aliases (
  source_login TEXT PRIMARY KEY,
  target_login TEXT NOT NULL,
  reason       TEXT NOT NULL,  -- 'admin' | 'name-match'
  created_at   TIMESTAMPTZ NOT NULL
);
```

**Resolution Flow:**
1. author_email → email_resolution → login (canonical GitHub login)
2. login → user_aliases → target_login (if alias exists)
3. Final resolved login used for ranking

**Worker:** `containers/user-enrichment-worker/worker.py`

### Stage 5: Database Insertion

**Purpose:** Store aggregated rollup data in Postgres

**Target Table:** `repo_user_daily_tool`

**Schema:** `migrations/00001_initial_schema.sql:47-55`
```sql
CREATE TABLE IF NOT EXISTS repo_user_daily_tool (
  repo_id     BIGINT NOT NULL REFERENCES repos(repo_id),
  user_id     BIGINT NOT NULL REFERENCES users(user_id),
  tool        TEXT   NOT NULL,        -- plain TEXT, not enum
  day         DATE   NOT NULL,
  commits     INT    NOT NULL,
  insert_time TIMESTAMPTZ NOT NULL DEFAULT transaction_timestamp(),
  PRIMARY KEY (repo_id, user_id, tool, day)
);
```

**Insertion Pattern:** (from migration/migrate_corpus.py:515-593)

1. **Upsert repos row** → get repo_id
2. **Upsert users rows** → get user_ids (using email as login placeholder initially)
3. **DELETE existing rollup rows** for this repo
4. **Bulk INSERT new rollup rows** using UNNEST

**Key Points:**
- `insert_time` is set by database DEFAULT, not application code
- DELETE + bulk INSERT ensures idempotence
- Only AI-tool-tagged commits are stored in rollup

## Author Login Extraction: Current Flow

### Current Implementation (Migration Path)

**Location:** `migration/migrate_corpus.py:385-513`

**Extraction Process:**
```python
# migration/migrate_corpus.py:436-443
author_email = row.get('author_email', '')
author_name = row.get('author_name', '')

# Track author name
if author_email and author_name:
    author_names[author_email] = author_name
```

**Usage in Rollup:**
```python
# migration/migrate_corpus.py:493
key = (author_email, repo_full_name, tool, commit_date)
rollup_counts[key] += 1
```

**Database Insertion:**
```python
# migration/migrate_corpus.py:559-572
# For migration, we use email as login placeholder
# Real identity resolution happens later via email_resolution
login = email  # Will be resolved later

user_query = sql.SQL("""
    INSERT INTO users (login)
    VALUES (%s)
    ON CONFLICT (login)
    DO UPDATE SET login = EXCLUDED.login
    RETURNING user_id
""")
```

### Author Login Extraction: Future Path (Post-Migration)

**Planned Implementation:** clone-worker (to be rewritten per plan.md:228-248)

**Extraction Points:**
1. **Git log parsing** - Extract `author_email` from commit metadata
2. **Noreply email parsing** - Extract login from `{id}+{login}@users.noreply.github.com`
3. **GitHub API lookup** - Search commits by author email when needed

**Resolution Worker:** `containers/user-enrichment-worker/worker.py:90-150`

```python
# containers/user-enrichment-worker/worker.py:90
def lookup_github_login(email: str) -> tuple[Optional[str], bool]:
    """Resolve one email. Returns (login_or_None, api_called)."""
    noreply = _extract_login_from_noreply(email)
    if noreply is not None:
        return noreply, False
    if _is_private_email(email) or not GITHUB_TOKEN:
        return None, False

    # GitHub commit search API call
    r = requests.get(
        "https://api.github.com/search/commits",
        # ... API call logic
    )
```

## Key Finding: No Separate "Commits" Table

**Important:** There is **NO separate commits table** in the v2 schema.

- The raw commit data lives in Parquet artifacts in ARMOR
- Only the aggregated rollup (AI-tool-tagged commits) lives in Postgres
- Rollup table: `repo_user_daily_tool` (aggregated by user/repo/tool/day)
- No individual commit records in the database

**Reference:** `migrations/00001_initial_schema.sql` confirms this schema

## Current Processing Path Summary

```
git repository
    ↓ (clone-worker extracts)
author_email, author_name, committed_at, message
    ↓ (detection.py tags)
AI tool detection results
    ↓ (rollup computation)
(author_email, repo, tool, day) → count
    ↓ (identity resolution - deferred)
author_email → login (via email_resolution table)
    ↓ (database insertion)
repo_user_daily_tool (repo_id, user_id, tool, day, commits)
```

## Files Identified

### Core Processing Logic
- **pkg/rollup/rollup.go** - Rollup computation from commits
- **pkg/pg/identity.go** - Email resolution ingestion
- **containers/clone-worker/detection.py** - AI tool detection catalog
- **containers/user-enrichment-worker/worker.py** - Email → login resolution

### Migration/Implementation
- **migration/migrate_corpus.py** - Streaming corpus migration logic
- **docs/plan/plan.md** - Complete architecture documentation

### Database Schema
- **migrations/00001_initial_schema.sql** - Core schema definitions

## Acceptance Criteria Status

✅ Located all commit insertion/upsert functions in pkg/pg/
   - Found: No direct commit insertion in pkg/pg (commits processed in migration)
   - Rollup insertion: `repo_user_daily_tool` table via migration/migrate_corpus.py

✅ Identified where author logins are extracted
   - Extraction point: `migration/migrate_corpus.py:436` (author_email field)
   - Resolution path: email_resolution table (pkg/pg/identity.go:94-234)
   - Alias merging: user_aliases table (migrations/00001_initial_schema.sql:39-44)

✅ Documented the current processing flow
   - This document provides complete flow documentation

✅ No code changes required (exploration only)
   - Confirmed: This was an exploration task only

## Next Steps (If Implementation Follows)

Based on this exploration, implementation would involve:

1. **Rewrite clone-worker** to extract commits from git (plan.md:228-248)
2. **Inline detection.py** for AI tool detection per commit
3. **Compute rollup** using pkg/rollup/rollup.go:ComputeRollup
4. **Upsert to Postgres** using pattern from migration/migrate_corpus.py:515-593
5. **Resolve identities** via user-enrichment-worker and email_resolution table

---

**Task Status:** ✅ COMPLETE - All acceptance criteria met
**Documentation:** Created notes/cg-1iq59.md
**Commit:** Pending (will commit this documentation file)
