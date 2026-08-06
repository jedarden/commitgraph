# Migration Module

This module handles streaming migration of the existing encrypted Hive-partitioned corpus (direct-B2, `provider/year/month` layout) into the new architecture.

## Critical Design Constraints

- **Streaming-only**: No `fetchall()`, no whole-partition materialization
- **Arrow Batch API**: Uses `pyarrow.RecordBatchReader` for streaming reads
- **OOM prevention**: A prior incident OOM'd a 2Gi pod materializing 400K commits' message bodies at once; this must not be repeated at 76M scale

## Architecture

The migration streams the existing corpus partition-by-partition, re-runs detection, and writes to both Postgres (rollup) and ARMOR (per-repo Parquet artifacts).

## Module Structure

- `streaming_reader.py` - Low-level Arrow RecordBatchReader wrapper for encrypted partitions
- `migrate_corpus.py` - Main migration orchestrator with progress tracking
- `process_repo.py` - Per-repo processing (detection + rollup computation)
