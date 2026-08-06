package ingestlog

import (
	"bytes"
	"encoding/json"
	"log"
	"strings"
	"testing"
	"time"
)

// TestLogStatsJSON verifies JSON summary logging produces valid, parseable output.
func TestLogStatsJSON(t *testing.T) {
	// Create a logger with custom output to capture the JSON
	var output bytes.Buffer
	customLogger := NewLoggerWithOutput(&log.Logger{})
	customLogger.output.SetOutput(&output)

	// Set some test stats
	customLogger.stats.TotalProcessed = 100
	customLogger.stats.TotalSkipped = 20
	customLogger.stats.TotalIngested = 80
	customLogger.stats.TotalRetries = 5
	customLogger.stats.TotalFailures = 2
	customLogger.stats.StartTime = time.Now().UTC().Add(-10 * time.Second)
	customLogger.stats.LastUpdateTime = time.Now().UTC()

	// Log stats as JSON
	err := customLogger.LogStatsJSON("Test Summary")
	if err != nil {
		t.Fatalf("LogStatsJSON failed: %v", err)
	}

	// Verify the output contains valid JSON
	outputStr := output.String()

	// Extract JSON from between the title lines
	lines := strings.Split(outputStr, "\n")
	var jsonLines []string
	inJSON := false
	for _, line := range lines {
		if strings.Contains(line, "===") {
			if inJSON {
				break // End of JSON section
			}
			inJSON = true
			continue
		}
		if inJSON && strings.TrimSpace(line) != "" {
			jsonLines = append(jsonLines, line)
		}
	}

	jsonStr := strings.Join(jsonLines, "\n")

	// Parse the JSON
	var summary map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &summary); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nJSON was:\n%s", err, jsonStr)
	}

	// Verify required fields exist
	requiredFields := []string{"timestamp", "title", "records", "percentages", "performance", "retries"}
	for _, field := range requiredFields {
		if _, exists := summary[field]; !exists {
			t.Errorf("Missing required field in JSON: %s", field)
		}
	}

	// Verify title
	if summary["title"] != "Test Summary" {
		t.Errorf("Expected title 'Test Summary', got %v", summary["title"])
	}

	// Verify records section
	records, ok := summary["records"].(map[string]interface{})
	if !ok {
		t.Fatal("records field is not a map")
	}

	if records["processed"].(float64) != float64(100) {
		t.Errorf("Expected processed=100, got %v", records["processed"])
	}
	if records["skipped"].(float64) != float64(20) {
		t.Errorf("Expected skipped=20, got %v", records["skipped"])
	}
	if records["ingested"].(float64) != float64(80) {
		t.Errorf("Expected ingested=80, got %v", records["ingested"])
	}

	// Verify percentages section
	percentages, ok := summary["percentages"].(map[string]interface{})
	if !ok {
		t.Fatal("percentages field is not a map")
	}

	skippedPercent := percentages["skipped_percent"].(float64)
	if skippedPercent != 20.0 {
		t.Errorf("Expected skipped_percent=20.0, got %v", skippedPercent)
	}

	ingestedPercent := percentages["ingested_percent"].(float64)
	if ingestedPercent != 80.0 {
		t.Errorf("Expected ingested_percent=80.0, got %v", ingestedPercent)
	}

	// Verify performance section
	performance, ok := summary["performance"].(map[string]interface{})
	if !ok {
		t.Fatal("performance field is not a map")
	}

	elapsedSeconds := performance["elapsed_seconds"].(float64)
	if elapsedSeconds <= 0 {
		t.Errorf("Expected elapsed_seconds > 0, got %v", elapsedSeconds)
	}

	ratePerSec := performance["rate_per_sec"].(float64)
	if ratePerSec <= 0 {
		t.Errorf("Expected rate_per_sec > 0, got %v", ratePerSec)
	}

	// Verify retries section
	retries, ok := summary["retries"].(map[string]interface{})
	if !ok {
		t.Fatal("retries field is not a map")
	}

	if retries["total_attempts"].(float64) != float64(5) {
		t.Errorf("Expected total_attempts=5, got %v", retries["total_attempts"])
	}
	if retries["final_failures"].(float64) != float64(2) {
		t.Errorf("Expected final_failures=2, got %v", retries["final_failures"])
	}
}

