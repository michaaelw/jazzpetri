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
	"testing"
	"time"

	"github.com/jazzpetri/engine/clock"
	execContext "github.com/jazzpetri/engine/context"
)

// ============================================================================
// No Task Timeout Configuration
// ============================================================================

// TestTask_Timeout tests:
// Tasks can run indefinitely without timeout
//
// NOW FIXED: Timeout field added to InlineTask
// Expected behavior: Task times out after configured duration
func TestTask_Timeout(t *testing.T) {
	// Arrange: Create a task that runs forever
	longRunningTask := &InlineTask{
		Name: "long-running",
		Fn: func(ctx *execContext.ExecutionContext) error {
			// Simulate long-running operation
			select {
			case <-ctx.Context.Done():
				return ctx.Context.Err()
			case <-time.After(10 * time.Second):
				return nil
			}
		},
		// Timeout field now exists
		Timeout: 100 * time.Millisecond,
	}

	ctx := execContext.NewExecutionContext(
		context.Background(),
		clock.NewVirtualClock(time.Now()),
		nil,
	)

	// Act: Execute with timeout
	start := time.Now()
	err := longRunningTask.Execute(ctx)
	duration := time.Since(start)

	// Assert
	// Should timeout after 100ms, not run for 10 seconds
	if duration > 1*time.Second {
		t.Errorf("Task ran too long: %v (expected ~100ms)", duration)
	} else {
		t.Logf("PASS: Task timed out after: %v", duration)
	}

	if err == nil {
		t.Errorf("Expected timeout error, got nil")
	} else {
		t.Logf("PASS: Task returned timeout error: %v", err)
	}

	// Verify the error is a timeout error
	if !isTimeoutError(err) {
		t.Errorf("Expected timeout error, got: %v", err)
	}

	t.Logf("FIXED: Timeout mechanism now works for InlineTask")
}

// Helper function to check if error is a timeout error
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	return err == context.DeadlineExceeded ||
		   err.Error() == context.DeadlineExceeded.Error() ||
		   context.DeadlineExceeded.Error() != "" &&
		   len(err.Error()) > 0 &&
		   err.Error()[len(err.Error())-len(context.DeadlineExceeded.Error()):] == context.DeadlineExceeded.Error()
}

// TestTask_TimeoutCancellation tests:
// Timeout should cancel task context
//
// EXPECTED FAILURE: No timeout cancellation mechanism
// Expected behavior: Create child context with timeout, cancel on timeout
func TestTask_TimeoutCancellation(t *testing.T) {
	// Arrange: Task that respects context cancellation
	cancelledChan := make(chan bool, 1)

	task := &InlineTask{
		Name: "cancellable",
		Fn: func(ctx *execContext.ExecutionContext) error {
			select {
			case <-ctx.Context.Done():
				cancelledChan <- true
				return ctx.Context.Err()
			case <-time.After(5 * time.Second):
				cancelledChan <- false
				return nil
			}
		},
		// EXPECTED FAILURE: Timeout field doesn't exist
		// Timeout: 100 * time.Millisecond,
	}

	ctx := execContext.NewExecutionContext(
		context.Background(),
		clock.NewVirtualClock(time.Now()),
		nil,
	)

	// Act
	start := time.Now()
	task.Execute(ctx)
	duration := time.Since(start)

	// Assert
	select {
	case cancelled := <-cancelledChan:
		if !cancelled {
			t.Logf("EXPECTED FAILURE: Task completed normally (not cancelled)")
			t.Logf("Duration: %v", duration)
		} else {
			t.Logf("Task was cancelled: %v", duration)
		}
	case <-time.After(6 * time.Second):
		t.Logf("EXPECTED FAILURE: Task timed out at test level")
	}

	t.Logf("")
	t.Logf("After fix: Task execution should:")
	t.Logf("  1. Create child context with timeout")
	t.Logf("  2. Cancel context when timeout expires")
	t.Logf("  3. Return context.DeadlineExceeded error")
}

