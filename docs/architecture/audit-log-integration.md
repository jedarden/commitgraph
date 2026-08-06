# Audit Log Integration

## Overview

The audit logging system (`pkg/audit/`) provides structured JSON logging for security-sensitive operations, specifically repo-level exclusion actions. This feeds **q-threat-exclusion-audit-log** for incident response and postmortem analysis.

## Event Schema

Every exclusion/clear operation logs a JSON event:

```json
{
  "timestamp": "2026-08-05T12:00:00Z",
  "operation": "exclude",
  "provider": "github",
  "repo_full_name": "owner/repo",
  "operator": "operator-on-call",
  "reason": "false attribution report from user@example.com",
  "rows_affected": 1,
  "incident_id": "INC-2026-0805-001"
}
```

Fields:
- `timestamp` - UTC timestamp of the operation
- `operation` - "exclude" or "clear"
- `provider` - git provider (e.g., "github")
- `repo_full_name` - repository identifier (e.g., "owner/name")
- `operator` - who performed the action (required)
- `reason` - why the exclusion was applied (empty for clear operations)
- `rows_affected` - 1 if repo existed, 0 if not found
- `incident_id` - optional incident tracking identifier

## Log Sink

Currently, the audit logger writes to stderr with `[AUDIT]` prefix. In production, this should be routed to a proper log sink:

### Option A: Loki (Recommended)

Configure the Loki stack to ingest audit logs as a separate stream:

```yaml
# loki-config.yaml (example)
clients:
  - url: http://loki.ardenone.com:3100/loki/api/v1/push

streams:
  - labels:
      app: repo-admin
      level: audit
      namespace: commitgraph
```

Benefits:
- Built-in label-based querying
- Retention policies
- Integration with existing observability stack

### Option B: Dedicated Postgres Table

Create an audit table in the commitgraph database:

```sql
CREATE TABLE exclusion_audit_log (
  log_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  timestamp TIMESTAMPTZ NOT NULL,
  operation TEXT NOT NULL,
  provider TEXT NOT NULL,
  repo_full_name TEXT NOT NULL,
  operator TEXT NOT NULL,
  reason TEXT,
  rows_affected INT NOT NULL,
  incident_id TEXT,
  INDEX (timestamp),
  INDEX (operator),
  INDEX (repo_full_name)
);
```

Modify `pkg/audit/logger.go` to write directly to this table.

Benefits:
- Co-located with the data it protects
- SQL-queryable
- Same backup/restore lifecycle as commitgraph

### Option C: Elasticsearch/Opensearch

If using ELK stack for observability, add audit logs as a separate index pattern.

Benefits:
- Full-text search across reasons
- Aggregation and visualization
- Existing infrastructure if ELK is already deployed

## Querying the Audit Log

### Loki Example

```logql
{app="repo-admin", level="audit"}
| json
| line_format "{{.operation}} {{.provider}}/{{.repo_full_name}} by {{.operator}}"
```

Find all exclusions by a specific operator:
```logql
{app="repo-admin", level="audit"}
| json
| operator="operator-on-call"
```

### Postgres Example

```sql
-- All exclusions in the last 24 hours
SELECT * FROM exclusion_audit_log
WHERE timestamp >= NOW() - INTERVAL '24 hours'
ORDER BY timestamp DESC;

-- All actions by a specific operator
SELECT * FROM exclusion_audit_log
WHERE operator = 'operator-on-call'
ORDER BY timestamp DESC;

-- All actions on a specific repo
SELECT * FROM exclusion_audit_log
WHERE provider = 'github' AND repo_full_name = 'owner/repo'
ORDER BY timestamp DESC;
```

## Integration with Incident Response

The audit log is designed to integrate with incident response workflows:

1. **Incident ID correlation** - Each log can optionally include an `incident_id` field to correlate with ticketing systems
2. **Operator attribution** - Every action is tied to a specific operator for accountability
3. **Reason documentation** - Human-readable reasons explain why each action was taken
4. **Immutable history** - Once written, audit logs are never modified (append-only)

## Retention

Recommended retention: **7 years** for security audit logs (industry standard for authentication/authorization events).

- **Loki**: Configure retention policy per stream
- **Postgres**: Use table partitioning and drop old partitions
- **Elasticsearch**: Configure index lifecycle management (ILM)

## Future Enhancements

Potential additions to the audit system:

1. **Real-time alerts** - Notify on-call when exclusions are applied
2. **Dashboard** - Grafana panel showing exclusion rate and patterns
3. **Anomaly detection** - Flag unusual patterns (e.g., bulk exclusions)
4. **Compliance export** - Generate reports for regulatory requirements

## References

- Runbook: `docs/runbooks/repo-exclusion.md`
- Plan: `docs/plan/plan.md` - "Threat model" section
- Tool: `cmd/repo-admin/main.go`
- Logger: `pkg/audit/logger.go`
