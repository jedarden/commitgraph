// Package handler provides HTTP handlers for the audit log query API.
package handler

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/jedarden/commitgraph/pkg/service"
)

// AuditLogsHandler handles HTTP requests for audit log queries.
type AuditLogsHandler struct {
	querier *service.AuditLogQuerier
}

// NewAuditLogsHandler creates a new audit logs handler.
func NewAuditLogsHandler(db *sql.DB) *AuditLogsHandler {
	return &AuditLogsHandler{
		querier: service.NewAuditLogQuerier(db),
	}
}

// RegisterRoutes registers the audit logs routes with the given mux.
func (h *AuditLogsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/audit-logs", h.handleGetAuditLogs)
}

// handleGetAuditLogs handles GET requests to /api/audit-logs
func (h *AuditLogsHandler) handleGetAuditLogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	params, err := parseQueryParams(r)
	if err != nil {
		h.writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validate parameters
	if err := validateParams(params); err != nil {
		h.writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Build service layer options
	opts := service.AuditLogQueryOptions{
		Limit:  params.Limit,
		Offset: params.Offset,
	}

	if params.StartDate != nil {
		opts.StartTime = params.StartDate
	}

	if params.EndDate != nil {
		// End date is inclusive, so set it to end of day
		endOfDay := time.Date(params.EndDate.Year(), params.EndDate.Month(), params.EndDate.Day(), 23, 59, 59, 0, time.UTC)
		opts.EndTime = &endOfDay
	}

	if params.Actor != "" {
		opts.Actor = params.Actor
	}

	if params.EventType != "" {
		opts.EventType = params.EventType
	}

	// Call service layer
	var result *service.AuditLogQueryResult
	if params.RepoID == 0 {
		// Query all repos (admin-only - should be gated by auth layer)
		result, err = h.querier.QueryAllAuditLogs(ctx, opts)
	} else {
		// Query specific repo
		result, err = h.querier.QueryAuditLogs(ctx, params.RepoID, opts)
	}

	if err != nil {
		log.Printf("Error querying audit logs: %v", err)
		h.writeError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Write JSON response
	h.writeJSONResponse(w, result)
}

// queryParams represents parsed query parameters
type queryParams struct {
	RepoID    int64
	StartDate *time.Time
	EndDate   *time.Time
	Actor     string
	EventType string
	Limit     int
	Offset    int
}

// parseQueryParams parses query parameters from the HTTP request
func parseQueryParams(r *http.Request) (queryParams, error) {
	var params queryParams

	// Parse repo_id (required for non-admin queries, but we'll allow 0 for admin queries)
	repoIDStr := r.URL.Query().Get("repo_id")
	if repoIDStr != "" {
		repoID, err := strconv.ParseInt(repoIDStr, 10, 64)
		if err != nil {
			return params, fmt.Errorf("invalid repo_id: %s must be a valid integer", repoIDStr)
		}
		params.RepoID = repoID
	}

	// Parse start_date
	startDateStr := r.URL.Query().Get("start_date")
	if startDateStr != "" {
		startDate, err := parseDate(startDateStr)
		if err != nil {
			return params, fmt.Errorf("invalid start_date: %w", err)
		}
		params.StartDate = startDate
	}

	// Parse end_date
	endDateStr := r.URL.Query().Get("end_date")
	if endDateStr != "" {
		endDate, err := parseDate(endDateStr)
		if err != nil {
			return params, fmt.Errorf("invalid end_date: %w", err)
		}
		params.EndDate = endDate
	}

	// Parse actor (optional)
	params.Actor = r.URL.Query().Get("actor")

	// Parse event_type (optional)
	params.EventType = r.URL.Query().Get("event_type")

	// Parse limit (default: 100)
	limitStr := r.URL.Query().Get("limit")
	if limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil {
			return params, fmt.Errorf("invalid limit: %s must be a valid integer", limitStr)
		}
		params.Limit = limit
	} else {
		params.Limit = 100 // default
	}

	// Parse offset (default: 0)
	offsetStr := r.URL.Query().Get("offset")
	if offsetStr != "" {
		offset, err := strconv.Atoi(offsetStr)
		if err != nil {
			return params, fmt.Errorf("invalid offset: %s must be a valid integer", offsetStr)
		}
		params.Offset = offset
	} else {
		params.Offset = 0 // default
	}

	return params, nil
}

// parseDate parses a date string in YYYY-MM-DD format
func parseDate(dateStr string) (*time.Time, error) {
	// Check format with regex first
	dateRegex := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	if !dateRegex.MatchString(dateStr) {
		return nil, fmt.Errorf("invalid date format: '%s'. Expected YYYY-MM-DD format", dateStr)
	}

	// Parse the date
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid date: '%s' is not a valid calendar date", dateStr)
	}

	// Check date range (1970-01-01 to 2100-12-31)
	minDate := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	maxDate := time.Date(2100, 12, 31, 23, 59, 59, 0, time.UTC)

	if t.Before(minDate) || t.After(maxDate) {
		return nil, fmt.Errorf("date out of range: '%s' must be between 1970-01-01 and 2100-12-31", dateStr)
	}

	return &t, nil
}

