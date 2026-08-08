# Parse Entry Point Function Signatures

## Overview
This document contains the complete function signatures for all 26 parse entry point functions identified in the catalog.

**Source**: Based on catalog from bead cg-3dpfa  
**Created**: 2026-08-08  
**Bead**: cg-f5wc6

---

## Summary by Package

| Package | Function Count |
|---------|---------------|
| cmd/ | 8 |
| pkg/handler/ | 3 |
| pkg/ingestlog/ | 8 |
| pkg/pg/ | 3 |
| pkg/warmstart/ | 1 |
| pkg/errors/ | 2 |

---

## cmd/ Directory (8 functions)

### cmd/audit-logs/main.go

**1. parseDate**
```go
func parseDate(dateStr string) (*time.Time, error)
```
- **File**: `cmd/audit-logs/main.go:211`
- **Parameters**: `dateStr string` - Date string in YYYY-MM-DD format
- **Returns**: `(*time.Time, error)` - Parsed time pointer or error

**2. handleQuery**
```go
func handleQuery(repoID int64, startDate, endDate, actor, eventType string, limit, offset int)
```
- **File**: `cmd/audit-logs/main.go:84`
- **Parameters**: 
  - `repoID int64` - Repository ID
  - `startDate string` - Start date string
  - `endDate string` - End date string
  - `actor string` - Actor filter
  - `eventType string` - Event type filter
  - `limit int` - Result limit
  - `offset int` - Pagination offset
- **Returns**: None (void function)

---

### cmd/load-admin-aliases/main.go

**3. parseAliasesFromConfigMap**
```go
func parseAliasesFromConfigMap(configMap *ConfigMap) ([]AliasEntry, error)
```
- **File**: `cmd/load-admin-aliases/main.go:227`
- **Parameters**: `configMap *ConfigMap` - ConfigMap struct pointer
- **Returns**: `([]AliasEntry, error)` - Slice of alias entries or error

---

### cmd/verify-email-resolution-dump/main.go

**4. parseInsertLine**
```go
func parseInsertLine(line string) (status, attemptedAt, updatedAt string)
```
- **File**: `cmd/verify-email-resolution-dump/main.go:65`
- **Parameters**: `line string` - SQL INSERT line
- **Returns**: `(status, attemptedAt, updatedAt string)` - Three extracted string values

---

### cmd/load-email-resolution-from-queue-api/main.go

**5. parseDump**
```go
func parseDump(dump string) ([]QueueAPIRow, error)
```
- **File**: `cmd/load-email-resolution-from-queue-api/main.go:163`
- **Parameters**: `dump string` - SQLite dump content
- **Returns**: `([]QueueAPIRow, error)` - Slice of queue API rows or error

**6. parseValuesString**
```go
func parseValuesString(valuesStr string) (QueueAPIRow, error)
```
- **File**: `cmd/load-email-resolution-from-queue-api/main.go:197`
- **Parameters**: `valuesStr string` - Comma-separated values from INSERT
- **Returns**: `(QueueAPIRow, error)` - Parsed row struct or error

**7. parseTime**
```go
func parseTime(s string) (time.Time, error)
```
- **File**: `cmd/load-email-resolution-from-queue-api/main.go:290`
- **Parameters**: `s string` - Time string to parse
- **Returns**: `(time.Time, error)` - Parsed time or error

**8. parseTimePtr**
```go
func parseTimePtr(s string) *time.Time
```
- **File**: `cmd/load-email-resolution-from-queue-api/main.go:312`
- **Parameters**: `s string` - Time string to parse
- **Returns**: `*time.Time` - Pointer to parsed time (nil for NULL/empty)

---

### cmd/get-audit-logs/main.go

**9. parseDate** (duplicate signature, different file)
```go
func parseDate(dateStr string) (time.Time, error)
```
- **File**: `cmd/get-audit-logs/main.go:219`
- **Parameters**: `dateStr string` - Date string
- **Returns**: `(time.Time, error)` - Parsed time (not a pointer) or error
- **Note**: Different return type from cmd/audit-logs version

---

## pkg/handler/ Directory (3 functions)

### pkg/handler/audit_logs.go

**10. parseQueryParams**
```go
func parseQueryParams(r *http.Request) (queryParams, error)
```
- **File**: `pkg/handler/audit_logs.go:107`
- **Parameters**: `r *http.Request` - HTTP request
- **Returns**: `(queryParams, error)` - Parsed query parameters struct or error
- **Where used**: HTTP handler entry point for audit logs API

