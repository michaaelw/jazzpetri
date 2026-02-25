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

// Package task provides the Task interface and built-in task adapters for Jazz3.
//
// Tasks are the units of work executed by transitions in the Petri net.
// The task package implements the Ports & Adapters pattern:
//   - Port: Task interface
//   - Adapters: InlineTask, HTTPTask, CommandTask
//
// All tasks:
//   - Execute within an ExecutionContext for observability
//   - Are thread-safe for concurrent execution
//   - Handle errors through ExecutionContext.ErrorHandler
//   - Support cancellation via context.Context
//
// Example usage:
//
//	// Inline task for simple Go functions
//	task := &task.InlineTask{
//	    Name: "process-order",
//	    Fn: func(ctx *context.ExecutionContext) error {
//	        // Access token data
//	        order := ctx.Token.Data.(OrderData)
//	        // Process order...
//	        return nil
//	    },
//	}
//
//	// HTTP task for external services
//	httpTask := &task.HTTPTask{
//	    Name:   "notify-service",
//	    Method: "POST",
//	    URL:    "https://api.example.com/notify",
//	    Headers: map[string]string{"Content-Type": "application/json"},
//	}
//
//	// Command task for subprocess execution
//	cmdTask := &task.CommandTask{
//	    Name:    "run-analysis",
//	    Command: "python",
//	    Args:    []string{"analyze.py", "--input", "data.csv"},
//	}
package task

import (
	"github.com/jazzpetri/engine/context"
)

// Task is the core interface for all tasks in Jazz3.
// Tasks execute within an ExecutionContext which provides access to:
//   - Token being processed (ctx.Token)
//   - Clock for time operations (ctx.Clock)
//   - Observability tools (ctx.Tracer, ctx.Metrics, ctx.Logger)
//   - Error handling (ctx.ErrorHandler, ctx.ErrorRecorder)
//   - Cancellation support (ctx.Context)
//
// Implementations must be thread-safe for concurrent execution by multiple
// transitions. Task configuration should be immutable after creation.
//
// Error Handling:
// Tasks should return errors directly. The ExecutionContext.ErrorHandler
// will process errors for retry/compensation logic.
//
// Observability:
// Tasks should use the ExecutionContext's observability tools:
//   - Start spans for tracing: ctx.Tracer.StartSpan("task-name")
//   - Record metrics: ctx.Metrics.Inc("task_executions")
//   - Log events: ctx.Logger.Info("Task started", fields)
//
// Cancellation:
// Tasks should respect ctx.Context cancellation for graceful shutdown:
//
//	select {
//	case <-ctx.Context.Done():
//	    return ctx.Context.Err()
//	case result := <-processingChan:
//	    // Handle result...
//	}
type Task interface {
	// Execute runs the task with the given execution context.
	// The context provides access to the token, clock, and observability tools.
	//
	// Implementations should:
	//   - Use ctx.Tracer.StartSpan() to create trace spans
	//   - Use ctx.Metrics to record task execution metrics
	//   - Use ctx.Logger for structured logging
	//   - Respect ctx.Context for cancellation
	//   - Return errors directly (ErrorHandler will process them)
	//
	// The ExecutionContext is fully configured and ready to use.
	// Tasks should not modify the ExecutionContext itself.
	//
	// Thread Safety:
	// Execute may be called concurrently by multiple goroutines.
	// Implementations must be safe for concurrent execution.
	//
	// Returns an error if the task execution fails. The error will be
	// processed by the ExecutionContext's ErrorHandler.
	Execute(ctx *context.ExecutionContext) error
}
