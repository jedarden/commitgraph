# cg-5xvg0: Login Revalidation Worker Implementation

## Task Description
Implement worker that detects renamed/dead logins and updates email_resolution table.

## Implementation Summary

### What Was Already Complete
The `login-revalidation-worker` already existed at `/home/coding/commitgraph/containers/login-revalidation-worker/main.go` and was mostly complete. The worker:

- ✅ Uses shared GitHub API rate limiting via `API_CALL_INTERVAL_SECS` (default 6 seconds)
- ✅ Claims rows from `email_revalidation` table using `FOR UPDATE SKIP LOCKED`
- ✅ Checks GitHub API for login liveness (200/404/403/429 handling)
- ✅ Updates `email_revalidation` table with results
- ✅ Handles terminal states (renamed/deleted) and retry logic
- ✅ Runs as an internal loop pattern suitable for Deployment

### What This Implementation Added

#### 1. Fixed Architecture Mismatch
The original code tried to call a non-existent HTTP endpoint `QUEUE_API_URL/email-resolution/resolve`. In commitgraph v2, identity resolution is done through Go packages, not HTTP endpoints.

**Fixed by:**
- Removing `QueueAPIURL` and `QueueAPIInternalToken` from configuration
- Adding imports for `github.com/jedarden/commitgraph/pkg/identity` and `github.com/jedarden/commitgraph/pkg/pg`
- Replacing HTTP endpoint call with direct database call using `identity.NewIngester` and `pg.NewIdentityIngester`

#### 2. Updated Identity Ingest Call
Changed from HTTP POST to Go package usage:
```go
// Old (incorrect):
row := ResolutionRow{
    Email:       email,
    GithubLogin: newLogin,
    Provider:    "github",
    WorkerID:    cfg.WorkerID,
    ResolvedAt:  time.Now().Format(time.RFC3339),
}
// POST to HTTP endpoint

// New (correct):
row := identity.ResolutionRow{
    Email:      email,
    Login:      newLogin,
    Source:     identity.SourceLive,  // Explicit source='live'
    ResolvedAt: time.Now().UTC(),
}
ingester.IngestResolution(ctx, []identity.ResolutionRow{row})
```

#### 3. Kubernetes Deployment Manifest
Created `/home/coding/commitgraph/k8s/login-revalidation-worker-deployment.yaml`:
- Uses Deployment with internal loop (NOT Job/CronJob per org policy)
- Single replica with Recreate strategy
- Proper resource limits (50m/64Mi requests, 500m/256Mi limits)
- Environment variables for configuration
- Probes for liveness/readiness
- Pinned image version (1.0.0) - never :latest

## Acceptance Criteria Status

- ✅ **Shared rate limiting**: Uses `API_CALL_INTERVAL_SECS` mechanism (default 6s)
- ✅ **Rename handling**: Updates `email_resolution.login` via identity ingest with `source='live'`
- ✅ **Deletion handling**: Sets `status='deleted'` and stops rechecking (NULL next_check_at)
- ✅ **Deployment with internal loop**: Created Deployment manifest following org patterns
- ✅ **ArgoCD deployment**: Ready for declarative-config deployment

## Key Design Decisions

### Why Direct Database Instead of HTTP Endpoint?
The v2 architecture uses Go packages (`pkg/identity`, `pkg/pg`) for identity resolution, not HTTP endpoints. This provides:
- Type safety (compile-time validation)
- Transaction support
- Proper conflict resolution via SQL `ON CONFLICT` clause
- Single bulk upsert path for all writers

### Why Deployment Instead of Job/CronJob?
Org-wide policy bans Jobs/CronJobs because:
- ArgoCD cannot manage them idempotently
- Their pods are not ArgoCD-owned and never get pruned
- They hold CPU/memory reservations indefinitely

Deployment with internal loop provides the same scheduling functionality while being ArgoCD-compatible.

### Rate Limiting Strategy
The worker uses a simple time-based gentlemen's agreement:
- `API_CALL_INTERVAL_SECS`: 6 seconds between GitHub API calls
- Shared budget: ~30 req/min GitHub search limit
- Conservative rate: 10 req/min per worker (6s intervals)
- Handles 403/429 responses with retry logic

## Files Modified

1. `/home/coding/commitgraph/containers/login-revalidation-worker/main.go`
   - Removed HTTP endpoint dependencies
   - Added Go identity package imports
   - Changed `updateEmailResolution()` to use direct database calls

2. `/home/coding/commitgraph/k8s/login-revalidation-worker-deployment.yaml` (NEW)
   - Kubernetes Deployment manifest
   - Environment configuration
   - Resource limits and probes

## Environment Variables

- `GITHUB_TOKEN`: GitHub API token (required)
- `POSTGRES_URL`: PostgreSQL connection string (required)
- `WORKER_ID`: Unique worker identifier (default: hostname)
- `CLAIM_BATCH`: Rows to claim per cycle (default: 50)
- `IDLE_SLEEP_SECS`: Seconds to sleep when no work (default: 60)
- `API_CALL_INTERVAL_SECS`: Seconds between GitHub API calls (default: 6)

## Deployment Steps

1. Build and push container image:
   ```bash
   docker build -t ronaldraygun/login-revalidation-worker:1.0.0 containers/login-revalidation-worker/
   docker push ronaldraygun/login-revalidation-worker:1.0.0
   ```

2. Add manifest to declarative-config repo:
   ```bash
   cp k8s/login-revalidation-worker-deployment.yaml ~/declarative-config/k8s/commitgraph/
   ```

3. ArgoCD will sync the Deployment automatically

## Testing

To test the worker locally:
```bash
export GITHUB_TOKEN="your-token"
export POSTGRES_URL="postgres://..."
go run containers/login-revalidation-worker/*.go
```

## Notes

- The worker gracefully handles all error cases and never exits on individual row failures
- Terminal states (`renamed`, `deleted`) have NULL `next_check_at` and are not rechecked
- Transient failures set status to `retry` with 5-minute backoff
- Validated logins are rechecked every 90 days
- All changes are conflict-safe via the identity ingest package's ON CONFLICT clause
