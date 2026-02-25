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

	"github.com/jazzpetri/engine/context"
)

// ConditionalTask executes one of two tasks based on a condition function.
// This is a composite task that implements the Task interface.
//
// ConditionalTask is ideal for:
//   - Branching workflows based on data
//   - Dynamic routing based on conditions
//   - Optional task execution
//
// Condition Evaluation:
// The condition function receives the ExecutionContext and returns a boolean.
// If true, ThenTask is executed. If false, ElseTask is executed (if provided).
//
// Example:
//
//	conditional := &ConditionalTask{
//	    Name: "route-order",
//	    Condition: func(ctx *context.ExecutionContext) bool {
//	        order := ctx.Token.Data.(*OrderToken)
//	        return order.Priority == "high"
//	    },
//	    ThenTask: &HTTPTask{Name: "express-shipping", URL: "..."},
//	    ElseTask: &HTTPTask{Name: "standard-shipping", URL: "..."},
//	}
//
// Optional ElseTask:
// The ElseTask is optional. If not provided and the condition is false,
// the ConditionalTask completes successfully without executing any task.
//
// Composability:
// ConditionalTask can contain other composite tasks, allowing unlimited nesting:
//
//	conditional := &ConditionalTask{
//	    Condition: isHighPriority,
//	    ThenTask: &SequentialTask{...},
//	    ElseTask: &ParallelTask{...},
//	}
//
// Thread Safety:
// ConditionalTask is safe for concurrent execution. Multiple goroutines can
// execute the same ConditionalTask simultaneously.
type ConditionalTask struct {
	// Name is the task identifier used in logs, traces, and metrics.
	Name string

	// Condition is the function that determines which branch to take.
	// Returns true to execute ThenTask, false to execute ElseTask.
	Condition func(ctx *context.ExecutionContext) bool

	// ThenTask is executed when Condition returns true.
	ThenTask Task

	// ElseTask is executed when Condition returns false.
	// If nil, the task completes successfully when condition is false.
	ElseTask Task
}

// Execute evaluates the condition and executes the appropriate task.
//
// Execution flow:
//  1. Start a trace span for the conditional
//  2. Record conditional execution metric
//  3. Evaluate the condition function
//  4. Execute ThenTask if true, ElseTask if false
//  5. Record which branch was taken
//  6. Complete the trace span
//
// Thread Safety:
// Execute is safe to call concurrently from multiple goroutines.
// The condition function must be thread-safe.
//
// Returns an error if:
//   - Name is empty (validation error)
//   - Condition function is nil (validation error)
//   - ThenTask is nil (validation error)
//   - Condition function panics (recovered and returned as error)
//   - The executed task returns an error
func (c *ConditionalTask) Execute(ctx *context.ExecutionContext) error {
	// Validate task configuration
	if c.Name == "" {
		return fmt.Errorf("conditional task: name is required")
	}
	if c.Condition == nil {
		return fmt.Errorf("conditional task: condition function is required")
	}
	if c.ThenTask == nil {
		return fmt.Errorf("conditional task: then task is required")
	}

	// Start trace span
	span := ctx.Tracer.StartSpan("task.conditional." + c.Name)
	defer span.End()

	// Record metrics
	ctx.Metrics.Inc("task_executions_total")
	ctx.Metrics.Inc("conditional_tasks_total")

	// Log conditional start
	ctx.Logger.Debug("Evaluating conditional task", map[string]interface{}{
		"task_name": c.Name,
		"token_id":  tokenID(ctx),
	})

	// Evaluate condition with panic recovery
	var conditionResult bool
	var conditionErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				conditionErr = fmt.Errorf("conditional task %s: condition panicked: %v", c.Name, r)
				span.RecordError(conditionErr)
			}
		}()
		conditionResult = c.Condition(ctx)
	}()

	// Check for condition evaluation error
	if conditionErr != nil {
		ctx.Metrics.Inc("conditional_errors_total")
		ctx.Logger.Error("Conditional task condition failed", map[string]interface{}{
			"task_name": c.Name,
			"error":     conditionErr.Error(),
		})
		return conditionErr
	}

	// Record which branch was taken
	span.SetAttribute("condition_result", conditionResult)
	if conditionResult {
		ctx.Metrics.Inc("condition_true_total")
	} else {
		ctx.Metrics.Inc("condition_false_total")
	}

	ctx.Logger.Debug("Conditional evaluated", map[string]interface{}{
		"task_name": c.Name,
		"result":    conditionResult,
	})

	// Execute the appropriate task
	var err error
	if conditionResult {
		ctx.Logger.Debug("Executing then branch", map[string]interface{}{
			"task_name": c.Name,
		})
		err = c.ThenTask.Execute(ctx)
	} else if c.ElseTask != nil {
		ctx.Logger.Debug("Executing else branch", map[string]interface{}{
			"task_name": c.Name,
		})
		err = c.ElseTask.Execute(ctx)
	} else {
		ctx.Logger.Debug("Else branch skipped (no else task)", map[string]interface{}{
			"task_name": c.Name,
		})
	}

	// Handle error
	if err != nil {
		span.RecordError(err)
		ctx.Metrics.Inc("conditional_errors_total")
		ctx.Logger.Error("Conditional task failed", map[string]interface{}{
			"task_name": c.Name,
			"branch":    map[bool]string{true: "then", false: "else"}[conditionResult],
			"error":     err.Error(),
		})
		return err
	}

	// Success
	ctx.Metrics.Inc("conditional_success_total")
	ctx.Logger.Debug("Conditional task completed", map[string]interface{}{
		"task_name": c.Name,
		"branch":    map[bool]string{true: "then", false: "else"}[conditionResult],
	})

	return nil
}
