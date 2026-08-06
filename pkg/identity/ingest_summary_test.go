package identity

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// TestIngester_GetSummary verifies the GetSummary method returns correct data structure.
func TestIngester_GetSummary(t *testing.T) {
	db := &mockDB{}
	ingester := NewIngester(db)

	// Test with no data
	summary := ingester.GetSummary()
	if summary == nil {
		t.Fatal("GetSummary returned nil")
	}

	// Verify all required fields exist
	requiredFields := []string{"processed", "ingested", "skipped", "skip_details"}
	for _, field := range requiredFields {
		if _, exists := summary[field]; !exists {
			t.Errorf("Missing required field in summary: %s", field)
		}
	}

	// Verify initial values are zero
	if summary["processed"].(int64) != 0 {
		t.Errorf("Expected initial processed=0, got %d", summary["processed"].(int64))
	}
	if summary["ingested"].(int64) != 0 {
		t.Errorf("Expected initial ingested=0, got %d", summary["ingested"].(int64))
	}
	if summary["skipped"].(int64) != 0 {
		t.Errorf("Expected initial skipped=0, got %d", summary["skipped"].(int64))
	}

	skipDetails := summary["skip_details"].(map[string]int64)
	if len(skipDetails) != 0 {
		t.Errorf("Expected empty skip_details, got %d entries", len(skipDetails))
	}
}

// TestIngester_GetSummary_WithData verifies GetSummary returns accurate counts after ingest.
func TestIngester_GetSummary_WithData(t *testing.T) {
	db := &mockDB{}
	ingester := NewIngester(db)

	now := time.Now().UTC()

	// First batch with mixed results
	db.result = &IngestResult{
		Ingested:    7,
		Skipped:     3,
		SkipDetails: map[SkipReason]int64{
			SkipReasonConflictManual: 2,
			SkipReasonConflictOlder:  1,
		},
	}

	rows1 := []ResolutionRow{
		{Email: "user1@example.com", Login: "user1", Source: SourceLive, ResolvedAt: now},
		{Email: "user2@example.com", Login: "user2", Source: SourceLive, ResolvedAt: now},
		{Email: "user3@example.com", Login: "user3", Source: SourceLive, ResolvedAt: now},
		{Email: "user4@example.com", Login: "user4", Source: SourceLive, ResolvedAt: now},
		{Email: "user5@example.com", Login: "user5", Source: SourceLive, ResolvedAt: now},
		{Email: "user6@example.com", Login: "user6", Source: SourceLive, ResolvedAt: now},
		{Email: "user7@example.com", Login: "user7", Source: SourceLive, ResolvedAt: now},
		{Email: "user8@example.com", Login: "user8", Source: SourceLive, ResolvedAt: now},
		{Email: "user9@example.com", Login: "user9", Source: SourceLive, ResolvedAt: now},
		{Email: "user10@example.com", Login: "user10", Source: SourceLive, ResolvedAt: now},
	}

	err := ingester.IngestResolution(nil, rows1)
	if err != nil {
		t.Fatalf("first batch failed: %v", err)
	}

	// Get summary and verify
	summary := ingester.GetSummary()

	if summary["processed"].(int64) != 10 {
		t.Errorf("Expected processed=10, got %d", summary["processed"].(int64))
	}
	if summary["ingested"].(int64) != 7 {
		t.Errorf("Expected ingested=7, got %d", summary["ingested"].(int64))
	}
	if summary["skipped"].(int64) != 3 {
		t.Errorf("Expected skipped=3, got %d", summary["skipped"].(int64))
	}

	skipDetails := summary["skip_details"].(map[string]int64)
	if len(skipDetails) != 2 {
		t.Errorf("Expected 2 skip detail entries, got %d", len(skipDetails))
	}
	if skipDetails["conflict_manual"] != 2 {
		t.Errorf("Expected conflict_manual=2, got %d", skipDetails["conflict_manual"])
	}
	if skipDetails["conflict_older"] != 1 {
		t.Errorf("Expected conflict_older=1, got %d", skipDetails["conflict_older"])
	}
}

