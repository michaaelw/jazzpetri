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

package context

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jazzpetri/engine/clock"
)

// TestNoOpErrorHandler verifies that NoOpErrorHandler passes through errors unchanged.
func TestNoOpErrorHandler(t *testing.T) {
	handler := &NoOpErrorHandler{}
	execCtx := NewExecutionContext(context.Background(), clock.NewRealTimeClock(), nil)

	testErr := errors.New("test error")
	result := handler.Handle(execCtx, testErr)

	// NoOpErrorHandler should return the error unchanged
	if result != testErr {
		t.Error("NoOpErrorHandler should return error unchanged")
	}
}

// TestNoOpErrorHandler_NilError verifies NoOpErrorHandler handles nil errors.
func TestNoOpErrorHandler_NilError(t *testing.T) {
	handler := &NoOpErrorHandler{}
	execCtx := NewExecutionContext(context.Background(), clock.NewRealTimeClock(), nil)

	result := handler.Handle(execCtx, nil)

	if result != nil {
		t.Error("NoOpErrorHandler should return nil when given nil")
	}
}

// TestNoOpErrorRecorder verifies that NoOpErrorRecorder methods execute without panic.
func TestNoOpErrorRecorder(t *testing.T) {
	recorder := &NoOpErrorRecorder{}

	// All methods should execute without panic
	recorder.RecordError(errors.New("test error"), map[string]interface{}{
		"workflow_id": "wf-123",
		"transition":  "t1",
	})

	recorder.RecordRetry(3, 5*time.Second)
	recorder.RecordRetry(1, time.Now().Add(time.Minute))

	recorder.RecordCompensation("refund-payment")
	recorder.RecordCompensation("rollback-transaction")

	// No assertions needed - just verify no panic
}

// TestNoOpErrorRecorder_NilMetadata verifies NoOpErrorRecorder handles nil metadata.
func TestNoOpErrorRecorder_NilMetadata(t *testing.T) {
	recorder := &NoOpErrorRecorder{}

	// Should not panic with nil metadata
	recorder.RecordError(errors.New("test"), nil)
}

// TestMockErrorHandler verifies the mock error handler implementation.
func TestMockErrorHandler(t *testing.T) {
	handler := &mockErrorHandler{}
	execCtx := NewExecutionContext(context.Background(), clock.NewRealTimeClock(), nil)

	// Handle multiple errors
	err1 := errors.New("error 1")
	err2 := errors.New("error 2")

	result1 := handler.Handle(execCtx, err1)
	result2 := handler.Handle(execCtx, err2)

	// Verify errors were recorded
	if len(handler.handledErrors) != 2 {
		t.Errorf("Expected 2 handled errors, got %d", len(handler.handledErrors))
	}

	if handler.handledErrors[0] != err1 {
		t.Error("First error not recorded correctly")
	}

	if handler.handledErrors[1] != err2 {
		t.Error("Second error not recorded correctly")
	}

	// Mock handler returns errors unchanged
	if result1 != err1 {
		t.Error("Handler should return first error unchanged")
	}

	if result2 != err2 {
		t.Error("Handler should return second error unchanged")
	}
}

// TestMockErrorRecorder verifies the mock error recorder implementation.
func TestMockErrorRecorder(t *testing.T) {
	recorder := &mockErrorRecorder{}

	// Record errors
	err1 := errors.New("error 1")
	err2 := errors.New("error 2")

	recorder.RecordError(err1, map[string]interface{}{"key": "value1"})
	recorder.RecordError(err2, map[string]interface{}{"key": "value2"})

	if len(recorder.errors) != 2 {
		t.Errorf("Expected 2 recorded errors, got %d", len(recorder.errors))
	}

	if recorder.errors[0] != err1 {
		t.Error("First error not recorded correctly")
	}

	// Record retries
	recorder.RecordRetry(1, 1*time.Second)
	recorder.RecordRetry(2, 2*time.Second)
	recorder.RecordRetry(3, 4*time.Second)

	if len(recorder.retries) != 3 {
		t.Errorf("Expected 3 retries, got %d", len(recorder.retries))
	}

	if recorder.retries[0] != 1 || recorder.retries[1] != 2 || recorder.retries[2] != 3 {
		t.Error("Retry attempts not recorded correctly")
	}

	// Record compensations
	recorder.RecordCompensation("undo-action-1")
	recorder.RecordCompensation("undo-action-2")

	if len(recorder.compensations) != 2 {
		t.Errorf("Expected 2 compensations, got %d", len(recorder.compensations))
	}

	if recorder.compensations[0] != "undo-action-1" {
		t.Error("First compensation not recorded correctly")
	}

	if recorder.compensations[1] != "undo-action-2" {
		t.Error("Second compensation not recorded correctly")
	}
}

