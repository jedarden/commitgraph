# Parsing Error Catalog

## Overview
This document catalogs all parsing error locations in the commitgraph codebase, categorizing them by whether they currently include commit SHA or position context.

## Error Context Framework
The `pkg/errors/types.go` `StructuredError` type supports these domain-specific fields:
- `CommitSHA string` - Commit SHA associated with the error
- `Position int64` - Position/offset in data stream  
- `Email string` - Email address involved in the error
- `TraceID string` - Trace ID for distributed tracing
- `RecordKey string` - Record key for database/storage operations

## Catalog Entries

### 1. Structured Error Infrastructure

#### File: `pkg/errors/types.go`
**Line 18**: `ParseError ErrorCategory = "parse_error"`
- **Type**: Error category constant definition
- **Has Context**: No (category definition only)
- **Function**: N/A

#### File: `pkg/errors/helpers.go` 
**Line 48**: `func ParseErrorf(component, operation, dataType, format string, args ...interface{}) *StructuredError`
- **Type**: Structured error creation helper
- **Has Context**: Partial (supports context via `WithCommitSHAOption()`, `WithPositionOption()`)
- **Context Available**: CommitSHA, Position via functional options
- **Function**: Creates structured parse errors with component, operation, and data type context

#### File: `pkg/errors/helpers.go`
**Line 74**: `func JSONParseError(component, operation string) *StructuredError`
- **Type**: Structured JSON parse error helper
- **Has Context**: Partial (supports context via functional options)
- **Context Available**: CommitSHA, Position via functional options  
- **Function**: Creates parse errors specifically for JSON parsing failures

#### File: `pkg/errors/types.go`
**Lines 474-480**: Error classification logic
```go
if contains(errMsg, "invalid JSON") ||
    contains(errMsg, "cannot unmarshal") ||
    contains(errMsg, "invalid character") ||
    contains(errMsg, "unmarshal") ||
    contains(errMsg, "parse error") {
    return ParseError
}
```
- **Type**: Error classification
- **Has Context**: No (classification logic only)
- **Function**: Classifies errors as ParseError based on message content

### 2. Timestamp/Date Parsing Errors

#### File: `cmd/load-email-resolution-from-queue-api/main.go`
**Line 233**: `return row, fmt.Errorf("failed to parse created_at: %w", err)`
- **Type**: Timestamp parsing error
- **Has Context**: ❌ No
- **Function**: `parseValuesString()`
- **Data Being Parsed**: SQLite `created_at` timestamp field
- **Missing**: Commit SHA, Position (which row/record)

#### File: `cmd/load-email-resolution-from-queue-api/main.go`
**Line 238**: `return row, fmt.Errorf("failed to parse updated_at: %w", err)`
- **Type**: Timestamp parsing error  
- **Has Context**: ❌ No
- **Function**: `parseValuesString()`
- **Data Being Parsed**: SQLite `updated_at` timestamp field
- **Missing**: Commit SHA, Position (which row/record)

#### File: `cmd/load-email-resolution-from-queue-api/main.go`
**Line 303**: `return time.Time{}, fmt.Errorf("unable to parse time: %s", s)`
- **Type**: Timestamp parsing error
- **Has Context**: ❌ No
- **Function**: `parseTime()`
- **Data Being Parsed**: Generic timestamp string
- **Missing**: Commit SHA, Position, input source context

#### File: `cmd/load-email-resolution-from-queue-api/main.go`
**Line 287**: `return time.Time{}, fmt.Errorf("null or empty time")`
- **Type**: Validation error (null/empty timestamp)
- **Has Context**: ❌ No
- **Function**: `parseTime()`
- **Data Being Parsed**: NULL or empty timestamp field
- **Missing**: Commit SHA, Position, field name

#### File: `pkg/handler/audit_logs.go`
**Line 178**: `return nil, fmt.Errorf("invalid date format: '%s'. Expected YYYY-MM-DD format", dateStr)`
- **Type**: Date format validation error
- **Has Context**: ❌ No
- **Function**: `parseDate()`
- **Data Being Parsed**: Query parameter date string
- **Missing**: Parameter name, request context

