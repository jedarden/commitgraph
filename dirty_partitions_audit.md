# dirty_partitions Audit Report

**Generated:** 2026-08-08  
**Repository:** commitgraph-deprecated  
**Task:** Audit all references to `dirty_partitions` table

## Summary

The `dirty_partitions` table and all its associated code are **DEAD CODE** following the 2026-08-05 decommissioning of compactor and filter-worker. The new v2 architecture (plan.md) replaces the incremental filter/aggregate passes with direct rollup during extraction, eliminating the need for partition state tracking.

**Total References Found:** 93 occurrences across 23 files  
**Action Required:** DELETE all references and drop the table

---

## Categorization by File Type

### 1. Schema/DDL Files (2 files, 8 occurrences)

| File | Lines | Type | Action |
|------|-------|------|--------|
| `/commitgraph-deprecated/schema.sql:6` | Comment | Documentation | DELETE |
| `/commitgraph-deprecated/schema.sql:132` | Comment | Documentation | DELETE |
| `/commitgraph-deprecated/schema.sql:146` | CREATE TABLE | Schema | DROP TABLE |
| `/commitgraph-deprecated/schema.sql:375` | Comment | Documentation | DELETE |
| `/commitgraph-deprecated/containers/queue-api/schema.sql:6` | Comment | Documentation | DELETE |
| `/commitgraph-deprecated/containers/queue-api/schema.sql:132` | Comment | Documentation | DELETE |
| `/commitgraph-deprecated/containers/queue-api/schema.sql:146` | CREATE TABLE | Schema | DROP TABLE |
| `/commitgraph-deprecated/containers/queue-api/schema.sql:163` | Comment | Documentation | DELETE |

**Action:** Remove these lines from schema.sql files. Create migration to `DROP TABLE IF EXISTS dirty_partitions`.

---

### 2. Core Go Implementation Files (3 files, ~40 occurrences)

#### `/commitgraph-deprecated/containers/queue-api/internal/storage/dirty_partitions.go`
**Lines:** 1-263 (entire file)  
**Purpose:** CRUD operations for dirty_partitions table  
**Status:** DEAD - No consumers remain  
**Action:** DELETE entire file

Key functions in this file:
- `BumpPartitionVersion()` - used by compactor (dead)
- `GetDirtyPartition()` - used by filter/aggregator (dead)
- `AdvanceCursor()` - used by filter/aggregator (dead)
- `cursorColumn()` - maps Consumer to column (dead)

#### `/commitgraph-deprecated/containers/queue-api/internal/server/dirty_partitions.go`
**Lines:** 1-200+ (entire file)  
**Purpose:** HTTP handlers for dirty_partitions endpoints  
**Status:** DEAD - No clients remain  
**Action:** DELETE entire file

#### `/commitgraph-deprecated/containers/queue-api/internal/storage/catalog.go`
**Lines:** 4, 7, 63, 176, 200, 223  
**Purpose:** Catalog version coordination with dirty_partitions  
**Status:** PARTIALLY DEAD - catalog_version table remains but dirty_partitions integration is dead  
**Action:** DELETE dirty_partitions-specific code only

Specific references:
- Line 4: Comment referencing dirty_partitions
- Line 7: Comment about dirty_partitions coordination
- Line 63: `ErrCatalogCASConflict` error message references dirty_partitions
- Line 176: Validation comment mentions dirty_partitions
- Line 200: SELECT query joining dirty_partitions
- Line 223: UPDATE query on dirty_partitions

---

### 3. Stats/Monitoring Code (2 files, ~15 occurrences)

#### `/commitgraph-deprecated/containers/queue-api/internal/storage/stats.go`
**Lines:** 18, 147, 211-267  
**Purpose:** Admin panel statistics including dirty_partitions lag metrics  
**Status:** DEAD - Lag metrics no longer meaningful  
**Action:** DELETE dirty_partitions sections (lines 211-267)

