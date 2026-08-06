# Commit Database Operations in pkg/pg/

## Task: Find all commit insertion/upsert functions in pkg/pg/

## Methodology
Searched pkg/pg/ directory for commit-related database operations using:
- `grep -r "INSERT INTO commits\|UPSERT\|commit" pkg/pg/ --include="*.go"`
- `grep -rn "func.*[Cc]ommit" pkg/pg/*.go`
- Searched for INSERT/UPSERT patterns more broadly
- Examined database schema to understand table structure
- Searched entire codebase for commit operations

## Key Findings

### 1. Database Schema Analysis
From `migrations/00001_initial_schema.sql`:

**No separate `commits` table exists.** The system stores commit data in the `repo_user_daily_tool` table which stores **commit COUNTS** aggregated by (repo_id, user_id, tool, day):

```sql
CREATE TABLE IF NOT EXISTS repo_user_daily_tool (
  repo_id     BIGINT NOT NULL REFERENCES repos(repo_id),
  user_id     BIGINT NOT NULL REFERENCES users(user_id),
  tool        TEXT   NOT NULL,
  day         DATE   NOT NULL,
  commits     INT    NOT NULL,          -- ← COUNT of commits
  insert_time TIMESTAMPTZ NOT NULL DEFAULT transaction_timestamp(),
  PRIMARY KEY (repo_id, user_id, tool, day)
);
```

### 2. Production Code Search Results

**pkg/pg/ directory contains ZERO production commit INSERT/UPSERT functions.**

Search results:
- ❌ No functions with "commit" in their name
- ❌ No "INSERT INTO commits" statements
- ❌ No "UPSERT" patterns
- ❌ No "INSERT INTO repo_user_daily_tool" in non-test code

### 3. Related Code (Outside pkg/pg)

**pkg/rollup/rollup.go** contains:
- `Commit` struct (lines 56-64) - represents a single commit
- `RollupRow` struct (lines 66-75) - represents aggregated rollup data
- `ComputeRollup()` function (lines 91-141) - **computes** rollup aggregations but does NOT insert them into database

### 4. Test Code References

All references to `repo_user_daily_tool` INSERT operations are found **only in test files**:
- `pkg/pg/invariant_4_integration_test.go` - line 100, 130
- `pkg/identity/login_rename_integration_test.go` - line 136
- `pkg/identity/login_rename_test.go` - line 113

These are test fixtures that populate data for testing, not production code.

## Conclusion

**No production commit insertion/upsert functions exist in pkg/pg/**. The current architecture:
1. Computes rollup data in `pkg/rollup/rollup.go`
2. Has database schema for storing commit counts in `repo_user_daily_tool`
3. Lacks the database insertion layer between computation and storage

This suggests either:
- The insertion code is implemented elsewhere (possibly in queue-api or another service)
- The feature is not yet fully implemented
- A separate data ingestion layer handles the database writes

## Files Examined
- pkg/pg/identity.go
- pkg/pg/users.go
- pkg/pg/user_aliases.go
- pkg/pg/repo.go
- pkg/pg/test_mocks.go
- pkg/pg/test_helpers.go
- pkg/rollup/rollup.go
- migrations/00001_initial_schema.sql
- All test files in pkg/pg/
