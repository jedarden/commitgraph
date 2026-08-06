# cg-5cqf: clone-worker Argo Workflows Build Implementation

## Task Summary
Add clone-worker build to Argo Workflows (iad-ci), not GitHub Actions.

## Implementation

### Files Created
1. **WorkflowTemplate**: `k8s/iad-ci/argo-workflows/commitgraph-clone-worker-build-workflowtemplate.yml`
   - Builds the commitgraph clone-worker Docker image
   - Reads semver from `containers/clone-worker/VERSION`
   - Tags image with exact version (never :latest)
   - Auto-bumps patch version if VERSION not changed
   - Uses Kaniko for daemonless Docker builds

2. **Sensor**: `k8s/iad-ci/argo-events/commitgraph-clone-worker-build-sensor.yml`
   - Triggers workflow on push to main branch
   - Filters for changes under `containers/clone-worker/`
   - Submits workflow via Argo Events

## Acceptance Criteria Status

- ✅ **WorkflowTemplate lives in declarative-config**: Committed to `jedarden/declarative-config`
- ✅ **Build tags with exact VERSION semver**: Line 128 uses `{{inputs.parameters.version}}`, never :latest
- ✅ **Template triggers on containers/clone-worker/ changes**: Sensor filters with regex `^containers/clone-worker/.*`
- ⏳ **Manual workflow submission**: Requires valid iad-ci credentials (cluster access currently unavailable)

## Workflow Features

### Version Resolution
- Clones repo and checks if VERSION changed in the commit
- If VERSION changed: uses the new version
- If VERSION not changed and not a CI auto-bump: auto-increments patch version
- Commits and pushes the bumped version with retry logic

### Docker Build
- Uses Kaniko executor for daemonless builds
- Context: entire repo (git clone with subpath Dockerfile)
- Destination: `ronaldraygun/commitgraph-clone-worker:{version}`
- Caching enabled for faster builds
- Retry strategy for transient failures

## Testing Notes

Due to iad-ci cluster credential issues (kubeconfig expired), the manual workflow submission could not be tested. The workflow template and sensor are syntactically correct and follow the same patterns as other working build templates in the repository:

- `commitgraph-build-workflowtemplate.yml` (multi-container build)
- `face-detection-build-workflowtemplate.yml` (single container)
- Other single-container templates in the same directory

## Verification Steps (once cluster access restored)

1. Check workflow template exists:
   ```bash
   kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig \
     get workflowtemplate -n argo-workflows commitgraph-clone-worker-build
   ```

2. Submit test workflow:
   ```bash
   kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig \
     create -f - <<EOF
   apiVersion: argoproj.io/v1alpha1
   kind: Workflow
   metadata:
     generateName: commitgraph-clone-worker-build-test-
     namespace: argo-workflows
   spec:
     serviceAccountName: argo-workflow
     workflowTemplateRef:
       name: commitgraph-clone-worker-build
   EOF
   ```

3. Monitor workflow execution:
   ```bash
   kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig \
     get workflows -n argo-workflows -l workflows.argoproj.io/workflow-template=commitgraph-clone-worker-build
   ```

## Commits
- `44840ce6`: feat(cg-5cqf): add commitgraph-clone-worker build workflow template
- `39365e9c`: feat(cg-5cqf): add commitgraph-clone-worker build sensor

Both pushed to `jedarden/declarative-config` main branch.
