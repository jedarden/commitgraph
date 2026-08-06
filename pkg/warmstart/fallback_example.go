// Package warmstart provides a reference implementation of the cold/warm clone fallback pattern.
//
// This example demonstrates how to integrate warmstart incremental fetch with
// fallback to full git clone when warmstart extraction fails.
//
// The pattern is:
// 1. Attempt to fetch and materialize warmstart artifact from ARMOR
// 2. If warmstart succeeds, perform incremental fetch (git fetch origin)
// 3. If warmstart fails, evaluate error type and either:
//    - Fall back to cold clone (most errors)
//    - Fail the job (permission errors, disk space, NotAGitRepo)
//
// See docs/runbooks/warmstart-error-handling.md for detailed error handling guidance.
package warmstart

import (
	"errors"
	"fmt"
	"log"
	"strings"
)

// CloneWithFallback attempts warmstart incremental fetch, falling back to cold clone on error.
//
// This function demonstrates the recommended pattern for integrating warmstart with
// fallback to full git clone. It handles all warmstart error types appropriately:
//
// - Truncated, MissingMember, CorruptPack: Fall back to cold clone
// - IO with permission/disk errors: Fail immediately (local infrastructure issue)
// - IO with network errors: Fall back to cold clone
// - Other with NotAGitRepo: Fail immediately (invalid target directory)
// - Other unknown errors: Fall back to cold clone for robustness
//
// Parameters:
//   - repoURL: Git repository URL (e.g., "https://github.com/user/repo")
//   - gitDir: Target directory for git repository (must be empty or non-existent)
//   - fetchWarmstart: Function to fetch warmstart artifact from storage
//   - coldClone: Function to perform full git clone
//
// Returns:
//   - error: nil on success, error if both warmstart and cold clone fail
//
// Example usage:
//
//	err := CloneWithFallback(repoURL, gitDir,
//		func() ([]byte, error) { return fetchFromARMOR(repoURL) },
//		func(url, dir string) error { return execGitClone(url, dir) })
//	if err != nil {
//		log.Fatalf("Clone failed: %v", err)
//	}
func CloneWithFallback(
	repoURL string,
	gitDir string,
	fetchWarmstart func() ([]byte, error),
	coldClone func(url string, dir string) error,
) error {
	// Step 1: Attempt warmstart artifact fetch
	tarballData, err := fetchWarmstart()
	if err != nil {
		log.Printf("Warmstart artifact not available, falling back to cold clone: %v", err)
		return coldClone(repoURL, gitDir)
	}

	// Step 2: Parse warmstart tarball
	snapshot, err := ParseTarball(tarballData)
	if err != nil {
		// Evaluate error type to determine recovery strategy
		fallback, fatalErr := ShouldFallbackToColdClone(err)
		if fatalErr != nil {
			return fmt.Errorf("warmstart fatal error, cannot fall back: %w", fatalErr)
		}
		if fallback {
			log.Printf("Warmstart extraction failed, falling back to cold clone: %v", err)
			return coldClone(repoURL, gitDir)
		}
		return fmt.Errorf("warmstart extraction error: %w", err)
	}

	// Step 3: Materialize warmstart snapshot
	if err := Materialize(gitDir, snapshot); err != nil {
		fallback, fatalErr := ShouldFallbackToColdClone(err)
		if fatalErr != nil {
			return fmt.Errorf("warmstart materialization fatal error, cannot fall back: %w", fatalErr)
		}
		if fallback {
			log.Printf("Warmstart materialization failed, falling back to cold clone: %v", err)
			return coldClone(repoURL, gitDir)
		}
		return fmt.Errorf("warmstart materialization error: %w", err)
	}

	// Step 4: Perform incremental fetch (warmstart succeeded)
	log.Printf("Warmstart succeeded, performing incremental fetch")
	if err := performIncrementalFetch(repoURL, gitDir); err != nil {
		return fmt.Errorf("incremental fetch failed: %w", err)
	}

	return nil
}

