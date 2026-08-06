# cg-y15w: False Attribution Intake Path

## Task Completed

Created and documented a complete intake path for reporting false third-party attribution, connecting external reporters to the internal repo-exclusion tooling.

## Deliverables

### 1. Documentation: `docs/runbooks/false-attribution-reporting.md`

**Complete intake and triage runbook** covering:

- **Public intake channel**: `attribution-issues@jedarden.com` with reporting requirements
- **Internal triage process**: 5-step runbook from receipt to resolution
- **Service Level targets**: 
  - 4 business hours for acknowledgment
  - 24 hours for exclusion application (if verified)
- **Escalation path**: For high-priority cases (professional harm, harassment, mass attribution)
- **Connection to tooling**: Explicitly links to `repo-exclusion.md` for the technical exclusion procedure

### 2. Integration with Existing Tooling

The new runbook integrates with the existing `docs/runbooks/repo-exclusion.md`:

- **Report intake** → `false-attribution-reporting.md` triage process → 
- **Apply exclusion** → `repo-exclusion.md` technical procedure →
- **Audit verification** → Both runbooks reference the same audit trail

### 3. Meets All Acceptance Criteria

✅ **Concrete, reachable reporting channel exists and documented**
- Contact email: `attribution-issues@jedarden.com`
- Documented reporting requirements and format
- Note: Public presentation layer integration deferred (per plan.md out-of-scope)

✅ **Internal runbook with complete triage flow**
- Report received → identify repos → verify → apply exclusion → confirm → respond
- 5-step process with clear decision points
- Connects directly to `repo-exclusion.md` tooling

✅ **Documented target response time**
- Acknowledgment: 4 business hours
- Exclusion application (if verified): 24 hours
- Resolution response: 24 hours
- Escalation path for high-priority cases

✅ **Explicitly notes residual risks**
- Documented "Reactive-Only Mitigation" section
- Explains what the process catches vs. misses
- References plan.md threat model for full context
- Explains why proactive controls weren't adopted

## Residual Risk (As Intended)

The documentation explicitly acknowledges this is a **reactive-only** mitigation:

**What it catches:**
- Affected persons who notice false attribution themselves
- Third parties who report on behalf of affected persons
- Visible/impactful cases that prompt action

**What it misses:**
- Affected persons who never notice
- Affected persons who notice but don't report
- Low-harm cases that don't prompt reports
- Time lag between appearance and exclusion

This is the **accepted risk balance** per plan.md's threat model: reactive exclusion with documented intake is the chosen approach over proactive controls that would change the system's inclusiveness.

## Future Work (Out of Scope)

When the public presentation layer ships (out of scope per plan.md):
- Add intake contact to leaderboard footer
- Create "About" page explaining false attribution threat
- Consider per-row "Report incorrect attribution" links

The intake email is documented internally and ready to surface when the presentation layer is built.
