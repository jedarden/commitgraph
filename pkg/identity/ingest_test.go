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

func (m *mockDB) IngestEmailResolution(ctx context.Context, rows []ResolutionRow) error {
	m.rowsReceived = rows
	if m.shouldError {
		return errors.New("test database error")
	}
	return nil
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
