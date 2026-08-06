# cg-1nrd: Clone-worker semver versioning verification

## Task
Establish `containers/clone-worker/VERSION` with semver, no `:latest` tags

## Status: COMPLETE ✅

All acceptance criteria have been verified and met:

### 1. VERSION file exists with valid semver
- **File**: `containers/clone-worker/VERSION`
- **Content**: `0.1.0`
- **Format**: Valid semver (MAJOR.MINOR.PATCH), no leading `v`, no trailing whitespace
- **Verified**: `xxd` shows clean bytes `302e312e300a` (0.1.0 + newline only)

### 2. Dockerfile/build reads VERSION file
- **CI Workflow**: `commitgraph-build-workflowtemplate.yml` in declarative-config
- **Detection logic** (lines 96-100):
  ```bash
  if [ -f "containers/$CONTAINER/VERSION" ]; then
    VERSION=$(cat "containers/$CONTAINER/VERSION" | tr -d '[:space:]')
  else
    VERSION="latest"
  fi
  ```
- **Image tag** (line 188): `ronaldraygun/commitgraph-{{inputs.parameters.name}}:{{inputs.parameters.version}}`
- **Result**: `clone-worker` images are tagged as `ronaldraygun/commitgraph-clone-worker:0.1.0`

### 3. No :latest references for this image
- **Searched**: All YAML, JSON, shell, and Markdown files in the repo
- **Found**: No `:latest` tags referencing `clone-worker` or `commitgraph-clone-worker`
- **Note**: Build tool `gcr.io/kaniko-project/executor:latest` is used, but that's the build infrastructure, not the image being built

### 4. Version-bump procedure documented
- **Location**: `containers/clone-worker/README.md` (lines 18-50)
- **Procedure**:
  1. Update `containers/clone-worker/VERSION` with new semver
  2. Commit with message: `chore(clone-worker): bump version to X.Y.Z`
  3. Push to trigger CI
  4. Argo Workflows automatically builds and tags with the new version
- **SemVer guidelines documented**:
  - Patch (X.Y.Z+1): Bug fixes, docs, non-breaking changes
  - Minor (X.Y+1.0): New features, backward-compatible
  - Major (X+1.0.0): Breaking changes

## CI/CD Integration

The `commitgraph-build` WorkflowTemplate in `iad-ci` cluster:
- Reads `containers/clone-worker/VERSION` dynamically
- Builds with Kaniko using root-context (`--dockerfile=containers/clone-worker/Dockerfile`)
- Tags as `ronaldraygun/commitgraph-clone-worker:<VERSION>`
- Pushes to Docker Hub registry
- Verifies manifest availability post-push

## Notes

**Workflow template repository reference**: The current workflow template points to `jedarden/commitgraph-deprecated` on GitHub, but this repo exists as `git.ardenone.com/jedarden/commitgraph` with a GitHub mirror at `jedarden/commitgraph` (200 response). The workflow template comment suggests the new repo was docs-only, but `containers/clone-worker/` now exists here. This may need a separate update to the workflow template in declarative-config, but is outside the scope of this task.

## Verification Commands

```bash
# Check VERSION format
cat containers/clone-worker/VERSION
cat -A containers/clone-worker/VERSION  # Verify no trailing whitespace

# Verify no :latest references
grep -r "latest" --include="*.yaml" --include="*.yml" --include="*.json" --include="*.sh" . | grep -i clone

# Check CI workflow logic
grep -A5 "VERSION" ~/declarative-config/k8s/iad-ci/argo-workflows/commitgraph-build-workflowtemplate.yml
```

## Completed

2026-08-06: All acceptance criteria verified and met. Semver versioning is properly established for the clone-worker container.
