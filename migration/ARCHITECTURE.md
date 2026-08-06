# Streaming Corpus Migration - Architecture Documentation

## Critical Design Constraint: OOM Prevention

The predecessor pipeline had a proven OOM incident: a 2Gi pod ran out of memory materializing 400K commits' message bodies from a single partition. At 76M commits across all partitions, repeating that pattern would cause catastrophic failures.

### The OOM Pattern That Must Be Avoided

```python
# ❌ FORBIDDEN - This pattern caused OOM incidents
import pyarrow.parquet as pq

partition_path = "provider=github/year=2024/month=08"
table = pq.read_table(partition_path)  # Materializes ENTIRE partition in memory

for i in range(table.num_rows):
    commit = table.slice(i, 1)  # All 400K+ commits already loaded
    process_commit(commit)      # OOM at scale
```

### The Streaming Pattern Required

```python
# ✓ REQUIRED - Streaming pattern, bounded memory
from streaming_reader import PartitionStream

stream = PartitionStream(partition_path, batch_size=10000)
for batch in stream.iter_batches():
    # Only 10K rows in memory at a time
    process_batch(batch)
    # Batch is discarded after processing
```

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                      Corpus Migration Flow                          │
└─────────────────────────────────────────────────────────────────────┘

  Existing Corpus (B2, encrypted, Hive-partitioned)
           │
           │ CorpusStream.iter_partitions()
           │
           ▼
  ┌─────────────────┐
  │  Encryption Key │  ← Validate migration credentials can decrypt
  │  Discovery      │    all epochs (not just current)
  └─────────────────┘
           │
           │ For each partition:
           │ PartitionStream.iter_batches()
           │
           ▼
  ┌─────────────────────────────────────────┐
  │     RecordBatchReader (Streaming)        │
  │  ┌──────────┐  ┌──────────┐  ┌────────┐│
  │  │ Batch 1  │→ │ Batch 2  │→ │ Batch N││
  │  │ (10K rows)│  │ (10K rows)│  │   ...  ││
  │  └──────────┘  └──────────┘  └────────┘│
  └─────────────────────────────────────────┘
                    │
                    │ Group by repo
                    ▼
  ┌─────────────────────────────────────────┐
  │      Per-Repo Processing                │
  │  ┌───────────────────────────────────┐│
  │  │ 1. Run detection.py on commits    ││
  │  │ 2. Compute rollup                 ││
  │  │ 3. Write Postgres (DELETE+INSERT) ││
  │  │ 4. Write ARMOR (Parquet artifact) ││
  │  └───────────────────────────────────┘│
  └─────────────────────────────────────────┘
                    │
                    ▼
           migration_progress table
                    │
                    ▼
              Resumption Support
```

## Component Architecture

### 1. `streaming_reader.py` - Low-level Streaming API

**Purpose**: Provides RecordBatchReader-based streaming of encrypted Parquet partitions.

**Key Classes**:
- `PartitionStream` - Streams a single partition batch-by-batch
- `CorpusStream` - Enumerates and streams all partitions

**Critical Properties**:
- Never materializes whole partitions
- Memory usage bounded by `batch_size` (default 10K rows)
- Compatible with encrypted Parquet (direct-B2 encryption)

**API Contract**:
```python
# ONLY these APIs are allowed in migration path:
PartitionStream.iter_batches()    # Yields RecordBatch objects
PartitionStream.iter_rows()        # Convenience wrapper, single-row Tables
CorpusStream.iter_partitions()     # Yields (partition_key, PartitionStream)
CorpusStream.iter_all_batches()    # Flattened iteration over all batches

