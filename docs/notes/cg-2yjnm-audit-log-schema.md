# Audit Log Storage Schema

## Overview

This document specifies the complete storage schema for audit logs in the commitgraph database. The audit log provides a durable, queryable record of all security-sensitive operations, specifically repository exclusion and un-exclusion actions.

**Table**: `exclusion_audit_log`  
**Migration**: `migrations/00007_create_exclusion_audit_log.sql`  
**Go package**: `pkg/audit/exclusion_query.go`

## Schema Definition

### Table Structure

```sql
CREATE TABLE exclusion_audit_log (
  id                  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  repo_id             BIGINT NOT NULL REFERENCES repos(repo_id) ON DELETE CASCADE,
  actor               TEXT NOT NULL,
  timestamp           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  event_type          TEXT NOT NULL,
  old_excluded_at     TIMESTAMPTZ,
  old_excluded_reason TEXT,
  new_excluded_at     TIMESTAMPTZ,
  new_excluded_reason TEXT
);
```

### Field Descriptions

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| `id` | BIGINT | PRIMARY KEY, GENERATED ALWAYS AS IDENTITY | Unique identifier for the audit entry (auto-incrementing) |
| `repo_id` | BIGINT | NOT NULL, FOREIGN KEY → repos(repo_id) ON DELETE CASCADE | Reference to the repository this action affects |
| `actor` | TEXT | NOT NULL | Who performed the action (e.g., 'admin', 'system', username) |
| `timestamp` | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | When the action was performed (UTC) |
| `event_type` | TEXT | NOT NULL | Type of event: 'exclude' or 'unexclude' |
| `old_excluded_at` | TIMESTAMPTZ | NULLABLE | Previous excluded_at value before this action (NULL if not excluded) |
| `old_excluded_reason` | TEXT | NULLABLE | Previous excluded_reason value before this action (NULL if not excluded or no reason) |
| `new_excluded_at` | TIMESTAMPTZ | NULLABLE | New excluded_at value after this action (NULL if unexcluded) |
| `new_excluded_reason` | TEXT | NULLABLE | New excluded_reason value after this action (NULL if unexcluded or no reason) |

## Indexes

### Primary Indexes

| Index Name | Fields | Type | Purpose |
|------------|--------|------|---------|
| `exclusion_audit_log_timestamp_idx` | `timestamp DESC` | B-tree | Efficient time-based queries, most recent first |
| `exclusion_audit_log_repo_idx` | `repo_id, timestamp DESC` | B-tree | Query audit history for specific repositories |
| `exclusion_audit_log_actor_idx` | `actor, timestamp DESC` | B-tree | Query audit history for specific actors |
| `exclusion_audit_log_active_exclusions_idx` | `new_excluded_at` | Partial (WHERE new_excluded_at IS NOT NULL) | Find currently excluded repos |

### Index Usage Patterns

1. **Timestamp DESC index** - Supports:
   - Recent audit log retrieval: `ORDER BY timestamp DESC LIMIT 100`
   - Date range filtering: `WHERE timestamp >= $1 AND timestamp <= $2`

2. **Composite repo_id index** - Supports:
   - Repository-specific history: `WHERE repo_id = $1 ORDER BY timestamp DESC`
   - Efficient join with repos table for provider/repo_full_name lookups

3. **Composite actor index** - Supports:
   - Actor accountability: `WHERE actor = $1 ORDER BY timestamp DESC`
   - Operator-specific audit trails

4. **Partial index on active exclusions** - Supports:
   - Finding all currently excluded repos: `WHERE new_excluded_at IS NOT NULL`
   - Periodic alerting on long-standing exclusions

## Relationships

### Foreign Key Relationship

```
repos (repo_id) ← exclusion_audit_log (repo_id)
    |
    | ON DELETE CASCADE
    |
    v
exclusion_audit_log records are deleted when their repo is deleted
```

This referential integrity ensures:
- Audit entries can only exist for valid repositories
- Cascading deletes maintain consistency when repos are removed
- Join queries can fetch provider/repo_full_name from repos table

## Go Data Model

### ExclusionAuditRecord

```go
type ExclusionAuditRecord struct {
    ID                 int64
    RepoID             int64
    Actor              string
    Timestamp          time.Time
    EventType          string // 'exclude' or 'unexclude'
    OldExcludedAt      *time.Time
    OldExcludedReason  *string
    NewExcludedAt      *time.Time
    NewExcludedReason  *string
}
```

### ExclusionAuditQueryOptions

```go
type ExclusionAuditQueryOptions struct {
    RepoID    int64       // Filter by repo (0 = all repos)
    Actor     string      // Filter by actor (empty = all actors)
    EventType string      // Filter by event type ('exclude' or 'unexclude', empty = all)
    StartDate time.Time   // Filter by timestamp start (zero = no filter)
    EndDate   time.Time   // Filter by timestamp end (zero = no filter)
    Offset    int         // Pagination offset (0 = first page)
    Limit     int         // Results limit (0 = default 100)
}
```

