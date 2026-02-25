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

// WorkflowInvoker is the interface for executing sub-workflows.
// This abstraction allows the task package to invoke workflows without
// depending on the engine package directly.
//
// This is the PORT in the Ports & Adapters pattern. The concrete implementation
// lives in the engine package and is injected at runtime.
type WorkflowInvoker interface {
	// StartWorkflow starts a new workflow instance.
	// Returns the instance ID for tracking.
	StartWorkflow(ctx *context.ExecutionContext, workflowID string, input interface{}) (string, error)

	// WaitForCompletion waits for a workflow instance to complete.
	// Returns the final output or an error.
	WaitForCompletion(ctx *context.ExecutionContext, instanceID string, timeout time.Duration) (*WorkflowResult, error)

	// GetWorkflowStatus returns the current status of a workflow instance.
	GetWorkflowStatus(ctx *context.ExecutionContext, instanceID string) (*WorkflowStatus, error)

	// CancelWorkflow cancels a running workflow instance.
	CancelWorkflow(ctx *context.ExecutionContext, instanceID string) error
}

// WorkflowResult contains the output from a completed workflow.
type WorkflowResult struct {
	// InstanceID is the unique identifier of the workflow instance.
	InstanceID string

	// WorkflowID is the workflow definition ID.
	WorkflowID string

	// Status is the final status.
	Status WorkflowStatusCode

	// Output is the workflow output data.
	Output interface{}

	// StartedAt is when the workflow started.
	StartedAt time.Time

	// CompletedAt is when the workflow completed.
	CompletedAt time.Time

	// Duration is the total execution time.
	Duration time.Duration

	// Metadata contains additional workflow data.
	Metadata map[string]interface{}
}

// WorkflowStatus represents the current state of a workflow instance.
type WorkflowStatus struct {
	// InstanceID is the unique identifier of the workflow instance.
	InstanceID string

	// WorkflowID is the workflow definition ID.
	WorkflowID string

	// Status is the current status.
	Status WorkflowStatusCode

	// CurrentPlace is the current place in the Petri net (if applicable).
	CurrentPlace string

	// Progress is the completion percentage (0-100).
	Progress int

	// StartedAt is when the workflow started.
	StartedAt time.Time

	// Metadata contains additional status data.
	Metadata map[string]interface{}
}

// WorkflowStatusCode represents workflow execution states.
type WorkflowStatusCode int

const (
	// WorkflowStatusPending indicates the workflow is queued.
	WorkflowStatusPending WorkflowStatusCode = iota

	// WorkflowStatusRunning indicates the workflow is executing.
	WorkflowStatusRunning

	// WorkflowStatusCompleted indicates successful completion.
	WorkflowStatusCompleted

	// WorkflowStatusFailed indicates the workflow failed.
	WorkflowStatusFailed

	// WorkflowStatusCancelled indicates the workflow was cancelled.
	WorkflowStatusCancelled

	// WorkflowStatusTimedOut indicates the workflow timed out.
	WorkflowStatusTimedOut
)

// String returns the string representation of the status code.
func (s WorkflowStatusCode) String() string {
	switch s {
	case WorkflowStatusPending:
		return "pending"
	case WorkflowStatusRunning:
		return "running"
	case WorkflowStatusCompleted:
		return "completed"
	case WorkflowStatusFailed:
		return "failed"
	case WorkflowStatusCancelled:
		return "cancelled"
	case WorkflowStatusTimedOut:
		return "timed_out"
	default:
		return "unknown"
	}
}

