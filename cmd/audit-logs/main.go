// audit-logs is a CLI tool for querying audit logs from the commitgraph database.
//
// This tool provides a modern interface to audit log data using the service layer.
// It supports filtering by repository, date range, actor, and event type with pagination.
//
// Usage:
//
//	audit-logs --repo-id 123 --output table
//	audit-logs --repo-id 123 --actor admin --output json
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"time"

	_ "github.com/lib/pq"
	"github.com/jedarden/commitgraph/pkg/service"
)

var (
	// Global flags
	outputFormat = flag.String("output", "table", "Output format: 'json' or 'table'")

	// Connection flags
	dbHost     = flag.String("db-host", "", "PostgreSQL host (required)")
	dbPort     = flag.String("db-port", "5432", "PostgreSQL port")
	dbName     = flag.String("db-name", "commitgraph", "PostgreSQL database name")
	dbUser     = flag.String("db-user", "", "PostgreSQL user (required)")
	dbPassword = flag.String("db-password", "", "PostgreSQL password (required, use env var in production)")
	sslMode    = flag.String("sslmode", "require", "PostgreSQL SSL mode")

	// Query-specific flags
	repoID    = flag.Int64("repo-id", 0, "Repository ID to query (required, must be > 0)")
	startDate = flag.String("start-date", "", "Start date for filtering (YYYY-MM-DD format, inclusive)")
	endDate   = flag.String("end-date", "", "End date for filtering (YYYY-MM-DD format, inclusive)")
	actor     = flag.String("actor", "", "Filter by actor (exact match, case-sensitive)")
	eventType = flag.String("event-type", "", "Filter by event type ('exclude' or 'unexclude')")
	limit     = flag.Int("limit", 100, "Maximum number of records to return (1-1000, default: 100)")
	offset    = flag.Int("offset", 0, "Number of records to skip for pagination (default: 0)")
)

func main() {
	flag.Usage = usage

	// Handle help flags (checked against the raw args, before any subcommand
	// stripping below, so "audit-logs help" and "audit-logs --help" both work).
	if len(os.Args) > 1 && (os.Args[1] == "help" || os.Args[1] == "-h" || os.Args[1] == "--help") {
		flag.Usage()
		os.Exit(0)
	}

	// "query" is documented (usage text, README examples) as a leading
	// subcommand token: `audit-logs query -repo-id 123`. flag.Parse() stops
	// at the first non-flag argument, so without this, that exact documented
	// invocation would silently discard every flag after "query". Accept and
	// strip it if present so both `audit-logs query -repo-id 123` and the
	// bare `audit-logs -repo-id 123` form work identically.
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "query" {
		args = args[1:]
	}

	// Parse all flags
	if err := flag.CommandLine.Parse(args); err != nil {
		os.Exit(2)
	}

	// Validate output format
	if *outputFormat != "json" && *outputFormat != "table" {
		log.Fatal("error: -output must be 'json' or 'table'")
	}

	// Execute query
	handleQuery(*repoID, *startDate, *endDate, *actor, *eventType, *limit, *offset)
}

