// Copyright 2026 The JazzPetri Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package task

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	execCtx "github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/clock"
	"github.com/jazzpetri/engine/token"
)

// TestHTTPTask_Execute_Success tests successful HTTP request
func TestHTTPTask_Execute_Success(t *testing.T) {
	// Arrange - create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type header")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success"}`))
	}))
	defer server.Close()

	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test-token-1"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	task := &HTTPTask{
		Name:   "test-http",
		Method: "POST",
		URL:    server.URL,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body:    []byte(`{"test":"data"}`),
		Timeout: 5 * time.Second,
	}

	// Act
	err := task.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

// TestHTTPTask_Execute_GET tests GET request
func TestHTTPTask_Execute_GET(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":"value"}`))
	}))
	defer server.Close()

	clk := clock.NewVirtualClock(time.Now())
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	task := &HTTPTask{
		Name:   "get-data",
		Method: "GET",
		URL:    server.URL,
	}

	// Act
	err := task.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

// TestHTTPTask_Execute_ErrorStatus tests HTTP error status codes
func TestHTTPTask_Execute_ErrorStatus(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"400 Bad Request", http.StatusBadRequest},
		{"401 Unauthorized", http.StatusUnauthorized},
		{"404 Not Found", http.StatusNotFound},
		{"500 Internal Server Error", http.StatusInternalServerError},
		{"503 Service Unavailable", http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte("error message"))
			}))
			defer server.Close()

			clk := clock.NewVirtualClock(time.Now())
			ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

			task := &HTTPTask{
				Name:   "error-task",
				Method: "GET",
				URL:    server.URL,
			}

			// Act
			err := task.Execute(ctx)

			// Assert
			if err == nil {
				t.Error("Expected error for error status code")
			}
			if !strings.Contains(err.Error(), "failed with status") {
				t.Errorf("Expected status error message, got: %v", err)
			}
		})
	}
}

// TestHTTPTask_Execute_NetworkError tests network failures
func TestHTTPTask_Execute_NetworkError(t *testing.T) {
	// Arrange - use invalid URL to trigger network error
	clk := clock.NewVirtualClock(time.Now())
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	task := &HTTPTask{
		Name:    "network-error",
		Method:  "GET",
		URL:     "http://localhost:9999", // Unlikely to be listening
		Timeout: 1 * time.Second,
	}

	// Act
	err := task.Execute(ctx)

	// Assert
	if err == nil {
		t.Error("Expected network error")
	}
}

// TestHTTPTask_Execute_Timeout tests request timeout
func TestHTTPTask_Execute_Timeout(t *testing.T) {
	// Arrange - create slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	clk := clock.NewVirtualClock(time.Now())
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	task := &HTTPTask{
		Name:    "timeout-task",
		Method:  "GET",
		URL:     server.URL,
		Timeout: 100 * time.Millisecond,
	}

	// Act
	err := task.Execute(ctx)

	// Assert
	if err == nil {
		t.Error("Expected timeout error")
	}
}

// TestHTTPTask_Execute_ContextCancellation tests context cancellation
func TestHTTPTask_Execute_ContextCancellation(t *testing.T) {
	// Arrange - create slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	clk := clock.NewVirtualClock(time.Now())
	goCtx, cancel := context.WithCancel(context.Background())
	ctx := execCtx.NewExecutionContext(goCtx, clk, nil)

	task := &HTTPTask{
		Name:   "cancellable-task",
		Method: "GET",
		URL:    server.URL,
	}

	// Cancel context before request completes
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	// Act
	err := task.Execute(ctx)

	// Assert
	if err == nil {
		t.Error("Expected cancellation error")
	}
}

// TestHTTPTask_Execute_NoName tests validation
func TestHTTPTask_Execute_NoName(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	task := &HTTPTask{
		Name:   "", // Empty name
		Method: "GET",
		URL:    "http://example.com",
	}

	// Act
	err := task.Execute(ctx)

	// Assert
	if err == nil {
		t.Error("Expected validation error")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("Expected name validation error, got: %v", err)
	}
}

// TestHTTPTask_Execute_NoMethod tests validation
func TestHTTPTask_Execute_NoMethod(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	task := &HTTPTask{
		Name:   "test",
		Method: "", // Empty method
		URL:    "http://example.com",
	}

	// Act
	err := task.Execute(ctx)

	// Assert
	if err == nil {
		t.Error("Expected validation error")
	}
	if !strings.Contains(err.Error(), "method is required") {
		t.Errorf("Expected method validation error, got: %v", err)
	}
}

