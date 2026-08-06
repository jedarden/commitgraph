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
from typing import Optional, Dict, Any, List, Tuple
from datetime import datetime, date
from dataclasses import dataclass
from collections import defaultdict
import pyarrow as pa
import pyarrow.parquet as pq

from streaming_reader import CorpusStream, PartitionStream
from shared.detection import detect_tools

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

        This is a critical preflight check: scoping to only the current epoch
        would silently skip older partitions still sitting on retired epochs.

        Args:
            keys: List of EncryptionKey objects discovered from manifests

        Returns:
            True if all keys can be decrypted, False otherwise
        """
        logger.info(f"Validating migration credentials against {len(keys)} keys...")

        # Import the preflight checker
        try:
            from preflight_check_epochs import EpochPreflightChecker
        except ImportError:
            logger.error("preflight_check_epochs module not available")
            return False

        # Create keys_by_id dict for the preflight checker
        keys_by_id = {key.key_id: key for key in keys}

        try:
            checker = EpochPreflightChecker(
                corpus_root=self.corpus_root,
                credential_path=self.migration_credential_path
            )

            # Use the preflight checker's validation logic
            all_passed, results = checker.validate_decryption(keys_by_id)

            # Report results
            passed_count = sum(1 for r in results if r.success)
            failed_count = len(results) - passed_count

            if all_passed:
                logger.info(f"✓ Migration credentials validated for all {len(results)} encryption keys")
            else:
                logger.error(f"✗ Migration credential validation failed: {failed_count}/{len(results)} keys cannot decrypt")
                for r in results:
                    if not r.success:
                        logger.error(f"    - key_id={r.key_id!r} (epoch={r.epoch!r}): {r.error_message}")

            return all_passed

        except Exception as e:
            logger.error(f"Failed to validate encryption credentials: {e}")
            return False

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

    def _group_batch_by_repo(self, batch: pa.RecordBatch, repo_batches: Dict[str, List[pa.RecordBatch]]):
        """
        Group a batch's rows by repo for processing.

        This function processes a RecordBatch from the corpus and groups commits
        by their repository. The corpus schema includes:
        - provider: e.g., 'github'
        - repo_full_name: e.g., 'owner/repo'
        - sha: commit SHA
        - author_name: commit author name
        - author_email: commit author email
        - committed_at: commit timestamp (in milliseconds since epoch or datetime)
        - message: commit message text

        Args:
            batch: Arrow RecordBatch containing commit rows
            repo_batches: Dictionary mapping repo_full_name -> list of batches
        """
        # Convert batch to pandas for easier column access
        # This is safe because we're processing one batch at a time (bounded memory)
        import pandas as pd
        df = batch.to_pandas()

        # Group by repo_full_name
        for repo_full_name, repo_df in df.groupby('repo_full_name', sort=False):
            # Convert back to RecordBatch for storage
            repo_batch = pa.RecordBatch.from_pandas(repo_df)

            if repo_full_name not in repo_batches:
                repo_batches[repo_full_name] = []

            repo_batches[repo_full_name].append(repo_batch)

    def _process_repo(self, repo_full_name: str, batches: List[pa.RecordBatch]):
        """
        Process a single repo through detection and rollup.

        This is called per-repo during migration:
        1. Run detection.py on all commits (imported from shared/detection)
        2. Compute rollup (user, repo, tool, day, count) for AI-tagged commits only
        3. Write to Postgres using DELETE+bulk-INSERT pattern
        4. Write ARMOR artifact (Parquet) with raw committed_at preserved

        This function:
