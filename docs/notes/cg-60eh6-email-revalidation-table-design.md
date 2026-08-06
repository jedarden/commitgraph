# Email Revalidation Table Design — cg-60eh6

## Context

**Edge case #6** from `docs/plan/plan.md`: "Login renamed after resolution — `email_resolution` now points at a dead login. Needs revalidation, which the predecessor tracked in a `username_revalidation` table; carry the concept forward."

The predecessor's `username_revalidation` table (implemented in `queue-api`) tracked periodic revalidation of cached email→login resolutions to detect renamed or deleted GitHub accounts. Without this layer, a cached resolution would permanently point at a stale or dead handle.

## Table Design

### Schema

```sql
CREATE TABLE email_revalidation (
  email           TEXT NOT NULL PRIMARY KEY,     -- References email_resolution.email
  login           TEXT NOT NULL,                 -- The login being revalidated
  last_checked_at TIMESTAMPTZ NOT NULL,          -- When we last performed a check
  next_check_at   TIMESTAMPTZ,                   -- When to check next (NULL = stop checking)
  status          TEXT NOT NULL,                 -- 'pending' | 'validated' | 'renamed' | 'deleted' | 'retry'
  new_login       TEXT,                          -- If status='renamed', what the account is now called
  check_error     TEXT,                          -- If status='retry', why the check failed
  created_at      TIMESTAMPTZ NOT NULL DEFAULT transaction_timestamp()
);
```

### Status Values

| Status | Meaning | next_check_at | Action |
|--------|---------|----------------|--------|
| `pending` | Never validated, needs immediate check | Set to `transaction_timestamp()` | Worker checks immediately |
| `validated` | Login is live and current | `last_checked_at + 90 days` | No action, will recheck on schedule |
| `renamed` | Account was renamed | `NULL` (stop rechecking) | Caller updates `email_resolution.login` and optionally creates `user_aliases` entry |
| `deleted` | Account is gone (404) | `NULL` (stop rechecking) | Caller marks resolution as stale; resolution retried later |
| `retry` | Transient failure (rate limit, network) | `last_checked_at + short backoff` | Worker retries without losing validation history |

## Design Decisions

### 1. Table Name: `email_revalidation` (not `username_revalidation`)

**Decision**: Named after the entity being validated (`email` from `email_resolution`) rather than the property being checked (`username`).

**Rationale**:
- In v2, identity lives in Postgres — we're validating `email_resolution` rows, not queue-api's cached resolutions
- The primary key is `email`, making the name consistent with the referent
- Avoids confusion with v1's `username_revalidation` (queue-api, provider-scoped)

### 2. Denormalized `login` Column

**Decision**: Store the `login` being validated redundantly (it already exists in `email_resolution.login`).

**Rationale**:
- Performance: `login` is needed for the GitHub API check; joining `email_resolution` adds cost
- Correctness: Captures the login *as it was when revalidation was seeded*, so if `email_resolution` is updated concurrently, the revalidation record still validates the original claim
- Auditing: `login` + `last_checked_at` form a historical record of what was checked when

### 3. Not a Duplicate of `user_aliases`

**Decision**: `email_revalidation` is a validation mechanism, not an aliasing mechanism.

**Key distinction**:
- `user_aliases`: "These logins are the same person" — a deliberate, authoritative merge of identities
- `email_revalidation`: "Is this cached resolution still correct?" — a correctness check whose result *may* feed `user_aliases` but does not replace it

**Workflow example**:
1. Worker claims an `email_revalidation` row with `login=oldhandle`
2. GitHub API returns 404 or a different login → status='renamed', `new_login=newhandle`
3. Caller updates `email_resolution` (if renamed, set `login=newhandle`)
4. Caller *optionally* creates a `user_aliases` entry (`oldhandle → newhandle, reason='rename'`) if this is a confirmed rename rather than a new user
5. Caller marks `email_revalidation.status='renamed'` with `next_check_at=NULL` (stop rechecking)

The revalidation table *discovers* the need for an alias; it doesn't *replace* the alias table.

### 4. 90-Day Revalidation Cadence

**Decision**: Default revalidation interval of 90 days (carried forward from the predecessor).

**Rationale**:
- Predecessor's documented design: "every login is re-checked against the provider on a 90-day cadence"
- Balance between staleness risk and API cost — logins are renamed infrequently
- Configurable per deployment via the worker's `intervalDays` parameter

### 5. Optimistic Claim Pattern (No Lease Columns)

**Decision**: No `claimed_by` / `lease_expires_at` columns; use an optimistic compare-and-set on `next_check_at`.

**Rationale**:
- Predecessor's pattern works well: a claim advances `next_check_at` up-front, and the UPDATE's WHERE clause guards against concurrent claims
- Simpler schema: no lease bookkeeping
- Benign failure mode: a crash between claim and record skips one revalidation cycle for that login — acceptable for a 90-day cadence
- Avoids lease-expiration cleanup logic

## Index Strategy

Three indexes support the three access patterns:

1. **`next_check_at` partial index** (worker's hot path): Find due rows, excluding deleted/renamed (terminal states)
2. **`status` partial index** (seed → claim): Find pending rows that have never been validated
3. **`login` index** (debugging): Find all revalidations for a specific login (manual review, audit)

All partial indexes minimize storage and maintenance cost.

## Migration Placement

**Decision**: Numbered migration `00003`, not folded into `00001`'s initial schema.

**Rationale**:
- This is a follow-on addition for the edge-case handling layer, not part of the core six-table schema
- Per the plan's edge cases section: "carry the concept forward" — a distinct table design, not a transcription
- Allows the core schema (migrations `00001`–`00002`) to stabilize before adding revalidation infrastructure

## Follow-On Work

This table design is the data layer only. Implementation requires:

1. **Worker logic** (likely in `user-enrichment-worker` or a new revalidation worker):
   - Seed revalidation rows when `email_resolution` is populated
   - Claim due rows and check logins against GitHub API
   - Record outcomes and update `email_resolution` / `user_aliases` accordingly
2. **Monitoring**: Alert on retry status spikes (indicates GitHub API rate limiting)
3. **Backfill**: One-time migration to seed `email_revalidation` from existing `email_resolution` rows

---

**Design recorded**: 2026-08-06
**Bead**: cg-60eh6
**Migration**: `migrations/00003_create_email_revalidation.sql`