// validateParams validates the parsed query parameters
func validateParams(params queryParams) error {
	// Validate repo_id
	if params.RepoID < 0 {
		return fmt.Errorf("invalid repo_id: %d. Must be a positive integer or 0 (for all repos)", params.RepoID)
	}

	// Validate limit (1-1000)
	if params.Limit < 1 || params.Limit > 1000 {
		return fmt.Errorf("invalid limit: %d. Must be between 1 and 1000", params.Limit)
	}

	// Validate offset (>= 0)
	if params.Offset < 0 {
		return fmt.Errorf("invalid offset: %d. Must be >= 0", params.Offset)
	}

	// Validate event_type if provided
	if params.EventType != "" {
		if params.EventType != "exclude" && params.EventType != "unexclude" {
			return fmt.Errorf("invalid event_type: '%s'. Must be 'exclude' or 'unexclude'", params.EventType)
		}
	}

	// Validate actor length
	if len(params.Actor) > 255 {
		return fmt.Errorf("actor too long: %d characters exceeds maximum of 255", len(params.Actor))
	}

	// Validate date chronology
	if params.StartDate != nil && params.EndDate != nil {
		if params.StartDate.After(*params.EndDate) {
			return fmt.Errorf("start date after end date: '%s' > '%s'",
				params.StartDate.Format("2006-01-02"), params.EndDate.Format("2006-01-02"))
		}
	}

	return nil
}

// apiResponse represents the standard API response structure
type apiResponse struct {
	Records    []auditLogRecordJSON `json:"records"`
	TotalCount int64                `json:"total_count"`
	Limit      int                  `json:"limit"`
	Offset     int                  `json:"offset"`
}

// auditLogRecordJSON represents an audit log record in JSON format
type auditLogRecordJSON struct {
	ID                 int64      `json:"id"`
	RepoID             int64      `json:"repo_id"`
	Actor              string     `json:"actor"`
	Timestamp          string     `json:"timestamp"`
	EventType          string     `json:"event_type"`
	OldExcludedAt      *string    `json:"old_excluded_at,omitempty"`
	OldExcludedReason  *string    `json:"old_excluded_reason,omitempty"`
	NewExcludedAt      *string    `json:"new_excluded_at,omitempty"`
	NewExcludedReason *string    `json:"new_excluded_reason,omitempty"`
}

// writeJSONResponse writes a successful JSON response
func (h *AuditLogsHandler) writeJSONResponse(w http.ResponseWriter, result *service.AuditLogQueryResult) {
	// Convert service layer records to JSON format
	recordsJSON := make([]auditLogRecordJSON, len(result.Records))
	for i, rec := range result.Records {
		recordsJSON[i] = auditLogRecordJSON{
			ID:         rec.ID,
			RepoID:     rec.RepoID,
			Actor:      rec.Actor,
			Timestamp:  rec.Timestamp.Format(time.RFC3339Nano),
			EventType:  rec.EventType,
			OldExcludedAt:      formatNullableTime(rec.OldExcludedAt),
			OldExcludedReason:  rec.OldExcludedReason,
			NewExcludedAt:      formatNullableTime(rec.NewExcludedAt),
			NewExcludedReason: rec.NewExcludedReason,
		}
	}

	response := apiResponse{
		Records:    recordsJSON,
		TotalCount: result.TotalCount,
		Limit:      result.Limit,
		Offset:     result.Offset,
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Total-Count", strconv.FormatInt(result.TotalCount, 10))
	w.Header().Set("X-Limit", strconv.Itoa(result.Limit))
	w.Header().Set("X-Offset", strconv.Itoa(result.Offset))

	// Write response
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(response); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
	}
}

// formatNullableTime converts a *time.Time to a ISO 8601 string or null
func formatNullableTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	formatted := t.Format(time.RFC3339Nano)
	return &formatted
}

// errorResponse represents an error response
type errorResponse struct {
	Error errorDetail `json:"error"`
}

// errorDetail represents error details
type errorDetail struct {
	Code    string                  `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// writeError writes an error response
func (h *AuditLogsHandler) writeError(w http.ResponseWriter, message string, statusCode int) {
	// Determine error code
	var code string
	switch statusCode {
	case http.StatusBadRequest:
		code = "INVALID_PARAMETER"
	case http.StatusNotFound:
		code = "REPO_NOT_FOUND"
	case http.StatusForbidden:
		code = "ACCESS_DENIED"
	case http.StatusInternalServerError:
		code = "INTERNAL_ERROR"
	default:
		code = "UNKNOWN_ERROR"
	}

	response := errorResponse{
		Error: errorDetail{
			Code:    code,
			Message: message,
			Details: nil,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(response); err != nil {
		log.Printf("Error encoding error response: %v", err)
	}
}
