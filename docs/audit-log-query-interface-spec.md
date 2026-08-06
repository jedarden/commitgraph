# Audit Log Query Interface Specification

## Overview

This document defines the unified interface contract for querying audit logs in the commitgraph system. It provides a standardized contract that both CLI and API implementations must follow to ensure consistent behavior across all interfaces.

**Version:** 1.0  
**Status:** Design Specification  
**Last Updated:** 2026-08-06  

## Table of Contents

1. [Query Parameters](#query-parameters)
2. [Response Structure](#response-structure)
3. [Parameter Validation Rules](#parameter-validation-rules)
4. [Pagination Behavior](#pagination-behavior)
5. [Error Responses](#error-responses)
6. [Implementation Requirements](#implementation-requirements)
7. [Examples](#examples)
8. [Security Considerations](#security-considerations)

---

## Query Parameters

### Core Parameters

| Parameter | Type | Required | Description | Default |
|-----------|------|----------|-------------|---------|
| `repo_id` | integer | **Yes** | Repository ID to query audit logs for | - |
| `start_date` | string | No | Start date for filtering (YYYY-MM-DD format, inclusive) | None |
| `end_date` | string | No | End date for filtering (YYYY-MM-DD format, inclusive) | None |
| `actor` | string | No | Filter by actor (exact match, case-sensitive) | None |
| `event_type` | string | No | Filter by event type (`exclude` or `unexclude`) | None |
| `limit` | integer | No | Maximum number of records to return | 100 |
| `offset` | integer | No | Number of records to skip for pagination | 0 |

### Parameter Semantics

#### repo_id (Required)
- **Type:** 64-bit signed integer
- **Required:** Yes for all queries
- **Validation:** Must be > 0
- **Description:** The repository ID to query audit logs for. This parameter is required for all queries to ensure scoped access to audit data.

#### start_date (Optional)
- **Type:** String (date in YYYY-MM-DD format)
- **Timezone:** UTC (dates are interpreted at midnight UTC)
- **Inclusivity:** Inclusive (records with timestamp >= start_date 00:00:00 UTC)
- **Validation:** Must match regex `^\d{4}-\d{2}-\d{2}$` and be a valid date
- **Description:** Filters audit records to those occurring on or after the specified date.

#### end_date (Optional)
- **Type:** String (date in YYYY-MM-DD format)
- **Timezone:** UTC (dates are interpreted at midnight UTC)
- **Inclusivity:** Inclusive (records with timestamp <= end_date 23:59:59 UTC)
- **Validation:** Must match regex `^\d{4}-\d{2}-\d{2}$` and be a valid date
- **Description:** Filters audit records to those occurring on or before the specified date.

#### actor (Optional)
- **Type:** String
- **Required:** No
- **Matching:** Exact match, case-sensitive
- **Validation:** Maximum length 255 characters
- **Description:** Filters audit records to those performed by the specified actor (user or system identity).

#### event_type (Optional)
- **Type:** String (enum)
- **Valid Values:** `exclude`, `unexclude`
- **Required:** No
- **Validation:** Must be one of the valid values if provided
- **Description:** Filters audit records by the type of event.

#### limit (Optional)
- **Type:** Integer
- **Required:** No
- **Range:** 1-1000
- **Default:** 100
- **Description:** Maximum number of records to return. Used for pagination.

#### offset (Optional)
- **Type:** Integer
- **Required:** No
- **Range:** >= 0
- **Default:** 0
- **Description:** Number of records to skip for offset-based pagination.

---

## Response Structure

### Standard Response Format

All successful audit log queries MUST return a response with the following structure:

```json
{
  "records": [
    {
      "id": 12345,
      "repo_id": 789,
      "actor": "admin@example.com",
      "timestamp": "2024-08-06T10:15:30.123456Z",
      "event_type": "exclude",
      "old_excluded_at": null,
      "old_excluded_reason": null,
      "new_excluded_at": "2024-08-06T00:00:00Z",
      "new_excluded_reason": "Spam report - ticket #1234"
    }
  ],
  "total_count": 150,
  "limit": 100,
  "offset": 0
}
```

### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `records` | Array | Array of audit log record objects (see below) |
| `total_count` | integer | Total count of records matching the filter (unpaginated) |
| `limit` | integer | The limit used for this query |
| `offset` | integer | The offset used for this query |

### Audit Log Record Object

Each record in the `records` array MUST contain the following fields:

| Field | Type | Nullable | Description |
|-------|------|----------|-------------|
| `id` | integer | No | Unique identifier for the audit log entry |
| `repo_id` | integer | No | Repository ID this audit entry relates to |
| `actor` | string | No | Identity that performed the action |
| `timestamp` | datetime | No | When the action occurred (ISO 8601, UTC) |
| `event_type` | string | No | Type of event (`exclude` or `unexclude`) |
| `old_excluded_at` | datetime | Yes | Previous exclusion state before this action |
| `old_excluded_reason` | string | Yes | Previous exclusion reason before this action |
| `new_excluded_at` | datetime | Yes | New exclusion state after this action |
| `new_excluded_reason` | string | Yes | New exclusion reason after this action |

### Field Types and Formats

- **datetime fields:** ISO 8601 format in UTC: `2024-08-06T10:15:30.123456Z`
- **integer fields:** 64-bit signed integers
- **string fields:** UTF-8 encoded, trimmed of leading/trailing whitespace
- **nullable fields:** `null` in JSON, `nil`/`null` in Go, `null` in SQL

### Empty Results

When no records match the query parameters:

```json
{
  "records": [],
  "total_count": 0,
  "limit": 100,
  "offset": 0
}
```

---

## Parameter Validation Rules

### Date Format Validation

**Format:** `YYYY-MM-DD`  
**Regex:** `^\d{4}-\d{2}-\d{2}$`

#### Validation Steps:

1. **Format check:** Must match the regex pattern
2. **Semantic validity:** Must represent a valid Gregorian calendar date
3. **Range check:** Must be within the range 1970-01-01 to 2100-12-31
4. **Chronology check:** If both `start_date` and `end_date` are provided, `start_date` <= `end_date`

#### Examples:

| Input | Valid | Reason |
|-------|-------|--------|
| `2024-08-06` | Yes | Valid format and date |
| `2024-02-30` | No | February 30th doesn't exist |
| `24-08-06` | No | Wrong format |
| `2024/08/06` | No | Wrong separator |
| `1969-12-31` | No | Before Unix epoch minimum |

### Integer Range Validation

#### repo_id
- **Type:** int64
- **Range:** `1 <= repo_id <= 9223372036854775807`
- **Error if:** 0, negative, or would overflow int64

#### limit
- **Type:** int
- **Range:** `1 <= limit <= 1000`
- **Default:** 100 if not provided or 0
- **Error if:** < 1 or > 1000

#### offset
- **Type:** int
- **Range:** `offset >= 0`
- **Default:** 0 if not provided
- **Error if:** < 0

### String Field Validation

#### actor
- **Type:** string
- **Max length:** 255 characters
- **Allowed characters:** UTF-8 (no ASCII control characters)
- **Trimmed:** Leading/trailing whitespace must be removed
- **Empty handling:** Empty string means "no filter" (not an error)

#### event_type
- **Type:** string (enum)
- **Valid values:** `exclude`, `unexclude`
- **Case-sensitive:** Must match exactly (lowercase)
- **Error if:** Any other value provided

### Multiple Parameter Validation

When multiple parameters are provided:

1. **Date chronology:** `start_date` must be <= `end_date`
2. **Pagination logic:** `offset` should be reasonable (warn if > 100000)
3. **Filter combination:** All filters apply with AND logic

---

## Pagination Behavior

### Default Behavior

- **Default limit:** 100 records
- **Default offset:** 0 (first page)
- **Maximum limit:** 1000 records (hard limit)

### Pagination Semantics

Offset-based pagination is used:

- **offset:** Number of records to skip
- **limit:** Maximum number of records to return
- **total_count:** Total number of records matching filters (unpaginated)

### Pagination Example

For a query with 250 total records:

| Request | Records Returned | Next Offset |
|---------|------------------|-------------|
| `limit=100, offset=0` | Records 1-100 | 100 |
| `limit=100, offset=100` | Records 101-200 | 200 |
| `limit=100, offset=200` | Records 201-250 | End (no more records) |

### Pagination Implementation Requirements

1. **Consistent ordering:** Results MUST be ordered by `timestamp DESC` (most recent first)
2. **Stable pagination:** Adding a record during pagination should not cause duplicate/skipped records
3. **Accurate counting:** `total_count` must reflect all matching records, not just current page
4. **End detection:** Client knows they've reached the end when `records.length == 0` or `offset + records.length >= total_count`

### Pagination Best Practices

1. **Avoid large offsets:** Offsets > 10,000 may have performance implications
2. **Prefer small limits:** Use 50-100 for typical UI, 100-500 for bulk exports
3. **Check total_count:** Always check `total_count` before requesting next page
4. **Handle gaps:** If results change between requests, handle gaps gracefully

---

## Error Responses

### Error Response Format

All errors MUST return a consistent error response structure:

```json
{
  "error": {
    "code": "INVALID_PARAMETER",
    "message": "Invalid date format: '2024/13/01'. Expected YYYY-MM-DD format.",
    "details": {
      "parameter": "start_date",
      "value": "2024/13/01",
      "constraint": "YYYY-MM-DD format, valid date"
    }
  }
}
```

### Standard Error Codes

| Error Code | HTTP Status | Description |
|------------|-------------|-------------|
| `INVALID_PARAMETER` | 400 | A query parameter failed validation |
| `REPO_NOT_FOUND` | 404 | The specified repo_id doesn't exist |
| `ACCESS_DENIED` | 403 | Caller lacks permission to query this repo's audit logs |
| `INTERNAL_ERROR` | 500 | Unexpected server error |
| `DATABASE_ERROR` | 500 | Database query failed |

### Validation Error Messages

#### Date Format Errors

```
"Invalid date format: '{value}'. Expected YYYY-MM-DD format."
"Invalid date: '{value}' is not a valid calendar date."
"Date out of range: '{value}' must be between 1970-01-01 and 2100-12-31."
"Start date after end date: '{start_date}' > '{end_date}'."
```

#### Integer Range Errors

```
"Invalid repo_id: {value}. Must be a positive integer."
"Invalid limit: {value}. Must be between 1 and 1000."
"Invalid offset: {value}. Must be >= 0."
```

#### String Field Errors

```
"Invalid event_type: '{value}'. Must be 'exclude' or 'unexclude'."
"Actor too long: {length} characters exceeds maximum of 255."
```

---

## Implementation Requirements

### For CLI Implementations

1. **Flag consistency:** Must use the parameter names defined in this spec
2. **Error messages:** Must provide clear, actionable error messages
3. **Output formats:** Must support at least JSON and table formats
4. **Exit codes:** Must use appropriate exit codes (0 for success, non-zero for errors)

#### Required CLI Flags

```bash
-repo-id int64         # Repository ID (required)
-start-date string     # Start date YYYY-MM-DD (optional)
-end-date string       # End date YYYY-MM-DD (optional)
-actor string          # Filter by actor (optional)
-event-type string     # Filter by event_type (optional)
-limit int             # Pagination limit (default: 100)
-offset int            # Pagination offset (default: 0)
-output string         # Output format: json|table (default: table)
```

### For API Implementations

1. **Endpoint:** Must use `/api/v1/audit-logs` or similar versioned path
2. **HTTP method:** GET for queries
3. **Content-Type:** Must use `application/json` for responses
4. **HTTP status codes:** Must use appropriate status codes (200, 400, 403, 404, 500)

#### Required API Endpoint

```
GET /api/v1/audit-logs?repo_id={repo_id}&start_date={start_date}&end_date={end_date}&actor={actor}&event_type={event_type}&limit={limit}&offset={offset}
```

#### API Response Headers

```http
Content-Type: application/json
X-Total-Count: 150
X-Limit: 100
X-Offset: 0
```

### Shared Requirements

1. **UTC timezone:** All datetime operations must use UTC
2. **ISO 8601 format:** All datetime outputs must use ISO 8601 format
3. **Null handling:** Nullable fields must be represented as `null` in JSON
4. **Validation order:** Validate required parameters first, then optional parameters
5. **Idempotency:** Repeating the same query with same parameters must return same results (data changes notwithstanding)

---

## Examples

### CLI Examples

#### Basic Query (All Audit Logs for a Repository)

```bash
get-audit-logs \
  -repo-id 789 \
  -output json
```

**Response:**
```json
{
  "records": [
    {
      "id": 12345,
      "repo_id": 789,
      "actor": "admin@example.com",
      "timestamp": "2024-08-06T10:15:30.123456Z",
      "event_type": "exclude",
      "old_excluded_at": null,
      "old_excluded_reason": null,
      "new_excluded_at": "2024-08-06T00:00:00Z",
      "new_excluded_reason": "Spam report"
    }
  ],
  "total_count": 150,
  "limit": 100,
  "offset": 0
}
```

#### Filtered Query (Date Range + Actor)

```bash
get-audit-logs \
  -repo-id 789 \
  -start-date 2024-01-01 \
  -end-date 2024-12-31 \
  -actor "admin@example.com" \
  -output json
```

#### Paginated Query

```bash
get-audit-logs \
  -repo-id 789 \
  -limit 50 \
  -offset 100 \
  -output table
```

### API Examples

#### Basic Query

```http
GET /api/v1/audit-logs?repo_id=789 HTTP/1.1
Host: api.example.com
Accept: application/json
```

**Response:**
```http
HTTP/1.1 200 OK
Content-Type: application/json
X-Total-Count: 150
X-Limit: 100
X-Offset: 0

{
  "records": [...],
  "total_count": 150,
  "limit": 100,
  "offset": 0
}
```

#### Filtered Query

```http
GET /api/v1/audit-logs?repo_id=789&start_date=2024-01-01&end_date=2024-12-31&event_type=exclude HTTP/1.1
```

#### Paginated Query

```http
GET /api/v1/audit-logs?repo_id=789&limit=50&offset=100 HTTP/1.1
```

### Error Response Examples

#### Invalid Date Format

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": {
    "code": "INVALID_PARAMETER",
    "message": "Invalid date format: '2024/13/01'. Expected YYYY-MM-DD format.",
    "details": {
      "parameter": "start_date",
      "value": "2024/13/01",
      "constraint": "YYYY-MM-DD format, valid date"
    }
  }
}
```

#### Repository Not Found

```http
HTTP/1.1 404 Not Found
Content-Type: application/json

{
  "error": {
    "code": "REPO_NOT_FOUND",
    "message": "Repository with repo_id=999999 not found.",
    "details": {
      "repo_id": 999999
    }
  }
}
```

---

## Security Considerations

### Access Control

1. **Repository-scoped access:** Users should only be able to query audit logs for repositories they have access to
2. **Audit trail of queries:** All audit log queries should themselves be logged
3. **Rate limiting:** Implement rate limiting to prevent abuse
4. **Authentication:** All API endpoints must require authentication

### Data Protection

1. **No sensitive data in URLs:** Avoid putting sensitive information in URL parameters (use POST if needed)
2. **HTTPS only:** All API communication must use HTTPS in production
3. **Input sanitization:** All input parameters must be sanitized to prevent SQL injection
4. **Output sanitization:** Ensure no sensitive information leaks in error messages

### Privacy Considerations

1. **Actor privacy:** The `actor` field may contain email addresses or personally identifiable information
2. **Reason field sensitivity:** The `new_excluded_reason` field may contain sensitive information
3. **Retention:** Audit logs should be retained according to the retention policy (90 days default)
4. **Compliance:** Ensure audit log access complies with relevant data protection regulations

---

## Implementation Checklist

### For CLI Implementation

- [ ] Support all required query parameters with correct names
- [ ] Implement all validation rules defined in this spec
- [ ] Return responses in the standard format
- [ ] Provide clear, actionable error messages
- [ ] Support both JSON and table output formats
- [ ] Use UTC timezone for all datetime operations
- [ ] Return appropriate exit codes

### For API Implementation

- [ ] Implement versioned endpoint (`/api/v1/audit-logs`)
- [ ] Support all query parameters via URL parameters
- [ ] Implement all validation rules defined in this spec
- [ ] Return responses in the standard JSON format
- [ ] Use appropriate HTTP status codes
- [ ] Include pagination metadata in response headers
- [ ] Require authentication and enforce access control
- [ ] Log all audit log queries for security auditing

### Testing Checklist

- [ ] Test all validation rules with invalid inputs
- [ ] Test pagination behavior with various limit/offset combinations
- [ ] Test date range filtering across timezone boundaries
- [ ] Test error responses for all error scenarios
- [ ] Test concurrent queries for consistency
- [ ] Test performance with large result sets
- [ ] Test security scenarios (unauthorized access, SQL injection attempts)

---

## Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2026-08-06 | Initial specification |

---

## Related Documentation

- [Audit Log CLI Documentation](audit-log-cli.md)
- [Audit Log Retention Policy](audit-log-retention.md)
- [Architecture: Audit Log Integration](architecture/audit-log-integration.md)
- [Exclusion Audit Query Implementation](../pkg/audit/exclusion_query.go)
- [Service Layer Implementation](../pkg/service/audit_query.go)

---

## Implementation Notes

### Database Schema Reference

This specification assumes the following database schema:

```sql
CREATE TABLE exclusion_audit_log (
  id BIGSERIAL PRIMARY KEY,
  repo_id BIGINT NOT NULL REFERENCES repos(repo_id),
  actor VARCHAR(255) NOT NULL,
  timestamp TIMESTAMP NOT NULL DEFAULT NOW(),
  event_type VARCHAR(20) NOT NULL CHECK (event_type IN ('exclude', 'unexclude')),
  old_excluded_at TIMESTAMP,
  old_excluded_reason TEXT,
  new_excluded_at TIMESTAMP,
  new_excluded_reason TEXT
);

CREATE INDEX idx_exclusion_audit_log_repo_id ON exclusion_audit_log(repo_id);
CREATE INDEX idx_exclusion_audit_log_timestamp ON exclusion_audit_log(timestamp DESC);
CREATE INDEX idx_exclusion_audit_log_actor ON exclusion_audit_log(actor);
```

### Performance Considerations

1. **Index usage:** Ensure queries use the appropriate indexes
2. **Count query optimization:** Consider materialized views for complex counts
3. **Pagination depth:** Avoid offsets > 10,000 for performance reasons
4. **Date range queries:** Use timestamp indexes for efficient date filtering

---

## Appendix A: Implementation Reference

This specification is based on the existing implementation in:
- `pkg/audit/exclusion_query.go` - Core query implementation
- `pkg/service/audit_query.go` - Service layer implementation  
- `cmd/get-audit-logs/main.go` - CLI reference implementation

---

## Appendix B: Future Enhancements

Potential future enhancements not included in this specification:

1. **Cursor-based pagination:** For improved performance with large datasets
2. **Streaming responses:** For very large result sets
3. **Advanced filtering:** Regex matching on reason field, wildcard actor matching
4. **Aggregation queries:** Statistics, trends, time-series data
5. **Export functionality:** CSV export, bulk download

---

*This specification is a living document. Update the version number and date when making changes.*