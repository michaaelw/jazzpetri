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

// InlineTask executes a Go function directly within the workflow engine.
// This is the simplest and most efficient task type for in-process operations.
//
// InlineTask is ideal for:
//   - Data transformations
//   - Business logic execution
//   - Token data manipulation
//   - Synchronous operations
//
// Example:
//
//	task := &InlineTask{
//	    Name: "calculate-total",
//	    Fn: func(ctx *context.ExecutionContext) error {
//	        order := ctx.Token.Data.(OrderData)
//	        total := order.Items.Sum()
//	        order.Total = total
//	        ctx.Logger.Info("Calculated total", map[string]interface{}{
//	            "order_id": order.ID,
//	            "total": total,
//	        })
//	        return nil
//	    },
//	}
//
// Thread Safety:
// InlineTask is safe for concurrent execution. Multiple goroutines can
// execute the same InlineTask simultaneously. The task function Fn receives
// its own ExecutionContext with its own token, ensuring isolation.
//
// Error Handling:
// If Fn panics, the panic is recovered and converted to an error.
// The error is then processed by the ExecutionContext's ErrorHandler.
//
// Timeout Support :
// If Timeout is set to a positive duration, the task will be cancelled
// if it exceeds the timeout. When timeout occurs, context.DeadlineExceeded
// error is returned. A timeout of 0 means no timeout (unlimited execution time).
type InlineTask struct {
	// Name is the task identifier used in logs, traces, and metrics.
	// Should be descriptive and unique within the workflow.
	Name string

	// Fn is the function to execute.
	// It receives the ExecutionContext with access to:
	//   - Token data (ctx.Token)
	//   - Clock (ctx.Clock)
	//   - Observability (ctx.Tracer, ctx.Metrics, ctx.Logger)
	//   - Error handling (ctx.ErrorHandler, ctx.ErrorRecorder)
	//   - Cancellation (ctx.Context)
	//
	// The function should return an error if execution fails.
	// Panics are recovered and converted to errors.
	Fn func(ctx *context.ExecutionContext) error

	// Timeout is the maximum duration for task execution.
	// Task Timeout Configuration
	//
	// If > 0: Task will be cancelled if it exceeds this duration
	// If = 0: No timeout (unlimited execution time) - default
	//
	// When timeout occurs, the task's context is cancelled and
	// context.DeadlineExceeded error is returned.
	//
	// Example:
	//   task := &InlineTask{
	//       Name:    "api-call",
	//       Fn:      callExternalAPI,
	//       Timeout: 5 * time.Second,
	//   }
	Timeout time.Duration
}

// Execute runs the inline task function with observability and error handling.
//
// Execution flow:
//  1. Create timeout context if Timeout > 0 
//  2. Start a trace span for the task
//  3. Record task execution metric
//  4. Execute the function with panic recovery
//  5. Check for timeout (context.DeadlineExceeded)
//  6. Handle errors through ErrorHandler
//  7. Record error if execution fails
//  8. Complete the trace span
//
// Thread Safety:
// Execute is safe to call concurrently from multiple goroutines.
// Each execution receives its own ExecutionContext.
//
// Returns an error if:
//   - The task function returns an error
//   - The task function panics (panic is converted to error)
//   - The task exceeds its timeout (context.DeadlineExceeded)
//   - Name or Fn is not set (validation error)
func (t *InlineTask) Execute(ctx *context.ExecutionContext) error {
	// Validate task configuration
	if t.Name == "" {
		return fmt.Errorf("inline task: name is required")
	}
	if t.Fn == nil {
		return fmt.Errorf("inline task: function is required")
	}

	// Create timeout context if configured
	var cancel stdContext.CancelFunc
	if t.Timeout > 0 {
		ctx.Context, cancel = stdContext.WithTimeout(ctx.Context, t.Timeout)
		defer cancel()
	}

	// Start trace span
	span := ctx.Tracer.StartSpan("task.inline." + t.Name)
	defer span.End()
	if t.Timeout > 0 {
		span.SetAttribute("timeout", t.Timeout.String())
	}

	// Record metrics
	ctx.Metrics.Inc("task_executions_total")
	ctx.Metrics.Inc("task_inline_executions_total")

	// Log task start
	logFields := map[string]interface{}{
		"task_type": "inline",
		"task_name": t.Name,
		"token_id":  tokenID(ctx),
	}
	if t.Timeout > 0 {
		logFields["timeout"] = t.Timeout.String()
	}
	ctx.Logger.Debug("Executing inline task", logFields)

	// Execute function with panic recovery
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("inline task %s panicked: %v", t.Name, r)
				span.RecordError(err)
				ctx.ErrorRecorder.RecordError(err, map[string]interface{}{
					"task_name": t.Name,
					"token_id":  tokenID(ctx),
					"panic":     true,
				})
			}
		}()
		err = t.Fn(ctx)
	}()

	// Handle error
	if err != nil {
		// Check if error is due to timeout
		if err == stdContext.DeadlineExceeded || ctx.Context.Err() == stdContext.DeadlineExceeded {
			span.RecordError(err)
			ctx.Metrics.Inc("task_timeout_total")
			ctx.Logger.Error("Inline task timeout", map[string]interface{}{
				"task_type": "inline",
				"task_name": t.Name,
				"token_id":  tokenID(ctx),
				"timeout":   t.Timeout.String(),
			})
			return fmt.Errorf("task timeout after %v: %w", t.Timeout, stdContext.DeadlineExceeded)
		}

		span.RecordError(err)
		ctx.Metrics.Inc("task_errors_total")
		ctx.Metrics.Inc("task_inline_errors_total")
		ctx.ErrorRecorder.RecordError(err, map[string]interface{}{
			"task_name": t.Name,
			"token_id":  tokenID(ctx),
		})
		ctx.Logger.Error("Inline task failed", map[string]interface{}{
			"task_type": "inline",
			"task_name": t.Name,
			"token_id":  tokenID(ctx),
			"error":     err.Error(),
		})
		// Let ErrorHandler process the error
		return ctx.ErrorHandler.Handle(ctx, err)
	}

	// Success
	ctx.Metrics.Inc("task_success_total")
	ctx.Metrics.Inc("task_inline_success_total")
	ctx.Logger.Debug("Inline task completed", map[string]interface{}{
		"task_type": "inline",
		"task_name": t.Name,
		"token_id":  tokenID(ctx),
	})

	return nil
}

// tokenID safely extracts token ID from context, returning empty string if token is nil
func tokenID(ctx *context.ExecutionContext) string {
	if ctx.Token != nil {
		return ctx.Token.ID
	}
	return ""
}
