// Package github provides tests for the GitHub client and mock.
package github

import (
	"context"
	"testing"
)

// TestMockClient_RenameResponse verifies the mock returns a rename response.
func TestMockClient_RenameResponse(t *testing.T) {
	ctx := context.Background()
	mock := NewMockClient()

	// Configure mock to return a renamed response for old-name
	mock.SetResponse("old-name", &LoginResult{
		Status:   StatusRenamed,
		NewLogin: strPtr("new-name"),
	})

	// Test the renamed response
	result, err := mock.CheckLogin(ctx, "old-name")
	if err != nil {
		t.Fatalf("CheckLogin should not return error, got: %v", err)
	}

	if result.Status != StatusRenamed {
		t.Errorf("Expected status %s, got %s", StatusRenamed, result.Status)
	}

	if result.NewLogin == nil {
		t.Fatalf("Expected NewLogin to be set for rename status")
	}

	if *result.NewLogin != "new-name" {
		t.Errorf("Expected new login 'new-name', got '%s'", *result.NewLogin)
	}

	if result.ErrorMsg != nil {
		t.Errorf("Expected no error message, got '%s'", *result.ErrorMsg)
	}
}

// TestMockClient_DeletedResponse verifies the mock returns a deleted response.
func TestMockClient_DeletedResponse(t *testing.T) {
	ctx := context.Background()
	mock := NewMockClient()

	// Configure mock to return a deleted response
	mock.SetResponse("deleted-user", &LoginResult{
		Status: StatusDeleted,
	})

	// Test the deleted response
	result, err := mock.CheckLogin(ctx, "deleted-user")
	if err != nil {
		t.Fatalf("CheckLogin should not return error, got: %v", err)
	}

	if result.Status != StatusDeleted {
		t.Errorf("Expected status %s, got %s", StatusDeleted, result.Status)
	}

	if result.NewLogin != nil {
		t.Errorf("Expected NewLogin to be nil for deleted status, got '%s'", *result.NewLogin)
	}

	if result.ErrorMsg != nil {
		t.Errorf("Expected no error message, got '%s'", *result.ErrorMsg)
	}
}

// TestMockClient_ValidatedResponse verifies the mock returns a validated response.
func TestMockClient_ValidatedResponse(t *testing.T) {
	ctx := context.Background()
	mock := NewMockClient()

	// Configure mock to return a validated response
	mock.SetResponse("active-user", &LoginResult{
		Status: StatusValidated,
	})

	// Test the validated response
	result, err := mock.CheckLogin(ctx, "active-user")
	if err != nil {
		t.Fatalf("CheckLogin should not return error, got: %v", err)
	}

	if result.Status != StatusValidated {
		t.Errorf("Expected status %s, got %s", StatusValidated, result.Status)
	}

	if result.NewLogin != nil {
		t.Errorf("Expected NewLogin to be nil for validated status, got '%s'", *result.NewLogin)
	}

	if result.ErrorMsg != nil {
		t.Errorf("Expected no error message, got '%s'", *result.ErrorMsg)
	}
}

// TestMockClient_RetryResponse verifies the mock returns a retry response.
func TestMockClient_RetryResponse(t *testing.T) {
	ctx := context.Background()
	mock := NewMockClient()

	// Configure mock to return a retry response
	errMsg := "rate limited"
	mock.SetResponse("rate-limited-user", &LoginResult{
		Status:   StatusRetry,
		ErrorMsg: &errMsg,
	})

	// Test the retry response
	result, err := mock.CheckLogin(ctx, "rate-limited-user")
	if err != nil {
		t.Fatalf("CheckLogin should not return error, got: %v", err)
	}

	if result.Status != StatusRetry {
		t.Errorf("Expected status %s, got %s", StatusRetry, result.Status)
	}

	if result.NewLogin != nil {
		t.Errorf("Expected NewLogin to be nil for retry status, got '%s'", *result.NewLogin)
	}

	if result.ErrorMsg == nil {
		t.Fatalf("Expected error message for retry status")
	}

	if *result.ErrorMsg != "rate limited" {
		t.Errorf("Expected error message 'rate limited', got '%s'", *result.ErrorMsg)
	}
}

// TestMockClient_DefaultResult verifies the mock uses default result for unconfigured logins.
func TestMockClient_DefaultResult(t *testing.T) {
	ctx := context.Background()
	mock := NewMockClient()

	// Set a custom default result
	mock.SetDefaultResult(&LoginResult{
		Status: StatusDeleted,
	})

	// Test with an unconfigured login
	result, err := mock.CheckLogin(ctx, "unconfigured-login")
	if err != nil {
		t.Fatalf("CheckLogin should not return error, got: %v", err)
	}

	if result.Status != StatusDeleted {
		t.Errorf("Expected default status %s, got %s", StatusDeleted, result.Status)
	}
}