// SubWorkflowTask invokes a nested workflow as part of the parent workflow.
// This enables workflow composition, reuse, and hierarchical orchestration.
//
// SubWorkflowTask is ideal for:
//   - Breaking complex workflows into reusable components
//   - Parallel execution of independent sub-processes
//   - Conditional workflow branches
//   - Workflow templates with parameterization
//   - Hierarchical process orchestration
//
// Example - Simple sub-workflow:
//
//	task := &SubWorkflowTask{
//	    Name:       "process-order",
//	    Invoker:    workflowInvoker,
//	    WorkflowID: "order-processing",
//	    InputFunc: func(ctx *context.ExecutionContext) (interface{}, error) {
//	        order := ctx.Token.Data.(*OrderData)
//	        return map[string]interface{}{
//	            "order_id": order.ID,
//	            "items":    order.Items,
//	        }, nil
//	    },
//	}
//
// Example - With timeout and failure handling:
//
//	task := &SubWorkflowTask{
//	    Name:       "verify-identity",
//	    Invoker:    workflowInvoker,
//	    WorkflowID: "identity-verification",
//	    Timeout:    5 * time.Minute,
//	    OnSuccess: func(ctx *context.ExecutionContext, result *WorkflowResult) error {
//	        ctx.Token.Metadata["verified"] = result.Output
//	        return nil
//	    },
//	    OnFailure: func(ctx *context.ExecutionContext, result *WorkflowResult) error {
//	        // Escalate to manual review
//	        return ctx.EmitEvent("manual-review-needed", result)
//	    },
//	}
//
// Example - Fire and forget (async):
//
//	task := &SubWorkflowTask{
//	    Name:       "send-notifications",
//	    Invoker:    workflowInvoker,
//	    WorkflowID: "notification-batch",
//	    Async:      true,  // Don't wait for completion
//	    Input: map[string]interface{}{
//	        "template": "order-confirmation",
//	    },
//	}
//
// Thread Safety:
// SubWorkflowTask is safe for concurrent execution. The WorkflowInvoker
// implementation must also be thread-safe.
type SubWorkflowTask struct {
	// Name is the task identifier used in logs, traces, and metrics.
	Name string

	// Invoker is the workflow execution backend.
	// This is the adapter that connects to the workflow engine.
	Invoker WorkflowInvoker

	// WorkflowID is the ID of the workflow to invoke.
	WorkflowID string

	// WorkflowIDFunc generates the workflow ID dynamically.
	// Takes precedence over WorkflowID if both are set.
	WorkflowIDFunc func(ctx *context.ExecutionContext) (string, error)

	// Input is the static input to pass to the sub-workflow.
	// Ignored if InputFunc is set.
	Input interface{}

	// InputFunc generates the input dynamically from context.
	// Takes precedence over Input if both are set.
	InputFunc func(ctx *context.ExecutionContext) (interface{}, error)

	// Async runs the sub-workflow without waiting for completion.
	// The instance ID is stored in token metadata for later tracking.
	Async bool

	// Timeout is how long to wait for the sub-workflow to complete.
	// Only applies when Async is false.
	// Zero means no timeout (wait indefinitely).
	Timeout time.Duration

	// OnSuccess is called when the sub-workflow completes successfully.
	OnSuccess func(ctx *context.ExecutionContext, result *WorkflowResult) error

	// OnFailure is called when the sub-workflow fails.
	// If nil, failure returns an error.
	OnFailure func(ctx *context.ExecutionContext, result *WorkflowResult) error

	// OnTimeout is called when the sub-workflow times out.
	// If nil, timeout returns an error.
	OnTimeout func(ctx *context.ExecutionContext, status *WorkflowStatus) error

	// PropagateCancel determines if parent cancellation should cancel sub-workflow.
	PropagateCancel bool
}

