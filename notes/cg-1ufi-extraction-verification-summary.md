# Queue-API Extraction Verification Summary (cg-1ufi)

**Date:** 2026-08-06  
**Bead:** cg-1ufi (Verify extracted queue-api table row counts against the live /stats endpoint)  
**Status:** ❌ **CANNOT COMPLETE - No extraction data exists to verify**

## Executive Summary

The verification task **cannot be completed** because the prerequisite extractions never occurred. Despite parent beads being marked as "closed," only planning and preparation work was completed - **zero rows were actually extracted** from queue-api SQLite database.

## What Was Supposed to Happen

Per TEARDOWN.md checklist:
> "- [ ] Verify row counts against the live `/stats` endpoint"

**Expected Workflow:**
```
1. Extract tables from queue-api (cg-5ol6)
   ├── email_resolution → Postgres identities table
   ├── blocklist → Postgres repos.excluded_at  
   └── tombstones → Postgres tombstones table

2. Capture /stats endpoint data (live queue-api)

3. Compare counts:
   ├── Extracted CSV row counts
   ├── /stats endpoint counts  
   └── Document discrepancies with tolerance

4. Only then: Disable queue-api manifests (TEARDOWN.md gate)
```

## What Actually Happened

### Parent Bead Status: "Planning Complete, No Execution"

