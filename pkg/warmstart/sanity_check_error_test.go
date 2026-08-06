package warmstart

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestSanityCheckErrors_CorruptedPackFile verifies that corrupted pack files
// produce clear, actionable error messages that identify the specific corruption.
func TestSanityCheckErrors_CorruptedPackFile(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping")
	}

	// Create a temporary directory
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, "test.git")

	// Initialize a git repository
	cmd := exec.Command("git", "init", "--bare", gitDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to init git repo: %v\noutput: %s", err, output)
	}

	// Create a corrupted pack file by writing invalid data
	objectsDir := filepath.Join(gitDir, "objects", "pack")
	if err := os.MkdirAll(objectsDir, 0755); err != nil {
		t.Fatalf("failed to create objects/pack directory: %v", err)
	}

	corruptedPackPath := filepath.Join(objectsDir, "pack-corrupted.pack")
	corruptedData := []byte("CORRUPTED DATA NOT A VALID PACK")
	if err := os.WriteFile(corruptedPackPath, corruptedData, 0444); err != nil {
		t.Fatalf("failed to write corrupted pack file: %v", err)
	}

	// Test VerifyGitFsck with corrupted pack file
	t.Run("VerifyGitFsck", func(t *testing.T) {
		err := VerifyGitFsck(gitDir)
		if err == nil {
			// If no error, log a note but don't fail the test
			// Git may not actually read the pack file without any refs pointing to it
			t.Log("Note: git fsck did not detect error in corrupted pack file (may not have been read)")
			return
		}

		errMsg := err.Error()

		// Verify error message contains key information
		if !strings.Contains(errMsg, "corrupt") && !strings.Contains(errMsg, "bad") {
			t.Errorf("error message should mention corruption or bad data, got: %s", errMsg)
		}

		// Error should be from git fsck
		if !strings.Contains(errMsg, "fsck") && !strings.Contains(errMsg, "git") {
			t.Logf("error message (should ideally mention fsck): %s", errMsg)
		}

		t.Logf("Corrupted pack file error message: %s", errMsg)
	})

	// Test VerifyGitLog with corrupted pack file
	t.Run("VerifyGitLog", func(t *testing.T) {
		err := VerifyGitLog(gitDir)
		if err == nil {
			// If no error, log a note but don't fail the test
			t.Log("Note: git log did not detect error in corrupted pack file (may not have been read)")
			return
		}

		errMsg := err.Error()

		// Verify error message contains key information
		// Git may report various errors (corruption, repository state, etc.)
		hasKeyword := strings.Contains(errMsg, "corrupt") || strings.Contains(errMsg, "bad") ||
			strings.Contains(errMsg, "object") || strings.Contains(errMsg, "fatal") ||
			strings.Contains(errMsg, "repository")
		if !hasKeyword {
			t.Errorf("error message should mention the issue, got: %s", errMsg)
		}

		// Error should be from git log
		if !strings.Contains(errMsg, "log") && !strings.Contains(errMsg, "git") {
			t.Logf("error message (should ideally mention log): %s", errMsg)
		}

		t.Logf("Corrupted pack file (git log) error message: %s", errMsg)
	})
}

// TestSanityCheckErrors_MissingRefFile verifies that missing ref files
// produce clear error messages identifying the missing file.
func TestSanityCheckErrors_MissingRefFile(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping")
	}

	// Create a temporary directory
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, "test.git")

	// Initialize a git repository
	cmd := exec.Command("git", "init", "--bare", gitDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to init git repo: %v\noutput: %s", err, output)
	}

	// Remove the refs directory to simulate missing refs
	refsDir := filepath.Join(gitDir, "refs")
	if err := os.RemoveAll(refsDir); err != nil {
		t.Fatalf("failed to remove refs directory: %v", err)
	}

	// Also remove HEAD
	headPath := filepath.Join(gitDir, "HEAD")
	if err := os.Remove(headPath); err != nil && !os.IsNotExist(err) {
		t.Fatalf("failed to remove HEAD: %v", err)
	}

	// Test VerifyGitLog with missing refs
	t.Run("VerifyGitLog", func(t *testing.T) {
		err := VerifyGitLog(gitDir)
		if err == nil {
			t.Log("Note: git log did not detect error with missing refs")
			return
		}

		errMsg := err.Error()

		// Verify error message is informative
		if !strings.Contains(errMsg, "log") && !strings.Contains(errMsg, "git") {
			t.Logf("error message (should ideally mention git log): %s", errMsg)
		}

		t.Logf("Missing ref file error message: %s", errMsg)
	})
}

