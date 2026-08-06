# Author Field Patterns Search Results

## Task: Search for author field pattern usage

**Bead ID:** cg-5l49y  
**Date:** 2026-08-06  
**Status:** ✅ Complete

---

## Summary

Searched the entire commitgraph codebase for author field patterns including `author.Login`, `author.Name`, `author.Email`, git author handling, and author login extraction points.

---

## Findings

### 1. Author Field Patterns

#### ✅ `AuthorEmail` Pattern
**Found in 3 files:**
- `cmd/load-email-resolution-from-queue-api/main.go` - Queue API email resolution loading
- `cmd/extract-sample-cache-data/main.go` - Sample cache data extraction  
- `pkg/rollup/rollup.go` - Commit rollup processing

#### ✅ `AuthorName` Pattern
**Found in 1 file:**
- `pkg/rollup/rollup.go` - Commit struct definition and rollup computation

#### ❌ `AuthorLogin` Pattern
**NOT FOUND** - This pattern doesn't exist directly in the codebase

#### ❌ `author.Login` / `author.Name` / `author.Email`
**NOT FOUND** - These patterns don't exist in the codebase

---

### 2. Git Author Handling Patterns

#### ✅ Commit Author Extraction
**Primary location:** `pkg/rollup/rollup.go:57-64`

```go
type Commit struct {
    SHA         string    // Commit SHA
    AuthorEmail string    // Author email (for identity resolution)
    AuthorName  string    // Author name
    CommittedAt time.Time // Commit date (UTC)
    Message     string    // Commit message
    Tools       []string  // Detected AI tools (empty if no AI tool detected)
}
```

#### ❌ go-git Library Usage
**NOT FOUND** - The codebase does NOT use the `go-git` library

#### ✅ Git Command Execution
**Found in test files:**
- `pkg/warmstart/sanity_check_error_test.go` - Multiple `exec.Command("git", ...)` calls
- `pkg/warmstart/extract_test.go` - Git commands for bare repo initialization
- `pkg/warmstart/fallback_example.go` - Placeholder implementation mentions

---

### 3. Author Login Extraction Points

#### ✅ Core Identity Resolution
**Primary package:** `pkg/identity/`

**Key files:**
- `pkg/identity/ingest.go` - ResolutionRow struct with Email, Login, Source, ResolvedAt
- `pkg/identity/snapshot_test.go` - Login field verification tests
- `pkg/identity/integration_test.go` - NULL login handling integration tests
- `pkg/identity/verify_null_handling_test.go` - NULL login verification

**Data structures:**
```go
type ResolutionRow struct {
    Email      string    // Email address (primary key)
    Login      string    // Resolved GitHub login
    Source     Source    // Source of this resolution: live, seed, or manual
    ResolvedAt time.Time // When this resolution was made
}
```

#### ✅ Database Layer
**Primary package:** `pkg/pg/`

**Key files:**
- `pkg/pg/identity.go` - Login field handling in database queries
- `pkg/pg/user_aliases.go` - SourceLogin/TargetLogin alias management
- `pkg/pg/users.go` - UsersSelectByLoginsQuery for login-based user lookup

#### ✅ Author Login Cache Operations
**Key commands:**
- `cmd/seed-author-login-cache/main.go` - Seeds email_resolution from claude-leaderboard's frozen cache
- `cmd/extract-sample-cache-data/main.go` - Extracts samples of author_login_cache pairs for testing

**Table references:**
- `author_login_cache` table mentioned in SQL files:
  - `test_null_logins.sql`
  - Multiple test references

#### ✅ Login Revalidation
**Container:** `containers/login-revalidation-worker/main.go`
- Connects to queue-api endpoint for email resolution ingest
- Uses queue-api client for posting login updates

---

### 4. Database Schema References

#### Email Resolution Table
**Schema:** `migrations/00001_initial_schema.sql`

```sql
CREATE TABLE IF NOT EXISTS email_resolution (
  email       TEXT PRIMARY KEY,
  login       TEXT NOT NULL,
  source      TEXT NOT NULL,          -- 'live' | 'seed' | 'manual'
  resolved_at TIMESTAMPTZ NOT NULL
);
```

#### Users Table
**Schema:** `migrations/00001_initial_schema.sql`

