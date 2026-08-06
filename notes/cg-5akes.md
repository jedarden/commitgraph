# Bead cg-5akes: Skipped and Ingested Counter Tracking

## Status: ✅ COMPLETE (Already Implemented)

## Acceptance Criteria Verification

All acceptance criteria for this bead have been verified as **already implemented** in the codebase:

### ✅ 1. Skipped counter increments with reason for non-ingested records
**Location:** `pkg/identity/ingest.go:74-75, 152-155`

- `Ingester.Skipped int64` - Tracks total skipped records
- `Ingester.SkipDetails map[SkipReason]int64` - Tracks breakdown by reason
- `Ingester.IngestResolution()` line 152-155: Accumulates skip counts and details

**Skip Reasons Defined:**
- `SkipReasonConflictManual` - Existing manual source won
- `SkipReasonConflictOlder` - Existing record has newer timestamp
- `SkipReasonValidation` - Row failed validation
- `SkipReasonDatabase` - Database error during ingest
- `SkipReasonOther` - Other skip reasons

### ✅ 2. Ingested counter increments on successful writes
**Location:** `pkg/identity/ingest.go:70-71, 151`

- `Ingester.Ingested int64` - Tracks successfully written records
- `Ingester.IngestResolution()` line 151: Increments counter on successful ingest

### ✅ 3. Records are counted in exactly one category (skipped OR ingested)
**Location:** `pkg/identity/ingest.go:137` and `pkg/pg/identity.go:224-225`

- `Ingester.Processed` tracks total records seen
- Database implementation ensures: `actualIngested + actualSkipped = len(rows)`
- Tests verify invariant: `Processed = Ingested + Skipped`

**Test Coverage:**
- `TestIngester_ProcessedInvariant` - Verifies single-batch invariant
- `TestIngester_ProcessedInvariant_MultipleBatches` - Verifies multi-batch invariant
- `TestIngestEmailResolution_CounterTracking` - Verifies PostgreSQL implementation

### ✅ 4. Skipped reasons are captured for logging
**Location:** `pkg/identity/ingest.go:51-59, 175-177` and `pkg/pg/identity.go:167-173`

- `SkipReason` type with String() method for logging
- `GetSkipDetails()` method returns breakdown map
- PostgreSQL implementation categorizes reasons at database layer:
  - Lines 167-169: Conflict with existing manual source
  - Lines 171-172: Conflict with newer timestamp

## Implementation Details

### Ingester Structure (`pkg/identity/ingest.go`)

```go
type Ingester struct {
    db          DB
    Processed   int64                 // Total records processed
    Ingested    int64                 // Successfully written records
    Skipped     int64                 // Records not written due to conflicts
    SkipDetails map[SkipReason]int64 // Breakdown of skip reasons
}
```

### Database Layer Implementation (`pkg/pg/identity.go`)

The PostgreSQL implementation performs intelligent skip reason tracking:

1. **Step 1:** Fetch existing rows for conflict detection (lines 103-146)
2. **Step 2:** Predict which rows will be skipped and categorize reasons (lines 148-177)
   - New rows: Mark as ingested
   - Conflict with manual: Categorize as `SkipReasonConflictManual`
   - Conflict with newer: Categorize as `SkipReasonConflictOlder`
3. **Step 3:** Execute bulk upsert with ON CONFLICT rule (lines 179-214)
4. **Step 4:** Return results with accurate counts (lines 216-234)

### Test Coverage

All counter tracking functionality is thoroughly tested:

**Identity Package Tests:**
- `TestIngester_SkipDetailsInitialization` - Verifies map initialization
- `TestIngester_IngestedCounter` - Verifies ingested counter increments
- `TestIngester_SkippedCounter` - Verifies skipped counter increments
- `TestIngester_SkipDetailsAccumulation` - Verifies accumulation across batches
- `TestIngester_ProcessedInvariant` - Verifies mutual exclusivity invariant
- `TestIngester_ProcessedInvariant_MultipleBatches` - Verifies multi-batch consistency
- `TestIngester_AllSkipReasons` - Verifies all skip reason types

**PostgreSQL Package Tests:**
- `TestIngestEmailResolution_CounterTracking` - Comprehensive counter tracking scenarios:
  - All new rows → all ingested
  - Existing manual wins → all skipped (conflict_manual)
  - Existing newer wins → skipped (conflict_older)
  - New manual wins → all ingested
  - Mixed results → proper categorization

## Usage Example

From `cmd/seed-email-resolution/main.go`:

```go
ingester := identity.NewIngester(pg.NewIdentityIngester(...))

// Ingest batches
for _, batch := range batches {
    err := ingester.IngestResolution(ctx, batch)
    // Check error...
}

// Get statistics
processed := ingester.GetProcessed()
ingested := ingester.GetIngested()
skipped := ingester.GetSkipped()
skipDetails := ingester.GetSkipDetails()

// Log summary
log.Printf("Processed: %d, Ingested: %d, Skipped: %d\n", processed, ingested, skipped)
for reason, count := range skipDetails {
    log.Printf("  %s: %d\n", reason, count)
}
```

## Conclusion

**All acceptance criteria for bead cg-5akes are already fully implemented and tested.**

The counter tracking implementation includes:
- ✅ Skipped counter with detailed reason categorization
- ✅ Ingested counter for successful writes
- ✅ Mutual exclusivity guarantee (each record counted exactly once)
- ✅ Skip reasons captured and exposed via getter methods
- ✅ Comprehensive test coverage

No additional code changes are required to complete this bead.

**Implementation Date:** This functionality was already present in the codebase when bead cg-5akes was claimed.
**Bead Status:** Ready to close as complete.