// Execute invokes the sub-workflow.
//
// Execution flow (sync mode):
//  1. Validate task configuration
//  2. Start trace span
//  3. Generate workflow ID (from WorkflowIDFunc or static WorkflowID)
//  4. Generate input (from InputFunc or static Input)
//  5. Start sub-workflow via Invoker.StartWorkflow
//  6. Wait for completion via Invoker.WaitForCompletion
//  7. Handle result (OnSuccess/OnFailure callbacks)
//  8. Record metrics and complete
//
// Execution flow (async mode):
//  1. Validate task configuration
//  2. Start trace span
//  3. Generate workflow ID and input
//  4. Start sub-workflow via Invoker.StartWorkflow
//  5. Store instance ID in token metadata
//  6. Return immediately (don't wait)
//
// Thread Safety:
// Execute is safe to call concurrently from multiple goroutines.
//
// Returns an error if:
//   - Task configuration is invalid (missing Name, Invoker, WorkflowID)
//   - Workflow ID or input generation fails
//   - Sub-workflow invocation fails
//   - Sub-workflow fails or times out (unless handled by callbacks)
func (s *SubWorkflowTask) Execute(ctx *context.ExecutionContext) error {
	// Validate configuration
	if s.Name == "" {
		return fmt.Errorf("sub-workflow task: name is required")
	}
	if s.Invoker == nil {
		return fmt.Errorf("sub-workflow task: invoker is required")
	}

	// Start trace span
	span := ctx.Tracer.StartSpan("task.subworkflow." + s.Name)
	defer span.End()

	// Record metrics
	ctx.Metrics.Inc("task_executions_total")
	ctx.Metrics.Inc("task_subworkflow_executions_total")

	// Get workflow ID
	var workflowID string
	var err error

	if s.WorkflowIDFunc != nil {
		workflowID, err = s.WorkflowIDFunc(ctx)
		if err != nil {
			return s.handleError(ctx, span, fmt.Errorf("workflow ID generation failed: %w", err))
		}
	} else {
		workflowID = s.WorkflowID
	}

	if workflowID == "" {
		return s.handleError(ctx, span, fmt.Errorf("sub-workflow task: workflow ID is required"))
	}

	span.SetAttribute("subworkflow.workflow_id", workflowID)
	span.SetAttribute("subworkflow.async", s.Async)

	// Get input
	var input interface{}

	if s.InputFunc != nil {
		input, err = s.InputFunc(ctx)
		if err != nil {
			return s.handleError(ctx, span, fmt.Errorf("input generation failed: %w", err))
		}
	} else {
		input = s.Input
	}

	// Log task start
	ctx.Logger.Debug("Starting sub-workflow", map[string]interface{}{
		"task_name":   s.Name,
		"workflow_id": workflowID,
		"async":       s.Async,
		"token_id":    tokenID(ctx),
	})

	// Start sub-workflow
	instanceID, err := s.Invoker.StartWorkflow(ctx, workflowID, input)
	if err != nil {
		return s.handleError(ctx, span, fmt.Errorf("failed to start sub-workflow: %w", err))
	}

	span.SetAttribute("subworkflow.instance_id", instanceID)
	ctx.Metrics.Inc("task_subworkflow_started_total")

	ctx.Logger.Info("Sub-workflow started", map[string]interface{}{
		"task_name":   s.Name,
		"workflow_id": workflowID,
		"instance_id": instanceID,
		"async":       s.Async,
		"token_id":    tokenID(ctx),
	})

	// Store instance ID in metadata
	ctx.Token.Metadata["subworkflow_instance_id"] = instanceID
	ctx.Token.Metadata["subworkflow_workflow_id"] = workflowID

	// Async mode: return immediately
	if s.Async {
		ctx.Metrics.Inc("task_success_total")
		ctx.Metrics.Inc("task_subworkflow_success_total")
		ctx.Logger.Debug("Sub-workflow task completed (async)", map[string]interface{}{
			"task_name":   s.Name,
			"instance_id": instanceID,
			"token_id":    tokenID(ctx),
		})
		return nil
	}

	// Sync mode: wait for completion
	return s.waitForCompletion(ctx, span, instanceID, workflowID)
}

