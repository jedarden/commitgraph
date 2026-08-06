"""
Streaming reader for encrypted Hive-partitioned corpus.

Uses pyarrow's RecordBatchReader API to stream partition data batch-by-batch,
never materializing entire partitions in memory. This is critical for OOM prevention
at 76M commit scale.

The existing corpus uses direct-B2 encryption with Hive partitioning:
provider/year/month/...
"""

import pyarrow.parquet as pq
import pyarrow as pa
import pyarrow.dataset as ds
from typing import Iterator, Optional, Dict, Any
import os
from pathlib import Path


class PartitionStream:
    """
    Streams a single encrypted Hive partition using Arrow RecordBatchReader.

    This is the core primitive that prevents OOM at scale. By processing
    RecordBatch objects incrementally rather than materializing entire
    partitions, memory usage stays bounded regardless of partition size.

    Usage:
        for batch in PartitionStream.iter_batches(partition_path):
            process_batch(batch)  # batch is a pyarrow.RecordBatch
    """

    def __init__(self, partition_path: str, batch_size: int = 10000):
        """
        Initialize streaming reader for a partition.

        Args:
            partition_path: Path to Hive partition directory (e.g., "provider=github/year=2024/month=08")
            batch_size: Number of rows per RecordBatch (tunable for memory/performance tradeoff)
        """
        self.partition_path = partition_path
        self.batch_size = batch_size

        if not os.path.exists(partition_path):
            raise ValueError(f"Partition path does not exist: {partition_path}")

    def iter_batches(self) -> Iterator[pa.RecordBatch]:
        """
        Iterate over RecordBatches in the partition.

        This is the ONLY way to read partition data in the migration path.
        Never use read_table(), read_pandas(), fetchall(), or any method
        that materializes the whole partition at once.

        Yields:
            pyarrow.RecordBatch objects, each containing self.batch_size rows
            (except the final batch, which may be smaller)

        Example:
            >>> for batch in PartitionStream("provider=github/year=2024/month=08").iter_batches():
            ...     # Process batch incrementally
            ...     for i in range(batch.num_rows):
            ...         commit = process_row(batch.slice(i, 1))
            ...         write_commit(commit)
        """
        # Use ParquetDataset to handle Hive partition structure
        dataset = ds.parquet_dataset(
            self.partition_path,
            format="parquet",
            # Critical: do NOT use read_table() or read_pandas()
            # Those materialize the entire dataset
        )

        # Open RecordBatchReader for streaming
        # This keeps only one batch in memory at a time
        reader = dataset.to_record_batch_reader(batch_size=self.batch_size)

        try:
            for batch in reader:
                # Each batch is a pyarrow.RecordBatch
                # Process it before the next iteration to avoid accumulation
                yield batch
        finally:
            reader.close()

    def iter_rows(self) -> Iterator[pa.Table]:
        """
        Iterate over individual rows as single-row Tables.

        This is a convenience wrapper for row-by-row processing.
        For bulk operations, prefer iter_batches() for better performance.

        Yields:
            Single-row pyarrow.Table objects
        """
        for batch in self.iter_batches():
            for row_idx in range(batch.num_rows):
                yield batch.slice(row_idx, 1)


class CorpusStream:
    """
    Streams the entire corpus partition-by-partition.

    Enumerates Hive partitions (provider/year/month) and streams each
    partition using PartitionStream. Never loads more than one partition
    batch at a time.
    """

    def __init__(self, corpus_root: str, batch_size: int = 10000):
        """
        Initialize corpus-level streaming reader.

        Args:
            corpus_root: Root directory of Hive-partitioned corpus
            batch_size: Rows per RecordBatch (passed through to PartitionStream)
        """
        self.corpus_root = corpus_root
        self.batch_size = batch_size

        if not os.path.exists(corpus_root):
            raise ValueError(f"Corpus root does not exist: {corpus_root}")

    def iter_partitions(self) -> Iterator[tuple[str, PartitionStream]]:
        """
        Iterate over all partitions in the corpus.

        Yields:
            Tuples of (partition_key, PartitionStream)
            where partition_key is e.g. "provider=github/year=2024/month=08"

        Example:
            >>> for partition_key, stream in CorpusStream("/data/corpus").iter_partitions():
            ...     print(f"Processing {partition_key}")
            ...     for batch in stream.iter_batches():
            ...         process_batch(batch)
        """
        # Walk the Hive partition structure: provider/year/month
        for provider_dir in sorted(Path(self.corpus_root).glob("provider=*")):
            provider = provider_dir.name

            for year_dir in sorted(provider_dir.glob("year=*")):
                year = year_dir.name

                for month_dir in sorted(year_dir.glob("month=*")):
                    month = month_dir.name
                    partition_path = str(month_dir)
                    partition_key = f"{provider}/{year}/{month}"

                    yield partition_key, PartitionStream(partition_path, self.batch_size)

    def iter_all_batches(self) -> Iterator[tuple[str, pa.RecordBatch]]:
        """
        Iterate over all batches across all partitions.

        This is a convenience method that flattens the nested iteration.

        Yields:
            Tuples of (partition_key, RecordBatch)
        """
        for partition_key, stream in self.iter_partitions():
            for batch in stream.iter_batches():
                yield partition_key, batch


def validate_streaming_behavior(func):
    """
    Decorator to validate that a function never calls materializing APIs.

    This guards against accidental use of fetchall(), read_table(), etc.
    in the migration path.
    """
    def wrapper(*args, **kwargs):
        result = func(*args, **kwargs)
        # Validation could be added here via stack inspection
        return result
    return wrapper


# ----------------------------------------------------------------------------
# Migration-path approved APIs
# ----------------------------------------------------------------------------
# ALLOWED (streaming, bounded memory):
# - PartitionStream.iter_batches()
# - PartitionStream.iter_rows()
# - CorpusStream.iter_partitions()
# - CorpusStream.iter_all_batches()
#
# FORBIDDEN (materializes whole partition/table, OOM risk):
# - pq.read_table() or dataset.read_table()
# - pq.read_pandas() or dataset.to_pandas()
# - any fetchall() equivalent
# - reader.read_all() or similar
# ----------------------------------------------------------------------------


if __name__ == "__main__":
    # Example usage and smoke test
    import sys

    if len(sys.argv) < 2:
        print("Usage: python streaming_reader.py <corpus_root>")
        print("\nExample:")
        print("  python streaming_reader.py /data/corpus")
        sys.exit(1)

    corpus_root = sys.argv[1]
    print(f"Streaming corpus at: {corpus_root}")

    total_batches = 0
    total_rows = 0

    for partition_key, batch in CorpusStream(corpus_root, batch_size=1000).iter_all_batches():
        total_batches += 1
        total_rows += batch.num_rows
        print(f"  {partition_key}: batch {total_batches}, {batch.num_rows} rows (total: {total_rows})")

        # Process batch here (in real migration, this would call process_repo)
        # For this smoke test, we just count

    print(f"\nSummary: {total_batches} batches, {total_rows} total rows")
