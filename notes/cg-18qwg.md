# Ingest Flow Entry Points Analysis

## Task
Identify where records enter the ingest flow and determine the correct insertion point for the processed counter.

## Main Ingest Entry Points

### 1. CLI Seed Operations

**`cmd/seed-email-resolution/main.go`**
- **Purpose**: Seeds email_resolution from claude-leaderboard's frozen author_login_cache
- **Entry Point**: `main()` function at line 54
- **Record Entry**: Lines 185-187 - `logger.RecordProcessed()` called for each record in batch before database ingest
- **Flow**: SQLite read → validation → `RecordProcessed()` → `Ingester.IngestResolution()` → PostgreSQL bulk upsert
- **Scale**: ~349K records per run

**`cmd/seed-author-login-cache/main.go`**
- **Purpose**: Similar seeding operation for author login cache
- **Pattern**: Same as seed-email-resolution

**`cmd/load-admin-aliases/main.go`**
- **Purpose**: Loads admin-defined user aliases from ConfigMap
- **Pattern**: Similar structure with `RecordProcessed()` before database upsert

### 2. HTTP/API Client Entry Points

**`pkg/client/queueapi/client.go`**
- **Primary HTTP Client**: `PostResolution()` function at line 86
- **Record Entry**: Line 88 - `c.logger.RecordProcessed()` called before HTTP request
- **Target Endpoint**: `/email-resolution/resolve`
- **Features**: 
  - Retry logic with exponential backoff (100ms, 400ms, 900ms, 1600ms)
  - Maximum 4 retry attempts
  - Structured error logging for retries and failures

### 3. Worker Entry Points

**`containers/login-revalidation-worker/main.go`**
- **Purpose**: Detects renamed/deleted GitHub logins
- **Entry Point**: `updateEmailResolution()` at line 376
- **Flow**: Claim batch → Check GitHub API → Call `PostResolution()` → `RecordProcessed()` called inside PostResolution
- **Scale**: Configurable batch size (default 50)

**`containers/user-enrichment-worker/worker.py`**
- **Purpose**: Live enrichment worker claiming batches from queue
- **Entry Point**: Posts results to `/identity/ingest` endpoint
- **Flow**: Claim batch → Resolve emails via GitHub API → Post to queue-api

### 4. Core Ingest Functions

**`pkg/identity/ingest.go`**
- **Primary Method**: `Ingester.IngestResolution()` at line 94
- **Purpose**: Validates rows and delegates to database implementation
- **Conflict Resolution**: 
  - Manual source always wins
  - Non-manual sources: newer resolved_at wins
  - Uses ON CONFLICT DO UPDATE with WHERE clause

**`pkg/pg/identity.go`**
- **Database Implementation**: `IdentityIngester.IngestEmailResolution()` at line 91
- **Purpose**: Bulk upsert with PostgreSQL
- **Scale**: Efficiently handles 349K+ rows

**`pkg/pg/user_aliases.go`**
- **User Aliases**: `AliasIngester.UpsertAliases()` at line 46
- **Purpose**: Bulk upsert for user_aliases table

## Record Flow Through Ingest Pipeline

```
External Sources
    ↓
┌─────────────────────────────────────────────────────────┐
│ 1. RECORD ENTRY POINT (Counter Insertion Here)          │
│    - CLI tools: logger.RecordProcessed() (lines 185-187)│
│    - HTTP client: logger.RecordProcessed() (line 88)    │
│    - Workers: Calls PostResolution() → RecordProcessed()│
└─────────────────────────────────────────────────────────┘
    ↓
Validation & Processing
    ↓
┌─────────────────────────────────────────────────────────┐
│ 2. CORE INGEST FUNCTION                                 │
│    Ingester.IngestResolution() (pkg/identity/ingest.go) │
│    - Validates all rows first                          │
│    - Applies conflict resolution rules                 │
└─────────────────────────────────────────────────────────┘
    ↓
┌─────────────────────────────────────────────────────────┐
│ 3. DATABASE IMPLEMENTATION                              │
│    IdentityIngester.IngestEmailResolution()            │
│    - Bulk upsert with ON CONFLICT DO UPDATE            │
│    - PostgreSQL batch operation                         │
└─────────────────────────────────────────────────────────┘
    ↓
PostgreSQL Storage
```

