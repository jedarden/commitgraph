package pg

import (
	"context"
	"testing"
	"time"

	"github.com/jedarden/commitgraph/pkg/identity"
)

// TestIngestEmailResolution_CounterTracking verifies that the Postgres implementation
// properly tracks ingested and skipped records with detailed skip reasons.
func TestIngestEmailResolution_CounterTracking(t *testing.T) {
	tests := []struct {
		name              string
		rows              []identity.ResolutionRow
		existingRows      map[string]struct{ login, source string; resolvedAt time.Time }
		wantIngested      int64
		wantSkipped       int64
		wantSkipManual    int64
		wantSkipOlder     int64
	}{
		{
			name: "all new rows - all ingested",
			rows: []identity.ResolutionRow{
				{
					Email:      "new1@example.com",
					Login:      "newuser1",
					Source:     identity.SourceLive,
					ResolvedAt: time.Now().UTC(),
				},
				{
					Email:      "new2@example.com",
					Login:      "newuser2",
					Source:     identity.SourceSeed,
					ResolvedAt: time.Now().UTC(),
				},
			},
			existingRows:   map[string]struct{ login, source string; resolvedAt time.Time }{},
			wantIngested:   2,
			wantSkipped:    0,
			wantSkipManual: 0,
			wantSkipOlder:  0,
		},
		{
			name: "existing manual wins - all skipped",
			rows: []identity.ResolutionRow{
				{
					Email:      "existing@example.com",
					Login:      "newlogin",
					Source:     identity.SourceLive,
					ResolvedAt: time.Now().UTC(),
				},
			},
			existingRows: map[string]struct{ login, source string; resolvedAt time.Time }{
				"existing@example.com": {
					login:      "oldlogin",
					source:     "manual",
					resolvedAt: time.Now().Add(-1 * time.Hour),
				},
			},
			wantIngested:   0,
			wantSkipped:    1,
			wantSkipManual: 1,
			wantSkipOlder:  0,
		},
		{
			name: "existing newer wins - skipped as older",
			rows: []identity.ResolutionRow{
				{
					Email:      "existing@example.com",
					Login:      "newlogin",
					Source:     identity.SourceLive,
					ResolvedAt: time.Now().Add(-1 * time.Hour),
				},
			},
			existingRows: map[string]struct{ login, source string; resolvedAt time.Time }{
				"existing@example.com": {
					login:      "oldlogin",
					source:     "live",
					resolvedAt: time.Now(),
				},
			},
			wantIngested:   0,
			wantSkipped:    1,
			wantSkipManual: 0,
			wantSkipOlder:  1,
		},
		{
			name: "new manual wins over existing",
			rows: []identity.ResolutionRow{
				{
					Email:      "existing@example.com",
					Login:      "newlogin",
					Source:     identity.SourceManual,
					ResolvedAt: time.Now().UTC(),
				},
			},
			existingRows: map[string]struct{ login, source string; resolvedAt time.Time }{
				"existing@example.com": {
					login:      "oldlogin",
					source:     "live",
					resolvedAt: time.Now(),
				},
			},
			wantIngested:   1,
			wantSkipped:    0,
			wantSkipManual: 0,
			wantSkipOlder:  0,
		},
		{
			name: "mixed results",
			rows: []identity.ResolutionRow{
				{
					Email:      "new@example.com",
					Login:      "newuser",
					Source:     identity.SourceLive,
					ResolvedAt: time.Now().UTC(),
				},
				{
					Email:      "existing-manual@example.com",
					Login:      "challenger",
					Source:     identity.SourceLive,
					ResolvedAt: time.Now().UTC(),
				},
				{
					Email:      "existing-older@example.com",
					Login:      "challenger",
					Source:     identity.SourceLive,
					ResolvedAt: time.Now().Add(-1 * time.Hour),
				},
			},
			existingRows: map[string]struct{ login, source string; resolvedAt time.Time }{
				"existing-manual@example.com": {
					login:      "winner",
					source:     "manual",
					resolvedAt: time.Now().Add(-1 * time.Hour),
				},
				"existing-older@example.com": {
					login:      "winner",
					source:     "live",
					resolvedAt: time.Now(),
				},
			},
			wantIngested:   1,
			wantSkipped:    2,
			wantSkipManual: 1,
			wantSkipOlder:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock that simulates existing rows
			db := &mockExecutorWithRows{
				existingRows: tt.existingRows,
			}
			ingester := NewIdentityIngester(db)

			result, err := ingester.IngestEmailResolution(context.Background(), tt.rows)
			if err != nil {
				t.Fatalf("IngestEmailResolution failed: %v", err)
			}

			if result.Ingested != tt.wantIngested {
				t.Errorf("Ingested = %d, want %d", result.Ingested, tt.wantIngested)
			}
			if result.Skipped != tt.wantSkipped {
				t.Errorf("Skipped = %d, want %d", result.Skipped, tt.wantSkipped)
			}
			if result.SkipDetails[identity.SkipReasonConflictManual] != tt.wantSkipManual {
				t.Errorf("SkipDetails[ConflictManual] = %d, want %d",
					result.SkipDetails[identity.SkipReasonConflictManual], tt.wantSkipManual)
			}
			if result.SkipDetails[identity.SkipReasonConflictOlder] != tt.wantSkipOlder {
				t.Errorf("SkipDetails[ConflictOlder] = %d, want %d",
					result.SkipDetails[identity.SkipReasonConflictOlder], tt.wantSkipOlder)
			}

			// Verify Processed = Ingested + Skipped invariant
			total := int64(len(tt.rows))
			if result.Ingested+result.Skipped != total {
				t.Errorf("Processed invariant failed: Ingested(%d) + Skipped(%d) = %d, want %d",
					result.Ingested, result.Skipped, result.Ingested+result.Skipped, total)
			}
		})
	}
}

