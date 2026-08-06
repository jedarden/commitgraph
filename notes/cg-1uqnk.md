# Git Fsck and Log Sanity Checks - Task cg-1uqnk

## Summary

This task required adding git fsck and git log sanity checks to the warmstart materialized directory. The implementation is complete and all requirements have been verified.

## Implementation Overview

### Core Functions (pkg/warmstart/extract.go)

1. **VerifyGitFsck(gitDir string) error** (lines 585-626)
   - Runs `git fsck --no-full --no-progress` to verify repository integrity
   - Uses `exec.LookPath` to check git availability
   - Returns clear errors for corruption detection
   - No network access (local file system only)

2. **VerifyGitLog(gitDir string) error** (lines 628-674)
   - Runs `git log --oneline -n 1` to verify commit history accessibility
   - Uses `exec.LookPath` to check git availability
   - Returns clear errors for corruption detection
   - No network access (local file system only)

3. **RunSanityChecks(gitDir string) error** (lines 676-706)
   - Convenience function running both checks in sequence
   - Returns first error encountered if any check fails
   - Used in production code (fallback_example.go line 298)

## Test Coverage

### No-Network Verification Tests (pkg/warmstart/no_network_test.go)

1. **TestVerifyGitFsck_NoNetworkAccess** (lines 12-114)
   - Creates valid git repository with commits
   - Runs git fsck with GIT_TRACE=1 to capture all git operations
   - Verifies no network-related keywords in trace output
   - ✅ PASS - No network operations detected

2. **TestVerifyGitLog_NoNetworkAccess** (lines 116-218)
   - Creates valid git repository with commits
   - Runs git log with GIT_TRACE=1 to capture all git operations
   - Verifies no network-related keywords in trace output
   - ✅ PASS - No network operations detected

3. **TestRunSanityChecks_NoNetworkAccess** (lines 220-313)
   - Creates valid git repository with commits
   - Runs RunSanityChecks and verifies no network-related errors
   - ✅ PASS - No network operations detected

### Error Detection Tests (pkg/warmstart/sanity_check_error_test.go)

1. **TestSanityCheckErrors_CorruptedPackFile**
   - Tests corrupted pack file detection with clear error messages

2. **TestSanityCheckErrors_MissingRefFile**
   - Tests missing ref file detection with clear error messages

3. **TestSanityCheckErrors_CorruptedGitConfig**
   - Tests corrupted config detection with clear error messages

4. **TestSanityCheckErrors_MissingObjectsDirectory**
   - Tests missing objects directory detection with clear error messages

5. **TestSanityCheckErrors_DistinguishesFsckFromLog**
   - Verifies error messages distinguish between fsck and log failures

6. **TestSanityCheckErrors_RunSanityChecksPropagation**
   - Verifies errors propagate correctly through RunSanityChecks

7. **TestSanityCheckErrors_SpecificFileInError**
   - Verifies error messages include specific file/component names

8. **TestSanityCheckErrors_ActionableMessages**
   - Verifies error messages are actionable and descriptive

### Integration Test (pkg/warmstart/extract_test.go)

**TestWarmStartTarballIntegration** (lines 3580-3769)
   - Creates source repository with real commits
   - Generates warm-start tarball with real git pack data
   - Materializes snapshot to target directory
   - Runs RunSanityChecks on materialized repository
   - Verifies git log can read commit history
   - Verifies ref points to valid commit
   - ✅ PASS - Complete workflow succeeds

### Success/Failure Tests (pkg/warmstart/extract_test.go)

**TestRunSanityChecks_Success** (lines 3446-3506)
   - Creates repository with commits
   - Verifies RunSanityChecks succeeds on valid repository
   - ✅ PASS

## Requirements Verification

All acceptance criteria from the task are met:

- ✅ **git fsck runs successfully on the materialized directory**
  - Implemented: `VerifyGitFsck()`
  - Verified: `TestRunSanityChecks_Success`, `TestWarmStartTarballIntegration`

