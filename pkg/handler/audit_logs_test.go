// Package handler provides tests for the audit log HTTP handler.
package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/jedarden/commitgraph/pkg/service"
)

// TestAuditLogsHandler_SuccessfulQuery tests a successful audit log query
func TestAuditLogsHandler_SuccessfulQuery(t *testing.T) {
	// Setup: Create test database and handler
	db := setupTestDB(t)
	defer teardownTestDB(t, db)
	handler := NewAuditLogsHandler(db)

	// Create a test mux and register the handler
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Create a test request
	req := httptest.NewRequest("GET", "/api/audit-logs?repo_id=1&limit=10", nil)
	w := httptest.NewRecorder()

	// Serve the request
	mux.ServeHTTP(w, req)

	// Check response
	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Check Content-Type
 contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	// Parse response
	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Validate response structure
	if apiResp.Limit != 10 {
		t.Errorf("Expected limit 10, got %d", apiResp.Limit)
	}
	if apiResp.Offset != 0 {
		t.Errorf("Expected offset 0, got %d", apiResp.Offset)
	}
}

// TestAuditLogsHandler_DateRangeParsing tests date range parsing
func TestAuditLogsHandler_DateRangeParsing(t *testing.T) {
	db := setupTestDB(t)
	defer teardownTestDB(t, db)
	handler := NewAuditLogsHandler(db)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	tests := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{
			name:       "valid date range",
			query:      "/api/audit-logs?repo_id=1&start_date=2024-01-01&end_date=2024-12-31",
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid date format",
			query:      "/api/audit-logs?repo_id=1&start_date=2024/01/01",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid date - February 30",
			query:      "/api/audit-logs?repo_id=1&start_date=2024-02-30",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "start date after end date",
			query:      "/api/audit-logs?repo_id=1&start_date=2024-12-31&end_date=2024-01-01",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.query, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			resp := w.Result()
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, resp.StatusCode)
			}
		})
	}
}

// TestAuditLogsHandler_ParameterValidation tests parameter validation
func TestAuditLogsHandler_ParameterValidation(t *testing.T) {
	db := setupTestDB(t)
	defer teardownTestDB(t, db)
	handler := NewAuditLogsHandler(db)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantCode   string // error code
	}{
		{
			name:       "invalid limit - negative",
			query:      "/api/audit-logs?repo_id=1&limit=-1",
			wantStatus: http.StatusBadRequest,
			wantCode:   "INVALID_PARAMETER",
		},
		{
			name:       "invalid limit - too large",
			query:      "/api/audit-logs?repo_id=1&limit=1001",
			wantStatus: http.StatusBadRequest,
			wantCode:   "INVALID_PARAMETER",
		},
		{
			name:       "invalid offset - negative",
			query:      "/api/audit-logs?repo_id=1&offset=-1",
			wantStatus: http.StatusBadRequest,
			wantCode:   "INVALID_PARAMETER",
		},
		{
			name:       "invalid event_type",
			query:      "/api/audit-logs?repo_id=1&event_type=invalid",
			wantStatus: http.StatusBadRequest,
			wantCode:   "INVALID_PARAMETER",
		},
		{
			name:       "valid event_type - exclude",
			query:      "/api/audit-logs?repo_id=1&event_type=exclude",
			wantStatus: http.StatusOK,
		},
		{
			name:       "valid event_type - unexclude",
			query:      "/api/audit-logs?repo_id=1&event_type=unexclude",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.query, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			resp := w.Result()
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, resp.StatusCode)
			}

			if tt.wantCode != "" {
				var errResp errorResponse
				if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
					t.Fatalf("Failed to decode error response: %v", err)
				}
				if errResp.Error.Code != tt.wantCode {
					t.Errorf("Expected error code %s, got %s", tt.wantCode, errResp.Error.Code)
				}
			}
		})
	}
}

// TestAuditLogsHandler_ResponseFormat tests the response format
func TestAuditLogsHandler_ResponseFormat(t *testing.T) {
	db := setupTestDB(t)
	defer teardownTestDB(t, db)

	// Insert test data
	insertTestAuditData(t, db)

	handler := NewAuditLogsHandler(db)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/api/audit-logs?repo_id=1", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Check response headers
	totalCount := resp.Header.Get("X-Total-Count")
	if totalCount == "" {
		t.Error("Missing X-Total-Count header")
	}
	limit := resp.Header.Get("X-Limit")
	if limit == "" {
		t.Error("Missing X-Limit header")
	}
	offset := resp.Header.Get("X-Offset")
	if offset == "" {
		t.Error("Missing X-Offset header")
	}

	// Validate response structure
	if len(apiResp.Records) == 0 {
		t.Log("No records found (database may be empty)")
		return
	}

	// Check first record structure
	rec := apiResp.Records[0]
	if rec.ID == 0 {
		t.Error("Record ID should not be zero")
	}
	if rec.RepoID == 0 {
		t.Error("RepoID should not be zero")
	}
	if rec.Actor == "" {
		t.Error("Actor should not be empty")
	}
	if rec.Timestamp == "" {
		t.Error("Timestamp should not be empty")
	}
	if rec.EventType == "" {
		t.Error("EventType should not be empty")
	}
}

