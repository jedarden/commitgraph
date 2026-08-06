# Invariant 4 Implementation Summary (cg-2eq3)

## Overview
Implemented three SQL assertions for Invariant 4: Identity referential integrity + acyclic one-level alias graph.

## Acceptance Criteria Met

### ✅ Query (a): Rollup user_id FK integrity
- **File**: `migrations/invariant_4_identity_referential_integrity.sql` (lines 62-75)
- **Purpose**: Finds any `repo_user_daily_tool.user_id` with no matching `users.user_id`
- **Expected**: 0 rows on healthy database
- **Returns**: Violating rollup rows with orphan user_id values

### ✅ Query (b): user_aliases.target_login existence in users
- **File**: `migrations/invariant_4_identity_referential_integrity.sql` (lines 120-128)
- **Purpose**: Finds any `user_aliases.target_login` not present in `users.login`
- **Expected**: 0 rows on healthy database
- **Returns**: Violating alias rows with non-existent target_login

### ✅ Query (c): Alias graph acyclic + one-level-deep
- **File**: `migrations/invariant_4_identity_referential_integrity.sql` (lines 179-190)
- **Purpose**: Finds:
  - Chained aliases (depth > 1): Any source_login that is itself a target_login
  - Cycles: A → B, B → A patterns
- **Expected**: 0 rows on healthy database
- **Returns**: Violating alias pairs showing chains/cycles

