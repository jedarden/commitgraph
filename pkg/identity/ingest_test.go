package identity

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// mockDB is a test double for the DB interface.
type mockDB struct {
	rowsReceived []ResolutionRow
	shouldError  bool
	result       *IngestResult // Allows customizing the result returned
}

func (m *mockDB) IngestEmailResolution(ctx context.Context, rows []ResolutionRow) (*IngestResult, error) {
	m.rowsReceived = rows
	if m.shouldError {
		return nil, errors.New("test database error")
	}
	// Use custom result if provided, otherwise default to all ingested
	if m.result != nil {
		return m.result, nil
	}
	// Return default result for successful test
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

// TestIngester_SkipDetailsInitialization verifies SkipDetails is initialized as non-nil map.
func TestIngester_SkipDetailsInitialization(t *testing.T) {
	db := &mockDB{}
	ingester := NewIngester(db)

	if ingester.SkipDetails == nil {
		t.Error("SkipDetails should be initialized as non-nil map")
	}

	if len(ingester.SkipDetails) != 0 {
		t.Errorf("SkipDetails should be empty initially, got %d entries", len(ingester.SkipDetails))
	}
}

// TestIngester_IngestedCounter verifies Ingested counter increments correctly.
func TestIngester_IngestedCounter(t *testing.T) {
	db := &mockDB{}
	ingester := NewIngester(db)

	// Initial state
	if ingester.GetIngested() != 0 {
		t.Errorf("expected initial ingested count to be 0, got %d", ingester.GetIngested())
	}

	now := time.Now().UTC()

	// First batch - DB reports all ingested
	rows1 := []ResolutionRow{
		{Email: "user1@example.com", Login: "user1", Source: SourceLive, ResolvedAt: now},
		{Email: "user2@example.com", Login: "user2", Source: SourceLive, ResolvedAt: now},
	}

	err := ingester.IngestResolution(context.Background(), rows1)
	if err != nil {
		t.Fatalf("first batch failed: %v", err)
	}

	// Ingested should be 2 (mock DB returns len(rows) as ingested)
	if ingester.GetIngested() != 2 {
		t.Errorf("expected ingested count to be 2 after first batch, got %d", ingester.GetIngested())
	}

	// Second batch with custom result
	db.result = &IngestResult{
		Ingested:    5,
		Skipped:     3,
		SkipDetails: map[SkipReason]int64{SkipReasonConflictManual: 2, SkipReasonConflictOlder: 1},
	}

	rows2 := []ResolutionRow{
		{Email: "user3@example.com", Login: "user3", Source: SourceSeed, ResolvedAt: now},
	}

	err = ingester.IngestResolution(context.Background(), rows2)
	if err != nil {
		t.Fatalf("second batch failed: %v", err)
	}

	// Ingested should be 2 + 5 = 7
	if ingester.GetIngested() != 7 {
		t.Errorf("expected ingested count to be 7 after second batch, got %d", ingester.GetIngested())
	}

	// Processed should be 3 (2 + 1)
	if ingester.GetProcessed() != 3 {
		t.Errorf("expected processed count to be 3, got %d", ingester.GetProcessed())
	}
}

// TestIngester_SkippedCounter verifies Skipped counter increments correctly.
func TestIngester_SkippedCounter(t *testing.T) {
	db := &mockDB{}
	ingester := NewIngester(db)

	// Initial state
	if ingester.GetSkipped() != 0 {
		t.Errorf("expected initial skipped count to be 0, got %d", ingester.GetSkipped())
	}

	now := time.Now().UTC()

	// First batch - DB reports some skipped
	db.result = &IngestResult{
		Ingested:    2,
		Skipped:     3,
		SkipDetails: map[SkipReason]int64{
			SkipReasonConflictManual: 1,
			SkipReasonConflictOlder:  2,
		},
	}

	rows := []ResolutionRow{
		{Email: "user1@example.com", Login: "user1", Source: SourceLive, ResolvedAt: now},
		{Email: "user2@example.com", Login: "user2", Source: SourceLive, ResolvedAt: now},
		{Email: "user3@example.com", Login: "user3", Source: SourceLive, ResolvedAt: now},
		{Email: "user4@example.com", Login: "user4", Source: SourceLive, ResolvedAt: now},
		{Email: "user5@example.com", Login: "user5", Source: SourceLive, ResolvedAt: now},
	}

	err := ingester.IngestResolution(context.Background(), rows)
	if err != nil {
		t.Fatalf("batch failed: %v", err)
	}

	// Skipped should be 3
	if ingester.GetSkipped() != 3 {
		t.Errorf("expected skipped count to be 3, got %d", ingester.GetSkipped())
	}

	// Second batch with different skip counts
	db.result = &IngestResult{
		Ingested:    1,
		Skipped:     4,
		SkipDetails: map[SkipReason]int64{
			SkipReasonConflictManual: 3,
			SkipReasonValidation:     1,
		},
	}

	rows2 := []ResolutionRow{
		{Email: "user6@example.com", Login: "user6", Source: SourceSeed, ResolvedAt: now},
	}

	err = ingester.IngestResolution(context.Background(), rows2)
	if err != nil {
		t.Fatalf("second batch failed: %v", err)
	}

	// Skipped should be 3 + 4 = 7
	if ingester.GetSkipped() != 7 {
		t.Errorf("expected skipped count to be 7 after second batch, got %d", ingester.GetSkipped())
	}
}

// TestIngester_SkipDetailsAccumulation verifies SkipDetails accumulates across multiple calls.
func TestIngester_SkipDetailsAccumulation(t *testing.T) {
	db := &mockDB{}
	ingester := NewIngester(db)

	now := time.Now().UTC()

	// First batch
	db.result = &IngestResult{
		Ingested:    3,
		Skipped:     2,
		SkipDetails: map[SkipReason]int64{
			SkipReasonConflictManual: 1,
			SkipReasonConflictOlder:  1,
		},
	}

	rows1 := []ResolutionRow{
		{Email: "user1@example.com", Login: "user1", Source: SourceLive, ResolvedAt: now},
	}

	err := ingester.IngestResolution(context.Background(), rows1)
	if err != nil {
		t.Fatalf("first batch failed: %v", err)
	}

	// Verify initial SkipDetails
	details := ingester.GetSkipDetails()
	if len(details) != 2 {
		t.Errorf("expected 2 skip reason categories, got %d", len(details))
	}
	if details[SkipReasonConflictManual] != 1 {
		t.Errorf("expected conflict_manual count to be 1, got %d", details[SkipReasonConflictManual])
	}
	if details[SkipReasonConflictOlder] != 1 {
		t.Errorf("expected conflict_older count to be 1, got %d", details[SkipReasonConflictOlder])
	}

	// Second batch - adds to existing reasons and introduces new ones
	db.result = &IngestResult{
		Ingested:    2,
		Skipped:     3,
		SkipDetails: map[SkipReason]int64{
			SkipReasonConflictManual: 2,  // Should add to existing
			SkipReasonValidation:     1,  // New reason
		},
	}

	rows2 := []ResolutionRow{
		{Email: "user2@example.com", Login: "user2", Source: SourceLive, ResolvedAt: now},
	}

	err = ingester.IngestResolution(context.Background(), rows2)
	if err != nil {
		t.Fatalf("second batch failed: %v", err)
	}

	// Verify accumulation
	details = ingester.GetSkipDetails()
	if len(details) != 3 {
		t.Errorf("expected 3 skip reason categories, got %d", len(details))
	}

	// conflict_manual should be 1 + 2 = 3
	if details[SkipReasonConflictManual] != 3 {
		t.Errorf("expected conflict_manual count to be 3 (accumulated), got %d", details[SkipReasonConflictManual])
	}

	// conflict_older should remain 1
	if details[SkipReasonConflictOlder] != 1 {
		t.Errorf("expected conflict_older count to remain 1, got %d", details[SkipReasonConflictOlder])
	}

	// validation should be 1
	if details[SkipReasonValidation] != 1 {
		t.Errorf("expected validation count to be 1, got %d", details[SkipReasonValidation])
	}

	// Third batch - zero skips should not affect existing details
	db.result = &IngestResult{
		Ingested:    5,
		Skipped:     0,
		SkipDetails: map[SkipReason]int64{},
	}

	rows3 := []ResolutionRow{
		{Email: "user3@example.com", Login: "user3", Source: SourceLive, ResolvedAt: now},
	}

	err = ingester.IngestResolution(context.Background(), rows3)
	if err != nil {
		t.Fatalf("third batch failed: %v", err)
	}

	// Verify no changes from zero-skip batch
	details = ingester.GetSkipDetails()
	if details[SkipReasonConflictManual] != 3 {
		t.Errorf("expected conflict_manual count to remain 3, got %d", details[SkipReasonConflictManual])
	}
}

// TestIngester_ProcessedInvariant verifies Processed = Ingested + Skipped invariant.
func TestIngester_ProcessedInvariant(t *testing.T) {
	now := time.Now().UTC()

	testCases := []struct {
		name     string
		result   *IngestResult
		rowCount int
	}{
		{
			name: "all ingested",
			result: &IngestResult{
				Ingested:    5,
				Skipped:     0,
				SkipDetails: map[SkipReason]int64{},
			},
			rowCount: 5,
		},
		{
			name: "all skipped",
			result: &IngestResult{
				Ingested:    0,
				Skipped:     4,
				SkipDetails: map[SkipReason]int64{
					SkipReasonConflictOlder: 4,
				},
			},
			rowCount: 4,
		},
		{
			name: "mixed ingest and skip",
			result: &IngestResult{
				Ingested:    3,
				Skipped:     2,
				SkipDetails: map[SkipReason]int64{
					SkipReasonConflictManual: 1,
					SkipReasonConflictOlder:  1,
				},
			},
			rowCount: 5,
		},
		{
			name: "multiple skip reasons",
			result: &IngestResult{
				Ingested:    2,
				Skipped:     8,
				SkipDetails: map[SkipReason]int64{
					SkipReasonConflictManual: 3,
					SkipReasonConflictOlder:  2,
					SkipReasonValidation:     2,
					SkipReasonDatabase:       1,
				},
			},
			rowCount: 10,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db := &mockDB{}
			ingester := NewIngester(db)

			// Build row batch
			rows := make([]ResolutionRow, tc.rowCount)
			for i := 0; i < tc.rowCount; i++ {
				rows[i] = ResolutionRow{
					Email:      fmt.Sprintf("user%d@example.com", i),
					Login:      fmt.Sprintf("user%d", i),
					Source:     SourceLive,
					ResolvedAt: now,
				}
			}

			db.result = tc.result

			err := ingester.IngestResolution(context.Background(), rows)
			if err != nil {
				t.Fatalf("batch failed: %v", err)
			}

			// Verify invariant: Processed = Ingested + Skipped
			processed := ingester.GetProcessed()
			ingested := ingester.GetIngested()
			skipped := ingester.GetSkipped()

			expectedProcessed := ingested + skipped
			if processed != expectedProcessed {
				t.Errorf("processed invariant violated: processed=%d, ingested=%d, skipped=%d, expected processed=%d",
					processed, ingested, skipped, expectedProcessed)
			}

			// Also verify it matches the input row count
			if processed != int64(tc.rowCount) {
				t.Errorf("processed count mismatch: got %d, expected %d (row count)",
					processed, tc.rowCount)
			}

			// Verify SkipDetails sum matches Skipped
			skipDetailsSum := int64(0)
			for _, count := range ingester.GetSkipDetails() {
				skipDetailsSum += count
			}
			if skipDetailsSum != skipped {
				t.Errorf("SkipDetails sum mismatch: sum=%d, skipped=%d", skipDetailsSum, skipped)
			}
		})
	}
}

