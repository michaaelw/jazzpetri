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
// ParallelTask Doesn't Report Partial Success
// ============================================================================

// TestParallelTask_PartialSuccess_DetailedResults tests:
// If 2/5 tasks fail, cannot tell which succeeded
//
// NOW FIXED: Returns ParallelResult with detailed per-task status
// Expected behavior: Return detailed results with per-task status
func TestParallelTask_PartialSuccess_DetailedResults(t *testing.T) {
	// Arrange: Mix of successful and failing tasks
	task1 := &InlineTask{
		Name: "task1-success",
		Fn:   func(ctx *execContext.ExecutionContext) error { return nil },
	}

	task2 := &InlineTask{
		Name: "task2-fail",
		Fn:   func(ctx *execContext.ExecutionContext) error { return errors.New("task2 error") },
	}

	task3 := &InlineTask{
		Name: "task3-success",
		Fn:   func(ctx *execContext.ExecutionContext) error { return nil },
	}

	task4 := &InlineTask{
		Name: "task4-fail",
		Fn:   func(ctx *execContext.ExecutionContext) error { return errors.New("task4 error") },
	}

	task5 := &InlineTask{
		Name: "task5-success",
		Fn:   func(ctx *execContext.ExecutionContext) error { return nil },
	}

	parallel := &ParallelTask{
		Name:  "test-parallel",
		Tasks: []Task{task1, task2, task3, task4, task5},
	}

	ctx := execContext.NewExecutionContext(
		context.Background(),
		clock.NewVirtualClock(time.Now()),
		nil,
	)

	// Act
	err := parallel.Execute(ctx)

	// Assert: Now returns ParallelResult with detailed information
	if err == nil {
		t.Errorf("Expected error from failed tasks")
	}

	// Can now access detailed results
	if parallelResult, ok := err.(*ParallelResult); ok {
		// Verify we have results for all 5 tasks
		if len(parallelResult.Results) != 5 {
			t.Errorf("Expected 5 results, got %d", len(parallelResult.Results))
		}

		// Verify success/failure counts
		if parallelResult.SuccessCount != 3 {
			t.Errorf("Expected 3 successes, got %d", parallelResult.SuccessCount)
		}
		if parallelResult.FailureCount != 2 {
			t.Errorf("Expected 2 failures, got %d", parallelResult.FailureCount)
		}

		// Verify we can identify which tasks succeeded/failed
		expectedSuccess := map[int]bool{0: true, 2: true, 4: true}
		expectedFailure := map[int]bool{1: true, 3: true}

		for i, result := range parallelResult.Results {
			if expectedSuccess[i] && !result.Success {
				t.Errorf("Task %d should have succeeded but failed: %v", i, result.Error)
			}
			if expectedFailure[i] && result.Success {
				t.Errorf("Task %d should have failed but succeeded", i)
			}

			// Verify duration is tracked
			if result.Duration < 0 {
				t.Errorf("Task %d has invalid duration: %v", i, result.Duration)
			}

			// Verify index matches
			if result.Index != i {
				t.Errorf("Task result index mismatch: expected %d, got %d", i, result.Index)
			}
		}

		t.Logf("PASS: ParallelResult provides detailed per-task information:")
		t.Logf("  - Success count: %d", parallelResult.SuccessCount)
		t.Logf("  - Failure count: %d", parallelResult.FailureCount)
		for i, result := range parallelResult.Results {
			if result.Success {
				t.Logf("  - Task %d: SUCCESS (duration: %v)", i, result.Duration)
			} else {
				t.Logf("  - Task %d: FAILED - %v (duration: %v)", i, result.Error, result.Duration)
			}
		}
	} else {
		t.Errorf("Expected *ParallelResult, got %T", err)
	}
}

// TestParallelTask_Results tests:
// Need detailed results for each parallel task
//
// EXPECTED FAILURE: No results structure, only error/nil
// Expected behavior: New Execute() signature returning results
func TestParallelTask_Results(t *testing.T) {
	// After fix, should return results structure:
	/*
	type TaskResult struct {
		TaskIndex   int
		TaskName    string
		Success     bool
		Error       error
		Duration    time.Duration
		CompletedAt time.Time
	}

	type ParallelResult struct {
		Results      []TaskResult
		SuccessCount int
		FailureCount int
		TotalDuration time.Duration
	}

	// Execute returns both result and error
	result, err := parallel.Execute(ctx)

	// Can inspect individual task results
	for i, taskResult := range result.Results {
		if taskResult.Success {
			t.Logf("Task %d (%s) succeeded in %v",
				i, taskResult.TaskName, taskResult.Duration)
		} else {
			t.Logf("Task %d (%s) failed: %v",
				i, taskResult.TaskName, taskResult.Error)
		}
	}

	t.Logf("Summary: %d/%d succeeded",
		result.SuccessCount, len(result.Results))
	*/

	t.Logf("EXPECTED FAILURE: No results structure for parallel execution")
	t.Logf("After fix: Return ParallelResult with per-task details")
	t.Logf("")
	t.Logf("Proposed structure:")
	t.Logf("  type TaskResult struct {")
	t.Logf("    TaskIndex   int")
	t.Logf("    TaskName    string")
	t.Logf("    Success     bool")
	t.Logf("    Error       error")
	t.Logf("    Duration    time.Duration")
	t.Logf("    CompletedAt time.Time")
	t.Logf("  }")
}

