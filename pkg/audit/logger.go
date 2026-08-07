// Package audit provides structured logging for security-sensitive operations.
package audit

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

// stdLogger is the subset of *log.Logger's interface that Logger depends on.
// Defining it as an interface (rather than embedding *log.Logger directly)
// lets tests substitute a lightweight recorder to capture and assert on
// output without shelling out to the real stderr stream.
type stdLogger interface {
	Println(v ...interface{})
}

// Logger writes structured audit logs for security-sensitive operations.
type Logger struct {
	output stdLogger
}

// NewLogger creates a new audit logger that writes to stderr (or a configured output).
func NewLogger() *Logger {
	return &Logger{
		output: log.New(os.Stderr, "[AUDIT] ", log.LstdFlags|log.Lmicroseconds|log.LUTC),
	}
}

// Event represents a single audit log event.
type Event struct {
	Timestamp     time.Time `json:"timestamp"`
	Operation     string    `json:"operation"`     // "exclude" or "clear"
	Provider      string    `json:"provider"`
	RepoFullName  string    `json:"repo_full_name"`
	Operator      string    `json:"operator"`      // who performed the action
	Reason        string    `json:"reason"`         // why (exclusion reason or empty for clear)
	RowsAffected  int64     `json:"rows_affected"`  // 1 if repo existed, 0 if not found
	IncidentID    string    `json:"incident_id,omitempty"` // optional incident tracking
}

// LogExclusion logs an exclusion or clear operation to the audit log.
//
// This feeds q-threat-exclusion-audit-log for incident response and postmortem analysis.
// Every exclusion/un-exclusion action must be logged with who/when/why.
func (l *Logger) LogExclusion(event Event) error {
	// Validate required fields
	if event.Operation == "" {
		return fmt.Errorf("operation is required")
	}
	if event.Provider == "" {
		return fmt.Errorf("provider is required")
	}
	if event.RepoFullName == "" {
		return fmt.Errorf("repo_full_name is required")
	}
	if event.Operator == "" {
		return fmt.Errorf("operator is required (for audit trail)")
	}

	// Set timestamp if not provided
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	// Serialize to JSON for structured log consumption
	jsonBytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal audit event: %w", err)
	}

	// Write to log output
	l.output.Println(string(jsonBytes))

	return nil
}

// LogExclusionInline is a convenience function for logging without a Logger instance.
// This is useful for quick scripts that don't want to manage a logger.
func LogExclusionInline(operation, provider, repoFullName, operator, reason string, rowsAffected int64, incidentID string) {
	logger := NewLogger()
	event := Event{
		Operation:    operation,
		Provider:     provider,
		RepoFullName: repoFullName,
		Operator:     operator,
		Reason:       reason,
		RowsAffected: rowsAffected,
		IncidentID:   incidentID,
	}
	if err := logger.LogExclusion(event); err != nil {
		// If logging fails, at least emit to stderr with a clear error marker
		log.Printf("ERROR: failed to write audit log: %v\n", err)
		log.Printf("ERROR: event was: %s %s/%s by %s\n", operation, provider, repoFullName, operator)
	}
}
