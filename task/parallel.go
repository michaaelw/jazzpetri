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
	stdContext "context"
	"fmt"
	"sync"
	"time"

	"github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/token"
)

// TaskResult represents the execution result of a single task within a parallel execution.
// Parallel Task Detailed Results
//
// This provides detailed information about each task's execution, including:
//   - Success/failure status
//   - Error details (if failed)
//   - Execution duration
//   - Task index in the parallel task list
type TaskResult struct {
	// Index is the position of this task in the parallel task list
	Index int

	// Success indicates whether the task executed without error
	Success bool

	// Error contains the error if the task failed, nil otherwise
	Error error

	// Duration is how long the task took to execute
	Duration time.Duration
}

// ParallelResult aggregates the results of all tasks in a parallel execution.
// Parallel Task Detailed Results
//
// This provides a complete view of parallel execution including:
//   - Per-task results with success/failure status
//   - Aggregate success/failure counts
//   - Individual task durations
//
// ParallelResult implements the error interface for backward compatibility.
// If any tasks failed, Error() returns a formatted error message.
type ParallelResult struct {
	// Results contains the result of each task, indexed by task position
	Results []TaskResult

	// SuccessCount is the number of tasks that completed successfully
	SuccessCount int

	// FailureCount is the number of tasks that failed
	FailureCount int
}

// Error implements the error interface for backward compatibility.
// Returns a formatted error message if any tasks failed, empty string otherwise.
func (pr *ParallelResult) Error() string {
	if pr.FailureCount == 0 {
		return ""
	}

	if pr.FailureCount == 1 {
		// Find the single error
		for _, result := range pr.Results {
			if !result.Success {
				return result.Error.Error()
			}
		}
	}

	// Multiple errors - use MultiError format
	multiErr := &MultiError{}
	for _, result := range pr.Results {
		if !result.Success {
			multiErr.Add(result.Error)
		}
	}
	return multiErr.Error()
}

// AllSucceeded returns true if all tasks completed successfully.
func (pr *ParallelResult) AllSucceeded() bool {
	return pr.FailureCount == 0
}

// HasErrors returns true if any tasks failed.
func (pr *ParallelResult) HasErrors() bool {
	return pr.FailureCount > 0
}

// ParallelTask executes a list of tasks concurrently using goroutines.
// This is a composite task that implements the Task interface.
//
// ParallelTask is ideal for:
//   - Independent operations that can run concurrently
//   - I/O-bound tasks (HTTP requests, database queries)
//   - Fan-out patterns where results are collected
//
// Token Isolation:
// Each task receives a COPY of the token to ensure isolation. Tasks cannot
// see each other's modifications to the token. The original token in the
// ExecutionContext remains unchanged.
//
// This isolation prevents race conditions and ensures predictable behavior.
//
// Error Handling:
// All tasks run to completion regardless of errors. If any task fails,
// all errors are collected into a MultiError and returned after all tasks complete.
//
// Example:
//
//	parallel := &ParallelTask{
//	    Name: "fetch-data",
//	    Tasks: []Task{
//	        &HTTPTask{Name: "fetch-users", URL: "https://api.example.com/users"},
//	        &HTTPTask{Name: "fetch-orders", URL: "https://api.example.com/orders"},
//	        &HTTPTask{Name: "fetch-products", URL: "https://api.example.com/products"},
//	    },
//	}
//
// Composability:
// ParallelTask can contain other composite tasks, allowing unlimited nesting:
//
//	parallel := &ParallelTask{
//	    Tasks: []Task{
//	        &SequentialTask{...},
//	        &RetryTask{...},
//	        &ConditionalTask{...},
//	    },
//	}
//
// Thread Safety:
// ParallelTask is safe for concurrent execution. Multiple goroutines can
// execute the same ParallelTask simultaneously.
//
// Cancellation:
// Context cancellation is propagated to all parallel tasks. Each task
// is responsible for respecting the context cancellation.
type ParallelTask struct {
	// Name is the task identifier used in logs, traces, and metrics.
	Name string

	// Tasks is the list of tasks to execute in parallel.
	// Each task receives a copy of the token for isolation.
	Tasks []Task

	// Timeout is the maximum duration for all parallel tasks to complete.
	// Task Timeout Configuration
	//
	// If > 0: All tasks will be cancelled if they don't complete within this duration
	// If = 0: No timeout (unlimited execution time) - default
	//
	// This is the total timeout for ALL tasks to complete, not per-task.
	// Individual tasks can have their own timeouts as well.
	//
	// When timeout occurs, all tasks' contexts are cancelled and
	// context.DeadlineExceeded error is returned.
	//
	// Example:
	//   parallel := &ParallelTask{
	//       Name:    "concurrent-apis",
	//       Tasks:   []Task{...},
	//       Timeout: 5 * time.Second, // All tasks must complete within 5s
	//   }
	Timeout time.Duration
}