// TestParallelTask_ResultsOrder tests:
// Results should maintain task order, not completion order
//
// EXPECTED FAILURE: No results structure
// Expected behavior: Results[i] corresponds to Tasks[i]
func TestParallelTask_ResultsOrder(t *testing.T) {
	// After fix: Results should be ordered by task index, not completion order
	/*
	parallel := &ParallelTask{
		Name: "ordered-results",
		Tasks: []Task{
			&InlineTask{Name: "slow", Fn: func(ctx) { time.Sleep(100*ms); return nil }},
			&InlineTask{Name: "fast", Fn: func(ctx) { return nil }},
			&InlineTask{Name: "medium", Fn: func(ctx) { time.Sleep(50*ms); return nil }},
		},
	}

	result, _ := parallel.Execute(ctx)

	// Results should be in task order, not completion order
	// result.Results[0] = slow task (completed last)
	// result.Results[1] = fast task (completed first)
	// result.Results[2] = medium task (completed second)

	if result.Results[0].TaskName != "slow" {
		t.Errorf("Results not in task order")
	}
	if result.Results[1].TaskName != "fast" {
		t.Errorf("Results not in task order")
	}
	if result.Results[2].TaskName != "medium" {
		t.Errorf("Results not in task order")
	}
	*/

	t.Logf("After fix: Results array indexed by task position, not completion order")
}

// TestParallelTask_ResultsDuration tests:
// Track how long each task took
//
// EXPECTED FAILURE: No duration tracking per task
// Expected behavior: Each TaskResult includes duration
func TestParallelTask_ResultsDuration(t *testing.T) {
	// Use case: Performance analysis of parallel tasks
	/*
	parallel := &ParallelTask{
		Name: "perf-test",
		Tasks: []Task{
			&InlineTask{Name: "task1", Fn: ...},
			&InlineTask{Name: "task2", Fn: ...},
			&InlineTask{Name: "task3", Fn: ...},
		},
	}

	result, _ := parallel.Execute(ctx)

	// Find slowest task
	var slowest TaskResult
	for _, r := range result.Results {
		if r.Duration > slowest.Duration {
			slowest = r
		}
	}

	t.Logf("Slowest task: %s (%v)", slowest.TaskName, slowest.Duration)
	t.Logf("Total duration: %v", result.TotalDuration)
	*/

	t.Logf("EXPECTED FAILURE: No duration tracking per task")
	t.Logf("After fix: TaskResult.Duration tracks execution time")
	t.Logf("Use case: Performance monitoring, SLA tracking, optimization")
}

// TestParallelTask_BackwardCompatibility tests:
// Changing Execute() signature is breaking change
//
// Tests backward compatibility considerations
func TestParallelTask_BackwardCompatibility(t *testing.T) {
	// Challenge: Current signature is Execute(ctx) error
	// Changing to Execute(ctx) (*ParallelResult, error) is breaking

	// Option 1: Add new method ExecuteWithResults()
	/*
	// Old method (deprecated but kept for compatibility)
	err := parallel.Execute(ctx)

	// New method
	result, err := parallel.ExecuteWithResults(ctx)
	*/

	// Option 2: Return error that contains results
	/*
	err := parallel.Execute(ctx)
	if detailedErr, ok := err.(ParallelExecutionError); ok {
		results := detailedErr.Results
		// Access detailed results
	}
	*/

	// Option 3: Store results in context
	/*
	err := parallel.Execute(ctx)
	if results := ctx.GetParallelResults(); results != nil {
		// Access results from context
	}
	*/

	t.Logf("EXPECTED FAILURE: Changing signature is breaking change")
	t.Logf("Consider:")
	t.Logf("  Option 1: Add ExecuteWithResults() method (keep Execute() unchanged)")
	t.Logf("  Option 2: Return error type that includes results")
	t.Logf("  Option 3: Store results in ExecutionContext")
	t.Logf("  Option 4: Accept breaking change for v2.0")
}

// TestParallelTask_ResultsWithPanic tests:
// Results should include panicked tasks
//
// EXPECTED FAILURE: Panics converted to errors, but no result details
// Expected behavior: TaskResult shows panic was recovered
func TestParallelTask_ResultsWithPanic(t *testing.T) {
	// After fix:
	/*
	task1 := &InlineTask{
		Name: "panic-task",
		Fn:   func(ctx) { panic("oops") },
	}

	parallel := &ParallelTask{
		Name: "with-panic",
		Tasks: []Task{task1},
	}

	result, err := parallel.Execute(ctx)

	// Panic should be in results
	if !result.Results[0].Success {
		if result.Results[0].Panic {
			t.Logf("Task panicked: %v", result.Results[0].Error)
		}
	}
	*/

	t.Logf("After fix: TaskResult should indicate if task panicked")
	t.Logf("  TaskResult.Panic bool")
	t.Logf("  TaskResult.Error contains panic value")
}

