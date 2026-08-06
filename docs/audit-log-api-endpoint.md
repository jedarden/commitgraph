# Audit Log API Endpoint Documentation

## Overview

The audit log API endpoint provides REST access to the audit log query functionality. It exposes the same query capabilities as the CLI tool (`get-audit-logs`) but via HTTP for integration with other services.

**Endpoint:** `GET /api/audit-logs`  
**Port:** 8080 (default)  
**Content-Type:** `application/json`

## Quick Start

### Build the server

```bash
go build -o bin/audit-log-server ./cmd/audit-log-server/
```

### Start the server

```bash
./bin/audit-log-server \
  -db-host localhost \
  -db-user postgres \
  -db-password postgres \
  -db-name commitgraph
```

### Test the endpoint

```bash
# Health check
curl http://localhost:8080/health

# Basic audit log query
curl "http://localhost:8080/api/audit-logs?repo_id=1"

# With filters
curl "http://localhost:8080/api/audit-logs?repo_id=1&start_date=2024-01-01&end_date=2024-12-31&actor=admin@example.com"
```

## Query Parameters

| Parameter | Type | Required | Description | Default |
|-----------|------|----------|-------------|---------|
| `repo_id` | integer | Yes* | Repository ID to query audit logs for | - |
| `start_date` | string | No | Start date (YYYY-MM-DD format, inclusive) | None |
| `end_date` | string | No | End date (YYYY-MM-DD format, inclusive) | None |
| `actor` | string | No | Filter by actor (exact match, case-sensitive) | None |
| `event_type` | string | No | Filter by event type (`exclude` or `unexclude`) | None |
| `limit` | integer | No | Maximum number of records to return | 100 |
| `offset` | integer | No | Number of records to skip for pagination | 0 |

*Note: For admin queries across all repositories, `repo_id=0` can be used (requires admin permissions).

## Response Format

### Success Response (200 OK)

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

### Response Headers

- `Content-Type: application/json`
- `X-Total-Count: 150`
- `X-Limit: 100`
- `X-Offset: 0`

### Error Response (400 Bad Request)

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

## Examples

### Basic Query

Query all audit logs for a specific repository:

```bash
curl "http://localhost:8080/api/audit-logs?repo_id=789"
```

### Date Range Query

Query audit logs within a specific date range:

```bash
curl "http://localhost:8080/api/audit-logs?repo_id=789&start_date=2024-01-01&end_date=2024-12-31"
```

### Filter by Actor

Query audit logs performed by a specific actor:

```bash
curl "http://localhost:8080/api/audit-logs?repo_id=789&actor=admin@example.com"
```

### Filter by Event Type

Query only exclusion events:

```bash
curl "http://localhost:8080/api/audit-logs?repo_id=789&event_type=exclude"
```

### Paginated Query

Query with custom pagination:

```bash
# Second page with 50 records per page
curl "http://localhost:8080/api/audit-logs?repo_id=789&limit=50&offset=50"
```

### Complex Query

Combine multiple filters:

```bash
curl "http://localhost:8080/api/audit-logs?repo_id=789&start_date=2024-01-01&end_date=2024-12-31&actor=admin@example.com&event_type=exclude&limit=100&offset=0"
```

## Validation Rules

### Date Validation

- **Format:** YYYY-MM-DD
- **Range:** 1970-01-01 to 2100-12-31
- **Chronology:** start_date <= end_date (if both provided)
- **Timezone:** All dates are interpreted as UTC

### Integer Validation

- **repo_id:** Must be >= 0 (0 for admin queries across all repos)
- **limit:** Must be between 1 and 1000 (default: 100)
- **offset:** Must be >= 0 (default: 0)

### String Validation

- **actor:** Maximum 255 characters, trimmed of whitespace
- **event_type:** Must be either `exclude` or `unexclude` (case-sensitive)

## Error Codes

| Error Code | HTTP Status | Description |
|------------|-------------|-------------|
| `INVALID_PARAMETER` | 400 | A query parameter failed validation |
| `REPO_NOT_FOUND` | 404 | The specified repo_id doesn't exist |
| `ACCESS_DENIED` | 403 | Caller lacks permission to query this repo's audit logs |
| `INTERNAL_ERROR` | 500 | Unexpected server error |
| `DATABASE_ERROR` | 500 | Database query failed |