// ShouldFallbackToColdClone evaluates whether a warmstart error should trigger fallback to cold clone.
//
// This function implements the error handling logic from docs/runbooks/warmstart-error-handling.md.
// It distinguishes between:
//
// 1. Fallback-appropriate errors: Truncated, MissingMember, CorruptPack, most IO and Other errors
// 2. Fatal errors: Permission errors, disk space errors, NotAGitRepo (should NOT fall back)
//
// Parameters:
//   - err: Error from ParseTarball or Materialize
//
// Returns:
//   - fallback: true if caller should fall back to cold clone
//   - fatalErr: non-nil if error is fatal (do NOT fall back, fail the job)
//
// Error handling rules:
//
// Fallback to cold clone (fallback=true, fatalErr=nil):
// - Truncated: Artifact is corrupt, unusable
// - MissingMember: Artifact is incomplete
// - CorruptPack: Pack data is corrupted
// - IO (network): Temporary I/O failure
// - IO (unknown): Other I/O issues
// - Other (unknown): Unexpected errors
//
// Fail immediately, do NOT fall back (fallback=false, fatalErr=err):
// - IO (permission): Local infrastructure issue, cold clone will also fail
// - IO (disk space): Disk full, cold clone will also fail
// - Other (NotAGitRepo): Invalid target directory
func ShouldFallbackToColdClone(err error) (fallback bool, fatalErr error) {
	if err == nil {
		return false, nil
	}

	// Check for NotAGitRepoError first (it's not a warmstart.Error type)
	if errors.Is(err, ErrNotAGitRepo) {
		log.Printf("Error: Target is not a git repository, not falling back")
		return false, fmt.Errorf("invalid target directory: %w", err)
	}

	var werr *Error
	if !errors.As(err, &werr) {
		// Non-warmstart error - fall back for robustness
		return true, nil
	}

	// Log error details for observability
	log.Printf("Warmstart error evaluation: kind=%s, member=%s, context=%s, offset=%d, underlying=%v",
		werr.Kind, werr.MemberName, werr.Context, werr.Offset, werr.Underlying)

	switch werr.Kind {
	case Truncated, MissingMember, CorruptPack:
		// These are artifact corruption errors - always safe to fall back
		return true, nil

	case IO:
		// Check if it's a permission error (do NOT fall back)
		if IsOsPermissionError(err) {
			log.Printf("Error: Permission error detected, not falling back (cold clone will also fail)")
			return false, fmt.Errorf("permission error during warmstart: %w", err)
		}

		// Check if it's a disk space error (do NOT fall back)
		if isDiskSpaceError(werr.Underlying) {
			log.Printf("Error: Disk space error detected, not falling back (cold clone will also fail)")
			return false, fmt.Errorf("disk space error during warmstart: %w", err)
		}

		// Other IO errors (network, temporary) - fall back
		return true, nil

	case Other:
		// Other unknown errors - fall back for robustness
		return true, nil

	default:
		// Unknown error kind - fall back for robustness
		return true, nil
	}
}

// isDiskSpaceError checks if an error is a disk space error.
//
// This detects both "no space left on device" and "disk quota exceeded" errors.
func isDiskSpaceError(err error) bool {
	if err == nil {
		return false
	}

	// Check error message for common disk space error patterns
	errMsg := strings.ToLower(err.Error())
	diskSpaceIndicators := []string{
		"no space left",
		"disk quota exceeded",
		"no space available",
		"disk full",
		"cannot allocate memory",
	}

	for _, indicator := range diskSpaceIndicators {
		if strings.Contains(errMsg, indicator) {
			return true
		}
	}

	return false
}

// performIncrementalFetch runs git fetch to retrieve only new commits since warmstart snapshot.
//
// This is a placeholder - the actual implementation would use exec.Command or a git library.
func performIncrementalFetch(repoURL, gitDir string) error {
	// Placeholder: In production, this would run:
	// git -C gitDir fetch origin main
	return fmt.Errorf("incremental fetch not implemented (placeholder)")
}

