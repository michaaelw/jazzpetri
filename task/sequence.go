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
	"time"

	"github.com/jazzpetri/engine/context"
)

// SequentialTask executes a list of tasks in order.
// This is a composite task that implements the Task interface.
//
// SequentialTask is ideal for:
//   - Multi-step workflows where each step depends on the previous
//   - Data transformation pipelines
//   - Workflows where order matters
//
// Token Flow:
// The token flows through the chain of tasks. Each task can read and modify
// the token, and changes are visible to subsequent tasks in the sequence.
//
// Error Handling :
// By default (ContinueOnError=false), execution stops at the first error.
// When ContinueOnError=true, all tasks execute regardless of errors, and
// errors are collected into a MultiError.
//
// Example with default error handling (stop on first error):
//
//	seq := &SequentialTask{
//	    Name: "order-processing",
//	    Tasks: []Task{
//	        &InlineTask{Name: "validate", Fn: validateOrder},
//	        &HTTPTask{Name: "payment", URL: "https://api.payment.com/charge"},
//	        &InlineTask{Name: "fulfill", Fn: fulfillOrder},
//	    },
//	    // ContinueOnError: false is the default
//	}
//
// Example with continue-on-error (collect all errors):
//
//	seq := &SequentialTask{
//	    Name: "data-migration",
//	    Tasks: []Task{...},
//	    ContinueOnError: true, // collect all errors
//	}
//
// Composability:
// SequentialTask can contain other composite tasks, allowing unlimited nesting:
//
//	seq := &SequentialTask{
//	    Tasks: []Task{
//	        &ParallelTask{...},
//	        &ConditionalTask{...},
//	        &RetryTask{...},
//	    },
//	}
//
// Thread Safety:
// SequentialTask is safe for concurrent execution. Multiple goroutines can
// execute the same SequentialTask simultaneously, each with their own token.
type SequentialTask struct {
	// Name is the task identifier used in logs, traces, and metrics.
	Name string

	// Tasks is the ordered list of tasks to execute.
	// Tasks are executed in order, and each task receives the same
	// (potentially modified) token from the previous task.
	Tasks []Task

	// ContinueOnError controls error handling behavior.
	// Continue-on-Error Configuration
	//
	// When false (default): Stop execution at first error (original behavior).
	// When true: Continue executing all tasks and collect errors into MultiError.
	//
	// Set to true for scenarios where you want to attempt all tasks regardless
	// of failures (e.g., batch processing, data migration, cleanup operations).
	//
	// Example:
	//   seq := &SequentialTask{
	//       Name: "cleanup",
	//       Tasks: []Task{...},
	//       ContinueOnError: true, // Execute all tasks even if some fail
	//   }
	ContinueOnError bool

	// Timeout is the maximum duration for the entire sequence execution.
	// Task Timeout Configuration
	//
	// If > 0: Entire sequence will be cancelled if it exceeds this duration
	// If = 0: No timeout (unlimited execution time) - default
	//
	// This is the total timeout for ALL tasks in the sequence, not per-task.
	// Individual tasks can have their own timeouts as well.
	//
	// When timeout occurs, the sequence's context is cancelled and
	// context.DeadlineExceeded error is returned.
	//
	// Example:
	//   seq := &SequentialTask{
	//       Name:    "multi-step",
	//       Tasks:   []Task{...},
	//       Timeout: 10 * time.Second, // Total timeout for all tasks
	//   }
	Timeout time.Duration
}