func handleQuery(repoID int64, startDate, endDate, actor, eventType string, limit, offset int) {
	// Validate required flags
	if *dbHost == "" {
		log.Fatal("error: -db-host is required")
	}
	if *dbUser == "" {
		log.Fatal("error: -db-user is required")
	}
	if *dbPassword == "" {
		log.Fatal("error: -db-password is required")
	}

	// Validate repo_id
	if repoID <= 0 {
		log.Fatal("error: -repo-id is required and must be > 0")
	}

	// Parse and validate dates
	var parsedStart, parsedEnd *time.Time
	var err error

	if startDate != "" {
		parsedStart, err = parseDate(startDate)
		if err != nil {
			log.Fatalf("error: invalid start-date: %v", err)
		}
	}

	if endDate != "" {
		parsedEnd, err = parseDate(endDate)
		if err != nil {
			log.Fatalf("error: invalid end-date: %v", err)
		}
		// End date is inclusive, so set to end of day
		endOfDay := time.Date(parsedEnd.Year(), parsedEnd.Month(), parsedEnd.Day(), 23, 59, 59, 0, time.UTC)
		parsedEnd = &endOfDay
	}

	// Validate date chronology
	if parsedStart != nil && parsedEnd != nil && parsedStart.After(*parsedEnd) {
		log.Fatalf("error: start_date (%s) cannot be after end_date (%s)", startDate, endDate)
	}

	// Validate event_type
	if eventType != "" {
		if eventType != "exclude" && eventType != "unexclude" {
			log.Fatalf("error: event-type must be 'exclude' or 'unexclude', got '%s'", eventType)
		}
	}

	// Validate limit
	if limit < 1 || limit > 1000 {
		log.Fatalf("error: limit must be between 1 and 1000, got %d", limit)
	}

	// Validate offset
	if offset < 0 {
		log.Fatalf("error: offset must be >= 0, got %d", offset)
	}

	// Validate actor length
	if len(actor) > 255 {
		log.Fatalf("error: actor too long: %d characters exceeds maximum of 255", len(actor))
	}

	// Connect to database
	connStr := fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=%s",
		*dbHost, *dbPort, *dbName, *dbUser, *dbPassword, *sslMode)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("error: failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()

	// Verify connection works
	if err := db.Ping(); err != nil {
		log.Fatalf("error: PostgreSQL ping failed: %v", err)
	}

	ctx := context.Background()

	// Create service layer querier
	querier := service.NewAuditLogQuerier(db)

	// Build query options
	opts := service.AuditLogQueryOptions{
		Limit:  limit,
		Offset: offset,
	}

	if parsedStart != nil {
		opts.StartTime = parsedStart
	}

	if parsedEnd != nil {
		opts.EndTime = parsedEnd
	}

	if actor != "" {
		opts.Actor = actor
	}

	if eventType != "" {
		opts.EventType = eventType
	}

	// Log query invocation
	log.Printf("Querying audit logs: repo_id=%d, start_time=%v, end_time=%v, actor=%s, event_type=%s, limit=%d, offset=%d",
		repoID, parsedStart, parsedEnd, actor, eventType, limit, offset)

	// Query audit logs
	result, err := querier.QueryAuditLogs(ctx, repoID, opts)
	if err != nil {
		log.Fatalf("error: failed to query audit logs: %v", err)
	}

	log.Printf("Query completed: returned %d records (total count: %d)", len(result.Records), result.TotalCount)

	// Output results
	if *outputFormat == "json" {
		outputJSON(result)
	} else {
		outputTable(result, repoID)
	}
}

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

func outputJSON(result *service.AuditLogQueryResult) {
	type auditLogRecordJSON struct {
		ID                 int64     `json:"id"`
		RepoID             int64     `json:"repo_id"`
		Actor              string    `json:"actor"`
		Timestamp          string    `json:"timestamp"`
		EventType          string    `json:"event_type"`
		OldExcludedAt      *string   `json:"old_excluded_at,omitempty"`
		OldExcludedReason  *string   `json:"old_excluded_reason,omitempty"`
		NewExcludedAt      *string   `json:"new_excluded_at,omitempty"`
		NewExcludedReason  *string   `json:"new_excluded_reason,omitempty"`
	}

	type apiResponse struct {
		Records    []auditLogRecordJSON `json:"records"`
		TotalCount int64                 `json:"total_count"`
		Limit      int                   `json:"limit"`
		Offset     int                   `json:"offset"`
	}

	// Convert service layer records to JSON format
	recordsJSON := make([]auditLogRecordJSON, len(result.Records))
	for i, rec := range result.Records {
		recordsJSON[i] = auditLogRecordJSON{
			ID:                rec.ID,
			RepoID:            rec.RepoID,
			Actor:             rec.Actor,
			Timestamp:         rec.Timestamp.Format(time.RFC3339Nano),
			EventType:         rec.EventType,
			OldExcludedAt:     formatNullableTime(rec.OldExcludedAt),
			OldExcludedReason: rec.OldExcludedReason,
			NewExcludedAt:     formatNullableTime(rec.NewExcludedAt),
			NewExcludedReason: rec.NewExcludedReason,
		}
	}

	response := apiResponse{
		Records:    recordsJSON,
		TotalCount: result.TotalCount,
		Limit:      result.Limit,
		Offset:     result.Offset,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(response); err != nil {
		log.Fatalf("error: failed to encode JSON: %v", err)
	}
}

func outputTable(result *service.AuditLogQueryResult, repoID int64) {
	if len(result.Records) == 0 {
		fmt.Println("No records found.")
		return
	}

	// Print summary
	fmt.Printf("Audit Logs for Repo ID: %d\n", repoID)
	fmt.Printf("Showing %d of %d total records (limit: %d, offset: %d)\n",
		len(result.Records), result.TotalCount, result.Limit, result.Offset)
	fmt.Println()

	// Print table header
	fmt.Println("┃ ID     │ Timestamp           │ Event      │ Actor      │ Old Excluded │ New Excluded │ Reason")
	fmt.Println("┃───────┼─────────────────────┼────────────┼────────────┼──────────────┼──────────────┼────────")

	for _, rec := range result.Records {
		timestamp := rec.Timestamp.Format("2006-01-02 15:04:05")
		oldExcluded := formatNullableTimeShort(rec.OldExcludedAt)
		newExcluded := formatNullableTimeShort(rec.NewExcludedAt)
		reason := formatNullableString(rec.NewExcludedReason)

		// Truncate long reasons
		if len(reason) > 50 && reason != "NULL" {
			reason = truncateString(*rec.NewExcludedReason, 47) + "..."
		}

		fmt.Printf("┃ %-5d │ %-19s │ %-10s │ %-10s │ %-12s │ %-12s │ %s\n",
			rec.ID, timestamp, rec.EventType, rec.Actor, oldExcluded, newExcluded, reason)
	}

	fmt.Println()
	fmt.Printf("Total: %d of %d records\n", len(result.Records), result.TotalCount)
	if result.TotalCount > int64(result.Limit) {
		fmt.Printf("Use --offset %d to see next page\n", result.Offset+result.Limit)
	}
}

func formatNullableTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	formatted := t.Format(time.RFC3339Nano)
	return &formatted
}

