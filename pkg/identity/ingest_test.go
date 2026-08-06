package identity

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mockDB is a test double for the DB interface.
type mockDB struct {
	rowsReceived []ResolutionRow
	shouldError  bool
}

func (m *mockDB) IngestEmailResolution(ctx context.Context, rows []ResolutionRow) (*IngestResult, error) {
	m.rowsReceived = rows
	if m.shouldError {
		return nil, errors.New("test database error")
	}
	// Return empty result for successful test
	return &IngestResult{
		Ingested:    int64(len(rows)),
		Skipped:     0,
		SkipDetails: make(map[SkipReason]int64),
	}, nil
}

// TestNewIngester verifies ingester construction.
func TestNewIngester(t *testing.T) {
	db := &mockDB{}
	ingester := NewIngester(db)
	if ingester == nil {
		t.Fatal("NewIngester returned nil")
	}
	if ingester.db != db {
		t.Error("ingester db field not set correctly")
	}
}

// TestIngestResolution_EmptyBatch verifies empty batch handling.
func TestIngestResolution_EmptyBatch(t *testing.T) {
	db := &mockDB{}
	ingester := NewIngester(db)

	err := ingester.IngestResolution(context.Background(), []ResolutionRow{})
	if err != nil {
		t.Errorf("IngestResolution with empty batch failed: %v", err)
	}
	if len(db.rowsReceived) != 0 {
		t.Error("expected no rows to be passed to DB for empty batch")
	}
}

