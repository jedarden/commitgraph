package warmstart

import (
	"errors"
	"testing"
)

// TestIdxFilenameFromPackFilename mirrors TestRefFilenameFromPackFilename for the
// .idx companion file naming convention.
func TestIdxFilenameFromPackFilename(t *testing.T) {
	tests := []struct {
		name         string
		packFilename string
		expected     string
	}{
		{
			name:         "basic pack filename",
			packFilename: "pack-abc123.pack",
			expected:     "pack-abc123.idx",
		},
		{
			name:         "pack filename with multiple dots",
			packFilename: "pack-test.123.pack",
			expected:     "pack-test.123.idx",
		},
		{
			name:         "pack filename with full path",
			packFilename: "objects/pack/pack-xyz.pack",
			expected:     "objects/pack/pack-xyz.idx",
		},
		{
			name:         "no pack extension",
			packFilename: "somefile",
			expected:     "somefile.idx",
		},
		{
			name:         "pack with double extension",
			packFilename: "pack-abc123.pack.promisor",
			expected:     "pack-abc123.pack.promisor.idx",
		},
		{
			name:         "empty string",
			packFilename: "",
			expected:     ".idx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IdxFilenameFromPackFilename(tt.packFilename)
			if result != tt.expected {
				t.Errorf("IdxFilenameFromPackFilename(%q) = %q, want %q", tt.packFilename, result, tt.expected)
			}
		})
	}
}

// TestIdxFileExistsInTarball mirrors TestRefFileExistsInTarball for .idx lookups.
func TestIdxFileExistsInTarball(t *testing.T) {
	tests := []struct {
		name         string
		packFilename string
		members      []TarballMember
		expected     bool
	}{
		{
			name:         "idx file exists",
			packFilename: "objects/pack/pack-abc123.pack",
			members: []TarballMember{
				{Name: "objects/pack/pack-abc123.pack", Data: []byte("pack data")},
				{Name: "objects/pack/pack-abc123.idx", Data: []byte("idx data")},
			},
			expected: true,
		},
		{
			name:         "idx file does not exist",
			packFilename: "objects/pack/pack-abc123.pack",
			members: []TarballMember{
				{Name: "objects/pack/pack-abc123.pack", Data: []byte("pack data")},
				{Name: "objects/pack/pack-abc123.ref", Data: []byte("ref data")},
			},
			expected: false,
		},
		{
			name:         "empty member list",
			packFilename: "objects/pack/pack-abc123.pack",
			members:      []TarballMember{},
			expected:     false,
		},
		{
			name:         "idx file exists with different pack",
			packFilename: "objects/pack/pack-abc123.pack",
			members: []TarballMember{
				{Name: "objects/pack/pack-def456.pack", Data: []byte("pack data")},
				{Name: "objects/pack/pack-def456.idx", Data: []byte("idx data")},
			},
			expected: false,
		},
		{
			name:         "pack file without objects/pack prefix",
			packFilename: "pack-abc123.pack",
			members: []TarballMember{
				{Name: "pack-abc123.pack", Data: []byte("pack data")},
				{Name: "pack-abc123.idx", Data: []byte("idx data")},
			},
			expected: true,
		},
		{
			name:         "idx file with similar but different name is not found",
			packFilename: "objects/pack/pack-abc123.pack",
			members: []TarballMember{
				{Name: "objects/pack/pack-abc123.pack", Data: []byte("pack data")},
				{Name: "objects/pack/pack-abc1234.idx", Data: []byte("idx data")},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IdxFileExistsInTarball(tt.packFilename, tt.members)
			if result != tt.expected {
				t.Errorf("IdxFileExistsInTarball(%q, members) = %v, want %v", tt.packFilename, result, tt.expected)
			}
		})
	}
}

// TestCollectMissingIdxFiles mirrors TestCollectMissingRefFiles: it verifies that
// CollectMissingIdxFiles reports the full list of .pack files lacking a companion
// .idx file, rather than failing fast on the first one found.
func TestCollectMissingIdxFiles(t *testing.T) {
	tests := []struct {
		name        string
		members     []TarballMember
		expected    []string
		description string
	}{
		{
			name: "all idx files present",
			members: []TarballMember{
				{Name: "objects/pack/pack-abc.pack", Data: []byte("pack data")},
				{Name: "objects/pack/pack-abc.idx", Data: []byte("idx data")},
				{Name: "objects/pack/pack-def.pack", Data: []byte("pack data")},
				{Name: "objects/pack/pack-def.idx", Data: []byte("idx data")},
			},
			expected:    []string{},
			description: "All .pack files have corresponding .idx files",
		},
		{
			name: "one idx file missing",
			members: []TarballMember{
				{Name: "objects/pack/pack-abc.pack", Data: []byte("pack data")},
				{Name: "objects/pack/pack-abc.idx", Data: []byte("idx data")},
				{Name: "objects/pack/pack-def.pack", Data: []byte("pack data")},
			},
			expected:    []string{"objects/pack/pack-def.idx"},
			description: "One .pack file missing its .idx file",
		},
		{
			name: "multiple idx files missing",
			members: []TarballMember{
				{Name: "objects/pack/pack-abc.pack", Data: []byte("pack data")},
				{Name: "objects/pack/pack-def.pack", Data: []byte("pack data")},
				{Name: "objects/pack/pack-ghi.pack", Data: []byte("pack data")},
				{Name: "objects/pack/pack-ghi.idx", Data: []byte("idx data")},
			},
			expected:    []string{"objects/pack/pack-abc.idx", "objects/pack/pack-def.idx"},
			description: "Multiple .pack files missing their .idx files",
		},
		{
			name: "no pack files",
			members: []TarballMember{
				{Name: "config.json", Data: []byte("{}")},
				{Name: "ref", Data: []byte("refs/heads/main abc123")},
			},
			expected:    []string{},
			description: "No .pack files present, should return empty list",
		},
		{
			name: "empty member list",
			members:     []TarballMember{},
			expected:    []string{},
			description: "Empty member list returns empty missing list",
		},
		{
			name: "preserves order of missing files",
			members: []TarballMember{
				{Name: "objects/pack/pack-aaa.pack", Data: []byte("pack data")},
				{Name: "objects/pack/pack-bbb.pack", Data: []byte("pack data")},
				{Name: "objects/pack/pack-ccc.pack", Data: []byte("pack data")},
				{Name: "objects/pack/pack-bbb.idx", Data: []byte("idx data")},
			},
			expected:    []string{"objects/pack/pack-aaa.idx", "objects/pack/pack-ccc.idx"},
			description: "Missing files are reported in pack file order",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CollectMissingIdxFiles(tt.members)

			if len(result) != len(tt.expected) {
				t.Errorf("CollectMissingIdxFiles() returned %d items, want %d", len(result), len(tt.expected))
				t.Logf("Got:      %v", result)
				t.Logf("Expected: %v", tt.expected)
				return
			}

			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("CollectMissingIdxFiles()[%d] = %q, want %q", i, result[i], tt.expected[i])
					t.Logf("Got:      %v", result)
					t.Logf("Expected: %v", tt.expected)
					return
				}
			}

			t.Logf("PASS: %s", tt.description)
		})
	}
}

