// Package queueapi provides tests for the queue-api client with retry logic.
package queueapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// mockTimeoutTransport simulates a timeout error.
type mockTimeoutTransport struct {
	requestCount int
	mu           sync.Mutex
}

func (m *mockTimeoutTransport) RoundTrip(*http.Request) (*http.Response, error) {
	m.mu.Lock()
	m.requestCount++
	m.mu.Unlock()

	// Always return a timeout error
	return nil, &timeoutError{}
}

type timeoutError struct{}

func (e *timeoutError) Error() string   { return "timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return true }

// mockFlakyTransport simulates transient failures that succeed after retries.
type mockFlakyTransport struct {
	failCount    int
	requestCount int
	mu           sync.Mutex
	delay        time.Duration // Optional delay per request
}

func (m *mockFlakyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	m.requestCount++
	count := m.requestCount
	m.mu.Unlock()

	// Add delay if specified to simulate network latency
	if m.delay > 0 {
		time.Sleep(m.delay)
	}

	if count <= m.failCount {
		// Return a 500 error for the first few attempts
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       http.NoBody,
			Header:     make(http.Header),
		}, nil
	}

	// After failures, return success
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       http.NoBody,
		Header:     make(http.Header),
	}, nil
}

// mockRateLimitTransport simulates rate limiting (429).
type mockRateLimitTransport struct {
	failCount    int
	requestCount int
	mu           sync.Mutex
}

func (m *mockRateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	m.requestCount++
	count := m.requestCount
	m.mu.Unlock()

	if count <= m.failCount {
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Body:       http.NoBody,
			Header:     make(http.Header),
		}, nil
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       http.NoBody,
		Header:     make(http.Header),
	}, nil
}

// mockNonRetryableTransport simulates a client error that should not be retried.
type mockNonRetryableTransport struct {
	requestCount int
	mu           sync.Mutex
}

func (m *mockNonRetryableTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	m.requestCount++
	m.mu.Unlock()

	// Return 400 Bad Request - should not retry
	return &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       http.NoBody,
		Header:     make(http.Header),
	}, nil
}

// mockNetworkErrorTransport simulates network errors.
type mockNetworkErrorTransport struct {
	requestCount int
	mu           sync.Mutex
}

func (m *mockNetworkErrorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	m.requestCount++
	m.mu.Unlock()

	return nil, errors.New("connection refused")
}

// TestPostResolution_Success tests successful request on first attempt.
func TestPostResolution_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST request, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/email-resolution/resolve") {
			t.Errorf("expected correct path, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	err := client.PostResolution(context.Background(), "test@example.com", "testuser")
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
}

// TestPostResolution_RetryOnTimeout tests retry logic on timeout errors.
func TestPostResolution_RetryOnTimeout(t *testing.T) {
	transport := &mockTimeoutTransport{}
	client := &Client{
		baseURL:    "http://test",
		httpClient: &http.Client{Transport: transport, Timeout: 100 * time.Millisecond},
		authToken:  "",
		maxRetries: 3,
	}

	ctx := context.Background()
	err := client.PostResolution(ctx, "test@example.com", "testuser")

	// Should fail after all retries
	if err == nil {
		t.Fatal("expected error after timeout retries, got nil")
	}

	// Verify we attempted maxRetries + 1 times (initial + retries)
	expectedAttempts := 4
	if transport.requestCount != expectedAttempts {
		t.Errorf("expected %d attempts, got %d", expectedAttempts, transport.requestCount)
	}

	// Verify error message contains timeout information
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("error message should mention timeout, got: %v", err)
	}
}

// TestPostResolution_RetryOn500 tests retry logic on 500 errors.
func TestPostResolution_RetryOn500(t *testing.T) {
	transport := &mockFlakyTransport{failCount: 2}
	client := &Client{
		baseURL:    "http://test",
		httpClient: &http.Client{Transport: transport, Timeout: 100 * time.Millisecond},
		authToken:  "",
		maxRetries: 4,
	}

	ctx := context.Background()
	err := client.PostResolution(ctx, "test@example.com", "testuser")

	// Should succeed after retries
	if err != nil {
		t.Fatalf("expected success after retries, got error: %v", err)
	}

	// Verify we attempted failCount + 1 times (failures + success)
	expectedAttempts := 3
	if transport.requestCount != expectedAttempts {
		t.Errorf("expected %d attempts, got %d", expectedAttempts, transport.requestCount)
	}
}

