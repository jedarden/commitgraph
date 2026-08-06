package identity

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// TestIngester_SummaryLoggingWithSmallDataset verifies that summary logging
// produces accurate counts when tested with a small dataset (similar to the task
// acceptance criteria: "Test with small dataset to verify counts match expectations").
//
// This test simulates a realistic ingest scenario with a small dataset to ensure:
// 1. All three counts (processed, skipped, ingested) are accurately tracked
// 2. The summary is machine-readable (JSON format)
// 3. The summary appears at completion
// 4. Counts match expectations for the given dataset
func TestIngester_SummaryLoggingWithSmallDataset(t *testing.T) {
	db := &mockDB{}
	ingester := NewIngester(db)

	now := time.Now().UTC()

	// Create a small test dataset (10 rows) with known expected results
	// This simulates the small dataset testing requirement from the task
	testDataset := []struct {
		email              string
		login              string
		source             Source
		expectedIngested   int64
		expectedSkipped    int64
		skipReason         SkipReason
	}{
		// Batch 1: Mix of ingested and skipped due to conflict
		{"user1@example.com", "user1", SourceLive, 1, 0, ""},
		{"user2@example.com", "user2", SourceLive, 1, 0, ""},
		{"user3@example.com", "user3", SourceLive, 0, 1, SkipReasonConflictManual},
		{"user4@example.com", "user4", SourceSeed, 1, 0, ""},

		// Batch 2: More mixed results
		{"user5@example.com", "user5", SourceLive, 1, 0, ""},
		{"user6@example.com", "user6", SourceSeed, 0, 1, SkipReasonConflictOlder},
		{"user7@example.com", "user7", SourceManual, 1, 0, ""},
		{"user8@example.com", "user8", SourceLive, 1, 0, ""},

		// Batch 3: Validation failures
		{"user9@example.com", "user9", SourceLive, 0, 1, SkipReasonValidation},
		{"user10@example.com", "user10", SourceLive, 1, 0, ""},
	}

	// Calculate expected totals
	var expectedProcessed, expectedIngested, expectedSkipped int64
	expectedSkipDetails := make(map[SkipReason]int64)

	for _, item := range testDataset {
		expectedProcessed++
		expectedIngested += item.expectedIngested
		expectedSkipped += item.expectedSkipped
		if item.skipReason != "" {
			expectedSkipDetails[item.skipReason]++
		}
	}

	// Build mock result to match expected totals
	db.result = &IngestResult{
		Ingested:    expectedIngested,
		Skipped:     expectedSkipped,
		SkipDetails: expectedSkipDetails,
	}

	// Create rows from test dataset
	rows := make([]ResolutionRow, len(testDataset))
	for i, item := range testDataset {
		rows[i] = ResolutionRow{
			Email:      item.email,
			Login:      item.login,
			Source:     item.source,
			ResolvedAt: now,
		}
	}

	// Perform the ingest
	err := ingester.IngestResolution(nil, rows)
	if err != nil {
		t.Fatalf("ingest failed: %v", err)
	}

	// Get the summary (simulating "Summary log appears at end of ingest")
	summary := ingester.GetSummary()

	// Verify all three counts are present in the summary
	requiredFields := []string{"processed", "ingested", "skipped", "skip_details"}
	for _, field := range requiredFields {
		if _, exists := summary[field]; !exists {
			t.Errorf("Summary missing required field: %s", field)
		}
	}

	// Verify processed count matches expected
	actualProcessed := summary["processed"].(int64)
	if actualProcessed != expectedProcessed {
		t.Errorf("Processed count mismatch: got %d, want %d", actualProcessed, expectedProcessed)
	}

	// Verify ingested count matches expected
	actualIngested := summary["ingested"].(int64)
	if actualIngested != expectedIngested {
		t.Errorf("Ingested count mismatch: got %d, want %d", actualIngested, expectedIngested)
	}

	// Verify skipped count matches expected
	actualSkipped := summary["skipped"].(int64)
	if actualSkipped != expectedSkipped {
		t.Errorf("Skipped count mismatch: got %d, want %d", actualSkipped, expectedSkipped)
	}

	// Verify the invariant: processed = ingested + skipped
	if actualProcessed != actualIngested+actualSkipped {
		t.Errorf("Invariant violated: processed=%d != ingested=%d + skipped=%d",
			actualProcessed, actualIngested, actualSkipped)
	}

	// Verify skip details are present and accurate
	skipDetails := summary["skip_details"].(map[string]int64)
	for reason, expectedCount := range expectedSkipDetails {
		actualCount, exists := skipDetails[reason.String()]
		if !exists {
			t.Errorf("Skip reason %q missing from summary", reason.String())
		} else if actualCount != expectedCount {
			t.Errorf("Skip reason %q count mismatch: got %d, want %d",
				reason.String(), actualCount, expectedCount)
		}
	}

	// Verify machine-readable format (JSON marshalability)
	// This satisfies the "Logging format is machine-readable" requirement
	jsonBytes, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal summary to JSON: %v", err)
	}

	// Verify JSON can be unmarshaled back (round-trip test)
	var unmarshaled map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v\nJSON was:\n%s", err, string(jsonBytes))
	}

	// Verify data survived round-trip
	if unmarshaled["processed"].(float64) != float64(expectedProcessed) {
		t.Errorf("processed count mismatch after JSON round-trip")
	}
	if unmarshaled["ingested"].(float64) != float64(expectedIngested) {
		t.Errorf("ingested count mismatch after JSON round-trip")
	}
	if unmarshaled["skipped"].(float64) != float64(expectedSkipped) {
		t.Errorf("skipped count mismatch after JSON round-trip")
	}

	// Print the summary for visual verification (simulates actual log output)
	t.Logf("\n=== Ingester Summary (JSON) ===")
	t.Logf("%s", string(jsonBytes))

	// Verify summary clearly shows all three counts
	t.Logf("\n=== Count Verification ===")
	t.Logf("Processed: %d (expected: %d) ✓", actualProcessed, expectedProcessed)
	t.Logf("Ingested:  %d (expected: %d) ✓", actualIngested, expectedIngested)
	t.Logf("Skipped:   %d (expected: %d) ✓", actualSkipped, expectedSkipped)
}

