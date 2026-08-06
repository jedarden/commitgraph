# Task cg-42om8: Implement ingest endpoint call for resolution write

## Investigation Summary

This task requested implementation of an ingest endpoint call for resolution write, replacing SQLite write logic with a call to the ingest endpoint client. Upon investigation, **this work was already completed in prior beads**.

## Current Implementation Status: ✅ COMPLETE

All acceptance criteria are already met:

### ✅ PostResolution Client Method Exists

**Location:** `pkg/client/queueapi/client.go`

```go
func (c *Client) PostResolution(ctx context.Context, email, githubUsername string) error {
	// Prepare the request with source="live" and current timestamp
	req := ResolutionRequest{
		Email:       email,
		GithubLogin: githubUsername,
		Source:      "live",
		ResolvedAt:  time.Now().Format(time.RFC3339),
	}
	// ... HTTP POST to /email-resolution/resolve
}
```

### ✅ Request Includes source="live"

Line 63 of `pkg/client/queueapi/client.go`:
```go
Source: "live",
```

### ✅ Request Includes Current Timestamp for resolved_at

Line 64 of `pkg/client/queueapi/client.go`:
```go
ResolvedAt: time.Now().Format(time.RFC3339),
```

### ✅ No SQLite Write Code to Remove

As documented in prior bead `cg-5pqwf`, there is no deprecated SQLite resolution write code in the codebase. The only remaining `github.com/mattn/go-sqlite3` imports are for **reading** from source databases (seed commands), not for writing resolutions.

All resolution writes go through:
1. PostgreSQL bulk upsert (`pkg/pg/identity.go`) - used by seed commands
2. Queue-api HTTP client (`pkg/client/queueapi/client.go`) - used by workers

## Active Usage

### Login Revalidation Worker

**Location:** `containers/login-revalidation-worker/main.go` (lines 375-378)

```go
func updateEmailResolution(ctx context.Context, cfg *Config, email, newLogin string) error {
	// Use the queue-api client to post the resolution
	return cfg.QueueAPIClient.PostResolution(ctx, email, newLogin)
}
```

Called when a renamed GitHub login is detected (line 260).

### User Enrichment Worker

**Location:** `containers/user-enrichment-worker/worker.py` (lines 139-158)

Python implementation also posts to the ingest endpoint with source='live'.

## Acceptance Criteria Status

| Criterion | Status | Evidence |
|-----------|--------|----------|
| Resolved pairs posted to ingest endpoint, not SQLite | ✅ COMPLETE | `login-revalidation-worker` uses `PostResolution` |
| Request includes source="live" | ✅ COMPLETE | `pkg/client/queueapi/client.go:63` |
| Request includes current timestamp for resolved_at | ✅ COMPLETE | `pkg/client/queueapi/client.go:64` |
| Old SQLite write code removed | ✅ COMPLETE | No SQLite write code exists (documented in cg-5pqwf) |

## Conclusion

**Task is already complete.** The ingest endpoint client implementation was completed in prior work, and all code correctly uses the new pattern. No changes are needed.
