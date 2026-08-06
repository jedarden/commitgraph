# cg-1mss9: Seed script verification for cg-3i96

## Task
Locate and verify seed script from cg-3i96 child 3.

## What was found

### cg-3i96 child 3 reference
The "child 3" reference in the task description refers to the third item in the cg-3i96 bead's notes file (`notes/cg-3i96.md`), which documents the seed script implementation and status.

### Seed script location
**Full path:** `/home/coding/commitgraph/cmd/seed-author-login-cache/main.go`

### Script verification
The seed script is **fully implemented and accessible**:

**File exists:** ✅ `/home/coding/commitgraph/cmd/seed-author-login-cache/main.go` is readable

**Implementation status:** ✅ Complete - ready to run once source database is accessible

**What the script does:**
- Reads from `~/backups/claude-leaderboard/hot.db`'s `author_login_cache` table
- Filters to positive resolutions only (skips NULL/empty logins)  
- Sets `source='seed'` and preserves original `resolved_at` timestamps
- Ingests into Postgres via the identity ingest endpoint
- Logs summary: pairs read, accepted, rejected

**Usage:**
```bash
cd ~/commitgraph
go run cmd/seed-author-login-cache/main.go \
  -db-host <host> \
  -db-user <user> \
  -db-password <password> \
  -claude-leaderboard-db ~/backups/claude-leaderboard/hot.db
```

**Current blocker (per cg-3i96):**
The source database (`~/backups/claude-leaderboard/hot.db`) is currently inaccessible due to PVC volume attachment issues on the apexalgo-iad cluster. The script itself is ready to run.

## Acceptance criteria
- [x] cg-3i96 child 3 reference is reviewed
- [x] Seed script file is located
- [x] Script path is documented
- [x] Script is accessible and readable

## References
- Parent bead: cg-3i96
- Plan section: docs/plan/plan.md "Explicitly out of scope"
- Script: cmd/seed-author-login-cache/main.go
- Related notes: notes/cg-3i96.md