# FORBIDDEN APIs (OOM risk):
pq.read_table()                    # Materializes whole table
pq.read_pandas()                   # Materializes as DataFrame
reader.read_all()                  # Materializes all batches
.fetchall()                        # Database fetchall equivalent
```

### 2. `migrate_corpus.py` - Migration Orchestrator

**Purpose**: Coordinates the full migration process with progress tracking and resumption.

**Key Classes**:
- `CorpusMigrator` - Main orchestrator
- `EncryptionKey` - Represents encryption epoch keys
- `MigrationProgress` - Tracks per-partition progress

**Migration Flow**:
1. **Encryption Key Discovery**
   - Walk all partition manifests
   - Collect all `key_id` values
   - Validate migration credentials can decrypt all epochs

2. **Partition Streaming**
   - Load `migration_progress` for resumption
   - For each uncompleted partition:
     - Mark `in_progress`
     - Stream batches via `RecordBatchReader`
     - Group commits by repo
     - Process each repo (detection + rollup + writes)
     - Mark `completed`

3. **Progress Tracking**
   - `migration_progress(partition_key, completed_at, total_repos, processed_repos, status)`
   - Supports resumption from interruption
   - Enables idempotence testing

### 3. Memory Usage Guarantees

**Per-Batch Memory**:
- `batch_size` rows × average row size
- Default: 10K rows × ~500 bytes/row ≈ 5MB per batch
- Tunable via `batch_size` parameter

**Total Memory**:
- Bounded by: batch size + processing overhead
- Independent of partition size (critical for large partitions)
- Independent of corpus size (critical for 76M commits)

**Comparison**:
```
Old pattern (materialize):
  Memory = partition_size (400K rows × 500 bytes ≈ 200MB per partition)
  At 76M scale: multiple partitions in parallel → OOM

New pattern (streaming):
  Memory = batch_size (10K rows × 500 bytes ≈ 5MB)
  At 76M scale: constant 5MB, independent of scale
```

## Encryption and Partitioning

### Hive Partition Structure

```
corpus/
├── provider=github/
│   ├── year=2024/
│   │   ├── month=08/
│   │   │   ├── part-00000.parquet.encrypted
│   │   │   ├── part-00001.parquet.encrypted
│   │   │   └── _manifest
│   │   └── month=07/
│   └── year=2023/
├── provider=gitlab/
└── provider=bitbucket/
```

### Encryption Epochs

- Partitions are encrypted with epoch keys (rotated over time)
- Migration must discover ALL epochs from manifests
- Validating against current epoch only would silently skip older partitions

### Compatibility

- The streaming API works with encrypted Parquet (direct-B2 encryption)
- No decryption step needed - pyarrow handles encrypted reads
- Each partition's `_manifest` contains encryption metadata

## Per-Repo Processing

For each repo in a partition, the migration:

1. **Runs `shared/detection.py`**
   - Python-based detection (not reimplemented in SQL)
   - Two sources of truth would drift
   - Already proven at scale

2. **Computes Rollup**
   - `(user, repo, tool, day, count)` per AI-tool-tagged commit
   - Groups by repo from streamed batches
   - No counter updates (replace-only pattern)

3. **Writes Postgres**
   - Same `DELETE + bulk INSERT` pattern as live clone-worker
   - Ensures migration and live traffic are logically identical

4. **Writes ARMOR**
   - Per-repo Parquet artifact (sha, author, email, date, message)
   - Enables redetect jobs without re-cloning
   - Uniform mechanism for old and new repos

## Progress Tracking and Resumption

### `migration_progress` Table

```sql
CREATE TABLE migration_progress (
    partition_key TEXT PRIMARY KEY,           -- e.g., "provider=github/year=2024/month=08"
    completed_at TIMESTAMPTZ,                 -- NULL if not completed
    total_repos INT NOT NULL,                 -- Total repos in partition
    processed_repos INT NOT NULL DEFAULT 0,    -- Repos processed so far
    status TEXT NOT NULL DEFAULT 'pending',   -- 'pending' | 'in_progress' | 'completed' | 'failed'
    started_at TIMESTAMPTZ,                   -- When partition processing started
    error_message TEXT                        -- Error details if status='failed'
);
```

### Resumption Flow

1. Load `migration_progress` for each partition
2. Skip partitions with `status='completed'`
3. Resume partitions with `status='in_progress'` or `'pending'`
4. Enables recovery from multi-hour job interruption

## Idempotence

**Requirement**: Migration must be idempotent - running twice produces identical rollup.

**Validation**:
- Postgres `DELETE + INSERT` pattern is replace-only by construction
- No counter updates (removed in v2 redesign)
- Test: Run migration twice, assert rollup is identical after second run

**Why This Matters**:
- Enables recovery from failures
- Validates that migration logic is correct
- Ensures no double-counting or data corruption

## Integration Points

### Postgres Schema
```sql
-- Rollup table (AI commits only)
CREATE TABLE repo_user_daily_tool (
    repo_id BIGINT NOT NULL REFERENCES repos(repo_id),
    user_id BIGINT NOT NULL REFERENCES users(user_id),
    tool TEXT NOT NULL,
    day DATE NOT NULL,
    commits INT NOT NULL,
    insert_time TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (repo_id, user_id, tool, day)
);

