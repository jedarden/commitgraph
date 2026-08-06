// Package warmstart implements materialization of git repository warm-start snapshots.
//
// A warm-start snapshot enables incremental fetch by restoring a repository's
// state from a previous scan, avoiding full re-clone. The artifact contains:
//   - Raw pack files (*.pack, *.idx, *.promisor, *.rev) from objects/pack/
//   - A single loose ref file (e.g., refs/heads/main)
//   - Three required git config values for partial clone support
//
// See docs/research/incremental-fetch-warm-start.md for empirical validation.
package warmstart

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var (
	// ErrInvalidTarball indicates the tarball is corrupted or truncated.
	ErrInvalidTarball = errors.New("warmstart: invalid tarball")

	// ErrMissingPackFiles indicates required pack files are missing.
	ErrMissingPackFiles = errors.New("warmstart: missing required pack files")

	// ErrMissingRef indicates the ref file is missing.
	ErrMissingRef = errors.New("warmstart: missing ref file")

	// ErrMissingConfig indicates the git config is missing.
	ErrMissingConfig = errors.New("warmstart: missing git config")

	// ErrInvalidConfig indicates the git config is malformed.
	ErrInvalidConfig = errors.New("warmstart: invalid git config")

	// ErrNotAGitRepo indicates the target directory is not a git repository.
	ErrNotAGitRepo = errors.New("warmstart: not a git repository")
)

// Config represents the git configuration values required for warm-start.
type Config struct {
	// CoreRepositoryFormatVersion must be "1" for promisor packs.
	CoreRepositoryFormatVersion string `json:"core.repositoryformatversion"`

	// RemoteOriginPromisor enables promisor pack functionality.
	RemoteOriginPromisor string `json:"remote.origin.promisor"`

	// RemoteOriginPartialCloneFilter specifies the filter (e.g., "blob:none").
	RemoteOriginPartialCloneFilter string `json:"remote.origin.partialclonefilter"`
}

// Validate checks that all required config values are present and valid.
func (c *Config) Validate() error {
	if c.CoreRepositoryFormatVersion == "" {
		return fmt.Errorf("%w: missing core.repositoryformatversion", ErrInvalidConfig)
	}
	if c.RemoteOriginPromisor == "" {
		return fmt.Errorf("%w: missing remote.origin.promisor", ErrInvalidConfig)
	}
	if c.RemoteOriginPartialCloneFilter == "" {
		return fmt.Errorf("%w: missing remote.origin.partialclonefilter", ErrInvalidConfig)
	}
	return nil
}

// TarballMember represents a file in the warm-start tarball.
type TarballMember struct {
	Name string
	Data []byte
}

// WarmStartSnapshot represents the extracted warm-start data.
type WarmStartSnapshot struct {
	// PackFiles contains all pack-related files (.pack, .idx, .promisor, .rev).
	PackFiles []TarballMember

	// RefPath is the ref path (e.g., "refs/heads/main").
	RefPath string

	// RefSHA is the SHA the ref points to.
	RefSHA string

	// Config holds the git configuration.
	Config Config
}

