package retiredepoch_test

import (
	"testing"

	"commitgraph/testdata/fixtures/retired-epoch"
)

// TestRetiredEpochFixturesExist verifies that all required fixture files are present
func TestRetiredEpochFixturesExist(t *testing.T) {
	paths := retiredepoch.GetFixturePaths(t)

	for name, path := range paths {
	 t.Logf("Fixture path: %s = %s", name, path)
	}
}

// TestPrimaryRetiredEpochKey verifies the primary retired epoch key_id
func TestPrimaryRetiredEpochKey(t *testing.T) {
	keyID := retiredepoch.GetPrimaryRetiredEpochKey(t)
	expectedKeyID := "epoch-2023-12-retired"

	if keyID != expectedKeyID {
	 t.Errorf("expected primary retired epoch key_id %s, got %s", expectedKeyID, keyID)
	}

	t.Logf("✓ Primary retired epoch key_id: %s", keyID)
}

// TestLoadRetiredEpochManifest verifies loading a retired epoch manifest
func TestLoadRetiredEpochManifest(t *testing.T) {
	manifest := retiredepoch.LoadManifest(t, "manifest-retired-epoch.json")

	// Verify basic structure
	if _, ok := manifest["encryption_keys"]; !ok {
	 t.Fatal("manifest missing encryption_keys field")
	}

	// Check for the retired epoch key
	retiredepoch.AssertRetiredEpochInManifest(t, manifest, "epoch-2023-12-retired")

	t.Logf("✓ Successfully loaded retired epoch manifest")
}

// TestMultiEpochManifest verifies the multi-epoch manifest with mixed current/retired keys
func TestMultiEpochManifest(t *testing.T) {
	manifest := retiredepoch.LoadManifest(t, "manifest-multi-epoch.json")

	encryptionKeys := manifest["encryption_keys"].([]interface{})
	if len(encryptionKeys) != 3 {
	 t.Errorf("expected 3 encryption keys, got %d", len(encryptionKeys))
	}

	// Verify we have the right mix of retired and current
	retiredCount := 0
	currentCount := 0

	for _, key := range encryptionKeys {
	 keyMap := key.(map[string]interface{})
	 status := keyMap["status"].(string)

	 switch status {
	 case "retired":
		 retiredCount++
	 case "current":
		 currentCount++
	 default:
		 t.Errorf("unexpected status: %s", status)
	 }
	}

	if retiredCount != 2 {
	 t.Errorf("expected 2 retired epochs, got %d", retiredCount)
	}

	if currentCount != 1 {
	 t.Errorf("expected 1 current epoch, got %d", currentCount)
	}

	t.Logf("✓ Multi-epoch manifest contains %d retired, %d current epochs", retiredCount, currentCount)
}

// TestSampleCommitsData verifies loading sample commit data
func TestSampleCommitsData(t *testing.T) {
	commits := retiredepoch.LoadSampleCommits(t, "sample-commits-2023-12.json")

	if len(commits) != 1 {
	 t.Errorf("expected 1 sample commit, got %d", len(commits))
	}

	// Verify the commit references the correct epoch key_id
	commit := commits[0]
	if commit["epoch_key_id"] != "epoch-2023-12-retired" {
	 t.Errorf("expected epoch_key_id 'epoch-2023-12-retired', got %v", commit["epoch_key_id"])
	}

	t.Logf("✓ Sample commits data loaded successfully")
}

// TestGetRetiredEpochFixtures verifies accessing all retired epoch fixtures
func TestGetRetiredEpochFixtures(t *testing.T) {
	fixtures := retiredepoch.GetRetiredEpochFixtures(t)

	if len(fixtures) < 1 {
	 t.Fatal("expected at least 1 retired epoch fixture")
	}

	// Verify we have a primary fixture
	primaryFound := false
	for _, fixture := range fixtures {
	 if fixture.PrimaryTestFixture {
	 primaryFound = true
	 t.Logf("Primary retired fixture: key_id=%s, epoch=%s", fixture.KeyID, fixture.Epoch)
	 }
	}

	if !primaryFound {
	 t.Error("no primary retired epoch fixture found")
	}

	t.Logf("✓ Found %d retired epoch fixtures", len(fixtures))
}

// TestFixtureAccessibility verifies all fixtures are accessible from test suite
func TestFixtureAccessibility(t *testing.T) {
	fixtureDir := retiredepoch.FixtureDir()
	t.Logf("Fixture directory: %s", fixtureDir)

	// Try loading each type of fixture
	files := []string{
	 "manifest-retired-epoch.json",
	 "manifest-current-epoch.json",
	 "manifest-multi-epoch.json",
	 "fixture-index.json",
	 "README.md",
	}

	for _, file := range files {
	 manifest := retiredepoch.LoadManifest(t, file)
	 t.Logf("✓ Successfully loaded: %s", file)
	 _ = manifest // Use the variable to avoid unused warning
	}

	t.Logf("✓ All fixture files are accessible")
}

// TestPrimaryRetiredFixture verifies the primary retired epoch fixture structure
func TestPrimaryRetiredFixture(t *testing.T) {
	fixture := retiredepoch.GetPrimaryRetiredFixture(t)

	if fixture.KeyID != "epoch-2023-12-retired" {
	 t.Errorf("expected primary fixture key_id 'epoch-2023-12-retired', got %s", fixture.KeyID)
	}

	if fixture.Status != "retired" {
	 t.Errorf("expected primary fixture status 'retired', got %s", fixture.Status)
	}

	if !fixture.PrimaryTestFixture {
	 t.Error("expected PrimaryTestFixture to be true for primary fixture")
	}

	t.Logf("✓ Primary retired fixture validated: key_id=%s", fixture.KeyID)
}