# Task cg-58yq: Idempotency Test Implementation

## Status: COMPLETE

The idempotency tests for `identity-seed-claude-leaderboard` were already fully implemented in the codebase at `cmd/identity-seed-claude-leaderboard/idempotency_test.go`.

## Acceptance Criteria Verification

All acceptance criteria from the task are fully met:

### 1. ✓ Snapshot capture after first run
**Test:** `TestSeedIdempotency` (line 222-227)
```go
// Step 4: Capture post-first-run snapshot
snapshot1, err := identity.CaptureSnapshot(db)
if err != nil {
    t.Fatalf("Failed to capture snapshot after first seed: %v", err)
}
t.Logf("✓ After first seed: %d rows, hash=%s", snapshot1.RowCount, snapshot1.Hash)
```

### 2. ✓ Second seed run
**Test:** `TestSeedIdempotency` (line 234-239)
```go
// Step 5: Run the seed script a second time
t.Log("Running second seed...")
if err := seedOnce(db); err != nil {
    t.Fatalf("Second seed failed: %v", err)
}
```

### 3. ✓ Identical snapshot assertion
**Test:** `TestSeedIdempotency` (line 249-260)
```go
// Step 7: Assert post-first-run and post-second-run snapshots are identical
identical, err := identity.CompareSnapshots(snapshot1, snapshot2)
if err != nil {
    t.Fatalf("Seed is NOT idempotent!\n%v", err)
}
if !identical {
    t.Fatal("Seed is NOT idempotent! CompareSnapshots returned false")
}
```

### 4. ✓ Interleaved live resolution test
**Test:** `TestSeedIdempotency_InterleavedLiveResolution` (lines 318-497)
- Seeds once with test data
- Inserts live row with newer `resolved_at` timestamp (line 395-405)
- Runs seed second time (line 444-448)
- Asserts live row is unchanged (line 461-472):
  - Same `resolved_at` value (still newer)
  - Same `source` value (still "live")
  - Same `login` value (live worker's resolution)

## Test Execution Results

All tests pass successfully:

```
=== RUN   TestSeedIdempotency
    ✓ Seed idempotency verified: no changes after second run
--- PASS: TestSeedIdempotency (0.00s)

=== RUN   TestSeedIdempotency_InterleavedLiveResolution
    ✓ Interleaved live resolution preserved: seed did NOT overwrite newer live data
--- PASS: TestSeedIdempotency_InterleavedLiveResolution (0.00s)

=== RUN   TestIdempotencyPlaceholder
    ✓ Test infrastructure verified successfully
--- PASS: TestIdempotencyPlaceholder (0.00s)

PASS
ok  	github.com/jedarden/commitgraph/cmd/identity-seed-claude-leaderboard	0.003s
```

## Implementation Details

The tests use the `identity` package's snapshot functionality:
- `identity.CaptureSnapshot(db)` - captures full database state with hash
- `identity.CompareSnapshots(snapshot1, snapshot2)` - byte-for-byte comparison
- `identity.WithFullRowData()` option for detailed row data

The seed logic uses the ON CONFLICT rule from the plan:
```sql
INSERT INTO email_resolution (email, login, source, resolved_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(email) DO UPDATE SET
    login = excluded.login,
    source = excluded.source,
    resolved_at = excluded.resolved_at
WHERE excluded.resolved_at > resolved_at
```

This ensures:
- No duplicate rows (PRIMARY KEY on email)
- No timestamp changes on re-seed (idempotent)
- Newer live resolutions are preserved (conflict resolution)

## Files Reviewed

- `/home/coding/commitgraph/cmd/identity-seed-claude-leaderboard/idempotency_test.go` - Main test file
- `/home/coding/commitgraph/pkg/identity/snapshot.go` - Snapshot capture and comparison
- `/home/coding/commitgraph/pkg/identity/ingest.go` - ResolutionRow and Source types

## Conclusion

The task is complete. The comprehensive idempotency test suite already exists and passes, covering all required scenarios:
- Basic idempotency (running seed twice produces identical results)
- Interleaved live resolution (seed respects newer live rows)
- Infrastructure verification (fixtures and setup)

No additional implementation needed. The tests verify the seed script is safe to re-run without corrupting state.