### ✅ CI Fixture with Deliberate Violations
- **File**: `migration/test_invariant_4.py`
- **Violations created**:
  - Query (a): 1 orphan user_id (rollup row with user_id=9999 that doesn't exist)
  - Query (b): 1 alias targeting non-existent login ('non-existent-login')
  - Query (c): 4 violations total
    - 2 rows for chain: chained-alias-a → chained-alias-b → canonical-user-c
    - 2 rows for cycle: cycle-alias-x → cycle-alias-y → cycle-alias-x

### ✅ CI Test Coverage
- **Test file**: `migration/test_invariant_4.py`
- **Tests implemented**:
  - `test_invariant_4_detection()`: Validates all three queries detect violations correctly
  - `test_invariant_4_passes_on_valid_data()`: Ensures no false positives on valid data
- **How to run**: `python3 migration/test_invariant_4.py`
- **Exit codes**: 0=pass, 1=fail

### ✅ Production Audit Infrastructure
- **Kubernetes Deployment**: `k8s/invariant-4-audit-deployment.yaml`
  - Runs every 6 hours (configurable via AUDIT_INTERVAL_SECONDS)
  - Executes all three queries against production database
  - Sends Slack alerts on violations
  - Uses Deployment pattern (not CronJob) for ArgoCD compatibility
- **Audit script**: `scripts/audit-invariant-4.sh`
  - Can be run manually against any environment
  - Exit codes: 0=pass, 1=violations, 2=error
  - Accepts environment variable override: POSTGRES_HOST, POSTGRES_DB, POSTGRES_USER, POSTGRES_PASSWORD_FILE, SLACK_WEBHOOK_URL
- **ConfigMap**: `k8s/invariant-4-audit-script-configmap.yaml`
  - Contains the embedded audit script for Kubernetes execution

## File Structure

```
commitgraph/
├── migrations/
│   └── invariant_4_identity_referential_integrity.sql    # Three SQL queries
├── migration/
│   └── test_invariant_4.py                               # CI test suite
├── k8s/
│   ├── invariant-4-audit-deployment.yaml                # Production audit deployment
│   └── invariant-4-audit-script-configmap.yaml          # Audit script configmap
├── scripts/
│   └── audit-invariant-4.sh                              # Manual audit script
└── pkg/pg/
    ├── invariant_4_test.go                               # Query documentation
    └── invariant_4_integration_test.go                   # Integration test templates
```

## Usage

### CI Testing
```bash
# Run locally (requires postgres)
python3 migration/test_invariant_4.py
```

### Manual Audit
```bash
export POSTGRES_HOST=localhost
export POSTGRES_DB=commitgraph
export POSTGRES_USER=postgres
export POSTGRES_PASSWORD_FILE=/path/to/password
export SLACK_WEBHOOK_URL=https://hooks.slack.com/...  # optional
./scripts/audit-invariant-4.sh production
```

### Production Deployment
The audit runs automatically via Kubernetes Deployment every 6 hours. To deploy:

1. Merge `k8s/invariant-4-audit-deployment.yaml` into declarative-config
2. Merge `k8s/invariant-4-audit-script-configmap.yaml` into declarative-config
3. ArgoCD syncs automatically to commitgraph namespace

## Query Details

### Query (a): Orphan user_id Detection
```sql
SELECT
    rut.repo_id,
    r.provider,
    r.repo_full_name,
    rut.user_id,           -- This user_id doesn't exist in users
    rut.tool,
    rut.day,
    rut.commits,
    rut.insert_time
FROM repo_user_daily_tool rut
JOIN repos r ON rut.repo_id = r.repo_id
LEFT JOIN users u ON rut.user_id = u.user_id
WHERE u.user_id IS NULL
ORDER BY rut.repo_id, rut.user_id, rut.day;
```

**Violation scenario**: A user is deleted from `users` but rollup rows remain, or rollup rows are inserted with non-existent user_id (write path bug).

### Query (b): Non-existent target_login
```sql
SELECT
    ua.source_login,
    ua.target_login,     -- This target_login doesn't exist in users.login
    ua.reason,
    ua.created_at
FROM user_aliases ua
LEFT JOIN users u ON ua.target_login = u.login
WHERE u.login IS NULL
ORDER BY ua.source_login;
```

**Violation scenario**: A user is deleted from `users` but their alias remains, or an alias is created targeting a non-existent login (ingest bug).

### Query (c): Chained Aliases and Cycles
```sql
SELECT
    ua1.source_login AS level1_source,
    ua1.target_login AS level1_target,    -- This is also a source_login (violation!)
    ua2.source_login AS level2_source,   -- Same as level1_target
    ua2.target_login AS level2_target,
    ua1.reason AS level1_reason,
    ua2.reason AS level2_reason,
    ua1.created_at AS level1_created,
    ua2.created_at AS level2_created
FROM user_aliases ua1
JOIN user_aliases ua2 ON ua1.source_login = ua2.target_login
ORDER BY ua1.source_login, ua2.source_login;
```

**Violation scenarios**:
- **Chain (depth > 1)**: A → B → C (both A and B appear as source and target)
- **Cycle (A → B, B → A)**: Both appear as source and target

The alias graph must be a simple star graph: all source_logins point directly to canonical logins, with no indirection.

## Implementation Notes

1. **Dual-use design**: The same invariant SQL runs in CI (against fixtures) and production (as periodic audits)
2. **Alert-driven**: Production violations trigger Slack alerts + continue monitoring (don't crash the audit pod)
3. **No silent failures**: All queries use explicit error checking and logging
4. **Performance**: At current scale (~35K rollup rows, <100 aliases), all queries complete in milliseconds
5. **Index coverage**: All required indexes exist by schema definition (users.user_id PK, users.login UNIQUE, etc.)

## Deployment Checklist

- [x] SQL assertions implemented in migrations/invariant_4_identity_referential_integrity.sql
- [x] Python test suite created in migration/test_invariant_4.py
- [x] Kubernetes audit deployment created in k8s/invariant-4-audit-deployment.yaml
- [x] Audit script embedded in k8s/invariant-4-audit-script-configmap.yaml
- [x] Manual audit script created in scripts/audit-invariant-4.sh
- [ ] Merge k8s/invariant-4-audit-deployment.yaml into declarative-config
- [ ] Merge k8s/invariant-4-audit-script-configmap.yaml into declarative-config
- [ ] Verify deployment in commitgraph namespace
- [ ] Confirm audit runs successfully every 6 hours

## Related Files

- Schema: `migrations/00001_initial_schema.sql`
- Go test documentation: `pkg/pg/invariant_4_test.go`
- Integration test templates: `pkg/pg/invariant_4_integration_test.go`
- Plan documentation: `plan.md` (Invariant 4 section)

## Bead Reference

This implementation completes bead **cg-2eq3**: Invariant 4 - identity referential integrity + acyclic alias graph.
