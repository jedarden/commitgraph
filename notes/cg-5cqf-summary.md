# cg-5cqf: Add clone-worker build to Argo Workflows

## Summary

Created an Argo WorkflowTemplate for building the commitgraph clone-worker container image on push to the commitgraph repository.

## Changes Made

### WorkflowTemplate Created
- **File**: `jedarden/declarative-config` → `k8s/iad-ci/argo-workflows/commitgraph-clone-worker-build-workflowtemplate.yml`
- **Template Name**: `commitgraph-clone-worker-build`
- **Target Repo**: `jedarden/commitgraph` (GitHub)
- **Image Registry**: `ronaldraygun/commitgraph-clone-worker:{version}`

### Features Implemented

1. **Version Management**
   - Reads semver from `containers/clone-worker/VERSION` file
   - Auto-bumps patch version if VERSION unchanged in commit
   - Prevents re-bumping if HEAD is a CI auto-bump commit (prevents infinite loops)
   - Tags image with exact semver (never `:latest` per task requirements)

2. **Build Process**
   - Uses Kaniko executor for building and pushing
   - Caching enabled via `ronaldraygun/cache` repo
   - Retry strategy: 2 attempts with exponential backoff
   - Resource limits: 4 CPU / 8Gi memory

3. **Trigger Configuration**
   - Triggers on changes under `containers/clone-worker/` in commitgraph repo
   - Handles concurrent builds with rebase-on-push strategy (5 attempts)

### Acceptance Criteria Status

- [x] **WorkflowTemplate in declarative-config, synced by ArgoCD app `argo-workflows-ns-iad-ci`**
  - File committed to `k8s/iad-ci/argo-workflows/`
  - Pushed to `jedarden/declarative-config` main branch
  - ArgoCD will auto-sync the template

- [x] **Build tags image with exact VERSION-file semver, never `:latest`**
  - Template uses `--destination=ronaldraygun/commitgraph-clone-worker:{{inputs.parameters.version}}`
  - No `:latest` tag included

- [x] **Triggers on changes under `containers/clone-worker/`**
  - Resolves version from `containers/clone-worker/VERSION`
  - Detects changes via git diff in resolve-version step
  - No GitHub Actions workflow file created

- [ ] **Manual/test workflow submission completes successfully**
  - STATUS: Not completed due to iad-ci kubeconfig credential issues
  - Template YAML validated syntactically
  - Ready for manual testing once cluster access restored

## Deployment Status

✅ **WorkflowTemplate pushed to declarative-config**
- Commit: `44840ce6`
- Repository: `jedarden/declarative-config`
- Path: `k8s/iad-ci/argo-workflows/commitgraph-clone-worker-build-workflowtemplate.yml`

⏳ **Pending: Manual workflow submission test**
- iad-ci kubeconfig token appears expired
- Cannot submit test workflow without valid cluster access
- Template structure validated against existing patterns

## Manual Testing Instructions

Once cluster access is restored, test with:

```bash
# Submit a manual workflow run
kubectl create -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: commitgraph-clone-worker-build-manual-
  namespace: argo-workflows
spec:
  workflowTemplateRef:
    name: commitgraph-clone-worker-build
EOF

# Monitor the workflow
kubectl get workflows -n argo-workflows -l workflows.argoproj.io/workflow-template=commitgraph-clone-worker-build

# Check logs from a specific workflow
kubectl logs -n argo-workflows <pod-name> -c main
```

## References

- Task: cg-5cqf
- CLAUDE.md section: "CI/CD — Argo Workflows (iad-ci)"
- Based on: `devimprint-clone-worker-workflowtemplate.yml` pattern
- Image target: `ronaldraygun/commitgraph-clone-worker:{VERSION}`
