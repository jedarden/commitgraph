# Idempotency Test Verification and Documentation (cg-68bm2)

## Summary

Verified and completed the idempotency test suite for the claude-leaderboard seed script.

## Changes Made

### 1. Fixed Test Failure
The `TestSeedIdempotency_InterleavedLiveResolution` test was failing because the `seedOnce()` function used an unconditional `ON CONFLICT DO UPDATE` that overwrote all existing data, including newer live resolutions.

**Fix:** Modified the SQL conflict resolution to only update when seed data has a newer `resolved_at`:
```sql
ON CONFLICT(email) DO UPDATE SET
    login = excluded.login,
    source = excluded.source,
    resolved_at = excluded.resolved_at
WHERE excluded.resolved_at > resolved_at
```

This implements the plan.md requirement: "Conflict resolution rule: only update if the seed data has a newer resolved_at."

### 2. Enhanced Package Documentation
Added comprehensive package-level doc comment explaining:
- **Why tests exist:** Protect against accidental double-runs and verify conflict resolution
- **What each test validates:** Clear descriptions of all three tests
- **Plan reference:** Links to docs/plan/plan.md section on claude-leaderboard seed

## Test Results

All tests pass cleanly:
- ✅ `TestSeedIdempotency` - Verifies running seed twice produces identical results
- ✅ `TestSeedIdempotency_InterleavedLiveResolution` - Verifies seed respects newer live resolutions
- ✅ `TestIdempotencyPlaceholder` - Infrastructure verification

```
PASS
coverage: [no statements]
ok  	github.com/jedarden/commitgraph/cmd/identity-seed-claude-leaderboard	0.004s
```

## Coverage Note

Coverage shows `[no statements]` because `main.go` is a placeholder. The test infrastructure is complete and ready to cover actual seed logic when implemented.

## Discoverability

Tests are properly discoverable from repo root with `go test -tags=integration ./...`
