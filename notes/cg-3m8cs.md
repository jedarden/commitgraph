# cg-3m8cs: Commit Database Operations Search Results

## Task
Find all commit insertion/upsert functions in pkg/pg/.

## Method
- Searched pkg/pg/ for INSERT INTO commits, UPSERT patterns, and functions with "commit" in the name
- Read all non-test Go files in pkg/pg/
- Examined database schema (migrations/00001_initial_schema.sql)
- Cross-referenced with rollup package for context

## Key Finding: No Commits Table Exists

**There are NO commit insertion/upsert functions in pkg/pg/ because there is NO `commits` table in the database schema.**

The database schema (migrations/00001_initial_schema.sql) contains only these tables:
1. `repos` - repository identity with exclusion tracking
2. `users` - developer identity
3. `email_resolution` - email→login resolution results
4. `user_aliases` - login→login alias mapping
5. `repo_user_daily_tool` - rollup table (AI-tool-tagged commits only, aggregated)
6. `corpus_stats` - global scalar totals

## Actual INSERT/UPSERT Operations Found in pkg/pg/

### 1. pkg/pg/identity.go:94 - `IngestEmailResolution`
- **Function:** `(i *IdentityIngester) IngestEmailResolution(ctx context.Context, rows []identity.ResolutionRow)`
- **Operation:** Bulk upsert into `email_resolution` table
- **SQL Pattern:** `INSERT INTO email_resolution ... ON CONFLICT (email) DO UPDATE ...`
- **Purpose:** Resolves email addresses to logins for commit authors
- **Lines:** 94-234

### 2. pkg/pg/user_aliases.go:46 - `UpsertAliases`
- **Function:** `(a *AliasIngester) UpsertAliases(ctx context.Context, rows []AliasRow)`
- **Operation:** Bulk upsert into `user_aliases` table
- **SQL Pattern:** `INSERT INTO user_aliases ... ON CONFLICT (source_login) DO UPDATE ...`
- **Purpose:** Maps non-canonical logins to canonical logins
- **Lines:** 46-88

### 3. pkg/pg/users.go:45 - `BatchUsersUpsertQuery` (constant)
- **Constant:** `BatchUsersUpsertQuery`
- **Operation:** SQL query template for bulk user upsert
- **SQL Pattern:** `INSERT INTO users (login) SELECT unnest($1::text[]) ON CONFLICT (login) DO NOTHING RETURNING login, user_id`
- **Purpose:** Creates new user records from logins
- **Lines:** 45-50

### 4. pkg/pg/repo.go:45 - `ApplyExclusion`
- **Function:** `(r *RepoExcluder) ApplyExclusion(ctx context.Context, req ExclusionRequest)`
- **Operation:** UPDATE operation on `repos` table (not INSERT)
- **SQL Pattern:** `UPDATE repos SET excluded_at = ...`
- **Purpose:** Applies or clears repo exclusions
- **Lines:** 45-89

## What About Commits?

Per pkg/rollup/rollup.go and docs/plan/plan.md:

1. **Raw commit data is stored externally in Parquet files**, not in PostgreSQL
2. **The `repo_user_daily_tool` table stores pre-aggregated rollups** (one row per user/repo/tool/day with a commit count)
3. **The commits column in `repo_user_daily_tool` is a counter**, not individual commit records
4. Rollup computation happens in Go code (pkg/rollup/rollup.go:91 - `ComputeRollup`) before any database insert

## Test-Only References

The only test files mentioning "commits" INSERT operations are:
- pkg/pg/invariant_4_integration_test.go:100,130 - Inserts into `repo_user_daily_tool` (testing the rollup table, not individual commits)

## Conclusion

**No individual commit record insertion/upsert functions exist in pkg/pg/** because:
1. There is no `commits` table in the schema
2. Commit data lives in external Parquet files
3. PostgreSQL only stores aggregated rollups in `repo_user_daily_tool.commits` column

If you need to find where commit data is actually written to disk, search for:
- Parquet file writing code (likely in a separate package)
- Queue-api service (containers/queue-api/)
- Warmstart snapshot code (pkg/warmstart/)