// TestParseTarball_MissingIdxFile mirrors TestParseTarball_MissingRefFile: it verifies
// that ParseTarball reports a MissingMember(".idx") error, with the full list of
// missing .idx files in the error Context, when one or more .pack files lack a
// companion .idx file.
func TestParseTarball_MissingIdxFile(t *testing.T) {
	configData := []byte(`{
		"core.repositoryformatversion": "1",
		"remote.origin.promisor": "true",
		"remote.origin.partialclonefilter": "blob:none"
	}`)
	refData := []byte("refs/heads/main abc123")

	tests := []struct {
		name        string
		members     []TarballMember
		errorKind   ErrorKind
		memberName  string
		description string
	}{
		{
			name: "single-pack-missing-idx",
			members: []TarballMember{
				{Name: "objects/pack/pack-123.pack", Data: []byte("PACK123456789")},
				// Missing pack-123.idx
				{Name: "objects/pack/pack-123.ref", Data: []byte("ref data")},
				{Name: "config.json", Data: configData},
				{Name: "ref", Data: refData},
			},
			errorKind:   MissingMember,
			memberName:  ".idx",
			description: "Single pack file missing its .idx file",
		},
		{
			name: "multiple-packs-multiple-idxs-missing",
			members: []TarballMember{
				{Name: "objects/pack/pack-abc.pack", Data: []byte("PACK123456789")},
				// Missing pack-abc.idx
				{Name: "objects/pack/pack-abc.ref", Data: []byte("ref abc")},
				{Name: "objects/pack/pack-def.pack", Data: []byte("PACK987654321")},
				// Missing pack-def.idx
				{Name: "objects/pack/pack-def.ref", Data: []byte("ref def")},
				{Name: "config.json", Data: configData},
				{Name: "ref", Data: refData},
			},
			errorKind:   MissingMember,
			memberName:  ".idx",
			description: "Multiple pack files missing their .idx files",
		},
		{
			name: "mixed-scenario-some-idxs-missing",
			members: []TarballMember{
				{Name: "objects/pack/pack-abc.pack", Data: []byte("PACK123456789")},
				{Name: "objects/pack/pack-abc.idx", Data: []byte("idx abc")},
				{Name: "objects/pack/pack-abc.ref", Data: []byte("ref abc")},
				{Name: "objects/pack/pack-def.pack", Data: []byte("PACK987654321")},
				// Missing pack-def.idx
				{Name: "objects/pack/pack-def.ref", Data: []byte("ref def")},
				{Name: "objects/pack/pack-ghi.pack", Data: []byte("PACK1111122222")},
				// Missing pack-ghi.idx
				{Name: "objects/pack/pack-ghi.ref", Data: []byte("ref ghi")},
				{Name: "config.json", Data: configData},
				{Name: "ref", Data: refData},
			},
			errorKind:   MissingMember,
			memberName:  ".idx",
			description: "Mixed scenario: some pack files have .idx, some missing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tarball := createTestTarball(t, tt.members)

			_, err := ParseTarball(tarball)
			if err == nil {
				t.Fatalf("%s: expected error for missing .idx file, got nil", tt.description)
			}

			var missingErr *Error
			if !errors.As(err, &missingErr) {
				t.Fatalf("%s: expected *Error type, got %T: %v", tt.description, err, err)
			}

			if missingErr.Kind != tt.errorKind {
				t.Errorf("%s: expected error kind %v, got %v", tt.description, tt.errorKind, missingErr.Kind)
			}

			if missingErr.MemberName != tt.memberName {
				t.Errorf("%s: expected member name %q, got %q", tt.description, tt.memberName, missingErr.MemberName)
			}

			if len(missingErr.Context) == 0 {
				t.Errorf("%s: expected error Context to list missing .idx files, got empty Context", tt.description)
			}

			t.Logf("%s: correctly detected missing .idx files: %v", tt.description, missingErr)
		})
	}
}