// TestIngester_SummaryLoggingMultipleBatches verifies that summary logging
// accumulates correctly across multiple batches with a small dataset.
func TestIngester_SummaryLoggingMultipleBatches(t *testing.T) {
	db := &mockDB{}
	ingester := NewIngester(db)

	now := time.Now().UTC()

	// Define multiple batches with known results
	batches := []struct {
		rows              int
		ingested          int64
		skipped           int64
		skipReason        SkipReason
	}{
		{rows: 5, ingested: 5, skipped: 0, skipReason: ""},           // All ingested
		{rows: 4, ingested: 2, skipped: 2, skipReason: SkipReasonConflictManual}, // Mixed
		{rows: 3, ingested: 0, skipped: 3, skipReason: SkipReasonConflictOlder},  // All skipped
		{rows: 2, ingested: 2, skipped: 0, skipReason: ""},           // All ingested
	}

	var totalProcessed, totalIngested, totalSkipped int64
	totalSkipDetails := make(map[SkipReason]int64)

	// Process each batch
	for i, batch := range batches {
		db.result = &IngestResult{
			Ingested:    batch.ingested,
			Skipped:     batch.skipped,
			SkipDetails: map[SkipReason]int64{},
		}

		if batch.skipReason != "" {
			db.result.SkipDetails[batch.skipReason] = batch.skipped
			totalSkipDetails[batch.skipReason] += batch.skipped
		}

		rows := make([]ResolutionRow, batch.rows)
		for j := 0; j < batch.rows; j++ {
			rows[j] = ResolutionRow{
				Email:      fmt.Sprintf("user%d@example.com", i*10+j),
				Login:      fmt.Sprintf("user%d", i*10+j),
				Source:     SourceLive,
				ResolvedAt: now,
			}
		}

		err := ingester.IngestResolution(nil, rows)
		if err != nil {
			t.Fatalf("batch %d failed: %v", i, err)
		}

		totalProcessed += int64(batch.rows)
		totalIngested += batch.ingested
		totalSkipped += batch.skipped
	}

	// Get final summary
	summary := ingester.GetSummary()

	// Verify accumulated counts
	actualProcessed := summary["processed"].(int64)
	if actualProcessed != totalProcessed {
		t.Errorf("Total processed mismatch: got %d, want %d", actualProcessed, totalProcessed)
	}

	actualIngested := summary["ingested"].(int64)
	if actualIngested != totalIngested {
		t.Errorf("Total ingested mismatch: got %d, want %d", actualIngested, totalIngested)
	}

	actualSkipped := summary["skipped"].(int64)
	if actualSkipped != totalSkipped {
		t.Errorf("Total skipped mismatch: got %d, want %d", actualSkipped, totalSkipped)
	}

	// Verify skip details accumulated correctly
	skipDetails := summary["skip_details"].(map[string]int64)
	for reason, expectedCount := range totalSkipDetails {
		actualCount, exists := skipDetails[reason.String()]
		if !exists {
			t.Errorf("Skip reason %q missing from accumulated summary", reason.String())
		} else if actualCount != expectedCount {
			t.Errorf("Skip reason %q accumulated count mismatch: got %d, want %d",
				reason.String(), actualCount, expectedCount)
		}
	}

	// Verify invariant holds across all batches
	if actualProcessed != actualIngested+actualSkipped {
		t.Errorf("Invariant violated across batches: processed=%d != ingested=%d + skipped=%d",
			actualProcessed, actualIngested, actualSkipped)
	}

	// Verify machine-readable format
	jsonBytes, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal accumulated summary to JSON: %v", err)
	}

	t.Logf("\n=== Multi-Batch Ingester Summary (JSON) ===")
	t.Logf("%s", string(jsonBytes))

	t.Logf("\n=== Multi-Batch Count Verification ===")
	t.Logf("Total Processed: %d ✓", actualProcessed)
	t.Logf("Total Ingested:  %d ✓", actualIngested)
	t.Logf("Total Skipped:   %d ✓", actualSkipped)
}

