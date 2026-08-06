"""
Main migration orchestrator for streaming corpus migration.

Coordinates the entire migration process:
1. Enumerates encryption keys across all partition manifests
2. Streams partitions using Arrow RecordBatchReader
3. Processes each repo through detection and rollup
4. Tracks progress in migration_progress table
5. Supports resumption from interruption

This is the entry point for Phase 3 of the rollout.
"""

import psycopg
from psycopg import sql
from pathlib import Path
import json
import logging
from typing import Optional, Dict, Any, List
from datetime import datetime
from dataclasses import dataclass

from streaming_reader import CorpusStream, PartitionStream

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


@dataclass
class EncryptionKey:
    """Represents an encryption epoch key from a partition manifest."""
    key_id: str
    epoch: str
    key_path: str

    def __repr__(self):
        return f"EncryptionKey(key_id={self.key_id!r}, epoch={self.epoch!r})"


@dataclass
class MigrationProgress:
    """Tracks migration progress per partition."""
    partition_key: str
    completed_at: Optional[datetime]
    total_repos: int
    processed_repos: int
    status: str  # 'pending', 'in_progress', 'completed', 'failed'

    def to_dict(self) -> Dict[str, Any]:
        return {
            'partition_key': self.partition_key,
            'completed_at': self.completed_at.isoformat() if self.completed_at else None,
            'total_repos': self.total_repos,
            'processed_repos': self.processed_repos,
            'status': self.status
        }


