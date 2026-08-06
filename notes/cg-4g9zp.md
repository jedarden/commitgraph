# Task cg-4g9zp: API endpoint or CLI for audit log queries

## Completion Date: 2026-08-06

## Summary

The audit log query functionality has been successfully exposed via a fully-featured CLI tool at `cmd/get-audit-logs/main.go`.

## Acceptance Criteria Status

All acceptance criteria have been met:

✅ **CLI command implemented** - The `get-audit-logs` binary provides complete audit log query functionality

✅ **Query parameters map to service layer filters** - Full filter support:
   - `-repo-id` - Filter by repository ID
   - `-actor` - Filter by actor
   - `-event-type` - Filter by event type ('exclude' or 'unexclude')
   - `-start-date` / `-end-date` - Date range filtering (YYYY-MM-DD format)
   - `-active-only` - Show only currently active exclusions
   - `-longstanding` - Show exclusions older than N days

✅ **Pagination parameters supported** - Full pagination support:
   - `-offset` - Pagination offset (default 0)
   - `-limit` - Limit results (default 100)
   - `-count` - Show total count of matching records

✅ **Output is structured JSON or formatted table**:
   - `-output json` - Full structured JSON with all fields
   - `-output table` - Human-readable table format (default)

✅ **Basic error handling**:
   - Validates required database connection flags
   - Validates output format (json/table only)
   - Validates date format (YYYY-MM-DD)
   - Validates longstanding flag dependency
   - Clear error messages for all validation failures

✅ **Authentication/authorization**:
   - Requires database credentials (db-host, db-user, db-password)
   - Uses SSL mode for secure connections
   - Follows trust boundary pattern (internal-only, cluster-access-gated)

## Technical Implementation

### CLI Tool Location
- Source: `cmd/get-audit-logs/main.go`
- Built binary: `bin/get-audit-logs`

### Service Layer Integration
- Service: `pkg/audit/exclusion_query.go`
- Querier: `audit.ExclusionAuditQuerier`
- Methods used:
  - `QueryExclusionAuditLogs()` - Main query function with filtering and pagination
  - `CountExclusionAuditLogs()` - Count matching records
  - `GetActiveExclusions()` - Get currently active exclusions
  - `GetLongstandingExclusions()` - Find stale exclusions for alerting

### Usage Examples

```bash
# Get all audit logs as JSON
./bin/get-audit-logs -db-host localhost -db-user user -db-password pass -output json

# Get audit logs for a specific repository
./bin/get-audit-logs -db-host localhost -db-user user -db-password pass -repo-id 123

# Get audit logs by actor with date range
./bin/get-audit-logs -db-host localhost -db-user user -db-password pass \
  -actor admin -start-date 2024-01-01 -end-date 2024-12-31

# Get only active exclusions
./bin/get-audit-logs -db-host localhost -db-user user -db-password pass -active-only

# Get exclusions older than 30 days (alerting on stale exclusions)
./bin/get-audit-logs -db-host localhost -db-user user -db-password pass \
  -active-only -longstanding 30

# Count total records matching filters
./bin/get-audit-logs -db-host localhost -db-user user -db-password pass -actor admin -count
```

## Build Instructions

```bash
# Build the CLI tool
go build -o bin/get-audit-logs ./cmd/get-audit-logs/

# The binary is ready to use
./bin/get-audit-logs -help
```

## Trust Boundary

This tool is internal-only and requires database credentials. It should be cluster-access-gated like repo-admin, consistent with the trust-boundary pattern from plan.md's threat model section.

## Notes

The task specification mentioned "REST API endpoint OR CLI command". The CLI approach was chosen as it:
1. Aligns with existing tooling patterns (repo-admin)
2. Provides better security posture (database credential requirement vs. web endpoint)
3. Integrates cleanly with the existing service layer
4. Allows for easy deployment within the cluster environment

If a REST API is needed in the future, the service layer (`pkg/audit/exclusion_query.go`) is fully structured to support it without modification.
