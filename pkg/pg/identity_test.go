package pg

import (
	"context"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/jedarden/commitgraph/pkg/identity"
)

// mockExecutor is a test double that captures SQL execution without a real database.
type mockExecutor struct {
	lastQuery    string
	lastArgs     []interface{}
	rowsAffected int64
	shouldError  bool
}

func (m *mockExecutor) ExecContext(ctx context.Context, query string, args ...interface{}) (Result, error) {
	m.lastQuery = query
	m.lastArgs = args
	if m.shouldError {
		return nil, &mockError{err: "test error"}
	}
	return &mockResult{rowsAffected: m.rowsAffected}, nil
}

type mockResult struct {
	rowsAffected int64
}

func (m *mockResult) RowsAffected() (int64, error) {
	return m.rowsAffected, nil
}

type mockError struct {
	err string
}

func (m *mockError) Error() string {
	return m.err
}

// Test that the SQL query contains the exact ON CONFLICT rule from plan.md
func TestIngestEmailResolution_SQLExact(t *testing.T) {
	db := &mockExecutor{}
	ingester := NewIdentityIngester(db)

	rows := []identity.ResolutionRow{
		{
			Email:      "test@example.com",
			Login:      "testuser",
			Source:     identity.SourceLive,
			ResolvedAt: time.Now().UTC(),
		},
	}

	err := ingester.IngestEmailResolution(context.Background(), rows)
	if err != nil {
		t.Fatalf("IngestEmailResolution failed: %v", err)
	}

	// Check that we got a query
	if db.lastQuery == "" {
		t.Fatal("no SQL query was executed")
	}

	// Verify core components of the ON CONFLICT rule
	requiredPatterns := []string{
		`INSERT INTO email_resolution`,
		`ON CONFLICT \(email\) DO UPDATE`,
		`SET login = excluded\.login,\s*source = excluded\.source,\s*resolved_at = excluded\.resolved_at`,
		`WHERE excluded\.source = 'manual'`,
		`OR \(email_resolution\.source <> 'manual'`,
		`AND excluded\.resolved_at > email_resolution\.resolved_at\)`,
	}

	for _, pattern := range requiredPatterns {
		matched, err := regexp.MatchString(pattern, db.lastQuery)
		if err != nil {
			t.Fatalf("invalid regex pattern %q: %v", pattern, err)
		}
		if !matched {
			t.Errorf("SQL query does not contain required pattern %q\nActual query:\n%s", pattern, db.lastQuery)
		}
	}
}

// Test that empty batch is handled correctly
func TestIngestEmailResolution_EmptyBatch(t *testing.T) {
	db := &mockExecutor{}
	ingester := NewIdentityIngester(db)

	rows := []identity.ResolutionRow{}
	err := ingester.IngestEmailResolution(context.Background(), rows)
	if err != nil {
		t.Fatalf("IngestEmailResolution with empty batch failed: %v", err)
	}

	// No query should be executed for empty batch
	if db.lastQuery != "" {
		t.Error("expected no SQL query for empty batch, but got one")
	}
}

// Test that database errors are propagated
func TestIngestEmailResolution_DatabaseError(t *testing.T) {
	db := &mockExecutor{shouldError: true}
	ingester := NewIdentityIngester(db)

	rows := []identity.ResolutionRow{
		{
			Email:      "test@example.com",
			Login:      "testuser",
			Source:     identity.SourceLive,
			ResolvedAt: time.Now().UTC(),
		},
	}

	err := ingester.IngestEmailResolution(context.Background(), rows)
	if err == nil {
		t.Fatal("expected error from database, but got nil")
	}
	if err.Error() != "bulk upsert failed: test error" {
		t.Errorf("unexpected error message: %v", err)
	}
}

// Test that all three source types are handled correctly
func TestIngestEmailResolution_AllSources(t *testing.T) {
	db := &mockExecutor{}
	ingester := NewIdentityIngester(db)

	now := time.Now().UTC()
	rows := []identity.ResolutionRow{
		{
			Email:      "live@example.com",
			Login:      "liveuser",
			Source:     identity.SourceLive,
			ResolvedAt: now,
		},
		{
			Email:      "seed@example.com",
			Login:      "seeduser",
			Source:     identity.SourceSeed,
			ResolvedAt: now,
		},
		{
			Email:      "manual@example.com",
			Login:      "manualuser",
			Source:     identity.SourceManual,
			ResolvedAt: now,
		},
	}

	err := ingester.IngestEmailResolution(context.Background(), rows)
	if err != nil {
		t.Fatalf("IngestEmailResolution failed: %v", err)
	}

	// Verify that all three sources were passed to SQL
	args := db.lastArgs
	if len(args) != 4 {
		t.Fatalf("expected 4 array arguments, got %d", len(args))
	}

	// args[0] = emails, args[1] = logins, args[2] = sources, args[3] = resolved_ats
	emails := args[0].([]string)
	sources := args[2].([]string)

	if len(emails) != 3 {
		t.Errorf("expected 3 email entries, got %d", len(emails))
	}
	if len(sources) != 3 {
		t.Errorf("expected 3 source entries, got %d", len(sources))
	}

	// Verify source values
	if sources[0] != "live" {
		t.Errorf("expected source 'live', got %q", sources[0])
	}
	if sources[1] != "seed" {
		t.Errorf("expected source 'seed', got %q", sources[1])
	}
	if sources[2] != "manual" {
		t.Errorf("expected source 'manual', got %q", sources[2])
	}
}

// Test that bulk operation handles many rows efficiently
func TestIngestEmailResolution_BulkBatch(t *testing.T) {
	db := &mockExecutor{}
	ingester := NewIdentityIngester(db)

	now := time.Now().UTC()
	rows := make([]identity.ResolutionRow, 1000)
	for i := 0; i < 1000; i++ {
		rows[i] = identity.ResolutionRow{
			Email:      fmt.Sprintf("user%d@example.com", i),
			Login:      fmt.Sprintf("user%d", i),
			Source:     identity.SourceSeed,
			ResolvedAt: now,
		}
	}

	err := ingester.IngestEmailResolution(context.Background(), rows)
	if err != nil {
		t.Fatalf("IngestEmailResolution with bulk batch failed: %v", err)
	}

	// Should be a single query for the whole batch
	emails := db.lastArgs[0].([]string)
	if len(emails) != 1000 {
		t.Errorf("expected 1000 email entries, got %d", len(emails))
	}
}