// TestIngester_ProcessedInvariant_MultipleBatches verifies invariant across multiple batches.
func TestIngester_ProcessedInvariant_MultipleBatches(t *testing.T) {
	db := &mockDB{}
	ingester := NewIngester(db)

	now := time.Now().UTC()

	batches := []struct {
		ingested int64
		skipped  int64
		reasons  map[SkipReason]int64
	}{
		{5, 0, map[SkipReason]int64{}},                                    // All ingested
		{2, 3, map[SkipReason]int64{SkipReasonConflictManual: 3}},       // Mixed
		{0, 4, map[SkipReason]int64{SkipReasonConflictOlder: 4}},       // All skipped
		{3, 2, map[SkipReason]int64{SkipReasonValidation: 2}},           // Mixed with validation
	}

	totalProcessed := int64(0)
	totalIngested := int64(0)
	totalSkipped := int64(0)

	for i, batch := range batches {
		db.result = &IngestResult{
			Ingested:    batch.ingested,
			Skipped:     batch.skipped,
			SkipDetails: batch.reasons,
		}

		rowCount := batch.ingested + batch.skipped
		rows := make([]ResolutionRow, rowCount)
		for j := 0; j < int(rowCount); j++ {
			rows[j] = ResolutionRow{
				Email:      fmt.Sprintf("batch%d_user%d@example.com", i, j),
				Login:      fmt.Sprintf("user%d", j),
				Source:     SourceLive,
				ResolvedAt: now,
			}
		}

		err := ingester.IngestResolution(context.Background(), rows)
		if err != nil {
			t.Fatalf("batch %d failed: %v", i, err)
		}

		totalProcessed += rowCount
		totalIngested += batch.ingested
		totalSkipped += batch.skipped

		// Verify invariant after each batch
		processed := ingester.GetProcessed()
		ingested := ingester.GetIngested()
		skipped := ingester.GetSkipped()

		if processed != ingested+skipped {
			t.Errorf("batch %d: invariant violated: processed=%d, ingested=%d, skipped=%d",
				i, processed, ingested, skipped)
		}

		if processed != totalProcessed {
			t.Errorf("batch %d: processed mismatch: got %d, expected %d",
				i, processed, totalProcessed)
		}
	}

	// Final verification
	if ingester.GetProcessed() != totalProcessed {
		t.Errorf("final processed mismatch: got %d, expected %d",
			ingester.GetProcessed(), totalProcessed)
	}
	if ingester.GetIngested() != totalIngested {
		t.Errorf("final ingested mismatch: got %d, expected %d",
			ingester.GetIngested(), totalIngested)
	}
	if ingester.GetSkipped() != totalSkipped {
		t.Errorf("final skipped mismatch: got %d, expected %d",
			ingester.GetSkipped(), totalSkipped)
	}
}

