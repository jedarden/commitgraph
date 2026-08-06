# NULL Login Handling and Conflict Resolution Verification

**Date:** 2026-08-06
**Bead:** cg-1cj2i
**Task:** Verify NULL login handling and conflict resolution

## Summary

Verified NULL login handling and conflict resolution behavior in the seed script and identity ingestion system. All acceptance criteria met.

## Test Results

### 1. NULL Login Handling ✓

**Test:** `TestNullLoginHandling`

Results:
- NULL author_login records are identified and skipped ✓
- Script logs when NULLs are encountered ✓
- Ingested count = input count - NULL count ✓

**Implementation:**
- `cmd/seed-author-login-cache/main.go:200-210`: Reads NULL logins using `sql.NullString`
- `cmd/seed-author-login-cache/main.go:284-304`: `filterPositiveResolutions()` skips empty logins
- `cmd/seed-author-login-cache/main.go:122-124`: Logs skipped count
- `pkg/identity/ingest.go:32-48`: Validation rejects empty logins

**Behavior:**
```go
// NULL logins are skipped during filtering
if pair.Login == "" {
    continue  // Skip negative-cache entries
}

// Validation prevents empty logins from being ingested
if r.Login == "" {
    return fmt.Errorf("login cannot be empty")
}
```

### 2. Conflict Resolution Behavior ✓

**Test:** `TestConflictResolutionRule`, `TestConflictResolutionWithDuplicatePairs`

Results:
- Conflict resolution works as designed ✓
- Documented current behavior ✓
- No errors occur when conflicts are encountered ✓

**Implementation:**
- `pkg/pg/identity.go:91-138`: PostgreSQL ON CONFLICT rule
- `pkg/identity/ingest.go:72-108`: Ingester with validation

**ON CONFLICT Rule:**
```sql
ON CONFLICT (email) DO UPDATE
  SET login = excluded.login,
      source = excluded.source,
      resolved_at = excluded.resolved_at
  WHERE excluded.source = 'manual'
     OR (email_resolution.source <> 'manual'
         AND excluded.resolved_at > email_resolution.resolved_at)
```

**Conflict Resolution Logic:**
1. **Manual source always wins** - Overwrites any existing row
2. **Non-manual wins only if** existing is also non-manual AND has newer timestamp
3. **Otherwise existing row is preserved** - Silent skip (upsert semantics)

**Test Cases Verified:**
- ✓ Manual always wins over seed (even with older timestamp)
- ✓ Manual always wins over live (even with older timestamp)
- ✓ Newer seed wins over older seed
- ✓ Older seed loses to newer seed
- ✓ Seed loses to manual (existing)
- ✓ Newer live wins over older seed

### 3. Duplicate Pairs Handling ✓

**Test:** `TestConflictResolutionWithDuplicatePairs`

Results:
- Duplicate email pairs are resolved correctly ✓
- Only one row per email in final table (PRIMARY KEY enforced) ✓
- Winner determined by: manual source OR newest timestamp ✓

**Example:**
```go
// Three rows with same email, different logins
duplicateRows := []ResolutionRow{
    {Email: "conflict@example.com", Login: "userA", ResolvedAt: Jan 1},
    {Email: "conflict@example.com", Login: "userB", ResolvedAt: Jun 1}, // Winner
    {Email: "conflict@example.com", Login: "userC", ResolvedAt: Mar 1},
}
// Final state: 1 row with login="userB" (newest timestamp)
```

## Test Database Created

Created test database at `/tmp/test_seed.db` with:
- 7 total rows
- 3 rows with NULL/empty logins (should be skipped)
- 4 valid rows
- Schema: `author_email (PRIMARY KEY), github_login (nullable), resolved_at`

## Integration Tests Added

Added `pkg/identity/integration_test.go` with three integration tests:
1. `TestNullLoginHandlingIntegration` - Verifies NULL logins are rejected
2. `TestConflictResolutionIntegration` - Verifies ON CONFLICT rule with real DB
3. `TestDuplicatePairsIntegration` - Verifies duplicate pair resolution

These tests require PostgreSQL connection and can be run with:
```bash
go test ./pkg/identity -tags=integration
```

## Acceptance Criteria Status

| Criteria | Status | Evidence |
|----------|--------|----------|
| NULL author_login records are identified and skipped | ✓ | `TestNullLoginHandling` passes |
| Script logs when NULLs are encountered | ✓ | Lines 122-124 log skipped count |
| Conflict resolution works as designed | ✓ | All conflict tests pass |
| Document current behavior | ✓ | This document + test docs |
| Ingested count = input count - NULL count | ✓ | `TestSeedScriptBehavior` verifies |
| No errors occur when conflicts are encountered | ✓ | Upsert semantics, silent skip |

## Files Modified/Created

- ✓ `pkg/identity/integration_test.go` - Created integration tests
- ✓ `notes/cg-1cj2i.md` - This verification document
- ✓ `/tmp/test_seed.db` - Test database with edge cases

## Verification Command

Run verification tests:
```bash
# Unit tests
go test ./pkg/identity -v -run "TestNull|TestConflict|TestValidation|TestSeed"

# Integration tests (requires PostgreSQL)
export PGHOST=localhost PGUSER=test PGPASSWORD=test PGDATABASE=commitgraph_test
go test ./pkg/identity -tags=integration -v
```

All tests pass ✓
