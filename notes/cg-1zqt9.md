# Audit Log Query Interface Design - cg-1zqt9

## Task Completed

Design audit log query interface contract for unified CLI and API implementation.

## Work Completed

Created comprehensive interface specification document at:
`docs/audit-log-query-interface-spec.md`

## Specification Coverage

### ✅ Query Parameters
- **repo_id** (required): Repository ID to query audit logs for
- **start_date** (optional): Start date in YYYY-MM-DD format, inclusive
- **end_date** (optional): End date in YYYY-MM-DD format, inclusive  
- **actor** (optional): Filter by actor (exact match, case-sensitive)
- **event_type** (optional): Filter by event type ('exclude' or 'unexclude')
- **limit** (optional): Pagination limit (default: 100, max: 1000)
- **offset** (optional): Pagination offset (default: 0)

### ✅ Response Structure
Standardized JSON response format with:
- `records`: Array of audit log entries
- `total_count`: Total matching records for pagination
- `limit`: Limit used in query
- `offset`: Offset used in query

Each audit record includes:
- `id`, `repo_id`, `actor`, `timestamp`, `event_type`
- `old_excluded_at`, `old_excluded_reason` (nullable)
- `new_excluded_at`, `new_excluded_reason` (nullable)

### ✅ Parameter Validation Rules
- **Date format**: YYYY-MM-DD with semantic validity checks
- **Integer ranges**: repo_id > 0, limit 1-1000, offset >= 0
- **String validation**: actor max 255 chars, event_type enum validation
- **Chronology checks**: start_date <= end_date

### ✅ Pagination Behavior
- Offset-based pagination with consistent ordering (timestamp DESC)
- Default limit: 100, maximum limit: 1000
- Stable pagination to prevent duplicates/skips
- Accurate total_count for all matching records

### ✅ Additional Specifications
- Error response format with standard error codes
- Implementation requirements for both CLI and API
- Security considerations and access control guidance
- Comprehensive examples for CLI and API usage
- Testing checklists for validation
- Database schema reference

## Design Principles

1. **Consistency**: Both CLI and API follow the same parameter naming and validation
2. **Usability**: Clear error messages and sensible defaults
3. **Security**: Repository-scoped access, audit trail of queries
4. **Performance**: Pagination limits, index usage guidance
5. **Extensibility**: Versioned API endpoints, documented enhancement paths

## Implementation Contract

The specification provides a complete contract that:
- CLI implementations can use for flag definitions and validation
- API implementations can use for endpoint design and response formatting
- Both can rely on for consistent behavior across interfaces
- Testing teams can use for validation test scenarios

## Files Created

- `docs/audit-log-query-interface-spec.md` - Complete specification (560+ lines)

## Status

✅ **Complete** - Design-only task, no code implementation required.