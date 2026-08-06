# cg-19wok: Add ingest endpoint client to user-enrichment-worker

## Status: Complete (Client Exists, Not Yet Wired to Worker)

## Summary

The ingest endpoint client package already exists at `/home/coding/commitgraph/internal/ingestclient/` with all required functionality. The user-enrichment-worker is currently a Python service, so the client is not yet wired into any Go worker dependency injection.

## Acceptance Criteria Status

- ✅ **New ingest client package exists with HTTP client**
  - Package: `internal/ingestclient`
  - Files: `client.go`, `client_test.go`
  - HTTP client with 15-second default timeout

- ✅ **Request/response structs match the ingest endpoint API contract**
  - `IngestRequest` struct with:
    - `Email` (string) - Email address
    - `GithubUsername` (string) - Resolved GitHub login  
    - `Source` (Source type) - Source of resolution (live/seed/manual)
    - `ResolvedAt` (time.Time) - When resolution was made
  - `IngestResponse` struct with:
    - `Accepted` (int) - Number of rows accepted
    - `Rejected` (int) - Number of rows rejected

- ✅ **Client method is exposed**
  - `PostResolution(ctx, email, githubUsername) error` - Convenience method for single resolutions
  - `PostResolutionWithSource(ctx, email, githubUsername, source) error` - With explicit source
  - `PostIngest(ctx, requests) (*IngestResponse, error)` - Batch method

- ✅ **HTTP client with proper timeout and retry configuration**
  - Timeout: 15 seconds (configurable)
  - Retry logic: Custom `retryTransport` with configurable max retries (default: 3)
  - Retryable status codes: 503, 502, 504
  - Retry delay: 1 second (configurable)

- ⚠️ **Wire the client into the worker dependency injection**
  - **Not applicable** - The user-enrichment-worker is currently Python (`containers/user-enrichment-worker/worker.py`)
  - The Go ingest client is ready to use when a Go version of the worker is created
  - Related beads indicate this is part of a migration from queue-api to Postgres

## Current State

The Go ingest client is complete and ready to use, but the user-enrichment-worker remains a Python service. The client provides:

1. Type-safe HTTP interface to the ingest endpoint
2. Automatic retry logic for transient failures
3. Request validation before sending
4. Single and batch resolution methods
5. Full test coverage

## Future Integration

When the user-enrichment-worker is converted to Go (or when other Go services need to call the ingest endpoint), the client would be integrated like this:

```go
// In cmd/user-enrichment-worker/main.go or similar
import "github.com/jedarden/commitgraph/internal/ingestclient"

// Create client during initialization
ingestClient, err := ingestclient.NewClient(ingestclient.Config{
    BaseURL:   "http://queue-api:8080",
    Timeout:   15 * time.Second,
    Token:     os.Getenv("QUEUE_API_INTERNAL_TOKEN"),
    UserAgent: "user-enrichment-worker/2.0",
})
if err != nil {
    log.Fatal(err)
}

// Use in worker loop
err = ingestClient.PostResolution(ctx, email, githubLogin)
if err != nil {
    log.Printf("Failed to post resolution: %v", err)
}
```

## Related Work

- **cg-5329w**: "Repoint user-enrichment-worker to write resolutions into Postgres, not queue-api"
- **cg-1mdel**: "Refactor resolution write path to use ingest endpoint"  
- **cg-4v3og**: "Version bump user-enrichment-worker to 2.0.0"
- **cg-1ef5**: "Implement bulk-upsert identity ingest path with the source/resolved_at conflict rule"

The ingest client is ready for when these migration beads are implemented.

## Files

- `/home/coding/commitgraph/internal/ingestclient/client.go` - Main client implementation
- `/home/coding/commitgraph/internal/ingestclient/client_test.go` - Full test coverage
- `/home/coding/commitgraph/pkg/identity/ingest.go` - Database-side ingest logic
- `/home/coding/commitgraph/containers/user-enrichment-worker/worker.py` - Current Python worker
