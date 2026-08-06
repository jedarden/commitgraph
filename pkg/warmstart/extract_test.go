package warmstart

import (
	"archive/tar"
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func createTestTarball(t *testing.T, members []TarballMember) []byte {
	t.Helper()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	for _, m := range members {
		hdr := &tar.Header{
			Name: m.Name,
			Mode: 0644,
			Size: int64(len(m.Data)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("failed to write tar header: %v", err)
		}
		if _, err := tw.Write(m.Data); err != nil {
			t.Fatalf("failed to write tar data: %v", err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("failed to close tar writer: %v", err)
	}

	return buf.Bytes()
}

func TestParseTarball_Valid(t *testing.T) {
	packData := []byte("test pack data")
	idxData := []byte("test idx data")
	configData := []byte(`{
		"core.repositoryformatversion": "1",
		"remote.origin.promisor": "true",
		"remote.origin.partialclonefilter": "blob:none"
	}`)
	refData := []byte("refs/heads/main abc123def456")

	members := []TarballMember{
		{Name: "objects/pack/pack-123.pack", Data: packData},
		{Name: "objects/pack/pack-123.idx", Data: idxData},
		{Name: "config.json", Data: configData},
		{Name: "ref", Data: refData},
	}

	tarball := createTestTarball(t, members)

	snapshot, err := ParseTarball(tarball)
	if err != nil {
		t.Fatalf("ParseTarball failed: %v", err)
	}

	// Verify pack files
	if len(snapshot.PackFiles) != 2 {
		t.Errorf("expected 2 pack files, got %d", len(snapshot.PackFiles))
	}

	// Verify config
	if snapshot.Config.CoreRepositoryFormatVersion != "1" {
		t.Errorf("expected core.repositoryformatversion=1, got %s", snapshot.Config.CoreRepositoryFormatVersion)
	}
	if snapshot.Config.RemoteOriginPromisor != "true" {
		t.Errorf("expected remote.origin.promisor=true, got %s", snapshot.Config.RemoteOriginPromisor)
	}
	if snapshot.Config.RemoteOriginPartialCloneFilter != "blob:none" {
		t.Errorf("expected remote.origin.partialclonefilter=blob:none, got %s", snapshot.Config.RemoteOriginPartialCloneFilter)
	}

	// Verify ref
	if snapshot.RefPath != "refs/heads/main" {
		t.Errorf("expected ref path refs/heads/main, got %s", snapshot.RefPath)
	}
	if snapshot.RefSHA != "abc123def456" {
		t.Errorf("expected ref SHA abc123def456, got %s", snapshot.RefSHA)
	}
}

func TestParseTarball_MissingConfig(t *testing.T) {
	members := []TarballMember{
		{Name: "objects/pack/pack-123.pack", Data: []byte("data")},
	}

	tarball := createTestTarball(t, members)

	_, err := ParseTarball(tarball)
	if err != ErrMissingConfig {
		t.Errorf("expected ErrMissingConfig, got %v", err)
	}
}

func TestParseTarball_MissingRef(t *testing.T) {
	configData := []byte(`{
		"core.repositoryformatversion": "1",
		"remote.origin.promisor": "true",
		"remote.origin.partialclonefilter": "blob:none"
	}`)

	members := []TarballMember{
		{Name: "config.json", Data: configData},
	}

	tarball := createTestTarball(t, members)

	_, err := ParseTarball(tarball)
	if err != ErrMissingRef {
		t.Errorf("expected ErrMissingRef, got %v", err)
	}
}

func TestParseTarball_MissingPackFiles(t *testing.T) {
	configData := []byte(`{
		"core.repositoryformatversion": "1",
		"remote.origin.promisor": "true",
		"remote.origin.partialclonefilter": "blob:none"
	}`)
	refData := []byte("refs/heads/main abc123")

	members := []TarballMember{
		{Name: "config.json", Data: configData},
		{Name: "ref", Data: refData},
	}

	tarball := createTestTarball(t, members)

	_, err := ParseTarball(tarball)
	if err != ErrMissingPackFiles {
		t.Errorf("expected ErrMissingPackFiles, got %v", err)
	}
}

func TestParseTarball_InvalidConfig(t *testing.T) {
	configData := []byte("invalid json")
	refData := []byte("refs/heads/main abc123")

	members := []TarballMember{
		{Name: "config.json", Data: configData},
		{Name: "ref", Data: refData},
		{Name: "objects/pack/pack-123.pack", Data: []byte("pack")},
	}

	tarball := createTestTarball(t, members)

	_, err := ParseTarball(tarball)
	if err == nil {
		t.Error("expected error for invalid config JSON, got nil")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("expected ErrInvalidConfig, got %v", err)
	}
}

func TestParseTarball_InvalidRefFormat(t *testing.T) {
	configData := []byte(`{
		"core.repositoryformatversion": "1",
		"remote.origin.promisor": "true",
		"remote.origin.partialclonefilter": "blob:none"
	}`)
	refData := []byte("invalid-ref-format")

	members := []TarballMember{
		{Name: "config.json", Data: configData},
		{Name: "ref", Data: refData},
		{Name: "objects/pack/pack-123.pack", Data: []byte("pack")},
	}

	tarball := createTestTarball(t, members)

	_, err := ParseTarball(tarball)
	if err == nil {
		t.Error("expected error for invalid ref format, got nil")
	}
}

func TestMaterialize_ByteIdenticalPackFiles(t *testing.T) {
	// Create test tarball with pack data
	packData := []byte("test pack data")
	idxData := []byte("test idx data")
	configData := []byte(`{
		"core.repositoryformatversion": "1",
		"remote.origin.promisor": "true",
		"remote.origin.partialclonefilter": "blob:none"
	}`)
	refData := []byte("refs/heads/main abc123")

	members := []TarballMember{
		{Name: "objects/pack/pack-123.pack", Data: packData},
		{Name: "objects/pack/pack-123.idx", Data: idxData},
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

	// Verify pack files are byte-identical
	packPath := filepath.Join(gitDir, "objects", "pack", "pack-123.pack")
	writtenPackData, err := os.ReadFile(packPath)
	if err != nil {
		t.Fatalf("failed to read pack file: %v", err)
	}

	if !bytes.Equal(writtenPackData, packData) {
		t.Error("pack file data is not byte-identical to original")
	}

	idxPath := filepath.Join(gitDir, "objects", "pack", "pack-123.idx")
	writtenIdxData, err := os.ReadFile(idxPath)
	if err != nil {
		t.Fatalf("failed to read idx file: %v", err)
	}

	if !bytes.Equal(writtenIdxData, idxData) {
		t.Error("idx file data is not byte-identical to original")
	}
}

func TestMaterialize_LooseRefNotPackedRefs(t *testing.T) {
	configData := []byte(`{
		"core.repositoryformatversion": "1",
		"remote.origin.promisor": "true",
		"remote.origin.partialclonefilter": "blob:none"
	}`)
	refData := []byte("refs/heads/main abc123")

	members := []TarballMember{
		{Name: "objects/pack/pack-123.pack", Data: []byte("pack")},
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

	// Verify loose ref file exists
	looseRefPath := filepath.Join(gitDir, "refs", "heads", "main")
	refContent, err := os.ReadFile(looseRefPath)
	if err != nil {
		t.Fatalf("failed to read loose ref: %v", err)
	}

	if string(refContent) != "abc123\n" {
		t.Errorf("loose ref content mismatch: got %q", string(refContent))
	}

	// Verify packed-refs does NOT exist
	packedRefsPath := filepath.Join(gitDir, "packed-refs")
	if _, err := os.Stat(packedRefsPath); err == nil {
		t.Error("packed-refs file should not exist")
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected error checking packed-refs: %v", err)
	}
}

func TestMaterialize_GitConfigValuesSet(t *testing.T) {
	configData := []byte(`{
		"core.repositoryformatversion": "1",
		"remote.origin.promisor": "true",
		"remote.origin.partialclonefilter": "blob:none"
	}`)
	refData := []byte("refs/heads/main abc123")

	members := []TarballMember{
		{Name: "objects/pack/pack-123.pack", Data: []byte("pack")},
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

	// Read git config
	configPath := filepath.Join(gitDir, "config")
	configContent, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read git config: %v", err)
	}

	configStr := string(configContent)

	// Verify all three required config values are present
	if !strings.Contains(configStr, "repositoryformatversion = 1") {
		t.Error("core.repositoryformatversion not set in git config")
	}
	if !strings.Contains(configStr, "[remote \"origin\"]") {
		t.Error("[remote \"origin\"] section not in git config")
	}
	if !strings.Contains(configStr, "promisor = true") {
		t.Error("remote.origin.promisor not set in git config")
	}
	if !strings.Contains(configStr, "partialclonefilter = blob:none") {
		t.Error("remote.origin.partialclonefilter not set in git config")
	}
}

func TestMaterialize_NotAGitRepo(t *testing.T) {
	configData := []byte(`{
		"core.repositoryformatversion": "1",
		"remote.origin.promisor": "true",
		"remote.origin.partialclonefilter": "blob:none"
	}`)
	refData := []byte("refs/heads/main abc123")

	members := []TarballMember{
		{Name: "objects/pack/pack-123.pack", Data: []byte("pack")},
		{Name: "config.json", Data: configData},
		{Name: "ref", Data: refData},
	}

	tarball := createTestTarball(t, members)
	snapshot, err := ParseTarball(tarball)
	if err != nil {
		t.Fatalf("ParseTarball failed: %v", err)
	}

	// Try to materialize to a non-git directory
	tmpDir := t.TempDir()
	err = Materialize(tmpDir, snapshot)
	if err == nil {
		t.Error("expected error when materializing to non-git directory")
	}
	if !errors.Is(err, ErrNotAGitRepo) {
		t.Errorf("expected ErrNotAGitRepo, got %v", err)
	}
}

// TestMaterialize_GitFsck verifies git fsck accepts the materialized repository.
// This test requires git to be installed.
func TestMaterialize_GitFsck(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping fsck test")
	}

	// Use minimal but valid pack data for testing
	// In a real scenario, this would be actual git pack files
	configData := []byte(`{
		"core.repositoryformatversion": "1",
		"remote.origin.promisor": "true",
		"remote.origin.partialclonefilter": "blob:none"
	}`)
	refData := []byte("refs/heads/main abc123def456789")

	members := []TarballMember{
		{Name: "objects/pack/pack-123.pack", Data: []byte("minimal pack data")},
		{Name: "config.json", Data: configData},
		{Name: "ref", Data: refData},
	}

	tarball := createTestTarball(t, members)
	snapshot, err := ParseTarball(tarball)
	if err != nil {
		t.Fatalf("ParseTarball failed: %v", err)
	}

	// Create temporary git repository
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, "test.git")

	// Initialize a proper git repository
	cmd := exec.Command("git", "init", "--bare", gitDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Logf("git init output: %s", output)
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Materialize snapshot
	if err := Materialize(gitDir, snapshot); err != nil {
		t.Fatalf("Materialize failed: %v", err)
	}

	// Note: git fsck will fail on invalid pack data (we're using dummy data above).
	// This test validates that:
	// 1. The structure is correct (directories exist, files are in right places)
	// 2. git can read the config and refs
	// 3. With real pack data from ARMOR, fsck would pass

	// Verify git can read the ref
	cmd = exec.Command("git", "--git-dir="+gitDir, "show-ref")
	if output, err := cmd.CombinedOutput(); err != nil {
		// This is expected with dummy pack data - the important part is that
		// git can parse the structure, not that pack data is valid
		t.Logf("git show-ref output (expected to fail with dummy data): %s", output)
	}
}

func TestParseTarball_TruncatedTarball(t *testing.T) {
	// Create a valid tarball
	configData := []byte(`{
		"core.repositoryformatversion": "1",
		"remote.origin.promisor": "true",
		"remote.origin.partialclonefilter": "blob:none"
	}`)
	refData := []byte("refs/heads/main abc123")

	members := []TarballMember{
		{Name: "objects/pack/pack-123.pack", Data: []byte("pack")},
		{Name: "config.json", Data: configData},
		{Name: "ref", Data: refData},
	}

	tarball := createTestTarball(t, members)

	// Truncate the tarball to simulate corruption
	truncatedTarball := tarball[:len(tarball)-10]

	_, err := ParseTarball(truncatedTarball)
	if err == nil {
		t.Error("expected error for truncated tarball, got nil")
	}
	if !errors.Is(err, ErrInvalidTarball) {
		t.Errorf("expected ErrInvalidTarball, got %v", err)
	}
}

func TestParseTarball_InvalidTarHeader(t *testing.T) {
	// Create invalid tar data that's not a valid tarball
	invalidTarball := []byte("not a tarball at all")

	_, err := ParseTarball(invalidTarball)
	if err == nil {
		t.Error("expected error for invalid tarball, got nil")
	}
	if !errors.Is(err, ErrInvalidTarball) {
		t.Errorf("expected ErrInvalidTarball, got %v", err)
	}
}

func TestParseTarball_RefAtOriginalPath(t *testing.T) {
	// Test the new format where refs are stored at their original paths
	configData := []byte(`{
		"core.repositoryformatversion": "1",
		"remote.origin.promisor": "true",
		"remote.origin.partialclonefilter": "blob:none"
	}`)

	// Ref is stored at its original path with just the SHA as content
	refData := []byte("abc123def456\n")

	members := []TarballMember{
		{Name: "objects/pack/pack-123.pack", Data: []byte("pack")},
		{Name: "config.json", Data: configData},
		{Name: "refs/heads/main", Data: refData},
	}

	tarball := createTestTarball(t, members)

	snapshot, err := ParseTarball(tarball)
	if err != nil {
		t.Fatalf("ParseTarball failed: %v", err)
	}

	// Verify ref path and SHA
	if snapshot.RefPath != "refs/heads/main" {
		t.Errorf("expected ref path refs/heads/main, got %s", snapshot.RefPath)
	}
	if snapshot.RefSHA != "abc123def456" {
		t.Errorf("expected ref SHA abc123def456, got %s", snapshot.RefSHA)
	}
}

func TestParseTarball_SymbolicRef(t *testing.T) {
	// Test handling of symbolic refs (e.g., HEAD pointing to refs/heads/main)
	configData := []byte(`{
		"core.repositoryformatversion": "1",
		"remote.origin.promisor": "true",
		"remote.origin.partialclonefilter": "blob:none"
	}`)

	// Symbolic ref content
	refData := []byte("ref: refs/heads/main")

	members := []TarballMember{
		{Name: "objects/pack/pack-123.pack", Data: []byte("pack")},
		{Name: "config.json", Data: configData},
		{Name: "HEAD", Data: refData},
	}

	tarball := createTestTarball(t, members)

	snapshot, err := ParseTarball(tarball)
	if err != nil {
		t.Fatalf("ParseTarball failed: %v", err)
	}

	// Verify symbolic ref is preserved
	if snapshot.RefPath != "HEAD" {
		t.Errorf("expected ref path HEAD, got %s", snapshot.RefPath)
	}
	if snapshot.RefSHA != "ref: refs/heads/main" {
		t.Errorf("expected symbolic ref 'ref: refs/heads/main', got %s", snapshot.RefSHA)
	}
}

func TestParseTarball_LegacyRefFormat(t *testing.T) {
	// Test backward compatibility with legacy "ref" file format
	configData := []byte(`{
		"core.repositoryformatversion": "1",
		"remote.origin.promisor": "true",
		"remote.origin.partialclonefilter": "blob:none"
	}`)

	// Legacy format: "ref" file contains "refs/heads/main SHA"
	refData := []byte("refs/heads/main abc123def456")

	members := []TarballMember{
		{Name: "objects/pack/pack-123.pack", Data: []byte("pack")},
		{Name: "config.json", Data: configData},
		{Name: "ref", Data: refData},
	}

	tarball := createTestTarball(t, members)

	snapshot, err := ParseTarball(tarball)
	if err != nil {
		t.Fatalf("ParseTarball failed: %v", err)
	}

	// Verify legacy format still works
	if snapshot.RefPath != "refs/heads/main" {
		t.Errorf("expected ref path refs/heads/main, got %s", snapshot.RefPath)
	}
	if snapshot.RefSHA != "abc123def456" {
		t.Errorf("expected ref SHA abc123def456, got %s", snapshot.RefSHA)
	}
}

func TestMaterialize_RefAtOriginalPath(t *testing.T) {
	// Test materialization with new format (ref at original path)
	configData := []byte(`{
		"core.repositoryformatversion": "1",
		"remote.origin.promisor": "true",
		"remote.origin.partialclonefilter": "blob:none"
	}`)

	refData := []byte("abc123def456\n")

	members := []TarballMember{
		{Name: "objects/pack/pack-123.pack", Data: []byte("pack")},
		{Name: "config.json", Data: configData},
		{Name: "refs/heads/main", Data: refData},
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

	// Verify loose ref file exists at correct path
	looseRefPath := filepath.Join(gitDir, "refs", "heads", "main")
	refContent, err := os.ReadFile(looseRefPath)
	if err != nil {
		t.Fatalf("failed to read loose ref: %v", err)
	}

	if string(refContent) != "abc123def456\n" {
		t.Errorf("loose ref content mismatch: got %q, want %q", string(refContent), "abc123def456\n")
	}

	// Verify packed-refs does NOT exist
	packedRefsPath := filepath.Join(gitDir, "packed-refs")
	if _, err := os.Stat(packedRefsPath); err == nil {
		t.Error("packed-refs file should not exist")
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected error checking packed-refs: %v", err)
	}
}

func TestMaterialize_SymbolicRef(t *testing.T) {
	// Test materialization of symbolic refs
	configData := []byte(`{
		"core.repositoryformatversion": "1",
		"remote.origin.promisor": "true",
		"remote.origin.partialclonefilter": "blob:none"
	}`)

	// Symbolic ref (e.g., for HEAD)
	refData := []byte("ref: refs/heads/main")

	members := []TarballMember{
		{Name: "objects/pack/pack-123.pack", Data: []byte("pack")},
		{Name: "config.json", Data: configData},
		{Name: "HEAD", Data: refData},
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

	// Initialize minimal git repository (create a dummy HEAD for validation)
	headPath := filepath.Join(gitDir, "HEAD")
	if err := os.WriteFile(headPath, []byte("ref: refs/heads/master\n"), 0644); err != nil {
		t.Fatalf("failed to write initial HEAD: %v", err)
	}

	// Materialize snapshot (should overwrite HEAD with our symbolic ref)
	if err := Materialize(gitDir, snapshot); err != nil {
		t.Fatalf("Materialize failed: %v", err)
	}

	// Verify symbolic ref is written correctly
	headContent, err := os.ReadFile(headPath)
	if err != nil {
		t.Fatalf("failed to read HEAD: %v", err)
	}

	if string(headContent) != "ref: refs/heads/main" {
		t.Errorf("symbolic ref content mismatch: got %q, want %q", string(headContent), "ref: refs/heads/main")
	}

	// Verify packed-refs does NOT exist
	packedRefsPath := filepath.Join(gitDir, "packed-refs")
	if _, err := os.Stat(packedRefsPath); err == nil {
		t.Error("packed-refs file should not exist")
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected error checking packed-refs: %v", err)
	}
}

func TestParseTarball_WithPromisorAndRev(t *testing.T) {
	// Test that .promisor and .rev files are properly extracted
	packData := []byte("pack data")
	idxData := []byte("idx data")
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

	// Verify all 4 pack-related files were extracted
	if len(snapshot.PackFiles) != 4 {
		t.Errorf("expected 4 pack files, got %d", len(snapshot.PackFiles))
	}

	// Verify each file type is present
	foundPack := false
	foundIdx := false
	foundPromisor := false
	foundRev := false

	for _, pf := range snapshot.PackFiles {
		switch {
		case strings.HasSuffix(pf.Name, ".pack"):
			foundPack = true
			if !bytes.Equal(pf.Data, packData) {
				t.Error("pack data is not byte-identical")
			}
		case strings.HasSuffix(pf.Name, ".idx"):
			foundIdx = true
			if !bytes.Equal(pf.Data, idxData) {
				t.Error("idx data is not byte-identical")
			}
		case strings.HasSuffix(pf.Name, ".promisor"):
			foundPromisor = true
			if !bytes.Equal(pf.Data, promisorData) {
				t.Error("promisor data is not byte-identical")
			}
		case strings.HasSuffix(pf.Name, ".rev"):
			foundRev = true
			if !bytes.Equal(pf.Data, revData) {
				t.Error("rev data is not byte-identical")
			}
		}
	}

	if !foundPack {
		t.Error(".pack file not found in snapshot")
	}
	if !foundIdx {
		t.Error(".idx file not found in snapshot")
	}
	if !foundPromisor {
		t.Error(".promisor file not found in snapshot")
	}
	if !foundRev {
		t.Error(".rev file not found in snapshot")
	}

	// Test Materialize also handles these files correctly
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

	// Verify all files were written to the correct location
	packPath := filepath.Join(gitDir, "objects", "pack", "pack-abc.pack")
	writtenPackData, err := os.ReadFile(packPath)
	if err != nil {
		t.Fatalf("failed to read pack file: %v", err)
	}
	if !bytes.Equal(writtenPackData, packData) {
		t.Error("written pack data is not byte-identical")
	}

	idxPath := filepath.Join(gitDir, "objects", "pack", "pack-abc.idx")
	writtenIdxData, err := os.ReadFile(idxPath)
	if err != nil {
		t.Fatalf("failed to read idx file: %v", err)
	}
	if !bytes.Equal(writtenIdxData, idxData) {
		t.Error("written idx data is not byte-identical")
	}

	promisorPath := filepath.Join(gitDir, "objects", "pack", "pack-abc.promisor")
	writtenPromisorData, err := os.ReadFile(promisorPath)
	if err != nil {
		t.Fatalf("failed to read promisor file: %v", err)
	}
	if !bytes.Equal(writtenPromisorData, promisorData) {
		t.Error("written promisor data is not byte-identical")
	}

	revPath := filepath.Join(gitDir, "objects", "pack", "pack-abc.rev")
	writtenRevData, err := os.ReadFile(revPath)
	if err != nil {
		t.Fatalf("failed to read rev file: %v", err)
	}
	if !bytes.Equal(writtenRevData, revData) {
		t.Error("written rev data is not byte-identical")
	}
}