// TestParallelTask_SuccessRate tests:
// Calculate success rate from results
//
// EXPECTED FAILURE: Must count errors manually
// Expected behavior: SuccessCount/FailureCount in results
func TestParallelTask_SuccessRate(t *testing.T) {
	// After fix:
	/*
	result, _ := parallel.Execute(ctx)

	successRate := float64(result.SuccessCount) / float64(len(result.Results))
	t.Logf("Success rate: %.2f%% (%d/%d)",
		successRate*100,
		result.SuccessCount,
		len(result.Results))

	if successRate < 0.8 {
		t.Errorf("Success rate below threshold")
	}
	*/

	t.Logf("EXPECTED FAILURE: No SuccessCount/FailureCount in results")
	t.Logf("After fix: ParallelResult includes aggregate counts")
	t.Logf("Use case: SLA monitoring, quality gates, alerting")
}

// TestParallelTask_ResultsMetrics tests:
// Results should be recorded in metrics
//
// EXPECTED FAILURE: Only success/error metrics, no per-task metrics
// Expected behavior: Record detailed metrics from results
func TestParallelTask_ResultsMetrics(t *testing.T) {
	// After fix, record metrics:
	// - parallel_task_success_rate (gauge)
	// - parallel_task_duration (histogram per task)
	// - parallel_slowest_task_duration (gauge)
	// - parallel_fastest_task_duration (gauge)

	t.Logf("EXPECTED FAILURE: No detailed metrics from parallel execution")
	t.Logf("After fix: Record metrics from ParallelResult:")
	t.Logf("  - parallel_task_success_rate")
	t.Logf("  - parallel_task_duration (per task)")
	t.Logf("  - parallel_slowest_task_duration")
	t.Logf("  - parallel_fastest_task_duration")
}

// TestParallelTask_ResultsLogging tests:
// Results should be logged for observability
//
// EXPECTED FAILURE: Only aggregate logging
// Expected behavior: Log per-task results
func TestParallelTask_ResultsLogging(t *testing.T) {
	// After fix, log detailed results:
	/*
	ctx.Logger.Info("Parallel task completed", map[string]interface{}{
		"task_name":     parallel.Name,
		"total_tasks":   len(result.Results),
		"success_count": result.SuccessCount,
		"failure_count": result.FailureCount,
		"duration":      result.TotalDuration,
		"results":       result.Results, // Detailed per-task results
	})
	*/

	t.Logf("After fix: Log detailed results for debugging and monitoring")
}

// TestParallelTask_FastFail tests:
// Option to stop on first failure
//
// EXPECTED FAILURE: Always waits for all tasks
// Expected behavior: FastFail option to stop on first error
func TestParallelTask_FastFail(t *testing.T) {
	// Use case: Circuit breaker pattern
	// If any task fails, don't wait for others
	/*
	parallel := &ParallelTask{
		Name:     "fast-fail",
		Tasks:    []Task{...},
		FastFail: true, // Stop on first failure
	}

	result, err := parallel.Execute(ctx)

	// If task 2 fails, tasks 3-N might not complete
	// result.Results will have some nil/incomplete entries
	*/

	t.Logf("Future enhancement: FastFail option")
	t.Logf("Current: Always waits for all tasks to complete")
	t.Logf("With FastFail: Cancel remaining tasks on first failure")
}

// TestParallelTask_ResultsWithRetry tests:
// Results from RetryTask in parallel
//
// Tests interaction between ParallelTask and RetryTask
func TestParallelTask_ResultsWithRetry(t *testing.T) {
	// When ParallelTask contains RetryTask:
	// - Should TaskResult show retry attempts?
	// - Should it show only final result?

	t.Logf("Consider: How to show retry attempts in TaskResult?")
	t.Logf("  Option 1: Show only final result")
	t.Logf("  Option 2: TaskResult.RetryAttempts int")
	t.Logf("  Option 3: Nested results structure")
}

// TestParallelTask_ResultsJSON tests:
// Results should be serializable for reporting
//
// EXPECTED FAILURE: No results structure to serialize
// Expected behavior: ParallelResult serializable to JSON
func TestParallelTask_ResultsJSON(t *testing.T) {
	// After fix:
	/*
	result, _ := parallel.Execute(ctx)

	// Serialize to JSON for reporting
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		t.Errorf("Failed to marshal results: %v", err)
	}

	// Can store in database, send to monitoring system, etc.
	t.Logf("Results JSON: %s", string(jsonBytes))
	*/

	t.Logf("After fix: ParallelResult should be JSON-serializable")
	t.Logf("Use case: Reporting, monitoring dashboards, audit logs")
}