// TestIngester_GetterMethods verifies all getter methods return correct values.
func TestIngester_GetterMethods(t *testing.T) {
	db := &mockDB{}
	ingester := NewIngester(db)

	now := time.Now().UTC()

	// Test with no data
	if ingester.GetProcessed() != 0 {
		t.Errorf("GetProcessed() should return 0 initially, got %d", ingester.GetProcessed())
	}
	if ingester.GetIngested() != 0 {
		t.Errorf("GetIngested() should return 0 initially, got %d", ingester.GetIngested())
	}
	if ingester.GetSkipped() != 0 {
		t.Errorf("GetSkipped() should return 0 initially, got %d", ingester.GetSkipped())
	}
	if len(ingester.GetSkipDetails()) != 0 {
		t.Errorf("GetSkipDetails() should return empty map initially, got %d entries", len(ingester.GetSkipDetails()))
	}

	// Add data
	db.result = &IngestResult{
		Ingested:    10,
		Skipped:     5,
		SkipDetails: map[SkipReason]int64{
			SkipReasonConflictManual: 2,
			SkipReasonConflictOlder:  3,
		},
	}

	rows := make([]ResolutionRow, 15)
	for i := 0; i < 15; i++ {
		rows[i] = ResolutionRow{
			Email:      fmt.Sprintf("user%d@example.com", i),
			Login:      fmt.Sprintf("user%d", i),
			Source:     SourceLive,
			ResolvedAt: now,
		}
	}

	err := ingester.IngestResolution(context.Background(), rows)
	if err != nil {
		t.Fatalf("batch failed: %v", err)
	}

	// Verify getter methods return expected values
	if got := ingester.GetProcessed(); got != 15 {
		t.Errorf("GetProcessed() = %d, want 15", got)
	}
	if got := ingester.GetIngested(); got != 10 {
		t.Errorf("GetIngested() = %d, want 10", got)
	}
	if got := ingester.GetSkipped(); got != 5 {
		t.Errorf("GetSkipped() = %d, want 5", got)
	}

	details := ingester.GetSkipDetails()
	if len(details) != 2 {
		t.Errorf("GetSkipDetails() returned %d entries, want 2", len(details))
	}
	if details[SkipReasonConflictManual] != 2 {
		t.Errorf("GetSkipDetails()[conflict_manual] = %d, want 2", details[SkipReasonConflictManual])
	}
	if details[SkipReasonConflictOlder] != 3 {
		t.Errorf("GetSkipDetails()[conflict_older] = %d, want 3", details[SkipReasonConflictOlder])
	}

	// Verify that returned map is the actual internal map (not a copy)
	// by modifying it and checking if changes are reflected
	details[SkipReasonValidation] = 99
	details2 := ingester.GetSkipDetails()
	if details2[SkipReasonValidation] != 99 {
		t.Error("GetSkipDetails() appears to return a copy instead of the actual map")
	}
}