## Query Patterns

### Common Queries

1. **Get recent audit events** (most recent first):
```sql
SELECT * FROM exclusion_audit_log
ORDER BY timestamp DESC
LIMIT 100;
```

2. **Get audit history for a specific repo**:
```sql
SELECT * FROM exclusion_audit_log
WHERE repo_id = $1
ORDER BY timestamp DESC;
```

3. **Get audit history for a specific actor**:
```sql
SELECT * FROM exclusion_audit_log
WHERE actor = $1
ORDER BY timestamp DESC;
```

4. **Date range filtering**:
```sql
SELECT * FROM exclusion_audit_log
WHERE timestamp >= $1 AND timestamp <= $2
ORDER BY timestamp DESC;
```

5. **Find active exclusions** (repos currently excluded):
```sql
WITH ranked_events AS (
    SELECT *, ROW_NUMBER() OVER (PARTITION BY repo_id ORDER BY timestamp DESC) as rn
    FROM exclusion_audit_log
)
SELECT * FROM ranked_events
WHERE rn = 1 AND event_type = 'exclude' AND new_excluded_at IS NOT NULL
ORDER BY timestamp DESC;
```

6. **Find long-standing exclusions** (for alerting):
```sql
WITH latest_exclude AS (
    SELECT DISTINCT ON (repo_id)
        repo_id, new_excluded_at, new_excluded_reason, actor, timestamp
    FROM exclusion_audit_log
    WHERE event_type = 'exclude' AND new_excluded_at IS NOT NULL
    ORDER BY repo_id, timestamp DESC
),
has_unexcluded AS (
    SELECT DISTINCT ON (repo_id)
        repo_id
    FROM exclusion_audit_log
    WHERE event_type = 'unexclude' AND new_excluded_at IS NULL
    ORDER BY repo_id, timestamp DESC
)
SELECT le.*, r.provider, r.repo_full_name
FROM latest_exclude le
INNER JOIN repos r ON le.repo_id = r.repo_id
LEFT JOIN has_unexcluded hu ON le.repo_id = hu.repo_id
WHERE hu.repo_id IS NULL
  AND le.new_excluded_at < NOW() - INTERVAL '30 days'
ORDER BY le.new_excluded_at ASC;
```

## Design Rationale

### Why Separate `old_` and `new_` Fields?

The schema captures **full state transition** rather than just storing a generic "details" JSON blob. This approach:

1. **Queryable**: SQL can filter/query on specific state fields (e.g., `WHERE new_excluded_at IS NOT NULL`)
2. **Type-safe**: Strong typing prevents malformed data
3. **Explicit**: Schema documents exactly what data is captured
4. **Efficient**: Partial indexes on specific fields (e.g., `new_excluded_at`) are possible

### Why Foreign Key to `repos.repo_id`?

Using `repo_id` instead of `provider`/`repo_full_name` strings:

1. **Referential integrity**: Cannot create audit entries for non-existent repos
2. **Space efficiency**: 8-byte BIGINT vs longer strings
3. **Cascade cleanup**: Automatic cleanup when repos are deleted
4. **Join performance**: Integer joins are faster than string comparisons

### Why `TIMESTAMPTZ` Instead of `TIMESTAMP`?

Using `TIMESTAMPTZ` (timestamp with time zone):

1. **Unambiguous**: All times stored in UTC, no time zone confusion
2. **Comparable**: Direct comparison without time zone conversion
3. **Audit compliance**: Precise timestamp for security events

## Operational Considerations

### Retention

Default retention policy: **90 days** (see `docs/audit-log-retention.md`)

Recommended implementation:
- **Option 1**: Partition by month, drop old partitions automatically
- **Option 2**: Scheduled cleanup job: `DELETE FROM exclusion_audit_log WHERE timestamp < NOW() - INTERVAL '90 days'`

### Monitoring

Monitor:
- **Table size**: Alert on excessive growth
- **Cleanup failures**: Alert if retention job fails
- **Query performance**: Monitor index effectiveness

### Backup

Audit logs should be:
- Included in regular database backups
- Archived before deletion if long-term retention is needed
- Consider immutable backup storage for compliance

## Related Documentation

- **Integration**: `docs/architecture/audit-log-integration.md` - Event schema and logging approach
- **Retention**: `docs/audit-log-retention.md` - Retention policy and cleanup
- **Runbook**: `docs/runbooks/repo-exclusion.md` - How to perform exclusion operations
- **Threat Model**: `docs/plan/plan.md` - "Reactive exclusion" residual risk

## Changes

| Date | Change | Author |
|------|--------|--------|
| 2026-08-06 | Initial schema documentation | cg-2yjnm |
