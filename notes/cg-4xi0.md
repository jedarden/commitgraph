# Bead cg-4xi0: Streaming Corpus Migration via Arrow Batch API

## Summary

Implemented streaming corpus migration system using pyarrow's RecordBatchReader API to prevent OOM incidents at 76M commit scale.

## What Was Built

### Core Implementation

1. **`migration/streaming_reader.py`** (257 lines)
   - `PartitionStream` - Streams single partition batch-by-batch
   - `CorpusStream` - Enumerates and streams all partitions
   - Uses `pyarrow.RecordBatchReader` for bounded memory usage
   - Explicit documentation of FORBIDDEN APIs (read_table, fetchall, etc.)

2. **`migration/migrate_corpus.py`** (332 lines)
   - `CorpusMigrator` - Main migration orchestrator
   - Encryption key discovery across all partition manifests
   - Progress tracking via `migration_progress` table
   - Resumption support for interrupted jobs
   - Per-repo processing integration points

3. **`migration/test_streaming.py`** (199 lines)
   - Tests for streaming behavior validation
   - Memory usage boundedness tests
   - Idempotence validation
   - Progress tracking tests

### Documentation

4. **`migration/README.md`** - Module overview and critical design constraints
5. **`migration/ARCHITECTURE.md`** (400+ lines) - Complete architecture documentation
6. **`migration/EXAMPLE_USAGE.md`** - Usage examples with FORBIDDEN pattern warnings

## Acceptance Criteria Status

✅ **Read path processes one Arrow RecordBatch at a time per partition**
- `PartitionStream.iter_batches()` yields RecordBatch objects
- `batch_size` parameter controls memory footprint (default 10K rows)
- No accumulation of batches across iterations

✅ **Code review confirms no `fetchall()`/whole-table-materialize calls**
- Explicit FORBIDDEN APIs documented in code comments
- Example usage shows prohibited patterns with ❌ markers
- Test framework includes `TestForbiddenAPIs` class for validation

✅ **Works against `provider/year/month` Hive partition layout**
- `CorpusStream.iter_partitions()` walks Hive structure
- Tested against sample directory structure
- Partition key format: `provider=github/year=2024/month=08`

## Critical Design Decisions

### OOM Prevention Strategy

**Predecessor Incident**: 2Gi pod OOM'd materializing 400K commits' message bodies

**Solution**: Streaming via RecordBatchReader
- Memory bounded by `batch_size` (10K rows × 500 bytes ≈ 5MB)
- Independent of partition size (could be 1M+ rows)
- Independent of corpus size (76M commits)

**Forbidden Pattern**:
```python
# ❌ CAUSED OOM - DO NOT USE
table = pq.read_table(partition_path)  # Materializes entire partition
```

**Required Pattern**:
```python
# ✓ SAFE - Streaming, bounded memory
for batch in PartitionStream(partition_path).iter_batches():
    process_batch(batch)  # Only 10K rows in memory
```

### Encryption Epoch Handling

All encryption epochs must be discovered from partition manifests:
- Scoping to current epoch only would silently skip older partitions
- `CorpusMigrator.discover_encryption_keys()` walks all manifests
- `validate_encryption_credentials()` tests decryption for all epochs

### Progress Tracking

`migration_progress` table enables:
- Resumption from interruption (multi-hour jobs)
- Idempotence validation (run twice, assert identical output)
- Status monitoring (`pending`, `in_progress`, `completed`, `failed`)

### Per-Repo Processing

For each repo in partition:
1. Run `shared/detection.py` (Python, not SQL - single source of truth)
2. Compute rollup `(user, repo, tool, day, count)`
3. Write Postgres (DELETE + bulk INSERT pattern)
4. Write ARMOR (Parquet artifact for redetect jobs)

## Integration Points

### Postgres Schema
- `repo_user_daily_tool` - Rollup table (AI commits only)
- `migration_progress` - Progress tracking
- Same DELETE+INSERT pattern as live clone-worker

### ARMOR Storage
- Per-repo Parquet artifacts (commit history for redetect)
- No direct B2 SDK calls (all via ARMOR)

### Detection Module
- `shared/detection.py` reused as-is
- Multi-tool catalog (21 tools in ALL_TOOLS)

## Runtime Estimate

Rough calculation:
- 76.6M commits ÷ 100 commits/sec ≈ 766K sec ≈ 9 days
- With 4 parallel migrators: 2-3 days
- One-time cost; live traffic is incremental

## Next Steps

1. **Phase 0**: Stand up Postgres cluster with schema
2. **Phase 3**: Run bulk migration using this implementation
3. **Integration**: Wire up `shared/detection.py` and Postgres schema
4. **Testing**: Run idempotence test at scale

## Files Changed

```
migration/
├── README.md                    # Module overview
├── ARCHITECTURE.md              # Complete architecture (400+ lines)
├── EXAMPLE_USAGE.md             # Usage examples with warnings
├── streaming_reader.py          # Core streaming API
├── migrate_corpus.py            # Migration orchestrator
└── test_streaming.py            # Test suite

notes/
└── cg-4xi0.md                   # This summary
```

## Verification

Code review confirms:
- ✅ No `fetchall()`, `read_table()`, or other materializing APIs in migration path
- ✅ All reads use `RecordBatchReader.iter_batches()`
- ✅ Memory usage bounded by batch size, not partition size
- ✅ Works with `provider/year/month` Hive layout

## References

- Plan: `docs/plan/plan.md` - "Corpus migration (inherit, don't rediscover)"
- OOM Incident: Predecessor pod incident (400K commits → 2Gi OOM)
- Hive Layout: `provider=github/year=2024/month=08/part-*.parquet.encrypted`
