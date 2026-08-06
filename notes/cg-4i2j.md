# Corpus Migration: Detection Integration (Bead cg-4i2j)

## Summary

Implemented streaming corpus migration with direct import and usage of `shared/detection.py` per repo during migration. The migration now computes `(user, repo, tool, day, count)` rollups by calling the detection function on each commit message.

## Changes Made

### 1. Created `shared/detection.py`
- **Purpose**: Single source of truth for AI tool signature detection
- **Interface**: `detect_tools(message: str) -> DetectionResult`
- **Catalog**: 14 AI tools with regex patterns (claude-code, cursor, copilot, etc.)
- **Usage**: Imported and called per-commit during migration
- **Key Design**: No SQL reimplementation — pure Python pattern matching

### 2. Updated `migration/migrate_corpus.py`
- **Added import**: `from shared.detection import detect_tools`
- **Implemented `_group_batch_by_repo()`**: Groups Arrow RecordBatches by repo_full_name
- **Implemented `_process_repo()`**: Core migration processing step
  - Calls `detect_tools()` per commit (imported from shared/detection.py)
  - Filters AI-tagged commits only
  - Applies date quarantine [2005-01-01, today+1] (compactor logic)
  - Computes `(user_email, repo, tool, day) -> count` rollup
- **Implemented `_write_rollup_to_postgres()`**: DELETE+bulk-INSERT pattern
  - Upserts repos and users for surrogate key allocation
  - Deletes existing repo rollup rows
  - Bulk inserts new rollup rows with UNNEST
  - Sets insert_time to transaction timestamp

### 3. Architecture Compliance

**Per plan.md "Corpus migration" step 3:**
> "Per repo: re-runs `shared/detection.py` (Python, not reimplemented in SQL — two sources of truth would drift) to compute `(user, repo, tool, day, count)`"

✓ **Direct import**: `from shared.detection import detect_tools`
✓ **No SQL pattern matching**: All regex patterns live in detection.py
✓ **Output shape**: `(user, repo, tool, day, count)` via rollup_counts dictionary
✓ **Per-repo processing**: Each repo's commits processed in `_process_repo()`

### 4. Key Implementation Details

**Date Quarantine (compactor logic from plan.md):**
```python
min_date = date(2005, 1, 1)
max_date = date.today() + datetime.timedelta(days=1)
if not (min_date <= commit_date <= max_date):
    continue  # Exclude from rollup
```

**Rollup Computation:**
```python
for tool in detection_result.tools:
    key = (author_email, repo_full_name, tool, commit_date)
    rollup_counts[key] += 1
```

**Postgres Write Pattern (DELETE+bulk-INSERT for idempotence):**
```sql
DELETE FROM repo_user_daily_tool WHERE repo_id = $1;
INSERT INTO repo_user_daily_tool SELECT * FROM UNNEST(...);
```

## Acceptance Criteria Met

- [x] Migration imports `shared/detection.py` directly, not a copy, port, or reimplementation
- [x] Code review confirms no SQL logic duplicates detection.py's pattern matching
- [x] Output shape is `(user, repo, tool, day, count)` per repo

## Testing

Manual testing of detection module:
```bash
# Test detection module directly
python shared/detection.py "feat: add feature

Co-Authored-By: Claude <noreply@anthropic.com>"

# Should output:
# Tools detected: {'claude-code'}
# Is AI-tagged: True
```

## Notes

- **Identity Resolution**: Migration uses email as login placeholder (will be resolved later via email_resolution table)
- **ARMOR Artifacts**: Parquet artifact writing (step 5b) is TODO for future implementation
- **Streaming**: Never materializes whole partition — processes RecordBatches incrementally
- **Idempotence**: DELETE+INSERT pattern ensures running migration twice produces identical results

## Files Modified

- `shared/__init__.py` (created)
- `shared/detection.py` (created)
- `migration/migrate_corpus.py` (updated: imports, _group_batch_by_repo, _process_repo, _write_rollup_to_postgres)
- `notes/cg-4i2j.md` (this file)

## Next Steps

1. Implement ARMOR artifact writing for redetection support
2. Add error handling for detection failures
3. Implement migration progress resumption logic
4. Test with real corpus partition data
