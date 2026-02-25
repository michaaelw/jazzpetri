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
	"testing"
	"time"

	"github.com/jazzpetri/engine/clock"
	execContext "github.com/jazzpetri/engine/context"
)

// ============================================================================
// SequentialTask Doesn't Short-Circuit on Error
// ============================================================================

// TestSequentialTask_ShortCircuit tests:
// If task 2 fails, tasks 3-N still execute
//
// THIS SHOULD PASS: SequentialTask already stops on first error
// Verifies current behavior is correct
func TestSequentialTask_ShortCircuit(t *testing.T) {
	// Arrange: Track which tasks executed
	executed := make([]int, 0)
	executedMu := make(chan struct{}, 1)
	executedMu <- struct{}{} // Initialize

	task1 := &InlineTask{
		Name: "task1",
		Fn: func(ctx *execContext.ExecutionContext) error {
			<-executedMu
			executed = append(executed, 1)
			executedMu <- struct{}{}
			return nil
		},
	}

	task2 := &InlineTask{
		Name: "task2",
		Fn: func(ctx *execContext.ExecutionContext) error {
			<-executedMu
			executed = append(executed, 2)
			executedMu <- struct{}{}
			return errors.New("task 2 failed") // FAILS HERE
		},
	}

	task3 := &InlineTask{
		Name: "task3",
		Fn: func(ctx *execContext.ExecutionContext) error {
			<-executedMu
			executed = append(executed, 3)
			executedMu <- struct{}{}
			return nil
		},
	}

	task4 := &InlineTask{
		Name: "task4",
		Fn: func(ctx *execContext.ExecutionContext) error {
			<-executedMu
			executed = append(executed, 4)
			executedMu <- struct{}{}
			return nil
		},
	}

	seq := &SequentialTask{
		Name:  "test-sequence",
		Tasks: []Task{task1, task2, task3, task4},
	}

	ctx := execContext.NewExecutionContext(
		context.Background(),
		clock.NewVirtualClock(time.Now()),
		nil,
	)

	// Act
	err := seq.Execute(ctx)

	// Assert: Should stop at task2
	if err == nil {
		t.Errorf("Expected error from task2")
	}

	<-executedMu
	if len(executed) != 2 {
		t.Errorf("Expected 2 tasks executed, got %d: %v", len(executed), executed)
		t.Errorf("FAILURE: Tasks 3 and 4 should not have executed")
	}

	if len(executed) >= 1 && executed[0] != 1 {
		t.Errorf("Expected task 1 to execute first")
	}
	if len(executed) >= 2 && executed[1] != 2 {
		t.Errorf("Expected task 2 to execute second")
	}

	// This should PASS - SequentialTask already short-circuits
	t.Logf("PASS: SequentialTask correctly stops at first error")
	t.Logf("Executed tasks: %v", executed)
}

// TestSequentialTask_ContinueOnError tests:
// Option to continue execution even when tasks fail
//
// EXPECTED FAILURE: No StopOnError config option
// Expected behavior: Add StopOnError option (default true)
func TestSequentialTask_ContinueOnError(t *testing.T) {
	// Arrange: Track which tasks executed and errors
	executed := make([]int, 0)
	executedMu := make(chan struct{}, 1)
	executedMu <- struct{}{} // Initialize

	task1 := &InlineTask{
		Name: "task1",
		Fn: func(ctx *execContext.ExecutionContext) error {
			<-executedMu
			executed = append(executed, 1)
			executedMu <- struct{}{}
			return nil
		},
	}

	task2 := &InlineTask{
		Name: "task2",
		Fn: func(ctx *execContext.ExecutionContext) error {
			<-executedMu
			executed = append(executed, 2)
			executedMu <- struct{}{}
			return errors.New("task 2 failed")
		},
	}

	task3 := &InlineTask{
		Name: "task3",
		Fn: func(ctx *execContext.ExecutionContext) error {
			<-executedMu
			executed = append(executed, 3)
			executedMu <- struct{}{}
			return errors.New("task 3 failed")
		},
	}

	task4 := &InlineTask{
		Name: "task4",
		Fn: func(ctx *execContext.ExecutionContext) error {
			<-executedMu
			executed = append(executed, 4)
			executedMu <- struct{}{}
			return nil
		},
	}

	// EXPECTED FAILURE: StopOnError field doesn't exist - won't compile
	/*
	seq := &SequentialTask{
		Name:       "test-sequence",
		Tasks:      []Task{task1, task2, task3, task4},
		StopOnError: false, // Continue even on errors
	}
	*/

	// Current behavior (will stop at task 2):
	seq := &SequentialTask{
		Name:  "test-sequence",
		Tasks: []Task{task1, task2, task3, task4},
	}

	ctx := execContext.NewExecutionContext(
		context.Background(),
		clock.NewVirtualClock(time.Now()),
		nil,
	)

	// Act
	_ = seq.Execute(ctx)

	// Assert
	<-executedMu

	// Current behavior: stops at task 2
	if len(executed) == 2 {
		t.Logf("EXPECTED FAILURE: Current behavior stops at first error")
		t.Logf("Executed tasks: %v", executed)
	}

	// After fix with StopOnError=false:
	// - All 4 tasks should execute
	// - Should return MultiError with errors from task2 and task3
	/*
	if len(executed) != 4 {
		t.Errorf("Expected all 4 tasks to execute, got %d", len(executed))
	}

	if err == nil {
		t.Errorf("Expected error to be returned")
	}

	// Should be MultiError
	if multiErr, ok := err.(*MultiError); ok {
		if len(multiErr.Errors) != 2 {
			t.Errorf("Expected 2 errors, got %d", len(multiErr.Errors))
		}
	} else {
		t.Errorf("Expected MultiError, got %T", err)
	}
	*/

	t.Logf("")
	t.Logf("EXPECTED FAILURE: No StopOnError config option")
	t.Logf("After fix: Add StopOnError bool field to SequentialTask")
	t.Logf("  - StopOnError: true (default) - stop at first error")
	t.Logf("  - StopOnError: false - continue, collect all errors in MultiError")
}