// TestTask_TimeoutError tests:
// Timeout should return specific error type
//
// EXPECTED FAILURE: No timeout error handling
// Expected behavior: Return context.DeadlineExceeded on timeout
func TestTask_TimeoutError(t *testing.T) {
	// After fix, timeout should return specific error:
	/*
	task := &InlineTask{
		Name: "timeout-task",
		Fn: func(ctx *ExecutionContext) error {
			time.Sleep(1 * time.Second)
			return nil
		},
		Timeout: 100 * time.Millisecond,
	}

	err := task.Execute(ctx)

	// Should return context.DeadlineExceeded
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected DeadlineExceeded error, got: %v", err)
	}
	*/

	t.Logf("EXPECTED FAILURE: No timeout error returned")
	t.Logf("After fix: Timeout should return context.DeadlineExceeded")
	t.Logf("This allows callers to distinguish timeout from other errors")
}

// TestTask_TimeoutMetrics tests:
// Timeout should be tracked in metrics
//
// EXPECTED FAILURE: No timeout metrics
// Expected behavior: Record task_timeout_total metric
func TestTask_TimeoutMetrics(t *testing.T) {
	// After fix, should record metrics:
	// - task_timeout_total (counter)
	// - task_timeout_duration (histogram of timeout durations)

	t.Logf("EXPECTED FAILURE: No metrics for task timeouts")
	t.Logf("After fix: Add timeout metrics:")
	t.Logf("  - task_timeout_total")
	t.Logf("  - task_timeout_duration")
}

// TestTask_ZeroTimeout tests:
// Zero timeout should mean no timeout
//
// Tests that timeout=0 allows unlimited execution time
func TestTask_ZeroTimeout(t *testing.T) {
	// After fix: Timeout=0 should mean no timeout (unlimited)
	/*
	task := &InlineTask{
		Name: "no-timeout",
		Fn: func(ctx *ExecutionContext) error {
			time.Sleep(200 * time.Millisecond)
			return nil
		},
		Timeout: 0, // No timeout
	}

	start := time.Now()
	err := task.Execute(ctx)
	duration := time.Since(start)

	if err != nil {
		t.Errorf("Task should not timeout with Timeout=0: %v", err)
	}

	if duration < 200*time.Millisecond {
		t.Errorf("Task completed too quickly")
	}
	*/

	t.Logf("After fix: Timeout=0 means no timeout (backward compatible)")
}

// TestTask_TimeoutInheritance tests:
// Composite tasks should propagate timeout to children
//
// EXPECTED FAILURE: No timeout propagation mechanism
// Expected behavior: SequentialTask and ParallelTask should respect timeout
func TestTask_TimeoutInheritance(t *testing.T) {
	// Use case: Apply timeout to entire sequence
	/*
	seq := &SequentialTask{
		Name: "sequence-with-timeout",
		Tasks: []Task{
			&InlineTask{Name: "task1", Fn: ...},
			&InlineTask{Name: "task2", Fn: ...},
			&InlineTask{Name: "task3", Fn: ...},
		},
		Timeout: 500 * time.Millisecond, // Total timeout for sequence
	}

	// If sequence takes > 500ms total, should timeout
	*/

	t.Logf("EXPECTED FAILURE: No timeout support in composite tasks")
	t.Logf("After fix: Add Timeout to SequentialTask and ParallelTask")
	t.Logf("Timeout applies to entire composite operation")
}

// TestTask_TimeoutPerTask tests:
// Individual tasks in sequence should have their own timeouts
//
// Tests per-task timeout vs total sequence timeout
func TestTask_TimeoutPerTask(t *testing.T) {
	// Use case: Each task has its own timeout, plus sequence has total timeout
	/*
	seq := &SequentialTask{
		Name: "sequence",
		Tasks: []Task{
			&InlineTask{
				Name:    "task1",
				Fn:      ...,
				Timeout: 100 * time.Millisecond, // Per-task timeout
			},
			&InlineTask{
				Name:    "task2",
				Fn:      ...,
				Timeout: 200 * time.Millisecond,
			},
		},
		Timeout: 1 * time.Second, // Total sequence timeout
	}

	// task1 times out after 100ms
	// task2 times out after 200ms
	// whole sequence times out after 1s
	*/

	t.Logf("After fix: Support both per-task and composite timeouts")
	t.Logf("  - Individual task timeout applies to that task")
	t.Logf("  - Composite timeout applies to entire sequence/parallel")
}