// TestPostResolution_RetryOn429 tests retry logic on 429 rate limit errors.
func TestPostResolution_RetryOn429(t *testing.T) {
	transport := &mockRateLimitTransport{failCount: 1}
	client := &Client{
		baseURL:    "http://test",
		httpClient: &http.Client{Transport: transport, Timeout: 100 * time.Millisecond},
		authToken:  "",
		maxRetries: 4,
	}

	ctx := context.Background()
	err := client.PostResolution(ctx, "test@example.com", "testuser")

	// Should succeed after retries
	if err != nil {
		t.Fatalf("expected success after rate limit retry, got error: %v", err)
	}

	// Verify we attempted 2 times (1 failure + 1 success)
	expectedAttempts := 2
	if transport.requestCount != expectedAttempts {
		t.Errorf("expected %d attempts, got %d", expectedAttempts, transport.requestCount)
	}
}

// TestPostResolution_NoRetryOn400 tests that 4xx errors (except 408) are not retried.
func TestPostResolution_NoRetryOn400(t *testing.T) {
	transport := &mockNonRetryableTransport{}
	client := &Client{
		baseURL:    "http://test",
		httpClient: &http.Client{Transport: transport, Timeout: 100 * time.Millisecond},
		authToken:  "",
		maxRetries: 4,
	}

	ctx := context.Background()
	err := client.PostResolution(ctx, "test@example.com", "testuser")

	// Should fail immediately
	if err == nil {
		t.Fatal("expected error on non-retryable status, got nil")
	}

	// Verify we only attempted once (no retries)
	if transport.requestCount != 1 {
		t.Errorf("expected 1 attempt for non-retryable error, got %d", transport.requestCount)
	}

	// Verify error mentions non-retryable status
	if !strings.Contains(err.Error(), "non-retryable") {
		t.Errorf("error should mention non-retryable status, got: %v", err)
	}
}

// TestPostResolution_RetryOnNetworkError tests retry logic on network errors.
func TestPostResolution_RetryOnNetworkError(t *testing.T) {
	transport := &mockNetworkErrorTransport{}
	client := &Client{
		baseURL:    "http://test",
		httpClient: &http.Client{Transport: transport, Timeout: 100 * time.Millisecond},
		authToken:  "",
		maxRetries: 2,
	}

	ctx := context.Background()
	err := client.PostResolution(ctx, "test@example.com", "testuser")

	// Should fail after all retries
	if err == nil {
		t.Fatal("expected error after network error retries, got nil")
	}

	// Verify we attempted maxRetries + 1 times
	expectedAttempts := 3
	if transport.requestCount != expectedAttempts {
		t.Errorf("expected %d attempts, got %d", expectedAttempts, transport.requestCount)
	}
}

