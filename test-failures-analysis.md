# Test Failures Analysis

**Analysis Date:** 2026-08-08  
**Repository:** commitgraph  
**Total Failing Tests:** 3

## Summary

This document analyzes all failing tests in the commitgraph test suite, categorizes them by failure type, and provides initial diagnoses for each failure.

---

## Failures by Category

### Category 1: Test Implementation Issues (2 failures)
Tests that have bugs in the test code itself rather than the production code.

- **TestRepoExcluder_GetExclusion_Success** (`pkg/pg/repo_test.go:304`)
- **TestRepoExcluder_GetExclusion_DatabaseError** (`pkg/pg/repo_test.go:323`)

### Category 2: Logging/Assertion Issues (1 failure)
Tests that fail due to incorrect assertions about log output or timing issues.

- **TestPostResolution_ContextCancellationLogging** (`pkg/client/queueapi/client_test.go:321`)

---

## Detailed Analysis

### 1. TestRepoExcluder_GetExclusion_Success

**File Location:** `pkg/pg/repo_test.go:304`  
**Test Name:** `TestRepoExcluder_GetExclusion_Success`

#### Error Message
```
repo_test.go:312: GetExclusion() expected error from mock scan, got nil
```

#### What the Test Checks
The test verifies that `GetExclusion()` properly handles errors from the database scan operation. It uses a mock database executor that is expected to return an error when scanning the row.

#### Test Code Analysis
```go
func TestRepoExcluder_GetExclusion_Success(t *testing.T) {
    db := &mockExecutor{}
    excluder := NewRepoExcluder(db)

    excludedAt, reason, err := excluder.GetExclusion(context.Background(), "github", "owner/repo")
    // This will fail because mockRow.Scan returns nil by default
    // We expect an error from the scan operation
    if err == nil {
        t.Fatal("GetExclusion() expected error from mock scan, got nil")
    }
    // ... more assertions
}
```

#### Production Code Being Tested
```go
// From pkg/pg/repo.go:146
func (r *RepoExcluder) GetExclusion(ctx context.Context, provider, repoFullName string) (*time.Time, string, error) {
    // ... validation code ...
    
    query := `SELECT excluded_at, excluded_reason FROM repos WHERE provider = $1 AND repo_full_name = $2`
    
    var excludedAt *time.Time
    var excludedReason *string
    
    err := r.db.QueryRowContext(ctx, query, provider, repoFullName).Scan(&excludedAt, &excludedReason)
    if err != nil {
        return nil, "", fmt.Errorf("get exclusion failed: %w", err)
    }
    
    // ... rest of function ...
    return excludedAt, reason, nil
}
```

#### Mock Implementation
```go
// From pkg/pg/test_mocks.go:258
type mockRow struct {
    shouldError bool
    data        interface{}
}

func (m *mockRow) Scan(dest ...interface{}) error {
    if m.shouldError {
        return &mockError{err: "test error"}
    }
    return nil  // ← Returns nil by default!
}
```

#### Root Cause
**Test Bug:** The test has contradictory code and comments. The comment states "This will fail because mockRow.Scan returns nil by default" but the test actually expects an error (`if err == nil { t.Fatal(...) }`). 

The mock `Scan()` method returns `nil` (no error) by default unless `shouldError` is set to `true`. Since the test creates a basic `&mockExecutor{}` without configuring `shouldError`, the scan succeeds with no error, causing the test to fail when it expects an error.

#### Diagnosis
**Type:** Test Bug  
**Severity:** Low  
**Fix Required:** The test needs to be fixed to either:
1. Set up the mock to actually return an error, OR
2. Change the test expectations to match the mock behavior (test success case instead of error case)

The test name suggests it's testing a "Success" case, which contradicts the error expectation.

---

### 2. TestRepoExcluder_GetExclusion_DatabaseError

**File Location:** `pkg/pg/repo_test.go:323`  
**Test Name:** `TestRepoExcluder_GetExclusion_DatabaseError`

#### Error Message
```
repo_test.go:329: GetExclusion() expected error, got nil
```

#### What the Test Checks
The test verifies that `GetExclusion()` properly handles database errors. It creates a mock executor with `shouldError: true` to simulate a database failure.

