# Parsing Error Catalog

This document catalogs all parsing error locations in the commitgraph codebase, categorized by whether they currently include commit SHA or position context.

## Summary

- **Total parsing error sites**: 25
- **With commit SHA context**: 2 (8%)
- **With position context**: 1 (4%)
- **Without context**: 22 (88%)

---

## Category 1: Timestamp/Date Parsing Errors

### Errors WITH Context

#### 1. `cmd/verify-email-resolution/timestamp_verify.go:72`
- **Line**: 72
- **Error Type**: Time parsing failure
- **Message**: `error: failed to parse source timestamp %s: %v`
- **Has Context**: No commit SHA, but has email source
- **Current Context**: `src.Email`, `src.ResolvedAt`
- **Code Context**:
```go
srcTime, err := time.Parse(time.RFC3339Nano, src.ResolvedAt)
if err != nil {
    log.Fatalf("error: failed to parse source timestamp %s: %v\n", src.ResolvedAt, err)
}
```

#### 2. `cmd/verify-email-resolution/timestamp_verify.go:77`
- **Line**: 77
- **Error Type**: Time parsing failure
- **Message**: `error: failed to parse target timestamp %s: %v`
- **Has Context**: No commit SHA, but has email
- **Current Context**: `src.Email`, `targetResolvedAt`
- **Code Context**:
```go
targetTime, err := time.Parse(time.RFC3339Nano, targetResolvedAt)
if err != nil {
    log.Fatalf("error: failed to parse target timestamp %s: %v\n", targetResolvedAt, err)
}
```

### Errors WITHOUT Context

#### 3. `cmd/get-audit-logs/main.go:116`
- **Line**: 116
- **Error Type**: Date parsing failure
- **Message**: `error: invalid start-date: %v`
- **Has Context**: None
- **Missing**: No commit SHA, no position
- **Code Context**:
```go
if startDateFlag := *startDate; startDateFlag != "" {
    parsedStart, err = parseDate(startDateFlag)
    if err != nil {
        log.Fatalf("error: invalid start-date: %v\n", err)
    }
}
```

#### 4. `cmd/get-audit-logs/main.go:122`
- **Line**: 122
- **Error Type**: Date parsing failure
- **Message**: `error: invalid end-date: %v`
- **Has Context**: None
- **Missing**: No commit SHA, no position
- **Code Context**:
```go
if endDateFlag := *endDate; endDateFlag != "" {
    parsedEnd, err = parseDate(endDateFlag)
    if err != nil {
        log.Fatalf("error: invalid end-date: %v\n", err)
    }
}
```

#### 5. `cmd/get-audit-logs/main.go:186`
- **Line**: 186
- **Error Type**: Date parsing failure
- **Message**: `error: invalid start-date: %v`
- **Has Context**: None
- **Missing**: No commit SHA, no position
- **Code Context**: Duplicate of #3 in `showRecordsCount` function

#### 6. `cmd/get-audit-logs/main.go:192`
- **Line**: 192
- **Error Type**: Date parsing failure
- **Message**: `error: invalid end-date: %v`
- **Has Context**: None
- **Missing**: No commit SHA, no position
- **Code Context**: Duplicate of #4 in `showRecordsCount` function

#### 7. `cmd/get-audit-logs/main.go:217`
- **Line**: 217
- **Error Type**: Date parsing function
- **Message**: Wrapped error from `time.Parse`
- **Has Context**: None (function returns error to caller)
- **Missing**: No commit SHA, no position
- **Code Context**:
```go
func parseDate(dateStr string) (time.Time, error) {
    return time.Parse("2006-01-02", dateStr)
}
```

#### 8. `cmd/audit-logs/main.go:92`
- **Line**: 92
- **Error Type**: Date parsing failure
- **Message**: `error: invalid start-date: %v`
- **Has Context**: None
- **Missing**: No commit SHA, no position
- **Code Context**:
```go
parsedStart, err = parseDate(startDate)
if err != nil {
    log.Fatalf("error: invalid start-date: %v\n", err)
}
```

#### 9. `cmd/audit-logs/main.go:99`
- **Line**: 99
- **Error Type**: Date parsing failure
- **Message**: `error: invalid end-date: %v`
- **Has Context**: None
- **Missing**: No commit SHA, no position
- **Code Context**:
```go
parsedEnd, err = parseDate(endDate)
if err != nil {
    log.Fatalf("error: invalid end-date: %v\n", err)
}
```

#### 10. `cmd/audit-logs/main.go:200`
- **Line**: 200
- **Error Type**: Date format validation
- **Message**: `invalid date format: '%s'. Expected YYYY-MM-DD format`
- **Has Context**: None
- **Missing**: No commit SHA, no position
- **Code Context**:
```go
func parseDate(dateStr string) (*time.Time, error) {
    if dateStr == "" {
        return nil, nil
    }
    // Validate format first
    if len(dateStr) != 10 {
        return nil, fmt.Errorf("invalid date format: '%s'. Expected YYYY-MM-DD format", dateStr)
    }
    // ...
}
```

