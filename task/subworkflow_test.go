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
	"testing"
	"time"

	"github.com/jazzpetri/engine/clock"
	"github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/token"
)

func TestSubWorkflowTask_SyncExecution(t *testing.T) {
	// Create virtual clock
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(start)

	// Create execution context
	tok := &token.Token{ID: "test-token", Metadata: make(map[string]interface{})}
	ctx := context.NewExecutionContext(stdContext.Background(), clk, tok)

	// Create mock invoker
	invoker := NewMockWorkflowInvoker()
	invoker.RegisterWorkflow("test-workflow", &MockWorkflow{
		Output:   map[string]interface{}{"result": "success"},
		Status:   WorkflowStatusCompleted,
		Duration: 100 * time.Millisecond,
	})

	// Create sub-workflow task
	var successCalled bool
	task := &SubWorkflowTask{
		Name:       "test-subworkflow",
		Invoker:    invoker,
		WorkflowID: "test-workflow",
		Input: map[string]interface{}{
			"input_data": "test",
		},
		OnSuccess: func(ctx *context.ExecutionContext, result *WorkflowResult) error {
			successCalled = true
			if result.Output.(map[string]interface{})["result"] != "success" {
				t.Errorf("expected result 'success', got %v", result.Output)
			}
			return nil
		},
	}

	// Execute in goroutine (uses Sleep)
	done := make(chan error)
	go func() {
		done <- task.Execute(ctx)
	}()

	// Advance time
	time.Sleep(10 * time.Millisecond)
	clk.AdvanceBy(200 * time.Millisecond)

	// Wait for completion
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("test timed out")
	}

	if !successCalled {
		t.Fatal("OnSuccess callback not called")
	}

	// Check instance ID was stored
	if ctx.Token.Metadata["subworkflow_instance_id"] == nil {
		t.Fatal("expected instance_id in metadata")
	}
}

func TestSubWorkflowTask_AsyncExecution(t *testing.T) {
	// Create virtual clock
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(start)

	// Create execution context
	tok := &token.Token{ID: "test-token", Metadata: make(map[string]interface{})}
	ctx := context.NewExecutionContext(stdContext.Background(), clk, tok)

	// Create mock invoker
	invoker := NewMockWorkflowInvoker()
	invoker.RegisterWorkflow("async-workflow", &MockWorkflow{
		Output:   "async result",
		Status:   WorkflowStatusCompleted,
		Duration: 10 * time.Second, // Long duration
	})

	// Create async sub-workflow task
	task := &SubWorkflowTask{
		Name:       "async-subworkflow",
		Invoker:    invoker,
		WorkflowID: "async-workflow",
		Async:      true, // Fire and forget
		Input:      "async input",
	}

	// Execute - should return immediately
	err := task.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check instance ID was stored (workflow started but not completed)
	if ctx.Token.Metadata["subworkflow_instance_id"] == nil {
		t.Fatal("expected instance_id in metadata")
	}
}

