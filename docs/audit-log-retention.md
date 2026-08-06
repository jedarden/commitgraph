# Audit Log Retention Policy

## Overview

This document specifies the retention policy for exclusion audit logs stored in the `exclusion_audit_log` table. These logs are critical for security incident response, compliance, and operational oversight.

## Default Retention Period

**90 days** is the default retention period for exclusion audit logs.

### Rationale

- **Security incident response**: 90 days provides sufficient window to investigate security incidents and perform postmortem analysis
- **Compliance**: Aligns with common industry practices for audit trail retention
- **Storage efficiency**: Prevents unbounded growth of the audit log table
- **Operational needs**: Covers typical quarterly review cycles

## Implementation Options

### Option 1: Database-Level Retention (Recommended)

Use PostgreSQL's partitioning with automatic drop of old partitions:

```sql
-- Partition by month, automatically drop partitions older than 90 days
CREATE TABLE exclusion_audit_log (
    -- ... schema ...
) PARTITION BY RANGE (timestamp);

-- Create monthly partitions
CREATE TABLE exclusion_audit_log_2024_01 PARTITION OF exclusion_audit_log
    FOR VALUES FROM ('2024-01-01') TO ('2024-02-01');

-- Automated cleanup via cron or extension
```

### Option 2: Application-Level Cleanup Job

Run a scheduled job that deletes old records:

```sql
-- Delete records older than 90 days
DELETE FROM exclusion_audit_log
WHERE timestamp < NOW() - INTERVAL '90 days';
```

This can be implemented as:
- A daily cron job
- A scheduled task in the application
- A PostgreSQL extension like `pg_cron`

### Option 3: Configuration via Environment Variable

Make retention period configurable:

```bash
# Environment variable for retention days
AUDIT_LOG_RETENTION_DAYS=90
```

The application would read this and apply cleanup accordingly.

## Current Implementation

**Status**: Not yet implemented. The audit log table exists but retention cleanup is not yet configured.

**Next steps**:
1. Choose implementation approach (recommend Option 1 with partitioning)
2. Implement cleanup mechanism
3. Add monitoring/alerting for cleanup failures
4. Document cleanup schedule in ops runbooks

## Query Interface

The audit log query interface (`get-audit-logs` CLI and `audit.ExclusionAuditQuerier`) provides filtering by date range, which allows:

- **Compliance queries**: "Show me all exclusions in the last 30 days"
- **Incident response**: "What exclusions did admin X perform between date Y and Z?"
- **Operational review**: "Show all exclusions older than 90 days before cleanup"

### Example Queries

```bash
# Get audit logs from the last 30 days
get-audit-logs -start-date $(date -d '30 days ago' +%Y-%m-%d) -output table

# Find longstanding exclusions (alerting on reactive exclusion risk)
get-audit-logs -active-only -longstanding 30 -output table

# Count total audit records
get-audit-logs -count
```

## Compliance Considerations

### Regulatory Requirements

If your organization is subject to regulatory requirements (SOC 2, ISO 27001, GDPR, etc.), ensure the retention policy meets:

- **Minimum retention**: Some regulations require minimum retention periods (e.g., 1 year, 7 years)
- **Data minimization**: GDPR requires not retaining data longer than necessary
- **Right to erasure**: Consider mechanisms to comply with deletion requests

### Audit Trail Integrity

Logs should be:
- **Immutable**: Once written, audit records should not be modified
- **Tamper-evident**: Use signatures or checksums if regulatory requirements demand
- **Backup**: Archive logs before deletion if long-term retention is needed

## Monitoring

Set up alerts for:
- **Cleanup failures**: If the retention job fails, alert to prevent disk space issues
- **Abnormal growth**: Alert if audit log growth rate exceeds expected thresholds
- **Near-full storage**: Alert if database storage is filling up despite retention

## References

- [Threat Model](../docs/plan.md) - "Reactive exclusion" residual risk
- [Audit Log Schema](../migrations/00007_create_exclusion_audit_log.sql)
- [Query Interface](../pkg/audit/exclusion_query.go)

## Changes

| Date | Change | Author |
|------|--------|--------|
| 2026-08-06 | Initial document, 90-day default policy | cg-5knf4 |
