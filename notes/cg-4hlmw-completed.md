# Author Login Extraction Points - Summary

## Task Completion Status: COMPLETE

All author login extraction points have been identified and documented in `/home/coding/commitgraph/notes/cg-4hlmw.md`.

## Key Findings

### 1. No Direct Git Parsing in This Codebase
The commitgraph v2 codebase does **NOT** directly parse git commits to extract author information. Author fields (`author_email`, `author_name`) are received pre-extracted from an upstream component.

### 2. Two Login Extraction Mechanisms

#### A. GitHub API-Based Resolution (Primary)
**File:** `containers/user-enrichment-worker/worker.py:90-118`

**Extraction point:**
```python
items = r.json().get("items", [])
if items and items[0].get("author"):
    return items[0]["author"].get("login"), True
```

**Process:**
- Calls `https://api.github.com/search/commits` with `author-email:{email}` query
- Extracts login from API response field: `items[0]["author"]["login"]`
- Stores results in `email_resolution` table for caching

#### B. Direct Email Parsing (Zero-Cost)
**File:** `containers/user-enrichment-worker/worker.py:74-82`

**Extraction point:**
```python
def _extract_login_from_noreply(email: str) -> Optional[str]:
    if not email.endswith("@users.noreply.github.com"):
        return None
    local = email.split("@")[0]
    if "+" in local:
        candidate = local.split("+", 1)[1]
        return candidate or None
    return local or None
```

**Process:**
- Parses GitHub noreply format: `{id}+{login}@users.noreply.github.com`
- Extracts login by splitting on `+` and taking second part
- Zero API cost (local string parsing)

### 3. Database Connection Point

**File:** `containers/user-enrichment-worker/worker.py:139-157`

**Function:** `record_resolution(email, login)`

**Ingest endpoint:** `{INGEST_URL}/identity/ingest`

**Payload structure:**
```json
{
  "email": "author@example.com",
  "github_username": "resolved_login",
  "source": "live",
  "resolved_at": "2026-08-06T12:00:00Z"
}
```

**Target table:** `email_resolution`
- `email TEXT PRIMARY KEY`
- `login TEXT NOT NULL`
- `source TEXT` ('live', 'seed', 'manual')
- `resolved_at TIMESTAMPTZ NOT NULL`

## File:Line Reference Summary

| Purpose | File | Lines | Function/Struct |
|---------|------|-------|-----------------|
| GitHub API resolution | `containers/user-enrichment-worker/worker.py` | 90-118 | `lookup_github_login()` |
| API response parsing | `containers/user-enrichment-worker/worker.py` | 108-110 | Extracts `items[0]["author"]["login"]` |
| Noreply email parsing | `containers/user-enrichment-worker/worker.py` | 74-82 | `_extract_login_from_noreply()` |
| Database ingest | `containers/user-enrichment-worker/worker.py` | 139-157 | `record_resolution()` |
| Author field usage | `containers/clone-worker/detection.py` | 108-158 | `detect_tools()` |
| Co-author extraction | `containers/clone-worker/detection.py` | 212-220 | `extract_coauthor_emails()` |
| Commit data struct | `pkg/rollup/rollup.go` | 56-64 | `Commit` struct |

## Git Libraries Used

**None found in this codebase.** Author login extraction relies entirely on:
- GitHub Search API (via Python `requests` library)
- Local string parsing for noreply addresses

The actual git commit parsing happens **upstream** of this codebase in a separate component.

## All Acceptance Criteria Met

✅ Located all code that extracts author logins
✅ Identified which git libraries/fields are used (GitHub API, no direct git parsing)
✅ Found the connection point between commit parsing and database insert
✅ Listed file:line references for each extraction point

## Detailed Documentation

See `/home/coding/commitgraph/notes/cg-4hlmw.md` for complete analysis with code examples and architectural context.