// TestErrorHandlerIntegration tests error handler integration with ExecutionContext.
func TestErrorHandlerIntegration(t *testing.T) {
	handler := &mockErrorHandler{}
	recorder := &mockErrorRecorder{}

	execCtx := NewExecutionContext(context.Background(), clock.NewRealTimeClock(), nil)
	execCtx = execCtx.WithErrorHandler(handler)
	execCtx = execCtx.WithErrorRecorder(recorder)

	// Simulate error handling
	testErr := errors.New("task failed")

	// Record the error
	execCtx.ErrorRecorder.RecordError(testErr, map[string]interface{}{
		"transition": "t1",
		"attempt":    1,
	})

	// Handle the error
	result := execCtx.ErrorHandler.Handle(execCtx, testErr)

	// Verify error was recorded
	if len(recorder.errors) != 1 {
		t.Error("Error should have been recorded")
	}

	// Verify error was handled
	if len(handler.handledErrors) != 1 {
		t.Error("Error should have been handled")
	}

	// Mock handler returns error unchanged
	if result != testErr {
		t.Error("Handler should return error")
	}
}

// TestErrorRecorderRetryWithDifferentTypes tests RecordRetry with different nextRetry types.
func TestErrorRecorderRetryWithDifferentTypes(t *testing.T) {
	recorder := &mockErrorRecorder{}

	// Test with duration
	recorder.RecordRetry(1, 5*time.Second)

	// Test with time.Time
	nextTime := time.Now().Add(10 * time.Second)
	recorder.RecordRetry(2, nextTime)

	// Test with int (edge case)
	recorder.RecordRetry(3, 15)

	// Verify all retries recorded
	if len(recorder.retries) != 3 {
		t.Errorf("Expected 3 retries, got %d", len(recorder.retries))
	}
}

// BenchmarkNoOpErrorHandler verifies zero allocations for error handler.
func BenchmarkNoOpErrorHandler(b *testing.B) {
	handler := &NoOpErrorHandler{}
	execCtx := NewExecutionContext(context.Background(), clock.NewRealTimeClock(), nil)
	testErr := errors.New("test error")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = handler.Handle(execCtx, testErr)
	}
}

// BenchmarkNoOpErrorRecorder verifies zero allocations for error recorder.
func BenchmarkNoOpErrorRecorder(b *testing.B) {
	recorder := &NoOpErrorRecorder{}
	testErr := errors.New("test error")
	metadata := map[string]interface{}{"key": "value"}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		recorder.RecordError(testErr, metadata)
		recorder.RecordRetry(1, 5*time.Second)
		recorder.RecordCompensation("undo-action")
	}
}

// TestCustomErrorHandler demonstrates custom error handler implementation.
func TestCustomErrorHandler(t *testing.T) {
	// Custom handler that transforms errors
	transformHandler := &customErrorHandler{
		transform: func(err error) error {
			return errors.New("transformed: " + err.Error())
		},
	}

	execCtx := NewExecutionContext(context.Background(), clock.NewRealTimeClock(), nil)
	testErr := errors.New("original error")

	result := transformHandler.Handle(execCtx, testErr)

	expected := "transformed: original error"
	if result.Error() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result.Error())
	}
}

// customErrorHandler is a test helper for custom error handling logic.
type customErrorHandler struct {
	transform func(error) error
}

func (c *customErrorHandler) Handle(ctx *ExecutionContext, err error) error {
	if c.transform != nil {
		return c.transform(err)
	}
	return err
}

// TestErrorHandlerSuppression tests error suppression pattern.
func TestErrorHandlerSuppression(t *testing.T) {
	// Handler that suppresses errors (returns nil)
	suppressHandler := &customErrorHandler{
		transform: func(err error) error {
			return nil // Suppress error
		},
	}

	execCtx := NewExecutionContext(context.Background(), clock.NewRealTimeClock(), nil)
	testErr := errors.New("error to suppress")

	result := suppressHandler.Handle(execCtx, testErr)

	if result != nil {
		t.Error("Handler should suppress error by returning nil")
	}
}