// Execute runs all tasks concurrently and waits for all to complete.
//
// Execution flow:
//  1. Start a trace span for the parallel execution
//  2. Record parallel execution metric
//  3. Launch all tasks as goroutines
//  4. Each task gets a copy of the token (isolation)
//  5. Wait for all tasks to complete using sync.WaitGroup
//  6. Collect all errors into a MultiError
//  7. Complete the trace span with task count attribute
//
// Thread Safety:
// Execute is safe to call concurrently from multiple goroutines.
// Internally uses sync.WaitGroup for coordination and mutex for error collection.
//
// Cancellation:
// Context cancellation is propagated to all tasks through their ExecutionContext.
// Tasks should check ctx.Context.Done() for graceful cancellation.
//
// Returns:
//   - nil if all tasks succeed
//   - MultiError if one or more tasks fail
//   - validation error if Name is empty or Tasks is empty
func (p *ParallelTask) Execute(ctx *context.ExecutionContext) error {
	// Validate task configuration
	if p.Name == "" {
		return fmt.Errorf("parallel task: name is required")
	}
	if len(p.Tasks) == 0 {
		return fmt.Errorf("parallel task: tasks list cannot be empty")
	}

	// Create timeout context if configured
	var cancel stdContext.CancelFunc
	if p.Timeout > 0 {
		ctx.Context, cancel = stdContext.WithTimeout(ctx.Context, p.Timeout)
		defer cancel()
	}

	// Start trace span
	span := ctx.Tracer.StartSpan("task.parallel." + p.Name)
	defer span.End()
	span.SetAttribute("task_count", len(p.Tasks))
	if p.Timeout > 0 {
		span.SetAttribute("timeout", p.Timeout.String())
	}

	// Record metrics
	ctx.Metrics.Inc("task_executions_total")
	ctx.Metrics.Inc("parallel_tasks_total")

	// Log parallel start
	logFields := map[string]interface{}{
		"task_type":  "parallel",
		"task_name":  p.Name,
		"task_count": len(p.Tasks),
		"token_id":   tokenID(ctx),
	}
	if p.Timeout > 0 {
		logFields["timeout"] = p.Timeout.String()
	}
	ctx.Logger.Debug("Executing parallel task", logFields)

	// Prepare for parallel execution
	var wg sync.WaitGroup
	resultsMu := &sync.Mutex{}

	// Collect detailed results for each task
	results := make([]TaskResult, len(p.Tasks))

	// Launch all tasks concurrently
	for i, task := range p.Tasks {
		wg.Add(1)
		go func(index int, t Task) {
			defer wg.Done()

			// Track task execution time
			taskStart := time.Now()

			// Panic Recovery
			// Wrap task execution in recover to convert panics to errors
			var taskErr error
			defer func() {
				if r := recover(); r != nil {
					panicErr := fmt.Errorf("parallel task panicked at index %d: %v", index, r)
					taskErr = panicErr

					ctx.Logger.Error("Parallel task panicked", map[string]interface{}{
						"task_type":     "parallel",
						"parallel_task": p.Name,
						"task_index":    index,
						"panic":         r,
					})

					// Record panic result
					resultsMu.Lock()
					results[index] = TaskResult{
						Index:    index,
						Success:  false,
						Error:    panicErr,
						Duration: time.Since(taskStart),
					}
					resultsMu.Unlock()
				}
			}()

			// Create a copy of the token for isolation
			// Deep copy token data to prevent race conditions
			var tokenCopy *token.Token
			if ctx.Token != nil {
				tokenCopy = &token.Token{
					ID: ctx.Token.ID,
				}

				// Attempt deep copy if the data implements Cloneable
				if cloneable, ok := ctx.Token.Data.(token.Cloneable); ok {
					// Deep copy: create independent copy of the data
					tokenCopy.Data = cloneable.Clone()
				} else {
					// Shallow copy: reference same data
					// This is acceptable if data is immutable or thread-safe
					tokenCopy.Data = ctx.Token.Data
				}
			}

			// Create a new context with the token copy
			taskCtx := ctx.WithToken(tokenCopy)

			// Log task start
			taskCtx.Logger.Debug("Executing parallel task", map[string]interface{}{
				"task_type":     "parallel",
				"parallel_task": p.Name,
				"task_index":    index,
			})

			// Execute the task
			taskErr = t.Execute(taskCtx)

			// Record task result
			resultsMu.Lock()
			results[index] = TaskResult{
				Index:    index,
				Success:  taskErr == nil,
				Error:    taskErr,
				Duration: time.Since(taskStart),
			}
			resultsMu.Unlock()

			// Log task completion
			if taskErr != nil {
				taskCtx.Logger.Error("Parallel task failed", map[string]interface{}{
					"task_type":     "parallel",
					"parallel_task": p.Name,
					"task_index":    index,
					"error":         taskErr.Error(),
					"duration":      time.Since(taskStart).String(),
				})
			} else {
				taskCtx.Logger.Debug("Parallel task succeeded", map[string]interface{}{
					"task_type":     "parallel",
					"parallel_task": p.Name,
					"task_index":    index,
					"duration":      time.Since(taskStart).String(),
				})
			}
		}(i, task)
	}

	// Wait for all tasks to complete
	wg.Wait()

	// Build ParallelResult from individual task results
	parallelResult := &ParallelResult{
		Results: results,
	}

	// Count successes and failures
	for _, result := range results {
		if result.Success {
			parallelResult.SuccessCount++
		} else {
			parallelResult.FailureCount++
		}
	}

	// Check if timeout occurred
	if ctx.Context.Err() == stdContext.DeadlineExceeded {
		span.RecordError(stdContext.DeadlineExceeded)
		ctx.Metrics.Inc("task_timeout_total")
		ctx.Logger.Error("Parallel task timeout", map[string]interface{}{
			"task_type":     "parallel",
			"task_name":     p.Name,
			"total_tasks":   len(p.Tasks),
			"timeout":       p.Timeout.String(),
			"failed_tasks":  parallelResult.FailureCount,
			"success_count": parallelResult.SuccessCount,
		})
		// Timeout is a failure - ensure FailureCount reflects it
		// (individual task errors already recorded in results)
		return parallelResult
	}

	// Check for errors
	if parallelResult.HasErrors() {
		span.RecordError(parallelResult)
		span.SetAttribute("failed_tasks", parallelResult.FailureCount)
		span.SetAttribute("success_tasks", parallelResult.SuccessCount)
		ctx.Metrics.Inc("parallel_errors_total")
		ctx.Logger.Error("Parallel task completed with errors", map[string]interface{}{
			"task_type":     "parallel",
			"task_name":     p.Name,
			"total_tasks":   len(p.Tasks),
			"failed_tasks":  parallelResult.FailureCount,
			"success_tasks": parallelResult.SuccessCount,
		})
		return parallelResult
	}

	// All tasks completed successfully
	ctx.Metrics.Inc("parallel_success_total")
	ctx.Logger.Debug("Parallel task completed successfully", map[string]interface{}{
		"task_type":     "parallel",
		"task_name":     p.Name,
		"task_count":    len(p.Tasks),
		"success_count": parallelResult.SuccessCount,
	})

	return nil
}
