package warmstart

import (
	"archive/tar"
	"bytes"
	"errors"
	"strings"
	"testing"
)

// TestMissingRefErrorMessage_SingleMissing verifies error message content when a single .ref file is missing.
func TestMissingRefErrorMessage_SingleMissing(t *testing.T) {
	configData := []byte(`{
		"core.repositoryformatversion": "1",
		"remote.origin.promisor": "true",
		"remote.origin.partialclonefilter": "blob:none"
	}`)
	refData := []byte("refs/heads/main abc123")
	idxData := []byte("test idx data")

	members := []TarballMember{
		{Name: "objects/pack/pack-123.pack", Data: []byte("PACK123456789")}, // 12 bytes, minimum valid header
		{Name: "objects/pack/pack-123.idx", Data: idxData},
		// Missing pack-123.ref
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

	// Verify error message contains the missing file name
	errMsg := missingErr.Error()
	if !strings.Contains(errMsg, "objects/pack/pack-123.ref") {
		t.Errorf("error message should contain missing file 'objects/pack/pack-123.ref', got: %s", errMsg)
	}

	// Verify error message mentions ".ref"
	if !strings.Contains(errMsg, ".ref") {
		t.Errorf("error message should mention '.ref', got: %s", errMsg)
	}

	// Verify error message mentions "missing"
	if !strings.Contains(errMsg, "missing") {
		t.Errorf("error message should mention 'missing', got: %s", errMsg)
	}

	t.Logf("Single missing .ref file error message: %s", errMsg)
}

// TestMissingRefErrorMessage_MultipleMissing verifies error message contains all missing .ref files.
func TestMissingRefErrorMessage_MultipleMissing(t *testing.T) {
	configData := []byte(`{
		"core.repositoryformatversion": "1",
		"remote.origin.promisor": "true",
		"remote.origin.partialclonefilter": "blob:none"
	}`)
	refData := []byte("refs/heads/main abc123")
	idxData1 := []byte("idx 1")
	idxData2 := []byte("idx 2")
	idxData3 := []byte("idx 3")

	members := []TarballMember{
		{Name: "objects/pack/pack-abc.pack", Data: []byte("PACK123456789")},
		{Name: "objects/pack/pack-abc.idx", Data: idxData1},
		// Missing pack-abc.ref
		{Name: "objects/pack/pack-def.pack", Data: []byte("PACK987654321")},
		{Name: "objects/pack/pack-def.idx", Data: idxData2},
		// Missing pack-def.ref
		{Name: "objects/pack/pack-ghi.pack", Data: []byte("PACK555555555")},
		{Name: "objects/pack/pack-ghi.idx", Data: idxData3},
		// Missing pack-ghi.ref
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

	// Verify error message contains all three missing files
	errMsg := missingErr.Error()

	expectedMissingFiles := []string{
		"objects/pack/pack-abc.ref",
		"objects/pack/pack-def.ref",
		"objects/pack/pack-ghi.ref",
	}

	for _, missingFile := range expectedMissingFiles {
		if !strings.Contains(errMsg, missingFile) {
			t.Errorf("error message should contain missing file '%s', got: %s", missingFile, errMsg)
		}
	}

	// Verify error message mentions ".ref"
	if !strings.Contains(errMsg, ".ref") {
		t.Errorf("error message should mention '.ref', got: %s", errMsg)
	}

	// Verify error message mentions "missing"
	if !strings.Contains(errMsg, "missing") {
		t.Errorf("error message should mention 'missing', got: %s", errMsg)
	}

	t.Logf("Multiple missing .ref files error message: %s", errMsg)
}

// TestMissingRefErrorMessage_CompleteList verifies the complete list of missing files is included.
func TestMissingRefErrorMessage_CompleteList(t *testing.T) {
	configData := []byte(`{
		"core.repositoryformatversion": "1",
		"remote.origin.promisor": "true",
		"remote.origin.partialclonefilter": "blob:none"
	}`)
	refData := []byte("refs/heads/main abc123")

	// Create 5 pack files, all missing .ref files
	members := []TarballMember{
		{Name: "objects/pack/pack-001.pack", Data: []byte("PACK123456789")},
		{Name: "objects/pack/pack-001.idx", Data: []byte("idx 1")},
		{Name: "objects/pack/pack-002.pack", Data: []byte("PACK987654321")},
		{Name: "objects/pack/pack-002.idx", Data: []byte("idx 2")},
		{Name: "objects/pack/pack-003.pack", Data: []byte("PACK555555555")},
		{Name: "objects/pack/pack-003.idx", Data: []byte("idx 3")},
		{Name: "objects/pack/pack-004.pack", Data: []byte("PACK111111111")},
		{Name: "objects/pack/pack-004.idx", Data: []byte("idx 4")},
		{Name: "objects/pack/pack-005.pack", Data: []byte("PACK222222222")},
		{Name: "objects/pack/pack-005.idx", Data: []byte("idx 5")},
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

	// Verify error message contains all 5 missing files
	errMsg := missingErr.Error()

	expectedMissingFiles := []string{
		"objects/pack/pack-001.ref",
		"objects/pack/pack-002.ref",
		"objects/pack/pack-003.ref",
		"objects/pack/pack-004.ref",
		"objects/pack/pack-005.ref",
	}

	missingCount := 0
	for _, missingFile := range expectedMissingFiles {
		if strings.Contains(errMsg, missingFile) {
			missingCount++
		} else {
			t.Errorf("error message should contain missing file '%s', got: %s", missingFile, errMsg)
		}
	}

	if missingCount != len(expectedMissingFiles) {
		t.Errorf("expected all %d missing files in error message, found %d", len(expectedMissingFiles), missingCount)
	}

	t.Logf("Complete list (5 files) error message: %s", errMsg)
}

// TestMissingRefErrorMessage_EdgeCaseEmptyList verifies behavior when no .ref files are missing.
func TestMissingRefErrorMessage_EdgeCaseEmptyList(t *testing.T) {
	configData := []byte(`{
		"core.repositoryformatversion": "1",
		"remote.origin.promisor": "true",
		"remote.origin.partialclonefilter": "blob:none"
	}`)
	refData := []byte("refs/heads/main abc123")
	packData := []byte("PACK123456789")
	idxData := []byte("test idx data")
	refFileData := []byte("test ref data")

	members := []TarballMember{
		{Name: "objects/pack/pack-123.pack", Data: packData},
		{Name: "objects/pack/pack-123.idx", Data: idxData},
		{Name: "objects/pack/pack-123.ref", Data: refFileData}, // Present
		{Name: "config.json", Data: configData},
		{Name: "ref", Data: refData},
	}

	tarball := createTestTarball(t, members)

	snapshot, err := ParseTarball(tarball)
	if err != nil {
		t.Fatalf("expected success when no .ref files are missing, got error: %v", err)
	}

	// Verify snapshot was created successfully
	if snapshot == nil {
		t.Error("expected non-nil snapshot when no .ref files are missing")
	}

	// Verify no error about missing .ref files
	t.Log("No error when all .ref files are present (edge case: empty missing list)")
}

// TestMissingRefErrorMessage_EdgeCaseDuplicates verifies error message when same pack appears multiple times.
func TestMissingRefErrorMessage_EdgeCaseDuplicates(t *testing.T) {
	configData := []byte(`{
		"core.repositoryformatversion": "1",
		"remote.origin.promisor": "true",
		"remote.origin.partialclonefilter": "blob:none"
	}`)
	refData := []byte("refs/heads/main abc123")

	// Create tarball with duplicate pack file names
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

	// Write same pack file twice (simulating duplicate entries)
	packData := []byte("PACK123456789")
	idxData := []byte("idx data")

	// First pack-123.pack
	hdr = &tar.Header{
		Name: "objects/pack/pack-123.pack",
		Mode: 0644,
		Size: int64(len(packData)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("failed to write tar header: %v", err)
	}
	if _, err := tw.Write(packData); err != nil {
		t.Fatalf("failed to write pack data: %v", err)
	}

	// First pack-123.idx
	hdr = &tar.Header{
		Name: "objects/pack/pack-123.idx",
		Mode: 0644,
		Size: int64(len(idxData)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("failed to write tar header: %v", err)
	}
	if _, err := tw.Write(idxData); err != nil {
		t.Fatalf("failed to write idx data: %v", err)
	}

	// Second pack-123.pack (duplicate)
	hdr = &tar.Header{
		Name: "objects/pack/pack-123.pack",
		Mode: 0644,
		Size: int64(len(packData)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("failed to write tar header: %v", err)
	}
	if _, err := tw.Write(packData); err != nil {
		t.Fatalf("failed to write pack data: %v", err)
	}

	// Second pack-123.idx (duplicate)
	hdr = &tar.Header{
		Name: "objects/pack/pack-123.idx",
		Mode: 0644,
		Size: int64(len(idxData)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("failed to write tar header: %v", err)
	}
	if _, err := tw.Write(idxData); err != nil {
		t.Fatalf("failed to write idx data: %v", err)
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("failed to close tar writer: %v", err)
	}

	tarball := buf.Bytes()

	_, err := ParseTarball(tarball)
	if err == nil {
		t.Fatal("expected error for missing .ref file, got nil")
	}

	// Verify it's a MissingMember error
	var missingErr *Error
	if !errors.As(err, &missingErr) {
		t.Fatalf("expected *Error type, got %T: %v", err, err)
	}

	// Verify error message contains the missing .ref file
	// (pack-123.ref may appear once or twice depending on implementation)
	errMsg := missingErr.Error()
	if !strings.Contains(errMsg, "pack-123.ref") {
		t.Errorf("error message should contain 'pack-123.ref', got: %s", errMsg)
	}

	t.Logf("Duplicate pack entries error message: %s", errMsg)
}

// TestMissingRefErrorMessage_ContextField verifies the Context field contains the complete list.
func TestMissingRefErrorMessage_ContextField(t *testing.T) {
	configData := []byte(`{
		"core.repositoryformatversion": "1",
		"remote.origin.promisor": "true",
		"remote.origin.partialclonefilter": "blob:none"
	}`)
	refData := []byte("refs/heads/main abc123")

	members := []TarballMember{
		{Name: "objects/pack/pack-alpha.pack", Data: []byte("PACK123456789")},
		{Name: "objects/pack/pack-alpha.idx", Data: []byte("idx alpha")},
		// Missing pack-alpha.ref
		{Name: "objects/pack/pack-beta.pack", Data: []byte("PACK987654321")},
		{Name: "objects/pack/pack-beta.idx", Data: []byte("idx beta")},
		// Missing pack-beta.ref
		{Name: "objects/pack/pack-gamma.pack", Data: []byte("PACK555555555")},
		{Name: "objects/pack/pack-gamma.idx", Data: []byte("idx gamma")},
		// Missing pack-gamma.ref
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

	// Verify Context field contains the list of missing files
	if missingErr.Context == "" {
		t.Error("Context field should not be empty when .ref files are missing")
	}

	// Check that all expected files are in the context
	expectedFiles := []string{
		"objects/pack/pack-alpha.ref",
		"objects/pack/pack-beta.ref",
		"objects/pack/pack-gamma.ref",
	}

	for _, expectedFile := range expectedFiles {
		if !strings.Contains(missingErr.Context, expectedFile) {
			t.Errorf("Context should contain '%s', got: %s", expectedFile, missingErr.Context)
		}
	}

	t.Logf("Context field with complete list: %s", missingErr.Context)
}

// TestMissingRefErrorMessage_FormattedProperly verifies error message formatting is correct.
func TestMissingRefErrorMessage_FormattedProperly(t *testing.T) {
	configData := []byte(`{
		"core.repositoryformatversion": "1",
		"remote.origin.promisor": "true",
		"remote.origin.partialclonefilter": "blob:none"
	}`)
	refData := []byte("refs/heads/main abc123")
	idxData := []byte("test idx data")

	members := []TarballMember{
		{Name: "objects/pack/pack-test.pack", Data: []byte("PACK123456789")},
		{Name: "objects/pack/pack-test.idx", Data: idxData},
		// Missing pack-test.ref
		{Name: "config.json", Data: configData},
		{Name: "ref", Data: refData},
	}

	tarball := createTestTarball(t, members)

	_, err := ParseTarball(tarball)
	if err == nil {
		t.Fatal("expected error for missing .ref file, got nil")
	}

	// Verify error message structure
	errMsg := err.Error()

	// Check for expected format components
	expectedComponents := []string{
		"warmstart",
		"missing required member",
		"member=.ref",
	}

	for _, component := range expectedComponents {
		if !strings.Contains(errMsg, component) {
			t.Errorf("error message should contain '%s', got: %s", component, errMsg)
		}
	}

	// Verify the file path is present
	if !strings.Contains(errMsg, "objects/pack/pack-test.ref") {
		t.Errorf("error message should contain the missing file path, got: %s", errMsg)
	}

	t.Logf("Properly formatted error message: %s", errMsg)
}