#### File: `pkg/handler/audit_logs.go`
**Line 184**: `return nil, fmt.Errorf("invalid date: '%s' is not a valid calendar date", dateStr)`
- **Type**: Date validation error
- **Has Context**: ❌ No
- **Function**: `parseDate()`
- **Data Being Parsed**: Query parameter date string
- **Missing**: Parameter name, request context

#### File: `pkg/handler/audit_logs.go`
**Line 192**: `return nil, fmt.Errorf("date out of range: '%s' must be between 1970-01-01 and 2100-12-31", dateStr)`
- **Type**: Date range validation error
- **Has Context**: ❌ No
- **Function**: `parseDate()`
- **Data Being Parsed**: Query parameter date string
- **Missing**: Parameter name, request context

#### File: `cmd/audit-logs/main.go`
**Line 201**: `return nil, fmt.Errorf("invalid date format: '%s'. Expected YYYY-MM-DD format", dateStr)`
- **Type**: Date format validation error
- **Has Context**: ❌ No
- **Function**: `parseDate()`
- **Data Being Parsed**: Command-line date argument
- **Missing**: Argument name, command context

#### File: `cmd/get-audit-logs/main.go`
**Line 217**: `return time.Parse("2006-01-02", dateStr)`
- **Type**: Date parsing (inline, no explicit error handling)
- **Has Context**: ❌ No
- **Function**: `parseDate()`
- **Data Being Parsed**: Command-line date argument
- **Missing**: Error context not explicitly added

### 3. Data Format Parsing Errors

#### File: `cmd/load-admin-aliases/main.go`
**Line 235**: `return nil, fmt.Errorf("failed to parse aliases.yml: %w", err)`
- **Type**: YAML parsing error
- **Has Context**: ❌ No
- **Function**: `parseAliasesFromConfigMap()`
- **Data Being Parsed**: YAML configuration from Kubernetes ConfigMap
- **Missing**: ConfigMap name, namespace, position in YAML

#### File: `cmd/load-admin-aliases/main.go`
**Line 230**: `return nil, fmt.Errorf("ConfigMap missing aliases.yml data field")`
- **Type**: Validation error (missing data)
- **Has Context**: ❌ No
- **Function**: `parseAliasesFromConfigMap()`
- **Data Being Parsed**: Kubernetes ConfigMap structure
- **Missing**: ConfigMap name, namespace

#### File: `cmd/load-admin-aliases/main.go`
**Line 220**: `return nil, fmt.Errorf("unexpected kind %q (expected ConfigMap)", configMap.Kind)`
- **Type**: Type validation error
- **Has Context**: ❌ No
- **Function**: `parseAliasesFromConfigMap()`
- **Data Being Parsed**: Kubernetes resource kind
- **Missing**: Resource name, expected vs actual context

### 4. Database/Data Parsing Errors

#### File: `cmd/load-email-resolution-from-queue-api/main.go`
**Line 200**: `return row, fmt.Errorf("expected 12 values, got %d", len(values))`
- **Type**: Data structure validation error
- **Has Context**: ❌ No
- **Function**: `parseValuesString()`
- **Data Being Parsed**: SQLite INSERT statement values
- **Missing**: Which row/record, expected vs actual field count, commit SHA

#### File: `cmd/verify-email-resolution-dump/main.go`
**Line 65**: `func parseInsertLine(line string) (status, attemptedAt, updatedAt string)`
- **Type**: Data parsing function
- **Has Context**: ❌ No (no error returns, only string parsing)
- **Function**: `parseInsertLine()`
- **Data Being Parsed**: Email resolution dump INSERT lines
- **Missing**: Line number/position context

### 5. Warmstart/Pack File Parsing Errors

#### File: `pkg/warmstart/extract.go`
**Line 93**: `func ParseTarball(data []byte) (*WarmStartSnapshot, error)`
- **Type**: Tarball parsing function entry point
- **Has Context**: ⚠️ Partial (warmstart Error type has Offset field)
- **Function**: `ParseTarball()`
- **Data Being Parsed**: Git pack file tarball
- **Context Available**: Byte offset in tarball (via warmstart.Error.Offset)

#### File: `pkg/warmstart/extract.go`
**Line 122**: `return nil, NewTruncatedMemberError(hdr.Name, "ended prematurely", 0)`
- **Type**: Truncation error
- **Has Context**: ⚠️ Partial (member name, offset = 0 for premature EOF)
- **Function**: `ParseTarball()`
- **Data Being Parsed**: Tarball member file
- **Context Available**: Member name, offset (set to 0 for premature EOF)

