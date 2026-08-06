# cg-19os: Date Quarantine Implementation

## Summary

Implemented date quarantine filtering for rollup computation to exclude commits with `committed_at` outside `[2005-01-01, today+1]` from the rollup while preserving raw values in the Parquet artifact.

## Implementation

Created `pkg/rollup/` package with:

1. **`rollup.go`**: Core rollup computation with date filtering
   - `QuarantineBounds`: Defines inclusive date range [2005-01-01, today+1 23:59:59.999999999]
   - `IsIncluded()`: Filters commits by date bounds
   - `ComputeRollup()`: Aggregates (user, repo, tool, day, count) excluding out-of-range dates

2. **`rollup_test.go`**: Comprehensive test coverage
   - Boundary conditions: 2004-12-31 ✗, 2005-01-01 ✓, today+1 ✓, today+2 ✗
   - 2170 incident scenario
   - Aggregation by day and tool
   - Parquet data preservation verification

3. **`README.md`**: Package documentation and usage examples

## Key Design Decisions

### Date Range Semantics
- **Lower bound**: Fixed at 2005-01-01 00:00:00 UTC (inclusive)
- **Upper bound**: End of today+1 day 23:59:59.999999999 UTC (inclusive)
- **Rationale**: Matches the old compactor's quarantine logic exactly

### Raw/Filtered Split
- **Rollup computation**: Excludes out-of-range dates
- **Parquet artifact**: Preserves all commits with unclamped `committed_at`
- **Rationale**: Prevents recurrence of 2170 incident while preserving raw data

### Testing Strategy
- Unit tests for all acceptance criteria
- Edge case coverage (nanosecond precision)
- Synthetic fixture reproduction of 2170 incident

## Acceptance Criteria Met

All acceptance criteria from bead cg-19os:

- ✅ Commit dated 2004-12-31 excluded from rollup
- ✅ Commit dated 2005-01-01 included in rollup
- ✅ Commit dated today+1 included, today+2 excluded
- ✅ Parquet artifact retains unclamped `committed_at` values
- ✅ Synthetic 2170 fixture produces zero rollup rows

## Integration Points

This package will be integrated into:
1. **Migration**: `migration/migrate_corpus.py` → `_process_repo()` step 4
2. **Clone-worker**: Future implementation step 4 (rollup computation)

## Testing

All tests pass:
```bash
go test ./pkg/rollup/... -v
# PASS: all 8 test cases
```

## Files Created

- `pkg/rollup/rollup.go` - Core implementation
- `pkg/rollup/rollup_test.go` - Test coverage
- `pkg/rollup/README.md` - Documentation
- `notes/cg-19os.md` - This summary

## References

- Plan: `docs/plan/plan.md` lines 306-335 (quarantine gap, fix)
- Schema: `migrations/001_initial_schema.sql` (repo_user_daily_tool table)
- Bead: cg-19os (original task specification)
