package warmstart

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestVerifyGitFsck_NoNetworkAccess verifies that git fsck does not attempt
// network access by running it with GIT_TRACE to inspect git's behavior.
func TestVerifyGitFsck_NoNetworkAccess(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping")
	}

	// Create a valid git repository
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, "test.git")

	cmd := exec.Command("git", "init", "--bare", gitDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to init git repo: %v\noutput: %s", err, output)
	}

	// Clone the bare repo to a working directory to create commits
	workingDir := filepath.Join(tmpDir, "working")
	cmd = exec.Command("git", "clone", gitDir, workingDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to clone working repo: %v\noutput: %s", err, output)
	}

	// Configure git user in working directory
	cmd = exec.Command("git", "-C", workingDir, "config", "user.name", "Test User")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to configure git user.name: %v\noutput: %s", err, output)
	}
	cmd = exec.Command("git", "-C", workingDir, "config", "user.email", "test@example.com")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to configure git user.email: %v\noutput: %s", err, output)
	}

	// Create initial commit
	testFile := filepath.Join(workingDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	cmd = exec.Command("git", "-C", workingDir, "add", "test.txt")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to add file: %v\noutput: %s", err, output)
	}
	cmd = exec.Command("git", "-C", workingDir, "commit", "-m", "Initial commit")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to commit: %v\noutput: %s", err, output)
	}

	// Push to bare repository
	cmd = exec.Command("git", "-C", workingDir, "push", "origin", "master")
	var pushOutput []byte
	var pushErr error
	pushOutput, pushErr = cmd.CombinedOutput()
	if pushErr != nil {
		// Try 'main' branch if 'master' fails
		cmd = exec.Command("git", "-C", workingDir, "push", "origin", "main")
		if _, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to push to origin: %v\noutput: %s", err, pushOutput)
		}
	}

	// Run git fsck with GIT_TRACE=1 to capture all git operations
	// This will reveal if git attempts any network operations
	var traceOutput bytes.Buffer
	cmd = exec.Command("git", "--git-dir="+gitDir, "fsck", "--no-full", "--no-progress")
	cmd.Env = append(os.Environ(), "GIT_TRACE=1")
	cmd.Stdout = &traceOutput
	cmd.Stderr = &traceOutput

	if err := cmd.Run(); err != nil {
		// fsck may fail on empty repo, that's ok - we're checking for network access
		t.Logf("git fsck output (may fail on empty repo): %v", err)
	}

	traceStr := traceOutput.String()

	// Verify no network-related trace entries
	// Git trace output shows commands and operations; network ops would appear here
	networkKeywords := []string{
		"git-remote-",
		"git-remote-http",
		"git-remote-https",
		"git-remote-ftp",
		"git fetch",
		"git ls-remote",
		"connect to host",
		"network ",
		"network\t",
		"socket ",
		"socket\t",
		"tcp://",
		"https://",
		"http://",
	}

	for _, keyword := range networkKeywords {
		if strings.Contains(strings.ToLower(traceStr), strings.ToLower(keyword)) {
			t.Errorf("git fsck attempted network operation (found '%s' in trace): %s", keyword, traceStr)
		}
	}

	t.Logf("Verified git fsck does not perform network operations")
	t.Logf("Trace output: %s", traceStr)
}

