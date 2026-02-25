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

// RecoveryTask wraps a task with error recovery and compensation logic.
// This is a composite task that implements the Task interface.
//
// RecoveryTask is ideal for:
//   - Saga patterns for distributed transactions
//   - Compensating transactions that need to rollback
//   - Error recovery workflows with cleanup actions
//
// Recovery Strategy:
// The primary task is executed first. If it succeeds, RecoveryTask succeeds.
// If it fails, the CompensationTask is executed to handle the error (rollback, cleanup, etc.).
//
// The CompensationTask's error (if any) is returned, not the original error.
// This allows the compensation to either:
//   - Succeed: returning nil indicates successful recovery
//   - Fail: returning an error indicates compensation failed
//
// Example:
//
//	recovery := &RecoveryTask{
//	    Name: "payment-with-rollback",
//	    Task: &HTTPTask{
//	        URL: "https://payment.example.com/charge",
//	        Method: "POST",
//	    },
//	    CompensationTask: &HTTPTask{
//	        URL: "https://payment.example.com/refund",
//	        Method: "POST",
//	    },
//	}
//
// Saga Pattern Example:
//
//	saga := &SequentialTask{
//	    Tasks: []Task{
//	        &RecoveryTask{
//	            Task: &InlineTask{Name: "reserve-inventory", Fn: reserveInventory},
//	            CompensationTask: &InlineTask{Name: "release-inventory", Fn: releaseInventory},
//	        },
//	        &RecoveryTask{
//	            Task: &InlineTask{Name: "charge-payment", Fn: chargePayment},
//	            CompensationTask: &InlineTask{Name: "refund-payment", Fn: refundPayment},
//	        },
//	    },
//	}
//
// Composability:
// RecoveryTask can wrap other composite tasks:
//
//	recovery := &RecoveryTask{
//	    Task: &SequentialTask{...},
//	    CompensationTask: &SequentialTask{...},
//	}
//
// Thread Safety:
// RecoveryTask is safe for concurrent execution. Multiple goroutines can
// execute the same RecoveryTask simultaneously.
type RecoveryTask struct {
	// Name is the task identifier used in logs, traces, and metrics.
	Name string

	// Task is the primary task to execute.
	Task Task

	// CompensationTask is executed if Task fails.
	// This task should handle error recovery, rollback, or cleanup.
	// If CompensationTask succeeds (returns nil), the RecoveryTask succeeds.
	// If CompensationTask fails, its error is returned.
	CompensationTask Task
}

// Execute runs the primary task and executes compensation on failure.
//
// Execution flow:
//  1. Start a trace span for the recovery task
//  2. Record recovery execution metric
//  3. Execute the primary task
//  4. If successful, return immediately
//  5. If failed, execute the compensation task
//  6. Return the compensation result (nil or error)
//  7. Complete the trace span
//
// Thread Safety:
// Execute is safe to call concurrently from multiple goroutines.
//
// Returns:
//   - nil if primary task succeeds
//   - nil if primary task fails but compensation succeeds
//   - error if compensation fails (the compensation error, not the original)
//   - validation error if configuration is invalid
func (r *RecoveryTask) Execute(ctx *context.ExecutionContext) error {
	// Validate task configuration
	if r.Name == "" {
		return fmt.Errorf("recovery task: name is required")
	}
	if r.Task == nil {
		return fmt.Errorf("recovery task: task is required")
	}
	if r.CompensationTask == nil {
		return fmt.Errorf("recovery task: compensation task is required")
	}

	// Start trace span
	span := ctx.Tracer.StartSpan("task.recovery." + r.Name)
	defer span.End()

	// Record metrics
	ctx.Metrics.Inc("task_executions_total")
	ctx.Metrics.Inc("recovery_tasks_total")

	// Log recovery start
	ctx.Logger.Debug("Executing recovery task", map[string]interface{}{
		"task_name": r.Name,
		"token_id":  tokenID(ctx),
	})

	// Execute the primary task
	err := r.Task.Execute(ctx)

	// Success - no compensation needed
	if err == nil {
		ctx.Metrics.Inc("recovery_success_total")
		ctx.Logger.Debug("Recovery task completed successfully", map[string]interface{}{
			"task_name":    r.Name,
			"compensated": false,
		})
		return nil
	}

	// Primary task failed - execute compensation
	span.SetAttribute("primary_failed", true)
	span.RecordError(err)
	ctx.Metrics.Inc("compensation_executed_total")

	ctx.Logger.Warn("Primary task failed, executing compensation", map[string]interface{}{
		"task_name":     r.Name,
		"primary_error": err.Error(),
	})

	// Start a sub-span for compensation
	compSpan := ctx.Tracer.StartSpan("task.recovery.compensation." + r.Name)
	defer compSpan.End()

	// Execute compensation
	compErr := r.CompensationTask.Execute(ctx)

	// Compensation succeeded
	if compErr == nil {
		ctx.Metrics.Inc("compensation_success_total")
		ctx.Logger.Info("Compensation succeeded, recovery complete", map[string]interface{}{
			"task_name":     r.Name,
			"primary_error": err.Error(),
		})
		return nil
	}

	// Compensation failed
	compSpan.RecordError(compErr)
	span.SetAttribute("compensation_failed", true)
	ctx.Metrics.Inc("compensation_errors_total")
	ctx.Metrics.Inc("recovery_errors_total")

	ctx.Logger.Error("Compensation failed", map[string]interface{}{
		"task_name":          r.Name,
		"primary_error":      err.Error(),
		"compensation_error": compErr.Error(),
	})

	// Return compensation error (not the original error)
	return compErr
}