-- Progress tracking
CREATE TABLE migration_progress (
    -- see above
);
```

### ARMOR Storage
- Per-repo Parquet artifacts
- Warm-start artifacts (separate from commit-history)
- No direct B2 SDK calls (all via ARMOR)

### Detection Module
- `shared/detection.py` reused as-is
- Multi-tool detection catalog
- No reimplementation in SQL

## Performance Considerations

### Batch Size Tuning
- **Default**: 10,000 rows
- **Smaller batches**: Lower memory, more iterations
- **Larger batches**: Higher memory, fewer iterations
- **Tuning target**: Balance memory vs. iteration overhead

### Concurrency
- Partition processing is embarrassingly parallel
- Repo processing within partition is sequential (per-partition transaction)
- Multiple migrator instances can run on different partitions

### Network I/O
- Streaming reduces memory but not network
- Still reads entire partition data over network
- Consider caching for repeated migrations (testing)

## Testing Strategy

1. **Unit Tests**
   - Streaming API behavior
   - Progress tracking
   - Resumption logic

2. **Integration Tests**
   - Small corpus (few partitions)
   - Idempotence validation
   - Encryption credential validation

3. **Scale Tests**
   - Large partition (100K+ commits)
   - Memory usage stays bounded
   - Duration at corpus scale

## Migration Runtime Estimate

Given:
- 76.6M commits total
- ~6.6 commits/row (rollup ratio)
- ~11.6M rollup rows expected
- Partition size varies by provider/year/month

Estimate (rough):
- 100 commits/sec processing (detection + rollup + writes)
- 76.6M commits ÷ 100/sec ≈ 766,000 sec ≈ 212 hours ≈ 9 days

With parallel migrators (4 instances):
- 9 days ÷ 4 ≈ 2-3 days

**Note**: This is a one-time cost. Live traffic is incremental.

## Rollback and Recovery

Since the old pipeline is torn down (as of 2026-08-05), there is no rollback to the old system.

**Recovery Options**:
1. **Restore Postgres from backup** - Rehearsed RTO from Phase 0
2. **Re-run migration** - Idempotent, safe to re-run
3. **Resume from interruption** - `migration_progress` enables resumption

**No Recovery Option**:
- Switch back to old pipeline (doesn't exist)

## Critical Success Criteria

1. ✓ **No `fetchall()` or whole-partition materialization** - Validated via code review
2. ✓ **Works with `provider/year/month` layout** - Tested against real corpus structure
3. ✓ **Memory usage bounded at scale** - Measured during scale test
4. ✓ **Supports resumption** - Validated via `migration_progress` table
5. ✓ **Idempotent** - Validated via test: run twice, assert identical output
6. ✓ **Handles all encryption epochs** - Validated via key discovery

## References

- Plan: `docs/plan/plan.md` - "Corpus migration (inherit, don't rediscover)"
- OOM Incident: Predecessor OOM'd 2Gi pod on 400K commits
- Schema: Postgres schema with `repo_user_daily_tool` rollup table
- Detection: `shared/detection.py` - Multi-tool detection catalog