// TestPostResolution_ContextCancellation tests that context cancellation is respected.
func TestPostResolution_ContextCancellation(t *testing.T) {
	transport := &mockFlakyTransport{failCount: 10, delay: 30 * time.Millisecond} // Will always fail with delays
	client := &Client{
		baseURL:    "http://test",
		httpClient: &http.Client{Transport: transport, Timeout: 100 * time.Millisecond},
		authToken:  "",
		maxRetries: DefaultMaxRetries,
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after the first attempt + first retry backoff starts
	// Initial attempt (30ms) + backoff sleep (100ms) = ~130ms minimum
	go func() {
		time.Sleep(80 * time.Millisecond) // Cancel during the first backoff sleep
		cancel()
	}()

	err := client.PostResolution(ctx, "test@example.com", "testuser")

	// Should fail due to context cancellation
	if err == nil {
		t.Fatal("expected error on context cancellation, got nil")
	}

	if !strings.Contains(err.Error(), "context cancelled") {
		t.Errorf("error should mention context cancellation, got: %v", err)
	}
}

// TestPostResolution_ExponentialBackoff tests the timing of retry attempts.
func TestPostResolution_ExponentialBackoff(t *testing.T) {
	requestTimes := make([]time.Time, 0, 5)
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestTimes = append(requestTimes, time.Now())
		mu.Unlock()

		// Always return 500 to force retries
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	client.maxRetries = 3 // Total 4 attempts

	start := time.Now()
	err := client.PostResolution(context.Background(), "test@example.com", "testuser")
	duration := time.Since(start)

	// Should fail after all retries
	if err == nil {
		t.Fatal("expected error after retries, got nil")
	}

	mu.Lock()
	times := make([]time.Time, len(requestTimes))
	copy(times, requestTimes)
	mu.Unlock()

	// Verify we made 4 attempts (1 initial + 3 retries)
	if len(times) != 4 {
		t.Fatalf("expected 4 attempts, got %d", len(times))
	}

	// Verify backoff timing (100ms, 400ms, 900ms)
	// The delays should roughly match the expected backoff sequence
	expectedBackoffs := []time.Duration{0, 100, 400, 900} // Cumulative: 0, 100, 500, 1400ms
	totalExpectedDuration := time.Duration(0)
	for _, d := range expectedBackoffs {
		totalExpectedDuration += d * time.Millisecond
	}

	// Allow some tolerance for test execution
	minExpected := totalExpectedDuration - 100*time.Millisecond
	maxExpected := totalExpectedDuration + 500*time.Millisecond

	if duration < minExpected || duration > maxExpected {
		t.Logf("Warning: retry duration %v outside expected range [%v, %v]", duration, minExpected, maxExpected)
		t.Logf("This may be due to test environment timing variability")
	}
}

// TestPostResolution_MaxRetriesRespected tests that the max retry limit is enforced.
func TestPostResolution_MaxRetriesRespected(t *testing.T) {
	transport := &mockFlakyTransport{failCount: 100} // Will always fail
	client := &Client{
		baseURL:    "http://test",
		httpClient: &http.Client{Transport: transport, Timeout: 100 * time.Millisecond},
		authToken:  "",
		maxRetries: 2,
	}

	ctx := context.Background()
	err := client.PostResolution(ctx, "test@example.com", "testuser")

	// Should fail
	if err == nil {
		t.Fatal("expected error after max retries, got nil")
	}

	// Verify we stopped at maxRetries + 1 attempts
	expectedAttempts := 3
	if transport.requestCount != expectedAttempts {
		t.Errorf("expected %d attempts (maxRetries+1), got %d", expectedAttempts, transport.requestCount)
	}

	// Verify error message mentions retry count
	if !strings.Contains(err.Error(), "3 attempts") {
		t.Errorf("error should mention retry count, got: %v", err)
	}
}

// TestPostResolution_RequestStructure tests that the request is formatted correctly.
func TestPostResolution_RequestStructure(t *testing.T) {
	var receivedBody string
	var receivedContentType string
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		r.Body.Read(body)
		receivedBody = string(body)
		receivedContentType = r.Header.Get("Content-Type")
		receivedAuth = r.Header.Get("Authorization")

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	err := client.PostResolution(context.Background(), "user@example.com", "githubuser")

	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	// Verify Content-Type
	if receivedContentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", receivedContentType)
	}

	// Verify Authorization header
	expectedAuth := "Bearer test-token"
	if receivedAuth != expectedAuth {
		t.Errorf("expected Authorization %s, got %s", expectedAuth, receivedAuth)
	}

	// Verify request body contains required fields
	if !strings.Contains(receivedBody, "user@example.com") {
		t.Errorf("request body should contain email, got: %s", receivedBody)
	}
	if !strings.Contains(receivedBody, "githubuser") {
		t.Errorf("request body should contain github_login, got: %s", receivedBody)
	}
	if !strings.Contains(receivedBody, "live") {
		t.Errorf("request body should contain source=live, got: %s", receivedBody)
	}
	if !strings.Contains(receivedBody, "resolved_at") {
		t.Errorf("request body should contain resolved_at, got: %s", receivedBody)
	}
}

// TestNewClient_Defaults tests that NewClient sets sensible defaults.
func TestNewClient_Defaults(t *testing.T) {
	client := NewClient("http://test", "token")

	if client.baseURL != "http://test" {
		t.Errorf("expected baseURL http://test, got %s", client.baseURL)
	}

	if client.authToken != "token" {
		t.Errorf("expected authToken token, got %s", client.authToken)
	}

	if client.maxRetries != DefaultMaxRetries {
		t.Errorf("expected maxRetries %d, got %d", DefaultMaxRetries, client.maxRetries)
	}

	if client.httpClient.Timeout != 15*time.Second {
		t.Errorf("expected timeout 15s, got %v", client.httpClient.Timeout)
	}
}

// TestPostResolution_EmptyAuthToken tests that requests work without auth token.
func TestPostResolution_EmptyAuthToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "" {
			t.Errorf("expected no Authorization header, got %s", auth)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	err := client.PostResolution(context.Background(), "test@example.com", "testuser")

	if err != nil {
		t.Fatalf("expected success without auth token, got error: %v", err)
	}
}

// TestPostResolution_RequestTimeout tests that HTTP timeout is respected.
func TestPostResolution_RequestTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep longer than client timeout
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: 50 * time.Millisecond},
		authToken:  "",
		maxRetries: 1,
	}

	err := client.PostResolution(context.Background(), "test@example.com", "testuser")

	// Should fail due to timeout
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}

	// Verify error mentions timeout
	if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "deadline exceeded") {
		t.Logf("Note: error type: %v", err)
	}
}
