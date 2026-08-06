# Invariant 2 Implementation Summary (cg-5bpf)

## Task
Implement Invariant 2: No rollup row has day outside [2005-01-01, current_date+1]

## Acceptance Criteria Status

### ✅ 1. SQL assertion finds any row with day outside bounds
**File**: `migrations/invariant_2_no_out_of_range_days.sql` (67 lines)

```sql
SELECT
    rut.repo_id, r.provider, r.repo_full_name,
    rut.user_id, u.login, rut.tool, rut.day, rut.commits, rut.insert_time
FROM repo_user_daily_tool rut
JOIN repos r ON rut.repo_id = r.repo_id
JOIN users u ON rut.user_id = u.user_id
WHERE
    rut.day < '2005-01-01'::DATE
    OR rut.day > (CURRENT_DATE + INTERVAL '1 day')::DATE
ORDER BY rut.day DESC;
```

**Result**: Returns 0 rows on pass, violation rows on fail

### ✅ 2. CI fixture includes deliberately out-of-range rows
**Files**:
- `migration/test_invariant_2.py` (319 lines) - Test suite
- `migration/fixtures/create_2170_fixture.py` (162 lines) - Fixture generator
- `.github/workflows/invariant-2-test.yml` (94 lines) - CI workflow

**Test Fixtures**:
- **2170-01-01** date (historical incident reproduction - bf-jyctj/93dc8d1)
- **2004-12-31** date (below minimum bound)
- Valid dates to test no false positives

**Test Execution**:
```bash
# Local testing
./scripts/run-invariant-2-test.sh

# CI execution (GitHub Actions)
# Runs on push/PR to main/develop branches
```

### ✅ 3. Runs as periodic production audit
**File**: `k8s/invariant-2-audit-cronjob.yaml` (228 lines)

**Schedule**: Every 6 hours (`0 */6 * * *`)
- Runs at hours 0, 6, 12, 18 UTC
- Keeps 4 successful jobs, 28 failed jobs for history
- 10-minute timeout, 2 retries on failure

**Deployment**:
```bash
kubectl apply -f k8s/invariant-2-audit-cronjob.yaml
```

**Manual execution**:
```bash
kubectl create job --from=cronjob/invariant-2-audit manual-audit-$(date +%s)
```

### ✅ 4. Alert wired on production violations
**Implementation**:
- **Slack webhook** integration via `SLACK_WEBHOOK_URL` environment variable
- **Kubernetes alerting** via job failure (exit code 1 on violations)
- **Alert message** includes:
  - Environment name
  - Violation count
  - Database details
  - Next steps for investigation
  - Reference to historical incident (bf-jyctj)

**Exit codes**:
- `0` = No violations (audit passed)
- `1` = Violations found (audit failed - ALERT TRIGGERED)
- `2` = Error running audit

## Implementation Components

| Component | File | Purpose |
|-----------|------|---------|
| SQL assertion | `migrations/invariant_2_no_out_of_range_days.sql` | Query finding violations |
| Test suite | `migration/test_invariant_2.py` | CI fixture tests |
| Fixture generator | `migration/fixtures/create_2170_fixture.py` | Test data creation |
| Audit script | `scripts/audit-invariant-2.sh` | Production audit runner |
| Local test runner | `scripts/run-invariant-2-test.sh` | Local development testing |
| CI workflow | `.github/workflows/invariant-2-test.yml` | GitHub Actions integration |
| Production CronJob | `k8s/invariant-2-audit-cronjob.yaml` | Periodic production audit |
| Argo WorkflowTemplate | `k8s/invariant-2-workflowtemplate.yaml` | Argo Workflows integration |

## Historical Context

This invariant is the **2170-incident guard**:
- A single 2170-dated commit once zeroed the board-wide AI-commit count
- Quarantine bead: `bf-jyctj`
- Commit: `93dc8d1`
- Aggregator fix: `946e815`

The clamp must be applied **before** any day value reaches Postgres to prevent recurrence.

## Testing

All acceptance criteria are verified and passing:
- ✅ SQL correctly detects out-of-range dates
- ✅ CI fixtures include historical incident reproduction
- ✅ Periodic production audit configured
- ✅ Alerting wired and non-silent

## Status

**COMPLETE** - All acceptance criteria met, implementation deployed and tested.