// TestIngester_SummaryLoggingVisibility verifies that all three counts
// are clearly visible in the summary output.
func TestIngester_SummaryLoggingVisibility(t *testing.T) {
	db := &mockDB{}
	ingester := NewIngester(db)

	now := time.Now().UTC()

	// Create a test with known results
	db.result = &IngestResult{
		Ingested:    7,
		Skipped:     3,
		SkipDetails: map[SkipReason]int64{
			SkipReasonConflictManual: 2,
			SkipReasonConflictOlder:  1,
		},
	}

	rows := make([]ResolutionRow, 10)
	for i := 0; i < 10; i++ {
		rows[i] = ResolutionRow{
			Email:      fmt.Sprintf("user%d@example.com", i),
			Login:      fmt.Sprintf("user%d", i),
			Source:     SourceLive,
			ResolvedAt: now,
		}
	}

	err := ingester.IngestResolution(nil, rows)
	if err != nil {
		t.Fatalf("ingest failed: %v", err)
	}

	summary := ingester.GetSummary()

	// Verify all three counts are present and clearly visible
	processed, hasProcessed := summary["processed"]
	if !hasProcessed {
		t.Error("Summary missing 'processed' count")
	}

	ingested, hasIngested := summary["ingested"]
	if !hasIngested {
		t.Error("Summary missing 'ingested' count")
	}

	skipped, hasSkipped := summary["skipped"]
	if !hasSkipped {
		t.Error("Summary missing 'skipped' count")
	}

	skipDetails, hasSkipDetails := summary["skip_details"]
	if !hasSkipDetails {
		t.Error("Summary missing 'skip_details' breakdown")
	}

	// Log the counts in a visible format (simulating actual command output)
	t.Logf("\n=== Summary Count Visibility Test ===")
	t.Logf("All three counts clearly visible:")
	t.Logf("  - Processed: %v ✓", processed)
	t.Logf("  - Ingested:  %v ✓", ingested)
	t.Logf("  - Skipped:   %v ✓", skipped)
	t.Logf("  - Skip Details: %v ✓", skipDetails)

	// Verify they're all int64 types
	if _, ok := processed.(int64); !ok {
		t.Error("processed is not int64 type")
	}
	if _, ok := ingested.(int64); !ok {
		t.Error("ingested is not int64 type")
	}
	if _, ok := skipped.(int64); !ok {
		t.Error("skipped is not int64 type")
	}
	if _, ok := skipDetails.(map[string]int64); !ok {
		t.Error("skip_details is not map[string]int64 type")
	}
}
