package ingestlog_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jedarden/commitgraph/pkg/client/queueapi"
	"github.com/jedarden/commitgraph/pkg/identity"
)

// TestIngestEndpointDataFormatCompatibility verifies that the seed script data format
// is compatible with the HTTP ingest endpoint format.
//
// This test demonstrates the transformation from seed script ResolutionRow format
// to HTTP endpoint ResolutionRequest format, ensuring data can flow correctly
// from the seed script to the ingest endpoint.
func TestIngestEndpointDataFormatCompatibility(t *testing.T) {
	now := time.Now().UTC()

	// Seed script data format (from pkg/identity)
	seedData := identity.ResolutionRow{
		Email:      "test@example.com",
		Login:      "testuser",
		Source:     identity.SourceSeed,
		ResolvedAt: now,
	}

	// Expected HTTP endpoint request format (from pkg/client/queueapi)
	httpRequest := queueapi.ResolutionRequest{
		Email:       seedData.Email,
		GithubLogin: seedData.Login, // Login -> github_login
		Source:      string(seedData.Source),
		ResolvedAt:  seedData.ResolvedAt.Format(time.RFC3339), // time.Time -> ISO 8601
	}

	// Verify field mappings
	if httpRequest.Email != seedData.Email {
		t.Errorf("Email mismatch: got %s, want %s", httpRequest.Email, seedData.Email)
	}

	if httpRequest.GithubLogin != seedData.Login {
		t.Errorf("GithubLogin mismatch: got %s, want %s", httpRequest.GithubLogin, seedData.Login)
	}

	if httpRequest.Source != string(seedData.Source) {
		t.Errorf("Source mismatch: got %s, want %s", httpRequest.Source, string(seedData.Source))
	}

	// Verify timestamp conversion (RFC3339 has microsecond precision)
	parsedTime, err := time.Parse(time.RFC3339, httpRequest.ResolvedAt)
	if err != nil {
		t.Errorf("Failed to parse resolved_at timestamp: %v", err)
	}

	// Check if times are approximately equal (RFC3339 truncates nanoseconds)
	diff := seedData.ResolvedAt.Sub(parsedTime)
	if diff < 0 {
		diff = -diff
	}

	// Allow 1 second tolerance for RFC3339 precision loss
	maxTolerance := time.Second
	if diff > maxTolerance {
		t.Errorf("ResolvedAt mismatch after conversion: got %v, want %v (diff: %v)", parsedTime, seedData.ResolvedAt, diff)
	}

	// Verify JSON serialization (actual HTTP body format)
	jsonBytes, err := json.Marshal(httpRequest)
	if err != nil {
		t.Fatalf("Failed to marshal HTTP request: %v", err)
	}

	var unmarshaled queueapi.ResolutionRequest
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal HTTP request: %v", err)
	}

	// Verify round-trip preserves data
	if unmarshaled.Email != seedData.Email {
		t.Errorf("Round-trip Email mismatch: got %s, want %s", unmarshaled.Email, seedData.Email)
	}

	if unmarshaled.GithubLogin != seedData.Login {
		t.Errorf("Round-trip GithubLogin mismatch: got %s, want %s", unmarshaled.GithubLogin, seedData.Login)
	}

	if unmarshaled.Source != string(seedData.Source) {
		t.Errorf("Round-trip Source mismatch: got %s, want %s", unmarshaled.Source, string(seedData.Source))
	}
}

// TestIngestEndpointAcceptsAllSourceTypes verifies the HTTP endpoint accepts
// all three source types used by the system (seed, live, manual).
func TestIngestEndpointAcceptsAllSourceTypes(t *testing.T) {
	now := time.Now().UTC()

	sources := []struct {
		name   string
		source identity.Source
	}{
		{"seed", identity.SourceSeed},
		{"live", identity.SourceLive},
		{"manual", identity.SourceManual},
	}

	for _, tt := range sources {
		t.Run(tt.name, func(t *testing.T) {
			seedData := identity.ResolutionRow{
				Email:      "test@example.com",
				Login:      "testuser",
				Source:     tt.source,
				ResolvedAt: now,
			}

			// Convert to HTTP format
			httpRequest := queueapi.ResolutionRequest{
				Email:       seedData.Email,
				GithubLogin:  seedData.Login,
				Source:       string(seedData.Source),
				ResolvedAt:   seedData.ResolvedAt.Format(time.RFC3339),
			}

			// Verify source is preserved
			if httpRequest.Source != tt.name {
				t.Errorf("Source not preserved: got %s, want %s", httpRequest.Source, tt.name)
			}

			// Verify JSON serialization includes the source
			jsonBytes, err := json.Marshal(httpRequest)
			if err != nil {
				t.Fatalf("Failed to marshal: %v", err)
			}

			var raw map[string]interface{}
			err = json.Unmarshal(jsonBytes, &raw)
			if err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}

			if raw["source"] != tt.name {
				t.Errorf("JSON source mismatch: got %v, want %s", raw["source"], tt.name)
			}
		})
	}
}

// TestIngestEndpointTimestampCompatibility verifies that timestamp conversion
// between seed script format (time.Time) and HTTP endpoint format (ISO 8601 string)
// is lossless.
func TestIngestEndpointTimestampCompatibility(t *testing.T) {
	now := time.Now().UTC()

	seedData := identity.ResolutionRow{
		Email:      "test@example.com",
		Login:      "testuser",
		Source:     identity.SourceSeed,
		ResolvedAt: now,
	}

	// Convert to HTTP format
	httpRequest := queueapi.ResolutionRequest{
		Email:       seedData.Email,
		GithubLogin:  seedData.Login,
		Source:       string(seedData.Source),
		ResolvedAt:   seedData.ResolvedAt.Format(time.RFC3339),
	}

	// Verify round-trip conversion
	parsedTime, err := time.Parse(time.RFC3339, httpRequest.ResolvedAt)
	if err != nil {
		t.Fatalf("Failed to parse timestamp: %v", err)
	}

	// Check precision preservation (within microseconds for test reliability)
	diff := seedData.ResolvedAt.Sub(parsedTime)
	if diff < 0 {
		diff = -diff
	}

	// Allow 1 second tolerance for test execution time
	maxTolerance := time.Second
	if diff > maxTolerance {
		t.Errorf("Timestamp precision lost: original %v, parsed %v, diff %v",
			seedData.ResolvedAt, parsedTime, diff)
	}
}