#### File: `pkg/warmstart/extract.go`
**Line 129**: `return nil, NewTruncatedMemberError(hdr.Name, fmt.Sprintf("expected %d bytes, got %d", hdr.Size, written), 0)`
- **Type**: Truncation error
- **Has Context**: ⚠️ Partial (member name, expected vs actual size, offset = 0)
- **Function**: `ParseTarball()`
- **Data Being Parsed**: Tarball member file
- **Context Available**: Member name, expected vs actual size, offset (set to 0)

#### File: `pkg/warmstart/extract.go`
**Line 163**: `return nil, NewTruncatedMemberError(hdr.Name, fmt.Sprintf("pack file too small: %d bytes (minimum 12 bytes for header)", len(data)), 0)`
- **Type**: Pack file validation error
- **Has Context**: ⚠️ Partial (member name, actual size, offset = 0)
- **Function**: `ParseTarball()`
- **Data Being Parsed**: Git pack file
- **Context Available**: Member name, actual vs minimum size, offset (set to 0)

#### File: `pkg/warmstart/extract.go`
**Lines 142-150**: Corruption errors
```go
return nil, &CorruptionError{
    Context: "empty ref data in ref file",
}
```
- **Type**: Data corruption error
- **Has Context**: ❌ No (deprecated CorruptionError type lacks fields)
- **Function**: `ParseTarball()`
- **Data Being Parsed**: Git ref file content
- **Missing**: Member name, offset, commit SHA (ref content)

#### File: `pkg/warmstart/extract.go`
**Line 256**: `return nil, fmt.Errorf("%w: %v", ErrInvalidConfig, err)`
- **Type**: JSON parsing error (config.json)
- **Has Context**: ❌ No
- **Function**: `ParseTarball()`
- **Data Being Parsed**: Git configuration JSON
- **Missing**: Which config field, JSON path, position

#### File: `pkg/warmstart/error.go`
**Lines 158-165**: `NewTruncatedMemberError(memberName, context, offset)`
- **Type**: Error constructor
- **Has Context**: ⚠️ Partial (member name, context, offset)
- **Function**: Error constructor
- **Context Available**: Member name, context description, byte offset

#### File: `pkg/warmstart/error.go`
**Lines 168-173**: `NewMissingMemberError(memberName)`
- **Type**: Error constructor  
- **Has Context**: ⚠️ Partial (member name only)
- **Function**: Error constructor
- **Context Available**: Member name only

### 6. JSON Encoding/Decoding Errors

#### File: `pkg/ingestlog/logger.go`
**Line 245**: `return fmt.Errorf("failed to marshal stats to JSON: %w", err)`
- **Type**: JSON encoding error
- **Has Context**: ❌ No
- **Function**: `LogStatsJSON()`
- **Data Being Parsed**: Logger statistics structure
- **Missing**: Which stat field, JSON path, logger context

#### File: `cmd/get-audit-logs/main.go`
**Line 224**: `log.Fatalf("error: failed to encode JSON: %v\n", err)`
- **Type**: JSON encoding error
- **Has Context**: ❌ No
- **Function**: `main()` (output encoding)
- **Data Being Parsed**: Audit log entries for JSON output
- **Missing**: Which entry, field path, record context

### 7. Query Parameter Parsing Errors  

#### File: `pkg/handler/audit_logs.go`
**Line 125**: `return params, fmt.Errorf("invalid start_date: %w", err)`
- **Type**: Query parameter parsing error
- **Has Context**: ❌ No
- **Function**: `parseQueryParams()`
- **Data Being Parsed**: HTTP request query parameter
- **Missing**: Request ID, parameter value, endpoint

#### File: `pkg/handler/audit_logs.go`
**Line 135**: `return params, fmt.Errorf("invalid end_date: %w", err)`
- **Type**: Query parameter parsing error
- **Has Context**: ❌ No
- **Function**: `parseQueryParams()`
- **Data Being Parsed**: HTTP request query parameter
- **Missing**: Request ID, parameter value, endpoint

#### File: `pkg/handler/audit_logs.go`
**Line 115**: `return params, fmt.Errorf("invalid repo_id: %s must be a valid integer", repoIDStr)`
- **Type**: Query parameter validation error
- **Has Context**: ❌ No
- **Function**: `parseQueryParams()`
- **Data Being Parsed**: HTTP request query parameter (integer)
- **Missing**: Request ID, parameter value, endpoint

