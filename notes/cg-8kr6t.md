# Commit Database Operation Functions - Detailed Signatures

## Overview
This document catalogs all database operation functions found in the commitgraph codebase that perform INSERT or UPDATE operations.

---

## 1. User Operations

### Function: `BatchUsersUpsertQuery`
- **File:** `pkg/pg/users.go:46`
- **Type:** Constant (SQL query string)
- **Operation:** UPSERT (INSERT ... ON CONFLICT DO NOTHING)
- **Target Table:** `users`
- **SQL Query:**
  ```sql
  INSERT INTO users (login)
  SELECT unnest($1::text[])
  ON CONFLICT (login) DO NOTHING
  RETURNING login, user_id
  ```
- **Parameters:**
  - `$1::text[]` - Array of login strings
- **Returns:** `login, user_id` pairs (both newly created and pre-existing)
- **Special Handling:** Idempotent - re-running with same login set returns consistent results
- **Usage Pattern:** Called via `db.Query(ctx, BatchUsersUpsertQuery, []string{"alice", "bob"})`

---

### Function: `UsersSelectByLoginsQuery`
- **File:** `pkg/pg/users.go:60`
- **Type:** Constant (SQL query string)
- **Operation:** SELECT (fallback query)
- **Target Table:** `users`
- **SQL Query:**
  ```sql
  SELECT login, user_id
  FROM users
  WHERE login = ANY($1::text[])
  ```
- **Parameters:**
  - `$1::text[]` - Array of login strings
- **Returns:** `login, user_id` pairs for existing logins only
- **Special Handling:** Use when all logins already exist and you want to avoid INSERT overhead

---

## 2. User Alias Operations

### Function: `UpsertAliases`
- **File:** `pkg/pg/user_aliases.go:46`
- **Type:** Method on `*AliasIngester`
- **Signature:**
  ```go
  func (a *AliasIngester) UpsertAliases(ctx context.Context, rows []AliasRow) error
  ```
- **Operation:** UPSERT (INSERT ... ON CONFLICT DO UPDATE)
- **Target Table:** `user_aliases`
- **Input Parameters:**
  - `ctx context.Context` - Context for the operation
  - `rows []AliasRow` - Slice of alias rows
    - `AliasRow.SourceLogin string` - The login to alias from (PRIMARY KEY)
    - `AliasRow.TargetLogin string` - The canonical login to alias to
    - `AliasRow.Reason string` - 'admin' or 'name-match'
    - `AliasRow.CreatedAt time.Time` - When this alias was created
- **Return Type:** `error`
- **SQL Operation:**
  ```sql
  INSERT INTO user_aliases (source_login, target_login, reason, created_at)
  SELECT unnest($1::text[]),
         unnest($2::text[]),
         unnest($3::text[]),
         unnest($4::timestamptz[])
  ON CONFLICT (source_login) DO UPDATE
    SET target_login = excluded.target_login,
        reason = excluded.reason,
        created_at = excluded.created_at
  ```
- **Special Handling:**
  - Bulk upsert using UNNEST for array parameters
  - Idempotent - re-running updates existing rows rather than erroring
  - Returns nil if rows slice is empty

---

### Function: `DeleteAdminAliases`
- **File:** `pkg/pg/user_aliases.go:123`
- **Type:** Method on `*AliasIngester`
- **Signature:**
  ```go
  func (a *AliasIngester) DeleteAdminAliases(ctx context.Context, sourceLogins []string) (int64, error)
  ```
- **Operation:** DELETE
- **Target Table:** `user_aliases`
- **Input Parameters:**
  - `ctx context.Context` - Context for the operation
  - `sourceLogins []string` - Array of source logins to delete
- **Return Type:** `(int64, error)` - Number of rows deleted
- **SQL Operation:**
  ```sql
  DELETE FROM user_aliases
  WHERE source_login = ANY($1::text[])
  AND reason = 'admin'
  ```
