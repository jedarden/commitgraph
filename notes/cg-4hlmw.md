# Author Login Extraction Points

## Task: Identify where author logins are extracted from commit data

## Summary

Author login extraction happens through **two separate mechanisms** in the commitgraph codebase:

### 1. GitHub API-Based Resolution (Primary Method)

**File:** `containers/user-enrichment-worker/worker.py`

**Function:** `lookup_github_login(email: str)` (lines 90-118)

**Process:**
- Takes an author email address
- First checks for GitHub noreply addresses: `{id}+{login}@users.noreply.github.com`
  - Extracts login directly from the email: `local.split("+", 1)[1]` (line 80)
  - Returns immediately with zero API cost
- For other emails, calls GitHub Search API:
  - Endpoint: `https://api.github.com/search/commits` (line 99)
  - Query parameter: `author-email:{email}` (line 104)
  - Headers: `Authorization: token {GITHUB_TOKEN}`, `Accept: application/vnd.github.cloak-preview+json` (lines 101-102)
- **Extraction point:** Parses API response and extracts login:
  ```python
  items = r.json().get("items", [])
  if items and items[0].get("author"):
      return items[0]["author"].get("login"), True
  ```
  (lines 108-110)

**Key characteristics:**
- Login is extracted from GitHub API response field `items[0]["author"]["login"]`
- One API call per unique email (expensive, hence the caching layer)
- Results are stored in `email_resolution` table for reuse

### 2. Direct Email Parsing (Zero-Cost Method)

**File:** `containers/user-enrichment-worker/worker.py`

**Function:** `_extract_login_from_noreply(email: str)` (lines 74-82)

**Process:**
- Parses GitHub noreply email format: `{id}+{login}@users.noreply.github.com`
- Extracts login by splitting on `+` and taking the second part
- Returns login immediately without API call

**Key characteristics:**
- Zero API cost (local parsing)
- Only works for GitHub noreply addresses
- Bot logins are preserved as-is (e.g., `dependabot[bot]`)

### 3. Commit Author Field Detection Context

**File:** `containers/clone-worker/detection.py`

**Functions:**
- `detect_tools(author_email, author_name, coauthor_trailer, commit_message)` (lines 108-158)
- `extract_coauthor_emails(commit_message)` (lines 212-220)

**Context:**
- This is **AI tool detection**, not login extraction
- Receives pre-extracted `author_email` and `author_name` as parameters
- Uses git log format that extracts these fields (see comment line 126)
- **Note:** The actual git log extraction happens upstream (likely in a clone/scanning component not in this codebase)

## Connection Point to Database

**File:** `containers/user-enrichment-worker/worker.py`

**Function:** `record_resolution(email, login)` (lines 139-157)

**Process:**
- POSTs resolved login to ingest endpoint
- Endpoint: `{INGEST_URL}/identity/ingest` (line 145)
- Payload includes:
  - `email`: Author email
  - `github_username`: Resolved login (or null for negative cache)
  - `source`: "live" (from live enrichment worker)
  - `resolved_at`: Current UTC timestamp

**Database table:** `email_resolution` (see schema files in `exports/*.sql`)
- `author_email TEXT PRIMARY KEY`
- `github_username TEXT` (the resolved login)
- `source TEXT` ('live', 'seed', 'manual')
- `resolved_at TIMESTAMPTZ`

## What Was NOT Found

No direct git parsing code in this codebase that extracts author fields from commit objects using:
- `git log` format strings (e.g., `%an`, `%ae`, `%aE`)
- `git show` commands
- Go git libraries (e.g., `go-git`)
- Direct object database parsing

**Reason:** This appears to be the **redesign** codebase (`commitgraph v2`). The README indicates:
- The predecessor system was torn down on 2026-08-05
- This is a redesign that collapses the write path
- Commit extraction likely happens in a separate component or inherited system

The detection code in `containers/clone-worker/detection.py` receives pre-extracted author fields, indicating the actual git parsing happens upstream of this codebase.

## File:Line References

| Extraction Point | File | Lines | Description |
|-----------------|------|-------|-------------|
| GitHub API login extraction | `containers/user-enrichment-worker/worker.py` | 90-118 | `lookup_github_login()` function |
| API response parsing | `containers/user-enrichment-worker/worker.py` | 108-110 | Extracts `items[0]["author"]["login"]` |
| Noreply email parsing | `containers/user-enrichment-worker/worker.py` | 74-82 | `_extract_login_from_noreply()` function |
| Database ingest | `containers/user-enrichment-worker/worker.py` | 139-157 | `record_resolution()` function |
| Author field usage | `containers/clone-worker/detection.py` | 108-158 | `detect_tools()` function |
| Co-author extraction | `containers/clone-worker/detection.py` | 212-220 | `extract_coauthor_emails()` function |
| Commit struct | `pkg/rollup/rollup.go` | 56-64 | `Commit` struct with author fields |

## Git Libraries/Fields Used

**GitHub API:**
- Library: `requests` Python library
- Endpoint: `https://api.github.com/search/commits`
- Response field: `items[0]["author"]["login"]`
- Auth: Bearer token via `Authorization` header

**No direct git parsing libraries found** - author data is received pre-extracted or resolved via API.

## Acceptance Criteria Status

- [x] Located all code that extracts author logins
- [x] Identified which git libraries/fields are used
- [x] Found the connection point between commit parsing and database insert
- [x] Listed file:line references for each extraction point