```sql
CREATE TABLE IF NOT EXISTS users (
  user_id    BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  login      TEXT NOT NULL UNIQUE,    -- canonical GitHub login
  profile_url TEXT,
  avatar_url  TEXT
);
```

#### Author Login Cache Table
**Schema:** `test_null_logins.sql`

```sql
CREATE TABLE author_login_cache (
  author_email TEXT,
  github_login TEXT,
  resolved_at  TEXT
);
```

---

### 5. Co-Authored-By Patterns

#### ✅ References Found
- `containers/queue-api/schema_test.go:180` - Mentions "Catalog version bumps (e.g., new Co-Authored-By pattern)"

**Note:** This appears to be a comment/annotation rather than active parsing code.

---

## Files Containing Author Field References

### Core Processing
1. `pkg/rollup/rollup.go` - Commit AuthorEmail/AuthorName fields and rollup logic
2. `pkg/identity/ingest.go` - ResolutionRow with Login field
3. `pkg/pg/identity.go` - Login field database operations
4. `pkg/pg/user_aliases.go` - SourceLogin/TargetLogin alias management

### Commands and Tools
5. `cmd/seed-author-login-cache/main.go` - Author login cache seeding
6. `cmd/extract-sample-cache-data/main.go` - Sample cache extraction  
7. `cmd/load-email-resolution-from-queue-api/main.go` - Email resolution loading
8. `containers/login-revalidation-worker/main.go` - Login revalidation worker

### Test Files
9. `pkg/identity/snapshot_test.go` - Login verification tests
10. `pkg/identity/integration_test.go` - NULL login integration tests
11. `pkg/identity/verify_null_handling_test.go` - NULL login verification
12. `pkg/warmstart/sanity_check_error_test.go` - Git command execution tests
13. `pkg/warmstart/extract_test.go` - Git operations in tests

### Database Schemas
14. `migrations/00001_initial_schema.sql` - Primary schema definitions
15. `test_null_logins.sql` - Author login cache test schema
16. `containers/queue-api/schema.sql` - Queue API schema

---

## Key Architectural Patterns

### 1. Author Information Flow
```
Git Commits → AuthorEmail/AuthorName (pkg/rollup/Commit)
                ↓
            Email Resolution (email_resolution table)
                ↓
            Login Resolution (author_login_cache → email_resolution)
                ↓
            User Identity (users table with login)
```

### 2. No Direct go-git Usage
- The codebase does NOT use the `go-git` library
- Git operations are performed via external command execution
- Test files use `exec.Command("git", ...)` for bare repo operations

### 3. Author Login Extraction Pattern
- Author logins are resolved through email_resolution table
- NULL login handling is extensively tested
- Three resolution sources: 'live', 'seed', 'manual'
- Conflict resolution: manual always wins, newest timestamp otherwise

---

## Acceptance Criteria Status

✅ **Searched for author.Login pattern across codebase** - Pattern not found (uses Login directly)  
✅ **Searched for author.Name and author.Email patterns** - Found AuthorName/AuthorEmail in pkg/rollup  
✅ **Found all files with git author handling code** - Found in pkg/rollup and test files  
✅ **Listed files where author login fields are referenced** - Comprehensive list above  

---

## Additional Context

### Related Documentation Found
- `docs/parsing-error-catalog.md` - Parsing error documentation
- `docs/cg-45rhy-parsing-error-catalog.md` - Comprehensive parsing error catalog
- `cmd/extract-sample-cache-data/README.md` - Author login cache extraction
- `cmd/seed-author-login-cache/testdata/README.md` - Test data documentation

### Queue-API Integration
- Queue API provides work queue for repository cloning and processing
- Email resolution flows through queue-api to commitgraph database
- Schema defined in `containers/queue-api/schema.sql`

---

## Conclusion

The codebase uses a structured approach to author field handling:
- Author emails and names are captured in the Commit struct for rollup processing
- Author logins are resolved through the email_resolution table  
- No direct `author.Login` pattern exists; instead, Login is a top-level field
- Git operations use command execution rather than go-git library
- Extensive NULL login handling and testing demonstrates robust identity resolution

---

**Search completed:** 2026-08-06  
**Total files examined:** ~100+ Go files, 20+ SQL/migration files, 15+ documentation files  
**Author field patterns identified:** 3 distinct patterns (AuthorEmail, AuthorName, Login)  
**Author login extraction points:** 6 primary locations identified