func (s *SubWorkflowTask) waitForCompletion(ctx *context.ExecutionContext, span context.Span, instanceID, workflowID string) error {
	// Set up cancellation propagation if enabled
	if s.PropagateCancel {
		go func() {
			<-ctx.Context.Done()
			// Parent cancelled, cancel sub-workflow
			if err := s.Invoker.CancelWorkflow(ctx, instanceID); err != nil {
				ctx.Logger.Warn("Failed to cancel sub-workflow", map[string]interface{}{
					"instance_id": instanceID,
					"error":       err.Error(),
				})
			}
		}()
	}

	// Wait for completion
	result, err := s.Invoker.WaitForCompletion(ctx, instanceID, s.Timeout)
	if err != nil {
		// Check if it's a timeout
		if s.OnTimeout != nil {
			status, statusErr := s.Invoker.GetWorkflowStatus(ctx, instanceID)
			if statusErr == nil {
				ctx.Metrics.Inc("task_subworkflow_timeout_total")
				if timeoutErr := s.OnTimeout(ctx, status); timeoutErr != nil {
					return s.handleError(ctx, span, fmt.Errorf("timeout handler failed: %w", timeoutErr))
				}
				return nil
			}
		}
		return s.handleError(ctx, span, fmt.Errorf("failed to wait for sub-workflow: %w", err))
	}

	span.SetAttribute("subworkflow.status", result.Status.String())
	span.SetAttribute("subworkflow.duration_ms", result.Duration.Milliseconds())

	// Handle result based on status
	switch result.Status {
	case WorkflowStatusCompleted:
		ctx.Metrics.Inc("task_subworkflow_completed_total")
		ctx.Token.Metadata["subworkflow_output"] = result.Output
		ctx.Token.Metadata["subworkflow_duration"] = result.Duration

		if s.OnSuccess != nil {
			if err := s.OnSuccess(ctx, result); err != nil {
				return s.handleError(ctx, span, fmt.Errorf("on_success handler failed: %w", err))
			}
		}

		ctx.Logger.Info("Sub-workflow completed", map[string]interface{}{
			"task_name":   s.Name,
			"instance_id": instanceID,
			"duration_ms": result.Duration.Milliseconds(),
			"token_id":    tokenID(ctx),
		})

	case WorkflowStatusFailed:
		ctx.Metrics.Inc("task_subworkflow_failed_total")

		if s.OnFailure != nil {
			if err := s.OnFailure(ctx, result); err != nil {
				return s.handleError(ctx, span, fmt.Errorf("on_failure handler failed: %w", err))
			}
		} else {
			// Default behavior: failure is an error
			return s.handleError(ctx, span, fmt.Errorf("sub-workflow failed: %s", workflowID))
		}

	case WorkflowStatusCancelled:
		ctx.Metrics.Inc("task_subworkflow_cancelled_total")
		return s.handleError(ctx, span, fmt.Errorf("sub-workflow was cancelled: %s", instanceID))

	case WorkflowStatusTimedOut:
		ctx.Metrics.Inc("task_subworkflow_timeout_total")
		if s.OnTimeout != nil {
			status := &WorkflowStatus{
				InstanceID: instanceID,
				WorkflowID: workflowID,
				Status:     WorkflowStatusTimedOut,
			}
			if err := s.OnTimeout(ctx, status); err != nil {
				return s.handleError(ctx, span, fmt.Errorf("timeout handler failed: %w", err))
			}
		} else {
			return s.handleError(ctx, span, fmt.Errorf("sub-workflow timed out: %s", instanceID))
		}

	default:
		return s.handleError(ctx, span, fmt.Errorf("unexpected sub-workflow status: %s", result.Status))
	}

	// Success
	ctx.Metrics.Inc("task_success_total")
	ctx.Metrics.Inc("task_subworkflow_success_total")

	return nil
}

// handleError processes errors with observability.
func (s *SubWorkflowTask) handleError(ctx *context.ExecutionContext, span context.Span, err error) error {
	span.RecordError(err)
	ctx.Metrics.Inc("task_errors_total")
	ctx.Metrics.Inc("task_subworkflow_errors_total")
	ctx.ErrorRecorder.RecordError(err, map[string]interface{}{
		"task_name":   s.Name,
		"workflow_id": s.WorkflowID,
		"token_id":    tokenID(ctx),
	})
	ctx.Logger.Error("Sub-workflow task failed", map[string]interface{}{
		"task_name":   s.Name,
		"workflow_id": s.WorkflowID,
		"error":       err.Error(),
		"token_id":    tokenID(ctx),
	})
	return ctx.ErrorHandler.Handle(ctx, err)
}

// MockWorkflowInvoker is a simple workflow invoker for testing.
type MockWorkflowInvoker struct {
	// Workflows maps workflow IDs to their mock behaviors.
	Workflows map[string]*MockWorkflow

	// Instances tracks started workflow instances.
	Instances map[string]*MockWorkflowInstance

	// nextID generates unique instance IDs.
	nextID int
}

// MockWorkflow defines the behavior of a mock workflow.
type MockWorkflow struct {
	// Output is the result output.
	Output interface{}

	// Status is the final status.
	Status WorkflowStatusCode

	// Duration is how long the workflow takes.
	Duration time.Duration

	// ShouldFail causes the workflow to fail.
	ShouldFail bool

	// FailError is the error to return.
	FailError error
}