Specific references:
- Line 18: Comment about dirty_partitions lag
- Line 147: Comment mentioning dirty_partitions table
- Line 211: Comment "dirty_partitions summary"
- Line 220: SELECT query for dirty_partitions summary
- Line 223: Error message "stats dirty_partitions summary"
- Line 226: Comment "dirty_partitions per-partition detail"
- Line 233: SELECT query for dirty_partitions rows
- Line 236: Error message "stats dirty_partitions rows"
- Line 250: Error message "scan dirty_partitions row"
- Line 267: Error message "iterate dirty_partitions rows"

#### `/commitgraph-deprecated/containers/queue-api/internal/server/stats.go`
**Purpose:** HTTP handlers for stats endpoints  
**Status:** Contains dirty_partitions handler code  
**Action:** DELETE dirty_partitions-specific handlers

---

### 4. Test Files (15 files, ~30 occurrences)

**All test files referencing dirty_partitions should be DELETED:**

| File | Purpose | Action |
|------|---------|--------|
| `/commitgraph-deprecated/containers/queue-api/internal/storage/dirty_partitions_test.go` | Unit tests for dirty_partitions.go | DELETE |
| `/commitgraph-deprecated/containers/queue-api/internal/storage/dirty_partitions_ops_test.go` | Operations tests | DELETE |
| `/commitgraph-deprecated/containers/queue-api/internal/storage/catalog_test.go` | Catalog CAS tests with dirty_partitions | DELETE dirty_partitions tests |
| `/commitgraph-deprecated/containers/queue-api/internal/storage/stats_test.go` | Stats tests including dirty_partitions | DELETE dirty_partitions tests |
| `/commitgraph-deprecated/containers/queue-api/internal/server/catalog_test.go` | HTTP endpoint tests | DELETE dirty_partitions tests |
| `/commitgraph-deprecated/containers/queue-api/internal/server/stats_test.go` | HTTP stats tests | DELETE dirty_partitions tests |
| `/commitgraph-deprecated/containers/queue-api/internal/server/dirty_partitions_test.go` | HTTP handler tests | DELETE entire file |
| `/commitgraph-deprecated/containers/queue-api/internal/storage/incremental_spine_property_test.go` | Property tests for incremental spine | DELETE (depends on dirty_partitions) |
| `/commitgraph-deprecated/containers/queue-api/internal/storage/schema_test.go` | Schema validation tests | DELETE dirty_partitions columns checks |
| `/commitgraph-deprecated/containers/queue-api/internal/replicate/acceptance_test.go` | Replication tests | DELETE dirty_partitions assertions |

---

### 5. Other Files (3 files, minor references)

#### `/commitgraph-deprecated/containers/queue-api/main.go`
- Line 4: Comment about dirty_partitions
- Line 79: Comment listing dirty_partitions as non-recomputable state
**Action:** DELETE comments

#### `/commitgraph-deprecated/containers/queue-api/internal/storage/epoch_status.go`
- Lines 27, 30, 60, 213, 393, 460, 468
**Purpose:** Epoch retirement logic using dirty_partitions key_id counts
**Status:** DEAD - Epoch retirement depends on dirty_partitions
**Action:** DELETE entire file or dirty_partitions-dependent functions

#### `/commitgraph-deprecated/containers/queue-api/internal/server/epoch_status.go`
**Purpose:** HTTP handlers for epoch status
**Status:** DEAD if it depends on dirty_partitions
**Action:** DELETE dirty_partitions-dependent endpoints

#### `/commitgraph-deprecated/containers/queue-api/internal/config/config.go`
**Purpose:** May have dirty_partitions-related config
**Action:** Review and DELETE dirty_partitions config

#### `/commitgraph-deprecated/containers/queue-api/internal/replicate/store.go`
**Purpose:** B2 replication
**Action:** Review for dirty_partitions references

#### `/commitgraph-deprecated/scripts/provision_b2_ops_bucket.py`
- Line 6: Comment mentioning dirty_partitions
**Action:** DELETE comment

---

## Complete File-by-File Reference Catalog

### Files to DELETE ENTIRELY (dead code, no remaining purpose):