// TestIngester_GetSummary_MultipleBatches verifies GetSummary accumulates across batches.
func TestIngester_GetSummary_MultipleBatches(t *testing.T) {
	db := &mockDB{}
	ingester := NewIngester(db)

	now := time.Now().UTC()

	batches := []struct {
		ingested int64
		skipped  int64
		reasons  map[SkipReason]int64
	}{
		{5, 0, map[SkipReason]int64{}},                                 // All ingested
		{2, 3, map[SkipReason]int64{SkipReasonConflictManual: 3}},      // Mixed
		{0, 4, map[SkipReason]int64{SkipReasonConflictOlder: 4}},       // All skipped
		{3, 2, map[SkipReason]int64{SkipReasonValidation: 2}},         // Mixed with validation
	}

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
				Email:      FormatEmail("batch", i, j),
				Login:      FormatLogin("user", j),
				Source:     SourceLive,
				ResolvedAt: now,
			}
		}

		err := ingester.IngestResolution(nil, rows)
		if err != nil {
			t.Fatalf("batch %d failed: %v", i, err)
		}
	}

	// Get final summary
	summary := ingester.GetSummary()

	// Expected totals: processed=19 (5+5+4+5), ingested=10 (5+2+0+3), skipped=9 (0+3+4+2)
	if summary["processed"].(int64) != 19 {
		t.Errorf("Expected processed=19, got %d", summary["processed"].(int64))
	}
	if summary["ingested"].(int64) != 10 {
		t.Errorf("Expected ingested=10, got %d", summary["ingested"].(int64))
	}
	if summary["skipped"].(int64) != 9 {
		t.Errorf("Expected skipped=9, got %d", summary["skipped"].(int64))
	}

	// Verify skip details accumulated correctly
	skipDetails := summary["skip_details"].(map[string]int64)
	if len(skipDetails) != 3 {
		t.Errorf("Expected 3 skip detail entries, got %d", len(skipDetails))
	}
	if skipDetails["conflict_manual"] != 3 {
		t.Errorf("Expected conflict_manual=3, got %d", skipDetails["conflict_manual"])
	}
	if skipDetails["conflict_older"] != 4 {
		t.Errorf("Expected conflict_older=4, got %d", skipDetails["conflict_older"])
	}
	if skipDetails["validation"] != 2 {
		t.Errorf("Expected validation=2, got %d", skipDetails["validation"])
	}

	// Verify invariant: processed = ingested + skipped
	processed := summary["processed"].(int64)
	ingested := summary["ingested"].(int64)
	skipped := summary["skipped"].(int64)
	if processed != ingested+skipped {
		t.Errorf("Invariant violated: processed=%d != ingested=%d + skipped=%d",
			processed, ingested, skipped)
	}
}

// TestIngester_GetSummary_Invariant verifies the Processed = Ingested + Skipped invariant.
func TestIngester_GetSummary_Invariant(t *testing.T) {
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

			rows := make([]ResolutionRow, tc.rowCount)
			for i := 0; i < tc.rowCount; i++ {
				rows[i] = ResolutionRow{
					Email:      FormatEmail("test", 0, i),
					Login:      FormatLogin("user", i),
					Source:     SourceLive,
					ResolvedAt: now,
				}
			}

			db.result = tc.result

			err := ingester.IngestResolution(nil, rows)
			if err != nil {
				t.Fatalf("batch failed: %v", err)
			}

			// Get summary and verify invariant
			summary := ingester.GetSummary()
			processed := summary["processed"].(int64)
			ingested := summary["ingested"].(int64)
			skipped := summary["skipped"].(int64)

			if processed != ingested+skipped {
				t.Errorf("Invariant violated: processed=%d, ingested=%d, skipped=%d, expected processed=%d",
					processed, ingested, skipped, ingested+skipped)
			}

			if processed != int64(tc.rowCount) {
				t.Errorf("processed count mismatch: got %d, expected %d (row count)",
					processed, tc.rowCount)
			}

			// Verify SkipDetails sum matches Skipped
			skipDetails := summary["skip_details"].(map[string]int64)
			skipDetailsSum := int64(0)
			for _, count := range skipDetails {
				skipDetailsSum += count
			}
			if skipDetailsSum != skipped {
				t.Errorf("SkipDetails sum mismatch: sum=%d, skipped=%d", skipDetailsSum, skipped)
			}
		})
	}
}

