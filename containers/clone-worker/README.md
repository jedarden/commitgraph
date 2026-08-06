# clone-worker Container

## Purpose

AI coding tool detection module. Identifies which AI tools produced commits by analyzing:
- Co-Authored-By trailer emails
- Author emails (bot-authored commits)
- Author name patterns
- Body text patterns

## Versioning

This container uses semantic versioning. The current version is stored in:
```
containers/clone-worker/VERSION
```

## Version Bump Procedure

To release a new version of the clone-worker container:

1. **Update the VERSION file:**
   ```bash
   # Bump patch version (bug fixes, small changes)
   echo "0.1.1" > containers/clone-worker/VERSION
   
   # Or bump minor version (new features, backward compatible)
   echo "0.2.0" > containers/clone-worker/VERSION
   
   # Or bump major version (breaking changes)
   echo "1.0.0" > containers/clone-worker/VERSION
   ```

2. **Commit the change:**
   ```bash
   git add containers/clone-worker/VERSION
   git commit -m "chore(clone-worker): bump version to 0.1.1"
   ```

3. **Push to trigger CI:**
   ```bash
   git push origin
   ```

4. **CI automatically builds and tags:**
   - Argo Workflows in `iad-ci` cluster reads the VERSION file
   - Kaniko builds the Docker image
   - Image is tagged as `ronaldraygun/clone-worker:<version>`
   - **Never**: `:latest` tags are used

## SemVer Guidelines

- **Patch (X.Y.Z+1)**: Bug fixes, documentation updates, non-breaking changes
- **Minor (X.Y+1.0)**: New features, backward-compatible additions
- **Major (X+1.0.0)**: Breaking changes, API changes, incompatible modifications

## CI/CD

Built by Argo Workflows in the `iad-ci` cluster via the `commitgraph-build` WorkflowTemplate (in `jedarden/declarative-config`).

The build process:
1. Reads `containers/clone-worker/VERSION`
2. Builds with Kaniko using `--dockerfile=containers/clone-worker/Dockerfile`
3. Tags the image: `ronaldraygun/clone-worker:<VERSION>`
4. Pushes to the registry

## Dependencies

- Python 3.12+ (uses only standard library, no external packages)
- No `requirements.txt` needed

## Files

- `detection.py`: Main detection catalog module
- `test_detection.py`: Unit tests
- `Dockerfile`: Container image definition
- `VERSION`: Current semver (read by CI for image tagging)
