# Commitgraph Database Operations Reference

**Bead:** cg-58vzn  
**Date:** 2026-08-06  
**Purpose:** Complete documentation of all PostgreSQL operations in the commitgraph system

## Overview

The commitgraph system does NOT have a dedicated `commits` table. Instead, it stores aggregated commit counts in the `repo_user_daily_tool.commits` column and manages metadata through the following database operations:

**Database Schema Tables:**
- `repos` - repository identity with exclusion tracking
- `users` - developer identity 
- `email_resolution` - email→login resolution results
- `user_aliases` - login→login alias mapping
- `repo_user_daily_tool` - main rollup table with aggregated commit counts
- `corpus_stats` - global scalar totals

## Operations by Category

### UPSERT Operations (INSERT with ON CONFLICT)

#### 1. Users Table - Batch User Creation
**File:** `pkg/pg/users.go:45-50`  
**Constant:** `BatchUsersUpsertQuery`

```go
const BatchUsersUpsertQuery = `
INSERT INTO users (login)
SELECT unnest($1::text[])
ON CONFLICT (login) DO NOTHING
RETURNING login, user_id
`
```

**SQL Operation Type:** INSERT with ON CONFLICT DO NOTHING  
**Purpose:** Creates new user records from logins, returning complete login→user_id mapping  
**Parameters:** 
- `$1::text[]` - Array of login strings  
**Returns:** `login, user_id` pairs (both newly created and pre-existing)  
**Idempotent:** Yes - re-running with same logins returns consistent results  

#### 2. User Aliases Table - Bulk Alias Management
**File:** `pkg/pg/user_aliases.go:46-88`  
**Function:** `func (a *AliasIngester) UpsertAliases(ctx context.Context, rows []AliasRow) error`

```go
type AliasRow struct {
    SourceLogin string    // PRIMARY KEY
    TargetLogin string    // Canonical login to alias to
    Reason      string    // 'admin' or 'name-match'
    CreatedAt   time.Time // Creation timestamp
}
```

**SQL Operation Type:** INSERT with ON CONFLICT DO UPDATE  
**Purpose:** Bulk upsert of alias mappings with conflict handling  
**Parameters:**
- `$1::text[]` - Source login array
- `$2::text[]` - Target login array  
- `$3::text[]` - Reason array
- `$4::timestamptz[]` - Created at timestamps  
**Returns:** error (nil on success)  
**Conflict Behavior:** Updates existing rows on source_login conflict  
**Idempotent:** Yes  

#### 3. Email Resolution Table - Email→Login Mapping
**File:** `pkg/pg/identity.go:94-234`  
**Function:** `func (i *IdentityIngester) IngestEmailResolution(ctx context.Context, rows []identity.ResolutionRow) (*identity.IngestResult, error)`

```go
type ResolutionRow struct {
    Email       string
    Login       string
    Source      string    // 'manual' or other sources
    ResolvedAt  time.Time
}
```

**SQL Operation Type:** INSERT with ON CONFLICT DO UPDATE (conditional)  
**Purpose:** Bulk upsert of email resolution rows with sophisticated conflict resolution  
**Parameters:**
- `$1::text[]` - Email array
- `$2::text[]` - Login array
- `$3::text[]` - Source array  
- `$4::timestamptz[]` - Resolved at timestamps  
**Returns:** `*identity.IngestResult` with counts:
- `Ingested` - Number of rows inserted/updated
- `Skipped` - Number of rows skipped due to conflict rules
- `SkipDetails` - Breakdown by skip reason  

**Conflict Resolution Rules:**
- Manual source always wins (overwrites any existing row)
- Non-manual sources win only if existing row is also non-manual AND new resolved_at is newer
- Otherwise existing row is preserved

**SQL Conflict Logic:**
```sql
ON CONFLICT (email) DO UPDATE
  SET login = excluded.login,
      source = excluded.source,
      resolved_at = excluded.resolved_at
  WHERE excluded.source = 'manual'
     OR (email_resolution.source <> 'manual'
         AND excluded.resolved_at > email_resolution.resolved_at)
```

**Idempotent:** Yes - sophisticated conflict detection prevents unwanted overwrites  

### SELECT Operations (Read)

#### 4. Users Table - Login Lookup
**File:** `pkg/pg/users.go:60-64`  
**Constant:** `UsersSelectByLoginsQuery`

```go
const UsersSelectByLoginsQuery = `
SELECT login, user_id
FROM users
WHERE login = ANY($1::text[])
`
```

**SQL Operation Type:** SELECT  
**Purpose:** Retrieves user_ids for existing logins without attempting insertion  
**Parameters:**
- `$1::text[]` - Array of login strings to look up  
**Returns:** `login, user_id` pairs for existing logins only  
**Use Case:** When all logins are known to exist and you want to avoid INSERT overhead  

#### 5. User Aliases Table - Admin Alias Retrieval
**File:** `pkg/pg/user_aliases.go:92-119`  
**Function:** `func (a *AliasIngester) GetAdminAliases(ctx context.Context) (map[string]string, error)`

**SQL Operation Type:** SELECT  
**Purpose:** Retrieves all admin aliases from the database  
**Parameters:** None  
**Returns:** `map[string]string` where key=source_login, value=target_login (reason='admin' only)  