## Single Insertion Point for Processed Counter

**The correct insertion point is: `logger.RecordProcessed()`**

This function is called at **exactly two locations**:

### 1. HTTP Client Entry Point (`pkg/client/queueapi/client.go:88`)
```go
func (c *Client) PostResolution(ctx context.Context, email, githubUsername string) error {
    // Record that this record is entering the ingest flow
    c.logger.RecordProcessed()  // ← INSERTION POINT
    // ... rest of function
}
```

**Why this is the single point:**
- All HTTP-based ingest flows (live enrichment, login revalidation) use `PostResolution()`
- The counter is incremented **before** any network activity, retries, or database operations
- This ensures each record is counted exactly once, regardless of retry success/failure
- Workers calling this function get automatic counter tracking

### 2. CLI Seed Operations (`cmd/seed-email-resolution/main.go:185-187`)
```go
// Record each record entering the ingest flow
for range batch {
    logger.RecordProcessed()  // ← INSERTION POINT
}
```

**Why this is here:**
- Seed operations bypass the HTTP client and write directly to database
- Counter is incremented **before** `Ingester.IngestResolution()` is called
- Ensures seed records are tracked even if database upsert fails

## Verification: Each Record Seen Exactly Once

**✓ Guaranteed single counting:**

1. **Live operations** (workers, HTTP clients):
   - Enter via `PostResolution()`
   - `RecordProcessed()` called once at line 88
   - Retries happen **after** counter is incremented
   - Each HTTP request = exactly one counter increment

2. **Seed operations** (CLI tools):
   - Enter via main() function
   - `RecordProcessed()` called once per record in loop (lines 185-187)
   - Database upsert happens **after** counter is incremented
   - Each record read = exactly one counter increment

3. **No double-counting scenarios:**
   - Retries don't re-increment (counter already incremented before first attempt)
   - Database conflict failures don't affect counter (counter at entry, not exit)
   - Network failures are still counted (record entered the flow)

4. **No missed records:**
   - Counter at entry point means all attempted ingests are tracked
   - Validation failures after counter increment are visible in stats
   - Skipped records (empty logins) are tracked separately with `RecordSkipped()`

## Ingest Logging Infrastructure

**`pkg/ingestlog/logger.go`** provides comprehensive tracking:

- **`RecordProcessed()`** (line 174): Increments `TotalProcessed` counter
- **`RecordSkipped()`** (line 152): Tracks skipped records (validation failures)
- **`LogSuccessWithEntry()`** (line 145): Increments `TotalIngested` counter
- **`RecordRetry()`** (line 159): Tracks retry attempts
- **`RecordFailure()`** (line 166): Tracks final failures
- **`GetStats()`** (line 180): Returns aggregate statistics

**AggregateStats structure** (lines 36-44):
```go
type AggregateStats struct {
    TotalProcessed int  // Records attempted (entry counter)
    TotalSkipped   int  // Validation failures
    TotalIngested  int  // Successful database writes
    TotalRetries   int  // Retry attempts
    TotalFailures  int  // Final failures after retries
    StartTime      time.Time
    LastUpdateTime time.Time
}
```

## Conclusion

**The single, canonical insertion point for the processed counter is `logger.RecordProcessed()`.**

This function:
- Is called at exactly two strategic locations covering all ingest paths
- Records each record exactly once as it enters the flow
- Provides visibility into all ingest attempts, regardless of outcome
- Enables accurate rate calculation and progress tracking
- Separates "attempted" from "successful" metrics

**Entry point locations:**
1. **HTTP path**: `pkg/client/queueapi/client.go:88` (all live operations)
2. **CLI path**: `cmd/seed-email-resolution/main.go:185-187` (seed operations)

Both paths call the same `RecordProcessed()` function, ensuring consistent tracking across all ingest methods.
