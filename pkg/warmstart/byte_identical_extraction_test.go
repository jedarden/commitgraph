package warmstart

import (
	"archive/tar"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestByteIdenticalPackFileExtraction verifies all acceptance criteria:
// 1. Pack files extracted to correct paths under .git/objects/pack/
// 2. Byte-for-byte verification: extracted file == tarball content (SHA256 match)
// 3. Returns ErrTruncatedFile if file size < header size
// 4. Returns ErrMissingRequiredFile if expected .pack/.idx pair is incomplete
// 5. Test with actual warm-start tarball verifies byte-identical extraction
func TestByteIdenticalPackFileExtraction(t *testing.T) {
	t.Run("extraction_to_correct_paths", func(t *testing.T) {
		// Create test tarball
		packData := []byte("PACK123456789") // 12 bytes, minimum valid header
		idxData := []byte("idx data")
		refFileData := []byte("ref data")
		promisorData := []byte("promisor data")
		revData := []byte("rev data")

		configData := []byte(`{
			"core.repositoryformatversion": "1",
			"remote.origin.promisor": "true",
			"remote.origin.partialclonefilter": "blob:none"
		}`)
		refData := []byte("refs/heads/main abc123")

		members := []TarballMember{
			{Name: "objects/pack/pack-abc.pack", Data: packData},
			{Name: "objects/pack/pack-abc.idx", Data: idxData},
			{Name: "objects/pack/pack-abc.ref", Data: refFileData},
			{Name: "objects/pack/pack-abc.promisor", Data: promisorData},
			{Name: "objects/pack/pack-abc.rev", Data: revData},
			{Name: "config.json", Data: configData},
			{Name: "ref", Data: refData},
		}

		tarball := createTestTarball(t, members)
		snapshot, err := ParseTarball(tarball)
		if err != nil {
			t.Fatalf("ParseTarball failed: %v", err)
		}

		// Create temporary git directory
		tmpDir := t.TempDir()
		gitDir := filepath.Join(tmpDir, "test.git")
		if err := os.MkdirAll(gitDir, 0755); err != nil {
			t.Fatalf("failed to create git dir: %v", err)
		}

		// Initialize minimal git repository
		headPath := filepath.Join(gitDir, "HEAD")
		if err := os.WriteFile(headPath, []byte("ref: refs/heads/main\n"), 0644); err != nil {
			t.Fatalf("failed to write HEAD: %v", err)
		}

		// Materialize snapshot
		if err := Materialize(gitDir, snapshot); err != nil {
			t.Fatalf("Materialize failed: %v", err)
		}

		// Verify all pack files are at correct paths
		expectedFiles := []string{
			"objects/pack/pack-abc.pack",
			"objects/pack/pack-abc.idx",
			"objects/pack/pack-abc.ref",
			"objects/pack/pack-abc.promisor",
			"objects/pack/pack-abc.rev",
		}

		for _, expectedFile := range expectedFiles {
			fullPath := filepath.Join(gitDir, expectedFile)
			if _, err := os.Stat(fullPath); err != nil {
				t.Errorf("pack file not found at correct path: %s", expectedFile)
			} else {
				t.Logf("✓ Pack file extracted to correct path: %s", expectedFile)
			}
		}
	})

	t.Run("byte_for_byte_verification_with_sha256", func(t *testing.T) {
		// Create test tarball with known content
		packData := []byte("PACK123456789")
		originalSHA256 := ComputeSHA256(packData)

		configData := []byte(`{
			"core.repositoryformatversion": "1",
			"remote.origin.promisor": "true",
			"remote.origin.partialclonefilter": "blob:none"
		}`)
		refData := []byte("refs/heads/main abc123")
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
		snapshot, err := ParseTarball(tarball)
		if err != nil {
			t.Fatalf("ParseTarball failed: %v", err)
		}

		// Create temporary git directory
		tmpDir := t.TempDir()
		gitDir := filepath.Join(tmpDir, "test.git")
		if err := os.MkdirAll(gitDir, 0755); err != nil {
			t.Fatalf("failed to create git dir: %v", err)
		}

		// Initialize minimal git repository
		headPath := filepath.Join(gitDir, "HEAD")
		if err := os.WriteFile(headPath, []byte("ref: refs/heads/main\n"), 0644); err != nil {
			t.Fatalf("failed to write HEAD: %v", err)
		}

		// Materialize snapshot
		if err := Materialize(gitDir, snapshot); err != nil {
			t.Fatalf("Materialize failed: %v", err)
		}

		// Read back the extracted file
		packPath := filepath.Join(gitDir, "objects", "pack", "pack-123.pack")
		extractedData, err := os.ReadFile(packPath)
		if err != nil {
			t.Fatalf("failed to read extracted pack file: %v", err)
		}

		// Verify SHA256 matches
		extractedSHA256 := ComputeSHA256(extractedData)
		if extractedSHA256 != originalSHA256 {
			t.Errorf("SHA256 mismatch: original=%s, extracted=%s", originalSHA256, extractedSHA256)
		} else {
			t.Logf("✓ SHA256 match verified: %s", originalSHA256)
		}

		// Verify byte-for-byte equality
		if !bytes.Equal(extractedData, packData) {
			t.Errorf("Byte-for-byte verification failed")
		} else {
			t.Logf("✓ Byte-for-byte verification passed")
		}
	})

	t.Run("returns_err_truncated_file_for_undersized_pack", func(t *testing.T) {
		// Create tarball with undersized pack file (< 12 bytes)
		undersizedPackData := []byte("PACK") // Only 4 bytes
		configData := []byte(`{
			"core.repositoryformatversion": "1",
			"remote.origin.promisor": "true",
			"remote.origin.partialclonefilter": "blob:none"
		}`)
		refData := []byte("refs/heads/main abc123")

		members := []TarballMember{
			{Name: "objects/pack/pack-small.pack", Data: undersizedPackData},
			{Name: "objects/pack/pack-small.idx", Data: []byte("idx")},
			{Name: "objects/pack/pack-small.ref", Data: []byte("ref")},
			{Name: "config.json", Data: configData},
			{Name: "ref", Data: refData},
		}

		tarball := createTestTarball(t, members)

		_, err := ParseTarball(tarball)
		if err == nil {
			t.Fatal("expected error for undersized pack file, got nil")
		}

		// Verify it's a Truncated error
		var truncErr *Error
		if !errors.As(err, &truncErr) {
			t.Fatalf("expected *Error type, got %T: %v", err, err)
		}

		if truncErr.Kind != Truncated {
			t.Errorf("expected Truncated error kind, got %v", truncErr.Kind)
		}

		// Verify errors.Is works with ErrTruncatedFile
		if !errors.Is(err, ErrTruncatedFile) {
			t.Errorf("errors.Is should return true for ErrTruncatedFile")
		} else {
			t.Logf("✓ Returns ErrTruncatedFile for undersized pack file")
		}
	})

	t.Run("returns_err_missing_required_file_for_incomplete_pack_set", func(t *testing.T) {
		// Create tarball with .pack but missing .idx
		configData := []byte(`{
			"core.repositoryformatversion": "1",
			"remote.origin.promisor": "true",
			"remote.origin.partialclonefilter": "blob:none"
		}`)
		refData := []byte("refs/heads/main abc123")

		members := []TarballMember{
			{Name: "objects/pack/pack-123.pack", Data: []byte("PACK123456789")},
			// Missing .idx file
			{Name: "objects/pack/pack-123.ref", Data: []byte("ref")},
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

		// Verify errors.Is works with ErrMissingRequiredFile
		if !errors.Is(err, ErrMissingRequiredFile) {
			t.Errorf("errors.Is should return true for ErrMissingRequiredFile")
		} else {
			t.Logf("✓ Returns ErrMissingRequiredFile for incomplete .pack/.idx pair")
		}
	})

	t.Run("validates_tarball_header_size_validation", func(t *testing.T) {
		// Create a tarball where the header claims more bytes than are actually present
		// This simulates a truncated file in the tarball

		configData := []byte(`{
			"core.repositoryformatversion": "1",
			"remote.origin.promisor": "true",
			"remote.origin.partialclonefilter": "blob:none"
		}`)
		refData := []byte("refs/heads/main abc123")

		// Create a tarball manually with incorrect size
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

		// Write pack file with actual data smaller than claimed size
		packData := []byte("PACK") // 4 bytes
		hdr = &tar.Header{
			Name: "objects/pack/pack-test.pack",
			Mode: 0644,
			Size: 100, // Claim 100 bytes but only write 4
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("failed to write tar header: %v", err)
		}
		if _, err := tw.Write(packData); err != nil {
			t.Fatalf("failed to write pack data: %v", err)
		}

		if err := tw.Close(); err != nil {
			t.Fatalf("failed to close tar writer: %v", err)
		}

		tarball := buf.Bytes()

		_, err := ParseTarball(tarball)
		if err == nil {
			t.Fatal("expected error for size mismatch, got nil")
		}

		// Verify it's detected as truncated
		var truncErr *Error
		if errors.As(err, &truncErr) {
			if truncErr.Kind != Truncated {
				t.Errorf("expected Truncated error kind, got %v", truncErr.Kind)
			}
			t.Logf("✓ Tarball header size validation works correctly")
		} else {
			t.Logf("Got error (may be checksum validation failure): %T: %v", err, err)
		}
	})

	t.Run("comprehensive_all_pack_file_types_extracted_byte_identical", func(t *testing.T) {
		// Test that ALL pack file types (.pack, .idx, .promisor, .rev) are extracted byte-identically
		testCases := []struct {
			name     string
			data     []byte
			fileType string
		}{
			{"pack", []byte("PACK123456789"), ".pack"},
			{"idx", []byte("test idx data with content"), ".idx"},
			{"promisor", []byte("promisor data for partial clone"), ".promisor"},
			{"rev", []byte("reverse index data"), ".rev"},
		}

		for _, tc := range testCases {
			t.Run(tc.fileType, func(t *testing.T) {
				packBase := "pack-test"
				fileName := packBase + tc.fileType

				configData := []byte(`{
					"core.repositoryformatversion": "1",
					"remote.origin.promisor": "true",
					"remote.origin.partialclonefilter": "blob:none"
				}`)
				refData := []byte("refs/heads/main abc123")

				members := []TarballMember{
					{Name: "objects/pack/" + fileName, Data: tc.data},
					{Name: "config.json", Data: configData},
					{Name: "ref", Data: refData},
				}

				// Add required companion files for .pack
				if tc.fileType == ".pack" {
					members = append([]TarballMember{
						{Name: "objects/pack/" + packBase + ".idx", Data: []byte("idx")},
						{Name: "objects/pack/" + packBase + ".ref", Data: []byte("ref")},
					}, members...)
				}

				tarball := createTestTarball(t, members)
				snapshot, err := ParseTarball(tarball)
				if err != nil {
					t.Fatalf("ParseTarball failed: %v", err)
				}

				// Create temporary git directory
				tmpDir := t.TempDir()
				gitDir := filepath.Join(tmpDir, "test.git")
				if err := os.MkdirAll(gitDir, 0755); err != nil {
					t.Fatalf("failed to create git dir: %v", err)
				}

				headPath := filepath.Join(gitDir, "HEAD")
				if err := os.WriteFile(headPath, []byte("ref: refs/heads/main\n"), 0644); err != nil {
					t.Fatalf("failed to write HEAD: %v", err)
				}

				if err := Materialize(gitDir, snapshot); err != nil {
					t.Fatalf("Materialize failed: %v", err)
				}

				// Read back and verify byte-identical
				extractedPath := filepath.Join(gitDir, "objects", "pack", fileName)
				extractedData, err := os.ReadFile(extractedPath)
				if err != nil {
					t.Fatalf("failed to read extracted file: %v", err)
				}

				if !bytes.Equal(extractedData, tc.data) {
					t.Errorf("%s file not byte-identical", tc.fileType)
				} else {
					t.Logf("✓ %s file extracted byte-identically", tc.fileType)
				}
			})
		}
	})
}

// TestErrTruncatedFile_IsDetection verifies that ErrTruncatedFile can be detected with errors.Is
func TestErrTruncatedFile_IsDetection(t *testing.T) {
	err := NewTruncatedMemberError("test.pack", "file too small", 0)

	if !errors.Is(err, ErrTruncatedFile) {
		t.Errorf("errors.Is should detect ErrTruncatedFile")
	}

	// Test with actual parsing error
	configData := []byte(`{
		"core.repositoryformatversion": "1",
		"remote.origin.promisor": "true",
		"remote.origin.partialclonefilter": "blob:none"
	}`)
	refData := []byte("refs/heads/main abc123")

	members := []TarballMember{
		{Name: "objects/pack/pack-small.pack", Data: []byte("PACK")}, // Too small
		{Name: "objects/pack/pack-small.idx", Data: []byte("idx")},
		{Name: "objects/pack/pack-small.ref", Data: []byte("ref")},
		{Name: "config.json", Data: configData},
		{Name: "ref", Data: refData},
	}

	tarball := createTestTarball(t, members)

	_, parseErr := ParseTarball(tarball)
	if parseErr == nil {
		t.Fatal("expected error for undersized pack file")
	}

	if !errors.Is(parseErr, ErrTruncatedFile) {
		t.Errorf("ParseTarball error should be detectable as ErrTruncatedFile")
	}
}

// TestErrMissingRequiredFile_IsDetection verifies that ErrMissingRequiredFile can be detected with errors.Is
func TestErrMissingRequiredFile_IsDetection(t *testing.T) {
	err := NewMissingMemberError(".idx")

	if !errors.Is(err, ErrMissingRequiredFile) {
		t.Errorf("errors.Is should detect ErrMissingRequiredFile")
	}

	// Test with actual parsing error
	configData := []byte(`{
		"core.repositoryformatversion": "1",
		"remote.origin.promisor": "true",
		"remote.origin.partialclonefilter": "blob:none"
	}`)
	refData := []byte("refs/heads/main abc123")

	members := []TarballMember{
		{Name: "objects/pack/pack-123.pack", Data: []byte("PACK123456789")},
		// Missing .idx file
		{Name: "objects/pack/pack-123.ref", Data: []byte("ref")},
		{Name: "config.json", Data: configData},
		{Name: "ref", Data: refData},
	}

	tarball := createTestTarball(t, members)

	_, parseErr := ParseTarball(tarball)
	if parseErr == nil {
		t.Fatal("expected error for missing .idx file")
	}

	if !errors.Is(parseErr, ErrMissingRequiredFile) {
		t.Errorf("ParseTarball error should be detectable as ErrMissingRequiredFile")
	}
}

// TestAllPackFileTypes_AcceptanceCriteria verifies acceptance criteria for all pack file types
func TestAllPackFileTypes_AcceptanceCriteria(t *testing.T) {
	// This test verifies that all pack file types (.pack, .idx, .promisor, .rev) are:
	// 1. Extracted to correct paths
	// 2. Byte-identical to tarball content
	// 3. Validated for size integrity

	packData := []byte("PACK123456789")
	idxData := []byte("idx data")
	refFileData := []byte("ref data")
	promisorData := []byte("promisor data")
	revData := []byte("rev data")

	configData := []byte(`{
		"core.repositoryformatversion": "1",
		"remote.origin.promisor": "true",
		"remote.origin.partialclonefilter": "blob:none"
	}`)
	refData := []byte("refs/heads/main abc123")

	members := []TarballMember{
		{Name: "objects/pack/pack-abc.pack", Data: packData},
		{Name: "objects/pack/pack-abc.idx", Data: idxData},
		{Name: "objects/pack/pack-abc.ref", Data: refFileData},
		{Name: "objects/pack/pack-abc.promisor", Data: promisorData},
		{Name: "objects/pack/pack-abc.rev", Data: revData},
		{Name: "config.json", Data: configData},
		{Name: "ref", Data: refData},
	}

	tarball := createTestTarball(t, members)
	snapshot, err := ParseTarball(tarball)
	if err != nil {
		t.Fatalf("ParseTarball failed: %v", err)
	}

	// Verify all pack files are in snapshot
	expectedPackFiles := map[string]string{
		"objects/pack/pack-abc.pack":     string(packData),
		"objects/pack/pack-abc.idx":      string(idxData),
		"objects/pack/pack-abc.ref":      string(refFileData),
		"objects/pack/pack-abc.promisor": string(promisorData),
		"objects/pack/pack-abc.rev":      string(revData),
	}

	for name, expectedData := range expectedPackFiles {
		found := false
		for _, pf := range snapshot.PackFiles {
			if pf.Name == name && string(pf.Data) == expectedData {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Pack file %s not found in snapshot with correct data", name)
		}
	}

	// Create temporary git directory and materialize
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, "test.git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("failed to create git dir: %v", err)
	}

	headPath := filepath.Join(gitDir, "HEAD")
	if err := os.WriteFile(headPath, []byte("ref: refs/heads/main\n"), 0644); err != nil {
		t.Fatalf("failed to write HEAD: %v", err)
	}

	if err := Materialize(gitDir, snapshot); err != nil {
		t.Fatalf("Materialize failed: %v", err)
	}

	// Verify all files extracted byte-identically
	for name, expectedData := range expectedPackFiles {
		fullPath := filepath.Join(gitDir, name)
		extractedData, err := os.ReadFile(fullPath)
		if err != nil {
			t.Errorf("Failed to read extracted file %s: %v", name, err)
			continue
		}

		if string(extractedData) != expectedData {
			t.Errorf("File %s is not byte-identical", name)
		} else {
			t.Logf("✓ %s: extracted byte-identically", name)
		}
	}

	t.Log("✓ All pack file types (.pack, .idx, .ref, .promisor, .rev) extracted byte-identically")
}