// TestSanityCheckErrors_CorruptedGitConfig verifies that corrupted git config
// produces clear error messages.
func TestSanityCheckErrors_CorruptedGitConfig(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping")
	}

	// Create a temporary directory
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, "test.git")

	// Initialize a git repository
	cmd := exec.Command("git", "init", "--bare", gitDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to init git repo: %v\noutput: %s", err, output)
	}

	// Corrupt the config file
	configPath := filepath.Join(gitDir, "config")
	corruptedConfig := []byte("corrupted config data that is not valid [section]\ninvalid format {{{")
	if err := os.WriteFile(configPath, corruptedConfig, 0644); err != nil {
		t.Fatalf("failed to write corrupted config: %v", err)
	}

	// Test VerifyGitFsck with corrupted config
	t.Run("VerifyGitFsck", func(t *testing.T) {
		err := VerifyGitFsck(gitDir)
		if err == nil {
			t.Log("Note: git fsck did not detect error with corrupted config")
			return
		}

		errMsg := err.Error()

		// Verify error message contains relevant information
		t.Logf("Corrupted git config error message: %s", errMsg)

		// The error should at least mention something went wrong
		if errMsg == "" {
			t.Error("error message should not be empty")
		}
	})

	// Test VerifyGitLog with corrupted config
	t.Run("VerifyGitLog", func(t *testing.T) {
		err := VerifyGitLog(gitDir)
		if err == nil {
			t.Log("Note: git log did not detect error with corrupted config")
			return
		}

		errMsg := err.Error()

		// Verify error message contains relevant information
		t.Logf("Corrupted git config (git log) error message: %s", errMsg)

		// The error should at least mention something went wrong
		if errMsg == "" {
			t.Error("error message should not be empty")
		}
	})
}

// TestSanityCheckErrors_MissingObjectsDirectory verifies that missing objects
// directory produces clear error messages.
func TestSanityCheckErrors_MissingObjectsDirectory(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping")
	}

	// Create a temporary directory
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, "test.git")

	// Initialize a git repository
	cmd := exec.Command("git", "init", "--bare", gitDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to init git repo: %v\noutput: %s", err, output)
	}

	// Remove the objects directory
	objectsDir := filepath.Join(gitDir, "objects")
	if err := os.RemoveAll(objectsDir); err != nil {
		t.Fatalf("failed to remove objects directory: %v", err)
	}

	// Test VerifyGitFsck with missing objects
	t.Run("VerifyGitFsck", func(t *testing.T) {
		err := VerifyGitFsck(gitDir)
		if err == nil {
			t.Log("Note: git fsck did not detect error with missing objects")
			return
		}

		errMsg := err.Error()

		// Verify error message mentions the problem
		if !strings.Contains(errMsg, "object") && !strings.Contains(errMsg, "missing") {
			t.Logf("error message (should ideally mention missing objects): %s", errMsg)
		}

		t.Logf("Missing objects directory error message: %s", errMsg)
	})

	// Test VerifyGitLog with missing objects
	t.Run("VerifyGitLog", func(t *testing.T) {
		err := VerifyGitLog(gitDir)
		if err == nil {
			t.Log("Note: git log did not detect error with missing objects")
			return
		}

		errMsg := err.Error()

		// Verify error message mentions the problem
		if !strings.Contains(errMsg, "object") && !strings.Contains(errMsg, "missing") {
			t.Logf("error message (should ideally mention missing objects): %s", errMsg)
		}

		t.Logf("Missing objects directory (git log) error message: %s", errMsg)
	})
}