// TestAuditLogsHandler_PaginationTests tests pagination behavior
func TestAuditLogsHandler_PaginationTests(t *testing.T) {
	db := setupTestDB(t)
	defer teardownTestDB(t, db)

	// Insert test data
	insertTestAuditData(t, db)

	handler := NewAuditLogsHandler(db)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	tests := []struct {
		name         string
		query        string
		expectedLimit int
		expectedOffset int
	}{
		{
			name:         "default pagination",
			query:        "/api/audit-logs?repo_id=1",
			expectedLimit: 100,
			expectedOffset: 0,
		},
		{
			name:         "custom limit",
			query:        "/api/audit-logs?repo_id=1&limit=5",
			expectedLimit: 5,
			expectedOffset: 0,
		},
		{
			name:         "custom offset",
			query:        "/api/audit-logs?repo_id=1&limit=10&offset=5",
			expectedLimit: 10,
			expectedOffset: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.query, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			resp := w.Result()
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
				return
			}

			var apiResp apiResponse
			if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if apiResp.Limit != tt.expectedLimit {
				t.Errorf("Expected limit %d, got %d", tt.expectedLimit, apiResp.Limit)
			}
			if apiResp.Offset != tt.expectedOffset {
				t.Errorf("Expected offset %d, got %d", tt.expectedOffset, apiResp.Offset)
			}
		})
	}
}

// Helper functions

// setupTestDB creates a test database connection
func setupTestDB(t *testing.T) *sql.DB {
	// Use environment variables or default to localhost
	connStr := "host=localhost port=5432 dbname=commitgraph_test user=postgres password=postgres sslmode=disable"

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Skipf("Skipping test: Cannot connect to test database: %v", err)
		return nil
	}

	if err := db.Ping(); err != nil {
		t.Skipf("Skipping test: Test database not available: %v", err)
		return nil
	}

	// Create test schema
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS exclusion_audit_log (
			id BIGSERIAL PRIMARY KEY,
			repo_id BIGINT NOT NULL,
			actor VARCHAR(255) NOT NULL,
			timestamp TIMESTAMP NOT NULL DEFAULT NOW(),
			event_type VARCHAR(20) NOT NULL CHECK (event_type IN ('exclude', 'unexclude')),
			old_excluded_at TIMESTAMP,
			old_excluded_reason TEXT,
			new_excluded_at TIMESTAMP,
			new_excluded_reason TEXT
		);

		CREATE INDEX IF NOT EXISTS idx_exclusion_audit_log_repo_id ON exclusion_audit_log(repo_id);
		CREATE INDEX IF NOT EXISTS idx_exclusion_audit_log_timestamp ON exclusion_audit_log(timestamp DESC);
	`)
	if err != nil {
		t.Fatalf("Failed to create test schema: %v", err)
	}

	// Clear test data
	_, _ = db.Exec("DELETE FROM exclusion_audit_log")

	return db
}

// teardownTestDB cleans up the test database
func teardownTestDB(t *testing.T, db *sql.DB) {
	if db != nil {
		_, _ = db.Exec("DROP TABLE IF EXISTS exclusion_audit_log")
		db.Close()
	}
}

// insertTestAuditData inserts test audit log records
func insertTestAuditData(t *testing.T, db *sql.DB) {
	ctx := context.Background()

	// Insert test records
	now := time.Now().UTC()
	testRecords := []service.AuditLogRecord{
		{
			RepoID:             1,
			Actor:              "admin@example.com",
			Timestamp:          now.Add(-2 * time.Hour),
			EventType:          "exclude",
			OldExcludedAt:      nil,
			OldExcludedReason:  nil,
			NewExcludedAt:      &now,
			NewExcludedReason:  stringPtr("Spam repository"),
		},
		{
			RepoID:             1,
			Actor:              "admin@example.com",
			Timestamp:          now.Add(-time.Hour),
			EventType:          "unexclude",
			OldExcludedAt:      &now,
			OldExcludedReason:  stringPtr("Spam repository"),
			NewExcludedAt:      nil,
			NewExcludedReason:  nil,
		},
		{
			RepoID:             2,
			Actor:              "system@example.com",
			Timestamp:          now,
			EventType:          "exclude",
			OldExcludedAt:      nil,
			OldExcludedReason:  nil,
			NewExcludedAt:      &now,
			NewExcludedReason:  stringPtr("Automated exclusion"),
		},
	}

	for _, rec := range testRecords {
		_, err := db.ExecContext(ctx, `
			INSERT INTO exclusion_audit_log
			(repo_id, actor, timestamp, event_type, old_excluded_at, old_excluded_reason, new_excluded_at, new_excluded_reason)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, rec.RepoID, rec.Actor, rec.Timestamp, rec.EventType, rec.OldExcludedAt, rec.OldExcludedReason, rec.NewExcludedAt, rec.NewExcludedReason)
		if err != nil {
			t.Fatalf("Failed to insert test data: %v", err)
		}
	}
}

func stringPtr(s string) *string {
	return &s
}