// CloneWithFallbackAndMetrics is a production-ready version that includes metrics and observability.
//
// This extends CloneWithFallback with:
// - Structured logging with correlation IDs
// - Metrics emission for monitoring
// - Detailed error context propagation
//
// See docs/runbooks/warmstart-error-handling.md for recommended metrics.
func CloneWithFallbackAndMetrics(
	repoURL string,
	gitDir string,
	correlationID string,
	fetchWarmstart func() ([]byte, error),
	coldClone func(url string, dir string) error,
	metrics MetricEmitter,
) error {
	log.Printf("[%s] Starting clone with fallback: repo=%s, dir=%s", correlationID, repoURL, gitDir)

	// Attempt warmstart
	tarballData, err := fetchWarmstart()
	if err != nil {
		metrics.EmitCounter("warmstart_attempt_total", "outcome", "artifact_unavailable")
		log.Printf("[%s] Warmstart artifact unavailable, falling back to cold clone: %v", correlationID, err)
		return coldClone(repoURL, gitDir)
	}

	metrics.EmitCounter("warmstart_attempt_total", "outcome", "attempted")

	snapshot, err := ParseTarball(tarballData)
	if err != nil {
		fallback, fatalErr := ShouldFallbackToColdClone(err)

		// Emit failure metric with error kind
		var werr *Error
		if errors.As(err, &werr) {
			metrics.EmitCounter("warmstart_failure_total", "error_kind", werr.Kind.String())
		}

		if fatalErr != nil {
			metrics.EmitCounter("warmstart_fatal_error_total", "reason", extractFatalReason(fatalErr))
			log.Printf("[%s] ERROR: Fatal warmstart error, cannot fall back: %v", correlationID, fatalErr)
			return fatalErr
		}

		if fallback {
			metrics.EmitCounter("warmstart_fallback_total", "trigger_error_kind", getErrorKindString(err))
			log.Printf("[%s] WARN: Warmstart extraction failed, falling back to cold clone: %v", correlationID, err)
			return coldClone(repoURL, gitDir)
		}

		metrics.EmitCounter("warmstart_attempt_total", "outcome", "error")
		return fmt.Errorf("warmstart extraction error: %w", err)
	}

	metrics.EmitCounter("warmstart_attempt_total", "outcome", "success")

	if err := Materialize(gitDir, snapshot); err != nil {
		fallback, fatalErr := ShouldFallbackToColdClone(err)

		var werr *Error
		if errors.As(err, &werr) {
			metrics.EmitCounter("warmstart_failure_total", "error_kind", werr.Kind.String(), "stage", "materialize")
		}

		if fatalErr != nil {
			metrics.EmitCounter("warmstart_fatal_error_total", "reason", extractFatalReason(fatalErr), "stage", "materialize")
			return fatalErr
		}

		if fallback {
			metrics.EmitCounter("warmstart_fallback_total", "trigger_error_kind", getErrorKindString(err), "stage", "materialize")
			log.Printf("[%s] WARN: Warmstart materialization failed, falling back to cold clone: %v", correlationID, err)
			return coldClone(repoURL, gitDir)
		}

		return fmt.Errorf("warmstart materialization error: %w", err)
	}

		// Run sanity checks on materialized repository
		// Verify repository integrity with git fsck and git log (no network access)
		if err := RunSanityChecks(gitDir); err != nil {
			metrics.EmitCounter("warmstart_sanity_check_failure_total", "reason", "integrity_check_failed")
			log.Printf("[%s] WARN: Warmstart sanity checks failed, falling back to cold clone: %v", correlationID, err)
			return coldClone(repoURL, gitDir)
		}

	log.Printf("[%s] Warmstart succeeded, performing incremental fetch", correlationID)
	if err := performIncrementalFetch(repoURL, gitDir); err != nil {
		return fmt.Errorf("incremental fetch failed: %w", err)
	}

	return nil
}

// getErrorKindString extracts the error kind string for metrics labels.
func getErrorKindString(err error) string {
	var werr *Error
	if errors.As(err, &werr) {
		return werr.Kind.String()
	}
	return "unknown"
}

// extractFatalReason extracts the reason for a fatal error for metrics.
func extractFatalReason(err error) string {
	if IsOsPermissionError(err) {
		return "permission"
	}
	if isDiskSpaceError(err) {
		return "disk_space"
	}
	if errors.Is(err, ErrNotAGitRepo) {
		return "not_git_repo"
	}
	return "unknown"
}

// MetricEmitter is an interface for emitting metrics (placeholder for production metrics system).
type MetricEmitter interface {
	EmitCounter(name string, labels ...string)
}

// ExampleMetricsEmitter is a simple logger-based metrics emitter for testing.
type ExampleMetricsEmitter struct{}

func (e *ExampleMetricsEmitter) EmitCounter(name string, labels ...string) {
	log.Printf("[METRIC] %s %v", name, labels)
}

// Example usage:
//
//	func main() {
//		metrics := &ExampleMetricsEmitter{}
//		err := CloneWithFallbackAndMetrics(
//			"https://github.com/user/repo",
//			"/tmp/repo",
//			"correlation-123",
//			func() ([]byte, error) { /* fetch from ARMOR */ },
//			func(url, dir string) error { /* git clone */ },
//			metrics,
//		)
//		if err != nil {
//			log.Fatal(err)
//		}
//	}
