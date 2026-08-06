# cg-5xvg0: Login Revalidation Worker Implementation

## Summary

Implemented a worker that detects renamed/deleted GitHub logins and updates `email_resolution` table.

## What Was Implemented

### 1. Worker Code (containers/login-revalidation-worker/)
- **main.go**: Full Go implementation with:
  - GitHub API liveness checking (user existence, rename detection, deletion detection)
  - Rate limiting via `API_CALL_INTERVAL_SECS` (default 6 seconds = ~10 req/min)
  - Retry logic with exponential backoff for transient failures
  - Integration with `identity-ingest-endpoint` for email resolution updates
  - Status tracking: pending, validated, renamed, deleted, retry

### 2. Database Schema (migrations/00003_create_email_revalidation.sql)
- `email_revalidation` table tracks rows needing revalidation
- Indexed for efficient claiming of due rows
- Status lifecycle: pending → validated/renamed/deleted/retry

### 3. Deployment (declarative-config/k8s/ord-devimprint/commitgraph/)
- **login-revalidation-worker-deployment.yml**: Deployment manifest
  - Single replica with internal loop (no CronJob)
  - Rate limit aware: `API_CALL_INTERVAL_SECS=6` (~10 req/min, well under 30 req/min budget)
  - Resource limits: 500m CPU, 384Mi memory
  - Uses existing secrets: github-pat, queue-db-postgresql, queue-api-auth

### 4. Build Pipeline (declarative-config/k8s/iad-ci/argo-workflows/)
- **commitgraph-login-revalidation-worker-build-workflowtemplate.yml**: Argo WorkflowTemplate
  - Kaniko-based Docker build
  - Auto-versioning logic
  - Pushes to `ronaldraygun/commitgraph-login-revalidation-worker:VERSION`

## Acceptance Criteria Status

✅ **Rate limit sharing**: Uses `API_CALL_INTERVAL_SECS` env var (default 6s), same pattern as user-enrichment-worker

✅ **Rename handling**: Calls `identity-ingest-endpoint` with `source='live'` on rename detection

✅ **Deletion handling**: Sets `status='deleted'` and stops rechecking (not silent deletion)

✅ **Deployment pattern**: Uses Deployment with internal loop, not Job/CronJob

## Next Steps

### Manual Step Required: Build Docker Image

The iad-ci kubeconfig credentials are expired. The Docker image needs to be built manually after refreshing credentials:

```bash
# After refreshing iad-ci.kubeconfig credentials:
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig create -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: login-revalidation-worker-build-manual-
  namespace: argo-workflows
spec:
  workflowTemplateRef:
    name: commitgraph-login-revalidation-worker-build
EOF
```

Alternatively, trigger via Argo UI at https://argo-ci.ardenone.com

### After Build

1. ArgoCD will auto-sync the deployment (automated syncPolicy enabled)
2. Monitor for first successful run and verify email_revalidation rows are processed

## Design Decisions

- **90-day revalidation interval**: Balances freshness with API budget usage
- **5-minute retry backoff**: For transient failures (rate limits, network errors)
- **Claim batch of 50**: Reasonable throughput without hogging connections
- **Priority to pending rows**: First-time checks get priority over rechecks