// TestLogStatsJSON_ZeroValues verifies JSON logging handles zero values correctly.
func TestLogStatsJSON_ZeroValues(t *testing.T) {
	var output bytes.Buffer
	customLogger := NewLoggerWithOutput(&log.Logger{})
	customLogger.output.SetOutput(&output)

	// Keep stats at zero values (default)

	err := customLogger.LogStatsJSON("Zero Values Test")
	if err != nil {
		t.Fatalf("LogStatsJSON with zero values failed: %v", err)
	}

	outputStr := output.String()

	// Extract JSON from between the title lines
	lines := strings.Split(outputStr, "\n")
	var jsonLines []string
	inJSON := false
	for _, line := range lines {
		if strings.Contains(line, "===") {
			if inJSON {
				break // End of JSON section
			}
			inJSON = true
			continue
		}
		if inJSON && strings.TrimSpace(line) != "" {
			jsonLines = append(jsonLines, line)
		}
	}

	jsonStr := strings.Join(jsonLines, "\n")

	var summary map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &summary); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	records, _ := summary["records"].(map[string]interface{})
	if records["processed"].(float64) != 0 {
		t.Errorf("Expected processed=0, got %v", records["processed"])
	}

	percentages, _ := summary["percentages"].(map[string]interface{})
	// With zero processed, percentages should be 0 (not NaN due to safe division)
	if percentages["skipped_percent"].(float64) != 0 {
		t.Errorf("Expected skipped_percent=0 with zero processed, got %v", percentages["skipped_percent"])
	}
}

// TestLogStatsJSON_AllIngested verifies JSON summary when all records are ingested.
func TestLogStatsJSON_AllIngested(t *testing.T) {
	var output bytes.Buffer
	customLogger := NewLoggerWithOutput(&log.Logger{})
	customLogger.output.SetOutput(&output)

	customLogger.stats.TotalProcessed = 50
	customLogger.stats.TotalIngested = 50
	customLogger.stats.TotalSkipped = 0

	err := customLogger.LogStatsJSON("All Ingested Test")
	if err != nil {
		t.Fatalf("LogStatsJSON failed: %v", err)
	}

	outputStr := output.String()

	// Extract JSON from between the title lines
	lines := strings.Split(outputStr, "\n")
	var jsonLines []string
	inJSON := false
	for _, line := range lines {
		if strings.Contains(line, "===") {
			if inJSON {
				break // End of JSON section
			}
			inJSON = true
			continue
		}
		if inJSON && strings.TrimSpace(line) != "" {
			jsonLines = append(jsonLines, line)
		}
	}

	jsonStr := strings.Join(jsonLines, "\n")

	var summary map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &summary); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	records, _ := summary["records"].(map[string]interface{})
	if records["processed"].(float64) != 50 {
		t.Errorf("Expected processed=50, got %v", records["processed"])
	}
	if records["ingested"].(float64) != 50 {
		t.Errorf("Expected ingested=50, got %v", records["ingested"])
	}
	if records["skipped"].(float64) != 0 {
		t.Errorf("Expected skipped=0, got %v", records["skipped"])
	}

	percentages, _ := summary["percentages"].(map[string]interface{})
	if percentages["ingested_percent"].(float64) != 100.0 {
		t.Errorf("Expected ingested_percent=100.0, got %v", percentages["ingested_percent"])
	}
	if percentages["skipped_percent"].(float64) != 0 {
		t.Errorf("Expected skipped_percent=0, got %v", percentages["skipped_percent"])
	}
}

// TestLogStatsJSON_AllSkipped verifies JSON summary when all records are skipped.
func TestLogStatsJSON_AllSkipped(t *testing.T) {
	var output bytes.Buffer
	customLogger := NewLoggerWithOutput(&log.Logger{})
	customLogger.output.SetOutput(&output)

	customLogger.stats.TotalProcessed = 30
	customLogger.stats.TotalIngested = 0
	customLogger.stats.TotalSkipped = 30

	err := customLogger.LogStatsJSON("All Skipped Test")
	if err != nil {
		t.Fatalf("LogStatsJSON failed: %v", err)
	}

	outputStr := output.String()

	// Extract JSON from between the title lines
	lines := strings.Split(outputStr, "\n")
	var jsonLines []string
	inJSON := false
	for _, line := range lines {
		if strings.Contains(line, "===") {
			if inJSON {
				break // End of JSON section
			}
			inJSON = true
			continue
		}
		if inJSON && strings.TrimSpace(line) != "" {
			jsonLines = append(jsonLines, line)
		}
	}

	jsonStr := strings.Join(jsonLines, "\n")

	var summary map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &summary); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	records, _ := summary["records"].(map[string]interface{})
	if records["processed"].(float64) != 30 {
		t.Errorf("Expected processed=30, got %v", records["processed"])
	}
	if records["ingested"].(float64) != 0 {
		t.Errorf("Expected ingested=0, got %v", records["ingested"])
	}
	if records["skipped"].(float64) != 30 {
		t.Errorf("Expected skipped=30, got %v", records["skipped"])
	}

	percentages, _ := summary["percentages"].(map[string]interface{})
	if percentages["ingested_percent"].(float64) != 0 {
		t.Errorf("Expected ingested_percent=0, got %v", percentages["ingested_percent"])
	}
	if percentages["skipped_percent"].(float64) != 100.0 {
		t.Errorf("Expected skipped_percent=100.0, got %v", percentages["skipped_percent"])
	}
}