**SQL Query:**
```sql
SELECT source_login, target_login
FROM user_aliases
WHERE reason = 'admin'
```

#### 6. Repos Table - Exclusion Status Check
**File:** `pkg/pg/repo.go:93-122`  
**Function:** `func (r *RepoExcluder) GetExclusion(ctx context.Context, provider, repoFullName string) (*time.Time, string, error)`

**SQL Operation Type:** SELECT  
**Purpose:** Retrieves current exclusion status for a specific repo  
**Parameters:**
- `$1` - Provider (e.g., "github")
- `$2` - Repository full name (e.g., "owner/name")  
**Returns:** `(excluded_at, excluded_reason, error)` or `(nil, "", error)`  

**SQL Query:**
```sql
SELECT excluded_at, excluded_reason
FROM repos
WHERE provider = $1 AND repo_full_name = $2
```

#### 7. Repos Table - List All Exclusions
**File:** `pkg/pg/repo.go:126-162`  
**Function:** `func (r *RepoExcluder) ListExclusions(ctx context.Context) ([]ExclusionInfo, error)`

**SQL Operation Type:** SELECT  
**Purpose:** Retrieves all currently excluded repos, ordered by exclusion date  
**Parameters:** None  
**Returns:** `[]ExclusionInfo` containing:
- `Provider` - Repository provider
- `RepoFullName` - Repository full name
- `ExcludedAt` - Exclusion timestamp
- `ExcludedReason` - Human-readable reason

**SQL Query:**
```sql
SELECT provider, repo_full_name, excluded_at, excluded_reason
FROM repos
WHERE excluded_at IS NOT NULL
ORDER BY excluded_at DESC
```

### UPDATE Operations (Modify)

#### 8. Repos Table - Apply/Clear Exclusions
**File:** `pkg/pg/repo.go:45-89`  
**Function:** `func (r *RepoExcluder) ApplyExclusion(ctx context.Context, req ExclusionRequest) (int64, error)`

```go
type ExclusionRequest struct {
    Provider       string     // e.g., "github"
    RepoFullName   string     // e.g., "owner/name"
    ExcludedAt     *time.Time // NULL for clear operations
    ExcludedReason string     // Required for exclude, empty for clear
    Operator       string     // Who is performing this action
}

type ExclusionOp string
const (
    OpExclude ExclusionOp = "exclude"
    OpClear   ExclusionOp = "clear"
)
```

**SQL Operation Type:** UPDATE  
**Purpose:** Applies or clears repository exclusions  
**Parameters (Exclude):**
- `$1` - Excluded timestamp
- `$2` - Exclusion reason
- `$3` - Provider
- `$4` - Repository full name  

**Parameters (Clear):**
- `$1` - Provider
- `$2` - Repository full name  

**Returns:** `int64` (rows affected: 1 if repo exists, 0 otherwise)  

**Exclude Query:**
```sql
UPDATE repos
SET excluded_at = $1,
    excluded_reason = $2
WHERE provider = $3 AND repo_full_name = $4
```

**Clear Query:**
```sql
UPDATE repos
SET excluded_at = NULL,
    excluded_reason = NULL
WHERE provider = $1 AND repo_full_name = $2
```

### DELETE Operations (Remove)

#### 9. User Aliases Table - Remove Admin Aliases
**File:** `pkg/pg/user_aliases.go:123-141`  
**Function:** `func (a *AliasIngester) DeleteAdminAliases(ctx context.Context, sourceLogins []string) (int64, error)`

**SQL Operation Type:** DELETE  
**Purpose:** Removes specific admin aliases by source_login  
**Parameters:**
- `$1::text[]` - Array of source login strings to delete  
**Returns:** `int64` (number of rows deleted)  

**SQL Query:**
```sql
DELETE FROM user_aliases
WHERE source_login = ANY($1::text[])
AND reason = 'admin'
```

## Database Architecture Notes

### No Individual Commits Table
The commitgraph system does **NOT** store individual commit records in PostgreSQL. Instead:

1. **Raw commit data** is stored externally in Parquet files (managed by separate services)
2. **Aggregated rollups** are stored in `repo_user_daily_tool` table
3. **The `commits` column** in `repo_user_daily_tool` is an integer counter, not individual records

### External Storage Locations
For actual commit data storage, refer to:
- `containers/queue-api/` - Queue API service
- `pkg/warmstart/` - Warmstart snapshot code  
- `pkg/rollup/` - Rollup computation logic

## Summary Statistics

**Total Operations Documented:** 9  
**UPSERT operations:** 3  
**SELECT operations:** 4  
**UPDATE operations:** 1  
**DELETE operations:** 1  

**Files Covered:**
- `pkg/pg/users.go` - 2 operations
- `pkg/pg/user_aliases.go` - 3 operations  
- `pkg/pg/identity.go` - 1 operation
- `pkg/pg/repo.go` - 3 operations

## Related Documentation

- **Bead cg-5xpxo:** Detailed analysis of false positives in grep search
- **Bead cg-3m8cs:** Original grep search results and methodology
- **Database Schema:** `migrations/00001_initial_schema.sql`
- **Architecture:** `docs/plan/plan.md`

---
*Generated as part of bead cg-58vzn - Comprehensive database operations documentation*