**cg-5ol6 (Extract queue-api's remaining tables)** - Status: CLOSED
- ✅ Planning completed (extraction strategy documented)
- ✅ Scripts created (`extract_queue_api_tables.py`, etc.)
- ✅ Postgres migrations prepared (00002_create_tombstones.sql)
- ❌ **Actual extraction: 0% - blocked on admin kubeconfig access**

**Evidence of No Extraction:**

```bash
# No exports directory exists
ls -la /home/coding/commitgraph/exports/
# Result: Exit code 2 (No such file or directory)

# No CSV or JSONL files in repo
find /home/coding/commitgraph -name "*.csv" -o -name "*.jsonl" | grep -v ".beads"
# Result: (no output)

# Verification status confirms no data
cat /home/coding/commitgraph/notes/cg-1ufi-verification-status.md
# Result: "❌ BLOCKED - Cannot verify extraction that has not occurred"
```

### Root Cause: Admin Kubeconfig Access Blocker

**Blocker Details:**
- **Required:** `~/.kube/ord-devimprint-admin.kubeconfig` with valid OIDC token
- **Current Status:** 401 Unauthorized (token expired ~3 days ago)
- **Impact:** Cannot `kubectl exec` into queue-api pod to run SQLite exports

**Blocked Commands:**
```bash
# ❌ BLOCKED - "unable to upgrade connection: Forbidden"
kubectl exec -n commitgraph queue-api-c5894c469-p9rhr -c queue-api \
  -- sqlite3 /data/queue.db "SELECT COUNT(*) FROM email_resolution;"

# ❌ BLOCKED - cannot create port-forward
kubectl port-forward -n commitgraph queue-api-c5894c469-p9rhr 8080:8080
```

### /stats Endpoint Not Accessible

**Attempted Access:**
```bash
# ❌ Not accessible through read-only proxy
kubectl exec -n commitgraph queue-api-c5894c469-p9rhr -c queue-api \
  -- wget -qO- http://localhost:8080/stats
# Result: Exit code 1 (endpoint not found or inaccessible)
```

**Proxy Limitations:**
- Read-only proxy: `http://kubectl-proxy-ord-devimprint:8001`
- ❌ Exec blocked by RBAC
- ❌ Cannot create temporary pods for port-forwarding
- ❌ No HTTP access to cluster-internal services

## Tables That Cannot Be Verified

| Table | Destination | Extraction Status | /stats Data | Verifiable |
|-------|-------------|-------------------|-------------|------------|
| `email_resolution` | Postgres identities table | ❌ Not extracted (365K+ rows) | ❌ Inaccessible | ❌ NO |
| `blocklist` | Postgres repos.excluded_at | ❌ Not extracted | ❌ Inaccessible | ❌ NO |
| `tombstones` | Postgres tombstones table | ❌ Not extracted | ❌ Inaccessible | ❌ NO |
| `repo_head_cursors` | Stays in queue-api PVC | N/A (preserved) | ❌ Inaccessible | ❌ NO |
| `catalog_version` | Stays in queue-api PVC | N/A (preserved) | ❌ Inaccessible | ❌ NO |

## What Would Be Required to Complete Verification

### Prerequisite Chain (All Currently Blocked)

```
1. Admin Access Restoration
   ├─ Refresh OIDC token from Rackspace Spot UI
   ├─ Download to ~/.kube/ord-devimprint-admin.kubeconfig
   └─ Verify: kubectl get pods -n commitgraph

2. Extraction Execution (Only possible with admin access)
   ├─ Run ./scripts/extract_queue_api_tables.py
   ├─ Extract email_resolution (365K+ rows)
   ├─ Extract blocklist 
   ├─ Extract tombstones
   └─ Create timestamped export files

3. /stats Endpoint Access (Currently non-functional)
   ├─ Confirm endpoint exists in queue-api
   ├─ Capture current table counts
   └─ Document endpoint schema

4. Verification Process (Only after above complete)
   ├─ Compare CSV row counts vs /stats endpoint counts
   ├─ Explain discrepancies (rows added between extraction/check)
   └─ Document results with timestamp
```

### Estimated Row Counts (From Schema Documentation)

**Expected When Extracted:**
- `email_resolution`: ~365,000+ rows (365K+ email→login mappings)
- `blocklist`: Unknown (likely 10-100 repos excluded)
- `tombstones`: Unknown (likely <100 GDPR/takedown exclusions)
- `repo_head_cursors`: ~98,747 repos (discovered repos awaiting scan)
- `catalog_version`: 1 row (singleton table)

## TEARDOWN.md Checklist Impact

**Original Checklist Item:**
> "- [ ] Verify row counts against the live `/stats` endpoint"

**Current Status:** ❌ **BLOCKED - Cannot proceed**

**Downstream Impact:**
```
❌ cg-1ufi (verify /stats) BLOCKED
    ↓ blocks
❌ "Only then: .disabled the queue-api manifests and the PVC"
    ↓ blocks
❌ TEARDOWN.md completion
```

## Documentation Delivered Despite Blocker

Although verification cannot proceed, comprehensive preparation was completed:

**✅ Created in cg-5ol6:**
- `/home/coding/commitgraph/scripts/extract_queue_api_tables.py` - Full extraction script
- `/home/coding/commitgraph/scripts/extract-blocklist.sh` - Blocklist-specific extraction
- `/home/coding/commitgraph/scripts/extract-tombstones.sh` - Tombstones-specific extraction  
- `/home/coding/commitgraph/migrations/00002_create_tombstones.sql` - Postgres schema
- `/home/coding/commitgraph/migrations/load_blocklist.sql` - Load script
- `/home/coding/commitgraph/notes/cg-5ol6-extraction-plan.md` - Full plan

**✅ Created in cg-1ufi:**
- `/home/coding/commitgraph/notes/cg-1ufi-verification-status.md` - Detailed blocker analysis
- `/home/coding/commitgraph/notes/cg-1ufi-extraction-verification-summary.md` - This document

**Ready for Immediate Execution:**
The moment admin kubeconfig access is restored, extractions can proceed using the prepared scripts. All SQL queries, bash commands, and verification steps are documented and ready to run.

## Conclusion

**This verification task cannot be completed because:**

1. **No extraction has occurred** - Parent bead cg-5ol6 completed only planning phase (scripts, docs, migrations) but was blocked on admin access before any actual data extraction
2. **No /stats endpoint is accessible** - Endpoint either doesn't exist, isn't exposed, or isn't reachable through available access methods
3. **No data exists to verify** - Zero CSV/JSONL exports were created; `/home/coding/commitgraph/exports/` directory doesn't exist

**Resolution Path:**
1. External dependency: Refresh ord-devimprint-admin.kubeconfig OIDC token from Rackspace Spot UI
2. Execute extraction scripts (cg-5ol6)
3. Capture /stats endpoint data (if endpoint exists)
4. Compare counts and document results
5. Only then can cg-1ufi verification proceed

**Recommendation:** This bead should remain open/blocking until:
- Admin access is restored AND
- Extractions complete AND  
- /stats endpoint data is captured

Only then can the verification required by TEARDOWN.md proceed.

---

**Report Generated:** 2026-08-06  
**Bead:** cg-1ufi (Verify extracted queue-api table row counts against live /stats endpoint)  
**Status:** ❌ CANNOT COMPLETE - No extraction data exists  
**Blocker:** Admin kubeconfig access to ord-devimprint cluster (external dependency)  
**Dependencies:** cg-5ol6 (must complete actual extraction first, not just planning)