func formatNullableTimeShort(t *time.Time) string {
	if t == nil {
		return "NULL"
	}
	return t.Format("2006-01-02")
}

func formatNullableString(s *string) string {
	if s == nil {
		return "NULL"
	}
	return *s
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

func usage() {
	fmt.Fprintf(os.Stderr, `audit-logs: CLI tool for querying audit logs from the commitgraph database

This tool provides a modern interface to audit log data using the service layer.
It supports filtering by repository, date range, actor, and event type with pagination.

Usage:
  audit-logs query [flags]

Flags:
  Global:
    -output string
        Output format: 'json' or 'table' (default "table")

  Database Connection:
    -db-host string
        PostgreSQL host (required)
    -db-port string
        PostgreSQL port (default "5432")
    -db-name string
        PostgreSQL database name (default "commitgraph")
    -db-user string
        PostgreSQL user (required)
    -db-password string
        PostgreSQL password (required, use env var in production)
    -sslmode string
        PostgreSQL SSL mode (default "require")

  Query Filters (for 'query' subcommand):
    -repo-id int
        Repository ID to query (required, must be > 0)
    -start-date string
        Start date for filtering (YYYY-MM-DD format, inclusive)
    -end-date string
        End date for filtering (YYYY-MM-DD format, inclusive)
    -actor string
        Filter by actor (exact match, case-sensitive, max 255 chars)
    -event-type string
        Filter by event type ('exclude' or 'unexclude')
    -limit int
        Maximum number of records to return (1-1000, default: 100)
    -offset int
        Number of records to skip for pagination (default: 0)

Subcommands:
  query
    Query audit logs with optional filtering and pagination

  help
    Show this help message

Examples:
  # Query all audit logs for a repository (table format)
  audit-logs query -repo-id 123

  # Query with date range filter
  audit-logs query -repo-id 123 -start-date 2024-01-01 -end-date 2024-12-31

  # Query by specific actor
  audit-logs query -repo-id 123 -actor admin

  # Query by event type with JSON output
  audit-logs query -repo-id 123 -event-type exclude -output json

  # Paginate through results
  audit-logs query -repo-id 123 -limit 50 -offset 0
  audit-logs query -repo-id 123 -limit 50 -offset 50

  # Combine multiple filters
  audit-logs query -repo-id 123 -actor admin -event-type exclude -start-date 2024-01-01 -output json

Database Connection:
  Use environment variables for sensitive data:
    export DB_PASSWORD=$(get_db_password)
    audit-logs query -repo-id 123 -db-host localhost -db-user user -db-password "$DB_PASSWORD"

Specification:
  This CLI follows the audit log query interface specification.
  See: docs/audit-log-query-interface-spec.md

For more information:
  https://github.com/jedarden/commitgraph
`)
}