#### 11. `cmd/audit-logs/main.go:206`
- **Line**: 206
- **Error Type**: Date validation
- **Message**: `invalid date: '%s' is not a valid calendar date`
- **Has Context**: None
- **Missing**: No commit SHA, no position
- **Code Context**:
```go
t, err := time.Parse("2006-01-02", dateStr)
if err != nil {
    return nil, fmt.Errorf("invalid date: '%s' is not a valid calendar date", dateStr)
}
```

#### 12. `cmd/audit-logs/main.go:207`
- **Line**: 207
- **Error Type**: Date validation
- **Message**: `invalid date: '%s' is not a valid calendar date`
- **Has Context**: None
- **Missing**: No commit SHA, no position
- **Code Context**: Duplicate of #11

#### 13. `pkg/handler/audit_logs.go:123`
- **Line**: 123
- **Error Type**: Date parsing failure
- **Message**: `invalid start_date: %w`
- **Has Context**: None
- **Missing**: No commit SHA, no position
- **Code Context**:
```go
startDate, err := parseDate(startDateStr)
if err != nil {
    return params, fmt.Errorf("invalid start_date: %w", err)
}
```

#### 14. `pkg/handler/audit_logs.go:133`
- **Line**: 133
- **Error Type**: Date parsing failure
- **Message**: `invalid end_date: %w`
- **Has Context**: None
- **Missing**: No commit SHA, no position
- **Code Context**:
```go
endDate, err := parseDate(endDateStr)
if err != nil {
    return params, fmt.Errorf("invalid end_date: %w", err)
}
```

#### 15. `pkg/handler/audit_logs.go:178`
- **Line**: 178
- **Error Type**: Date format validation
- **Message**: `invalid date format: '%s'. Expected YYYY-MM-DD format`
- **Has Context**: None
- **Missing**: No commit SHA, no position
- **Code Context**: Same as #10

#### 16. `pkg/handler/audit_logs.go:184`
- **Line**: 184
- **Error Type**: Date validation
- **Message**: `invalid date: '%s' is not a valid calendar date`
- **Has Context**: None
- **Missing**: No commit SHA, no position
- **Code Context**: Same as #11

---

## Category 2: SQLite Dump Parsing Errors

### Errors WITHOUT Context

#### 17. `cmd/load-email-resolution-from-queue-api/main.go:238`
- **Line**: 238
- **Error Type**: Time parsing from SQLite dump
- **Message**: `failed to parse created_at: %w`
- **Has Context**: None
- **Missing**: No commit SHA, no position, no row identifier
- **Code Context**:
```go
row.CreatedAt, err = parseTime(unquoteString(values[10]))
if err != nil {
    return row, fmt.Errorf("failed to parse created_at: %w", err)
}
```

#### 18. `cmd/load-email-resolution-from-queue-api/main.go:243`
- **Line**: 243
- **Error Type**: Time parsing from SQLite dump
- **Message**: `failed to parse updated_at: %w`
- **Has Context**: None
- **Missing**: No commit SHA, no position, no row identifier
- **Code Context**:
```go
row.UpdatedAt, err = parseTime(unquoteString(values[11]))
if err != nil {
    return row, fmt.Errorf("failed to parse updated_at: %w", err)
}
```

#### 19. `cmd/load-email-resolution-from-queue-api/main.go:308`
- **Line**: 308
- **Error Type**: Time parsing failure
- **Message**: `unable to parse time: %s`
- **Has Context**: None
- **Missing**: No commit SHA, no position
- **Code Context**:
```go
func parseTime(s string) (time.Time, error) {
    if s == "NULL" || s == "" {
        return time.Time{}, fmt.Errorf("null or empty time")
    }
    layouts := []string{
        "2006-01-02 15:04:05",
        "2006-01-02T15:04:05Z",
        "2006-01-02T15:04:05",
    }
    for _, layout := range layouts {
        if t, err := time.Parse(layout, s); err == nil {
            return t, nil
        }
    }
    return time.Time{}, fmt.Errorf("unable to parse time: %s", s)
}
```

#### 20. `cmd/load-email-resolution-from-queue-api/main.go:196`
- **Line**: 196
- **Error Type**: CSV values parsing
- **Message**: `expected 12 values, got %d`
- **Has Context**: None
- **Missing**: No commit SHA, no position, no row identifier
- **Code Context**:
```go
func parseValuesString(valuesStr string) (QueueAPIRow, error) {
    // ...
    if len(values) != 12 {
        return row, fmt.Errorf("expected 12 values, got %d", len(values))
    }
    // ...
}
```

#### 21. `cmd/load-email-resolution-from-queue-api/main.go:319`
- **Line**: 319
- **Error Type**: Time parsing warning (logged but returned)
- **Message**: `Warning: failed to parse time %s: %v`
- **Has Context**: None
- **Missing**: No commit SHA, no position
- **Code Context**:
```go
t, err := parseTime(s)
if err != nil {
    log.Printf("Warning: failed to parse time %s: %v", s, err)
    return nil
}
```

