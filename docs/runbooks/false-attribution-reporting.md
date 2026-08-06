# False Attribution Reporting - Intake & Triage Runbook

## Purpose

This runbook documents the complete intake and triage path for receiving and responding to false attribution reports. It connects external reporters to the internal repo-exclusion tooling documented in `repo-exclusion.md`.

## Intake Channel

### Public Reporting Contact

**Primary contact:** `attribution-issues@jedarden.com`

**Alternative:** GitHub issue on `jedarden/commitgraph` repository (if accessible)

**Documentation location:** This email is referenced from the public leaderboard at `commitgraph.jedarden.com` in the footer or "About" section as the contact for attribution concerns.

### What to Include in Reports

When directing someone to report, request they include:

1. **Their GitHub username** (the identity being falsely attributed)
2. **The repository or repositories** they suspect are fabricating their commits
3. **Approximate timeframe** of the false attribution (e.g., "last 30 days")
4. **Brief context** explaining why they believe the attribution is false
5. **Optional:** Any evidence they have (e.g., "I have never committed to that repo")

### Report Format

Reports should be sent with the subject line prefix: `[ATTRIBUTION]`

Example: `[ATTRIBUTION] False attribution - alice-researcher`

## Internal Triage Process

### Step 1: Receive and Log Report

**Time to acknowledge:** Within 4 business hours

When a report arrives:

1. **Assign incident number**: Format `INC-YYYY-MMDD-###` (e.g., `INC-2026-0806-001`)
2. **Initial auto-reply template**:
   ```
   Thank you for your report. We have received your false attribution claim and 
   assigned it incident number INC-2026-0806-001.
   
   A human operator will review your report within 4 business hours. If the claim 
   is verified, the false attribution will be removed from the public leaderboard 
   within 24 hours.
   
   Your report:
   - GitHub username: {their username}
   - Suspected repo(s): {repo names they provided}
   - Timeframe: {timeframe}
   ```

3. **Log the report** in the incident tracker (internal spreadsheet, ticket system, or similar):
   - Incident ID
   - Reporter contact
   - Affected GitHub login
   - Suspected repos
   - Received timestamp
   - Status (triage / investigating / resolved / closed)

### Step 2: Initial Verification (Triage)

**Time to complete:** Within 4 business hours of receipt

Before applying any exclusion, perform basic verification:

1. **Check the leaderboard**:
   - Does the reporter's GitHub login appear?
   - Is there AI commit activity attributed to them?
   - What is the `top_repo` field showing?

2. **Check their actual GitHub activity**:
   - Visit their real GitHub profile
   - Do they have recent commits that could plausibly be the attributed commits?
   - This helps distinguish "wrong repo" from "completely fabricated"

3. **Check the suspected repo(s)**:
   - Does the repo exist?
   - Is it owned by someone other than the reporter?
   - Does it have commits with the reporter's email address?

**Decision points:**

- **If claim is obviously invalid** (e.g., reporter is listed as repo owner, or commits match their actual GitHub history):
  - Respond explaining why exclusion wasn't applied
  - Close incident with rationale
  
- **If claim is plausible and investigation supports it**:
  - Proceed to Step 3 (Apply Exclusion)
  - Update incident status to "investigating"

- **If claim requires deeper investigation** (e.g., complex ownership dispute):
  - Respond to reporter asking for more information
  - Update incident status to "investigating"
  - Set follow-up reminder (24 hours)

### Step 3: Apply Exclusion (if verified)

**Time to complete:** Within 24 hours of verified report

Follow the procedure in `repo-exclusion.md`:

1. **Identify the correct `(provider, repo_full_name)`** (see `repo-exclusion.md` "Finding the Correct (provider, repo_full_name)")
2. **Apply the exclusion** using the `repo-admin exclude` tool
3. **Include incident reference** in the exclusion reason:
   ```
   false attribution report from affected user, incident INC-2026-0806-001
   ```

4. **Wait for next aggregation cycle** (~15 minutes)
5. **Verify the fix** - check that the user's rank/commits dropped appropriately

### Step 4: Respond to Reporter

**Time to complete:** Within 24 hours of verified report (simultaneous with Step 3)

Once exclusion is applied and verified:

1. **Send resolution email**:
   ```
   Your false attribution report (INC-2026-0806-001) has been verified and resolved.
   
   We have excluded the repository '{repo_full_name}' from contributing to your 
   attributed commits. This change will be reflected on the public leaderboard 
   within approximately 15 minutes.
   
   If you believe this exclusion was applied in error, or if you need to report 
   additional repositories, please reply to this email.
   
   Technical reference: Exclusion applied at {timestamp}, operator: {operator name}
   ```

2. **Update incident status** to "resolved"
3. **Document the resolution** in the incident log

### Step 5: Follow-up (if applicable)

**Timeframe:** 7 days post-resolution