// TestIngester_GetSummary_JSONMarhalability verifies GetSummary output is JSON-marshalable.
func TestIngester_GetSummary_JSONMarshalability(t *testing.T) {
	db := &mockDB{}
	ingester := NewIngester(db)

	now := time.Now().UTC()

	// Set up some test data
	db.result = &IngestResult{
		Ingested:    15,
		Skipped:     5,
		SkipDetails: map[SkipReason]int64{
			SkipReasonConflictManual: 3,
			SkipReasonConflictOlder:  2,
		},
	}

	rows := make([]ResolutionRow, 20)
	for i := 0; i < 20; i++ {
		rows[i] = ResolutionRow{
			Email:      FormatEmail("json", 0, i),
			Login:      FormatLogin("user", i),
			Source:     SourceLive,
			ResolvedAt: now,
		}
	}

	err := ingester.IngestResolution(nil, rows)
	if err != nil {
		t.Fatalf("batch failed: %v", err)
	}

	// Get summary and marshal to JSON
	summary := ingester.GetSummary()
	jsonBytes, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal summary to JSON: %v", err)
	}

	// Verify we can unmarshal it back
	var unmarshaled map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v\nJSON was:\n%s", err, string(jsonBytes))
	}

	// Verify the data survived the round-trip
	if unmarshaled["processed"].(float64) != float64(20) {
		t.Errorf("Expected processed=20 after round-trip, got %v", unmarshaled["processed"])
	}
	if unmarshaled["ingested"].(float64) != float64(15) {
		t.Errorf("Expected ingested=15 after round-trip, got %v", unmarshaled["ingested"])
	}
	if unmarshaled["skipped"].(float64) != float64(5) {
		t.Errorf("Expected skipped=5 after round-trip, got %v", unmarshaled["skipped"])
	}

	// Verify skip_details structure
	skipDetails, ok := unmarshaled["skip_details"].(map[string]interface{})
	if !ok {
		t.Fatal("skip_details is not a map after unmarshaling")
	}

	if len(skipDetails) != 2 {
		t.Errorf("Expected 2 skip detail entries, got %d", len(skipDetails))
	}
	if skipDetails["conflict_manual"].(float64) != 3 {
		t.Errorf("Expected conflict_manual=3 after round-trip, got %v", skipDetails["conflict_manual"])
	}
	if skipDetails["conflict_older"].(float64) != 2 {
		t.Errorf("Expected conflict_older=2 after round-trip, got %v", skipDetails["conflict_older"])
	}
}

// TestIngester_GetSummary_AllSkipReasons verifies all skip reason types appear in summary.
func TestIngester_GetSummary_AllSkipReasons(t *testing.T) {
	db := &mockDB{}
	ingester := NewIngester(db)

	now := time.Now().UTC()

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

	err := ingester.IngestResolution(nil, rows)
	if err != nil {
		t.Fatalf("batch failed: %v", err)
	}

	summary := ingester.GetSummary()
	skipDetails := summary["skip_details"].(map[string]int64)

	expectedReasons := []SkipReason{
		SkipReasonConflictManual,
		SkipReasonConflictOlder,
		SkipReasonValidation,
		SkipReasonDatabase,
		SkipReasonOther,
	}

	for _, reason := range expectedReasons {
		count, exists := skipDetails[reason.String()]
		if !exists {
			t.Errorf("Skip reason %q not found in summary", reason.String())
		}
		if count != 1 {
			t.Errorf("Skip reason %q has count %d, want 1", reason.String(), count)
		}
	}

	// Verify total skipped
	if summary["skipped"].(int64) != 5 {
		t.Errorf("Expected skipped=5, got %d", summary["skipped"].(int64))
	}
}

// FormatEmail is a helper function to format email addresses for testing.
func FormatEmail(prefix string, batchIdx, rowIdx int) string {
	return fmt.Sprintf("%s%d_user%d@example.com", prefix, batchIdx, rowIdx)
}

// FormatLogin is a helper function to format login names for testing.
func FormatLogin(prefix string, idx int) string {
	return fmt.Sprintf("%s%d", prefix, idx)
}