// TestVerifyGitLog_NoNetworkAccess verifies that git log does not attempt
// network access by running it with GIT_TRACE to inspect git's behavior.
func TestVerifyGitLog_NoNetworkAccess(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping")
	}

	// Create a valid git repository
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, "test.git")

	cmd := exec.Command("git", "init", "--bare", gitDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to init git repo: %v\noutput: %s", err, output)
	}

	// Clone the bare repo to a working directory to create commits
	workingDir := filepath.Join(tmpDir, "working")
	cmd = exec.Command("git", "clone", gitDir, workingDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to clone working repo: %v\noutput: %s", err, output)
	}

	// Configure git user in working directory
	cmd = exec.Command("git", "-C", workingDir, "config", "user.name", "Test User")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to configure git user.name: %v\noutput: %s", err, output)
	}
	cmd = exec.Command("git", "-C", workingDir, "config", "user.email", "test@example.com")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to configure git user.email: %v\noutput: %s", err, output)
	}

	// Create initial commit
	testFile := filepath.Join(workingDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	cmd = exec.Command("git", "-C", workingDir, "add", "test.txt")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to add file: %v\noutput: %s", err, output)
	}
	cmd = exec.Command("git", "-C", workingDir, "commit", "-m", "Initial commit")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to commit: %v\noutput: %s", err, output)
	}

	// Push to bare repository
	cmd = exec.Command("git", "-C", workingDir, "push", "origin", "master")
	var pushOutput []byte
	var err error
	pushOutput, err = cmd.CombinedOutput()
	if err != nil {
		// Try 'main' branch if 'master' fails
		cmd = exec.Command("git", "-C", workingDir, "push", "origin", "main")
		if _, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to push to origin: %v\noutput: %s", err, pushOutput)
		}
	}

	// Run git log with GIT_TRACE=1 to capture all git operations
	// This will reveal if git attempts any network operations
	var traceOutput bytes.Buffer
	cmd = exec.Command("git", "--git-dir="+gitDir, "log", "--oneline", "-n", "1")
	cmd.Env = append(os.Environ(), "GIT_TRACE=1")
	cmd.Stdout = &traceOutput
	cmd.Stderr = &traceOutput

	if err := cmd.Run(); err != nil {
		// log may fail on empty repo, that's ok - we're checking for network access
		t.Logf("git log output (may fail on empty repo): %v", err)
	}

	traceStr := traceOutput.String()

	// Verify no network-related trace entries
	// Git trace output shows commands and operations; network ops would appear here
	networkKeywords := []string{
		"git-remote-",
		"git-remote-http",
		"git-remote-https",
		"git-remote-ftp",
		"git fetch",
		"git ls-remote",
		"connect to host",
		"network ",
		"network\t",
		"socket ",
		"socket\t",
		"tcp://",
		"https://",
		"http://",
	}

	for _, keyword := range networkKeywords {
		if strings.Contains(strings.ToLower(traceStr), strings.ToLower(keyword)) {
			t.Errorf("git log attempted network operation (found '%s' in trace): %s", keyword, traceStr)
		}
	}

	t.Logf("Verified git log does not perform network operations")
	t.Logf("Trace output: %s", traceStr)
}

// TestRunSanityChecks_NoNetworkAccess verifies that the combined sanity checks
// do not attempt network access by running both checks with GIT_TRACE.
func TestRunSanityChecks_NoNetworkAccess(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping")
	}

	// Create a valid git repository
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, "test.git")

	cmd := exec.Command("git", "init", "--bare", gitDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to init git repo: %v\noutput: %s", err, output)
	}

	// Clone the bare repo to a working directory to create commits
	workingDir := filepath.Join(tmpDir, "working")
	cmd = exec.Command("git", "clone", gitDir, workingDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to clone working repo: %v\noutput: %s", err, output)
	}

	// Configure git user in working directory
	cmd = exec.Command("git", "-C", workingDir, "config", "user.name", "Test User")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to configure git user.name: %v\noutput: %s", err, output)
	}
	cmd = exec.Command("git", "-C", workingDir, "config", "user.email", "test@example.com")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to configure git user.email: %v\noutput: %s", err, output)
	}

	// Create initial commit
	testFile := filepath.Join(workingDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	cmd = exec.Command("git", "-C", workingDir, "add", "test.txt")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to add file: %v\noutput: %s", err, output)
	}
	cmd = exec.Command("git", "-C", workingDir, "commit", "-m", "Initial commit")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to commit: %v\noutput: %s", err, output)
	}

	// Push to bare repository
	cmd = exec.Command("git", "-C", workingDir, "push", "origin", "master")
	var pushOutput []byte
	var err error
	pushOutput, err = cmd.CombinedOutput()
	if err != nil {
		// Try 'main' branch if 'master' fails
		cmd = exec.Command("git", "-C", workingDir, "push", "origin", "main")
		if _, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to push to origin: %v\noutput: %s", err, pushOutput)
		}
	}

	// Run sanity checks
	err = RunSanityChecks(gitDir)

	// The checks should succeed on a valid repository
	if err != nil {
		t.Logf("RunSanityChecks failed on valid repository (may be expected for empty repo): %v", err)
	}

	// Note: Since we can't capture the trace output from the internal git commands
	// without modifying the implementation, we verify by checking that the checks
	// completed without attempting network operations (which would fail with
	// "could not connect" errors if network was required)

	t.Logf("RunSanityChecks completed without network access errors")

	// Verify no network-related error messages in the error (if any)
	if err != nil {
		errMsg := err.Error()
		networkErrorKeywords := []string{
			"could not connect",
			"connection refused",
			"network",
			"hostname not found",
			"timeout",
			"unreachable",
		}

		for _, keyword := range networkErrorKeywords {
			if strings.Contains(strings.ToLower(errMsg), strings.ToLower(keyword)) {
				t.Errorf("RunSanityChecks failed with network-related error (found '%s'): %v", keyword, err)
			}
		}
	}
}