// Execute runs all tasks in sequence.
//
// Respects ContinueOnError configuration
//
// Execution flow:
//  1. Start a trace span for the sequence
//  2. Record sequence execution metric
//  3. Execute each task in order
//  4. Pass the token through the chain (tasks can modify it)
//  5. If ContinueOnError=false (default): stop on first error and return it
//  6. If ContinueOnError=true: continue and collect all errors into MultiError
//  7. Complete the trace span with task count attribute
//
// Thread Safety:
// Execute is safe to call concurrently from multiple goroutines.
// Each execution operates on its own ExecutionContext and token.
//
// Cancellation:
// Context cancellation is checked before each task. If the context is
// cancelled, execution stops and context.Canceled is returned.
//
// Returns an error if:
//   - Name is empty (validation error)
//   - Tasks is empty (validation error)
//   - ContinueOnError=false and any task fails (first error returned)
//   - ContinueOnError=true and one or more tasks fail (MultiError returned)
//   - Context is cancelled
func (s *SequentialTask) Execute(ctx *context.ExecutionContext) error {
	// Validate task configuration
	if s.Name == "" {
		return fmt.Errorf("sequential task: name is required")
	}
	if len(s.Tasks) == 0 {
		return fmt.Errorf("sequential task: tasks list cannot be empty")
	}

	// Create timeout context if configured
	var cancel stdContext.CancelFunc
	if s.Timeout > 0 {
		ctx.Context, cancel = stdContext.WithTimeout(ctx.Context, s.Timeout)
		defer cancel()
	}

	// Default behavior is stop on error (ContinueOnError = false)
	continueOnError := s.ContinueOnError

	// Start trace span
	span := ctx.Tracer.StartSpan("task.sequence." + s.Name)
	defer span.End()
	span.SetAttribute("task_count", len(s.Tasks))
	span.SetAttribute("continue_on_error", continueOnError)
	if s.Timeout > 0 {
		span.SetAttribute("timeout", s.Timeout.String())
	}

	// Record metrics
	ctx.Metrics.Inc("task_executions_total")
	ctx.Metrics.Inc("sequence_tasks_total")

	// Log sequence start
	logFields := map[string]interface{}{
		"task_type":         "sequence",
		"task_name":         s.Name,
		"task_count":        len(s.Tasks),
		"continue_on_error": continueOnError,
		"token_id":          tokenID(ctx),
	}
	if s.Timeout > 0 {
		logFields["timeout"] = s.Timeout.String()
	}
	ctx.Logger.Debug("Executing sequential task", logFields)

	// Error collection for StopOnError=false mode
	multiErr := &MultiError{}

	// Execute each task in order
	for i, task := range s.Tasks {
		// Check for cancellation/timeout before each task
		select {
		case <-ctx.Context.Done():
			err := ctx.Context.Err()
			span.RecordError(err)

			// Distinguish timeout from cancellation
			if err == stdContext.DeadlineExceeded {
				ctx.Metrics.Inc("task_timeout_total")
				ctx.Logger.Error("Sequential task timeout", map[string]interface{}{
					"task_type":       "sequence",
					"task_name":       s.Name,
					"completed_tasks": i,
					"total_tasks":     len(s.Tasks),
					"timeout":         s.Timeout.String(),
				})
				return fmt.Errorf("sequence timeout after %v (completed %d/%d tasks): %w",
					s.Timeout, i, len(s.Tasks), stdContext.DeadlineExceeded)
			}

			ctx.Metrics.Inc("sequence_cancelled_total")
			ctx.Logger.Warn("Sequential task cancelled", map[string]interface{}{
				"task_type":       "sequence",
				"task_name":       s.Name,
				"completed_tasks": i,
				"total_tasks":     len(s.Tasks),
			})
			return err
		default:
		}

		// Execute the task
		ctx.Logger.Debug("Executing sequential task step", map[string]interface{}{
			"task_type":  "sequence",
			"task_name":  s.Name,
			"task_index": i,
			"step":       i + 1,
			"total":      len(s.Tasks),
		})

		err := task.Execute(ctx)
		if err != nil {
			// Record error
			span.RecordError(err)
			ctx.Logger.Error("Sequential task step failed", map[string]interface{}{
				"task_type":   "sequence",
				"task_name":   s.Name,
				"task_index":  i,
				"failed_step": i + 1,
				"total_steps": len(s.Tasks),
				"error":       err.Error(),
			})

			if !continueOnError {
				// Original behavior: stop immediately
				span.SetAttribute("failed_at_step", i+1)
				ctx.Metrics.Inc("sequence_errors_total")
				ctx.Metrics.Inc("sequence_stopped_on_error_total")
				return err
			} else {
				// Continue execution, collect error
				multiErr.Add(err)
			}
		}
	}

	// Check if any errors were collected
	if continueOnError && multiErr.HasErrors() {
		span.SetAttribute("failed_tasks", len(multiErr.Errors))
		ctx.Metrics.Inc("sequence_errors_total")
		ctx.Metrics.Inc("sequence_completed_with_errors_total")
		ctx.Logger.Error("Sequential task completed with errors", map[string]interface{}{
			"task_type":    "sequence",
			"task_name":    s.Name,
			"total_tasks":  len(s.Tasks),
			"failed_tasks": len(multiErr.Errors),
		})
		return multiErr
	}

	// All tasks completed successfully
	ctx.Metrics.Inc("sequence_success_total")
	ctx.Logger.Debug("Sequential task completed", map[string]interface{}{
		"task_type":  "sequence",
		"task_name":  s.Name,
		"task_count": len(s.Tasks),
	})

	return nil
}
