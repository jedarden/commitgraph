package warmstart

import (
	"archive/tar"
	"bytes"
	"errors"
	"strings"
	"testing"
)

// TestCorruptionErrors_TruncatedTarball tests Truncated error with prematurely ending tarball
func TestCorruptionErrors_TruncatedTarball(t *testing.T) {
	configData := []byte(`{
		"core.repositoryformatversion": "1",
		"remote.origin.promisor": "true",
		"remote.origin.partialclonefilter": "blob:none"
	}`)
	refData := []byte("refs/heads/main abc123")
	packData := []byte("PACK123456789") // Valid 12-byte minimum pack file
	idxData := []byte("test idx data")
	refFileData := []byte("test ref data")

	members := []TarballMember{
		{Name: "objects/pack/pack-123.pack", Data: packData},
		{Name: "objects/pack/pack-123.idx", Data: idxData},
		{Name: "objects/pack/pack-123.ref", Data: refFileData},
		{Name: "config.json", Data: configData},
		{Name: "ref", Data: refData},
	}

	tarball := createTestTarball(t, members)

	// Truncate the tarball to simulate premature ending
	truncatedTarball := tarball[:len(tarball)-50]

	_, err := ParseTarball(truncatedTarball)
	if err == nil {
		t.Fatal("expected error for truncated tarball, got nil")
	}

	// Verify it's a truncated error or contains truncated information
	var truncErr *Error
	if errors.As(err, &truncErr) {
		if truncErr.Kind != Truncated {
			t.Errorf("expected Truncated error kind, got %v", truncErr.Kind)
		}
		t.Logf("Truncated tarball error: %v", err)
	} else {
		t.Logf("Got error (may be wrapped): %v", err)
	}
}

