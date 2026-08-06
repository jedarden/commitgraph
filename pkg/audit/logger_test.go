package audit

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestLogExclusion(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	logger := &Logger{
		output: &testLogger{writer: &buf},
	}

	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	event := Event{
		Timestamp:    now,
		Operation:    "exclude",
		Provider:     "github",
		RepoFullName: "owner/repo",
		Operator:     "test-operator",
		Reason:       "false attribution report",
		RowsAffected: 1,
		IncidentID:   "INC-001",
	}

	err := logger.LogExclusion(event)
	if err != nil {
		t.Fatalf("LogExclusion failed: %v", err)
	}

	// Verify JSON output
	output := strings.TrimSpace(buf.String())
	var decoded Event
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, output)
	}

	if decoded.Operation != "exclude" {
		t.Errorf("Operation = %q, want %q", decoded.Operation, "exclude")
	}
	if decoded.Provider != "github" {
		t.Errorf("Provider = %q, want %q", decoded.Provider, "github")
	}
	if decoded.RepoFullName != "owner/repo" {
		t.Errorf("RepoFullName = %q, want %q", decoded.RepoFullName, "owner/repo")
	}
	if decoded.Operator != "test-operator" {
		t.Errorf("Operator = %q, want %q", decoded.Operator, "test-operator")
	}
	if decoded.Reason != "false attribution report" {
		t.Errorf("Reason = %q, want %q", decoded.Reason, "false attribution report")
	}
	if decoded.RowsAffected != 1 {
		t.Errorf("RowsAffected = %d, want 1", decoded.RowsAffected)
	}
	if decoded.IncidentID != "INC-001" {
		t.Errorf("IncidentID = %q, want %q", decoded.IncidentID, "INC-001")
	}
}

func TestLogExclusionValidation(t *testing.T) {
	logger := NewLogger()

	tests := []struct {
		name    string
		event   Event
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid event",
			event: Event{
				Timestamp:    time.Now().UTC(),
				Operation:    "exclude",
				Provider:     "github",
				RepoFullName: "owner/repo",
				Operator:     "operator",
				Reason:       "test reason",
				RowsAffected: 1,
			},
			wantErr: false,
		},
		{
			name: "missing operation",
			event: Event{
				Provider:     "github",
				RepoFullName: "owner/repo",
				Operator:     "operator",
			},
			wantErr: true,
			errMsg:  "operation is required",
		},
		{
			name: "missing provider",
			event: Event{
				Operation:    "exclude",
				RepoFullName: "owner/repo",
				Operator:     "operator",
			},
			wantErr: true,
			errMsg:  "provider is required",
		},
		{
			name: "missing repo_full_name",
			event: Event{
				Operation: "exclude",
				Provider:  "github",
				Operator:  "operator",
			},
			wantErr: true,
			errMsg:  "repo_full_name is required",
		},
		{
			name: "missing operator",
			event: Event{
				Operation:    "exclude",
				Provider:     "github",
				RepoFullName: "owner/repo",
			},
			wantErr: true,
			errMsg:  "operator is required",
		},
		{
			name: "timestamp auto-set",
			event: Event{
				Operation:    "exclude",
				Provider:     "github",
				RepoFullName: "owner/repo",
				Operator:     "operator",
				Reason:       "test",
				RowsAffected: 1,
				// Timestamp is zero - should be auto-set
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := logger.LogExclusion(tt.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("LogExclusion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error message = %q, want containing %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestLogExclusionInline(t *testing.T) {
	// This is a smoke test - we can't easily capture stderr output
	// Just verify it doesn't panic
	LogExclusionInline("exclude", "github", "owner/repo", "test-operator", "test reason", 1, "INC-001")
}

// testLogger is a minimal logger for testing.
type testLogger struct {
	writer *bytes.Buffer
}

func (t *testLogger) Println(v interface{}) {
	t.writer.WriteString(v.(string))
	t.writer.WriteString("\n")
}