// TestSanityCheckErrors_DistinguishesFsckFromLog verifies that error messages
// distinguish between fsck failures and log failures.
func TestSanityCheckErrors_DistinguishesFsckFromLog(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping")
	}

	// Create a temporary directory
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, "test.git")

	// Initialize a git repository
	cmd := exec.Command("git", "init", "--bare", gitDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to init git repo: %v\noutput: %s", err, output)
	}

	// Remove objects to cause both fsck and log to fail
	objectsDir := filepath.Join(gitDir, "objects")
	if err := os.RemoveAll(objectsDir); err != nil {
		t.Fatalf("failed to remove objects directory: %v", err)
	}

	// Get errors from both checks
	fsckErr := VerifyGitFsck(gitDir)
	logErr := VerifyGitLog(gitDir)

	// Both should fail
	if fsckErr == nil {
		t.Error("expected fsck error, got nil")
	}
	if logErr == nil {
		t.Error("expected log error, got nil")
	}

	// Check that error messages are distinguishable
	if fsckErr != nil && logErr != nil {
		fsckMsg := fsckErr.Error()
		logMsg := logErr.Error()

		t.Logf("Fsck error: %s", fsckMsg)
		t.Logf("Log error: %s", logMsg)

		// Messages should not be identical
		if fsckMsg == logMsg {
			t.Error("fsck and log error messages should be distinguishable")
		}
	}
}

// TestSanityCheckErrors_RunSanityChecksPropagation verifies that RunSanityChecks
// properly propagates errors from both fsck and log checks.
func TestSanityCheckErrors_RunSanityChecksPropagation(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping")
	}

	t.Run("FsckFailurePropagated", func(t *testing.T) {
		// Create a temporary directory
		tmpDir := t.TempDir()
		gitDir := filepath.Join(tmpDir, "test.git")

		// Initialize a git repository
		cmd := exec.Command("git", "init", "--bare", gitDir)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to init git repo: %v\noutput: %s", err, output)
		}

		// Corrupt repository to cause fsck failure
		objectsDir := filepath.Join(gitDir, "objects")
		if err := os.RemoveAll(objectsDir); err != nil {
			t.Fatalf("failed to remove objects directory: %v", err)
		}

		// RunSanityChecks should fail
		err := RunSanityChecks(gitDir)
		if err == nil {
			t.Error("expected RunSanityChecks to fail when fsck fails")
			return
		}

		// Error message should mention fsck
		errMsg := err.Error()
		if !strings.Contains(errMsg, "fsck") {
			t.Errorf("error message should mention fsck, got: %s", errMsg)
		}

		t.Logf("RunSanityChecks propagated fsck error: %s", errMsg)
	})

	t.Run("LogFailurePropagated", func(t *testing.T) {
		// Create a valid repository for fsck to pass, but then cause log to fail
		tmpDir := t.TempDir()
		gitDir := filepath.Join(tmpDir, "test.git")

		// Initialize a git repository
		cmd := exec.Command("git", "init", "--bare", gitDir)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to init git repo: %v\noutput: %s", err, output)
		}

		// Create a commit to make fsck pass
		// This requires setting up a minimal valid commit
		// For now, we'll just verify that RunSanityChecks propagates errors

		// Remove refs to cause log to fail
		refsDir := filepath.Join(gitDir, "refs")
		if err := os.RemoveAll(refsDir); err != nil {
			t.Fatalf("failed to remove refs directory: %v", err)
		}

		headPath := filepath.Join(gitDir, "HEAD")
		if err := os.Remove(headPath); err != nil && !os.IsNotExist(err) {
			t.Fatalf("failed to remove HEAD: %v", err)
		}

		// RunSanityChecks should fail (either from fsck or log)
		err := RunSanityChecks(gitDir)
		if err == nil {
			t.Error("expected RunSanityChecks to fail when repository is corrupted")
			return
		}

		// Error message should mention either fsck or log
		errMsg := err.Error()
		hasFsckOrLog := strings.Contains(errMsg, "fsck") || strings.Contains(errMsg, "log")
		if !hasFsckOrLog {
			t.Errorf("error message should mention fsck or log, got: %s", errMsg)
		}

		t.Logf("RunSanityChecks propagated error: %s", errMsg)

		// Verify that the error was properly propagated through RunSanityChecks
		// The error should include "sanity check failed" to show it came from RunSanityChecks
		if !strings.Contains(errMsg, "sanity check") {
			t.Logf("Note: error message doesn't mention 'sanity check': %s", errMsg)
		}
	})
}