// mockExecutorWithRows simulates a database with pre-existing rows.
type mockExecutorWithRows struct {
	mockExecutor
	existingRows map[string]struct{ login, source string; resolvedAt time.Time }
}

func (m *mockExecutorWithRows) QueryContext(ctx context.Context, query string, args ...interface{}) (Rows, error) {
	// Check if this is the fetch query
	if len(args) > 0 {
		if emails, ok := args[0].([]string); ok {
			// Return mock rows with existing data
			return &mockRowsWithData{existingRows: m.existingRows, emails: emails}, nil
		}
	}
	return &mockRows{}, nil
}

// mockRowsWithData implements Rows interface with actual existing row data.
type mockRowsWithData struct {
	existingRows  map[string]struct{ login, source string; resolvedAt time.Time }
	emails        []string
	currentIndex  int
}

func (m *mockRowsWithData) Next() bool {
	// Find the next email that has existing data
	for m.currentIndex < len(m.emails) {
		email := m.emails[m.currentIndex]
		if _, exists := m.existingRows[email]; exists {
			return true
		}
		m.currentIndex++
	}
	return false
}

func (m *mockRowsWithData) Scan(dest ...interface{}) error {
	if m.currentIndex >= len(m.emails) {
		return nil
	}

	email := m.emails[m.currentIndex]
	row, exists := m.existingRows[email]
	if !exists {
		m.currentIndex++
		return nil
	}

	// Expected scan order: email, login, source, resolved_at
	if len(dest) != 4 {
		return nil
	}

	if emailPtr, ok := dest[0].(*string); ok {
		*emailPtr = email
	}
	if loginPtr, ok := dest[1].(*string); ok {
		*loginPtr = row.login
	}
	if sourcePtr, ok := dest[2].(*string); ok {
		*sourcePtr = row.source
	}
	if resolvedAtPtr, ok := dest[3].(*time.Time); ok {
		*resolvedAtPtr = row.resolvedAt
	}

	m.currentIndex++
	return nil
}

func (m *mockRowsWithData) Close() error {
	return nil
}

func (m *mockRowsWithData) Err() error {
	return nil
}