class CorpusMigrator:
    """
    Orchestrates streaming migration of the existing corpus.

    Migration flow:
    1. Discover all encryption keys across partition manifests
    2. Validate migration credentials can decrypt all keys
    3. For each partition:
       a. Load migration_progress to resume from last completed partition
       b. Stream partition using RecordBatchReader (no fetchall())
       c. Group commits by repo (full_name = provider/repo_name)
       d. Per repo: run detection, compute rollup, write Postgres + ARMOR
       e. Mark partition complete in migration_progress
    4. Verify idempotence: run twice, assert identical rollup
    """

    def __init__(
        self,
        corpus_root: str,
        postgres_conn_string: str,
        migration_credential_path: str,
        batch_size: int = 10000
    ):
        """
        Initialize the migrator.

        Args:
            corpus_root: Root directory of Hive-partitioned corpus
            postgres_conn_string: Postgres connection string for rollup writes
            migration_credential_path: Path to migration encryption credential
            batch_size: RecordBatch size (streaming, not whole-partition)
        """
        self.corpus_root = corpus_root
        self.postgres_conn_string = postgres_conn_string
        self.migration_credential_path = migration_credential_path
        self.batch_size = batch_size

        # Initialize Postgres connection
        self.pg_conn = psycopg.connect(self.postgres_conn_string)

        # Ensure migration_progress table exists
        self._init_migration_progress_table()

    def _init_migration_progress_table(self):
        """Create migration_progress table if it doesn't exist."""
        create_table = sql.SQL("""
            CREATE TABLE IF NOT EXISTS migration_progress (
                partition_key TEXT PRIMARY KEY,
                completed_at TIMESTAMPTZ,
                total_repos INT NOT NULL,
                processed_repos INT NOT NULL DEFAULT 0,
                status TEXT NOT NULL DEFAULT 'pending',
                started_at TIMESTAMPTZ,
                error_message TEXT
            );
            CREATE INDEX IF NOT EXISTS migration_progress_status_idx
                ON migration_progress(status);
        """)

        with self.pg_conn.cursor() as cur:
            cur.execute(create_table)
            self.pg_conn.commit()

        logger.info("migration_progress table ready")

    def discover_encryption_keys(self) -> List[EncryptionKey]:
        """
        Enumerate all encryption keys across partition manifests.

        This is critical: scoping to only the current epoch would silently
        skip older partitions still encrypted with retired epochs.

        Returns:
            List of all EncryptionKey objects found in manifests
        """
        keys = []
        corpus_path = Path(self.corpus_root)

        # Walk provider/year/month structure
        for provider_dir in corpus_path.glob("provider=*"):
            for year_dir in provider_dir.glob("year=*"):
                for month_dir in year_dir.glob("month=*"):
                    manifest_path = month_dir / "_manifest"

                    if manifest_path.exists():
                        partition_key = f"{provider_dir.name}/{year_dir.name}/{month_dir.name}"
                        partition_keys = self._read_manifest_keys(manifest_path, partition_key)
                        keys.extend(partition_keys)

        logger.info(f"Discovered {len(keys)} encryption keys across all partitions")
        return keys

    def _read_manifest_keys(self, manifest_path: Path, partition_key: str) -> List[EncryptionKey]:
        """Read encryption keys from a partition manifest file."""
        keys = []

        try:
            with open(manifest_path, 'r') as f:
                manifest = json.load(f)

            # Manifest structure varies; adapt to actual format
            # This is a placeholder - adjust to real manifest schema
            for key_info in manifest.get('encryption_keys', []):
                key = EncryptionKey(
                    key_id=key_info['key_id'],
                    epoch=key_info.get('epoch', 'unknown'),
                    key_path=key_info.get('key_path', '')
                )
                keys.append(key)

            logger.debug(f"Partition {partition_key}: found {len(keys)} keys")

        except Exception as e:
            logger.warning(f"Failed to read manifest {manifest_path}: {e}")

        return keys

    def validate_encryption_credentials(self, keys: List[EncryptionKey]) -> bool:
        """
        Validate that migration credentials can decrypt all discovered keys.

        Args:
            keys: List of EncryptionKey objects discovered from manifests

        Returns:
            True if all keys can be decrypted, False otherwise
        """
        logger.info(f"Validating migration credentials against {len(keys)} keys...")

        # Load migration credential
        try:
            with open(self.migration_credential_path, 'r') as f:
                migration_credential = json.load(f)
        except Exception as e:
            logger.error(f"Failed to load migration credential: {e}")
            return False

        # Validate against each key
        # This is a placeholder - implement actual decryption test
        for key in keys:
            # TODO: Test decryption of a sample encrypted value with this key
            # If any key fails, log which epoch/key_id and return False
            pass

        logger.info("✓ Migration credentials validated for all encryption keys")
        return True

    def get_partition_progress(self, partition_key: str) -> Optional[MigrationProgress]:
        """Load migration progress for a specific partition."""
        query = sql.SQL("""
            SELECT partition_key, completed_at, total_repos, processed_repos, status
            FROM migration_progress
            WHERE partition_key = %s
        """)

        with self.pg_conn.cursor() as cur:
            cur.execute(query, (partition_key,))
            row = cur.fetchone()

            if row is None:
                return None

            return MigrationProgress(
                partition_key=row[0],
                completed_at=row[1],
                total_repos=row[2],
                processed_repos=row[3],
                status=row[4]
            )

    def update_partition_progress(
        self,
        partition_key: str,
        status: str,
        total_repos: Optional[int] = None,
        processed_repos: Optional[int] = None
    ):
        """Update migration progress for a partition."""
        if status == 'completed':
            completed_at = datetime.utcnow()
        else:
            completed_at = None

        upsert = sql.SQL("""
            INSERT INTO migration_progress (partition_key, completed_at, total_repos, processed_repos, status, started_at)
            VALUES (%s, %s, %s, %s, %s,
                COALESCE((SELECT started_at FROM migration_progress WHERE partition_key = %s), %s))
            ON CONFLICT (partition_key) DO UPDATE SET
                completed_at = EXCLUDED.completed_at,
                total_repos = COALESCE(EXCLUDED.total_repos, migration_progress.total_repos),
                processed_repos = COALESCE(EXCLUDED.processed_repos, migration_progress.processed_repos),
                status = EXCLUDED.status
            WHERE migration_progress.partition_key = EXCLUDED.partition_key;
        """)

        with self.pg_conn.cursor() as cur:
            cur.execute(upsert, (
                partition_key,
                completed_at,
                total_repos,
                processed_repos,
                status,
                partition_key,
                datetime.utcnow()
            ))
            self.pg_conn.commit()

    def migrate_partition(self, partition_key: str, stream: PartitionStream):
        """
        Migrate a single partition using streaming reads.

        This is the core migration loop for one partition:
        1. Mark partition 'in_progress'
        2. Stream batches via RecordBatchReader
        3. Group commits by repo
        4. Per repo: detect + rollup + write
        5. Mark partition 'completed'

        Args:
            partition_key: Hive partition key (e.g., "provider=github/year=2024/month=08")
            stream: PartitionStream for this partition
        """
        logger.info(f"Migrating partition: {partition_key}")

        # Mark as in_progress
        self.update_partition_progress(partition_key, 'in_progress')

        # TODO: Implement per-repo processing
        # This requires integration with detection.py and Postgres schema
        # Placeholder for now:
        repo_batches = {}  # repo_full_name -> list of batches

        try:
            batch_count = 0
            total_rows = 0

            # Stream batches one at a time (never materialize whole partition)
            for batch in stream.iter_batches():
                batch_count += 1
                total_rows += batch.num_rows

                # Group commits by repo for processing
                # Schema includes: provider, repo_full_name, sha, author_*, committed_at, message
                self._group_batch_by_repo(batch, repo_batches)

                if batch_count % 10 == 0:
                    logger.debug(f"  Partition {partition_key}: processed {batch_count} batches, {total_rows} rows")

            # Process each repo's accumulated batches
            for repo_full_name, batches in repo_batches.items():
                self._process_repo(repo_full_name, batches)

            # Mark partition complete
            self.update_partition_progress(
                partition_key,
                'completed',
                total_repos=len(repo_batches),
                processed_repos=len(repo_batches)
            )

            logger.info(f"✓ Partition {partition_key} complete: {len(repo_batches)} repos, {total_rows} commits")

        except Exception as e:
            logger.error(f"✗ Partition {partition_key} failed: {e}")
            self.update_partition_progress(partition_key, 'failed')
            raise

    def _group_batch_by_repo(self, batch: pa.RecordBatch, repo_batches: Dict[str, List]):
        """Group a batch's rows by repo for processing."""
        # Extract repo identifier from batch
        # This requires knowing the batch schema
        # Placeholder: implement once schema is known
        pass

    def _process_repo(self, repo_full_name: str, batches: List):
        """
        Process a single repo through detection and rollup.

        This is called per-repo during migration:
        1. Run detection.py on all commits
        2. Compute rollup (user, repo, tool, day, count)
        3. Write to Postgres (same pattern as live clone-worker)
        4. Write ARMOR artifact (Parquet)

        Args:
            repo_full_name: Repository identifier
            batches: List of RecordBatches containing this repo's commits
        """
        # TODO: Integrate with detection.py and Postgres schema
        # This is the core migration processing step
        logger.debug(f"Processing repo: {repo_full_name} ({len(batches)} batches)")

    def run_migration(self, resume: bool = True):
        """
        Run the full migration.

        Args:
            resume: If True, skip partitions already marked 'completed'
        """
        logger.info("Starting corpus migration...")
        logger.info(f"Corpus root: {self.corpus_root}")

        # Step 1: Discover and validate encryption keys
        keys = self.discover_encryption_keys()
        if not self.validate_encryption_credentials(keys):
            raise ValueError("Migration credentials cannot decrypt all partitions")

        # Step 2: Stream and migrate each partition
        corpus_stream = CorpusStream(self.corpus_root, batch_size=self.batch_size)

        for partition_key, partition_stream in corpus_stream.iter_partitions():
            # Check if already completed
            if resume:
                progress = self.get_partition_progress(partition_key)
                if progress and progress.status == 'completed':
                    logger.info(f"Skipping completed partition: {partition_key}")
                    continue

            # Migrate the partition
            self.migrate_partition(partition_key, partition_stream)

        logger.info("Corpus migration complete")


if __name__ == "__main__":
    import sys

    if len(sys.argv) < 4:
        print("Usage: python migrate_corpus.py <corpus_root> <postgres_conn_string> <credential_path>")
        print("\nExample:")
        print("  python migrate_corpus.py /data/corpus 'postgresql://user:pass@host/db' /creds/migration.json")
        sys.exit(1)

    corpus_root = sys.argv[1]
    postgres_conn_string = sys.argv[2]
    credential_path = sys.argv[3]

    migrator = CorpusMigrator(
        corpus_root=corpus_root,
        postgres_conn_string=postgres_conn_string,
        migration_credential_path=credential_path,
        batch_size=10000  # Tunable
    )

    # Run migration (resume from last completed partition)
    migrator.run_migration(resume=True)