// TestIngester_AllSkipReasons verifies all skip reason types are tracked correctly.
func TestIngester_AllSkipReasons(t *testing.T) {
	db := &mockDB{}
	ingester := NewIngester(db)

	now := time.Now().UTC()

	// Test all skip reason types
	db.result = &IngestResult{
		Ingested:    0,
		Skipped:     5,
		SkipDetails: map[SkipReason]int64{
			SkipReasonConflictManual: 1,
			SkipReasonConflictOlder:  1,
			SkipReasonValidation:     1,
			SkipReasonDatabase:       1,
			SkipReasonOther:          1,
		},
	}

	rows := []ResolutionRow{
		{Email: "user@example.com", Login: "user", Source: SourceLive, ResolvedAt: now},
	}

	err := ingester.IngestResolution(context.Background(), rows)
	if err != nil {
		t.Fatalf("batch failed: %v", err)
	}

	details := ingester.GetSkipDetails()

	// Verify all reasons are tracked
	expectedReasons := []SkipReason{
		SkipReasonConflictManual,
		SkipReasonConflictOlder,
		SkipReasonValidation,
		SkipReasonDatabase,
		SkipReasonOther,
	}

	for _, reason := range expectedReasons {
		if count, exists := details[reason]; !exists {
			t.Errorf("skip reason %q not found in SkipDetails", reason)
		} else if count != 1 {
			t.Errorf("skip reason %q has count %d, want 1", reason, count)
		}
	}

	// Verify total skipped
	if ingester.GetSkipped() != 5 {
		t.Errorf("GetSkipped() = %d, want 5", ingester.GetSkipped())
	}

	// Verify SkipDetails sum
	sum := int64(0)
	for _, count := range details {
		sum += count
	}
	if sum != 5 {
		t.Errorf("SkipDetails sum = %d, want 5", sum)
	}
}

// TestSkipReasonString verifies the String() method returns correct string representation.
func TestSkipReasonString(t *testing.T) {
	tests := []struct {
		reason  SkipReason
		wantStr string
	}{
		{SkipReasonConflictManual, "conflict_manual"},
		{SkipReasonConflictOlder, "conflict_older"},
		{SkipReasonValidation, "validation"},
		{SkipReasonDatabase, "database"},
		{SkipReasonOther, "other"},
	}

	for _, tt := range tests {
		t.Run(tt.wantStr, func(t *testing.T) {
			got := tt.reason.String()
			if got != tt.wantStr {
				t.Errorf("SkipReason.String() = %q, want %q", got, tt.wantStr)
			}
		})
	}
}