- If the reporter replies with additional repos, repeat from Step 2
- If the reporter provides evidence the repo is actually theirs, see `repo-exclusion.md` "Clearing an Exclusion"
- Close incident after 7 days with no further communication

## Target Response Times (Service Level)

| Stage | Target | Rationale |
|-------|--------|-----------|
| **Initial acknowledgment** | 4 business hours | Allows time for human review during business hours |
| **Investigation decision** | 4 business hours | Same window as acknowledgment - single triage pass |
| **Exclusion applied (if verified)** | 24 hours | Bounds the damage duration; allows for thorough verification |
| **Resolution response** | 24 hours | Simultaneous with exclusion; reporter knows when fixed |

**Notes:**
- These targets are **business hours** (Monday-Friday, ~9am-5pm in the operator's timezone)
- Weekend reports are queued for Monday morning triage
- The 24-hour exclusion target is from **verification**, not from initial receipt
- False attribution causing immediate harm (e.g., linking to professional identity) should be escalated and processed faster if possible

## Escalation Path

### High-Priority Cases

Escalate to immediate processing (outside business hours if needed) if:

1. **Professional harm** - The false attribution links to a real identity that could cause professional damage (e.g., academic researcher, public figure)
2. **Harassment** - The reporter indicates this is part of a harassment campaign
3. **Mass attribution** - Multiple users report false attribution from the same repository

**Escalation procedure:**
1. Mark incident as "high priority" in the tracker
2. Send expedited acknowledgment to reporter (within 1 hour if possible)
3. Apply exclusion immediately (skip deeper investigation if claim is prima facie credible)
4. Document why expedited processing was used

### Disputed Exclusions

If someone disputes an exclusion (e.g., repo owner claims the commits are legitimate):

1. Request evidence from both parties
2. Review the actual commit data (author email, timestamps)
3. Make a judgment call based on:
   - Whether the author email matches the reporter's verified GitHub account email
   - Whether the commit timestamps align with the reporter's actual activity
   - Whether the repo has signs of being a fabrication vehicle (e.g., many unrelated author emails)
4. Document the decision thoroughly
5. Be willing to reverse if new evidence emerges

## Residual Risk (Reactive-Only Mitigation)

This intake and triage process is **reactive only** and has inherent limitations:

### What This Process Catches

- Affected persons who notice the false attribution themselves
- Third parties who notice and report on behalf of the affected person
- Cases where the false attribution is visible and impacting enough to prompt action

### What This Process Misses

- **Affected persons who never notice** - their false attribution persists indefinitely
- **Affected persons who notice but don't report** - they may not know how to report, or may not care enough
- **Low-harm cases** - minor false attributions that don't rise to the level of prompting a report
- **Time lag** - damage is done between when false attribution appears and when exclusion is applied

### Why This Is Accepted Risk (Per plan.md)

Proactive mitigations that would catch these cases were considered but **not adopted**:

1. **Requiring verified GitHub account emails** - Would exclude legitimate contributors who don't use verified emails
2. **Capping single-repo contribution** - Would penalize legitimate contributors who work primarily in one repo
3. **Minimum repo signal threshold** - Would exclude legitimate small projects and personal repos

These proactive controls would materially change the system's inclusiveness and data model. Reactive exclusion with a documented intake path is the chosen balance between comprehensiveness and operational complexity.

**References:**
- `plan.md` "Threat model" section for full threat discussion
- `repo-exclusion.md` for the technical exclusion procedure
- `plan.md` Invariant 6 for the audit mechanism that verifies exclusion is working

## Connecting Reports to Exclusion Tooling

This intake process is the **front door** that connects external reporters to the **repo-exclusion.md** back-end tooling:

1. **Report intake** (this runbook) → incident logged → verified → 
2. **Apply exclusion** (`repo-exclusion.md` procedure) → 
3. **Verify fix** (`repo-exclusion.md` "Checking Exclusion Status") → 
4. **Respond to reporter** (this runbook's resolution template)

The audit trail documented in `repo-exclusion.md` ("Audit Trail" section) serves as the permanent record linking each exclusion to its originating incident report.

## Public-Facing Documentation (Future)

When the public presentation layer ships (outside scope of this redesign), the intake contact should be surfaced:

- **Footer on leaderboard page**: "Concerns about attribution? Email attribution-issues@jedarden.com"
- **"About" page**: Brief explanation of the false attribution threat and how to report it
- **Per-row link**: (Optional) "Report incorrect attribution" link next to each leaderboard row that pre-fills a report email with the user's login

For now, while the public serving is the frozen `leaderboard.json`, the intake email is documented internally and will be wired into the public layer when it ships.

## Incident Record Retention

Keep incident records (spreadsheet/ticket system) for at least **1 year** to support:
- Pattern analysis (are specific repos repeatedly used for false attribution?)
- Postmortem reviews (did we miss anything?)
- Audit trail verification (linking incident IDs to exclusion audit logs)

After 1 year, archive rather than delete (may be useful for long-term trend analysis).