**11. parseDate** (duplicate signature, different file)
```go
func parseDate(dateStr string) (*time.Time, error)
```
- **File**: `pkg/handler/audit_logs.go:174`
- **Parameters**: `dateStr string` - Date string in YYYY-MM-DD format
- **Returns**: `(*time.Time, error)` - Parsed time pointer or error

**12. handleGetAuditLogs**
```go
func (h *AuditLogsHandler) handleGetAuditLogs(w http.ResponseWriter, r *http.Request)
```
- **File**: `pkg/handler/audit_logs.go:35`
- **Parameters**: 
  - `w http.ResponseWriter` - HTTP response writer
  - `r *http.Request` - HTTP request
- **Returns**: None (HTTP handler method)
- **Where used**: HTTP route handler for GET /api/audit-logs

---

## pkg/ingestlog/ Directory (8 functions)

### pkg/ingestlog/logger.go

**13. CaptureUserContext**
```go
func CaptureUserContext(email, githubUsername string) (UserContext, error)
```
- **File**: `pkg/ingestlog/logger.go:910`
- **Parameters**: 
  - `email string` - User's email address
  - `githubUsername string` - GitHub username
- **Returns**: `(UserContext, error)` - User context struct or error

**14. CaptureUserID**
```go
func CaptureUserID(userID string) string
```
- **File**: `pkg/ingestlog/logger.go:934`
- **Parameters**: `userID string` - User ID (optional)
- **Returns**: `string` - User ID string (empty if not provided)

**15. CaptureSessionID**
```go
func CaptureSessionID(sessionID string) string
```
- **File**: `pkg/ingestlog/logger.go:950`
- **Parameters**: `sessionID string` - Session ID (optional)
- **Returns**: `string` - Session ID string (empty if not provided)

**16. CaptureRequestID**
```go
func CaptureRequestID(requestID string) string
```
- **File**: `pkg/ingestlog/logger.go:966`
- **Parameters**: `requestID string` - Request ID (optional)
- **Returns**: `string` - Request ID string (empty if not provided)

**17. CaptureEndpointName**
```go
func CaptureEndpointName(endpoint string) (string, error)
```
- **File**: `pkg/ingestlog/logger.go:983`
- **Parameters**: `endpoint string` - Endpoint identifier
- **Returns**: `(string, error)` - Endpoint string or error

**18. CaptureMethod**
```go
func CaptureMethod(method string) (string, error)
```
- **File**: `pkg/ingestlog/logger.go:1000`
- **Parameters**: `method string` - HTTP method (GET, POST, etc.)
- **Returns**: `(string, error)` - Method string or error

**19. CapturePath**
```go
func CapturePath(path string) (string, error)
```
- **File**: `pkg/ingestlog/logger.go:1017`
- **Parameters**: `path string` - Request path
- **Returns**: `(string, error)` - Path string or error

**20. CaptureEndpointContext**
```go
func CaptureEndpointContext(endpoint, method, path, url string, attemptNumber int, statusCode int, responseBody string) (EndpointContext, error)
```
- **File**: `pkg/ingestlog/logger.go:1041`
- **Parameters**: 
  - `endpoint string` - Endpoint identifier
  - `method string` - HTTP method
  - `path string` - Request path
  - `url string` - Full HTTP endpoint URL
  - `attemptNumber int` - Current retry attempt (1-based)
  - `statusCode int` - HTTP status code (0 if not applicable)
  - `responseBody string` - Response body content
- **Returns**: `(EndpointContext, error)` - Endpoint context struct or error

---

## pkg/pg/ Directory (3 functions)

### pkg/pg/identity.go

**21. IngestEmailResolution** (method on IdentityIngester)
```go
func (i *IdentityIngester) IngestEmailResolution(ctx context.Context, rows []identity.ResolutionRow) (*identity.IngestResult, error)
```
- **File**: `pkg/pg/identity.go:94`
- **Parameters**: 
  - `ctx context.Context` - Context
  - `rows []identity.ResolutionRow` - Slice of resolution rows
- **Returns**: `(*identity.IngestResult, error)` - Ingest result pointer or error
- **Where used**: Database ingest entry point for email resolution

---

### pkg/pg/user_aliases.go

**22. UpsertAliases** (method on AliasIngester)
```go
func (a *AliasIngester) UpsertAliases(ctx context.Context, rows []AliasRow) error
```
- **File**: `pkg/pg/user_aliases.go:46`
- **Parameters**: 
  - `ctx context.Context` - Context
  - `rows []AliasRow` - Slice of alias rows