1. `/commitgraph-deprecated/containers/queue-api/internal/storage/dirty_partitions.go` (263 lines)
2. `/commitgraph-deprecated/containers/queue-api/internal/server/dirty_partitions.go` (200+ lines)
3. `/commitgraph-deprecated/containers/queue-api/internal/server/dirty_partitions_test.go`
4. `/commitgraph-deprecated/containers/queue-api/internal/storage/dirty_partitions_ops_test.go`
5. `/commitgraph-deprecated/containers/queue-api/internal/storage/dirty_partitions_test.go`
6. `/commitgraph-deprecated/containers/queue-api/internal/storage/incremental_spine_property_test.go`

### Files to PARTIALLY DELETE (remove dirty_partitions sections):

1. `/commitgraph-deprecated/schema.sql` - Remove lines 6, 132, 146-159, 375
2. `/commitgraph-deprecated/containers/queue-api/schema.sql` - Remove lines 6, 132, 146-159, 163-167
3. `/commitgraph-deprecated/containers/queue-api/internal/storage/catalog.go` - Remove dirty_partitions CAS logic
4. `/commitgraph-deprecated/containers/queue-api/internal/storage/stats.go` - Remove dirty_partitions stats (lines 18, 147, 211-267)
5. `/commitgraph-deprecated/containers/queue-api/internal/server/stats.go` - Remove dirty_partitions handlers
6. `/commitgraph-deprecated/containers/queue-api/internal/storage/epoch_status.go` - Remove dirty_partitions key_id counting
7. `/commitgraph-deprecated/containers/queue-api/internal/server/epoch_status.go` - Remove dependent endpoints
8. `/commitgraph-deprecated/containers/queue-api/main.go` - Remove dirty_partitions comments
9. `/commitgraph-deprecated/scripts/provision_b2_ops_bucket.py` - Remove dirty_partitions comment
10. Test files - Remove dirty_partitions-specific test cases

---

## Migration Required

### SQLite Migration

Create migration file: `/commitgraph-deprecated/migrations/drop_dirty_partitions.sql`

```sql
-- Drop dirty_partitions table and its indexes
-- Migration: drop_dirty_partitions
-- Date: 2026-08-08
-- Reason: Compactor and filter-worker decommissioned 2026-08-05
--          New v2 architecture uses direct rollup, no incremental passes

DROP TABLE IF EXISTS dirty_partitions;

-- Note: catalog_version table remains - used for detection catalog versioning
-- but no longer coordinates with dirty_partitions
```

---

## Verification Steps

After deletion:

1. ✅ Verify no code references `dirty_partitions`:
   ```bash
   grep -r "dirty_partitions" /commitgraph-deprecated/containers/queue-api/ --exclude-dir=.beads
   # Should return only plan/docs comments
   ```

2. ✅ Verify no code references `filtered_version`:
   ```bash
   grep -r "filtered_version" /commitgraph-deprecated/containers/queue-api/ --exclude-dir=.beads
   # Should return nothing
   ```

3. ✅ Verify no code references `aggregated_version`:
   ```bash
   grep -r "aggregated_version" /commitgraph-deprecated/containers/queue-api/ --exclude-dir=.beads
   # Should return nothing
   ```

4. ✅ Run queue-api test suite to ensure no broken imports
5. ✅ Verify queue-api starts successfully without dirty_partitions

---

## Conclusion

**All 93 references to `dirty_partitions` are DEAD CODE following the 2026-08-05 teardown.**

The table and all its associated code paths were used exclusively by:
- **compactor** (bumped `version` column) - DECOMMISSIONED
- **filter-worker** (advanced `filtered_version` cursor) - DECOMMISSIONED  
- **Stage-1 aggregator** (advanced `aggregated_version` cursor) - REPLACED by v2 direct rollup

**Action:** Complete deletion of all dirty_partitions code + DROP TABLE migration.

No remaining live code paths read from or write to dirty_partitions. The new v2 architecture (plan.md) replaces incremental filter/aggregate passes with in-extraction rollup, eliminating the need for partition state coordination entirely.
