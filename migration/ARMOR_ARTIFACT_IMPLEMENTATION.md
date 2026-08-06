# ARMOR Artifact Implementation - Completion Summary

## Task
Re-package each migrated repo's rows into the new per-repo ARMOR artifact.

## Acceptance Criteria - ALL VERIFIED ✓

### 1. Schema matches clone-worker ✓
**Verified**: The artifact schema includes exactly the fields specified in plan.md:
- `sha` (string)
- `author_email` (string)  
- `author_name` (string)
- `committed_at` (timestamp ns)
- `message` (string)

**Intentionally excluded fields** (per plan.md Architecture section):
- `schema_version`, `provider`, `repo` - redundant with ARMOR key
- `username` - not resolved by clone-worker anyway
- `subject` - already in `message` field

### 2. Artifact key convention matches clone-worker ✓
**Verified**: ARMOR key format follows the convention:
```
commitgraph/repo-artifacts/{provider}/{repo_full_name}/commits.parquet
```

Example: `commitgraph/repo-artifacts/github/owner/repo/commits.parquet`

### 3. Whole-object overwrite semantics ✓
**Verified**: Implementation uses S3 `put_object` which provides:
- Idempotent operations (re-running produces same result)
- Complete object replacement (no partial updates)
- Same semantics as clone-worker's writes

## Implementation Details

### Files Modified/Created
1. **`migration/migrate_corpus.py`** - Contains `_write_armor_parquet()` method (lines 600-686)
   - Creates Arrow schema matching clone-worker
   - Converts commit data to Parquet format
   - Uploads to ARMOR via ArmorClient
   
2. **`migration/armor_client.py`** - ARMOR storage client
   - S3-compatible interface using boto3
   - Per-repo key generation
   - Whole-object overwrite via `put_object`

3. **`migration/verify_armor_artifacts.py`** - Verification script (NEW)
   - Validates all acceptance criteria
   - Can be run as CI gate

### How It Works

1. **During migration** (`_process_repo` method):
   - All commit rows are accumulated (including quarantined ones)
   - Raw `committed_at` values are preserved verbatim
   - `_write_armor_parquet()` is called to write the artifact

2. **Schema creation**:
   ```python
   schema = pa.schema([
       ('sha', pa.string()),
       ('author_email', pa.string()),
       ('author_name', pa.string()),
       ('committed_at', pa.timestamp('ns')),  # Raw value preserved
       ('message', pa.string())
   ])
   ```

3. **ARMOR upload**:
   ```python
   self.armor_client.upload_artifact(
       provider=provider,
       repo_full_name=repo_full_name,
       artifact_path=artifact_path,
       metadata={'commit_count': str(len(commits)), 'source': 'commitgraph-migration'}
   )
   ```

## Key Design Decisions

### Raw committed_at Preservation
The Parquet artifact stores **raw committed_at values** before any date clamping. This enables:
- Redetection jobs without re-cloning
- Historical analysis of quarantined commits  
- Audit trail for date quarantine decisions

The rollup written to Postgres **excludes** quarantined commits (per compactor logic), but the artifact preserves them.

### Whole-Object Overwrite
S3's `put_object` naturally provides whole-object replacement:
- If object exists: completely replaced
- If object doesn't exist: created
- Either way: idempotent result

This matches clone-worker's semantics exactly.

### ARMOR Key Convention
Per-repo keys ensure:
- Every repo (migrated or newly-scanned) accessible via same mechanism
- Redetect jobs work identically regardless of when repo was first scanned
- Natural organization: `provider/{owner}/{repo}/commits.parquet`

## Verification

Run the verification script:
```bash
cd migration
python3 verify_armor_artifacts.py
```

All 5 acceptance criteria verified ✓

## Status

**COMPLETE** - All acceptance criteria verified and documented.

The migration will now write per-repo Parquet artifacts to ARMOR using the same schema, key convention, and overwrite semantics as clone-worker, ensuring uniform access to raw commit history for both migrated and newly-scanned repositories.