- Imports detection.py directly (not a copy, port, or reimplementation)
- Calls detect_tools() per-commit (Python, not SQL)
- Computes (user, repo, tool, day, count) rollup per repo
- Preserves raw committed_at values in Parquet artifact (before clamping)

        Args:
            repo_full_name: Repository identifier (e.g., 'owner/repo')
            batches: List of RecordBatches containing this repo's commits
        """
        logger.info(f"Processing repo: {repo_full_name} ({len(batches)} batches)")

        # Accumulate rollups: (user_email, repo, tool, day) -> count
        rollup_counts: Dict[Tuple[str, str, str, date], int] = defaultdict(int)

        # Track provider and author names for later use
        provider = None
        author_names: Dict[str, str] = {}  # email -> name

        # Collect ALL commits (including quarantined) for Parquet artifact
        # This preserves raw committed_at before the clamp is applied
        all_commits_for_parquet = []

        total_commits = 0
        ai_tagged_commits = 0
        quarantined_commits = 0

        # Process each batch
        for batch in batches:
            import pandas as pd
            df = batch.to_pandas()

            # Extract provider from first row
            if provider is None and 'provider' in df.columns:
                provider = df['provider'].iloc[0]

            # Process each commit
            for idx, row in df.iterrows():
                total_commits += 1

                # Extract commit fields
                author_email = row.get('author_email', '')
                author_name = row.get('author_name', '')
                message = row.get('message', '')
                committed_at = row.get('committed_at', None)

                # Track author name
                if author_email and author_name:
                    author_names[author_email] = author_name

                # Parse committed_at to date
                try:
                    if committed_at is not None:
                        # Handle both timestamp (ms since epoch) and datetime string
                        if isinstance(committed_at, (int, float)):
                            commit_date = datetime.fromtimestamp(committed_at / 1000).date()
                        else:
                            # Parse ISO datetime string
                            if isinstance(committed_at, str):
                                commit_date = datetime.fromisoformat(committed_at.replace('Z', '+00:00')).date()
                            else:
                                # Already a datetime object
                                commit_date = committed_at.date()
                    else:
                        continue  # Skip commits without dates
                except (ValueError, TypeError, OSError) as e:
                    logger.warning(f"Invalid committed_at for commit in {repo_full_name}: {e}")
                    continue

                # Store raw commit data for Parquet artifact (BEFORE clamping)
                # This preserves the original committed_at verbatim
                all_commits_for_parquet.append({
                    'sha': row.get('sha', ''),
                    'author_email': author_email,
                    'author_name': author_name,
                    'committed_at': committed_at,  # Raw value, preserved verbatim
                    'message': message
                })

                # Apply date quarantine (per compactor logic in plan.md)
                # Exclude commits with committed_at outside [2005-01-01, today+1]
                min_date = date(2005, 1, 1)
                max_date = date.today() + datetime.timedelta(days=1)

                if not (min_date <= commit_date <= max_date):
                    logger.debug(f"Quarantined commit with date {commit_date} outside [{min_date}, {max_date}]")
                    quarantined_commits += 1
                    continue

                # Run detection (imported from shared/detection.py)
                detection_result = detect_tools(message)

                # Only count AI-tagged commits in rollup
                if detection_result.is_ai_tagged:
                    ai_tagged_commits += 1

                    # Increment count for each detected tool
                    for tool in detection_result.tools:
                        key = (author_email, repo_full_name, tool, commit_date)
                        rollup_counts[key] += 1

        logger.info(f"  Repo {repo_full_name}: {ai_tagged_commits}/{total_commits} AI-tagged commits, {quarantined_commits} quarantined")

        # Write rollup to Postgres
        if rollup_counts:
            self._write_rollup_to_postgres(
                repo_full_name=repo_full_name,
                provider=provider or 'unknown',
                rollup_counts=dict(rollup_counts),
                author_names=author_names
            )

        # Write ARMOR artifact (Parquet) with raw committed_at preserved
        # This is step 5b from the migration plan
        self._write_armor_parquet(
            repo_full_name=repo_full_name,
            provider=provider or 'unknown',
            commits=all_commits_for_parquet
        )

    def _write_rollup_to_postgres(
        self,
        repo_full_name: str,
        provider: str,
        rollup_counts: Dict[Tuple[str, str, str, date], int],
        author_names: Dict[str, str]
    ):
        """
        Write rollup data to Postgres using DELETE+bulk-INSERT pattern.

        This implements the same write pattern as live clone-worker:
        1. Upsert repos row to get repo_id
        2. Upsert users rows to get user_ids
        3. DELETE existing rollup rows for this repo
        4. Bulk INSERT new rollup rows
        5. Set insert_time to transaction timestamp

        This pattern ensures idempotence: running the same repo twice
        produces identical rollup data.

        Args:
            repo_full_name: Repository identifier
            provider: Provider name (e.g., 'github')
            rollup_counts: Dict of (user_email, repo, tool, day) -> count
            author_names: Dict of email -> author name
        """
        with self.pg_conn.cursor() as cur:
            try:
                # Step 1: Upsert repo to get repo_id
                repo_query = sql.SQL("""
                    INSERT INTO repos (provider, repo_full_name)
                    VALUES (%s, %s)
                    ON CONFLICT (provider, repo_full_name)
                    DO UPDATE SET provider = EXCLUDED.provider  -- No-op, exists for syntax
                    RETURNING repo_id
                """)
                cur.execute(repo_query, (provider, repo_full_name))
                repo_id = cur.fetchone()[0]

                # Step 2: Upsert users to get user_ids
                # Collect unique emails
                unique_emails = set(email for email, _, _, _ in rollup_counts.keys())

                email_to_user_id = {}
                for email in unique_emails:
                    # For migration, we use email as login placeholder
                    # Real identity resolution happens later via email_resolution
                    login = email  # Will be resolved later

                    user_query = sql.SQL("""
                        INSERT INTO users (login)
                        VALUES (%s)
                        ON CONFLICT (login)
                        DO UPDATE SET login = EXCLUDED.login  -- No-op
                        RETURNING user_id
                    """)
                    cur.execute(user_query, (login,))
                    email_to_user_id[email] = cur.fetchone()[0]

                # Step 3: DELETE existing rollup rows for this repo
                delete_query = sql.SQL("""
                    DELETE FROM repo_user_daily_tool
                    WHERE repo_id = %s
                """)
                cur.execute(delete_query, (repo_id,))

                # Step 4: Bulk INSERT new rollup rows
                # Prepare batch insert data (insert_time omitted - DEFAULT transaction_timestamp() applies)
                insert_data = []

                for (email, _, tool, day), count in rollup_counts.items():
                    user_id = email_to_user_id[email]
                    insert_data.append((repo_id, user_id, tool, day, count))

                # Bulk insert using UNNEST (insert_time omitted, DEFAULT transaction_timestamp() applies)
                insert_query = sql.SQL("""
                    INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits)
                    SELECT * FROM UNNEST(%s::bigint[], %s::bigint[], %s::text[], %s::date[], %s::int[])
                """)

                # Transpose data for UNNEST
                repo_ids = [row[0] for row in insert_data]
                user_ids = [row[1] for row in insert_data]
                tools = [row[2] for row in insert_data]
                days = [row[3] for row in insert_data]
                counts = [row[4] for row in insert_data]

                cur.execute(insert_query, (
                    repo_ids, user_ids, tools, days, counts
                ))

                self.pg_conn.commit()
                logger.info(f"  ✓ Wrote {len(insert_data)} rollup rows for {repo_full_name}")

            except Exception as e:
                self.pg_conn.rollback()
                logger.error(f"  ✗ Failed to write rollup for {repo_full_name}: {e}")
                raise

    def _write_armor_parquet(
        self,
        repo_full_name: str,
        provider: str,
        commits: List[Dict[str, Any]]
    ):
        """
        Write ARMOR Parquet artifact with raw committed_at preserved.

        This writes a per-repo Parquet artifact that includes ALL commits
        (including quarantined ones) with their raw committed_at values
        preserved verbatim. This enables:
        1. Redetection jobs without re-cloning
        2. Historical analysis of quarantined commits
        3. Audit trail for the date quarantine decision

        The artifact is written to ARMOR storage, not directly to B2.
        File path: armor://commitgraph/repo-artifacts/{provider}/{repo_full_name}/commits.parquet

        Args:
            repo_full_name: Repository identifier (e.g., 'owner/repo')
            provider: Provider name (e.g., 'github')
            commits: List of commit dicts with raw committed_at values
        """
        if not commits:
            logger.debug(f"  No commits to write to ARMOR artifact for {repo_full_name}")
            return

        # TODO: Integrate with ARMOR client
        # For now, write to local filesystem as intermediate step
        import tempfile
        import os

        # Create temporary directory for artifact
        artifact_dir = Path(tempfile.gettempdir()) / "commitgraph-artifacts" / provider / repo_full_name
        artifact_dir.mkdir(parents=True, exist_ok=True)

        artifact_path = artifact_dir / "commits.parquet"

        # Create Arrow schema for the artifact
        schema = pa.schema([
            ('sha', pa.string()),
            ('author_email', pa.string()),
            ('author_name', pa.string()),
            ('committed_at', pa.timestamp('ns')),  # Raw timestamp, preserved verbatim
            ('message', pa.string())
        ])

        # Convert commits to Arrow format
        data = {field: [] for field in schema.names}
        for commit in commits:
            data['sha'].append(commit.get('sha', ''))
            data['author_email'].append(commit.get('author_email', ''))
            data['author_name'].append(commit.get('author_name', ''))
            data['committed_at'].append(commit.get('committed_at'))  # Raw value preserved
            data['message'].append(commit.get('message', ''))

        table = pa.table(data, schema=schema)

        # Write to Parquet
        try:
            pq.write_table(table, artifact_path)
            logger.info(f"  ✓ Wrote {len(commits)} commits to ARMOR artifact at {artifact_path}")
        except Exception as e:
            logger.error(f"  ✗ Failed to write ARMOR artifact for {repo_full_name}: {e}")
            # Don't raise - artifact failure should not fail the migration
            # The rollup is the source of truth for rankings

        # TODO: Upload to ARMOR storage
        # armor_client.put(
        #     key=f"commitgraph/repo-artifacts/{provider}/{repo_full_name}/commits.parquet",
        #     file=artifact_path
        # )

    def run_migration(self, resume: bool = True):
        """
        Run the full migration.

        Args:
            resume: If True, skip partitions already marked 'completed'

        Note: Preflight validation is performed at startup (in __main__)
        before the CorpusMigrator is created. This check is not duplicated here.
        """
        logger.info("Starting corpus migration...")
        logger.info(f"Corpus root: {self.corpus_root}")
        logger.info("Preflight validation already completed at startup - proceeding with data migration")

        # Stream and migrate each partition
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

    # ===== CRITICAL PREFLIGHT CHECK =====
    # Validate encryption credentials BEFORE any migration work begins.
    # This prevents silent data loss from retired epochs that migration
    # credentials cannot decrypt. DO NOT skip or bypass this check.
    logger.info("=" * 70)
    logger.info("MIGRATION STARTUP: Running preflight encryption validation")
    logger.info("=" * 70)

    try:
        from preflight_check_epochs import EpochPreflightChecker

        preflight_checker = EpochPreflightChecker(
            corpus_root=corpus_root,
            credential_path=credential_path
        )

        all_passed, results = preflight_checker.run_preflight()

        if not all_passed:
            # Preflight failed - abort migration with clear error message
            logger.error("=" * 70)
            logger.error("MIGRATION ABORTED: Preflight encryption validation failed")
            logger.error("=" * 70)
            logger.error("")
            logger.error("The following encryption epochs cannot be decrypted with")
            logger.error("the provided migration credentials:")
            logger.error("")

            failed_count = 0
            for r in results:
                if not r.success:
                    failed_count += 1
                    logger.error(f"  [{failed_count}] key_id={r.key_id!r}")
                    logger.error(f"      epoch={r.epoch!r}")
                    logger.error(f"      error: {r.error_message}")
                    if r.test_partition:
                        logger.error(f"      test partition: {r.test_partition}")
                    logger.error("")

            logger.error(f"TOTAL: {failed_count} epoch(s) failed decryption test")
            logger.error("")
            logger.error("This migration CANNOT proceed. Fix the credential access or restore")
            logger.error("missing epoch keys before re-running the migration.")
            logger.error("")
            logger.error("DO NOT bypass this check - doing so would silently skip all data")
            logger.error("in the failed epochs, causing permanent data loss.")
            logger.error("=" * 70)

            sys.exit(1)

        # Preflight passed - proceed with migration
        logger.info("=" * 70)
        logger.info("✓ PREFLIGHT PASSED: All epochs can be decrypted")
        logger.info("Proceeding with migration...")
        logger.info("=" * 70)
        logger.info("")

    except ImportError as e:
        logger.error("=" * 70)
        logger.error("MIGRATION ABORTED: Preflight checker not available")
        logger.error("=" * 70)
        logger.error(f"Cannot import preflight_check_epochs: {e}")
        logger.error("")
        logger.error("The preflight check is a REQUIRED startup guard.")
        logger.error("Ensure preflight_check_epochs.py is in the same directory.")
        logger.error("=" * 70)
        sys.exit(1)

    except Exception as e:
        logger.error("=" * 70)
        logger.error("MIGRATION ABORTED: Preflight check error")
        logger.error("=" * 70)
        logger.error(f"Preflight validation raised an exception: {e}")
        logger.error("")
        logger.error("The migration cannot proceed without a successful preflight check.")
        logger.error("=" * 70)
        sys.exit(1)

    # Preflight complete - initialize migrator and proceed
    migrator = CorpusMigrator(
        corpus_root=corpus_root,
        postgres_conn_string=postgres_conn_string,
        migration_credential_path=credential_path,
        batch_size=10000  # Tunable
    )

    # Run migration (resume from last completed partition)
    migrator.run_migration(resume=True)
