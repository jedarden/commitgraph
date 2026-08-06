"""
Example: Streaming Corpus Migration

This demonstrates the exact pattern for streaming corpus reads without
materializing whole partitions.

CRITICAL RULES:
- ✓ USE: RecordBatchReader iter_batches()
- ✓ USE: Process batch immediately, don't accumulate
- ✗ FORBIDDEN: read_table(), read_pandas(), fetchall()
- ✗ FORBIDDEN: Accumulating all batches in a list

The migration path must follow this pattern exactly to avoid OOM at scale.
"""

from streaming_reader import CorpusStream, PartitionStream
from migrate_corpus import CorpusMigrator
import logging

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


def example_partition_stream():
    """
    Example: Stream a single partition batch-by-batch.
    """
    partition_path = "provider=github/year=2024/month=08"

    stream = PartitionStream(partition_path, batch_size=10000)

    total_commits = 0
    for batch in stream.iter_batches():
        # Process this batch immediately
        for i in range(batch.num_rows):
            commit = extract_commit_from_batch(batch, i)
            process_commit(commit)

        total_commits += batch.num_rows
        logger.info(f"Processed batch: {batch.num_rows} commits (total: {total_commits})")


def example_corpus_stream():
    """
    Example: Stream entire corpus partition-by-partition.
    """
    corpus_root = "/data/corpus"

    corpus_stream = CorpusStream(corpus_root, batch_size=10000)

    for partition_key, stream in corpus_stream.iter_partitions():
        logger.info(f"Processing partition: {partition_key}")

        for batch in stream.iter_batches():
            # Process batch immediately - no accumulation
            process_batch(batch)


def example_full_migration():
    """
    Example: Full migration with progress tracking and resumption.
    """
    migrator = CorpusMigrator(
        corpus_root="/data/corpus",
        postgres_conn_string="postgresql://user:pass@host/db",
        migration_credential_path="/creds/migration.json",
        batch_size=10000
    )

    # Discover encryption keys and validate credentials
    keys = migrator.discover_encryption_keys()
    logger.info(f"Discovered {len(keys)} encryption keys")

    if not migrator.validate_encryption_credentials(keys):
        raise ValueError("Cannot decrypt all partitions")

    # Run migration (resumes from last completed partition)
    migrator.run_migration(resume=True)


def extract_commit_from_batch(batch, row_idx):
    """
    Extract a single commit from a RecordBatch row.

    This is called per-row during streaming.
    """
    # Schema: sha, author_name, author_email, committed_at, message
    # Plus provider, repo_full_name for grouping
    return {
        'sha': batch.column('sha')[row_idx].as_py(),
        'author_name': batch.column('author_name')[row_idx].as_py(),
        'author_email': batch.column('author_email')[row_idx].as_py(),
        'committed_at': batch.column('committed_at')[row_idx].as_py(),
        'message': batch.column('message')[row_idx].as_py(),
        # ... other fields
    }


def process_commit(commit):
    """
    Process a single commit (placeholder for actual detection/rollup).
    """
    # Run detection, compute rollup, etc.
    pass


def process_batch(batch):
    """
    Process an entire batch at once (more efficient than row-by-row).

    This is the preferred pattern when you need to operate on multiple
    commits at once (e.g., grouping by repo).
    """
    # Group by repo within this batch
    repos = {}
    for i in range(batch.num_rows):
        repo_full_name = batch.column('repo_full_name')[i].as_py()
        if repo_full_name not in repos:
            repos[repo_full_name] = []
        repos[repo_full_name].append(extract_commit_from_batch(batch, i))

    # Process each repo's commits from this batch
    for repo_full_name, commits in repos.items():
        process_repo_commits(repo_full_name, commits)


def process_repo_commits(repo_full_name, commits):
    """
    Process commits for a single repo.

    This is where detection.py runs and rollups are computed.
    """
    # TODO: Integrate with detection.py
    # TODO: Write to Postgres (DELETE + bulk INSERT pattern)
    # TODO: Write ARMOR artifact (Parquet)
    pass


# ----------------------------------------------------------------------------
# FORBIDDEN PATTERNS - DO NOT USE
# ----------------------------------------------------------------------------

def forbidden_materialize_all():
    """
    ❌ FORBIDDEN: Materializes entire partition in memory.

    This pattern caused OOM incidents in the predecessor.
    DO NOT USE.
    """
    import pyarrow.parquet as pq

    partition_path = "provider=github/year=2024/month=08"

    # ❌ WRONG - materializes whole partition
    table = pq.read_table(partition_path)
    for i in range(table.num_rows):
        # table holds ALL rows in memory
        commit = extract_row_from_table(table, i)
        process_commit(commit)


def forbidden_fetchall():
    """
    ❌ FORBIDDEN: Fetches all rows at once.

    This pattern caused OOM incidents in the predecessor.
    DO NOT USE.
    """
    # ❌ WRONG - fetches all rows
    # all_commits = some_query.fetchall()
    pass


# ----------------------------------------------------------------------------
# VALIDATION CHECKLIST
# ----------------------------------------------------------------------------

def validate_streaming_behavior(func):
    """
    Use this decorator to validate that a function never calls
    materializing APIs in the migration path.

    This is a safety check to prevent accidental OOM.
    """
    def wrapper(*args, **kwargs):
        # TODO: Inspect call stack for forbidden APIs
        # - pq.read_table
        # - pq.read_pandas
        # - reader.read_all
        # - .fetchall()
        return func(*args, **kwargs)
    return wrapper


if __name__ == "__main__":
    # Run the example
    print("Example: streaming corpus migration")
    print("\nSee example_corpus_stream() and example_full_migration()")