// ParseTarball extracts and validates a warm-start tarball.
func ParseTarball(data []byte) (*WarmStartSnapshot, error) {
	tr := tar.NewReader(bytes.NewReader(data))

	snapshot := &WarmStartSnapshot{}
	var configData []byte
	var foundConfig, foundRef bool
	packExtensions := map[string]bool{
		".pack":     true,
		".idx":      true,
		".ref":      true,
		".promisor": true,
		".rev":      true,
	}

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidTarball, err)
		}

		// Read file content
		var buf bytes.Buffer
		written, err := io.Copy(&buf, tr)
		if err != nil {
			// Check if this is an unexpected EOF (truncated file)
			if err == io.ErrUnexpectedEOF || errors.Is(err, io.ErrUnexpectedEOF) {
				return nil, NewTruncatedMemberError(hdr.Name, "ended prematurely", 0)
			}
			return nil, fmt.Errorf("%w: failed to read %s: %v", ErrInvalidTarball, hdr.Name, err)
		}

		// Verify we read the expected number of bytes
		if written != hdr.Size {
			return nil, NewTruncatedMemberError(hdr.Name, fmt.Sprintf("expected %d bytes, got %d", hdr.Size, written), 0)
		}
		data := buf.Bytes()

		switch hdr.Name {
		case "config.json":
			foundConfig = true
			configData = data
		case "ref":
			// Legacy format: ref file contains "refs/heads/main SHA"
			foundRef = true
			refParts := strings.TrimSpace(string(data))
			if refParts == "" {
				return nil, &CorruptionError {
					Context: "empty ref data in ref file",
				}
			}
			parts := strings.Fields(refParts)
			if len(parts) != 2 {
				return nil, &CorruptionError {
					Context: fmt.Sprintf("invalid ref format in ref file: expected 'refpath SHA', got '%s'", refParts),
				}
			}
			snapshot.RefPath = parts[0]
			snapshot.RefSHA = parts[1]
		default:
			// Check if it's a pack file (objects/pack/*.pack, etc.)
			if strings.HasPrefix(hdr.Name, "objects/pack/") {
				ext := filepath.Ext(hdr.Name)
				// Handle double extensions like .pack.promisor
				base := filepath.Base(hdr.Name)
				if packExtensions[ext] || strings.HasSuffix(base, ".promisor") || strings.HasSuffix(base, ".rev") {
			// Validate pack file header size (minimum 12 bytes: "PACK" + version + object count)
			if ext == ".pack" && len(data) < 12 {
				return nil, NewTruncatedMemberError(hdr.Name, fmt.Sprintf("pack file too small: %d bytes (minimum 12 bytes for header)", len(data)), 0)
			}
					snapshot.PackFiles = append(snapshot.PackFiles, TarballMember{
						Name: hdr.Name,
						Data: data,
					})
				}
			} else if strings.HasPrefix(hdr.Name, "refs/") || hdr.Name == "HEAD" || strings.HasPrefix(hdr.Name, "refs/tags/") {
				// New format: ref at its original path (e.g., "refs/heads/main")
				// Also handles top-level refs like "HEAD" and tags
				// The file content is just the SHA or symbolic ref target
				foundRef = true
				snapshot.RefPath = hdr.Name
				snapshot.RefSHA = strings.TrimSpace(string(data))
				// Check if it's a symbolic ref (starts with "ref:")
				if strings.HasPrefix(snapshot.RefSHA, "ref:") {
					// Symbolic ref - store as-is without newline
					snapshot.RefSHA = strings.TrimSpace(snapshot.RefSHA)
				}
			}
		}
	}

	// Validate required components
	if !foundConfig {
		return nil, ErrMissingConfig
	}
	if !foundRef {
		return nil, ErrMissingRef
	}
	if len(snapshot.PackFiles) == 0 {
		return nil, ErrMissingPackFiles
	}

	// Validate that at least one .pack file is present
	foundPack := false
	for _, pf := range snapshot.PackFiles {
		if strings.HasSuffix(pf.Name, ".pack") {
			foundPack = true
			break
		}
	}
	if !foundPack {
		return nil, NewMissingMemberError(".pack")
	}

	// Collect base names of all .pack files for corresponding file validation
	var packBaseNames []string
	for _, pf := range snapshot.PackFiles {
		if strings.HasSuffix(pf.Name, ".pack") {
			// Extract base name without extension for corresponding file checks
			baseName := strings.TrimSuffix(pf.Name, ".pack")
			packBaseNames = append(packBaseNames, baseName)
		}
	}

	// Validate that corresponding .idx files exist for each .pack file
	for _, baseName := range packBaseNames {
		idxName := baseName + ".idx"
		foundIdx := false
		for _, pf := range snapshot.PackFiles {
			if pf.Name == idxName {
				foundIdx = true
				break
			}
		}
		if !foundIdx {
			return nil, NewMissingMemberError(".idx")
		}
	}

	// Validate that corresponding .ref files exist for each .pack file
	var missingRefFiles []string
	for _, baseName := range packBaseNames {
		refName := baseName + ".ref"
		foundRef := false
		for _, pf := range snapshot.PackFiles {
			if pf.Name == refName {
				foundRef = true
				break
			}
		}
		if !foundRef {
			missingRefFiles = append(missingRefFiles, refName)
		}
	}
	if len(missingRefFiles) > 0 {
		return nil, &Error{
			Kind:       MissingMember,
			MemberName: ".ref",
			Context:    fmt.Sprintf("missing .ref files: %s", strings.Join(missingRefFiles, ", ")),
		}
	}

	// Parse config
	if err := json.Unmarshal(configData, &snapshot.Config); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}
	if err := snapshot.Config.Validate(); err != nil {
		return nil, err
	}

	return snapshot, nil
}

