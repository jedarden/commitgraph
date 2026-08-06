// Package retiredepoch provides test helper functions for accessing retired epoch test fixtures
package retiredepoch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// FixtureDir returns the path to the retired epoch fixtures directory
func FixtureDir() string {
	return "testdata/fixtures/retired-epoch"
}

// FixturePath returns the full path to a fixture file
func FixturePath(filename string) string {
	return filepath.Join(FixtureDir(), filename)
}

// LoadManifest loads a manifest file from the fixtures
func LoadManifest(t *testing.T, filename string) map[string]interface{} {
	t.Helper()
	path := FixturePath(filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read manifest file %s: %v", path, err)
	}

	var manifest map[string]interface{}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("failed to parse manifest file %s: %v", path, err)
	}

	return manifest
}

// LoadFixtureIndex loads the fixture index
func LoadFixtureIndex(t *testing.T) map[string]interface{} {
	t.Helper()
	return LoadManifest(t, "fixture-index.json")
}

// GetPrimaryRetiredEpochKey returns the primary retired epoch key_id for testing
func GetPrimaryRetiredEpochKey(t *testing.T) string {
	t.Helper()
	index := LoadFixtureIndex(t)
	return index["primary_retired_epoch_key"].(string)
}

// RetiredEpochFixture represents a retired epoch test fixture
type RetiredEpochFixture struct {
	KeyID              string `json:"key_id"`
	Epoch              string `json:"epoch"`
	Status             string `json:"status"`
	ManifestFile       string `json:"manifest_file"`
	SampleData         string `json:"sample_data"`
	PrimaryTestFixture bool   `json:"primary_test_fixture"`
}

// GetRetiredEpochFixtures returns all retired epoch fixtures
func GetRetiredEpochFixtures(t *testing.T) []RetiredEpochFixture {
	t.Helper()
	index := LoadFixtureIndex(t)

	var fixtures []RetiredEpochFixture
	retiredEpochs := index["retired_epochs"].([]interface{})

	for _, re := range retiredEpochs {
	 epochMap := re.(map[string]interface{})
	 fixture := RetiredEpochFixture{
		KeyID:              epochMap["key_id"].(string),
		Epoch:              epochMap["epoch"].(string),
		Status:             epochMap["status"].(string),
		ManifestFile:       epochMap["manifest_file"].(string),
		SampleData:         epochMap["sample_data"].(string),
		PrimaryTestFixture: epochMap["primary_test_fixture"].(bool),
	 }
	 fixtures = append(fixtures, fixture)
	}

	return fixtures
}

// GetPrimaryRetiredFixture returns the primary retired epoch fixture
func GetPrimaryRetiredFixture(t *testing.T) RetiredEpochFixture {
	t.Helper()
	fixtures := GetRetiredEpochFixtures(t)

	for _, fixture := range fixtures {
	 if fixture.PrimaryTestFixture {
	 return fixture
	 }
	}

	t.Fatal("no primary retired epoch fixture found")
	return RetiredEpochFixture{}
}

// LoadSampleCommits loads sample commit data for a specific epoch
func LoadSampleCommits(t *testing.T, filename string) []map[string]interface{} {
	t.Helper()
	path := FixturePath(filename)
	data, err := os.ReadFile(path)
	if err != nil {
	 t.Fatalf("failed to read sample commits file %s: %v", path, err)
	}

	var commits []map[string]interface{}
	if err := json.Unmarshal(data, &commits); err != nil {
	 t.Fatalf("failed to parse sample commits file %s: %v", path, err)
	}

	return commits
}

// AssertRetiredEpochInManifest checks if a manifest contains the retired epoch key
func AssertRetiredEpochInManifest(t *testing.T, manifest map[string]interface{}, expectedKeyID string) {
	t.Helper()
	encryptionKeys, ok := manifest["encryption_keys"].([]interface{})
	if !ok {
	 t.Fatal("manifest missing encryption_keys field")
	}

	found := false
	for _, key := range encryptionKeys {
	 keyMap := key.(map[string]interface{})
	 if keyMap["key_id"] == expectedKeyID {
	 found = true
	 if keyMap["status"] != "retired" {
		 t.Errorf("expected status 'retired' for key_id %s, got %v", expectedKeyID, keyMap["status"])
	 }
	 break
	 }
	}

	if !found {
	 t.Errorf("expected to find retired epoch key_id %s in manifest", expectedKeyID)
	}
}

// GetFixturePaths returns all fixture file paths for easy access
func GetFixturePaths(t *testing.T) map[string]string {
	t.Helper()
	index := LoadFixtureIndex(t)
	paths := make(map[string]string)

	// Add individual file paths
	paths["manifest_retired"] = FixturePath("manifest-retired-epoch.json")
	paths["manifest_current"] = FixturePath("manifest-current-epoch.json")
	paths["manifest_multi"] = FixturePath("manifest-multi-epoch.json")
	paths["commits_2023_12"] = FixturePath("sample-commits-2023-12.json")
	paths["commits_2024_08"] = FixturePath("sample-commits-2024-08.json")
	paths["commits_2022_06"] = FixturePath("sample-commits-2022-06.json")
	paths["index"] = FixturePath("fixture-index.json")
	paths["readme"] = FixturePath("README.md")

	return paths
}