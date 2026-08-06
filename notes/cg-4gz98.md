# cg-4gz98: Seed Script Verification

## Task Summary

Located and verified the seed script referenced in cg-3i96 child 3 (cg-58yq).

## Seed Script Location

**Full Path:** `/home/coding/commitgraph/cmd/seed-author-login-cache/main.go`

## Script Details

### Main Script
- **File:** `cmd/seed-author-login-cache/main.go`
- **Type:** Go source file (requires compilation to binary)
- **Size:** 12,410 bytes
- **Permissions:** `-rw-r--r--` (readable, writable by owner)
- **Language:** Go (Golang)

### Purpose
Seeds the `email_resolution` table from claude-leaderboard's frozen `author_login_cache` table with 349,425 `(email, login)` pairs as a one-time migration.

### Interpreter/Build Requirements
- **Language:** Go
- **Build Command:** `go build cmd/seed-author-login-cache/main.go` (or via `cargo build` equivalent in the project's build system)
- **Dependencies:** 
  - `github.com/mattn/go-sqlite3` (SQLite driver)
  - `github.com/lib/pq` (PostgreSQL driver)
  - Internal packages: `github.com/jedarden/commitgraph/pkg/identity` and `github.com/jedarden/commitgraph/pkg/pg`

### Test Infrastructure
**Location:** `cmd/seed-author-login-cache/testdata/`

Contains:
- `sample.db` - SQLite database with 50 sample pairs for testing
- `verify_sample.sh` - Shell script to verify the sample database
- `README.md` - Documentation for test data

### Usage Pattern
The script requires PostgreSQL connection parameters and accepts an optional path to the claude-leaderboard SQLite database:

```bash
seed-author-login-cache \
  -claude-leaderboard-db ~/backups/claude-leaderboard/hot.db \
  -db-host <host> \
  -db-user <user> \
  -db-password <password> \
  -db-name commitgraph \
  -db-port 5432 \
  -sslmode require \
  -batch-size 1000
```

### Key Features
1. Reads from `~/backups/claude-leaderboard/hot.db` (author_login_cache table)
2. Filters to positive resolutions only (skips NULL/empty logins)
3. Sets `source='seed'` on all ingested rows
4. Preserves original `resolved_at` timestamps from cache
5. Ingests in batches (default 1000 rows per batch)
6. Logs summary: pairs read, positive resolutions, accepted, rejected

## Verification Status

✅ **Script exists and is readable**
✅ **Full path documented**
✅ **Build requirements identified** (Go compilation required)
✅ **Test infrastructure available**
✅ **Source database location confirmed** (`~/backups/claude-leaderboard/hot.db`)

## Related Beads

- **Parent:** cg-3i96 - Seed email_resolution from claude-leaderboard's frozen author_login_cache
- **Self:** cg-4gz98 - Locate and verify seed script from cg-3i96
- **Reference:** cg-58yq - Test that running the seed twice is a no-op

## Verification Date

2026-08-06
