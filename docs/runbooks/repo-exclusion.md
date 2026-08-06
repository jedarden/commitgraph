# Repo Exclusion Runbook

## Purpose

This runbook documents the operational procedure for applying and clearing repo-level exclusions to mitigate false attribution threats. See `plan.md` "Threat model" section for context.

## Threat Model Summary

The core exposure is that commit metadata is entirely attacker-controlled:
- Anyone can `git config user.email someone_else@example.com`
- Anyone can add `Co-Authored-By: some-tool` trailers
- Detection is pure pattern matching with no verification

Two harms follow:
1. **Rank inflation** - someone manufactures AI-tagged commits to climb the leaderboard (self-limiting)
2. **Third-party attribution** - commits appear under someone else's identity and link to their real profile (the serious one)

## Mitigation: Repo-Level Exclusion

The schema supports this directly via:
- `repos.excluded_at TIMESTAMPTZ` - non-NULL = excluded from ranking
- `repos.excluded_reason TEXT` - human-readable justification

Exclusion is:
- **Reactive** - requires someone to notice (typically the affected person)
- **Reversible** - clearing `excluded_at` restores contribution on next aggregation cycle
- **Applied at ranking time** - takes effect on next publish, no re-scan needed

## When to Apply Exclusion

Apply exclusion when you receive a **credible false attribution report**:

1. Someone (typically the affected person) reports commits attributed to them that they didn't author
2. The commits are in a repository the reporter doesn't own or control
3. The attribution is harming the reporter by linking their real GitHub profile to fabricated activity

**Do not exclude** for:
- Disagreement about whether commits are AI-generated (the tool detection is the ground truth)
- Rank inflation without third-party harm (self-limiting problem)
- Disputed ownership within a project (internal governance issue)

## Finding the Correct (provider, repo_full_name)

You need two pieces of information to apply an exclusion:

### 1. Identify the provider

Almost always `github` for this system. Check the `providers[]` field on the leaderboard entry.

### 2. Identify the repo_full_name

The `repo_full_name` is the repository identifier (e.g., `owner/name`). To find it:

**Option A: From the leaderboard entry**
- Look at the `top_repo` field on the affected user's leaderboard row
- This is the repo contributing the most AI commits to their total

**Option B: Query the database directly**

```bash
# Connect to the Postgres instance (cluster-access-gated)
kubectl --server=http://traefik-ardenone-manager:8001 exec -n commitgraph -c postgres \
  postgres://commitgraph:$PASSWORD@postgres-commitgraph/commitgraph \
  -- psql -c "
    SELECT DISTINCT r.provider, r.repo_full_name
    FROM repos r
    JOIN repo_user_daily_tool rut ON r.repo_id = rut.repo_id
    JOIN users u ON rut.user_id = u.user_id
    WHERE u.login = 'AFFECTED_USER_LOGIN'
      AND rut.day >= CURRENT_DATE - INTERVAL '30 days'
    ORDER BY rut.commits DESC;
  "
```

Replace `AFFECTED_USER_LOGIN` with the affected person's GitHub login.

**Option C: From the raw corpus artifact**

If you have access to the ARMOR corpus artifacts, search the affected user's commits:
```bash
# This requires ARMOR access and the encryption key
# Pattern: author_email field in the Parquet artifact
```

## Applying an Exclusion

Once you have `(provider, repo_full_name)` and a clear reason:

```bash
# From a pod with cluster access (or via kubectl exec)
repo-admin exclude \
  -db-host postgres-commitgraph \
  -db-user commitgraph \
  -db-password "$DB_PASSWORD" \
  -operator "your-name-or-incident-id" \
  github owner/repo \
  "false attribution report from affected user <email@example.com>, incident #12345"
```

**Required fields:**
- `-operator`: who is performing this action (for audit log)
- `reason`: human-readable justification (required - cannot be empty)

**Result:**
- The tool logs `[AUDIT]` entry with who/when/why
- `repos.excluded_at` is set to `now()`
- `repos.excluded_reason` is set to your provided reason
- On next aggregation cycle (~15 min), that repo's contributions are filtered out
- The affected user's rank drops immediately on next publish

## Clearing an Exclusion

If the report was mistaken or new evidence emerges:

```bash
repo-admin clear \
  -db-host postgres-commitgraph \
  -db-user commitgraph \
  -db-password "$DB_PASSWORD" \
  -operator "your-name-or-incident-id" \
  github owner/repo
```

**Result:**
- `[AUDIT]` log entry records the reversal
- `repos.excluded_at` and `repos.excluded_reason` are set to `NULL`
- On next aggregation cycle, the repo's contributions are restored
- The user's rank climbs back up on next publish

## Checking Exclusion Status

To see if a repo is currently excluded:

```bash
repo-admin status \
  -db-host postgres-commitgraph \
  -db-user commitgraph \
  -db-password "$DB_PASSWORD" \
  github owner/repo
```

**Output:**
- `not excluded` - repo is not excluded
- `excluded since <timestamp> (reason: <text>)` - repo is excluded

## Listing All Exclusions

To see all currently excluded repos:

```bash
repo-admin list \
  -db-host postgres-commitgraph \
  -db-user commitgraph \
  -db-password "$DB_PASSWORD"
```

**Output:** formatted list of all excluded repos with timestamps and reasons.

## Audit Trail

Every exclusion and clear operation is logged to **q-threat-exclusion-audit-log**:

- **Who** - the `-operator` flag value
- **When** - timestamp of the operation
- **Why** - the exclusion reason (or "clear" for reversals)
- **What** - `provider` and `repo_full_name`
- **Result** - rows affected (1 if repo existed, 0 if not found)

This log is critical for incident response and postmortem analysis.

## Example Incident Response

**Scenario:** User `alice-researcher` emails claiming their name appears on the leaderboard with AI commits they didn't author, linking to their real GitHub profile.

**Step 1: Verify the claim**
- Check the leaderboard: `alice-researcher` shows 500 AI commits in last 30 days
- Check their actual GitHub activity: they have zero commits in that period
- Check the `top_repo` field: `suspicious-fork/alice-code`

**Step 2: Find the repo**
- Query the database (see Option B above)
- Confirm `suspicious-fork/alice-code` is contributing to her total

**Step 3: Apply exclusion**
```bash
repo-admin exclude \
  -db-host postgres-commitgraph \
  -db-user commitgraph \
  -db-password "$DB_PASSWORD" \
  -operator "operator-on-call" \
  github suspicious-fork/alice-code \
  "false attribution report from alice@example.com, incident INC-2026-0805-001"
```

**Step 4: Verify fix**
- Wait for next aggregation cycle (~15 minutes)
- Check leaderboard: `alice-researcher`'s count should drop (ideally to zero)
- Confirm with reporter that the false attribution is removed

**Step 5: Document**
- Incident record references the audit log entry
- If reporter provides evidence the repo is actually theirs, clear the exclusion

## Trust Boundary

This tool is **internal-only, cluster-access-gated**:
- Not exposed on any public or user-facing surface
- Requires direct cluster access (kubectl exec or internal network)
- Follows the same pattern as other internal-only endpoints (ingest path, seed endpoint)
- Documented in plan.md under "Identity ingest endpoint" trust boundary

## Residual Risk

Exclusion is **reactive**, not proactive:
- Requires the affected person to notice and report
- Damage is already done when exclusion is applied (the false attribution was already public)
- No proactive verification of author identity (would require GitHub account verification)

**Future hardening (not implemented):**
- Requiring verified GitHub account emails
- Capping single-repo contribution per user
- Minimum repo signal before counting

See plan.md "Threat model" section for full discussion.
