// Package github provides test mocks for the GitHub client.
package github

import (
	"context"
	"sync"
)

// MockClient is a mock implementation of Client for testing.
// It returns predefined responses without making HTTP calls.
type MockClient struct {
	mu             sync.Mutex
	responses      map[string]*LoginResult // login -> result mapping
	defaultResult  *LoginResult             // returned when login not in responses
	callCount      int                      // number of CheckLogin calls
	calledLogins   []string                 // logins that were checked (in order)
}

// NewMockClient creates a new mock client with no predefined responses.
// By default, it returns StatusValidated for any login.
// Use SetResponse to configure specific responses.
func NewMockClient() *MockClient {
	return &MockClient{
		responses:     make(map[string]*LoginResult),
		defaultResult: &LoginResult{Status: StatusValidated},
	}
}

// SetResponse configures the mock to return a specific result for a given login.
//
// Example:
//   mock.SetResponse("old-name", &LoginResult{
//       Status:   StatusRenamed,
//       NewLogin: strPtr("new-name"),
//   })
//   mock.SetResponse("deleted-user", &LoginResult{
//       Status: StatusDeleted,
//   })
func (m *MockClient) SetResponse(login string, result *LoginResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[login] = result
}

// SetDefaultResult configures the default result returned for logins
// without specific responses configured via SetResponse.
func (m *MockClient) SetDefaultResult(result *LoginResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaultResult = result
}

// CheckLogin checks a login and returns the configured result.
// If no specific result was configured for this login, returns the default result.
func (m *MockClient) CheckLogin(ctx context.Context, login string) (*LoginResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.callCount++
	m.calledLogins = append(m.calledLogins, login)

	if result, ok := m.responses[login]; ok {
		return result, nil
	}
	return m.defaultResult, nil
}

// CallCount returns the number of times CheckLogin was called.
func (m *MockClient) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// CalledLogins returns a list of logins that were checked, in order.
// This is useful for verifying that the worker processed logins in the expected sequence.
func (m *MockClient) CalledLogins() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Return a copy to prevent external modification
	logins := make([]string, len(m.calledLogins))
	copy(logins, m.calledLogins)
	return logins
}

// WasCalled checks if a specific login was queried.
func (m *MockClient) WasCalled(login string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, called := range m.calledLogins {
		if called == login {
			return true
		}
	}
	return false
}

// Reset clears all configured responses and call history.
func (m *MockClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses = make(map[string]*LoginResult)
	m.callCount = 0
	m.calledLogins = nil
}