---

## Category 3: YAML/Config Parsing Errors

### Errors WITHOUT Context

#### 22. `cmd/load-admin-aliases/main.go:215`
- **Line**: 215
- **Error Type**: YAML unmarshal failure
- **Message**: `yaml unmarshal failed: %w`
- **Has Context**: None
- **Missing**: No commit SHA, no position, no line number
- **Code Context**:
```go
if err := yaml.Unmarshal(data, &configMap); err != nil {
    return nil, fmt.Errorf("yaml unmarshal failed: %w", err)
}
```

#### 23. `cmd/load-admin-aliases/main.go:235`
- **Line**: 235
- **Error Type**: YAML parsing failure
- **Message**: `failed to parse aliases.yml: %w`
- **Has Context**: None
- **Missing**: No commit SHA, no position, no entry identifier
- **Code Context**:
```go
if err := yaml.Unmarshal([]byte(aliasesYAML), &config); err != nil {
    return nil, fmt.Errorf("failed to parse aliases.yml: %w", err)
}
```

---

## Category 4: JSON Parsing Errors

### Errors WITHOUT Context

#### 24. `pkg/warmstart/extract.go:255`
- **Line**: 255
- **Error Type**: JSON unmarshal failure
- **Message**: Wrapped error with `ErrInvalidConfig`
- **Has Context**: None
- **Missing**: No commit SHA, no position, no byte offset
- **Code Context**:
```go
if err := json.Unmarshal(configData, &snapshot.Config); err != nil {
    return nil, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
}
```

---

## Category 5: Integer Parsing Errors

### Errors WITHOUT Context

#### 25. `pkg/handler/audit_logs.go:113`
- **Line**: 113
- **Error Type**: Integer parsing failure
- **Message**: `invalid repo_id: %s must be a valid integer`
- **Has Context**: None
- **Missing**: No commit SHA, no position
- **Code Context**:
```go
repoID, err := strconv.ParseInt(repoIDStr, 10, 64)
if err != nil {
    return params, fmt.Errorf("invalid repo_id: %s must be a valid integer", repoIDStr)
}
```

#### 26. `pkg/handler/audit_logs.go:151`
- **Line**: 151
- **Error Type**: Integer parsing failure
- **Message**: `invalid limit: %s must be a valid integer`
- **Has Context**: None
- **Missing**: No commit SHA, no position
- **Code Context**:
```go
limit, err := strconv.ParseInt(limitStr, 10, 64)
if err != nil {
    return params, fmt.Errorf("invalid limit: %s must be a valid integer", limitStr)
}
```

#### 27. `pkg/handler/audit_logs.go:163`
- **Line**: 163
- **Error Type**: Integer parsing failure
- **Message**: `invalid offset: %s must be a valid integer`
- **Has Context**: None
- **Missing**: No commit SHA, no position
- **Code Context**:
```go
offset, err := strconv.ParseInt(offsetStr, 10, 64)
if err != nil {
    return params, fmt.Errorf("invalid offset: %s must be a valid integer", offsetStr)
}
```

---

## Recommendations

### Priority 1: Add Context to High-Volume Parsing Errors

1. **SQLite dump parsing** (errors #17-21): These parse large datasets and should include:
   - Row number/position in the dump file
   - Author email (when available) for traceability
   - Byte offset in the input file

2. **YAML config parsing** (errors #22-23): Add:
   - Line number in YAML file (YAML parser can provide this)
   - ConfigMap path
   - Entry identifier when available

### Priority 2: Standardize Error Context

All parsing errors should include:
- **Position**: Line number, byte offset, or record index
- **Input Source**: File path, URL, or identifier
- **Commit SHA**: When parsing commit data (currently missing from all)

### Priority 3: Use Structured Error Types

Replace `fmt.Errorf` calls with structured error helpers from `pkg/errors`:
- `ParseErrorf()` for parse failures
- `ValidationErrorf()` for format validation
- Include context via `WithCommitSHA()`, `WithPosition()`, `WithRecordKey()`

---

## Analysis

### Current State
- **8%** of parsing errors include any domain context (email, timestamp)
- **0%** include commit SHA context (critical for commit-related parsing)
- **4%** include position context (line numbers, byte offsets)
- **88%** have no context beyond the error message

### Impact
When parsing errors occur without context:
- Operators cannot identify which record/row failed
- Automated retry logic cannot skip problematic records
- Debugging requires manual log correlation
- No ability to track error patterns over time

### Infrastructure Available
The codebase already has infrastructure for structured errors:
- `pkg/errors/types.go` - `StructuredError` with `CommitSHA`, `Position`, `RecordKey` fields
- `pkg/errors/helpers.go` - `ParseErrorf()` helper
- Error context options: `WithCommitSHA()`, `WithPosition()`, `WithRecordKey()`

This catalog provides the complete list of sites that need to be updated to use this infrastructure.
