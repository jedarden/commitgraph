# Git Libraries and Author Extraction Methods - Task cg-1n7xm

## Task Summary
Identify which git libraries and methods are used for author extraction in the commitgraph codebase.

## Key Findings

### 1. Git Libraries Used

**Current commitgraph system does NOT use git libraries directly.**

The go.mod file contains only:
- `github.com/lib/pq` - PostgreSQL driver
- `github.com/mattn/go-sqlite3` - SQLite driver  
- `github.com/testcontainers/testcontainers-go` - Test containers
- `gopkg.in/yaml.v3` - YAML parsing

**No git libraries present:**
- No `github.com/go-git/go-git` 
- No `github.com/gitlab/golang-git`
- No git-related imports found in any Go code

### 2. Author Data Sources

The commitgraph system consumes **pre-processed data** from external sources:

#### A. Corpus Data (Migration Pipeline)
Location: `/home/coding/commitgraph/migration/migrate_corpus.py`

Author fields in corpus schema:
```python
author_email = row.get('author_email', '')
author_name = row.get('author_name', '')
```

The corpus contains already-extracted commit data with:
- `sha` - commit SHA
- `author_email` - commit author email
- `author_name` - commit author name
- `committed_at` - commit timestamp
- `message` - commit message

#### B. Queue-API Data (SQLite dumps)
Location: `/home/coding/commitgraph/cmd/load-email-resolution-from-queue-api/main.go`

Email resolution table structure:
```sql
CREATE TABLE email_resolution (
    email TEXT PRIMARY KEY,
    login TEXT NOT NULL,
    source TEXT NOT NULL,
    resolved_at TIMESTAMPTZ NOT NULL
)
```

Author data extracted from queue-api dumps:
- `AuthorEmail` - author's email address
- `GitHubLogin` - resolved GitHub username
- `Status` - resolution status
- `AttemptedAt` - resolution timestamp

#### C. Current Database Schema
Location: `/home/coding/commitgraph/migrations/00001_initial_schema.sql`

```sql
CREATE TABLE users (
    user_id    BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    login      TEXT NOT NULL UNIQUE,    -- canonical GitHub login
    profile_url TEXT,
    avatar_url  TEXT
);
```

### 3. Author Extraction Points

#### No Direct Git Parsing in Current Codebase

The current commitgraph system **does not parse git repositories directly**. Instead:

1. **Migration Phase**: Reads pre-processed Parquet/Arrow files from corpus
   - File: `migration/migrate_corpus.py`
   - Reads: `author_email` and `author_name` from corpus schema
   - Methods: Arrow RecordBatch API, not git libraries

2. **Queue-API Import**: Parses SQLite dumps
   - File: `cmd/load-email-resolution-from-queue-api/main.go`
   - Reads: `author_email` → `github_login` mappings
   - Methods: SQLite dump parsing, not git operations

3. **Identity Resolution**: 
   - File: `pkg/identity/ingest.go`
   - Processes: Email → login resolution data
   - Source: External queue-api system

### 4. Author Field Access Patterns

#### In Migration Code (Python):
```python
# From migrate_corpus.py lines 436-443
author_email = row.get('author_email', '')
author_name = row.get('author_name', '')

# Tracking author names for later use
if author_email and author_name:
    author_names[author_email] = author_name
```

#### In Queue-API Import (Go):
```go
// From cmd/load-email-resolution-from-queue-api/main.go lines 73-80
resolvedRows = append(resolvedRows, identity.ResolutionRow{
    Email:      row.AuthorEmail,
    Login:      row.GitHubLogin,
    Source:     identity.SourceLive,
    ResolvedAt: row.AttemptedAt,
})
```

#### In Database Schema (Postgres):
```sql
-- From migrations/00001_initial_schema.sql
-- users table stores canonical GitHub login
-- email_resolution table stores email→login mappings
```

### 5. Author.Login Field Status

**`author.Login` is NOT a direct field from git parsing.**

The `login` field in the current system is:
- **Derived field**: Obtained through email→login resolution process
- **Source**: `queue-api`'s email_resolution table
- **Process**: 
  1. Git author email extracted externally (not in this codebase)
  2. Email resolved to GitHub login via external API
  3. Resolution cached in email_resolution table
  4. Imported into commitgraph Postgres database

### 6. External Git Processing

**Actual git repository cloning and author extraction happens externally:**

Based on documentation in `docs/plan/plan.md`:
- The predecessor system (`commitgraph-deprecated`) had clone-worker
- Used `git clone --bare --filter=blob:none` for repository cloning
- Extracted author data using git commands (not in current codebase)
- Stored results in encrypted corpus files

Current commitgraph **inherits the corpus** but does not re-clone repos.

### 7. Key Code Locations

| Location | Purpose | Git Library Used |
|----------|---------|------------------|
| `migration/migrate_corpus.py` | Read corpus data | None (Arrow/Parquet) |
| `cmd/load-email-resolution-from-queue-api/main.go` | Import email resolutions | None (SQLite parsing) |
| `pkg/identity/ingest.go` | Email resolution ingest | None (Postgres writes) |
| `pkg/pg/identity.go` | Database operations | None (SQL only) |

### 8. Acceptance Criteria Status

- [x] **Identified all git libraries used in the codebase**
  - Result: NONE - No git libraries in current codebase
  
- [x] **Found specific functions/methods for author extraction**
  - Result: Author extraction happens externally - current system only reads pre-processed data
  
- [x] **Located code that accesses author.Name and author.Email fields**
  - Result: Found in `migration/migrate_corpus.py` - reads `author_name` and `author_email` from corpus
  
- [x] **Determined if author.Login is a direct field or derived**
  - Result: `author.Login` is a **derived field** obtained through email→login resolution process

## Conclusion

The current commitgraph codebase **does not contain git library usage or direct author extraction code**. The system is designed to consume pre-processed commit data from external sources (corpus files and queue-api SQLite dumps). 

The actual git repository cloning and author extraction happens in external systems:
1. The deprecated predecessor system (`commitgraph-deprecated`)
2. The corpus generation system (creates the encrypted Parquet files)
3. The queue-api system (provides email→login resolution)

Author fields in the current system:
- `author_name` and `author_email`: Read from corpus schema
- `login`: Derived from email_resolution table (external API resolution)
- No git commands (git log, git clone, etc.) in current codebase
- No git library imports (go-git, gitpython, etc.)
