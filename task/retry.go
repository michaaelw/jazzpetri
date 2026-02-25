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
	"fmt"
	"time"

	"github.com/jazzpetri/engine/context"
)

// RetryTask wraps a task with retry logic and exponential backoff.
// This is a composite task that implements the Task interface.
//
// RetryTask is ideal for:
//   - HTTP requests that may fail transiently
//   - Database operations with temporary connection issues
//   - External service calls that need resilience
//
// Retry Strategy:
// The task is retried up to MaxAttempts times. Between attempts, there is
// a backoff delay that can be:
//   - Fixed: Use Backoff field (e.g., 100ms)
//   - Custom: Use BackoffFunc for exponential or custom strategies
//
// If BackoffFunc is provided, it overrides the Backoff field.
//
// Example with fixed backoff:
//
//	retry := &RetryTask{
//	    Name: "fetch-with-retry",
//	    Task: &HTTPTask{URL: "https://api.example.com/data"},
//	    MaxAttempts: 3,
//	    Backoff: 100 * time.Millisecond,
//	}
//
// Example with exponential backoff:
//
//	retry := &RetryTask{
//	    Name: "fetch-with-retry",
//	    Task: &HTTPTask{URL: "https://api.example.com/data"},
//	    MaxAttempts: 5,
//	    BackoffFunc: func(attempt int) time.Duration {
//	        return time.Duration(attempt*attempt) * 100 * time.Millisecond
//	    },
//	}
//
// Cancellation:
// Context cancellation is checked before each retry attempt and during backoff.
// If the context is cancelled, retrying stops immediately.
//
// Composability:
// RetryTask can wrap other composite tasks:
//
//	retry := &RetryTask{
//	    Task: &SequentialTask{...},
//	    MaxAttempts: 3,
//	}
//
// Thread Safety:
// RetryTask is safe for concurrent execution. Multiple goroutines can
// execute the same RetryTask simultaneously.
type RetryTask struct {
	// Name is the task identifier used in logs, traces, and metrics.
	Name string

	// Task is the task to execute with retry logic.
	Task Task

	// MaxAttempts is the maximum number of execution attempts (including the first).
	// Must be at least 1.
	MaxAttempts int

	// Backoff is the fixed delay between retry attempts.
	// This is used if BackoffFunc is nil.
	// Use 0 for no delay (not recommended for production).
	Backoff time.Duration

	// BackoffFunc returns the delay before the next retry attempt.
	// If provided, this overrides the Backoff field.
	// The attempt parameter starts at 1 for the first retry (after initial failure).
	//
	// Example exponential backoff:
	//   func(attempt int) time.Duration {
	//       return time.Duration(1<<uint(attempt)) * 100 * time.Millisecond
	//   }
	BackoffFunc func(attempt int) time.Duration
}

// Execute runs the task with retry logic and backoff.
//
// Execution flow:
//  1. Start a trace span for the retry task
//  2. Record retry execution metric
//  3. Attempt to execute the task (first attempt)
//  4. If successful, return immediately
//  5. If failed, wait for backoff duration (check for cancellation)
//  6. Retry up to MaxAttempts times
//  7. If all attempts fail, return the last error
//  8. Complete the trace span with attempt count
//
// Thread Safety:
// Execute is safe to call concurrently from multiple goroutines.
//
// Cancellation:
// Context cancellation is checked before each attempt and during backoff.
// Uses ctx.Clock.After() for backoff to support testable time.
//
// Returns:
//   - nil if any attempt succeeds
//   - the last error if all attempts fail
//   - validation error if configuration is invalid
//   - context.Canceled if context is cancelled
func (r *RetryTask) Execute(ctx *context.ExecutionContext) error {
	// Validate task configuration
	if r.Name == "" {
		return fmt.Errorf("retry task: name is required")
	}
	if r.Task == nil {
		return fmt.Errorf("retry task: task is required")
	}
	if r.MaxAttempts < 1 {
		return fmt.Errorf("retry task: max attempts must be at least 1")
	}

	// Start trace span
	span := ctx.Tracer.StartSpan("task.retry." + r.Name)
	defer span.End()

	// Record metrics
	ctx.Metrics.Inc("task_executions_total")
	ctx.Metrics.Inc("retry_tasks_total")

	// Log retry start
	ctx.Logger.Debug("Executing retry task", map[string]interface{}{
		"task_name":    r.Name,
		"max_attempts": r.MaxAttempts,
		"token_id":     tokenID(ctx),
	})

	var lastErr error

	// Attempt execution up to MaxAttempts times
	for attempt := 1; attempt <= r.MaxAttempts; attempt++ {
		// Check for cancellation before each attempt
		select {
		case <-ctx.Context.Done():
			err := ctx.Context.Err()
			span.RecordError(err)
			ctx.Metrics.Inc("retry_cancelled_total")
			ctx.Logger.Warn("Retry task cancelled", map[string]interface{}{
				"task_name": r.Name,
				"attempt":   attempt,
			})
			return err
		default:
		}

		// Log attempt
		ctx.Metrics.Inc("retry_attempts_total")
		ctx.Logger.Debug("Retry attempt", map[string]interface{}{
			"task_name": r.Name,
			"attempt":   attempt,
			"max":       r.MaxAttempts,
		})

		// Execute the task
		err := r.Task.Execute(ctx)

		// Success!
		if err == nil {
			span.SetAttribute("attempts", attempt)
			ctx.Metrics.Inc("retry_success_total")
			if attempt > 1 {
				ctx.Logger.Info("Retry task succeeded after retries", map[string]interface{}{
					"task_name": r.Name,
					"attempts":  attempt,
				})
			} else {
				ctx.Logger.Debug("Retry task succeeded on first attempt", map[string]interface{}{
					"task_name": r.Name,
				})
			}
			return nil
		}

		// Failed - save error
		lastErr = err
		ctx.Logger.Warn("Retry attempt failed", map[string]interface{}{
			"task_name": r.Name,
			"attempt":   attempt,
			"max":       r.MaxAttempts,
			"error":     err.Error(),
		})

		// If this was the last attempt, don't backoff
		if attempt == r.MaxAttempts {
			break
		}

		// Calculate backoff duration
		var backoffDuration time.Duration
		if r.BackoffFunc != nil {
			backoffDuration = r.BackoffFunc(attempt)
		} else {
			backoffDuration = r.Backoff
		}

		// Log backoff
		ctx.Logger.Debug("Backing off before retry", map[string]interface{}{
			"task_name": r.Name,
			"attempt":   attempt,
			"backoff":   backoffDuration.String(),
		})

		// Wait for backoff duration (or until cancelled)
		if backoffDuration > 0 {
			timer := ctx.Clock.After(backoffDuration)
			select {
			case <-timer:
				// Backoff complete, continue to next attempt
			case <-ctx.Context.Done():
				// Cancelled during backoff
				err := ctx.Context.Err()
				span.RecordError(err)
				ctx.Metrics.Inc("retry_cancelled_total")
				ctx.Logger.Warn("Retry task cancelled during backoff", map[string]interface{}{
					"task_name": r.Name,
					"attempt":   attempt,
				})
				return err
			}
		}
	}

	// All attempts failed
	span.RecordError(lastErr)
	span.SetAttribute("attempts", r.MaxAttempts)
	span.SetAttribute("failed", true)
	ctx.Metrics.Inc("retry_failure_total")
	ctx.Logger.Error("Retry task failed after all attempts", map[string]interface{}{
		"task_name": r.Name,
		"attempts":  r.MaxAttempts,
		"error":     lastErr.Error(),
	})

	return lastErr
}
