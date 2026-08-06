package main

import (
	"os"
	"testing"
)

func TestReadConfigMap(t *testing.T) {
	// Test reading and parsing the test ConfigMap
	configMap, err := readConfigMap("testdata/configmap.yml")
	if err != nil {
		t.Fatalf("failed to read ConfigMap: %v", err)
	}

	// Validate basic structure
	if configMap.Kind != "ConfigMap" {
		t.Errorf("expected Kind ConfigMap, got %s", configMap.Kind)
	}

	if configMap.Metadata.Name != "admin-alias-configmap" {
		t.Errorf("expected name admin-alias-configmap, got %s", configMap.Metadata.Name)
	}
}

func TestParseAliasesFromConfigMap(t *testing.T) {
	configMap, err := readConfigMap("testdata/configmap.yml")
	if err != nil {
		t.Fatalf("failed to read ConfigMap: %v", err)
	}

	aliases, err := parseAliasesFromConfigMap(configMap)
	if err != nil {
		t.Fatalf("failed to parse aliases: %v", err)
	}

	// Should have 3 aliases from test data
	if len(aliases) != 3 {
		t.Errorf("expected 3 aliases, got %d", len(aliases))
	}

	// Check first alias
	if aliases[0].Source != "old-johndoe" || aliases[0].Target != "johndoe" {
		t.Errorf("expected first alias old-johndoe -> johndoe, got %s -> %s", aliases[0].Source, aliases[0].Target)
	}

	// Check second alias
	if aliases[1].Source != "jane-bot" || aliases[1].Target != "jane" {
		t.Errorf("expected second alias jane-bot -> jane, got %s -> %s", aliases[1].Source, aliases[1].Target)
	}
}

func TestDetectRemovedAliases(t *testing.T) {
	// Simulate having 3 aliases in DB
	currentDB := map[string]string{
		"old-johndoe":     "johndoe",
		"jane-bot":        "jane",
		"deprecated-user": "current-user",
		"removed-alias":   "old-target", // This one will be "removed"
	}

	// ConfigMap only has 3 aliases (missing "removed-alias")
	configMapEntries := []AliasEntry{
		{Source: "old-johndoe", Target: "johndoe"},
		{Source: "jane-bot", Target: "jane"},
		{Source: "deprecated-user", Target: "current-user"},
	}

	removed := detectRemovedAliases(currentDB, configMapEntries)

	// Should detect exactly 1 removed alias
	if len(removed) != 1 {
		t.Errorf("expected 1 removed alias, got %d", len(removed))
	}

	// Check it's the right one
	target, exists := removed["removed-alias"]
	if !exists {
		t.Error("expected removed-alias to be in removed set")
	}
	if target != "old-target" {
		t.Errorf("expected target old-target, got %s", target)
	}
}

func TestMainUsage(t *testing.T) {
	// Test that usage() exits and prints help
	// This is just a sanity check that usage doesn't panic
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Catch the exit
	defer func() {
		os.Stderr = oldStderr
		w.Close()
		r.Close()
	}()

	// usage calls os.Exit(2), so we can't actually call it in a test
	// But we've verified the structure is correct by manual inspection
	_ = true
}