func TestSubWorkflowTask_DynamicWorkflowID(t *testing.T) {
	// Create virtual clock
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(start)

	// Create execution context with token data
	tok := &token.Token{
		ID:       "test-token",
		Metadata: make(map[string]interface{}),
	}
	tok.Metadata["workflow_type"] = "approval"
	ctx := context.NewExecutionContext(stdContext.Background(), clk, tok)

	// Create mock invoker
	invoker := NewMockWorkflowInvoker()
	invoker.RegisterWorkflow("approval-workflow", &MockWorkflow{
		Output: "approved",
		Status: WorkflowStatusCompleted,
	})

	// Create sub-workflow task with dynamic workflow ID
	task := &SubWorkflowTask{
		Name:    "dynamic-workflow",
		Invoker: invoker,
		WorkflowIDFunc: func(ctx *context.ExecutionContext) (string, error) {
			wfType := ctx.Token.Metadata["workflow_type"].(string)
			return wfType + "-workflow", nil
		},
		Async: true,
	}

	// Execute
	err := task.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSubWorkflowTask_DynamicInput(t *testing.T) {
	// Create virtual clock
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(start)

	// Create execution context with token data
	tok := &token.Token{
		ID:       "test-token",
		Metadata: make(map[string]interface{}),
	}
	tok.Metadata["order_id"] = "ORD-123"
	ctx := context.NewExecutionContext(stdContext.Background(), clk, tok)

	// Create mock invoker
	invoker := NewMockWorkflowInvoker()
	invoker.RegisterWorkflow("process-order", &MockWorkflow{
		Output: "processed",
		Status: WorkflowStatusCompleted,
	})

	// Create sub-workflow task with dynamic input
	task := &SubWorkflowTask{
		Name:       "process-order",
		Invoker:    invoker,
		WorkflowID: "process-order",
		InputFunc: func(ctx *context.ExecutionContext) (interface{}, error) {
			return map[string]interface{}{
				"order_id": ctx.Token.Metadata["order_id"],
				"priority": "high",
			}, nil
		},
		Async: true,
	}

	// Execute
	err := task.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check the input was passed correctly
	instanceID := ctx.Token.Metadata["subworkflow_instance_id"].(string)
	instance := invoker.Instances[instanceID]
	input := instance.Input.(map[string]interface{})
	if input["order_id"] != "ORD-123" {
		t.Errorf("expected order_id ORD-123, got %v", input["order_id"])
	}
}

func TestSubWorkflowTask_WorkflowFailed(t *testing.T) {
	// Create virtual clock
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(start)

	// Create execution context
	tok := &token.Token{ID: "test-token", Metadata: make(map[string]interface{})}
	ctx := context.NewExecutionContext(stdContext.Background(), clk, tok)

	// Create mock invoker with failing workflow
	invoker := NewMockWorkflowInvoker()
	invoker.RegisterWorkflow("failing-workflow", &MockWorkflow{
		Status:     WorkflowStatusFailed,
		ShouldFail: true,
	})

	// Create sub-workflow task with OnFailure handler
	var failureCalled bool
	task := &SubWorkflowTask{
		Name:       "failing-subworkflow",
		Invoker:    invoker,
		WorkflowID: "failing-workflow",
		OnFailure: func(ctx *context.ExecutionContext, result *WorkflowResult) error {
			failureCalled = true
			return nil
		},
	}

	// Execute
	err := task.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !failureCalled {
		t.Fatal("OnFailure callback not called")
	}
}

func TestSubWorkflowTask_WorkflowFailedNoHandler(t *testing.T) {
	// Create virtual clock
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(start)

	// Create execution context
	tok := &token.Token{ID: "test-token", Metadata: make(map[string]interface{})}
	ctx := context.NewExecutionContext(stdContext.Background(), clk, tok)

	// Create mock invoker with failing workflow
	invoker := NewMockWorkflowInvoker()
	invoker.RegisterWorkflow("failing-workflow", &MockWorkflow{
		Status:     WorkflowStatusFailed,
		ShouldFail: true,
	})

	// Create sub-workflow task without OnFailure handler
	task := &SubWorkflowTask{
		Name:       "failing-subworkflow",
		Invoker:    invoker,
		WorkflowID: "failing-workflow",
	}

	// Execute - should fail
	err := task.Execute(ctx)
	if err == nil {
		t.Fatal("expected error for failed workflow")
	}
}

func TestSubWorkflowTask_Timeout(t *testing.T) {
	// Create virtual clock
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(start)

	// Create execution context
	tok := &token.Token{ID: "test-token", Metadata: make(map[string]interface{})}
	ctx := context.NewExecutionContext(stdContext.Background(), clk, tok)

	// Create mock invoker with slow workflow
	invoker := NewMockWorkflowInvoker()
	invoker.RegisterWorkflow("slow-workflow", &MockWorkflow{
		Output:   "slow result",
		Status:   WorkflowStatusCompleted,
		Duration: 1 * time.Hour, // Very slow
	})

	// Create sub-workflow task with short timeout
	var timeoutCalled bool
	task := &SubWorkflowTask{
		Name:       "timeout-subworkflow",
		Invoker:    invoker,
		WorkflowID: "slow-workflow",
		Timeout:    5 * time.Second,
		OnTimeout: func(ctx *context.ExecutionContext, status *WorkflowStatus) error {
			timeoutCalled = true
			return nil
		},
	}

	// Execute
	err := task.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !timeoutCalled {
		t.Fatal("OnTimeout callback not called")
	}
}

func TestSubWorkflowTask_UnknownWorkflow(t *testing.T) {
	// Create virtual clock
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(start)

	// Create execution context
	tok := &token.Token{ID: "test-token", Metadata: make(map[string]interface{})}
	ctx := context.NewExecutionContext(stdContext.Background(), clk, tok)

	// Create mock invoker (no workflows registered)
	invoker := NewMockWorkflowInvoker()

	// Create sub-workflow task with unknown workflow
	task := &SubWorkflowTask{
		Name:       "unknown-subworkflow",
		Invoker:    invoker,
		WorkflowID: "nonexistent-workflow",
	}

	// Execute - should fail
	err := task.Execute(ctx)
	if err == nil {
		t.Fatal("expected error for unknown workflow")
	}
}

func TestSubWorkflowTask_ValidationErrors(t *testing.T) {
	// Create virtual clock
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(start)

	tok := &token.Token{ID: "test-token", Metadata: make(map[string]interface{})}
	ctx := context.NewExecutionContext(stdContext.Background(), clk, tok)

	invoker := NewMockWorkflowInvoker()

	// Test missing name
	task := &SubWorkflowTask{
		Invoker:    invoker,
		WorkflowID: "test",
	}
	err := task.Execute(ctx)
	if err == nil || err.Error() != "sub-workflow task: name is required" {
		t.Fatalf("expected name required error, got: %v", err)
	}

	// Test missing invoker
	task = &SubWorkflowTask{
		Name:       "test",
		WorkflowID: "test",
	}
	err = task.Execute(ctx)
	if err == nil || err.Error() != "sub-workflow task: invoker is required" {
		t.Fatalf("expected invoker required error, got: %v", err)
	}

	// Test missing workflow ID
	task = &SubWorkflowTask{
		Name:    "test",
		Invoker: invoker,
	}
	err = task.Execute(ctx)
	if err == nil {
		t.Fatal("expected workflow ID required error")
	}
}

func TestWorkflowStatusCode_String(t *testing.T) {
	tests := []struct {
		status   WorkflowStatusCode
		expected string
	}{
		{WorkflowStatusPending, "pending"},
		{WorkflowStatusRunning, "running"},
		{WorkflowStatusCompleted, "completed"},
		{WorkflowStatusFailed, "failed"},
		{WorkflowStatusCancelled, "cancelled"},
		{WorkflowStatusTimedOut, "timed_out"},
		{WorkflowStatusCode(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.status.String()
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestMockWorkflowInvoker(t *testing.T) {
	// Create virtual clock
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(start)

	tok := &token.Token{ID: "test-token", Metadata: make(map[string]interface{})}
	ctx := context.NewExecutionContext(stdContext.Background(), clk, tok)

	invoker := NewMockWorkflowInvoker()

	// Test registering and starting workflow
	invoker.RegisterWorkflow("test-wf", &MockWorkflow{
		Output: "test output",
		Status: WorkflowStatusCompleted,
	})

	instanceID, err := invoker.StartWorkflow(ctx, "test-wf", "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if instanceID == "" {
		t.Fatal("expected instance ID")
	}

	// Test getting status
	status, err := invoker.GetWorkflowStatus(ctx, instanceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Status != WorkflowStatusRunning {
		t.Errorf("expected running status, got %s", status.Status)
	}

	// Test cancelling
	err = invoker.CancelWorkflow(ctx, instanceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test unknown instance
	_, err = invoker.GetWorkflowStatus(ctx, "unknown")
	if err == nil {
		t.Fatal("expected error for unknown instance")
	}

	err = invoker.CancelWorkflow(ctx, "unknown")
	if err == nil {
		t.Fatal("expected error for unknown instance")
	}
}
