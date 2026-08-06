# Audit Logs CLI Implementation - cg-62gn6

## Task Completed

Implement CLI command for audit log queries using the service layer.

## Work Completed

### Created CLI Command
**File:** `cmd/audit-logs/main.go`

A new CLI tool that queries audit logs using the modern service layer (`pkg/service`) instead of the legacy `pkg/audit` package.

### Features Implemented

#### ✅ Flag Interface
All required flags from the specification are implemented:
- `--repo-id` (required, must be > 0)
- `--start-date` (optional, YYYY-MM-DD format)
- `--end-date` (optional, YYYY-MM-DD format)
- `--actor` (optional, exact match, case-sensitive, max 255 chars)
- `--event-type` (optional, 'exclude' or 'unexclude')
- `--limit` (optional, 1-1000, default: 100)
- `--offset` (optional, >= 0, default: 0)
- `--output` (optional, 'json' or 'table', default: 'table')

Database connection flags:
- `--db-host` (required)
- `--db-user` (required)
- `--db-password` (required)
- `--db-port` (default: "5432")
- `--db-name` (default: "commitgraph")
- `--sslmode` (default: "require")

#### ✅ Parameter Validation
Comprehensive validation matching the specification:
- Date format validation (YYYY-MM-DD with semantic checks)
- Date chronology validation (start_date <= end_date)
- Integer range validation (repo_id > 0, limit 1-1000, offset >= 0)
- String validation (actor max 255 chars, event_type enum)
- Error messages are clear and actionable

#### ✅ Service Layer Integration
Uses `pkg/service.AuditLogQuerier`:
- Calls `QueryAuditLogs(ctx, repoID, opts)` with parsed parameters
- Properly converts date strings to `time.Time` pointers
- End dates are made inclusive (set to 23:59:59 UTC)
- Handles all optional filters correctly

#### ✅ Output Formatting
Two output formats:

**Table format** (default):
```bash
$ audit-logs -repo-id 123 -output table

Audit Logs for Repo ID: 123
Showing 100 of 500 total records (limit: 100, offset: 0)

┃ ID     │ Timestamp           │ Event      │ Actor      │ Old Excluded │ New Excluded │ Reason
┃───────┼─────────────────────┼────────────┼────────────┼──────────────┼──────────────┼────────
┃ 12345 │ 2024-08-06 10:30:00 │ exclude    │ admin      │ NULL         │ 2024-08-06   │ Policy ...
```

**JSON format**:
```bash
$ audit-logs -repo-id 123 -output json

{
  "records": [
    {
      "id": 12345,
      "repo_id": 123,
      "actor": "admin",
      "timestamp": "2024-08-06T10:30:00Z",
      "event_type": "exclude",
      "old_excluded_at": null,
      "old_excluded_reason": null,
      "new_excluded_at": "2024-08-06T00:00:00Z",
      "new_excluded_reason": "Policy violation"
    }
  ],
  "total_count": 500,
  "limit": 100,
  "offset": 0
}
```

#### ✅ Error Handling
Comprehensive error handling:
- Database connection failures
- Invalid flag values (clear error messages)
- Invalid date formats (YYYY-MM-DD validation)
- Invalid event types (must be 'exclude' or 'unexclude')
- Out-of-range limits (1-1000)
- Invalid chronology (start_date > end_date)
- String length violations (actor > 255 chars)

All errors provide clear, actionable messages.

### Testing Results

#### ✅ Validation Tests
All validation tests pass:
- ✅ repo-id = 0 → "error: -repo-id is required and must be > 0"
- ✅ event-type = "invalid" → "error: event-type must be 'exclude' or 'unexclude', got 'invalid'"
- ✅ limit = 0 → "error: limit must be between 1 and 1000, got 0"
- ✅ limit = 1001 → "error: limit must be between 1 and 1000, got 1001"
- ✅ start_date > end_date → "error: start_date (2024-12-31) cannot be after end_date (2024-01-01)"
- ✅ actor > 255 chars → "error: actor too long: 300 characters exceeds maximum of 255"

#### ✅ Build Test
Binary builds successfully:
```bash
$ go build -o /tmp/audit-logs ./cmd/audit-logs/
# Success - no compilation errors
```

#### ✅ Help Output
Comprehensive help with examples:
```bash
$ audit-logs --help
# Shows detailed usage, all flags, and multiple examples
```

### Comparison with Legacy Tool

**Legacy tool** (`cmd/get-audit-logs/main.go`):
- Uses `pkg/audit` package
- Has additional features (active-only, longstanding, count mode)
- More complex, multiple output modes

**New tool** (`cmd/audit-logs/main.go`):
- Uses `pkg/service` layer (modern architecture)
- Follows the audit log query interface specification
- Simpler, focused on query functionality
- Consistent with REST API handler implementation

### Design Decisions

1. **Single command structure**: Unlike the spec example showing `audit-logs query`, this uses a single command structure for simplicity and consistency with other CLI tools in the repo.

2. **Service layer first**: Direct integration with `pkg/service.AuditLogQuerier` instead of the legacy `pkg/audit` package, aligning with the modern architecture.

3. **Validation order**: Database connection flags are validated first, then query parameters, matching the logical execution order.

4. **Date handling**: End dates are made inclusive by setting them to 23:59:59 UTC, matching the specification's semantic requirements.

5. **Output formats**: Both JSON (machine-readable) and table (human-readable) formats for flexibility.

### Integration Points

This CLI integrates with:
- **Service Layer**: `pkg/service.AuditLogQuerier`
- **Database**: PostgreSQL via `github.com/lib/pq`
- **Specification**: Follows `docs/audit-log-query-interface-spec.md`

### Files Created

- `cmd/audit-logs/main.go` - Complete CLI implementation (400+ lines)
- `notes/cg-62gn6.md` - This documentation

### Usage Examples

```bash
# Basic query with table output
audit-logs -repo-id 123 -db-host localhost -db-user user -db-password pass

# Query with date range
audit-logs -repo-id 123 -start-date 2024-01-01 -end-date 2024-12-31 -db-host localhost -db-user user -db-password pass

# Query by actor with JSON output
audit-logs -repo-id 123 -actor admin -output json -db-host localhost -db-user user -db-password pass

# Paginate results
audit-logs -repo-id 123 -limit 50 -offset 0 -db-host localhost -db-user user -db-password pass

# Combine filters
audit-logs -repo-id 123 -actor admin -event-type exclude -start-date 2024-01-01 -output json -db-host localhost -db-user user -db-password pass
```

### Status

✅ **Complete** - All acceptance criteria met:
- ✅ CLI command accepts all required flags
- ✅ Calls service layer with parsed parameters
- ✅ Outputs formatted table and JSON (user-selectable)
- ✅ Basic error handling for invalid parameters
- ✅ Manual verification complete (build + validation tests)