// Materialize writes the warm-start snapshot to a git directory.
//
// The target directory must be an empty git repository (initialized with
// `git init --bare` or `git init`). After materialization, the repository
// will be ready for incremental fetch via `git fetch origin`.
func Materialize(gitDir string, snapshot *WarmStartSnapshot) error {
	// Verify target is a git directory
	headPath := filepath.Join(gitDir, "HEAD")
	if _, err := os.Stat(headPath); err != nil {
		return &NotAGitRepoError{
			Path:   gitDir,
			Reason: fmt.Sprintf("HEAD not found at %s", headPath),
		}
	}

	// Create objects/pack directory
	packDir := filepath.Join(gitDir, "objects", "pack")
	if err := os.MkdirAll(packDir, 0755); err != nil {
		return fmt.Errorf("failed to create pack directory: %w", err)
	}

	// Write pack files byte-identical
	for _, pf := range snapshot.PackFiles {
		// Strip "objects/pack/" prefix if present
		targetName := strings.TrimPrefix(pf.Name, "objects/pack/")
		targetPath := filepath.Join(packDir, targetName)

		if err := os.WriteFile(targetPath, pf.Data, 0444); err != nil {
			return fmt.Errorf("failed to write pack file %s: %w", targetName, err)
		}
	}

	// Write loose ref file (NOT packed-refs)
	refDir := filepath.Dir(snapshot.RefPath)
	if refDir != "." {
		fullRefDir := filepath.Join(gitDir, refDir)
		if err := os.MkdirAll(fullRefDir, 0755); err != nil {
			return fmt.Errorf("failed to create ref directory: %w", err)
		}
	}
	refPath := filepath.Join(gitDir, snapshot.RefPath)

	// Determine the ref content based on whether it's a symbolic ref
	var refContent []byte
	if strings.HasPrefix(snapshot.RefSHA, "ref:") {
		// Symbolic ref - store as-is without newline
		refContent = []byte(snapshot.RefSHA)
	} else {
		// Direct ref - store SHA with newline
		refContent = []byte(snapshot.RefSHA + "\n")
	}

	if err := os.WriteFile(refPath, refContent, 0444); err != nil {
		return fmt.Errorf("failed to write ref file: %w", err)
	}

	// Write git config values
	if err := writeGitConfig(gitDir, &snapshot.Config); err != nil {
		return fmt.Errorf("failed to write git config: %w", err)
	}

	return nil
}