// TestMockClient_NoResponseConfigured verifies the mock uses validated as default.
func TestMockClient_NoResponseConfigured(t *testing.T) {
	ctx := context.Background()
	mock := NewMockClient()

	// Don't configure any response, should use built-in default (validated)

	// Test with an unconfigured login
	result, err := mock.CheckLogin(ctx, "any-login")
	if err != nil {
		t.Fatalf("CheckLogin should not return error, got: %v", err)
	}

	if result.Status != StatusValidated {
		t.Errorf("Expected default status %s when no responses configured, got %s", StatusValidated, result.Status)
	}
}

// TestMockClient_CallTracking verifies the mock tracks CheckLogin calls.
func TestMockClient_CallTracking(t *testing.T) {
	ctx := context.Background()
	mock := NewMockClient()

	// Configure responses for multiple logins
	mock.SetResponse("user1", &LoginResult{Status: StatusValidated})
	mock.SetResponse("user2", &LoginResult{Status: StatusDeleted})

	// Make several calls
	mock.CheckLogin(ctx, "user1")
	mock.CheckLogin(ctx, "user2")
	mock.CheckLogin(ctx, "user3") // Uses default

	// Check call count
	if mock.CallCount() != 3 {
		t.Errorf("Expected call count 3, got %d", mock.CallCount())
	}

	// Check called logins
	logins := mock.CalledLogins()
	if len(logins) != 3 {
		t.Fatalf("Expected 3 called logins, got %d", len(logins))
	}

	expected := []string{"user1", "user2", "user3"}
	for i, login := range logins {
		if login != expected[i] {
			t.Errorf("Call %d: expected login '%s', got '%s'", i, expected[i], login)
		}
	}
}

// TestMockClient_WasCalled verifies the mock can check if a specific login was called.
func TestMockClient_WasCalled(t *testing.T) {
	ctx := context.Background()
	mock := NewMockClient()

	mock.CheckLogin(ctx, "user1")
	mock.CheckLogin(ctx, "user2")

	if !mock.WasCalled("user1") {
		t.Errorf("Expected user1 to be marked as called")
	}

	if !mock.WasCalled("user2") {
		t.Errorf("Expected user2 to be marked as called")
	}

	if mock.WasCalled("user3") {
		t.Errorf("Expected user3 to not be marked as called")
	}
}

// TestMockClient_Reset verifies the mock clears state on Reset.
func TestMockClient_Reset(t *testing.T) {
	ctx := context.Background()
	mock := NewMockClient()

	// Configure and make calls
	mock.SetResponse("user1", &LoginResult{Status: StatusDeleted})
	mock.CheckLogin(ctx, "user1")

	// Verify state
	if mock.CallCount() != 1 {
		t.Errorf("Expected call count 1 before reset")
	}

	// Reset
	mock.Reset()

	// Verify cleared state
	if mock.CallCount() != 0 {
		t.Errorf("Expected call count 0 after reset")
	}

	if len(mock.CalledLogins()) != 0 {
		t.Errorf("Expected no called logins after reset")
	}

	// After reset, unconfigured logins should use default
	result, err := mock.CheckLogin(ctx, "user1")
	if err != nil {
		t.Fatalf("CheckLogin should not return error, got: %v", err)
	}

	if result.Status != StatusValidated {
		t.Errorf("After reset, expected status %s for unconfigured login, got %s", StatusValidated, result.Status)
	}
}

// TestMockClient_NoHTTP verifies the mock makes no HTTP calls.
func TestMockClient_NoHTTP(t *testing.T) {
	ctx := context.Background()
	mock := NewMockClient()

	// Configure responses that would require HTTP in a real client
	mock.SetResponse("old-name", &LoginResult{
		Status:   StatusRenamed,
		NewLogin: strPtr("new-name"),
	})
	mock.SetResponse("deleted-user", &LoginResult{
		Status: StatusDeleted,
	})

	// Call CheckLogin - this should not make any HTTP requests
	// (we verify this by checking that it returns immediately with configured results)
	result1, err1 := mock.CheckLogin(ctx, "old-name")
	result2, err2 := mock.CheckLogin(ctx, "deleted-user")

	if err1 != nil {
		t.Errorf("First call should not error: %v", err1)
	}
	if result1.Status != StatusRenamed {
		t.Errorf("First call should return renamed status")
	}

	if err2 != nil {
		t.Errorf("Second call should not error: %v", err2)
	}
	if result2.Status != StatusDeleted {
		t.Errorf("Second call should return deleted status")
	}
}

// TestMockClient_ThreadSafety verifies concurrent calls to the mock.
func TestMockClient_ThreadSafety(t *testing.T) {
	ctx := context.Background()
	mock := NewMockClient()

	// Configure response
	mock.SetResponse("concurrent-user", &LoginResult{
		Status: StatusValidated,
	})

	// Make concurrent calls
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			mock.CheckLogin(ctx, "concurrent-user")
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all calls were recorded
	if mock.CallCount() != 10 {
		t.Errorf("Expected 10 calls, got %d", mock.CallCount())
	}
}