- ✅ **git log outputs commit history without errors**
  - Implemented: `VerifyGitLog()`
  - Verified: `TestRunSanityChecks_Success`, `TestWarmStartTarballIntegration`

- ✅ **Neither check invokes network access**
  - Verified: `TestVerifyGitFsck_NoNetworkAccess`, `TestVerifyGitLog_NoNetworkAccess`, `TestRunSanityChecks_NoNetworkAccess`
  - Tests use GIT_TRACE to capture and verify no network operations

- ✅ **Sanity checks fail with clear error messages if corruption exists**
  - Verified: All `TestSanityCheckErrors_*` tests
  - Error messages include specific files, corruption details, and actionable information

- ✅ **Integration test creates a warm-start tarball, extracts it, runs sanity checks**
  - Implemented: `TestWarmStartTarballIntegration`
  - Complete end-to-end workflow tested

- ✅ **Test verifies no network calls are made during checks**
  - Verified: Multiple no-network tests using GIT_TRACE
  - All network keywords checked in trace output

## Test Results

All tests passing:

```bash
# No-network tests
=== RUN   TestVerifyGitFsck_NoNetworkAccess
--- PASS: TestVerifyGitFsck_NoNetworkAccess (0.06s)
=== RUN   TestVerifyGitLog_NoNetworkAccess
--- PASS: TestVerifyGitLog_NoNetworkAccess (0.05s)
=== RUN   TestRunSanityChecks_NoNetworkAccess
--- PASS: TestRunSanityChecks_NoNetworkAccess (0.05s)

# Error detection tests
=== RUN   TestSanityCheckErrors_CorruptedPackFile
--- PASS: TestSanityCheckErrors_CorruptedPackFile (0.01s)
=== RUN   TestSanityCheckErrors_MissingRefFile
--- PASS: TestSanityCheckErrors_MissingRefFile (0.00s)
=== RUN   TestSanityCheckErrors_CorruptedGitConfig
--- PASS: TestSanityCheckErrors_CorruptedGitConfig (0.00s)
=== RUN   TestSanityCheckErrors_MissingObjectsDirectory
--- PASS: TestSanityCheckErrors_MissingObjectsDirectory (0.01s)
=== RUN   TestSanityCheckErrors_DistinguishesFsckFromLog
--- PASS: TestSanityCheckErrors_DistinguishesFsckFromLog (0.01s)
=== RUN   TestSanityCheckErrors_RunSanityChecksPropagation
--- PASS: TestSanityCheckErrors_RunSanityChecksPropagation (0.01s)
=== RUN   TestSanityCheckErrors_SpecificFileInError
--- PASS: TestSanityCheckErrors_SpecificFileInError (0.00s)
=== RUN   TestSanityCheckErrors_ActionableMessages
--- PASS: TestSanityCheckErrors_ActionableMessages (0.00s)

# Integration tests
=== RUN   TestRunSanityChecks_Success
--- PASS: TestRunSanityChecks_Success (0.05s)
=== RUN   TestWarmStartTarballIntegration
--- PASS: TestWarmStartTarballIntegration (0.07s)
```

## Production Usage

The sanity checks are already integrated into the production warmstart workflow in `fallback_example.go` (lines 297-302):

```go
// Run sanity checks on materialized repository
// Verify repository integrity with git fsck and git log (no network access)
if err := RunSanityChecks(gitDir); err != nil {
    metrics.EmitCounter("warmstart_sanity_check_failure_total", "reason", "integrity_check_failed")
    log.Printf("[%s] WARN: Warmstart sanity checks failed, falling back to cold clone: %v", correlationID, err)
    return coldClone(repoURL, gitDir)
}
```

## Conclusion

The git fsck and git log sanity checks are fully implemented with comprehensive test coverage. All acceptance criteria are met and verified through automated tests. The implementation ensures:
- Repository integrity verification without network access
- Clear, actionable error messages for debugging
- Complete integration with warmstart workflow
- Comprehensive test coverage including no-network verification

Task completed successfully.