- **Special Handling:**
  - Only deletes aliases with reason='admin'
  - Returns 0 if sourceLogins slice is empty

---

## 3. Email Resolution Operations

### Function: `IngestEmailResolution`
- **File:** `pkg/pg/identity.go:94`
- **Type:** Method on `*IdentityIngester`
- **Signature:**
  ```go
  func (i *IdentityIngester) IngestEmailResolution(ctx context.Context, rows []identity.ResolutionRow) (*identity.IngestResult, error)
  ```
- **Operation:** UPSERT (INSERT ... ON CONFLICT DO UPDATE with WHERE clause)
- **Target Table:** `email_resolution`
- **Input Parameters:**
  - `ctx context.Context` - Context for the operation
  - `rows []identity.ResolutionRow` - Slice of resolution rows
    - `ResolutionRow.Email string` - Email address (primary key)
    - `ResolutionRow.Login string` - Resolved GitHub login
    - `ResolutionRow.Source identity.Source` - 'live' | 'seed' | 'manual'
    - `ResolutionRow.ResolvedAt time.Time` - When this resolution was made
- **Return Type:** `(*identity.IngestResult, error)`
  - `IngestResult.Ingested int64` - Number of rows written (inserted or updated)
  - `IngestResult.Skipped int64` - Number of rows skipped due to conflict resolution
  - `IngestResult.SkipDetails map[identity.SkipReason]int64` - Breakdown of skip reasons