// TestHTTPTask_Execute_NoURL tests validation
func TestHTTPTask_Execute_NoURL(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	task := &HTTPTask{
		Name:   "test",
		Method: "GET",
		URL:    "", // Empty URL
	}

	// Act
	err := task.Execute(ctx)

	// Assert
	if err == nil {
		t.Error("Expected validation error")
	}
	if !strings.Contains(err.Error(), "url is required") {
		t.Errorf("Expected url validation error, got: %v", err)
	}
}

// TestHTTPTask_Execute_CustomHeaders tests custom headers
func TestHTTPTask_Execute_CustomHeaders(t *testing.T) {
	// Arrange
	receivedHeaders := make(map[string]string)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders["Authorization"] = r.Header.Get("Authorization")
		receivedHeaders["X-Custom"] = r.Header.Get("X-Custom")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	clk := clock.NewVirtualClock(time.Now())
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	task := &HTTPTask{
		Name:   "custom-headers",
		Method: "GET",
		URL:    server.URL,
		Headers: map[string]string{
			"Authorization": "Bearer token123",
			"X-Custom":      "custom-value",
		},
	}

	// Act
	err := task.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if receivedHeaders["Authorization"] != "Bearer token123" {
		t.Errorf("Authorization header not received correctly")
	}
	if receivedHeaders["X-Custom"] != "custom-value" {
		t.Errorf("X-Custom header not received correctly")
	}
}

// TestHTTPTask_Execute_RequestBody tests request body
func TestHTTPTask_Execute_RequestBody(t *testing.T) {
	// Arrange
	var receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	clk := clock.NewVirtualClock(time.Now())
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	expectedBody := `{"key":"value","number":42}`
	task := &HTTPTask{
		Name:   "with-body",
		Method: "POST",
		URL:    server.URL,
		Body:   []byte(expectedBody),
	}

	// Act
	err := task.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if receivedBody != expectedBody {
		t.Errorf("Expected body %s, got %s", expectedBody, receivedBody)
	}
}

// TestHTTPTask_Execute_Concurrent tests thread safety
func TestHTTPTask_Execute_Concurrent(t *testing.T) {
	// Arrange
	var requestCount int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	clk := clock.NewVirtualClock(time.Now())

	task := &HTTPTask{
		Name:   "concurrent-http",
		Method: "GET",
		URL:    server.URL,
	}

	// Act - execute concurrently
	const numGoroutines = 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)
			err := task.Execute(ctx)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		}()
	}

	wg.Wait()

	// Assert
	if requestCount != numGoroutines {
		t.Errorf("Expected %d requests, got %d", numGoroutines, requestCount)
	}
}

// TestHTTPTask_Execute_WithCustomObservability tests observability
func TestHTTPTask_Execute_WithCustomObservability(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	clk := clock.NewVirtualClock(time.Now())
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	tracer := &testTracer{}
	metrics := &testMetrics{}
	logger := &testLogger{}

	ctx = ctx.WithTracer(tracer)
	ctx = ctx.WithMetrics(metrics)
	ctx = ctx.WithLogger(logger)

	task := &HTTPTask{
		Name:   "observable-http",
		Method: "GET",
		URL:    server.URL,
	}

	// Act
	err := task.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify observability
	if tracer.spanCount != 1 {
		t.Errorf("Expected 1 span, got %d", tracer.spanCount)
	}
	if metrics.counters["task_http_executions_total"] != 1 {
		t.Errorf("Expected task_http_executions_total=1")
	}
	if metrics.counters["task_http_success_total"] != 1 {
		t.Errorf("Expected task_http_success_total=1")
	}
	if len(metrics.histograms["task_http_duration_seconds"]) != 1 {
		t.Error("Expected duration observation")
	}
}

// TestHTTPTask_Execute_CustomClient tests custom HTTP client
func TestHTTPTask_Execute_CustomClient(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	clk := clock.NewVirtualClock(time.Now())
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	customClient := &http.Client{
		Timeout: 1 * time.Second,
	}

	task := &HTTPTask{
		Name:   "custom-client",
		Method: "GET",
		URL:    server.URL,
		Client: customClient,
	}

	// Act
	err := task.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}
