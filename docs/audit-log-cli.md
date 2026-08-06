# Audit Log CLI Documentation

## Overview

The `get-audit-logs` CLI tool provides comprehensive querying capabilities for the exclusion audit log. It supports filtering, pagination, multiple output formats, and specialized queries for operational oversight.

## Installation

The CLI is built as part of the commitgraph project:

```bash
go build -o bin/get-audit-logs ./cmd/get-audit-logs
```

## Usage

### Basic Syntax

```bash
get-audit-logs [flags]
```

### Connection Parameters (Required)

| Flag | Description | Example |
|------|-------------|---------|
| `-db-host` | PostgreSQL host (required) | `-db-host localhost` |
| `-db-port` | PostgreSQL port (default: 5432) | `-db-port 5432` |
| `-db-name` | Database name (default: commitgraph) | `-db-name commitgraph` |
| `-db-user` | PostgreSQL user (required) | `-db-user commitgraph` |
| `-db-password` | PostgreSQL password (required) | `-db-password secret` |
| `-sslmode` | SSL mode (default: require) | `-sslmode require` |

### Filter Parameters

| Flag | Description | Example |
|------|-------------|---------|
| `-repo-id` | Filter by repository ID (0 = all repos) | `-repo-id 123` |
| `-actor` | Filter by actor (empty = all actors) | `-actor admin` |
| `-event-type` | Filter by event type ('exclude' or 'unexclude') | `-event-type exclude` |
| `-start-date` | Start date for filtering (YYYY-MM-DD) | `-start-date 2024-01-01` |
| `-end-date` | End date for filtering (YYYY-MM-DD) | `-end-date 2024-12-31` |
| `-active-only` | Show only currently active exclusions | `-active-only` |
| `-longstanding` | Show exclusions older than N days (requires -active-only) | `-longstanding 30` |

### Pagination Parameters

| Flag | Description | Example |
|------|-------------|---------|
| `-offset` | Pagination offset (0 = first page) | `-offset 100` |
| `-limit` | Limit results (0 = default 100) | `-limit 50` |

### Output Parameters

| Flag | Description | Example |
|------|-------------|---------|
| `-output` | Output format: 'json' or 'table' (default: table) | `-output json` |
| `-count` | Show total count of matching records | `-count` |

## Examples

### Get All Audit Logs

```bash
get-audit-logs \
  -db-host localhost \
  -db-user commitgraph \
  -db-password secret \
  -output table
```

### Query Specific Repository

```bash
get-audit-logs \
  -db-host localhost \
  -db-user commitgraph \
  -db-password secret \
  -repo-id 123 \
  -output table
```

### Filter by Actor and Date Range

```bash
get-audit-logs \
  -db-host localhost \
  -db-user commitgraph \
  -db-password secret \
  -actor admin \
  -start-date 2024-01-01 \
  -end-date 2024-12-31 \
  -output json
```

### Get Active Exclusions

```bash
get-audit-logs \
  -db-host localhost \
  -db-user commitgraph \
  -db-password secret \
  -active-only \
  -output table
```

### Find Longstanding Exclusions

```bash
get-audit-logs \
  -db-host localhost \
  -db-user commitgraph \
  -db-password secret \
  -active-only \
  -longstanding 30 \
  -output table
```

### Paginate Results

```bash
get-audit-logs \
  -db-host localhost \
  -db-user commitgraph \
  -db-password secret \
  -limit 50 \
  -offset 0 \
  -output table
```

### Count Total Records

```bash
get-audit-logs \
  -db-host localhost \
  -db-user commitgraph \
  -db-password secret \
  -actor admin \
  -count
```

## Output Formats

### Table Format (Default)

Human-readable table format for terminal review:

```
┃ ID     │ Repo ID │ Actor      │ Timestamp           │ Event    │ Old Excluded │ New Excluded │ Reason
┃───────┼─────────┼────────────┼─────────────────────┼──────────┼──────────────┼──────────────┼────────
┃ 1     │ 123     │ admin      │ 2024-08-06 10:15:30  │ exclude  │ NULL         │ 2024-08-06   │ spam report
┃ 2     │ 456     │ admin      │ 2024-08-05 15:20:45  │ exclude  │ NULL         │ 2024-08-05   │ policy violation

Total: 2 records
```

### JSON Format

Structured JSON for programmatic consumption:

```json
[
  {
    "id": 1,
    "repo_id": 123,
    "actor": "admin",
    "timestamp": "2024-08-06T10:15:30Z",
    "event_type": "exclude",
    "old_excluded_at": null,
    "old_excluded_reason": null,
    "new_excluded_at": "2024-08-06T00:00:00Z",
    "new_excluded_reason": "spam report"
  }
]
```

## Specialized Queries

### Active Exclusions

Use `-active-only` to show only currently active exclusions (repos that have an exclude event without a subsequent unexclude event):

```bash
get-audit-logs -active-only -output table
```

### Longstanding Exclusions

Use `-longstanding N` with `-active-only` to find exclusions older than N days. This is useful for alerting on the "reactive exclusion" residual risk described in the threat model:

```bash
get-audit-logs -active-only -longstanding 30 -output table
```

Output shows:
```
┃ Repo ID │ Provider  │ Repo Full Name         │ Excluded At         │ Duration │ Actor      │ Reason
┃─────────┼───────────┼────────────────────────┼─────────────────────┼──────────┼────────────┼────────
┃ 123     │ github    │ owner/repo              │ 2024-07-01 10:00:00  │ 36 days  │ admin      │ spam report
```

## Error Handling

The CLI provides clear error messages for common issues:

### Invalid Date Format

```
error: invalid start-date: parsing time "2024/01/01" as "2006-01-02": ...
```

### Database Connection Issues

```
error: failed to connect to PostgreSQL: connection refused
```

### Invalid Flag Combinations

```
error: -longstanding requires -active-only=true
```

## Trust Boundary

This CLI tool is **internal-only** and requires database credentials. It should be:
- Cluster-access-gated like `repo-admin`
- Not exposed on any public or user-facing surface
- Used with database credentials from environment variables in production

## Environment Variables

For production use, set database credentials as environment variables:

```bash
export DB_HOST=localhost
export DB_USER=commitgraph
export DB_PASSWORD=secret

get-audit-logs \
  -db-host $DB_HOST \
  -db-user $DB_USER \
  -db-password $DB_PASSWORD \
  -output table
```

## Related Documentation

- [Audit Log Retention Policy](audit-log-retention.md)
- [Exclusion Audit Query Implementation](../pkg/audit/exclusion_query.go)
- [Service Layer](../pkg/service/audit_query.go)