// TestIngestResolution_Validation verifies row validation.
func TestIngestResolution_Validation(t *testing.T) {
	tests := []struct {
		name    string
		row     ResolutionRow
		wantErr string
	}{
		{
			name: "valid row",
			row: ResolutionRow{
				Email:      "test@example.com",
				Login:      "testuser",
				Source:     SourceLive,
				ResolvedAt: time.Now().UTC(),
			},
			wantErr: "",
		},
		{
			name: "empty email",
			row: ResolutionRow{
				Email:      "",
				Login:      "testuser",
				Source:     SourceLive,
				ResolvedAt: time.Now().UTC(),
			},
			wantErr: "email cannot be empty",
		},
		{
			name: "empty login",
			row: ResolutionRow{
				Email:      "test@example.com",
				Login:      "",
				Source:     SourceLive,
				ResolvedAt: time.Now().UTC(),
			},
			wantErr: "login cannot be empty",
		},
		{
			name: "invalid source",
			row: ResolutionRow{
				Email:      "test@example.com",
				Login:      "testuser",
				Source:     Source("invalid"),
				ResolvedAt: time.Now().UTC(),
			},
			wantErr: "invalid source",
		},
		{
			name: "zero resolved_at",
			row: ResolutionRow{
				Email:      "test@example.com",
				Login:      "testuser",
				Source:     SourceLive,
				ResolvedAt: time.Time{},
			},
			wantErr: "resolved_at cannot be zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := &mockDB{}
			ingester := NewIngester(db)

			rows := []ResolutionRow{tt.row}
			err := ingester.IngestResolution(context.Background(), rows)

			if tt.wantErr != "" {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.wantErr)
				} else if err.Error()[:7] != "row 0: " {
					t.Errorf("error should be prefixed with row index: %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// TestIngestResolution_AllSources verifies all three source types are accepted.
func TestIngestResolution_AllSources(t *testing.T) {
	now := time.Now().UTC()
	rows := []ResolutionRow{
		{
			Email:      "live@example.com",
			Login:      "liveuser",
			Source:     SourceLive,
			ResolvedAt: now,
		},
		{
			Email:      "seed@example.com",
			Login:      "seeduser",
			Source:     SourceSeed,
			ResolvedAt: now,
		},
		{
			Email:      "manual@example.com",
			Login:      "manualuser",
			Source:     SourceManual,
			ResolvedAt: now,
		},
	}

	db := &mockDB{}
	ingester := NewIngester(db)

	err := ingester.IngestResolution(context.Background(), rows)
	if err != nil {
		t.Errorf("IngestResolution failed: %v", err)
	}

	if len(db.rowsReceived) != 3 {
		t.Errorf("expected 3 rows received, got %d", len(db.rowsReceived))
	}

	// Verify sources are preserved
	if db.rowsReceived[0].Source != SourceLive {
		t.Errorf("expected SourceLive, got %v", db.rowsReceived[0].Source)
	}
	if db.rowsReceived[1].Source != SourceSeed {
		t.Errorf("expected SourceSeed, got %v", db.rowsReceived[1].Source)
	}
	if db.rowsReceived[2].Source != SourceManual {
		t.Errorf("expected SourceManual, got %v", db.rowsReceived[2].Source)
	}
}

// TestIngestResolution_PropagatesDBError verifies database errors are propagated.
func TestIngestResolution_PropagatesDBError(t *testing.T) {
	db := &mockDB{shouldError: true}
	ingester := NewIngester(db)

	rows := []ResolutionRow{
		{
			Email:      "test@example.com",
			Login:      "testuser",
			Source:     SourceLive,
			ResolvedAt: time.Now().UTC(),
		},
	}

	err := ingester.IngestResolution(context.Background(), rows)
	if err == nil {
		t.Fatal("expected database error to be propagated, got nil")
	}
	if err.Error() != "test database error" {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestIngester_ProcessedCounter verifies the processed counter tracks total records.
func TestIngester_ProcessedCounter(t *testing.T) {
	db := &mockDB{}
	ingester := NewIngester(db)

	// Initially, counter should be zero
	if ingester.GetProcessed() != 0 {
		t.Errorf("expected initial processed count to be 0, got %d", ingester.GetProcessed())
	}

	now := time.Now().UTC()

	// First batch of 2 rows
	rows1 := []ResolutionRow{
		{
			Email:      "user1@example.com",
			Login:      "user1",
			Source:     SourceLive,
			ResolvedAt: now,
		},
		{
			Email:      "user2@example.com",
			Login:      "user2",
			Source:     SourceLive,
			ResolvedAt: now,
		},
	}

	err := ingester.IngestResolution(context.Background(), rows1)
	if err != nil {
		t.Fatalf("first batch failed: %v", err)
	}

	// Counter should be 2 after first batch
	if ingester.GetProcessed() != 2 {
		t.Errorf("expected processed count to be 2 after first batch, got %d", ingester.GetProcessed())
	}

	// Second batch of 3 rows
	rows2 := []ResolutionRow{
		{
			Email:      "user3@example.com",
			Login:      "user3",
			Source:     SourceSeed,
			ResolvedAt: now,
		},
		{
			Email:      "user4@example.com",
			Login:      "user4",
			Source:     SourceSeed,
			ResolvedAt: now,
		},
		{
			Email:      "user5@example.com",
			Login:      "user5",
			Source:     SourceSeed,
			ResolvedAt: now,
		},
	}

	err = ingester.IngestResolution(context.Background(), rows2)
	if err != nil {
		t.Fatalf("second batch failed: %v", err)
	}

	// Counter should be 5 after second batch (2 + 3)
	if ingester.GetProcessed() != 5 {
		t.Errorf("expected processed count to be 5 after second batch, got %d", ingester.GetProcessed())
	}

	// Empty batch should not change counter
	err = ingester.IngestResolution(context.Background(), []ResolutionRow{})
	if err != nil {
		t.Fatalf("empty batch failed: %v", err)
	}

	if ingester.GetProcessed() != 5 {
		t.Errorf("expected processed count to remain 5 after empty batch, got %d", ingester.GetProcessed())
	}
}

// TestIngester_ProcessedCounter_SingleRecord verifies single record ingest (counter = 1).
func TestIngester_ProcessedCounter_SingleRecord(t *testing.T) {
	db := &mockDB{}
	ingester := NewIngester(db)

	// Initially, counter should be zero
	if ingester.GetProcessed() != 0 {
		t.Errorf("expected initial processed count to be 0, got %d", ingester.GetProcessed())
	}

	now := time.Now().UTC()

	// Single row
	rows := []ResolutionRow{
		{
			Email:      "single@example.com",
			Login:      "singleuser",
			Source:     SourceLive,
			ResolvedAt: now,
		},
	}

	err := ingester.IngestResolution(context.Background(), rows)
	if err != nil {
		t.Fatalf("single record ingest failed: %v", err)
	}

	// Counter should be 1 after single record
	if ingester.GetProcessed() != 1 {
		t.Errorf("expected processed count to be 1 after single record, got %d", ingester.GetProcessed())
	}
}

// TestIngester_ProcessedCounter_Reingest verifies re-ingest scenarios (counter tracks all attempts).
func TestIngester_ProcessedCounter_Reingest(t *testing.T) {
	db := &mockDB{}
	ingester := NewIngester(db)

	now := time.Now().UTC()

	// First ingest of a record
	rows1 := []ResolutionRow{
		{
			Email:      "user@example.com",
			Login:      "user1",
			Source:     SourceLive,
			ResolvedAt: now,
		},
	}

	err := ingester.IngestResolution(context.Background(), rows1)
	if err != nil {
		t.Fatalf("first ingest failed: %v", err)
	}

	if ingester.GetProcessed() != 1 {
		t.Errorf("expected processed count to be 1 after first ingest, got %d", ingester.GetProcessed())
	}

	// Re-ingest the same email with different data (this would cause ON CONFLICT in real DB)
	rows2 := []ResolutionRow{
		{
			Email:      "user@example.com", // Same email
			Login:      "user2",             // Different login
			Source:     SourceSeed,
			ResolvedAt: now.Add(time.Hour),
		},
	}

	err = ingester.IngestResolution(context.Background(), rows2)
	if err != nil {
		t.Fatalf("re-ingest failed: %v", err)
	}

	// Counter should be 2 (1 + 1) because it tracks all ingest attempts
	if ingester.GetProcessed() != 2 {
		t.Errorf("expected processed count to be 2 after re-ingest, got %d", ingester.GetProcessed())
	}

	// Ingest a batch with mixed new and duplicate emails
	rows3 := []ResolutionRow{
		{
			Email:      "user@example.com", // Duplicate
			Login:      "user3",
			Source:     SourceManual,
			ResolvedAt: now.Add(2 * time.Hour),
		},
		{
			Email:      "new@example.com", // New
			Login:      "newuser",
			Source:     SourceLive,
			ResolvedAt: now,
		},
		{
			Email:      "another@example.com", // New
			Login:      "another",
			Source:     SourceLive,
			ResolvedAt: now,
		},
	}

	err = ingester.IngestResolution(context.Background(), rows3)
	if err != nil {
		t.Fatalf("mixed batch failed: %v", err)
	}

	// Counter should be 5 (1 + 1 + 3) = 5 total records processed
	if ingester.GetProcessed() != 5 {
		t.Errorf("expected processed count to be 5 after mixed batch, got %d", ingester.GetProcessed())
	}
}

// TestResolutionRow_Validate tests the Validate method directly.
func TestResolutionRow_Validate(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name    string
		row     ResolutionRow
		wantErr bool
	}{
		{
			name: "valid live row",
			row: ResolutionRow{
				Email:      "user@example.com",
				Login:      "user",
				Source:     SourceLive,
				ResolvedAt: now,
			},
			wantErr: false,
		},
		{
			name: "valid seed row",
			row: ResolutionRow{
				Email:      "user@example.com",
				Login:      "user",
				Source:     SourceSeed,
				ResolvedAt: now,
			},
			wantErr: false,
		},
		{
			name: "valid manual row",
			row: ResolutionRow{
				Email:      "user@example.com",
				Login:      "user",
				Source:     SourceManual,
				ResolvedAt: now,
			},
			wantErr: false,
		},
		{
			name: "empty email",
			row: ResolutionRow{
				Email:      "",
				Login:      "user",
				Source:     SourceLive,
				ResolvedAt: now,
			},
			wantErr: true,
		},
		{
			name: "empty login",
			row: ResolutionRow{
				Email:      "user@example.com",
				Login:      "",
				Source:     SourceLive,
				ResolvedAt: now,
			},
			wantErr: true,
		},
		{
			name: "invalid source",
			row: ResolutionRow{
				Email:      "user@example.com",
				Login:      "user",
				Source:     Source("bogus"),
				ResolvedAt: now,
			},
			wantErr: true,
		},
		{
			name: "zero time",
			row: ResolutionRow{
				Email:      "user@example.com",
				Login:      "user",
				Source:     SourceLive,
				ResolvedAt: time.Time{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.row.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