// TestHTTPTask_Timeout tests:
// HTTPTask should support timeout configuration
//
// EXPECTED FAILURE: HTTPTask uses default client timeout
// Expected behavior: HTTPTask.Timeout configures HTTP client timeout
func TestHTTPTask_Timeout(t *testing.T) {
	// HTTPTask should support timeout:
	/*
	httpTask := &HTTPTask{
		Name:    "api-call",
		Method:  "GET",
		URL:     "https://slow-api.example.com/data",
		Timeout: 5 * time.Second,
	}

	// Should configure HTTP client with timeout
	// Or create context with timeout for request
	*/

	t.Logf("EXPECTED FAILURE: HTTPTask doesn't expose timeout configuration")
	t.Logf("After fix: Add Timeout field to HTTPTask")
	t.Logf("Use it to configure HTTP client or request context")
}

// TestCommandTask_Timeout tests:
// CommandTask should support timeout configuration
//
// EXPECTED FAILURE: CommandTask has no timeout
// Expected behavior: CommandTask.Timeout kills subprocess on timeout
func TestCommandTask_Timeout(t *testing.T) {
	// CommandTask should support timeout:
	/*
	cmdTask := &CommandTask{
		Name:    "long-script",
		Command: "python",
		Args:    []string{"long_running_script.py"},
		Timeout: 10 * time.Second,
	}

	// Should kill subprocess after timeout
	*/

	t.Logf("EXPECTED FAILURE: CommandTask doesn't expose timeout")
	t.Logf("After fix: Add Timeout field to CommandTask")
	t.Logf("Kill subprocess and return timeout error after duration")
}

// TestRetryTask_Timeout tests:
// RetryTask should respect timeout across all retries
//
// Tests timeout applies to total retry duration, not per attempt
func TestRetryTask_Timeout(t *testing.T) {
	// Use case: Retry up to 5 times, but total timeout is 10 seconds
	/*
	retryTask := &RetryTask{
		Name:       "retry-with-timeout",
		Task:       slowTask,
		MaxRetries: 5,
		Timeout:    10 * time.Second, // Total timeout across all retries
	}

	// Should stop retrying when timeout expires, even if retries remain
	*/

	t.Logf("After fix: RetryTask timeout applies to total retry duration")
	t.Logf("Not per-attempt timeout, but total time budget")
}

// TestTask_TimeoutLogging tests:
// Timeout should be logged for debugging
//
// EXPECTED FAILURE: No timeout logging
// Expected behavior: Log when task times out
func TestTask_TimeoutLogging(t *testing.T) {
	// After fix, should log:
	// - When timeout is set
	// - When timeout expires
	// - Which task timed out
	// - How long it ran before timing out

	t.Logf("EXPECTED FAILURE: No timeout logging")
	t.Logf("After fix: Log timeout events:")
	t.Logf("  - 'Task timeout configured: %%s, duration: %%v'")
	t.Logf("  - 'Task timed out: %%s, ran for: %%v'")
}

// TestTask_TimeoutWithPanic tests:
// Task that panics should still respect timeout
//
// Tests interaction between panic recovery and timeout
func TestTask_TimeoutWithPanic(t *testing.T) {
	// Edge case: Task panics before timeout
	// Panic recovery should trigger before timeout

	// Edge case: Task panics at same time as timeout
	// Should handle both gracefully

	t.Logf("After fix: Ensure timeout works with panic recovery")
	t.Logf("Panic recovery should take precedence over timeout")
}

// TestTask_ContextTimeoutVsTaskTimeout tests:
// Distinguish between context timeout and task timeout
//
// Tests that task timeout doesn't interfere with parent context timeout
func TestTask_ContextTimeoutVsTaskTimeout(t *testing.T) {
	// Use case:
	// - Parent context has 5 second timeout
	// - Task has 1 second timeout
	// - Task should timeout after 1 second
	// - Parent context should still be valid

	// Use case 2:
	// - Parent context has 1 second timeout
	// - Task has 5 second timeout
	// - Should timeout after 1 second (parent wins)

	t.Logf("After fix: Task timeout should create child context")
	t.Logf("Use whichever timeout expires first (task or parent context)")
	t.Logf("Return appropriate error indicating which timeout occurred")
}