// TestLogStatsJSON_Mixed verifies JSON summary with mixed ingest and skip.
func TestLogStatsJSON_Mixed(t *testing.T) {
	var output bytes.Buffer
	customLogger := NewLoggerWithOutput(&log.Logger{})
	customLogger.output.SetOutput(&output)

	customLogger.stats.TotalProcessed = 1000
	customLogger.stats.TotalIngested = 750
	customLogger.stats.TotalSkipped = 250
	customLogger.stats.TotalRetries = 10
	customLogger.stats.TotalFailures = 3

	err := customLogger.LogStatsJSON("Mixed Test")
	if err != nil {
		t.Fatalf("LogStatsJSON failed: %v", err)
	}

	outputStr := output.String()

	// Extract JSON from between the title lines
	lines := strings.Split(outputStr, "\n")
	var jsonLines []string
	inJSON := false
	for _, line := range lines {
		if strings.Contains(line, "===") {
			if inJSON {
				break // End of JSON section
			}
			inJSON = true
			continue
		}
		if inJSON && strings.TrimSpace(line) != "" {
			jsonLines = append(jsonLines, line)
		}
	}

	jsonStr := strings.Join(jsonLines, "\n")

	var summary map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &summary); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Verify the invariant: processed = ingested + skipped
	records, _ := summary["records"].(map[string]interface{})
	processed := int64(records["processed"].(float64))
	ingested := int64(records["ingested"].(float64))
	skipped := int64(records["skipped"].(float64))

	if processed != ingested+skipped {
		t.Errorf("Invariant violated: processed=%d != ingested=%d + skipped=%d",
			processed, ingested, skipped)
	}

	percentages, _ := summary["percentages"].(map[string]interface{})
	ingestedPercent := percentages["ingested_percent"].(float64)
	skippedPercent := percentages["skipped_percent"].(float64)

	// Verify percentages sum to 100 (within floating point tolerance)
	totalPercent := ingestedPercent + skippedPercent
	if totalPercent < 99.9 || totalPercent > 100.1 {
		t.Errorf("Percentages don't sum to ~100: ingested=%.2f + skipped=%.2f = %.2f",
			ingestedPercent, skippedPercent, totalPercent)
	}

	// Verify expected values
	if ingestedPercent != 75.0 {
		t.Errorf("Expected ingested_percent=75.0, got %v", ingestedPercent)
	}
	if skippedPercent != 25.0 {
		t.Errorf("Expected skipped_percent=25.0, got %v", skippedPercent)
	}
}

// TestLogStatsJSON_JSONValidity verifies the JSON output is valid and parseable.
func TestLogStatsJSON_JSONValidity(t *testing.T) {
	var output bytes.Buffer
	customLogger := NewLoggerWithOutput(&log.Logger{})
	customLogger.output.SetOutput(&output)

	customLogger.stats.TotalProcessed = 100
	customLogger.stats.TotalSkipped = 30
	customLogger.stats.TotalIngested = 70
	customLogger.stats.TotalRetries = 5
	customLogger.stats.TotalFailures = 1

	err := customLogger.LogStatsJSON("Validity Test")
	if err != nil {
		t.Fatalf("LogStatsJSON failed: %v", err)
	}

	outputStr := output.String()

	// Extract JSON from between the title lines
	lines := strings.Split(outputStr, "\n")
	var jsonLines []string
	inJSON := false
	for _, line := range lines {
		if strings.Contains(line, "===") {
			if inJSON {
				break // End of JSON section
			}
			inJSON = true
			continue
		}
		if inJSON && strings.TrimSpace(line) != "" {
			jsonLines = append(jsonLines, line)
		}
	}

	jsonStr := strings.Join(jsonLines, "\n")

	// Verify it's valid JSON by checking we can unmarshal it
	var rawData map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &rawData); err != nil {
		t.Fatalf("Output is not valid JSON: %v\nOutput was:\n%s", err, jsonStr)
	}

	// Verify it's also valid when marshaled again (round-trip test)
 remarshaled, err := json.Marshal(rawData)
	if err != nil {
		t.Fatalf("Failed to remarshal JSON: %v", err)
	}

	if len(remarshaled) == 0 {
		t.Error("Remarshaled JSON is empty")
	}

	// Verify we can unmarshal the remarshaled data
	var remarshaledData map[string]interface{}
	if err := json.Unmarshal(remarshaled, &remarshaledData); err != nil {
		t.Fatalf("Failed to unmarshal remarshaled JSON: %v", err)
	}

	// Verify the data survived the round-trip
	if remarshaledData["title"] != rawData["title"] {
		t.Error("Title changed during round-trip")
	}
}