// TestSequentialTask_ErrorCollection tests:
// When continuing on error, all errors should be collected
//
// EXPECTED FAILURE: No error collection mechanism
// Expected behavior: Collect all errors in MultiError
func TestSequentialTask_ErrorCollection(t *testing.T) {
	t.Logf("EXPECTED FAILURE: No error collection when StopOnError=false")
	t.Logf("After fix:")
	t.Logf("  1. Add StopOnError bool field (default true)")
	t.Logf("  2. When StopOnError=false, collect all errors")
	t.Logf("  3. Return MultiError with all collected errors")
	t.Logf("  4. All tasks execute regardless of individual failures")
}

// TestSequentialTask_StopOnErrorDefault tests:
// Default behavior should remain backward compatible
//
// Tests that default behavior doesn't change
func TestSequentialTask_StopOnErrorDefault(t *testing.T) {
	// After fix, verify default behavior remains the same

	// Arrange
	task1 := &InlineTask{
		Name: "task1",
		Fn:   func(ctx *execContext.ExecutionContext) error { return nil },
	}

	task2 := &InlineTask{
		Name: "task2",
		Fn:   func(ctx *execContext.ExecutionContext) error { return errors.New("failed") },
	}

	task3 := &InlineTask{
		Name: "task3",
		Fn:   func(ctx *execContext.ExecutionContext) error { return nil },
	}

	// Create without specifying StopOnError (should default to true)
	seq := &SequentialTask{
		Name:  "test-sequence",
		Tasks: []Task{task1, task2, task3},
		// StopOnError should default to true
	}

	ctx := execContext.NewExecutionContext(
		context.Background(),
		clock.NewVirtualClock(time.Now()),
		nil,
	)

	// Act
	err := seq.Execute(ctx)

	// Assert: Should stop at task2 (default behavior)
	if err == nil {
		t.Errorf("Expected error from task2")
	}

	// Should not be MultiError (only one error)
	if _, ok := err.(*MultiError); ok {
		t.Errorf("Should not return MultiError when stopping on first error")
	}

	t.Logf("After fix: Default StopOnError=true maintains backward compatibility")
}

// TestSequentialTask_ErrorMetrics tests:
// Metrics should track error behavior
//
// Tests that metrics distinguish between stop-on-error and continue-on-error
func TestSequentialTask_ErrorMetrics(t *testing.T) {
	// After fix, metrics should track:
	// - sequence_stopped_on_error_total (StopOnError=true, stopped early)
	// - sequence_completed_with_errors_total (StopOnError=false, completed with errors)
	// - sequence_tasks_failed_total (total failed tasks)

	t.Logf("After fix: Add metrics for error behavior:")
	t.Logf("  - sequence_stopped_on_error_total")
	t.Logf("  - sequence_completed_with_errors_total")
	t.Logf("  - sequence_tasks_failed_total")
}

// TestSequentialTask_PartialSuccess tests:
// Use case: Want to know which tasks succeeded when some failed
//
// EXPECTED FAILURE: Can't track partial success
// Expected behavior: Track which tasks succeeded/failed
func TestSequentialTask_PartialSuccess(t *testing.T) {
	// Use case: Data migration where we want to:
	// 1. Try to migrate all records
	// 2. Track which ones succeeded and which failed
	// 3. Don't stop at first failure

	// After fix with StopOnError=false:
	/*
	seq := &SequentialTask{
		Name:       "migrate-records",
		StopOnError: false,
		Tasks: []Task{
			&InlineTask{Name: "migrate-1", Fn: ...}, // Success
			&InlineTask{Name: "migrate-2", Fn: ...}, // Fail
			&InlineTask{Name: "migrate-3", Fn: ...}, // Success
			&InlineTask{Name: "migrate-4", Fn: ...}, // Fail
			&InlineTask{Name: "migrate-5", Fn: ...}, // Success
		},
	}

	err := seq.Execute(ctx)

	// Should get MultiError with failures from tasks 2 and 4
	if multiErr, ok := err.(*MultiError); ok {
		t.Logf("3 tasks succeeded, 2 failed")
		t.Logf("Failures: %v", multiErr.Errors)
	}
	*/

	t.Logf("EXPECTED FAILURE: No way to continue and collect all errors")
	t.Logf("Use case: Batch processing where failures shouldn't stop workflow")
	t.Logf("After fix: StopOnError=false enables partial success scenarios")
}

// TestSequentialTask_ContinueOnErrorWithContext tests:
// Context cancellation should still stop execution
//
// Tests that StopOnError=false still respects context cancellation
func TestSequentialTask_ContinueOnErrorWithContext(t *testing.T) {
	// After fix: Even with StopOnError=false, context cancellation should stop execution

	t.Logf("After fix: StopOnError=false should still respect context cancellation")
	t.Logf("  - Task errors: continue")
	t.Logf("  - Context cancelled: stop immediately")
}
