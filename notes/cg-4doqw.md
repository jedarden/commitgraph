# Git Commit Parsing Entry Points - cg-4doqw

## Executive Summary

**Finding**: This codebase does NOT parse raw git commit objects. Commits are already parsed and stored in an external Parquet corpus on B2. The "parsing" that happens here is simply reading columns from Parquet rows.

## Architecture Overview

```
External Git Repos → (Parsed elsewhere) → Encrypted Parquet Corpus on B2
                                                  ↓
                                          commitgraph Go code
                                          (reads Parquet columns)
```

## Key Finding: No Git Object Parsing

The codebase has:
- **No git library dependencies** (checked go.mod - no go-git, gitobj, etc.)
- **No `git log` or `git cat-file` commands** for parsing
- **No commit object byte-parsing code**

Instead, commits are **already parsed** and stored in a Hive-partitioned encrypted Parquet corpus on Backblaze B2.

## Commit Data Flow

### 1. External Corpus (Source of Truth)

The corpus is stored on B2 in Hive-partitioned format:

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

**Important**: The actual git commit parsing happens BEFORE this corpus is created. The corpus is the output of some external parsing process not in this codebase.

### 2. Migration Entry Point

**File**: `/home/coding/commitgraph/migration/migrate_corpus.py`

**Key Function**: `CorpusMigrator._process_repo()` (lines 400-513)

This is where commit fields are **extracted** (not parsed from git objects):

```python
# Line 435-439: Extract commit fields from Parquet row
author_email = row.get('author_email', '')
author_name = row.get('author_name', '')
message = row.get('message', '')
committed_at = row.get('committed_at', None)
```

The `row` here comes from:
```python
# Line 425: Convert RecordBatch to pandas
df = batch.to_pandas()

# Line 432: Iterate over rows
for idx, row in df.iterrows():
```

Where `batch` is a PyArrow `RecordBatch` from the encrypted Parquet file.

### 3. Streaming Architecture

**File**: `/home/coding/commitgraph/migration/streaming_reader.py`

**Key Classes**:
- `PartitionStream` - Streams a single partition batch-by-batch
- `CorpusStream` - Enumerates and streams all partitions

**Critical**: Uses `RecordBatchReader` to avoid OOM - never materializes whole partitions in memory.

## Commit Object Structure

### Schema from Parquet Corpus

The commit fields available in the Parquet corpus are:

| Field        | Type            | Description                              |
|--------------|-----------------|------------------------------------------|
| `provider`   | string          | Git provider (github, gitlab, etc.)      |
| `repo_full_name` | string     | Repository (owner/name)                   |
| `sha`        | string          | Commit SHA hash                          |
| `author_name`  | string        | Author name                              |
| `author_email` | string        | Author email                             |
| `committed_at` | datetime/int   | Commit timestamp (ms since epoch or ISO) |
| `message`    | string          | Commit message                           |

### Internal Go Struct

**File**: `/home/coding/commitgraph/pkg/rollup/rollup.go`

**Struct**: `Commit` (simplified for rollup purposes)

```go
type Commit struct {
    SHA         string    // Commit SHA
    AuthorEmail string    // Author email (for identity resolution)
    AuthorName  string    // Author name
    CommittedAt time.Time // Commit date (UTC)
    Message     string    // Commit message
    Tools       []string  // Detected AI tools (empty if no AI tool detected)
}
```

This is NOT the parsed git object structure - it's a simplified struct for rollup aggregation.

## Commit Field Extraction Points

### 1. Python Migration (Primary)

**Location**: `migration/migrate_corpus.py:435-439`

```python
author_email = row.get('author_email', '')
author_name = row.get('author_name', '')
message = row.get('message', '')
committed_at = row.get('committed_at', None)
# sha is extracted at line 467 for ARMOR artifact
```

### 2. Rollup Aggregation

**Location**: `migration/migrate_corpus.py:446-457`

**Date Parsing Logic**:

```python
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
```

### 3. ARMOR Artifact Creation

**Location**: `migration/migrate_corpus.py:466-472`

```python
all_commits_for_parquet.append({
    'sha': row.get('sha', ''),
    'author_email': author_email,
    'author_name': author_name,
    'committed_at': committed_at,  # Raw value, preserved verbatim
    'message': message
})
```

## Key Insights

### 1. No Git Object Parsing

The codebase never calls:
- `git cat-file -p <sha>` (to read commit objects)
- `git log --format=` (to parse commits)
- Any git library to parse raw commit bytes

### 2. Pre-Parsed Corpus

The commits come **already parsed** from an external system:
- The corpus is the **source of truth** for commit data
- This codebase only **reads** from the corpus
- Parsing happens upstream (not in this repository)

### 3. Parquet as Storage Format

The corpus uses encrypted Parquet because:
- Columnar storage for efficient filtering
- Encryption at rest on B2
- Hive partitioning for provider/year/month layout
- Streaming via PyArrow RecordBatchReader (OOM-safe)

### 4. Two "Parsing" Layers

There are two types of "parsing" in the codebase:

#### Layer 1: Git Object Parsing (NOT in this codebase)
- Raw git commit objects → parsed fields
- Happens **externally** before corpus creation
- Not visible in this codebase

#### Layer 2: Parquet Row Reading (in this codebase)
- Parquet rows → Python dictionaries/Go structs
- Simple column extraction (not git parsing)
- Lines 435-439 in `migrate_corpus.py`

## Acceptance Criteria - Completed

- [x] **Found commit parsing functions**: The "parsing" happens at `migrate_corpus.py:435-439` where Parquet row columns are extracted
- [x] **Identified the main parsing entry points**: 
  - Primary: `CorpusMigrator._process_repo()` in `migration/migrate_corpus.py`
  - Streaming: `PartitionStream.iter_batches()` in `migration/streaming_reader.py`
- [x] **Documented the commit object structure and key fields**: Documented the Parquet schema (7 fields) and the internal Go `Commit` struct

## Related Files

### Primary Entry Points
- `migration/migrate_corpus.py` - Main migration orchestrator
- `migration/streaming_reader.py` - Parquet streaming API
- `pkg/rollup/rollup.go` - Internal Go Commit struct

### Architecture Documentation
- `migration/ARCHITECTURE.md` - Streaming corpus migration architecture
- `docs/parsing-error-catalog.md` - All parsing error locations in codebase

## Next Steps (If You Need Actual Git Parsing)

If you need to parse raw git commit objects (not from Parquet), you would need to:

1. **Add a git library dependency** to `go.mod`:
   ```go
   import github.com/go-git/go-git/v5
   ```

2. **Create a git parser function** (example):
   ```go
   func ParseCommitObject(hash plumbing.Hash) (*Commit, error) {
       // Use go-git to read and parse commit object
   }
   ```

3. **Wire it into the ingest pipeline** before corpus creation

However, **this is not needed** for the current architecture since the corpus is already the source of truth.
