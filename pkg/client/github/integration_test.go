// Package github provides integration tests for the mock GitHub client.
// These tests demonstrate how the mock can be used in place of the real GitHub API
// for testing the revalidation worker's login detection logic.
package github

import (
	"context"
	"testing"
)

// TestMockClient_RevalidationWorkerScenario tests a realistic revalidation worker scenario.
// This test simulates the worker checking various logins and handling different outcomes.
func TestMockClient_RevalidationWorkerScenario(t *testing.T) {
	ctx := context.Background()
	mock := NewMockClient()

	// Configure mock responses matching the three key scenarios:
	// 1. old-name was renamed to new-name
	mock.SetResponse("old-name", &LoginResult{
		Status:   StatusRenamed,
		NewLogin: strPtr("new-name"),
	})

	// 2. deleted-user account was deleted
	mock.SetResponse("deleted-user", &LoginResult{
		Status: StatusDeleted,
	})

	// 3. active-user is still valid (uses default)

	// Simulate worker checking these logins
	cases := []struct {
		login         string
		expectedStatus LoginStatus
		expectedNewLogin *string
	}{
		{
			login:         "old-name",
			expectedStatus: StatusRenamed,
			expectedNewLogin: strPtr("new-name"),
		},
		{
			login:         "deleted-user",
			expectedStatus: StatusDeleted,
			expectedNewLogin: nil,
		},
		{
			login:         "active-user",
			expectedStatus: StatusValidated,
			expectedNewLogin: nil,
		},
	}

	for _, tc := range cases {
		result, err := mock.CheckLogin(ctx, tc.login)
		if err != nil {
			t.Errorf("CheckLogin(%q): unexpected error: %v", tc.login, err)
			continue
		}

		if result.Status != tc.expectedStatus {
			t.Errorf("CheckLogin(%q): expected status %s, got %s",
				tc.login, tc.expectedStatus, result.Status)
		}

		if result.NewLogin == nil && tc.expectedNewLogin != nil {
			t.Errorf("CheckLogin(%q): expected NewLogin=%q, got nil",
				tc.login, *tc.expectedNewLogin)
		}

		if result.NewLogin != nil && tc.expectedNewLogin == nil {
			t.Errorf("CheckLogin(%q): expected NewLogin=nil, got %q",
				tc.login, *result.NewLogin)
		}

		if result.NewLogin != nil && tc.expectedNewLogin != nil &&
			*result.NewLogin != *tc.expectedNewLogin {
			t.Errorf("CheckLogin(%q): expected NewLogin=%q, got %q",
				tc.login, *tc.expectedNewLogin, *result.NewLogin)
		}
	}

	// Verify all logins were checked
	if mock.CallCount() != len(cases) {
		t.Errorf("Expected %d calls, got %d", len(cases), mock.CallCount())
	}

	// Verify specific logins were called
	if !mock.WasCalled("old-name") {
		t.Errorf("Expected 'old-name' to be called")
	}
	if !mock.WasCalled("deleted-user") {
		t.Errorf("Expected 'deleted-user' to be called")
	}
	if !mock.WasCalled("active-user") {
		t.Errorf("Expected 'active-user' to be called")
	}
}

// TestMockClient_LoginRenameFlow tests a complete login rename flow.
// This simulates the revalidation worker detecting a rename and the downstream
// process of updating email_resolution.
func TestMockClient_LoginRenameFlow(t *testing.T) {
	ctx := context.Background()
	mock := NewMockClient()

	// Initial state: login "alice" exists
	mock.SetDefaultResult(&LoginResult{Status: StatusValidated})

	// Worker checks "alice" - login is valid
	result, err := mock.CheckLogin(ctx, "alice")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.Status != StatusValidated {
		t.Errorf("Expected alice to be validated, got %s", result.Status)
	}

	// Simulate time passing... alice renames to "alice-new"

	// Update mock to reflect the rename
	mock.SetResponse("alice", &LoginResult{
		Status:   StatusRenamed,
		NewLogin: strPtr("alice-new"),
	})

	// Worker checks "alice" again - now detects rename
	result, err = mock.CheckLogin(ctx, "alice")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.Status != StatusRenamed {
		t.Errorf("Expected alice to be renamed, got %s", result.Status)
	}
	if result.NewLogin == nil || *result.NewLogin != "alice-new" {
		t.Errorf("Expected new login 'alice-new', got %v", result.NewLogin)
	}

	// Verify call tracking: "alice" was checked twice
	if mock.CallCount() != 2 {
		t.Errorf("Expected 2 calls, got %d", mock.CallCount())
	}

	logins := mock.CalledLogins()
	if len(logins) != 2 || logins[0] != "alice" || logins[1] != "alice" {
		t.Errorf("Expected 2 calls to 'alice', got %v", logins)
	}
}

// TestMockClient_RateLimitScenario tests handling rate limit responses.
func TestMockClient_RateLimitScenario(t *testing.T) {
	ctx := context.Background()
	mock := NewMockClient()

	// Simulate GitHub API rate limiting
	rateLimitMsg := "rate limited"
	mock.SetResponse("user1", &LoginResult{
		Status:   StatusRetry,
		ErrorMsg: &rateLimitMsg,
	})

	result, err := mock.CheckLogin(ctx, "user1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Status != StatusRetry {
		t.Errorf("Expected retry status, got %s", result.Status)
	}

	if result.ErrorMsg == nil || *result.ErrorMsg != "rate limited" {
		t.Errorf("Expected error message 'rate limited', got %v", result.ErrorMsg)
	}
}

// TestMockClient_BatchProcessing tests a batch of logins being checked.
// This simulates the worker processing a batch of rows from email_revalidation.
func TestMockClient_BatchProcessing(t *testing.T) {
	ctx := context.Background()
	mock := NewMockClient()

	// Configure mock with various responses
	responses := map[string]*LoginResult{
		"user-1": {Status: StatusValidated},
		"user-2": {Status: StatusDeleted},
		"user-3": {Status: StatusRenamed, NewLogin: strPtr("user-3-renamed")},
		"user-4": {Status: StatusValidated},
		"user-5": {Status: StatusDeleted},
	}

	for login, result := range responses {
		mock.SetResponse(login, result)
	}

	// Simulate batch processing
	logins := []string{"user-1", "user-2", "user-3", "user-4", "user-5"}
	for _, login := range logins {
		_, err := mock.CheckLogin(ctx, login)
		if err != nil {
			t.Errorf("CheckLogin(%q): unexpected error: %v", login, err)
		}
	}

	// Verify all were processed
	if mock.CallCount() != len(logins) {
		t.Errorf("Expected %d calls, got %d", len(logins), mock.CallCount())
	}

	// Verify order was preserved
	calledLogins := mock.CalledLogins()
	for i, login := range logins {
		if calledLogins[i] != login {
			t.Errorf("Call %d: expected '%s', got '%s'", i, login, calledLogins[i])
		}
	}
}