#### Test Code Analysis
```go
func TestRepoExcluder_GetExclusion_DatabaseError(t *testing.T) {
    db := &mockExecutor{shouldError: true}  // ← Error flag set
    excluder := NewRepoExcluder(db)

    excludedAt, reason, err := excluder.GetExclusion(context.Background(), "github", "owner/repo")
    if err == nil {
        t.Fatal("GetExclusion() expected error, got nil")
    }
    // ... more assertions ...
}
```

#### Mock Implementation Analysis
```go
// From pkg/pg/test_mocks.go:58
type mockExecutor struct {
    lastQuery    string
    lastArgs     []interface{}
    rowsAffected int64
    shouldError  bool  // ← This flag
}

func (m *mockExecutor) QueryRowContext(ctx context.Context, query string, args ...interface{}) Row {
    m.lastQuery = query
    m.lastArgs = args
    // Return nil - caller must handle this
    return &mockRow{}  // ← Note: shouldError is NOT passed to mockRow!
}
```

#### Root Cause
**Test Bug:** The `shouldError` flag on `mockExecutor` only affects the `ExecContext` method (see line 68-71 in test_mocks.go), but `GetExclusion()` uses `QueryRowContext()`. 

The `QueryRowContext()` method creates a new `mockRow{}` without passing the `shouldError` flag, so the returned `mockRow` always has `shouldError = false`, causing `Scan()` to succeed with no error.

#### Diagnosis
**Type:** Test Bug  
**Severity:** Low  
**Fix Required:** The `QueryRowContext()` mock implementation needs to be updated to pass the `shouldError` flag to the created `mockRow`, OR the test needs to use a different approach to simulate database errors.

---

### 3. TestPostResolution_ContextCancellationLogging

**File Location:** `pkg/client/queueapi/client_test.go:321`  
**Test Name:** `TestPostResolution_ContextCancellationLogging`

#### Error Message
```
client_test.go:366: expected to find a failure log entry for context cancellation
```

#### What the Test Checks
The test verifies that when a context is cancelled during retry attempts, a failure log entry is created with the correct fields including "context canceled" message.

#### Test Code Analysis
```go
func TestPostResolution_ContextCancellationLogging(t *testing.T) {
    var loggedEntries []string
    
    // Custom logger to capture log output
    customOutput := logWriterFunc(func(p []byte) (n int, err error) {
        loggedEntries = append(loggedEntries, string(p))
        return len(p), nil
    })
    
    customLogger := ingestlog.NewLoggerWithOutput(log.New(customOutput, "[INGEST-LOG] ", log.LstdFlags|log.Lmicroseconds|log.LUTC))
    
    // Setup: maxRetries=2, context cancels after 50ms
    transport := &mockFlakyTransport{failCount: 10, delay: 20 * time.Millisecond}
    client := &Client{
        baseURL:    "http://test-api",
        httpClient: &http.Client{Transport: transport, Timeout: 100 * time.Millisecond},
        authToken:  "",
        maxRetries: 2,
    }
    client.SetLogger(customLogger)
    
    ctx, cancel := context.WithCancel(context.Background())
    
    // Cancel during backoff to trigger context cancellation logging
    go func() {
        time.Sleep(50 * time.Millisecond)
        cancel()
    }()
    
    err := client.PostResolution(ctx, "user@example.com", "githubuser")
    
    // Should fail due to context cancellation
    if err == nil {
        t.Fatal("expected error on context cancellation, got nil")
    }
    
    // Find the failure log entry
    var failureEntry string
    for _, entry := range loggedEntries {
        if strings.Contains(entry, `"event_type":"failure"`) && strings.Contains(entry, "context canceled") {
            failureEntry = entry
            break
        }
    }
    
    if failureEntry == "" {
        t.Fatal("expected to find a failure log entry for context cancellation")  // ← FAILS HERE
    }
    // ... more field validation ...
}
```

#### Production Code Flow
From `pkg/client/queueapi/client.go:136-186`, the retry loop:

1. **Attempt 0** (t=0ms): HTTP request takes 20ms, fails with 500
2. **Check context before backoff sleep** (t≈20ms): Context NOT cancelled yet (scheduled for 50ms)
3. **Sleep 100ms** for backoff
4. **Context cancelled** at t=50ms during sleep
5. **Wake up** at t≈120ms, **Attempt 1** starts, takes 20ms, fails with 500
6. **Check context before backoff** (t≈140ms): Context WAS cancelled at 50ms → enters cancellation handling
7. **Log context cancellation** and return error

The code at lines 164-186 handles context cancellation:
```go
select {
case <-ctx.Done():
    // Log context cancellation with full context before returning
    totalDurationMs := time.Since(startTime).Milliseconds()
    entry := ingestlog.LogEntryFromError(
        email,
        githubUsername,
        fmt.Sprintf("%s/email-resolution/resolve", c.baseURL),
        ctx.Err(),
        0, // statusCode
        "", // responseBody
        attempt,
        c.maxRetries,
        0, // retryDelayMs
        totalDurationMs,
    )
    
    if logErr := c.log().LogFailureWithEntry(&entry); logErr != nil {
        // Fallback logging
    }
    return fmt.Errorf("context cancelled during retry backoff: %w", ctx.Err())
default:
}
```

#### Root Cause Analysis
The test is looking for a log entry with `"event_type":"failure"` and `"context canceled"`, but based on the production code flow:

1. The context cancellation DOES get logged (via `LogFailureWithEntry`)
2. However, the log format or timing may not match what the test expects

Looking at the passing test `TestPostResolution_ContextCancellation` (which runs immediately before), it produces this log output:
```
[INGEST-LOG] ... {"event_type":"failure",...,"message":"context canceled",...}
```

This suggests the logging code works correctly. The issue is likely:

**Possible Timing Issue:** The `TestPostResolution_ContextCancellationLogging` test has `maxRetries: 2` with a 50ms cancellation delay, while the passing test has `maxRetries: DefaultMaxRetries` (4) with an 80ms cancellation delay. With only 2 retries, the context might be cancelled at a different point in the retry loop that doesn't trigger the expected logging path.

**Alternative Theory:** The test's custom logger might not be properly capturing the log entries due to concurrency issues or logger setup problems.

#### Diagnosis
**Type:** Test Implementation Issue (likely)  
**Severity:** Medium  
**Fix Required:** Investigate why the failure log entry is not being captured:
1. Add debug output to print all captured log entries
2. Verify the custom logger is actually being used
3. Check if there's a race condition in log capture
4. Consider increasing the cancellation delay or retry count to match the passing test pattern

---

## Recommendations

### Immediate Actions
1. **Fix TestRepoExcluder_GetExclusion_Success**: Either rename the test to reflect error testing and fix the mock, or change it to test the actual success case
2. **Fix TestRepoExcluder_GetExclusion_DatabaseError**: Update the mock implementation or the test setup to properly simulate database query errors
3. **Investigate TestPostResolution_ContextCancellationLogging**: Add debug logging to understand why the failure log entry is not being captured

### Mock Infrastructure Improvements
The mock implementations in `pkg/pg/test_mocks.go` need enhancement:
- `mockRow` should accept an error state during construction
- `mockExecutor.QueryRowContext()` should propagate the `shouldError` flag
- Consider adding a dedicated `mockRowWithError` type for cleaner error testing

### Test Design Patterns
- Tests should use descriptive names that match their expectations (e.g., `TestGetExclusion_ErrorHandling` vs `TestGetExclusion_Success`)
- Mock setup should be explicit about error states
- Tests capturing log output should include assertions about the number of entries captured to aid debugging

---

## Conclusion

All three failing tests appear to have **test implementation bugs** rather than production code bugs. The issues stem from:
1. Incomplete mock implementations that don't properly simulate error conditions
2. Test expectations that don't match the mock behavior
3. Potential timing or concurrency issues in log capture

Fixing these tests will require:
- Enhancing the mock infrastructure in `pkg/pg/test_mocks.go`
- Clarifying test intent through better naming and structure
- Investigating the log capture mechanism in the context cancellation test

**No production code changes appear to be needed** based on this analysis.