// MockWorkflowInstance tracks a running mock instance.
type MockWorkflowInstance struct {
	WorkflowID string
	Input      interface{}
	StartedAt  time.Time
	Workflow   *MockWorkflow
}

// NewMockWorkflowInvoker creates a new mock workflow invoker for testing.
func NewMockWorkflowInvoker() *MockWorkflowInvoker {
	return &MockWorkflowInvoker{
		Workflows: make(map[string]*MockWorkflow),
		Instances: make(map[string]*MockWorkflowInstance),
	}
}

// RegisterWorkflow registers a mock workflow behavior.
func (m *MockWorkflowInvoker) RegisterWorkflow(workflowID string, workflow *MockWorkflow) {
	m.Workflows[workflowID] = workflow
}

// StartWorkflow starts a mock workflow instance.
func (m *MockWorkflowInvoker) StartWorkflow(ctx *context.ExecutionContext, workflowID string, input interface{}) (string, error) {
	workflow, exists := m.Workflows[workflowID]
	if !exists {
		return "", fmt.Errorf("unknown workflow: %s", workflowID)
	}

	m.nextID++
	instanceID := fmt.Sprintf("instance-%d", m.nextID)

	m.Instances[instanceID] = &MockWorkflowInstance{
		WorkflowID: workflowID,
		Input:      input,
		StartedAt:  ctx.Clock.Now(),
		Workflow:   workflow,
	}

	return instanceID, nil
}

// WaitForCompletion simulates waiting for a workflow to complete.
func (m *MockWorkflowInvoker) WaitForCompletion(ctx *context.ExecutionContext, instanceID string, timeout time.Duration) (*WorkflowResult, error) {
	instance, exists := m.Instances[instanceID]
	if !exists {
		return nil, fmt.Errorf("unknown instance: %s", instanceID)
	}

	workflow := instance.Workflow

	// Simulate workflow duration
	if workflow.Duration > 0 {
		// Check timeout
		if timeout > 0 && workflow.Duration > timeout {
			return &WorkflowResult{
				InstanceID: instanceID,
				WorkflowID: instance.WorkflowID,
				Status:     WorkflowStatusTimedOut,
				StartedAt:  instance.StartedAt,
			}, nil
		}

		ctx.Clock.Sleep(workflow.Duration)
	}

	if workflow.ShouldFail {
		if workflow.FailError != nil {
			return nil, workflow.FailError
		}
		return &WorkflowResult{
			InstanceID:  instanceID,
			WorkflowID:  instance.WorkflowID,
			Status:      WorkflowStatusFailed,
			StartedAt:   instance.StartedAt,
			CompletedAt: ctx.Clock.Now(),
			Duration:    workflow.Duration,
		}, nil
	}

	return &WorkflowResult{
		InstanceID:  instanceID,
		WorkflowID:  instance.WorkflowID,
		Status:      WorkflowStatusCompleted,
		Output:      workflow.Output,
		StartedAt:   instance.StartedAt,
		CompletedAt: ctx.Clock.Now(),
		Duration:    workflow.Duration,
	}, nil
}

// GetWorkflowStatus returns the current status of a mock instance.
func (m *MockWorkflowInvoker) GetWorkflowStatus(ctx *context.ExecutionContext, instanceID string) (*WorkflowStatus, error) {
	instance, exists := m.Instances[instanceID]
	if !exists {
		return nil, fmt.Errorf("unknown instance: %s", instanceID)
	}

	return &WorkflowStatus{
		InstanceID: instanceID,
		WorkflowID: instance.WorkflowID,
		Status:     WorkflowStatusRunning,
		StartedAt:  instance.StartedAt,
	}, nil
}

// CancelWorkflow cancels a mock instance.
func (m *MockWorkflowInvoker) CancelWorkflow(ctx *context.ExecutionContext, instanceID string) error {
	_, exists := m.Instances[instanceID]
	if !exists {
		return fmt.Errorf("unknown instance: %s", instanceID)
	}

	delete(m.Instances, instanceID)
	return nil
}