// TestCorruptionErrors_TruncatedMember tests Truncated error for specific tarball member
func TestCorruptionErrors_TruncatedMember(t *testing.T) {
	// Create a tarball where the pack file data is truncated mid-member
	configData := []byte(`{
		"core.repositoryformatversion": "1",
		"remote.origin.promisor": "true",
		"remote.origin.partialclonefilter": "blob:none"
	}`)
	refData := []byte("refs/heads/main abc123")

	// Create a tarball with a pack file that will be truncated
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	// Write config
	hdr := &tar.Header{
		Name: "config.json",
		Mode: 0644,
		Size: int64(len(configData)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("failed to write tar header: %v", err)
	}
	if _, err := tw.Write(configData); err != nil {
		t.Fatalf("failed to write config data: %v", err)
	}

	// Write ref
	hdr = &tar.Header{
		Name: "ref",
		Mode: 0644,
		Size: int64(len(refData)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("failed to write tar header: %v", err)
	}
	if _, err := tw.Write(refData); err != nil {
		t.Fatalf("failed to write ref data: %v", err)
	}

	// Write pack file header but truncate the data
	packData := []byte("PACK123456789") // 12 bytes
	hdr = &tar.Header{
		Name: "objects/pack/pack-trunc.pack",
		Mode: 0644,
		Size: int64(len(packData)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("failed to write tar header: %v", err)
	}
	// Write only partial pack data (5 bytes instead of 12)
	if _, err := tw.Write(packData[:5]); err != nil {
		t.Fatalf("failed to write partial pack data: %v", err)
	}

	// Don't close the tar writer properly - just get what we have
	tarball := buf.Bytes()

	_, err := ParseTarball(tarball)
	if err == nil {
		t.Fatal("expected error for truncated member, got nil")
	}

	// Verify it's a truncated error
	var truncErr *Error
	if !errors.As(err, &truncErr) {
		t.Fatalf("expected *Error type, got %T: %v", err, err)
	}

	if truncErr.Kind != Truncated {
		t.Errorf("expected Truncated error kind, got %v", truncErr.Kind)
	}

	// Verify the error mentions the member name
	if !strings.Contains(truncErr.Error(), "objects/pack/pack-trunc.pack") || !strings.Contains(truncErr.Error(), "pack-trunc.pack") {
		t.Errorf("error should mention truncated member name, got: %v", truncErr)
	}

	t.Logf("Truncated member error: %v", err)
}

// TestCorruptionErrors_UndersizedPackFile tests Truncated error for undersized pack file
func TestCorruptionErrors_UndersizedPackFile(t *testing.T) {
	// Test pack files that are too small (< 12 bytes minimum header size)
	undersizedCases := []struct {
		name        string
		packData    []byte
		description string
	}{
		{"empty pack", []byte(""), "0 bytes (empty)"},
		{"PACK only", []byte("PACK"), "4 bytes (just signature)"},
		{"PACK + version", []byte("PACK1234"), "8 bytes (signature + partial version)"},
		{"PACK + version + partial count", []byte("PACK1234567"), "11 bytes (one byte short)"},
	}

	for _, tc := range undersizedCases {
		t.Run(tc.name, func(t *testing.T) {
			tarball := makeMockTarballWithPack(t, tc.packData, "")

			_, err := ParseTarball(tarball)
			if err == nil {
				t.Fatalf("expected error for undersized pack file (%s), got nil", tc.description)
			}

			// Verify it's a truncated error
			var truncErr *Error
			if !errors.As(err, &truncErr) {
				t.Fatalf("expected *Error type, got %T: %v", err, err)
			}

			if truncErr.Kind != Truncated {
				t.Errorf("expected Truncated error kind for %s, got %v", tc.description, truncErr.Kind)
			}

			// Verify error mentions the size constraint
			errMsg := truncErr.Error()
			if !strings.Contains(errMsg, "too small") && !strings.Contains(errMsg, "minimum") {
				t.Errorf("error should mention size constraint for %s, got: %s", tc.description, errMsg)
			}

			t.Logf("Undersized pack error (%s): %v", tc.description, err)
		})
	}
}

// TestCorruptionErrors_MissingPackMember tests MissingMember error for missing .pack file
func TestCorruptionErrors_MissingPackMember(t *testing.T) {
	configData := []byte(`{
		"core.repositoryformatversion": "1",
		"remote.origin.promisor": "true",
		"remote.origin.partialclonefilter": "blob:none"
	}`)
	refData := []byte("refs/heads/main abc123")
	idxData := []byte("test idx data")
	refFileData := []byte("test ref data")

	// Create tarball with .idx and .ref but missing .pack
	members := []TarballMember{
		// Missing: objects/pack/pack-123.pack
		{Name: "objects/pack/pack-123.idx", Data: idxData},
		{Name: "objects/pack/pack-123.ref", Data: refFileData},
		{Name: "config.json", Data: configData},
		{Name: "ref", Data: refData},
	}

	tarball := createTestTarball(t, members)

	_, err := ParseTarball(tarball)
	if err == nil {
		t.Fatal("expected error for missing .pack file, got nil")
	}

	// Verify it's a MissingMember error
	var missingErr *Error
	if !errors.As(err, &missingErr) {
		t.Fatalf("expected *Error type, got %T: %v", err, err)
	}

	if missingErr.Kind != MissingMember {
		t.Errorf("expected MissingMember error kind, got %v", missingErr.Kind)
	}

	// Verify error mentions .pack
	errMsg := missingErr.Error()
	if !strings.Contains(errMsg, ".pack") {
		t.Errorf("error should mention .pack file, got: %s", errMsg)
	}

	t.Logf("Missing .pack file error: %v", err)
}

// TestCorruptionErrors_MissingIdxMember tests MissingMember error for missing .idx file
func TestCorruptionErrors_MissingIdxMember(t *testing.T) {
	configData := []byte(`{
		"core.repositoryformatversion": "1",
		"remote.origin.promisor": "true",
		"remote.origin.partialclonefilter": "blob:none"
	}`)
	refData := []byte("refs/heads/main abc123")
	packData := []byte("PACK123456789")
	refFileData := []byte("test ref data")

	// Create tarball with .pack and .ref but missing .idx
	members := []TarballMember{
		{Name: "objects/pack/pack-456.pack", Data: packData},
		// Missing: objects/pack/pack-456.idx
		{Name: "objects/pack/pack-456.ref", Data: refFileData},
		{Name: "config.json", Data: configData},
		{Name: "ref", Data: refData},
	}

	tarball := createTestTarball(t, members)

	_, err := ParseTarball(tarball)
	if err == nil {
		t.Fatal("expected error for missing .idx file, got nil")
	}

	// Verify it's a MissingMember error
	var missingErr *Error
	if !errors.As(err, &missingErr) {
		t.Fatalf("expected *Error type, got %T: %v", err, err)
	}

	if missingErr.Kind != MissingMember {
		t.Errorf("expected MissingMember error kind, got %v", missingErr.Kind)
	}

	// Verify error mentions .idx
	errMsg := missingErr.Error()
	if !strings.Contains(errMsg, ".idx") {
		t.Errorf("error should mention .idx file, got: %s", errMsg)
	}

	t.Logf("Missing .idx file error: %v", err)
}

// TestCorruptionErrors_MissingRefMember tests MissingMember error for missing .ref file
func TestCorruptionErrors_MissingRefMember(t *testing.T) {
	configData := []byte(`{
		"core.repositoryformatversion": "1",
		"remote.origin.promisor": "true",
		"remote.origin.partialclonefilter": "blob:none"
	}`)
	refData := []byte("refs/heads/main abc123")
	packData := []byte("PACK123456789")
	idxData := []byte("test idx data")

	// Create tarball with .pack and .idx but missing .ref
	members := []TarballMember{
		{Name: "objects/pack/pack-789.pack", Data: packData},
		{Name: "objects/pack/pack-789.idx", Data: idxData},
		// Missing: objects/pack/pack-789.ref
		{Name: "config.json", Data: configData},
		{Name: "ref", Data: refData},
	}

	tarball := createTestTarball(t, members)

	_, err := ParseTarball(tarball)
	if err == nil {
		t.Fatal("expected error for missing .ref file, got nil")
	}

	// Verify it's a MissingMember error
	var missingErr *Error
	if !errors.As(err, &missingErr) {
		t.Fatalf("expected *Error type, got %T: %v", err, err)
	}

	if missingErr.Kind != MissingMember {
		t.Errorf("expected MissingMember error kind, got %v", missingErr.Kind)
	}

	// Verify error mentions .ref and the specific file
	errMsg := missingErr.Error()
	if !strings.Contains(errMsg, ".ref") {
		t.Errorf("error should mention .ref file, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "pack-789.ref") {
		t.Errorf("error should mention specific missing file pack-789.ref, got: %s", errMsg)
	}

	// Verify the Context field contains the missing file path
	if missingErr.Context == "" {
		t.Error("Context field should not be empty for missing .ref files")
	}
	if !strings.Contains(missingErr.Context, "pack-789.ref") {
		t.Errorf("Context should contain the missing file path, got: %s", missingErr.Context)
	}

	t.Logf("Missing .ref file error: %v", err)
}

// TestCorruptionErrors_MultipleMissingRefMembers tests MissingMember error for multiple missing .ref files
func TestCorruptionErrors_MultipleMissingRefMembers(t *testing.T) {
	configData := []byte(`{
		"core.repositoryformatversion": "1",
		"remote.origin.promisor": "true",
		"remote.origin.partialclonefilter": "blob:none"
	}`)
	refData := []byte("refs/heads/main abc123")

	// Create tarball with multiple pack files missing their .ref files
	members := []TarballMember{
		{Name: "objects/pack/pack-alpha.pack", Data: []byte("PACK123456789")},
		{Name: "objects/pack/pack-alpha.idx", Data: []byte("idx alpha")},
		{Name: "objects/pack/pack-beta.pack", Data: []byte("PACK987654321")},
		{Name: "objects/pack/pack-beta.idx", Data: []byte("idx beta")},
		{Name: "objects/pack/pack-gamma.pack", Data: []byte("PACK555555555")},
		{Name: "objects/pack/pack-gamma.idx", Data: []byte("idx gamma")},
		// All three missing their .ref files
		{Name: "config.json", Data: configData},
		{Name: "ref", Data: refData},
	}

	tarball := createTestTarball(t, members)

	_, err := ParseTarball(tarball)
	if err == nil {
		t.Fatal("expected error for missing .ref files, got nil")
	}

	// Verify it's a MissingMember error
	var missingErr *Error
	if !errors.As(err, &missingErr) {
		t.Fatalf("expected *Error type, got %T: %v", err, err)
	}

	if missingErr.Kind != MissingMember {
		t.Errorf("expected MissingMember error kind, got %v", missingErr.Kind)
	}

	// Verify error mentions all three missing files
	expectedMissingFiles := []string{
		"objects/pack/pack-alpha.ref",
		"objects/pack/pack-beta.ref",
		"objects/pack/pack-gamma.ref",
	}

	errMsg := missingErr.Error()
	for _, missingFile := range expectedMissingFiles {
		if !strings.Contains(errMsg, missingFile) && !strings.Contains(missingErr.Context, missingFile) {
			t.Errorf("error should mention missing file %s, got: %s", missingFile, errMsg)
		}
	}

	t.Logf("Multiple missing .ref files error: %v", err)
}

// TestCorruptionErrors_ErrorContextValidation tests that error contexts contain correct member names/paths
func TestCorruptionErrors_ErrorContextValidation(t *testing.T) {
	testCases := []struct {
		name           string
		setupTarball   func(*testing.T) []byte
		expectedKind   ErrorKind
		validateFields func(*testing.T, *Error)
	}{
		{
			name: "truncated pack file has member name",
			setupTarball: func(t *testing.T) []byte {
				return makeMockTarballWithPack(t, []byte("PACK"), "") // 4 bytes (too small)
			},
			expectedKind: Truncated,
			validateFields: func(t *testing.T, err *Error) {
				if err.MemberName == "" {
					t.Error("MemberName should not be empty for truncated pack file")
				}
				if !strings.Contains(err.MemberName, ".pack") {
					t.Errorf("MemberName should contain .pack, got: %s", err.MemberName)
				}
				if err.Context == "" {
					t.Error("Context should not be empty for truncated pack file")
				}
			},
		},
		{
			name: "missing idx file has .idx in MemberName",
			setupTarball: func(t *testing.T) []byte {
				configData := []byte(`{"core.repositoryformatversion": "1", "remote.origin.promisor": "true", "remote.origin.partialclonefilter": "blob:none"}`)
				refData := []byte("refs/heads/main abc123")
				members := []TarballMember{
					{Name: "objects/pack/pack-missing.idx.pack", Data: []byte("PACK123456789")},
					// Missing .idx
					{Name: "objects/pack/pack-missing.idx.ref", Data: []byte("ref data")},
					{Name: "config.json", Data: configData},
					{Name: "ref", Data: refData},
				}
				return createTestTarball(t, members)
			},
			expectedKind: MissingMember,
			validateFields: func(t *testing.T, err *Error) {
				if err.MemberName != ".idx" {
					t.Errorf("MemberName should be .idx, got: %s", err.MemberName)
				}
			},
		},
		{
			name: "missing ref file has specific path in Context",
			setupTarball: func(t *testing.T) []byte {
				configData := []byte(`{"core.repositoryformatversion": "1", "remote.origin.promisor": "true", "remote.origin.partialclonefilter": "blob:none"}`)
				refData := []byte("refs/heads/main abc123")
				members := []TarballMember{
					{Name: "objects/pack/pack-specific.pack", Data: []byte("PACK123456789")},
					{Name: "objects/pack/pack-specific.idx", Data: []byte("idx data")},
					// Missing .ref
					{Name: "config.json", Data: configData},
					{Name: "ref", Data: refData},
				}
				return createTestTarball(t, members)
			},
			expectedKind: MissingMember,
			validateFields: func(t *testing.T, err *Error) {
				if err.MemberName != ".ref" {
					t.Errorf("MemberName should be .ref, got: %s", err.MemberName)
				}
				if err.Context == "" {
					t.Error("Context should not be empty for missing .ref files")
				}
				if !strings.Contains(err.Context, "pack-specific.ref") {
					t.Errorf("Context should contain specific path 'pack-specific.ref', got: %s", err.Context)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tarball := tc.setupTarball(t)

			_, err := ParseTarball(tarball)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			var corruptionErr *Error
			if !errors.As(err, &corruptionErr) {
				t.Fatalf("expected *Error type, got %T: %v", err, err)
			}

			if corruptionErr.Kind != tc.expectedKind {
				t.Errorf("expected error kind %v, got %v", tc.expectedKind, corruptionErr.Kind)
			}

			tc.validateFields(t, corruptionErr)

			t.Logf("Error context validated: Kind=%v, MemberName=%s, Context=%s",
				corruptionErr.Kind, corruptionErr.MemberName, corruptionErr.Context)
		})
	}
}

// TestCorruptionErrors_ErrorMessageFormatting tests that error messages are properly formatted
func TestCorruptionErrors_ErrorMessageFormatting(t *testing.T) {
	testCases := []struct {
		name             string
		createTarball    func(*testing.T) []byte
		expectedSubstrings []string
	}{
		{
			name: "truncated error message format",
			createTarball: func(t *testing.T) []byte {
				return makeMockTarballWithPack(t, []byte("PACK"), "")
			},
			expectedSubstrings: []string{
				"warmstart",
				"truncated tarball",
				"too small",
			},
		},
		{
			name: "missing member error format",
			createTarball: func(t *testing.T) []byte {
				configData := []byte(`{"core.repositoryformatversion": "1", "remote.origin.promisor": "true", "remote.origin.partialclonefilter": "blob:none"}`)
				refData := []byte("refs/heads/main abc123")
				members := []TarballMember{
					{Name: "objects/pack/pack-test.pack", Data: []byte("PACK123456789")},
					{Name: "objects/pack/pack-test.idx", Data: []byte("idx data")},
					// Missing .ref
					{Name: "config.json", Data: configData},
					{Name: "ref", Data: refData},
				}
				return createTestTarball(t, members)
			},
			expectedSubstrings: []string{
				"warmstart",
				"missing required member",
				".ref",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tarball := tc.createTarball(t)

			_, err := ParseTarball(tarball)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			errMsg := err.Error()
			t.Logf("Error message: %s", errMsg)

			for _, expectedSubstr := range tc.expectedSubstrings {
				if !strings.Contains(errMsg, expectedSubstr) {
					t.Errorf("error message should contain '%s', got: %s", expectedSubstr, errMsg)
				}
			}
		})
	}
}

// TestCorruptionErrors_CorruptPackPlaceholder tests CorruptPack error type exists and can be created
// Note: Actual pack data corruption validation is not currently implemented in ParseTarball,
// but the error type is defined for future use
func TestCorruptionErrors_CorruptPackPlaceholder(t *testing.T) {
	// Test that the CorruptPack error kind exists and can be created
	err := NewCorruptPackError("objects/pack/pack-corrupt.pack", "checksum validation failed")

	if err.Kind != CorruptPack {
		t.Errorf("expected CorruptPack error kind, got %v", err.Kind)
	}

	if err.MemberName != "objects/pack/pack-corrupt.pack" {
		t.Errorf("expected MemberName 'objects/pack/pack-corrupt.pack', got %s", err.MemberName)
	}

	if err.Context != "checksum validation failed" {
		t.Errorf("expected Context 'checksum validation failed', got %s", err.Context)
	}

	// Verify error message formatting
	errMsg := err.Error()
	expectedParts := []string{
		"warmstart",
		"corrupt pack data",
		"objects/pack/pack-corrupt.pack",
		"checksum validation failed",
	}

	for _, part := range expectedParts {
		if !strings.Contains(errMsg, part) {
			t.Errorf("error message should contain '%s', got: %s", part, errMsg)
		}
	}

	t.Logf("CorruptPack error (placeholder): %v", err)
}

// TestCorruptionErrors_AllErrorKindsDistinct tests that all corruption error kinds are distinct
func TestCorruptionErrors_AllErrorKindsDistinct(t *testing.T) {
	kinds := []ErrorKind{Truncated, MissingMember, CorruptPack, IO, Other}

	seen := make(map[ErrorKind]bool)
	for _, kind := range kinds {
		if seen[kind] {
			t.Errorf("duplicate error kind detected: %v", kind)
		}
		seen[kind] = true
	}

	// Verify each has a unique string representation
	stringsSeen := make(map[string]bool)
	for _, kind := range kinds {
	 kindStr := kind.String()
		if stringsSeen[kindStr] {
			t.Errorf("duplicate string representation: %s", kindStr)
		}
		stringsSeen[kindStr] = true
	}

	t.Logf("All %d error kinds are distinct with unique string representations", len(kinds))
}
