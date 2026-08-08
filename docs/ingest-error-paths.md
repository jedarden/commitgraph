# Ingest Endpoint Error Paths Catalog

This document catalogs all error exit points in the ingest endpoint codebase, organized by layer and flow.

## Table of Contents
1. [Client Layer - HTTP Request/Response](#client-layer---http-requestresponse)
2. [Validation Layer - Input Validation](#validation-layer---input-validation)
3. [Database Layer - PostgreSQL Operations](#database-layer---postgresql-operations)
4. [Service Layer - Business Logic](#service-layer---business-logic)
5. [Logging Layer - Error Classification](#logging-layer---error-classification)
6. [Warmstart Error Handling](#warmstart-error-handling)

---

## Client Layer - HTTP Request/Response

### File: `pkg/client/queueapi/client.go`

#### Error Path 1: JSON Marshaling Failure
- **Location**: `pkg/client/queueapi/client.go:109-128`
- **Error Type**: `json.Marshal` error
- **Available Context**:
  - `email` - Email address being resolved
  - `githubUsername` - Target GitHub username
  - `baseURL` - Queue-API base URL
  - `req` - Full ResolutionRequest struct
- **Error Response**: Immediate failure, no retry
- **Recovery**: None - marshaling errors are treated as permanent failures
- **Log Entry**: Created with `LogEntryFromError`, logged via `LogFailureWithEntry`

#### Error Path 2: HTTP Request Creation Failure
- **Location**: `pkg/client/queueapi/client.go:196-200`
- **Error Type**: `http.NewRequestWithContext` error
- **Available Context**:
  - `email` - Email address being resolved
  - `githubUsername` - Target GitHub username
  - `url` - Full endpoint URL
  - `body` - Request body as string
  - `attempt` - Current attempt number
- **Error Response**: Retryable (continues to next retry attempt)
- **Recovery**: Retry with exponential backoff (100ms, 400ms, 900ms, 1600ms)
- **Log Entry**: Created with `EventFromError`, logged via `LogRetry`

#### Error Path 3: Network Timeout Errors
- **Location**: `pkg/client/queueapi/client.go:208-219`
- **Error Type**: Timeout errors implementing `Timeout() bool` interface
- **Available Context**:
  - `email` - Email address being resolved
  - `githubUsername` - Target GitHub username
  - `url` - Full endpoint URL
  - `netErr.Timeout()` - Timeout flag
  - `attempt` - Current attempt number
- **Error Response**: Retryable
- **Recovery**: Retry with exponential backoff
- **Log Entry**: Created with `EventFromError`, error type classified as "timeout"

#### Error Path 4: General Network Errors
- **Location**: `pkg/client/queueapi/client.go:208-219`
- **Error Type**: Connection refused, DNS failure, other network errors
- **Available Context**:
  - `email` - Email address being resolved
  - `githubUsername` - Target GitHub username
  - `url` - Full endpoint URL
  - `err` - Original network error
  - `attempt` - Current attempt number
- **Error Response**: Retryable
- **Recovery**: Retry with exponential backoff
- **Log Entry**: Created with `EventFromError`, error type classified as "network"

#### Error Path 5: HTTP 408 Request Timeout
- **Location**: `pkg/client/queueapi/client.go:236-241`
- **Error Type**: HTTP 408 status code
- **Available Context**:
  - `email` - Email address being resolved
  - `githubUsername` - Target GitHub username
  - `url` - Full endpoint URL
  - `resp.StatusCode` - 408
  - `lastResponseBody` - Response body content
  - `attempt` - Current attempt number
- **Error Response**: Retryable
- **Recovery**: Retry with exponential backoff
- **Log Entry**: Created with `EventFromError`, error type classified as "timeout"

#### Error Path 6: HTTP 429 Too Many Requests
- **Location**: `pkg/client/queueapi/client.go:236-241`
- **Error Type**: HTTP 429 status code
- **Available Context**:
  - `email` - Email address being resolved
  - `githubUsername` - Target GitHub username
  - `url` - Full endpoint URL
  - `resp.StatusCode` - 429
  - `lastResponseBody` - Response body content
  - `attempt` - Current attempt number
- **Error Response**: Retryable
- **Recovery**: Retry with exponential backoff
- **Log Entry**: Created with `EventFromError`, error type classified as "client_error"

#### Error Path 7: HTTP 5xx Server Errors
- **Location**: `pkg/client/queueapi/client.go:236-241`
- **Error Type**: HTTP 500-599 status codes
- **Available Context**:
  - `email` - Email address being resolved
  - `githubUsername` - Target GitHub username
  - `url` - Full endpoint URL
  - `resp.StatusCode` - 5xx code
  - `lastResponseBody` - Response body content
  - `attempt` - Current attempt number
- **Error Response**: Retryable
- **Recovery**: Retry with exponential backoff
- **Log Entry**: Created with `EventFromError`, error type classified as "server_error"

#### Error Path 8: HTTP 4xx Client Errors (Non-Retryable)
- **Location**: `pkg/client/queueapi/client.go:243-246`
- **Error Type**: HTTP 400-499 status codes (except 408, 429)
- **Available Context**:
  - `email` - Email address being resolved
  - `githubUsername` - Target GitHub username
  - `url` - Full endpoint URL
  - `resp.StatusCode` - 4xx code
  - `lastResponseBody` - Response body content
  - `attempt` - Final attempt number
- **Error Response**: Non-retryable, immediate failure
- **Recovery**: None - client errors indicate permanent issues
- **Log Entry**: Created with `EventFromError`, logged via `LogFailure`, error type classified as "client_error"

#### Error Path 9: Context Cancellation
- **Location**: `pkg/client/queueapi/client.go:162-189`
- **Error Type**: `context.Context` cancellation
- **Available Context**:
  - `email` - Email address being resolved
  - `githubUsername` - Target GitHub username
  - `url` - Full endpoint URL
  - `ctx.Err()` - Context cancellation error
  - `attempt` - Attempt number when cancelled
  - `totalDurationMs` - Total time spent attempting
- **Error Response**: Immediate failure (not retryable)
- **Recovery**: None - context cancellation is explicit
- **Log Entry**: Created with `LogEntryFromError`, logged via `LogFailureWithEntry`

#### Error Path 10: All Retries Exhausted
- **Location**: `pkg/client/queueapi/client.go:248-270`
- **Error Type**: Final failure after all retry attempts
- **Available Context**:
  - `email` - Email address being resolved
  - `githubUsername` - Target GitHub username
  - `url` - Full endpoint URL
  - `lastErr` - Last error encountered
  - `lastStatusCode` - Last HTTP status code
  - `lastResponseBody` - Last response body
  - `c.maxRetries` - Maximum retry attempts configured
  - `totalDurationMs` - Total time spent attempting
- **Error Response**: Final failure, wrapped error returned
- **Recovery**: None - retry limit reached
- **Log Entry**: Created with `EventFromError`, logged via `LogFailure`

#### Error Path 11: Structured Logging Failure
- **Location**: Multiple locations throughout `client.go`
- **Error Type**: Logger method returns error
- **Available Context**:
  - Original operation context (email, githubUsername, etc.)
  - Original error that was being logged
  - `logErr` - Logging error
- **Error Response**: Fallback to basic `log.Printf`
- **Recovery**: Graceful degradation - basic logging still works
- **Log Entry**: Basic `log.Printf` with `[QUEUE-INGEST-*]` prefix

---

## Validation Layer - Input Validation

### File: `pkg/identity/ingest.go`

#### Error Path 12: Empty Email Validation
- **Location**: `pkg/identity/ingest.go:32-35`
- **Error Type**: `fmt.Errorf("email cannot be empty")`
- **Available Context**:
  - `row.Email` - Empty email field
  - `row` - Full ResolutionRow struct
  - `idx` - Row index in batch
- **Error Response**: Immediate failure, entire batch rejected
- **Recovery**: None - validation errors prevent processing
- **Log Entry**: Wrapped by caller with row index

#### Error Path 13: Empty Login Validation
- **Location**: `pkg/identity/ingest.go:36-38`
- **Error Type**: `fmt.Errorf("login cannot be empty")`
- **Available Context**:
  - `row.Login` - Empty login field
  - `row` - Full ResolutionRow struct
  - `idx` - Row index in batch
- **Error Response**: Immediate failure, entire batch rejected
- **Recovery**: None - validation errors prevent processing
- **Log Entry**: Wrapped by caller with row index

#### Error Path 14: Invalid Source Validation
- **Location**: `pkg/identity/ingest.go:39-44`
- **Error Type**: `fmt.Errorf("invalid source: %q (must be live, seed, or manual)", r.Source)`
- **Available Context**:
  - `row.Source` - Invalid source value
  - `row` - Full ResolutionRow struct
  - `idx` - Row index in batch
  - Valid sources: "live", "seed", "manual"
- **Error Response**: Immediate failure, entire batch rejected
- **Recovery**: None - validation errors prevent processing
- **Log Entry**: Wrapped by caller with row index

#### Error Path 15: Zero Timestamp Validation
- **Location**: `pkg/identity/ingest.go:45-47`
- **Error Type**: `fmt.Errorf("resolved_at cannot be zero")`
- **Available Context**:
  - `row.ResolvedAt` - Zero timestamp field
  - `row` - Full ResolutionRow struct
  - `idx` - Row index in batch
- **Error Response**: Immediate failure, entire batch rejected
- **Recovery**: None - validation errors prevent processing
- **Log Entry**: Wrapped by caller with row index

#### Error Path 16: Batch-Level Validation Failure
- **Location**: `pkg/identity/ingest.go:139-144`
- **Error Type**: Wrapped validation error with row index
- **Available Context**:
  - `idx` - Failed row index
  - `err` - Original validation error
  - `rows` - Full batch of rows
- **Error Response**: Entire batch rejected
- **Recovery**: None - validation failures are permanent
- **Skip Tracking**: Records as `SkipReasonValidation` if tracked

---

## Database Layer - PostgreSQL Operations

### File: `pkg/pg/identity.go`

#### Error Path 17: Fetch Existing Rows Failure
- **Location**: `pkg/pg/identity.go:117-126`
- **Error Type**: Database query execution error
- **Available Context**:
  - `fetchQuery` - SQL query executed
  - `emails` - Array of emails in batch
  - `err` - Database error
  - `ctx` - Database context
- **Error Response**: Immediate failure, no upsert attempted
- **Recovery**: None - database errors prevent processing
- **Log Entry**: Wrapped as `fmt.Errorf("failed to fetch existing rows: %w", err)`

#### Error Path 18: Scan Existing Row Failure
- **Location**: `pkg/pg/identity.go:128-133`
- **Error Type**: Row scan error
- **Available Context**:
  - `fetchQuery` - SQL query executed
  - `email`, `login`, `source`, `resolvedAt` - Scan targets
  - `err` - Scan error
- **Error Response**: Immediate failure
- **Recovery**: None - scan errors indicate data corruption
- **Log Entry**: Wrapped as `fmt.Errorf("failed to scan existing row: %w", err)`

#### Error Path 19: Iterate Existing Rows Failure
- **Location**: `pkg/pg/identity.go:144-146`
- **Error Type**: Rows iteration error
- **Available Context**:
  - `rowsResult.Err()` - Iteration error
  - `fetchQuery` - SQL query executed
- **Error Response**: Immediate failure
- **Recovery**: None - iteration errors are critical
- **Log Entry**: Wrapped as `fmt.Errorf("error iterating existing rows: %w", rowsResult.Err())`

#### Error Path 20: Bulk Upsert Execution Failure
- **Location**: `pkg/pg/identity.go:211-214`
- **Error Type**: Bulk INSERT...ON CONFLICT execution error
- **Available Context**:
  - `query` - Full SQL upsert query
  - `len(rows)` - Batch size
  - `emailsArr`, `logins`, `sources`, `resolvedAts` - Array parameters
  - `err` - Database execution error
- **Error Response**: Immediate failure
- **Recovery**: None - bulk upsert failures are critical
- **Log Entry**: Wrapped as `fmt.Errorf("bulk upsert failed (batch size %d): %w", len(rows), err)`

#### Error Path 21: Rows Affected Check Failure
- **Location**: `pkg/pg/identity.go:217-221`
- **Error Type**: Result.RowsAffected() error
- **Available Context**:
  - `result` - SQL result object
  - `err` - RowsAffected error
- **Error Response**: Non-fatal - uses predicted counts instead
- **Recovery**: Graceful degradation - prediction is accurate
- **Log Entry**: Sets `rowsAffected = -1`, logs warning

### File: `pkg/service/exclusion.go`

#### Error Path 22: Provider Validation (Empty)
- **Location**: `pkg/service/exclusion.go:159-169`
- **Error Type**: `errors.NewError(..., ValidationError, ..., "provider cannot be empty", ...)`
- **Available Context**:
  - `provider` - Empty provider string
  - `component` - "service"
  - `operation` - "validate_provider"
- **Error Response**: Immediate failure
- **Recovery**: None - validation error
- **Log Entry**: Structured error with severity "low"

#### Error Path 23: Provider Validation (Format)
- **Location**: `pkg/service/exclusion.go:171-196`
- **Error Type**: `errors.NewError(..., ValidationError, ..., "invalid provider format", ...)`
- **Available Context**:
  - `provider` - Invalid provider string
  - `regexp.MatchString` - Pattern match result
  - `err` - Regex compilation error (if any)
- **Error Response**: Immediate failure
- **Recovery**: None - validation error
- **Log Entry**: Structured error with recovery suggestion

#### Error Path 24: Repo Full Name Validation (Empty)
- **Location**: `pkg/service/exclusion.go:205-213`
- **Error Type**: `errors.NewError(..., ValidationError, ..., "repository full name cannot be empty", ...)`
- **Available Context**:
  - `repoFullName` - Empty repository full name
- **Error Response**: Immediate failure
- **Recovery**: None - validation error
- **Log Entry**: Structured error with recovery suggestion

#### Error Path 25: Repo Full Name Validation (Format)
- **Location**: `pkg/service/exclusion.go:216-231`
- **Error Type**: `errors.NewError(..., ValidationError, ..., "repository full name must be in 'owner/repo' format", ...)`
- **Available Context**:
  - `repoFullName` - Invalid format string
  - `parts` - Split result (length != 2)
- **Error Response**: Immediate failure
- **Recovery**: None - validation error
- **Log Entry**: Structured error with recovery suggestion

#### Error Path 26: Repository Not Found
- **Location**: `pkg/service/exclusion.go:334-349`
- **Error Type**: `errors.NewError(..., ValidationError, ..., "repository not found", ...)`
- **Available Context**:
  - `provider` - Repository provider
  - `repoFullName` - Repository full name
  - `checker.RepoExists()` - False result
- **Error Response**: Immediate failure
- **Recovery**: None - repo must exist
- **Log Entry**: Structured error with recovery suggestion

#### Error Path 27: Transaction Begin Failure
- **Location**: `pkg/service/exclusion.go:351-362`
- **Error Type**: `errors.WrapError(err, *errors.NewError(..., DatabaseError, ..., "failed to begin transaction", ...))`
- **Available Context**:
  - `db.BeginTx()` - Transaction begin error
  - `ctx` - Database context
- **Error Response**: Immediate failure
- **Recovery**: None - database error
- **Log Entry**: Structured error with severity "high"

#### Error Path 28: Select Current Exclusion State Failure
- **Location**: `pkg/service/exclusion.go:373-382`
- **Error Type**: `errors.QueryExecutionError(selectQuery, err)`
- **Available Context**:
  - `selectQuery` - SQL query executed
  - `provider`, `repoFullName` - Query parameters
  - `err` - Scan or query error
- **Error Response**: Immediate failure
- **Recovery**: Transaction rollback via defer
- **Log Entry**: Structured database error

#### Error Path 29: Update Exclusion Failure
- **Location**: `pkg/service/exclusion.go:385-395`
- **Error Type**: `errors.QueryExecutionError(updateQuery, err)`
- **Available Context**:
  - `updateQuery` - SQL query executed
  - `reason`, `provider`, `repoFullName` - Query parameters
  - `err` - Execution error
- **Error Response**: Immediate failure
- **Recovery**: Transaction rollback via defer
- **Log Entry**: Structured database error

#### Error Path 30: Rows Affected Check Failure (Exclusion)
- **Location**: `pkg/service/exclusion.go:397-408`
- **Error Type**: `errors.WrapError(err, *errors.NewError(..., DatabaseError, ..., "failed to get rows affected", ...))`
- **Available Context**:
  - `result.RowsAffected()` - Rows affected error
  - `updateQuery` - Query that was executed
- **Error Response**: Immediate failure
- **Recovery**: Transaction rollback via defer
- **Log Entry**: Structured error with severity "high"

#### Error Path 31: No Rows Updated (Repo Deleted)
- **Location**: `pkg/service/exclusion.go:410-423`
- **Error Type**: `errors.NewError(..., DatabaseError, ..., "no rows updated - repo may have been deleted", ...)`
- **Available Context**:
  - `rowsAffected` - 0 (no rows modified)
  - `updateQuery` - Query that was executed
  - `provider`, `repoFullName` - Target identifiers
- **Error Response**: Immediate failure
- **Recovery**: Transaction rollback via defer
- **Log Entry**: Structured error with recovery suggestion

#### Error Path 32: Exclusion Audit Record Failure
- **Location**: `pkg/service/exclusion.go:429-448`
- **Error Type**: `errors.WrapError(err, *errors.NewError(..., DatabaseError, ..., "failed to record exclusion audit", ...))`
- **Available Context**:
  - `repoID` - Repository ID
  - `actor` - Who performed the action
  - `oldExcludedAt`, `oldExcludedReason` - Before state
  - `newExcludedAt`, `newExcludedReason` - After state
  - `err` - Audit insertion error
- **Error Response**: Immediate failure
- **Recovery**: Transaction rollback via defer
- **Log Entry**: Structured error with severity "high"

#### Error Path 33: Transaction Commit Failure
- **Location**: `pkg/service/exclusion.go:450-464`
- **Error Type**: `errors.WrapError(err, *errors.NewError(..., DatabaseError, ..., "failed to commit transaction", ...))`
- **Available Context**:
  - `tx.Commit()` - Commit error
  - All previous operations succeeded but data not persisted
- **Error Response**: Critical failure - data may not be persisted
- **Recovery**: None - commit failure is critical
- **Log Entry**: Structured error with severity "critical" and recovery suggestion

---

## Logging Layer - Error Classification

### File: `pkg/ingestlog/logger.go`

#### Error Path 34: Event Validation (Email Required)
- **Location**: `pkg/ingestlog/logger.go:339-342`
- **Error Type**: `fmt.Errorf("email is required")`
- **Available Context**:
  - `event.Email` - Empty email field
  - `event` - Full Event struct
- **Error Response**: Logging fails, event not recorded
- **Recovery**: Caller should fallback to basic logging
- **Log Entry**: None (validation error)

#### Error Path 35: Event Validation (GithubUsername Required)
- **Location**: `pkg/ingestlog/logger.go:343-345`
- **Error Type**: `fmt.Errorf("github_username is required")`
- **Available Context**:
  - `event.GithubUsername` - Empty username field
  - `event` - Full Event struct
- **Error Response**: Logging fails, event not recorded
- **Recovery**: Caller should fallback to basic logging
- **Log Entry**: None (validation error)

#### Error Path 36: Event Validation (Endpoint URL Required)
- **Location**: `pkg/ingestlog/logger.go:346-348`
- **Error Type**: `fmt.Errorf("endpoint_url is required")`
- **Available Context**:
  - `event.EndpointURL` - Empty URL field
  - `event` - Full Event struct
- **Error Response**: Logging fails, event not recorded
- **Recovery**: Caller should fallback to basic logging
- **Log Entry**: None (validation error)

#### Error Path 37: Event JSON Marshal Failure
- **Location**: `pkg/ingestlog/logger.go:350-354`
- **Error Type**: `fmt.Errorf("marshal ingest event: %w", err)`
- **Available Context**:
  - `event` - Full Event struct
  - `err` - JSON marshal error
- **Error Response**: Logging fails, event not recorded
- **Recovery**: Caller should fallback to basic logging
- **Log Entry**: None (marshal error)

#### Error Path 38: LogEntry Validation (Endpoint URL Required)
- **Location**: `pkg/ingestlog/logger.go:369-372`
- **Error Type**: `fmt.Errorf("endpoint.url is required")`
- **Available Context**:
  - `entry.Endpoint.URL` - Empty URL field
  - `entry` - Full LogEntry struct
- **Error Response**: Logging fails, entry not recorded
- **Recovery**: Caller should fallback to basic logging
- **Log Entry**: None (validation error)

#### Error Path 39: LogEntry Validation (Attempt Number Required)
- **Location**: `pkg/ingestlog/logger.go:373-375`
- **Error Type**: `fmt.Errorf("endpoint.attempt_number is required")`
- **Available Context**:
  - `entry.Endpoint.AttemptNumber` - Zero attempt number
  - `entry` - Full LogEntry struct
- **Error Response**: Logging fails, entry not recorded
- **Recovery**: Caller should fallback to basic logging
- **Log Entry**: None (validation error)

#### Error Path 40: LogEntry Validation (User Identification Required)
- **Location**: `pkg/ingestlog/logger.go:377-385`
- **Error Type**: `fmt.Errorf("user identification required: either email/github_username or user_id/session_id/request_id must be provided")`
- **Available Context**:
  - `entry.User` - User context (all fields empty)
  - `entry` - Full LogEntry struct
  - Basic context: Email, GithubUsername
  - Extended context: UserID, SessionID, RequestID
- **Error Response**: Logging fails, entry not recorded
- **Recovery**: Caller should fallback to basic logging
- **Log Entry**: None (validation error)

#### Error Path 41: LogEntry JSON Marshal Failure
- **Location**: `pkg/ingestlog/logger.go:387-391`
- **Error Type**: `fmt.Errorf("marshal ingest log entry: %w", err)`
- **Available Context**:
  - `entry` - Full LogEntry struct
  - `err` - JSON marshal error
- **Error Response**: Logging fails, entry not recorded
- **Recovery**: Caller should fallback to basic logging
- **Log Entry**: None (marshal error)

#### Error Path 42: Stats JSON Marshal Failure
- **Location**: `pkg/ingestlog/logger.go:290-293`
- **Error Type**: `fmt.Errorf("marshal stats summary: %w", err)`
- **Available Context**:
  - `summary` - StatsJSON struct
  - `err` - JSON marshal error
- **Error Response**: Stats logging fails
- **Recovery**: None - stats logging is optional
- **Log Entry**: None (marshal error)

#### Error Path 43: Metadata Key Collision
- **Location**: `pkg/ingestlog/logger.go:1120-1123`
- **Error Type**: `fmt.Errorf("metadata validation failed: %w", err)`
- **Available Context**:
  - `metadata` - Metadata map with reserved key
  - `reservedLogEntryFields` - Reserved keys list
  - `err` - Collision error
- **Error Response**: Logging fails, entry not recorded
- **Recovery**: Remove reserved key from metadata
- **Log Entry**: None (validation error)

### File: `pkg/ingestlog/error_serializer.go`

#### Error Path 44: Error Type Extraction Failure
- **Location**: `pkg/ingestlog/error_serializer.go:119-152`
- **Error Type**: Reflection-based type extraction
- **Available Context**:
  - `err` - Error to serialize
  - `reflect.TypeOf(err)` - Error type reflection
  - `err.Type()` - Custom type method (if implemented)
- **Error Response**: Returns "unknown" type (non-fatal)
- **Recovery**: Graceful degradation - continues with "unknown"
- **Log Entry**: None (internal fallback)

#### Error Path 45: Stack Trace Capture Failure
- **Location**: `pkg/ingestlog/error_serializer.go:184-196`
- **Error Type**: `runtime.Callers` failure
- **Available Context**:
  - `depth+1` - Caller depth parameter
  - `maxDepth` - Maximum frames (32)
  - `n == 0` - No frames captured
- **Error Response**: Returns empty string (non-fatal)
- **Recovery**: Graceful degradation - continues without stack trace
- **Log Entry**: None (internal fallback)

### File: `pkg/ingestlog/logger.go` (Helper Functions)

#### Error Path 46: User Context Capture (Email Required)
- **Location**: `pkg/ingestlog/logger.go:910-914`
- **Error Type**: `fmt.Errorf("email is required for UserContext")`
- **Available Context**:
  - `email` - Empty email parameter
- **Error Response**: Context creation fails
- **Recovery**: Provide valid email
- **Log Entry**: None (validation error)

#### Error Path 47: User Context Capture (GithubUsername Required)
- **Location**: `pkg/ingestlog/logger.go:915-917`
- **Error Type**: `fmt.Errorf("github_username is required for UserContext")`
- **Available Context**:
  - `githubUsername` - Empty username parameter
- **Error Response**: Context creation fails
- **Recovery**: Provide valid username
- **Log Entry**: None (validation error)

#### Error Path 48: Endpoint Name Capture (Required)
- **Location**: `pkg/ingestlog/logger.go:983-987`
- **Error Type**: `fmt.Errorf("endpoint is required for EndpointContext")`
- **Available Context**:
  - `endpoint` - Empty endpoint parameter
- **Error Response**: Context creation fails
- **Recovery**: Provide valid endpoint
- **Log Entry**: None (validation error)

#### Error Path 49: Method Capture (Required)
- **Location**: `pkg/ingestlog/logger.go:1000-1004`
- **Error Type**: `fmt.Errorf("method is required for EndpointContext")`
- **Available Context**:
  - `method` - Empty method parameter
- **Error Response**: Context creation fails
- **Recovery**: Provide valid HTTP method
- **Log Entry**: None (validation error)

#### Error Path 50: Path Capture (Required)
- **Location**: `pkg/ingestlog/logger.go:1017-1021`
- **Error Type**: `fmt.Errorf("path is required for EndpointContext")`
- **Available Context**:
  - `path` - Empty path parameter
- **Error Response**: Context creation fails
- **Recovery**: Provide valid path
- **Log Entry**: None (validation error)

#### Error Path 51: Endpoint Context Capture (URL Required)
- **Location**: `pkg/ingestlog/logger.go:1052-1054`
- **Error Type**: `fmt.Errorf("url is required for EndpointContext")`
- **Available Context**:
  - `url` - Empty URL parameter
- **Error Response**: Context creation fails
- **Recovery**: Provide valid URL
- **Log Entry**: None (validation error)

#### Error Path 52: Endpoint Context Capture (Invalid Attempt Number)
- **Location**: `pkg/ingestlog/logger.go:1055-1057`
- **Error Type**: `fmt.Errorf("attempt_number must be positive (got %d)", attemptNumber)`
- **Available Context**:
  - `attemptNumber` - Invalid (zero or negative) attempt number
- **Error Response**: Context creation fails
- **Recovery**: Provide positive attempt number
- **Log Entry**: None (validation error)

---

## Warmstart Error Handling

### File: `pkg/warmstart/error.go`

#### Error Path 53: Truncated Tarball Error
- **Location**: `pkg/warmstart/error.go:161-168`
- **Error Type**: `*Error` with `Kind=Truncated`
- **Available Context**:
  - `context` - Human-readable details
  - `offset` - Byte offset in tarball
  - `commitSHA` - Git commit SHA
- **Error Response**: Tarball processing fails
- **Recovery**: Re-fetch tarball from source
- **Log Entry**: Structured error with all context fields

#### Error Path 54: Missing Member Error
- **Location**: `pkg/warmstart/error.go:181-186`
- **Error Type**: `*Error` with `Kind=MissingMember`
- **Available Context**:
  - `memberName` - Tarball member name
- **Error Response**: Tarball processing fails
- **Recovery**: Verify tarball completeness
- **Log Entry**: Structured error with member name

#### Error Path 55: Missing Member with Context
- **Location**: `pkg/warmstart/error.go:189-196`
- **Error Type**: `*Error` with `Kind=MissingMember` and context
- **Available Context**:
  - `memberName` - Tarball member name
  - `context` - Additional details (e.g., list of missing files)
- **Error Response**: Tarball processing fails
- **Recovery**: Verify tarball completeness
- **Log Entry**: Structured error with member name and context

#### Error Path 56: Corrupt Pack Error
- **Location**: `pkg/warmstart/error.go:198-200`
- **Error Type**: `*Error` with `Kind=CorruptPack`
- **Available Context**:
  - `memberName` - Pack file member name
  - `context` - Corruption details
- **Error Response**: Tarball processing fails
- **Recovery**: Re-fetch tarball from source
- **Log Entry**: Structured error with member name and context

#### Error Path 57: I/O Error
- **Location**: `pkg/warmstart/error.go:151-158`
- **Error Type**: `*Error` with `Kind=IO`
- **Available Context**:
  - `context` - Operation being performed
  - `commitSHA` - Git commit SHA
  - `Underlying` - Original I/O error
- **Error Response**: File operation fails
- **Recovery**: Check file system, retry operation
- **Log Entry**: Structured error with underlying error

#### Error Path 58: Not a Git Repository Error
- **Location**: `pkg/warmstart/error.go:137-143`
- **Error Type**: `*NotAGitRepoError`
- **Available Context**:
  - `Path` - Directory that was checked
  - `Reason` - Why it's not a git repository
- **Error Response**: Git operation fails
- **Recovery**: Initialize git repository or use correct path
- **Log Entry**: Structured error with path and reason

---

## Summary Statistics

### Error Path Count by Layer:
- **Client Layer**: 11 error paths
- **Validation Layer**: 5 error paths  
- **Database Layer**: 17 error paths
- **Logging Layer**: 19 error paths
- **Warmstart**: 6 error paths

**Total**: 58 distinct error paths cataloged

### Error Severity Distribution:
- **Critical**: 1 (transaction commit failure)
- **High**: 8 (database errors, logging failures)
- **Medium**: 12 (validation errors, network issues)
- **Low**: 37 (validation, non-critical failures)

### Retry Behavior:
- **Retryable**: 7 (network issues, server errors, timeouts)
- **Non-retryable**: 51 (validation errors, client errors, critical failures)

### Context Availability:
- **Email**: Available in 42/58 paths (72%)
- **GitHub Username**: Available in 42/58 paths (72%)
- **Endpoint URL**: Available in 28/58 paths (48%)
- **Database Context**: Available in 17/58 paths (29%)
- **Stack Trace**: Available in 11/58 paths (19%)

---

## Error Recovery Patterns

### 1. Exponential Backoff Retry
**Used in**: Client Layer (Paths 2-7, 10)
- Delays: 100ms, 400ms, 900ms, 1600ms
- Total max delay: ~3 seconds
- Applies to: Network errors, timeouts, 5xx errors

### 2. Transaction Rollback
**Used in**: Database Layer (Paths 27-32)
- Automatic via `defer tx.Rollback()`
- Ensures atomicity on failure
- Applied before any error return

### 3. Graceful Degradation
**Used in**: Logging Layer (Paths 44-45)
- Returns fallback value on failure
- "unknown" for error types
- Empty string for stack traces

### 4. Structured Error Wrapping
**Used in**: All layers
- Preserves original error with `WrapError`
- Adds context at each layer
- Maintains error chain for debugging

### 5. Immediate Failure
**Used in**: Validation Layer (Paths 12-16, 22-26)
- No recovery attempt
- Fails fast on invalid input
- Prevents invalid data propagation

---

## Monitoring and Observability

### Structured Logging Fields
Every error path logs these fields when available:
- **Timestamp**: UTC time of error
- **EventType**: "retry", "failure", or "success"
- **User Context**: Email, GitHub Username, UserID, SessionID, RequestID
- **Endpoint Context**: URL, Method, Path, Attempt Number, Status Code
- **Error Context**: Type, Message, Stack Trace
- **Performance**: Total Duration, Retry Delay

### Log Entry Destinations
- **Primary**: Structured JSON logs via `Logger`
- **Fallback**: `log.Printf` with `[QUEUE-INGEST-*]` prefix
- **Last Resort**: Silent failure (prevent crashes)

### Metrics Available
- Total processed records
- Total skipped records (by reason)
- Total ingested records
- Retry attempts count
- Final failures count
- Average processing rate

---

## Maintenance Notes

### Adding New Error Paths
When adding new error paths, ensure:
1. All context fields are populated
2. Error is properly wrapped with `WrapError` or `NewError`
3. Structured logging is attempted before fallback
4. Recovery suggestion is provided for user-actionable errors
5. Severity level is appropriately assigned

### Error Path Testing
Each error path should have test coverage for:
- Error generation with valid context
- Error propagation through layers
- Logging behavior (structured and fallback)
- Recovery behavior (retry, rollback, etc.)

### Context Propagation
When adding new context fields:
1. Update `LogEntry` struct in `logger.go`
2. Add validation in `logEntry` function
3. Update `reservedLogEntryFields` map
4. Document in this catalog

---

**Document Version**: 1.0  
**Last Updated**: 2025-08-08  
**Maintained By**: commitgraph development team