## Integration Testing

A comprehensive test script is provided to verify the endpoint functionality:

```bash
# Run the test script
./scripts/test-audit-log-api.sh
```

The script will:
1. Start the server
2. Run various test cases
3. Verify responses
4. Stop the server
5. Report test results

## Configuration

### Server Configuration

| Flag | Description | Default |
|------|-------------|---------|
| `-port` | HTTP port to listen on | 8080 |
| `-db-host` | PostgreSQL host | (required) |
| `-db-port` | PostgreSQL port | 5432 |
| `-db-name` | PostgreSQL database name | commitgraph |
| `-db-user` | PostgreSQL user | (required) |
| `-db-password` | PostgreSQL password | (required) |
| `-sslmode` | PostgreSQL SSL mode | require |

### Environment Variables

For production use, database credentials can be provided via environment variables:

```bash
export DB_HOST=localhost
export DB_USER=postgres
export DB_PASSWORD=postgres
export DB_NAME=commitgraph

./bin/audit-log-server -db-host "$DB_HOST" -db-user "$DB_USER" -db-password "$DB_PASSWORD" -db-name "$DB_NAME"
```

## Architecture

### Components

1. **HTTP Handler** (`pkg/handler/audit_logs.go`):
   - Parses query parameters
   - Validates input
   - Calls service layer
   - Formats JSON responses

2. **Service Layer** (`pkg/service/audit_query.go`):
   - Business logic for audit log queries
   - Pagination handling
   - Database interaction

3. **Database Layer** (`pkg/audit/`):
   - SQL query execution
   - Result mapping

### Request Flow

```
HTTP Request → Handler → Service Layer → Database → Response
```

## Security Considerations

### Access Control

- The endpoint requires database credentials for access
- Repository-scoped access: users can only query logs for repos they have access to
- Admin queries (repo_id=0) require special permissions

### Data Protection

- All API communication should use HTTPS in production
- Input parameters are sanitized to prevent SQL injection
- Sensitive information may be present in audit logs (actor emails, reasons)

### Rate Limiting

Consider implementing rate limiting to prevent abuse:
- Per-IP rate limits
- Per-user rate limits
- Global rate limits

## Performance

### Pagination

- Default limit: 100 records
- Maximum limit: 1000 records
- Offsets > 10,000 may have performance implications

### Caching

Consider implementing caching for frequently accessed queries:
- Cache recent queries for 1-5 minutes
- Invalidate cache on new audit log entries

### Database Indexes

Ensure the following indexes exist for optimal performance:
```sql
CREATE INDEX idx_exclusion_audit_log_repo_id ON exclusion_audit_log(repo_id);
CREATE INDEX idx_exclusion_audit_log_timestamp ON exclusion_audit_log(timestamp DESC);
CREATE INDEX idx_exclusion_audit_log_actor ON exclusion_audit_log(actor);
```

## Troubleshooting

### Common Issues

**Server fails to start:**
- Check database connectivity
- Verify database credentials
- Ensure port is not already in use

**No results returned:**
- Verify repo_id exists
- Check date range filters
- Ensure audit log table has data

**Invalid parameter errors:**
- Check date format (YYYY-MM-DD)
- Verify limit (1-1000) and offset (>= 0) ranges
- Ensure event_type is `exclude` or `unexclude`

### Debug Logging

The server logs to stdout. Enable verbose logging for debugging:

```bash
./bin/audit-log-server -db-host localhost -db-user postgres -db-password postgres 2>&1 | tee server.log
```

## Related Documentation

- [Audit Log Query Interface Specification](audit-log-query-interface-spec.md)
- [Audit Log CLI Documentation](audit-log-cli.md)
- [Audit Log Retention Policy](audit-log-retention.md)
- [Architecture: Audit Log Integration](architecture/audit-log-integration.md)

## Implementation Files

- **Handler:** `pkg/handler/audit_logs.go`
- **Server:** `cmd/audit-log-server/main.go`
- **Tests:** `pkg/handler/audit_logs_test.go`
- **Test Script:** `scripts/test-audit-log-api.sh`