- **SQL Operation:**
  ```sql
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
- **Special Handling:**
  - **Conflict Resolution Rule:**
    - Manual source always wins (overwrites any existing row)
    - Non-manual sources win only if existing row is also non-manual AND new resolved_at is newer
    - Otherwise existing row is preserved
  - Pre-fetches existing rows for conflict detection
  - Predicts skip reasons before attempting upsert
  - Efficient for large batches (349K+ rows)
  - Returns `&identity.IngestResult{Ingested: 0, Skipped: 0, ...}` if rows slice is empty

---

## 4. Repository Exclusion Operations

### Function: `ApplyExclusion`
- **File:** `pkg/pg/repo.go:45`
- **Type:** Method on `*RepoExcluder`
- **Signature:**
  ```go
  func (r *RepoExcluder) ApplyExclusion(ctx context.Context, req ExclusionRequest) (int64, error)
  ```
- **Operation:** UPDATE
- **Target Table:** `repos`
- **Input Parameters:**
  - `ctx context.Context` - Context for the operation
  - `req ExclusionRequest` - Exclusion request
    - `ExclusionRequest.Provider string` - e.g., "github"
    - `ExclusionRequest.RepoFullName string` - e.g., "owner/name"
    - `ExclusionRequest.ExcludedAt *time.Time` - NULL for clear operations
    - `ExclusionRequest.ExcludedReason string` - Human-readable reason
    - `ExclusionRequest.Operator string` - Who is performing this action
- **Return Type:** `(int64, error)` - Number of rows affected (should be 1 if repo exists, 0 otherwise)
- **SQL Operation (exclude):**
  ```sql
  UPDATE repos
  SET excluded_at = $1,
      excluded_reason = $2
  WHERE provider = $3 AND repo_full_name = $4
  ```
- **SQL Operation (clear):**
  ```sql
  UPDATE repos
  SET excluded_at = NULL,
      excluded_reason = NULL
  WHERE provider = $1 AND repo_full_name = $2
  ```
- **Special Handling:**
  - Requires provider, repo_full_name, and operator to be non-empty
  - Requires excluded_reason for exclude operations
  - Sets excluded_at to NULL for clear operations

---

## 5. Exclusion Audit Log Operations

### Function: `RecordExclusionAudit`
- **File:** `pkg/service/exclusion.go:532`
- **Type:** Variable holding function implementation
- **Signature:**
  ```go
  var RecordExclusionAudit = func(
    ctx context.Context,
    tx Transactor,
    repoID int64,
    actor string,
    eventType string,
    oldExcludedAt *time.Time,
    oldExcludedReason *string,
    newExcludedAt *time.Time,
    newExcludedReason *string,
  ) error
  ```
- **Implementation:** `recordExclusionAuditImpl` (line 490)
- **Operation:** INSERT
- **Target Table:** `exclusion_audit_log`
- **Input Parameters:**
  - `ctx context.Context` - Context for the operation
  - `tx Transactor` - Database transaction
  - `repoID int64` - Repository ID
  - `actor string` - Who performed the action
  - `eventType string` - 'exclude' or 'unexclude'
  - `oldExcludedAt *time.Time` - Previous excluded_at value
  - `oldExcludedReason *string` - Previous excluded_reason value
  - `newExcludedAt *time.Time` - New excluded_at value (NULL for clear)
  - `newExcludedReason *string` - New excluded_reason value (NULL for clear)
- **Return Type:** `error`
- **SQL Operation:**
  ```sql
  INSERT INTO exclusion_audit_log (
    repo_id,
    actor,
    timestamp,
    event_type,
    old_excluded_at,
    old_excluded_reason,
    new_excluded_at,
    new_excluded_reason
  ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
  ```
- **Special Handling:**
  - Records audit trail for all exclusion changes
  - Stores both before and after states
  - Always called within a transaction alongside repo update
  - Function is a variable to allow test mocking

---

## 6. High-Level Service Operations (Transactional)

### Function: `SetRepoExclusionWithActor`
- **File:** `pkg/service/exclusion.go:248`
- **Type:** Function
- **Signature:**
  ```go
  func SetRepoExclusionWithActor(ctx context.Context, db Transactioner, provider, repoFullName, reason, actor string) error
  ```
- **Operation:** TRANSACTION (UPDATE repos + INSERT audit log)
- **Target Tables:** `repos`, `exclusion_audit_log`
- **Input Parameters:**
  - `ctx context.Context` - Context for the operation
  - `db Transactioner` - Database connection with transaction support
  - `provider string` - Repository provider (e.g., "github")
  - `repoFullName string` - Repository full name (e.g., "owner/repo")
  - `reason string` - Human-readable reason for exclusion (must not be empty)
  - `actor string` - Who performed the action
- **Return Type:** `error`
- **Transaction Flow:**
  1. Validate provider format (lowercase alphanumeric)
  2. Validate repo_full_name format (owner/repo)
  3. Validate reason is not empty
  4. Check if repo exists using `RepoExists`
  5. Begin transaction
  6. Query current exclusion state (repo_id, excluded_at, excluded_reason)
  7. Update repos table with new exclusion
  8. Record audit log entry
  9. Commit transaction
- **Special Handling:**
  - All-or-nothing transaction: both repo update and audit log must succeed
  - Defer rollback on error
  - Captures before-state for audit trail
  - Returns error if repo not found or validation fails

---

### Function: `ClearRepoExclusionWithActor`
- **File:** `pkg/service/exclusion.go:398`
- **Type:** Function
- **Signature:**
  ```go
  func ClearRepoExclusionWithActor(ctx context.Context, db Transactioner, provider, repoFullName, actor string) error
  ```
- **Operation:** TRANSACTION (UPDATE repos + INSERT audit log)
- **Target Tables:** `repos`, `exclusion_audit_log`
- **Input Parameters:**
  - `ctx context.Context` - Context for the operation
  - `db Transactioner` - Database connection with transaction support
  - `provider string` - Repository provider (e.g., "github")
  - `repoFullName string` - Repository full name (e.g., "owner/repo")
  - `actor string` - Who performed the action
- **Return Type:** `error`
- **Transaction Flow:**
  1. Validate provider format
  2. Validate repo_full_name format
  3. Check if repo exists
  4. Begin transaction
  5. Query current exclusion state
  6. Update repos table (NULL both fields)
  7. Record audit log entry with NULL new values
  8. Commit transaction
- **Special Handling:**
  - All-or-nothing transaction
  - Defer rollback on error
  - No-op if repo not currently excluded (succeeds with 1 row affected)
  - Records un-exclude event in audit trail

---

## 7. Additional Query Functions (Read-Only)

### Function: `GetAdminAliases`
- **File:** `pkg/pg/user_aliases.go:92`
- **Type:** Method on `*AliasIngester`
- **Signature:**
  ```go
  func (a *AliasIngester) GetAdminAliases(ctx context.Context) (map[string]string, error)
  ```
- **Operation:** SELECT
- **Target Table:** `user_aliases`
- **Input Parameters:**
  - `ctx context.Context` - Context for the operation
- **Return Type:** `(map[string]string, error)` - Map of source_login -> target_login
- **SQL Operation:**
  ```sql
  SELECT source_login, target_login
  FROM user_aliases
  WHERE reason = 'admin'
  ```
- **Special Handling:**
  - Only returns admin aliases (reason='admin')
  - Returns convenience map instead of struct slice
  - Used for building ConfigMap from current database state

---

### Function: `GetExclusion`
- **File:** `pkg/pg/repo.go:93`
- **Type:** Method on `*RepoExcluder`
- **Signature:**
  ```go
  func (r *RepoExcluder) GetExclusion(ctx context.Context, provider, repoFullName string) (*time.Time, string, error)
  ```
- **Operation:** SELECT
- **Target Table:** `repos`
- **Input Parameters:**
  - `ctx context.Context` - Context for the operation
  - `provider string` - Repository provider
  - `repoFullName string` - Repository full name
- **Return Type:** `(*time.Time, string, error)` - Excluded timestamp, reason, and error
- **SQL Operation:**
  ```sql
  SELECT excluded_at, excluded_reason
  FROM repos
  WHERE provider = $1 AND repo_full_name = $2
  ```
- **Special Handling:**
  - Converts nullable database fields to Go nil/empty string
  - Returns (nil, "", error) if query fails
  - Used for checking current exclusion status

---

### Function: `ListExclusions`
- **File:** `pkg/pg/repo.go:126`
- **Type:** Method on `*RepoExcluder`
- **Signature:**
  ```go
  func (r *RepoExcluder) ListExclusions(ctx context.Context) ([]ExclusionInfo, error)
  ```
- **Operation:** SELECT
- **Target Table:** `repos`
- **Input Parameters:**
  - `ctx context.Context` - Context for the operation
- **Return Type:** `([]ExclusionInfo, error)` - Array of excluded repo information
  - `ExclusionInfo.Provider string`
  - `ExclusionInfo.RepoFullName string`
  - `ExclusionInfo.ExcludedAt time.Time`
  - `ExclusionInfo.ExcludedReason string`
- **SQL Operation:**
  ```sql
  SELECT provider, repo_full_name, excluded_at, excluded_reason
  FROM repos
  WHERE excluded_at IS NOT NULL
  ORDER BY excluded_at DESC
  ```
- **Special Handling:**
  - Only returns repos where excluded_at IS NOT NULL
  - Sorted by excluded_at DESC (most recent first)
  - Used for administrative overview

---

## 8. Helper Functions

### Function: `RepoExists`
- **File:** `pkg/service/exclusion.go:64`
- **Type:** Method on `*RepoChecker`
- **Signature:**
  ```go
  func (r *RepoChecker) RepoExists(ctx context.Context, provider, repoFullName string) bool
  ```
- **Operation:** SELECT
- **Target Table:** `repos`
- **Input Parameters:**
  - `ctx context.Context` - Context for the operation
  - `provider string` - Repository provider
  - `repoFullName string` - Repository full name
- **Return Type:** `bool` - True if repo exists, false otherwise
- **SQL Operation:**
  ```sql
  SELECT 1
  FROM repos
  WHERE provider = $1 AND repo_full_name = $2
  LIMIT 1
  ```
- **Special Handling:**
  - Fail-safe design: returns false on any error (including query errors)
  - Returns false for empty inputs
  - Used for validation before database operations

---

### Function: `SetRepoExclusion`
- **File:** `pkg/service/exclusion.go:220`
- **Type:** Function
- **Signature:**
  ```go
  func SetRepoExclusion(ctx context.Context, db Transactioner, provider, repoFullName, reason string) error
  ```
- **Operation:** Wrapper function
- **Special Handling:**
  - Convenience wrapper that calls `SetRepoExclusionWithActor` with actor="system"
  - Maintains backward compatibility

---

### Function: `ClearRepoExclusion`
- **File:** `pkg/service/exclusion.go:369`
- **Type:** Function
- **Signature:**
  ```go
  func ClearRepoExclusion(ctx context.Context, db Transactioner, provider, repoFullName string) error
  ```
- **Operation:** Wrapper function
- **Special Handling:**
  - Convenience wrapper that calls `ClearRepoExclusionWithActor` with actor="system"
  - Maintains backward compatibility

---

## Summary Statistics

- **Total Functions Documented:** 16
- **INSERT Operations:** 1 (`RecordExclusionAudit`)
- **UPSERT Operations:** 3 (`UpsertAliases`, `BatchUsersUpsertQuery`, `IngestEmailResolution`)
- **UPDATE Operations:** 1 (`ApplyExclusion`)
- **DELETE Operations:** 1 (`DeleteAdminAliases`)
- **TRANSACTION Operations:** 2 (`SetRepoExclusionWithActor`, `ClearRepoExclusionWithActor`)
- **SELECT Operations:** 4 (`UsersSelectByLoginsQuery`, `GetAdminAliases`, `GetExclusion`, `ListExclusions`)
- **Helper/Wrapper Functions:** 4 (`RepoExists`, `SetRepoExclusion`, `ClearRepoExclusion`, `RecordExclusionAudit` impl)

## Tables Targeted

1. `users` - User identity table
2. `user_aliases` - Login alias mappings  
3. `email_resolution` - Email→login resolution results
4. `repos` - Repository identity with exclusion tracking
5. `exclusion_audit_log` - Audit trail for exclusion changes

## Conflict Resolution Patterns

1. **DO NOTHING** - `BatchUsersUpsertQuery` (preserves existing on conflict)
2. **DO UPDATE** - `UpsertAliases` (updates all fields on conflict)
3. **DO UPDATE with WHERE** - `IngestEmailResolution` (selective update based on source/timestamp)
4. **Transactions** - Exclusion operations (atomic multi-table updates)

## Key Design Patterns

1. **Bulk Operations:** Most write operations use UNNEST with array parameters for efficiency
2. **Conflict Resolution:** ON CONFLICT clauses implement business logic (manual source wins, newer timestamp wins)
3. **Audit Trail:** Service layer functions maintain before/after state in audit logs
4. **Idempotency:** Most operations are designed to be safely re-runnable
5. **Transaction Safety:** Critical operations use transactions to ensure atomicity
6. **Nullable Handling:** Extensive use of pointers and NULL checks for optional fields
7. **Validation:** Input validation before database operations (format, existence checks)
8. **Counter Tracking:** Some operations return detailed statistics (ingested vs skipped counts)
9. **Interface-based Design:** Uses Executor/DBExecutor interfaces for testability
10. **Fail-safe Validation:** Helper functions like `RepoExists` return false on errors rather than propagating

## Notes

- Most database operations are in `pkg/pg/` package (PostgreSQL-specific implementations)
- Service layer (`pkg/service/`) provides transactional business logic wrappers
- Identity layer (`pkg/identity/`) provides domain models for email resolution
- All exclusion operations are wrapped in transactions for audit trail consistency
- Conflict resolution implements business rules (manual wins, newer timestamps win)
- No dedicated INSERT/Upsert functions found for `repo_user_daily_tool` table in main codebase (only test fixtures)
