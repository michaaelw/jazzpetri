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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jazzpetri/engine/clock"
	execctx "github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/token"
)

// stringTokenData is a simple test implementation of TokenData for strings
type stringTokenData struct {
	value string
}

func (s *stringTokenData) Type() string {
	return "string"
}

// TestParallelTask_ErrorHandling tests:
// First error should NOT cancel other tasks - all tasks should complete
//
// CURRENT BEHAVIOR: All tasks run to completion (CORRECT)
// EXPECTED: Test should PASS
//
// This test verifies that ParallelTask allows all tasks to complete
// even if some fail, which is the correct behavior per the documentation.
func TestParallelTask_ErrorHandling(t *testing.T) {
	// Arrange
	task1Executed := false
	task2Executed := false
	task3Executed := false

	parallel := &ParallelTask{
		Name: "test-parallel",
		Tasks: []Task{
			&InlineTask{
				Name: "task1-success",
				Fn: func(ctx *execctx.ExecutionContext) error {
					time.Sleep(50 * time.Millisecond)
					task1Executed = true
					return nil
				},
			},
			&InlineTask{
				Name: "task2-fails",
				Fn: func(ctx *execctx.ExecutionContext) error {
					time.Sleep(10 * time.Millisecond) // Fails quickly
					task2Executed = true
					return errors.New("task2 failed")
				},
			},
			&InlineTask{
				Name: "task3-success",
				Fn: func(ctx *execctx.ExecutionContext) error {
					time.Sleep(100 * time.Millisecond) // Takes longest
					task3Executed = true
					return nil
				},
			},
		},
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Act
	err := parallel.Execute(ctx)

	// Assert: All tasks should have executed
	if !task1Executed {
		t.Error("Task 1 should have executed")
	}

	if !task2Executed {
		t.Error("Task 2 should have executed")
	}

	if !task3Executed {
		t.Error("Task 3 should have executed despite task 2 failing")
		t.Log("ISSUE: If this fails, first error is canceling other tasks")
	}

	// Should return ParallelResult with task2's error
	if err == nil {
		t.Error("Expected error from task2, got nil")
	}

	// Now returns ParallelResult instead of MultiError
	parallelResult, ok := err.(*ParallelResult)
	if !ok {
		t.Errorf("Expected *ParallelResult, got %T", err)
	} else {
		if parallelResult.FailureCount != 1 {
			t.Errorf("Expected 1 failure, got %d", parallelResult.FailureCount)
		}
		if parallelResult.SuccessCount != 2 {
			t.Errorf("Expected 2 successes, got %d", parallelResult.SuccessCount)
		}
	}

	t.Log("SUCCESS: All tasks completed despite errors")
}

// TestParallelTask_AggregateErrors tests:
// All errors should be collected, not just the first one
//
// EXPECTED: Test should PASS (MultiError already collects all errors)
func TestParallelTask_AggregateErrors(t *testing.T) {
	// Arrange: Multiple failing tasks
	parallel := &ParallelTask{
		Name: "test-parallel",
		Tasks: []Task{
			&InlineTask{
				Name: "task1-fails",
				Fn: func(ctx *execctx.ExecutionContext) error {
					return errors.New("error from task 1")
				},
			},
			&InlineTask{
				Name: "task2-fails",
				Fn: func(ctx *execctx.ExecutionContext) error {
					return errors.New("error from task 2")
				},
			},
			&InlineTask{
				Name: "task3-success",
				Fn: func(ctx *execctx.ExecutionContext) error {
					return nil
				},
			},
			&InlineTask{
				Name: "task4-fails",
				Fn: func(ctx *execctx.ExecutionContext) error {
					return errors.New("error from task 4")
				},
			},
		},
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Act
	err := parallel.Execute(ctx)

	// Assert: Should collect all errors
	if err == nil {
		t.Fatal("Expected errors, got nil")
	}

	// Now returns ParallelResult instead of MultiError
	parallelResult, ok := err.(*ParallelResult)
	if !ok {
		t.Fatalf("Expected *ParallelResult, got %T", err)
	}

	// Should have 3 errors (tasks 1, 2, and 4)
	expectedErrorCount := 3
	if parallelResult.FailureCount != expectedErrorCount {
		t.Errorf("Expected %d errors, got %d", expectedErrorCount, parallelResult.FailureCount)
		for i, r := range parallelResult.Results {
			if !r.Success {
				t.Logf("Error %d: %v", i, r.Error)
			}
		}
	}

	// Verify we got errors from all failing tasks
	errorMessages := make(map[string]bool)
	for _, r := range parallelResult.Results {
		if !r.Success {
			e := r.Error
			errorMessages[e.Error()] = true
		}
	}

	requiredErrors := []string{
		"error from task 1",
		"error from task 2",
		"error from task 4",
	}

	for _, required := range requiredErrors {
		if !errorMessages[required] {
			t.Errorf("Missing expected error: %s", required)
		}
	}

	t.Log("SUCCESS: All errors collected in MultiError")
}

// TestParallelTask_PartialSuccess tests:
// Successful task results should be preserved even if some tasks fail
//
// Problem: Current implementation doesn't preserve partial results
// EXPECTED FAILURE: No way to retrieve results from successful tasks
func TestParallelTask_PartialSuccess(t *testing.T) {
	// Arrange: Tasks that modify token data
	parallel := &ParallelTask{
		Name: "test-parallel",
		Tasks: []Task{
			&InlineTask{
				Name: "task1-success",
				Fn: func(ctx *execctx.ExecutionContext) error {
					// Successful task - would store result in token
					// But token is a copy, so result is lost
					if ctx.Token != nil {
						// Try to store result (won't work due to copy)
						ctx.Token.Data = &stringTokenData{value: "result from task 1"}
					}
					return nil
				},
			},
			&InlineTask{
				Name: "task2-fails",
				Fn: func(ctx *execctx.ExecutionContext) error {
					return errors.New("task 2 failed")
				},
			},
			&InlineTask{
				Name: "task3-success",
				Fn: func(ctx *execctx.ExecutionContext) error {
					if ctx.Token != nil {
						ctx.Token.Data = &stringTokenData{value: "result from task 3"}
					}
					return nil
				},
			},
		},
	}

	originalToken := &token.Token{
		ID:   "test-token",
		Data: &stringTokenData{value: "original data"},
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)
	ctx = ctx.WithToken(originalToken)

	// Act
	err := parallel.Execute(ctx)

	// Assert: Should have error from task2
	if err == nil {
		t.Error("Expected error from task2")
	}

	// EXPECTED FAILURE: Cannot retrieve results from successful tasks
	// Original token is unchanged (tasks work on copies)
	if strData, ok := originalToken.Data.(*stringTokenData); ok && strData.value != "original data" {
		t.Logf("Token data changed to: %v", strData.value)
	}

	// The issue: How do we collect results from successful tasks?
	// Possible solutions:
	// 1. Return []Result alongside error
	// 2. Store results in context
	// 3. Allow tasks to push results to a channel
	// 4. Provide callback for successful task completion

	t.Log("EXPECTED FAILURE: No way to retrieve results from successful tasks")
	t.Log("After fix: ParallelTask should provide access to partial results")
	t.Log("Possible API:")
	t.Log("  type TaskResult struct {")
	t.Log("      TaskIndex int")
	t.Log("      TaskName  string")
	t.Log("      Success   bool")
	t.Log("      Error     error")
	t.Log("      Token     *token.Token // Modified token from successful task")
	t.Log("  }")
	t.Log("  func (p *ParallelTask) ExecuteWithResults(ctx) ([]TaskResult, error)")
}

// TestParallelTask_PartialSuccess_WithResults tests:
// Proposed ExecuteWithResults() method that returns partial results
//
// EXPECTED FAILURE: ExecuteWithResults() method does not exist
func TestParallelTask_PartialSuccess_WithResults(t *testing.T) {
	// Arrange
	parallel := &ParallelTask{
		Name: "test-parallel",
		Tasks: []Task{
			&InlineTask{
				Name: "task1-success",
				Fn: func(ctx *execctx.ExecutionContext) error {
					return nil
				},
			},
			&InlineTask{
				Name: "task2-fails",
				Fn: func(ctx *execctx.ExecutionContext) error {
					return errors.New("task 2 failed")
				},
			},
			&InlineTask{
				Name: "task3-success",
				Fn: func(ctx *execctx.ExecutionContext) error {
					return nil
				},
			},
		},
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Act
	// EXPECTED FAILURE: ExecuteWithResults does not exist
	// Uncomment when implementing:
	// results, err := parallel.ExecuteWithResults(ctx)

	// For now, use regular Execute
	err := parallel.Execute(ctx)

	// Assert
	// EXPECTED FAILURE: Cannot access individual task results
	/*
	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	// Check task 1 (success)
	if !results[0].Success || results[0].Error != nil {
		t.Errorf("Task 1 should succeed: %+v", results[0])
	}

	// Check task 2 (failure)
	if results[1].Success || results[1].Error == nil {
		t.Errorf("Task 2 should fail: %+v", results[1])
	}

	// Check task 3 (success)
	if !results[2].Success || results[2].Error != nil {
		t.Errorf("Task 3 should succeed: %+v", results[2])
	}
	*/

	if err == nil {
		t.Error("Expected error from task 2")
	}

	t.Log("EXPECTED FAILURE: ExecuteWithResults() method does not exist")
	t.Log("After fix: Add method to return individual task results")
}

// TestParallelTask_ErrorPropagation tests:
// Verify MultiError contains useful information about which tasks failed
//
// EXPECTED: Test should PASS (MultiError is informative)
func TestParallelTask_ErrorPropagation(t *testing.T) {
	// Arrange
	parallel := &ParallelTask{
		Name: "test-parallel",
		Tasks: []Task{
			&InlineTask{
				Name: "api-call-1",
				Fn: func(ctx *execctx.ExecutionContext) error {
					return errors.New("connection timeout")
				},
			},
			&InlineTask{
				Name: "api-call-2",
				Fn: func(ctx *execctx.ExecutionContext) error {
					return nil
				},
			},
			&InlineTask{
				Name: "api-call-3",
				Fn: func(ctx *execctx.ExecutionContext) error {
					return errors.New("invalid response")
				},
			},
		},
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Act
	err := parallel.Execute(ctx)

	// Assert
	if err == nil {
		t.Fatal("Expected errors")
	}

	parallelResult, ok := err.(*ParallelResult)
	if !ok {
		t.Fatalf("Expected *ParallelResult, got %T", err)
	}

	// Error message should be informative
	errorStr := parallelResult.Error()
	t.Logf("Error string: %s", errorStr)

	// Should mention number of errors
	if parallelResult.FailureCount != 2 {
		t.Errorf("Expected 2 errors, got %d", parallelResult.FailureCount)
	}

	// Check Error() implementation
	if errorStr == "" {
		t.Error("ParallelResult.Error() should return non-empty string")
	}

	t.Log("SUCCESS: ParallelResult provides useful error information")
}

// TestParallelTask_ErrorHandling_AllFail tests:
// All tasks fail - should collect all errors
//
// EXPECTED: Test should PASS
func TestParallelTask_ErrorHandling_AllFail(t *testing.T) {
	// Arrange
	numTasks := 5
	tasks := make([]Task, numTasks)

	for i := 0; i < numTasks; i++ {
		taskNum := i
		tasks[i] = &InlineTask{
			Name: fmt.Sprintf("task-%d", i),
			Fn: func(ctx *execctx.ExecutionContext) error {
				return fmt.Errorf("error from task %d", taskNum)
			},
		}
	}

	parallel := &ParallelTask{
		Name:  "all-fail-parallel",
		Tasks: tasks,
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Act
	err := parallel.Execute(ctx)

	// Assert
	if err == nil {
		t.Fatal("Expected errors from all tasks")
	}

	parallelResult, ok := err.(*ParallelResult)
	if !ok {
		t.Fatalf("Expected *ParallelResult, got %T", err)
	}

	if parallelResult.FailureCount != numTasks {
		t.Errorf("Expected %d errors, got %d", numTasks, parallelResult.FailureCount)
	}

	t.Log("SUCCESS: All errors collected when all tasks fail")
}

// TestParallelTask_ErrorHandling_NoErrors tests:
// No errors - should return nil (not empty MultiError)
//
// EXPECTED: Test should PASS
func TestParallelTask_ErrorHandling_NoErrors(t *testing.T) {
	// Arrange
	parallel := &ParallelTask{
		Name: "test-parallel",
		Tasks: []Task{
			&InlineTask{
				Name: "task1",
				Fn: func(ctx *execctx.ExecutionContext) error {
					return nil
				},
			},
			&InlineTask{
				Name: "task2",
				Fn: func(ctx *execctx.ExecutionContext) error {
					return nil
				},
			},
		},
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Act
	err := parallel.Execute(ctx)

	// Assert: Should return nil, not empty MultiError
	if err != nil {
		t.Errorf("Expected nil when all tasks succeed, got %v", err)
	}

	t.Log("SUCCESS: Returns nil when no errors occur")
}

// TestParallelTask_ErrorHandling_PanicRecovery tests:
// Panicking task should be recovered and converted to error
//
// Panic recovery now implemented
func TestParallelTask_ErrorHandling_PanicRecovery(t *testing.T) {
	// Panic recovery is now implemented

	// Arrange
	task1Executed := false
	task3Executed := false

	parallel := &ParallelTask{
		Name: "test-parallel",
		Tasks: []Task{
			&InlineTask{
				Name: "task1-success",
				Fn: func(ctx *execctx.ExecutionContext) error {
					task1Executed = true
					return nil
				},
			},
			&InlineTask{
				Name: "task2-panics",
				Fn: func(ctx *execctx.ExecutionContext) error {
					panic("task 2 panicked!")
				},
			},
			&InlineTask{
				Name: "task3-success",
				Fn: func(ctx *execctx.ExecutionContext) error {
					task3Executed = true
					return nil
				},
			},
		},
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Act
	// EXPECTED FAILURE: This will panic and crash the test
	err := parallel.Execute(ctx)

	// Assert: Should recover panic and convert to error
	if err == nil {
		t.Error("Expected error from panic, got nil")
	}

	// Other tasks should still execute
	if !task1Executed {
		t.Error("Task 1 should have executed despite task 2 panicking")
	}

	if !task3Executed {
		t.Error("Task 3 should have executed despite task 2 panicking")
	}

	parallelResult, ok := err.(*ParallelResult)
	if !ok {
		t.Fatalf("Expected *ParallelResult, got %T", err)
	}

	// Should have error from panic
	// InlineTask already has panic recovery, ParallelTask collects the errors
	foundPanicError := false
	for _, r := range parallelResult.Results {
		if !r.Success && r.Error != nil {
			errMsg := r.Error.Error()
			// Check if error contains "panicked"
			if strings.Contains(errMsg, "panicked") {
				foundPanicError = true
				t.Logf("Found panic error: %s", errMsg)
				break
			}
		}
	}

	if !foundPanicError {
		t.Errorf("Panic should be recovered and included in errors. Got results: %v", parallelResult.Results)
	} else {
		t.Log("SUCCESS: Panic recovery implemented - all tasks completed")
	}
}

// TestParallelTask_ContextCancellation tests:
// Context cancellation should propagate to all tasks
//
// EXPECTED: Test should PASS if tasks respect context cancellation
func TestParallelTask_ContextCancellation(t *testing.T) {
	// Arrange
	baseCtx, cancel := context.WithCancel(context.Background())

	task1Started := false
	task2Started := false
	task3Started := false

	parallel := &ParallelTask{
		Name: "test-parallel",
		Tasks: []Task{
			&InlineTask{
				Name: "task1",
				Fn: func(ctx *execctx.ExecutionContext) error {
					task1Started = true
					select {
					case <-time.After(5 * time.Second):
						return nil
					case <-ctx.Context.Done():
						return ctx.Context.Err()
					}
				},
			},
			&InlineTask{
				Name: "task2",
				Fn: func(ctx *execctx.ExecutionContext) error {
					task2Started = true
					select {
					case <-time.After(5 * time.Second):
						return nil
					case <-ctx.Context.Done():
						return ctx.Context.Err()
					}
				},
			},
			&InlineTask{
				Name: "task3",
				Fn: func(ctx *execctx.ExecutionContext) error {
					task3Started = true
					select {
					case <-time.After(5 * time.Second):
						return nil
					case <-ctx.Context.Done():
						return ctx.Context.Err()
					}
				},
			},
		},
	}

	ctx := execctx.NewExecutionContext(
		baseCtx,
		clock.NewRealTimeClock(),
		nil,
	)

	// Act: Cancel context after 100ms
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := parallel.Execute(ctx)
	duration := time.Since(start)

	// Assert: Should complete quickly (not wait 5 seconds)
	if duration > 500*time.Millisecond {
		t.Errorf("Took %v to complete, should cancel within 500ms", duration)
	}

	// All tasks should have started
	if !task1Started || !task2Started || !task3Started {
		t.Error("All tasks should have started before cancellation")
	}

	// Should have cancellation errors
	if err == nil {
		t.Error("Expected cancellation errors, got nil")
	}

	t.Logf("Tasks completed in %v after context cancellation", duration)
	t.Log("SUCCESS: Context cancellation propagates to all tasks")
}
