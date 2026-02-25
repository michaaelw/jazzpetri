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

// WaitTask pauses workflow execution for a specified duration or until a deadline.
// This enables time-based progression, SLA boundaries, and scheduling patterns.
//
// WaitTask is ideal for:
//   - Adding delays between workflow steps
//   - Implementing rate limiting
//   - Waiting for external processes to complete
//   - SLA timeout boundaries
//   - Scheduled task execution
//
// Example - Fixed delay:
//
//	task := &WaitTask{
//	    Name:     "cool-down",
//	    Duration: 5 * time.Second,
//	}
//
// Example - Until specific time:
//
//	task := &WaitTask{
//	    Name:     "wait-until-9am",
//	    Until:    time.Date(2024, 1, 15, 9, 0, 0, 0, time.Local),
//	}
//
// Example - Dynamic duration:
//
//	task := &WaitTask{
//	    Name: "dynamic-wait",
//	    DurationFunc: func(ctx *context.ExecutionContext) time.Duration {
//	        // Calculate wait time based on token data
//	        priority := ctx.Token.Data.(*Order).Priority
//	        if priority == "high" {
//	            return 1 * time.Second
//	        }
//	        return 10 * time.Second
//	    },
//	}
//
// Thread Safety:
// WaitTask is safe for concurrent execution. Each execution uses its own
// timer and respects context cancellation.
//
// Cancellation:
// WaitTask respects ctx.Context cancellation. If the context is cancelled
// during the wait, the task returns immediately with the context error.
type WaitTask struct {
	// Name is the task identifier used in logs, traces, and metrics.
	Name string

	// Duration is the fixed amount of time to wait.
	// Ignored if DurationFunc is set.
	Duration time.Duration

	// Until is a specific time to wait until.
	// If set and in the past, the task completes immediately.
	// Takes precedence over Duration if both are set.
	Until time.Time

	// DurationFunc calculates the wait duration dynamically based on context.
	// This allows wait times to be determined at runtime based on token data.
	// Takes precedence over Duration if both are set.
	DurationFunc func(ctx *context.ExecutionContext) time.Duration
}

// Execute pauses execution for the specified duration.
//
// Execution flow:
//  1. Validate task configuration
//  2. Start trace span
//  3. Calculate wait duration (from Until, DurationFunc, or Duration)
//  4. Wait using Clock.Sleep (respects VirtualClock for testing)
//  5. Handle context cancellation during wait
//  6. Record metrics and complete
//
// The task uses ctx.Clock.Sleep() which allows:
//   - Testability with VirtualClock (instant advancement)
//   - Proper time tracking in distributed tracing
//   - Context cancellation support
//
// Thread Safety:
// Execute is safe to call concurrently from multiple goroutines.
//
// Returns an error if:
//   - Task configuration is invalid (missing Name, no duration specified)
//   - Context is cancelled during the wait
func (w *WaitTask) Execute(ctx *context.ExecutionContext) error {
	// Validate configuration
	if w.Name == "" {
		return fmt.Errorf("wait task: name is required")
	}

	// Calculate wait duration
	var waitDuration time.Duration

	if !w.Until.IsZero() {
		// Wait until specific time
		waitDuration = w.Until.Sub(ctx.Clock.Now())
		if waitDuration < 0 {
			waitDuration = 0 // Already past, complete immediately
		}
	} else if w.DurationFunc != nil {
		// Dynamic duration
		waitDuration = w.DurationFunc(ctx)
	} else if w.Duration > 0 {
		// Fixed duration
		waitDuration = w.Duration
	} else {
		return fmt.Errorf("wait task: duration, until, or duration_func is required")
	}

	// Start trace span
	span := ctx.Tracer.StartSpan("task.wait." + w.Name)
	defer span.End()

	span.SetAttribute("wait.duration", waitDuration.String())
	if !w.Until.IsZero() {
		span.SetAttribute("wait.until", w.Until.Format(time.RFC3339))
	}

	// Record metrics
	ctx.Metrics.Inc("task_executions_total")
	ctx.Metrics.Inc("task_wait_executions_total")

	// Log task start
	ctx.Logger.Debug("Executing wait task", map[string]interface{}{
		"task_name":     w.Name,
		"wait_duration": waitDuration.String(),
		"token_id":      tokenID(ctx),
	})

	// If no wait needed, complete immediately
	if waitDuration <= 0 {
		ctx.Metrics.Inc("task_success_total")
		ctx.Metrics.Inc("task_wait_success_total")
		ctx.Logger.Debug("Wait task completed immediately (duration <= 0)", map[string]interface{}{
			"task_name": w.Name,
			"token_id":  tokenID(ctx),
		})
		return nil
	}

	// Wait with cancellation support
	startTime := ctx.Clock.Now()

	// Create a channel for the sleep completion
	done := make(chan struct{})
	go func() {
		ctx.Clock.Sleep(waitDuration)
		close(done)
	}()

	// Wait for either completion or cancellation
	select {
	case <-done:
		// Normal completion
		actualDuration := ctx.Clock.Now().Sub(startTime)
		ctx.Metrics.Inc("task_success_total")
		ctx.Metrics.Inc("task_wait_success_total")
		ctx.Metrics.Observe("task_wait_duration_seconds", actualDuration.Seconds())

		ctx.Logger.Debug("Wait task completed", map[string]interface{}{
			"task_name":       w.Name,
			"wait_duration":   waitDuration.String(),
			"actual_duration": actualDuration.String(),
			"token_id":        tokenID(ctx),
		})
		return nil

	case <-ctx.Context.Done():
		// Context cancelled
		err := ctx.Context.Err()
		span.RecordError(err)
		ctx.Metrics.Inc("task_errors_total")
		ctx.Metrics.Inc("task_wait_cancelled_total")
		ctx.Logger.Warn("Wait task cancelled", map[string]interface{}{
			"task_name":     w.Name,
			"wait_duration": waitDuration.String(),
			"error":         err.Error(),
			"token_id":      tokenID(ctx),
		})
		return err
	}
}