- **Returns**: `error` - Error if upsert fails
- **Where used**: Database ingest entry point for user aliases

---

### pkg/identity/ingest.go

**23. IngestResolution** (method on Ingester)
```go
func (i *Ingester) IngestResolution(ctx context.Context, rows []ResolutionRow) error
```
- **File**: `pkg/identity/ingest.go:140`
- **Parameters**: 
  - `ctx context.Context` - Context
  - `rows []ResolutionRow` - Slice of resolution rows
- **Returns**: `error` - Error if ingest fails
- **Where used**: High-level ingest entry point with validation

---

## pkg/warmstart/ Directory (1 function)

### pkg/warmstart/extract.go

**24. parseConfigKey**
```go
func parseConfigKey(key string) (string, string)
```
- **File**: `pkg/warmstart/extract.go:465`
- **Parameters**: `key string` - Git config key (e.g., "core.repositoryformatversion")
- **Returns**: `(string, string)` - Section and variable (e.g., "[core]", "repositoryformatversion")
- **Note**: This function does not return an error - returns empty strings on invalid input

---

## pkg/errors/ Directory (2 functions)

### pkg/errors/helpers.go

**25. JSONParseError**
```go
func JSONParseError(component, operation string) *StructuredError
```
- **File**: `pkg/errors/helpers.go:87`
- **Parameters**: 
  - `component string` - Component identifier
  - `operation string` - Operation identifier
- **Returns**: `*StructuredError` - Structured error pointer
- **Note**: Deprecated - use JSONParseErrorWithCommit instead

**26. JSONParseErrorWithCommit**
```go
func JSONParseErrorWithCommit(component, operation, commitSHA string) *StructuredError
```
- **File**: `pkg/errors/helpers.go:81`
- **Parameters**: 
  - `component string` - Component identifier
  - `operation string` - Operation identifier
  - `commitSHA string` - Commit SHA for context
- **Returns**: `*StructuredError` - Structured error pointer with commit context

---

## Signature Analysis

### Return Type Patterns

**Error Returning Functions (13 functions)**
- Most parse functions return `(T, error)` for error handling
- Standard Go pattern for parsing operations

**Void Functions (2 functions)**
- `handleQuery`, `handleGetAuditLogs` - side-effect only operations

**Pointer Returns (7 functions)**
- Functions returning pointers to allow nil/empty states
- Typically for optional or validation-failed cases

**String Only Returns (4 functions)**
- Capture functions that don't validate (CaptureUserID, CaptureSessionID, CaptureRequestID)
- parseInsertLine returns multiple strings as a tuple pattern

### Parameter Patterns

**Validation-heavy (8 Capture functions)**
- All take string parameters
- Return validated structs or errors
- Used for structured logging context

**HTTP Request Handling (2 functions)**
- parseQueryParams, handleGetAuditLogs
- Entry points from HTTP layer

**Database Operations (3 functions)**
- IngestEmailResolution, UpsertAliases, IngestResolution
- Take context and batch rows
- High-volume data ingestion entry points

**Time Parsing (3 variations)**
- Three different `parseDate` signatures across files
- Two return `*time.Time`, one returns `time.Time`
- Different validation and error handling approaches

### Entry Point Types

**CLI Entry Points (7 functions in cmd/)**
- Command-line argument parsing
- File input parsing
- Direct user input handling

**HTTP Entry Points (2 functions in pkg/handler/)**
- Request parameter parsing
- Request handling

**Ingest Entry Points (3 functions in pkg/pg/, pkg/identity/)**
- Database write operations
- Batch data processing

**Logging Entry Points (8 functions in pkg/ingestlog/)**
- Context capture and validation
- Structured logging preparation

**Error Creation (2 functions in pkg/errors/)**
- Structured error construction
- Error context management

---

## Completeness Check

✅ **All 26 functions from the catalog have complete signatures documented**

- ✅ All parameters with types
- ✅ All return types documented
- ✅ File locations recorded
- ✅ Structured format for next step
- ✅ Analysis of patterns and entry point types

---

## Next Steps

This signature extraction enables:

1. **Error Analysis** - Map which entry points can produce which error types
2. **Validation Analysis** - Identify all validation patterns across entry points
3. **Error Flow Tracing** - Track error propagation from entry points to error constructors
4. **Comprehensive Catalog** - Link signatures to parsing error catalog

The signatures are now ready for integration with error path analysis and validation documentation.
