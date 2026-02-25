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
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	execCtx "github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/clock"
	"github.com/jazzpetri/engine/token"
)

// TestInlineTask_Execute_Success tests successful execution of inline task
func TestInlineTask_Execute_Success(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test-token-1"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	executed := false
	task := &InlineTask{
		Name: "test-task",
		Fn: func(ctx *execCtx.ExecutionContext) error {
			executed = true
			return nil
		},
	}

	// Act
	err := task.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if !executed {
		t.Error("Expected task function to be executed")
	}
}

// TestInlineTask_Execute_Error tests error handling
func TestInlineTask_Execute_Error(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test-token-1"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	expectedErr := errors.New("task failed")
	task := &InlineTask{
		Name: "failing-task",
		Fn: func(ctx *execCtx.ExecutionContext) error {
			return expectedErr
		},
	}

	// Act
	err := task.Execute(ctx)

	// Assert
	if err == nil {
		t.Error("Expected error, got nil")
	}
	// NoOpErrorHandler passes through errors unchanged
	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}

// TestInlineTask_Execute_Panic tests panic recovery
func TestInlineTask_Execute_Panic(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test-token-1"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	task := &InlineTask{
		Name: "panicking-task",
		Fn: func(ctx *execCtx.ExecutionContext) error {
			panic("something went wrong")
		},
	}

	// Act
	err := task.Execute(ctx)

	// Assert
	if err == nil {
		t.Error("Expected error from panic, got nil")
	}
	if !strings.Contains(err.Error(), "panicked") {
		t.Errorf("Expected panic error message, got: %v", err)
	}
	if !strings.Contains(err.Error(), "something went wrong") {
		t.Errorf("Expected panic message in error, got: %v", err)
	}
}

// TestInlineTask_Execute_NoName tests validation of task name
func TestInlineTask_Execute_NoName(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test-token-1"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	task := &InlineTask{
		Name: "", // Empty name
		Fn: func(ctx *execCtx.ExecutionContext) error {
			return nil
		},
	}

	// Act
	err := task.Execute(ctx)

	// Assert
	if err == nil {
		t.Error("Expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("Expected name validation error, got: %v", err)
	}
}

// TestInlineTask_Execute_NoFunction tests validation of task function
func TestInlineTask_Execute_NoFunction(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test-token-1"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	task := &InlineTask{
		Name: "test-task",
		Fn:   nil, // No function
	}

	// Act
	err := task.Execute(ctx)

	// Assert
	if err == nil {
		t.Error("Expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "function is required") {
		t.Errorf("Expected function validation error, got: %v", err)
	}
}

// TestInlineTask_Execute_TokenAccess tests access to token data
func TestInlineTask_Execute_TokenAccess(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{
		ID:   "test-token-1",
		Data: &testTokenData{Value: 42},
	}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	var capturedValue int
	task := &InlineTask{
		Name: "token-access-task",
		Fn: func(ctx *execCtx.ExecutionContext) error {
			data := ctx.Token.Data.(*testTokenData)
			capturedValue = data.Value
			return nil
		},
	}

	// Act
	err := task.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if capturedValue != 42 {
		t.Errorf("Expected value 42, got %d", capturedValue)
	}
}

// TestInlineTask_Execute_NilToken tests execution with nil token
func TestInlineTask_Execute_NilToken(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil) // Nil token

	executed := false
	task := &InlineTask{
		Name: "nil-token-task",
		Fn: func(ctx *execCtx.ExecutionContext) error {
			executed = true
			if ctx.Token != nil {
				t.Error("Expected nil token")
			}
			return nil
		},
	}

	// Act
	err := task.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error with nil token, got: %v", err)
	}
	if !executed {
		t.Error("Expected task to execute with nil token")
	}
}

// TestInlineTask_Execute_Concurrent tests thread safety
func TestInlineTask_Execute_Concurrent(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())

	var counter int
	var mu sync.Mutex

	task := &InlineTask{
		Name: "concurrent-task",
		Fn: func(ctx *execCtx.ExecutionContext) error {
			mu.Lock()
			defer mu.Unlock()
			counter++
			return nil
		},
	}

	// Act - execute concurrently
	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			tok := &token.Token{ID: string(rune('0' + id))}
			ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)
			err := task.Execute(ctx)
			if err != nil {
				t.Errorf("Goroutine %d: unexpected error: %v", id, err)
			}
		}(i)
	}

	wg.Wait()

	// Assert
	if counter != numGoroutines {
		t.Errorf("Expected counter=%d, got %d", numGoroutines, counter)
	}
}