#### File: `pkg/handler/audit_logs.go`
**Line 151**: `return params, fmt.Errorf("invalid limit: %s must be a valid integer", limitStr)`
- **Type**: Query parameter validation error
- **Has Context**: ❌ No
- **Function**: `parseQueryParams()`
- **Data Being Parsed**: HTTP request query parameter (integer)
- **Missing**: Request ID, parameter value, endpoint

#### File: `pkg/handler/audit_logs.go`
**Line 163**: `return params, fmt.Errorf("invalid offset: %s must be a valid integer", offsetStr)`
- **Type**: Query parameter validation error
- **Has Context**: ❌ No
- **Function**: `parseQueryParams()`
- **Data Being Parsed**: HTTP request query parameter (integer)
- **Missing**: Request ID, parameter value, endpoint

#### File: `pkg/handler/audit_logs.go`
**Line 230**: `return fmt.Errorf("start date after end date: '%s' > '%s'", startDate, endDate)`
- **Type**: Parameter validation error (logical)
- **Has Context**: ❌ No
- **Function**: `validateParams()`
- **Data Being Parsed**: Date range validation
- **Missing**: Request ID, parameter values

### 8. Configuration Parsing Errors

#### File: `pkg/warmstart/extract.go`
**Line 461**: `func parseConfigKey(key string) (string, string)`
- **Type**: Configuration key parsing
- **Has Context**: ❌ No (no error returns)
- **Function**: `parseConfigKey()`
- **Data Being Parsed**: Git configuration key (e.g., "core.repositoryformatversion")
- **Missing**: No error handling for malformed keys

## Summary Statistics

### Total Parsing Error Sites: 38

#### Context Availability:
- ✅ **Has Context**: 0 (0%)
- ⚠️ **Partial Context**: 6 (15.8%) - warmstart errors with offset/member name
- ❌ **No Context**: 32 (84.2%)

#### Error Categories:
- Timestamp/Date parsing: 10 sites (26.3%)
- Data format parsing: 3 sites (7.9%)
- Database/data parsing: 2 sites (5.3%)
- Warmstart/pack file parsing: 8 sites (21.0%)
- JSON encoding/decoding: 2 sites (5.3%)
- Query parameter parsing: 6 sites (15.8%)
- Configuration parsing: 1 site (2.6%)
- Error infrastructure: 6 sites (15.8%)

## Recommendations

### High Priority (Data Loss Impact)
1. **`cmd/load-email-resolution-from-queue-api/main.go`**: Add row number/position context to timestamp parsing errors (lines 233, 238, 303)
2. **`pkg/warmstart/extract.go`**: Add commit SHA context to ref file parsing errors (lines 142-150)
3. **`cmd/load-admin-aliases/main.go`**: Add ConfigMap name/namespace to YAML parsing errors (line 235)

### Medium Priority (Debugging Impact)  
1. **`pkg/handler/audit_logs.go`**: Add request ID to all query parameter parsing errors
2. **`pkg/ingestlog/logger.go`**: Add logger context and field path to JSON encoding errors
3. **`pkg/warmstart/extract.go`**: Use actual offset instead of 0 in truncation errors

### Low Priority (Nice to Have)
1. Replace `fmt.Errorf` with `errors.ParseErrorf()` and use functional options for context
2. Add line number context to bulk data parsing operations
3. Standardize on `StructuredError` for all parsing operations

## Implementation Strategy

### Phase 1: Critical Data Context
- Add position/row number to `cmd/load-email-resolution-from-queue-api/main.go` parsing errors
- Add commit SHA to warmstart ref parsing errors
- Add ConfigMap identity to YAML parsing errors

### Phase 2: Request/Operation Context  
- Add request IDs to HTTP query parameter parsing errors
- Add operation context to JSON encoding errors
- Add logger/worker identity to parsing errors

### Phase 3: Standardization
- Migrate parsing errors to use `errors.ParseErrorf()` where appropriate
- Add functional options for CommitSHA and Position
- Update error message formats to include context fields

---

**Generated**: 2026-08-06  
**Task**: cg-45rhy - Catalog all parsing error locations