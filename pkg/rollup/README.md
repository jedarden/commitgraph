# Rollup Package

The `rollup` package provides rollup computation for commitgraph v2.

## Purpose

This package implements the core rollup computation that aggregates AI-tool-tagged commits by `(user, repo, tool, day)` while applying **date quarantine filtering** to exclude out-of-range commits from the rollup table.

## Date Quarantine

The quarantine filter excludes commits with `committed_at` outside the range `[2005-01-01, today+1]` (UTC) from the rollup computation:

- **Lower bound**: 2005-01-01 00:00:00 UTC (inclusive)
- **Upper bound**: Today + 1 day 23:59:59.999999999 UTC (inclusive)

This prevents malformed or maliciously-dated commits (e.g., the 2170 incident) from corrupting the rollup while preserving the raw data in the Parquet artifact.

## Usage

### Creating Quarantine Bounds

```go
import "github.com/jedarden/commitgraph/pkg/rollup"

// Create bounds for the current date
bounds := rollup.NewQuarantineBounds(time.Now().UTC())
```

### Filtering Commits

```go
// Check if a commit date is within bounds
if bounds.IsIncluded(commit.CommittedAt) {
    // Include in rollup
}
```

### Computing Rollups

```go
commits := []rollup.Commit{
    {
        SHA:         "abc123",
        AuthorEmail: "user@example.com",
        CommittedAt: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
        Tools:       []string{"claude"},
    },
}

// Compute rollup with date filtering
rollupRows := rollup.ComputeRollup(commits, repoID, bounds)

// rollupRows contains only commits within the date bounds
// The original commits slice is preserved for Parquet writing
```

## Architecture

The rollup computation is designed to preserve the **raw/filtered split**:

1. **Rollup computation** (step 4): Excludes out-of-range commits via `ComputeRollup()`
2. **Parquet artifact** (step 5b): Writes all commits with unclamped `committed_at` values

This separation ensures:
- Ranking queries never see out-of-range dates
- Raw data is preserved for re-detection and analysis
- The 2170 incident cannot recur

## Testing

The package includes comprehensive tests covering:

- Boundary conditions (2004-12-31, 2005-01-01, today+1, today+2)
- The 2170 incident scenario
- Aggregation by day and tool
- Parquet data preservation

Run tests with:
```bash
go test ./pkg/rollup/... -v
```

## Acceptance Criteria

All acceptance criteria from bead cg-19os are met:

- ✅ 2004-12-31 excluded from rollup
- ✅ 2005-01-01 included in rollup  
- ✅ today+1 included, today+2 excluded
- ✅ Parquet artifact preserves unclamped `committed_at`
- ✅ Synthetic 2170 fixture produces zero rollup rows

## Integration

This package is used by:
- `migration/migrate_corpus.py` (via Go bindings or CLI)
- Future clone-worker implementation (step 4 rollup computation)

## References

- Plan: `docs/plan/plan.md` - Architecture, quarantine section
- Schema: `migrations/001_initial_schema.sql` - `repo_user_daily_tool` table
- Bead: cg-19os - Original task specification