// writeGitConfig sets the three required promisor config values in git config.
// It uses git config commands to ensure the values are set correctly.
func writeGitConfig(gitDir string, config *Config) error {
	// Set core.repositoryformatversion
	if err := runGitConfig(gitDir, "core.repositoryformatversion", config.CoreRepositoryFormatVersion); err != nil {
		return fmt.Errorf("failed to set core.repositoryformatversion: %w", err)
	}

	// Set remote.origin.promisor
	if err := runGitConfig(gitDir, "remote.origin.promisor", config.RemoteOriginPromisor); err != nil {
		return fmt.Errorf("failed to set remote.origin.promisor: %w", err)
	}

	// Set remote.origin.partialclonefilter
	if err := runGitConfig(gitDir, "remote.origin.partialclonefilter", config.RemoteOriginPartialCloneFilter); err != nil {
		return fmt.Errorf("failed to set remote.origin.partialclonefilter: %w", err)
	}

	return nil
}

// runGitConfig runs git config command to set a value
func runGitConfig(gitDir, key, value string) error {
	// We'll directly manipulate the config file since we can't rely on git being available
	// and we need this to work even in environments without git
	return setGitConfigValue(filepath.Join(gitDir, "config"), key, value)
}

// setGitConfigValue sets a git config value by parsing and rewriting the config file
func setGitConfigValue(configPath, key, value string) error {
	// Read existing config
	existingConfig, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		// Config file doesn't exist - create minimal default config
		existingConfig = []byte("[core]\nrepositoryformatversion = 0\n")
	}

	// Parse the config file
	lines := strings.Split(string(existingConfig), "\n")
	var outputLines []string
	var inCurrentSection bool
	valueSet := false

	// Parse the key to get section and variable
	// e.g., "core.repositoryformatversion" -> section="[core]", variable="repositoryformatversion"
	// e.g., "remote.origin.promisor" -> section='[remote "origin"]', variable="promisor"
	section, variable := parseConfigKey(key)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check if this is a section header
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			inCurrentSection = (trimmed == section)
			outputLines = append(outputLines, line)
			continue
		}

		// Check if this is the variable we want to set
		if inCurrentSection && strings.HasPrefix(trimmed, variable+" ") {
			// Replace this line with the new value
			outputLines = append(outputLines, "\t"+variable+" = "+value)
			valueSet = true
			continue
		}

		outputLines = append(outputLines, line)
	}

	// If the value wasn't set, add it
	if !valueSet {
		// Check if the section exists
		sectionExists := false
		for _, line := range outputLines {
			if strings.TrimSpace(line) == section {
				sectionExists = true
				break
			}
		}

		if !sectionExists {
			// Add the section and the value
			outputLines = append(outputLines, section)
		}

		// Find the section and add the value after it
		for i, line := range outputLines {
			if strings.TrimSpace(line) == section {
				// Insert the value after this line
				outputLines = append(outputLines[:i+1], append([]string{"\t" + variable + " = " + value}, outputLines[i+1:]...)...)
				break
			}
		}
	}

	return os.WriteFile(configPath, []byte(strings.Join(outputLines, "\n")), 0644)
}

// parseConfigKey parses a git config key into section and variable
// e.g., "core.repositoryformatversion" -> "[core]", "repositoryformatversion"
// e.g., "remote.origin.promisor" -> `[remote "origin"]`, "promisor"
func parseConfigKey(key string) (string, string) {
	parts := strings.Split(key, ".")
	if len(parts) == 2 {
		// Simple case: "core.repositoryformatversion"
		return "[" + parts[0] + "]", parts[1]
	}
	if len(parts) == 3 {
		// Remote case: "remote.origin.promisor"
		return `[remote "` + parts[1] + `"]`, parts[2]
	}
	return "", ""
}

// RefFilenameFromPackFilename constructs the expected .ref filename from a .pack filename.
// It strips the .pack extension and appends .ref.
// For example: "pack-abc123.pack" becomes "pack-abc123.ref"
// Edge cases handled:
//   - No .pack extension: appends .ref to the input as-is
//   - Multiple dots: only the final .pack extension is stripped
//   - Double extensions: "pack-abc123.pack.promisor" would become "pack-abc123.pack.promisor.ref"
func RefFilenameFromPackFilename(packFilename string) string {
	return strings.TrimSuffix(packFilename, ".pack") + ".ref"
}

