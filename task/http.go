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
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jazzpetri/engine/context"
)

// HTTPTask executes HTTP requests to external services.
// This enables integration with REST APIs, webhooks, and microservices.
//
// HTTPTask is ideal for:
//   - Calling external APIs
//   - Triggering webhooks
//   - Microservice communication
//   - Event notifications
//
// Example:
//
//	task := &HTTPTask{
//	    Name:   "notify-webhook",
//	    Method: "POST",
//	    URL:    "https://api.example.com/webhooks/order",
//	    Headers: map[string]string{
//	        "Content-Type": "application/json",
//	        "Authorization": "Bearer token123",
//	    },
//	    Body: []byte(`{"event":"order.created","order_id":"123"}`),
//	    Timeout: 30 * time.Second,
//	}
//
// The response body is read and logged but not currently returned.
// Future enhancements could store the response in the token data.
//
// Thread Safety:
// HTTPTask is safe for concurrent execution. The configuration (URL, Headers, etc.)
// is immutable after creation. Each execution creates its own HTTP request.
//
// Error Handling:
// HTTPTask returns errors for:
//   - Network failures (connection refused, timeout, DNS errors)
//   - HTTP error status codes (4xx, 5xx)
//   - Request/response processing errors
type HTTPTask struct {
	// Name is the task identifier used in logs, traces, and metrics.
	Name string

	// Method is the HTTP method (GET, POST, PUT, DELETE, etc.).
	Method string

	// URL is the target endpoint URL.
	// Must be a valid HTTP/HTTPS URL.
	URL string

	// Headers are HTTP headers to include in the request.
	// Common headers: Content-Type, Authorization, Accept
	Headers map[string]string

	// Body is the request body for POST/PUT/PATCH requests.
	// For GET/DELETE requests, this should be nil.
	Body []byte

	// Timeout is the maximum duration for the HTTP request.
	// If zero, defaults to 30 seconds.
	// The timeout includes connection, request, and response time.
	Timeout time.Duration

	// Client is an optional custom HTTP client.
	// If nil, a default client with the specified Timeout is used.
	// This allows custom transport, TLS, proxy configuration, etc.
	Client *http.Client
}

// Execute performs the HTTP request with observability and error handling.
//
// Execution flow:
//  1. Validate task configuration
//  2. Start trace span
//  3. Create HTTP request
//  4. Apply headers
//  5. Execute request with timeout and cancellation support
//  6. Read response body
//  7. Check HTTP status code
//  8. Record metrics and logs
//
// The task respects ctx.Context cancellation, allowing graceful shutdown
// of in-flight HTTP requests.
//
// Thread Safety:
// Execute is safe to call concurrently from multiple goroutines.
//
// Returns an error if:
//   - Task configuration is invalid (missing URL, Method, Name)
//   - HTTP request fails (network error, timeout)
//   - Response status code indicates error (4xx, 5xx)
//   - Context is cancelled before request completes
func (h *HTTPTask) Execute(ctx *context.ExecutionContext) error {
	// Validate configuration
	if h.Name == "" {
		return fmt.Errorf("http task: name is required")
	}
	if h.Method == "" {
		return fmt.Errorf("http task: method is required")
	}
	if h.URL == "" {
		return fmt.Errorf("http task: url is required")
	}

	// Start trace span
	span := ctx.Tracer.StartSpan("task.http." + h.Name)
	defer span.End()

	span.SetAttribute("http.method", h.Method)
	span.SetAttribute("http.url", h.URL)
	span.SetAttribute("task.name", h.Name)

	// Record metrics
	ctx.Metrics.Inc("task_executions_total")
	ctx.Metrics.Inc("task_http_executions_total")

	// Log task start
	ctx.Logger.Debug("Executing HTTP task", map[string]interface{}{
		"task_name":   h.Name,
		"http_method": h.Method,
		"http_url":    h.URL,
		"token_id":    tokenID(ctx),
	})

	// Create HTTP request
	var bodyReader io.Reader
	if h.Body != nil {
		bodyReader = bytes.NewReader(h.Body)
	}

	req, err := http.NewRequestWithContext(ctx.Context, h.Method, h.URL, bodyReader)
	if err != nil {
		return h.handleError(ctx, span, fmt.Errorf("failed to create request: %w", err))
	}

	// Apply headers
	for key, value := range h.Headers {
		req.Header.Set(key, value)
	}

	// Create or use HTTP client
	client := h.Client
	if client == nil {
		timeout := h.Timeout
		if timeout == 0 {
			timeout = 30 * time.Second
		}
		client = &http.Client{
			Timeout: timeout,
		}
	}

	// Execute request
	startTime := ctx.Clock.Now()
	resp, err := client.Do(req)
	duration := ctx.Clock.Now().Sub(startTime).Seconds()

	// Record duration metric
	ctx.Metrics.Observe("task_http_duration_seconds", duration)

	if err != nil {
		return h.handleError(ctx, span, fmt.Errorf("http request failed: %w", err))
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return h.handleError(ctx, span, fmt.Errorf("failed to read response: %w", err))
	}

	span.SetAttribute("http.status_code", resp.StatusCode)
	span.SetAttribute("http.response_size", len(respBody))

	// Check status code
	if resp.StatusCode >= 400 {
		err := fmt.Errorf("http request failed with status %d: %s", resp.StatusCode, string(respBody))
		return h.handleError(ctx, span, err)
	}

	// Success
	ctx.Metrics.Inc("task_success_total")
	ctx.Metrics.Inc("task_http_success_total")

	ctx.Logger.Debug("HTTP task completed", map[string]interface{}{
		"task_name":       h.Name,
		"http_status":     resp.StatusCode,
		"response_size":   len(respBody),
		"duration_ms":     duration * 1000,
		"token_id":        tokenID(ctx),
	})

	return nil
}

// handleError processes errors with observability and error handling
func (h *HTTPTask) handleError(ctx *context.ExecutionContext, span context.Span, err error) error {
	span.RecordError(err)
	ctx.Metrics.Inc("task_errors_total")
	ctx.Metrics.Inc("task_http_errors_total")
	ctx.ErrorRecorder.RecordError(err, map[string]interface{}{
		"task_name":   h.Name,
		"http_method": h.Method,
		"http_url":    h.URL,
		"token_id":    tokenID(ctx),
	})
	ctx.Logger.Error("HTTP task failed", map[string]interface{}{
		"task_name":   h.Name,
		"http_method": h.Method,
		"http_url":    h.URL,
		"error":       err.Error(),
		"token_id":    tokenID(ctx),
	})
	return ctx.ErrorHandler.Handle(ctx, err)
}