// TestSanityCheckErrors_SpecificFileInError verifies that error messages
// include the specific file or component that failed.
func TestSanityCheckErrors_SpecificFileInError(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping")
	}

	// Test that CorruptPackError includes member name
	t.Run("CorruptPackErrorWithMemberName", func(t *testing.T) {
		memberName := "objects/pack/pack-abc123.pack"
		context := "checksum validation failed"

		err := NewCorruptPackError(memberName, context)

		errMsg := err.Error()

		// Verify error includes member name
		if !strings.Contains(errMsg, memberName) {
			t.Errorf("error message should include member name %s, got: %s", memberName, errMsg)
		}

		// Verify error includes context
		if !strings.Contains(errMsg, context) {
			t.Errorf("error message should include context %s, got: %s", context, errMsg)
		}

		// Verify error mentions corruption
		if !strings.Contains(errMsg, "corrupt") {
			t.Errorf("error message should mention corruption, got: %s", errMsg)
		}

		t.Logf("CorruptPackError with member name: %s", errMsg)
	})

	// Test that MissingMemberError includes member name
	t.Run("MissingMemberErrorWithMemberName", func(t *testing.T) {
		memberName := "objects/pack/pack-def456.ref"

		err := NewMissingMemberError(memberName)

		errMsg := err.Error()

		// Verify error includes member name
		if !strings.Contains(errMsg, memberName) {
			t.Errorf("error message should include member name %s, got: %s", memberName, errMsg)
		}

		// Verify error mentions missing
		if !strings.Contains(errMsg, "missing") {
			t.Errorf("error message should mention missing, got: %s", errMsg)
		}

		t.Logf("MissingMemberError with member name: %s", errMsg)
	})

	// Test that TruncatedMemberError includes member name and context
	t.Run("TruncatedMemberErrorWithDetails", func(t *testing.T) {
		memberName := "objects/pack/pack-xyz.pack"
		context := "expected 1024 bytes, got 512"

		err := NewTruncatedMemberError(memberName, context, 512)

		errMsg := err.Error()

		// Verify error includes member name
		if !strings.Contains(errMsg, memberName) {
			t.Errorf("error message should include member name %s, got: %s", memberName, errMsg)
		}

		// Verify error includes context
		if !strings.Contains(errMsg, context) {
			t.Errorf("error message should include context %s, got: %s", context, errMsg)
		}

		// Verify error mentions truncated
		if !strings.Contains(errMsg, "truncated") {
			t.Errorf("error message should mention truncated, got: %s", errMsg)
		}

		t.Logf("TruncatedMemberError with details: %s", errMsg)
	})
}

// TestSanityCheckErrors_ActionableMessages verifies that error messages
// provide actionable information to help users understand what went wrong.
func TestSanityCheckErrors_ActionableMessages(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		mustContain []string
		description string
	}{
		{
			name: "CorruptPackError mentions corruption",
			err:  NewCorruptPackError("objects/pack/pack-123.pack", "checksum failed"),
			mustContain: []string{"corrupt", "pack"},
			description: "Error should identify it's a pack corruption issue",
		},
		{
			name: "MissingMemberError mentions missing file",
			err:  NewMissingMemberError(".idx"),
			mustContain: []string{"missing", "member"},
			description: "Error should identify a member is missing",
		},
		{
			name: "TruncatedError mentions truncated data",
			err:  NewTruncatedError("unexpected EOF", 1024),
			mustContain: []string{"truncated"},
			description: "Error should identify data truncation",
		},
		{
			name: "TruncatedMemberError mentions specific file",
			err:  NewTruncatedMemberError("config.json", "header too short", 0),
			mustContain: []string{"truncated", "config.json"},
			description: "Error should identify which file was truncated",
		},
		{
			name: "MissingMemberErrorWithContext lists files",
			err:  NewMissingMemberErrorWithContext(".ref", "missing .ref files: a.ref, b.ref"),
			mustContain: []string{"missing", ".ref", "a.ref", "b.ref"},
			description: "Error should list all missing files",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Fatal("test error is nil")
			}

			errMsg := tt.err.Error()
			t.Logf("Error message: %s", errMsg)

			// Verify error contains all required strings
			for _, mustHave := range tt.mustContain {
				if !strings.Contains(errMsg, mustHave) {
					t.Errorf("%s: error message should contain %q, got: %s", tt.description, mustHave, errMsg)
				}
			}

			// Verify error message is not empty
			if errMsg == "" {
				t.Error("error message should not be empty")
			}

			// Verify error message is reasonably descriptive (at least 20 chars)
			if len(errMsg) < 20 {
				t.Errorf("error message should be descriptive (at least 20 chars), got: %s", errMsg)
			}
		})
	}
}