// RefFileExistsInTarball checks if a .ref file exists in the tarball for a given .pack file.
// It uses RefFilenameFromPackFilename to construct the expected .ref filename and searches
// the provided member list for a matching file.
//
// Parameters:
//   - packFilename: The .pack file name (e.g., "objects/pack/pack-abc123.pack")
//   - members: Slice of TarballMember representing files in the tarball
//
// Returns:
//   - true if the corresponding .ref file is found in the member list, false otherwise
//
// Example:
//   packFile := "objects/pack/pack-abc123.pack"
//   members := []TarballMember{{Name: "objects/pack/pack-abc123.ref", Data: ...}}
//   found := RefFileExistsInTarball(packFile, members) // returns true
func RefFileExistsInTarball(packFilename string, members []TarballMember) bool {
	expectedRefName := RefFilenameFromPackFilename(packFilename)
	for _, member := range members {
		if member.Name == expectedRefName {
			return true
		}
	}
	return false
}

// CollectMissingRefFiles collects all missing .ref files across all pack files.
// It iterates over each pack file in the members list, checks if the corresponding
// .ref file exists, and collects the names of missing .ref files.
//
// Parameters:
//   - members: Slice of TarballMember representing files in the tarball
//
// Returns:
//   - []string: List of missing .ref file names (empty if all present)
//
// Example:
//   members := []TarballMember{
//       {Name: "objects/pack/pack-abc.pack", Data: ...},
//       {Name: "objects/pack/pack-def.pack", Data: ...},
//       {Name: "objects/pack/pack-abc.ref", Data: ...},
//   }
//   missing := CollectMissingRefFiles(members) // returns ["objects/pack/pack-def.ref"]
func CollectMissingRefFiles(members []TarballMember) []string {
	var missingRefFiles []string

	for _, member := range members {
		// Only check .pack files
		if !strings.HasSuffix(member.Name, ".pack") {
			continue
		}

		// Check if the corresponding .ref file exists
		if !RefFileExistsInTarball(member.Name, members) {
			expectedRefName := RefFilenameFromPackFilename(member.Name)
			missingRefFiles = append(missingRefFiles, expectedRefName)
		}
	}

	return missingRefFiles
}

// ValidateRefFiles validates .ref file existence for a given list of .pack files.
// For each .pack file, it constructs the expected .ref filename and checks if it exists
// on the filesystem using os.Stat.
//
// Parameters:
//   - packFiles: List of .pack file paths (e.g., []string{"objects/pack/pack-abc123.pack"})
//
// Returns:
//   - []string: List of missing .ref file names (empty if all present)
//
// Edge cases handled:
//   - Empty input: returns empty slice
//   - Duplicate pack names: each is checked independently (duplicate .ref entries possible)
//   - Files with non-.pack extensions: still processed (constructs .ref by appending if no .pack suffix)
//   - Filesystem errors: treats non-existence as missing; other errors are ignored
//
// Example:
//   packFiles := []string{"objects/pack/pack-abc.pack", "objects/pack/pack-def.pack"}
//   // If only pack-abc.ref exists on filesystem:
//   missing := ValidateRefFiles(packFiles) // returns ["objects/pack/pack-def.ref"]
func ValidateRefFiles(packFiles []string) []string {
	var missingRefFiles []string

	for _, packFile := range packFiles {
		// Construct expected .ref filename using existing helper
		refFile := RefFilenameFromPackFilename(packFile)

		// Check if .ref file exists on filesystem
		if _, err := os.Stat(refFile); err != nil {
			if os.IsNotExist(err) {
				missingRefFiles = append(missingRefFiles, refFile)
			}
			// Other errors (permission, etc.) are ignored - file not accessible treated as missing
		}
	}

	return missingRefFiles
}