// TestInlineTask_Execute_WithCustomObservability tests with custom observability
func TestInlineTask_Execute_WithCustomObservability(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test-token-1"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	// Use test observability implementations
	tracer := &testTracer{}
	metrics := &testMetrics{}
	logger := &testLogger{}

	ctx = ctx.WithTracer(tracer)
	ctx = ctx.WithMetrics(metrics)
	ctx = ctx.WithLogger(logger)

	task := &InlineTask{
		Name: "observable-task",
		Fn: func(ctx *execCtx.ExecutionContext) error {
			return nil
		},
	}

	// Act
	err := task.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify observability calls
	if tracer.spanCount != 1 {
		t.Errorf("Expected 1 span, got %d", tracer.spanCount)
	}
	if metrics.counters["task_executions_total"] != 1 {
		t.Errorf("Expected task_executions_total=1, got %f",
			metrics.counters["task_executions_total"])
	}
	if metrics.counters["task_inline_executions_total"] != 1 {
		t.Errorf("Expected task_inline_executions_total=1, got %f",
			metrics.counters["task_inline_executions_total"])
	}
	if metrics.counters["task_success_total"] != 1 {
		t.Errorf("Expected task_success_total=1, got %f",
			metrics.counters["task_success_total"])
	}
	if logger.debugCount < 2 {
		t.Errorf("Expected at least 2 debug logs, got %d", logger.debugCount)
	}
}

// TestInlineTask_Execute_WithCustomObservability_Error tests observability on error
func TestInlineTask_Execute_WithCustomObservability_Error(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test-token-1"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	tracer := &testTracer{}
	metrics := &testMetrics{}
	logger := &testLogger{}
	recorder := &testErrorRecorder{}

	ctx = ctx.WithTracer(tracer)
	ctx = ctx.WithMetrics(metrics)
	ctx = ctx.WithLogger(logger)
	ctx = ctx.WithErrorRecorder(recorder)

	expectedErr := errors.New("task error")
	task := &InlineTask{
		Name: "failing-task",
		Fn: func(ctx *execCtx.ExecutionContext) error {
			return expectedErr
		},
	}

	// Act
	err := task.Execute(ctx)

	// Assert
	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}

	// Verify error metrics
	if metrics.counters["task_errors_total"] != 1 {
		t.Errorf("Expected task_errors_total=1, got %f",
			metrics.counters["task_errors_total"])
	}
	if metrics.counters["task_inline_errors_total"] != 1 {
		t.Errorf("Expected task_inline_errors_total=1, got %f",
			metrics.counters["task_inline_errors_total"])
	}
	if logger.errorCount != 1 {
		t.Errorf("Expected 1 error log, got %d", logger.errorCount)
	}
	if recorder.errorCount != 1 {
		t.Errorf("Expected 1 recorded error, got %d", recorder.errorCount)
	}
	if tracer.lastSpan == nil || tracer.lastSpan.errorCount != 1 {
		t.Error("Expected span to record error")
	}
}

// testTokenData is a simple token data type for testing
type testTokenData struct {
	Value int
}

func (t *testTokenData) Type() string {
	return "test-token"
}

// Test observability implementations
type testTracer struct {
	spanCount int
	lastSpan  *testSpan
}

func (t *testTracer) StartSpan(name string) execCtx.Span {
	t.spanCount++
	span := &testSpan{name: name}
	t.lastSpan = span
	return span
}

type testSpan struct {
	name       string
	ended      bool
	errorCount int
	attributes map[string]interface{}
}

func (s *testSpan) End() {
	s.ended = true
}

func (s *testSpan) SetAttribute(key string, value interface{}) {
	if s.attributes == nil {
		s.attributes = make(map[string]interface{})
	}
	s.attributes[key] = value
}

func (s *testSpan) RecordError(err error) {
	s.errorCount++
}

type testMetrics struct {
	mu        sync.Mutex
	counters  map[string]float64
	gauges    map[string]float64
	histograms map[string][]float64
}

func (m *testMetrics) Inc(name string) {
	m.Add(name, 1)
}

func (m *testMetrics) Add(name string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.counters == nil {
		m.counters = make(map[string]float64)
	}
	m.counters[name] += value
}

func (m *testMetrics) Observe(name string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.histograms == nil {
		m.histograms = make(map[string][]float64)
	}
	m.histograms[name] = append(m.histograms[name], value)
}

func (m *testMetrics) Set(name string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.gauges == nil {
		m.gauges = make(map[string]float64)
	}
	m.gauges[name] = value
}

type testLogger struct {
	mu         sync.Mutex
	debugCount int
	infoCount  int
	warnCount  int
	errorCount int
}

func (l *testLogger) Debug(msg string, fields map[string]interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.debugCount++
}

func (l *testLogger) Info(msg string, fields map[string]interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.infoCount++
}

func (l *testLogger) Warn(msg string, fields map[string]interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.warnCount++
}

func (l *testLogger) Error(msg string, fields map[string]interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errorCount++
}

type testErrorRecorder struct {
	mu         sync.Mutex
	errorCount int
	errors     []error
}

func (r *testErrorRecorder) RecordError(err error, metadata map[string]interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errorCount++
	r.errors = append(r.errors, err)
}

func (r *testErrorRecorder) RecordRetry(attempt int, nextRetry interface{}) {}

func (r *testErrorRecorder) RecordCompensation(action string) {}
