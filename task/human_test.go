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

func TestHumanTask_ApprovalApproved(t *testing.T) {
	// Create virtual clock
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(start)

	// Create execution context
	tok := &token.Token{ID: "test-token", Metadata: make(map[string]interface{})}
	ctx := context.NewExecutionContext(stdContext.Background(), clk, tok)

	// Create channel interaction
	interaction := NewChannelHumanInteraction()

	// Create human task
	var approvedCalled bool
	task := &HumanTask{
		Name:        "test-approval",
		Interaction: interaction,
		Mode:        HumanModeApproval,
		Timeout:     10 * time.Second,
		Request: &ApprovalRequest{
			Title:       "Test Approval",
			Description: "Please approve this test",
			Priority:    "high",
		},
		OnApproved: func(ctx *context.ExecutionContext, decision *HumanDecision) error {
			approvedCalled = true
			if decision.DecisionBy != "test-user" {
				t.Errorf("expected decisionBy test-user, got %s", decision.DecisionBy)
			}
			return nil
		},
	}

	// Execute in goroutine
	done := make(chan error)
	go func() {
		done <- task.Execute(ctx)
	}()

	// Wait for request to be made, then simulate approval
	time.Sleep(10 * time.Millisecond)
	err := interaction.SimulateApproval("approval-1", true, "test-user", "Looks good!")
	if err != nil {
		t.Fatalf("failed to simulate approval: %v", err)
	}

	// Wait for completion
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("test timed out")
	}

	if !approvedCalled {
		t.Fatal("OnApproved callback not called")
	}
}

func TestHumanTask_ApprovalRejected(t *testing.T) {
	// Create virtual clock
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(start)

	// Create execution context
	tok := &token.Token{ID: "test-token", Metadata: make(map[string]interface{})}
	ctx := context.NewExecutionContext(stdContext.Background(), clk, tok)

	// Create channel interaction
	interaction := NewChannelHumanInteraction()

	// Create human task
	var rejectedCalled bool
	task := &HumanTask{
		Name:        "test-rejection",
		Interaction: interaction,
		Mode:        HumanModeApproval,
		Timeout:     10 * time.Second,
		Request: &ApprovalRequest{
			Title:       "Test Rejection",
			Description: "This will be rejected",
		},
		OnRejected: func(ctx *context.ExecutionContext, decision *HumanDecision) error {
			rejectedCalled = true
			if decision.Comments != "Not approved" {
				t.Errorf("expected comments 'Not approved', got %s", decision.Comments)
			}
			return nil
		},
	}

	// Execute in goroutine
	done := make(chan error)
	go func() {
		done <- task.Execute(ctx)
	}()

	// Wait for request, then simulate rejection
	time.Sleep(10 * time.Millisecond)
	err := interaction.SimulateApproval("approval-1", false, "manager", "Not approved")
	if err != nil {
		t.Fatalf("failed to simulate rejection: %v", err)
	}

	// Wait for completion
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("test timed out")
	}

	if !rejectedCalled {
		t.Fatal("OnRejected callback not called")
	}
}

func TestHumanTask_ApprovalRejectedNoHandler(t *testing.T) {
	// Create virtual clock
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(start)

	// Create execution context
	tok := &token.Token{ID: "test-token", Metadata: make(map[string]interface{})}
	ctx := context.NewExecutionContext(stdContext.Background(), clk, tok)

	// Create channel interaction
	interaction := NewChannelHumanInteraction()

	// Create human task without OnRejected handler
	task := &HumanTask{
		Name:        "test-rejection-error",
		Interaction: interaction,
		Mode:        HumanModeApproval,
		Timeout:     10 * time.Second,
		Request: &ApprovalRequest{
			Title: "Test Rejection Error",
		},
	}

	// Execute in goroutine
	done := make(chan error)
	go func() {
		done <- task.Execute(ctx)
	}()

	// Wait for request, then simulate rejection
	time.Sleep(10 * time.Millisecond)
	err := interaction.SimulateApproval("approval-1", false, "manager", "Denied")
	if err != nil {
		t.Fatalf("failed to simulate rejection: %v", err)
	}

	// Wait for completion - should error
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected error for rejection without handler")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("test timed out")
	}
}

func TestHumanTask_DataEntry(t *testing.T) {
	// Create virtual clock
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(start)

	// Create execution context
	tok := &token.Token{ID: "test-token", Metadata: make(map[string]interface{})}
	ctx := context.NewExecutionContext(stdContext.Background(), clk, tok)

	// Create channel interaction
	interaction := NewChannelHumanInteraction()

	// Create human task for data entry
	var dataReceived map[string]interface{}
	task := &HumanTask{
		Name:        "test-data-entry",
		Interaction: interaction,
		Mode:        HumanModeDataEntry,
		Timeout:     10 * time.Second,
		DataRequest: &DataEntryRequest{
			Title:       "Enter Address",
			Description: "Please provide your shipping address",
			Fields: []DataField{
				{Name: "street", Label: "Street", Type: "text", Required: true},
				{Name: "city", Label: "City", Type: "text", Required: true},
			},
		},
		OnDataReceived: func(ctx *context.ExecutionContext, data *HumanDataResponse) error {
			dataReceived = data.Data
			return nil
		},
	}

	// Execute in goroutine
	done := make(chan error)
	go func() {
		done <- task.Execute(ctx)
	}()

	// Wait for request, then simulate data entry
	time.Sleep(10 * time.Millisecond)
	err := interaction.SimulateDataEntry("data-entry-1", map[string]interface{}{
		"street": "123 Main St",
		"city":   "Anytown",
	}, "customer@example.com")
	if err != nil {
		t.Fatalf("failed to simulate data entry: %v", err)
	}

	// Wait for completion
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("test timed out")
	}

	if dataReceived == nil {
		t.Fatal("OnDataReceived callback not called")
	}
	if dataReceived["street"] != "123 Main St" {
		t.Errorf("expected street '123 Main St', got %v", dataReceived["street"])
	}
}

func TestHumanTask_DynamicRequest(t *testing.T) {
	// Create virtual clock
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(start)

	// Create execution context with token data
	tok := &token.Token{
		ID:       "test-token",
		Metadata: make(map[string]interface{}),
	}
	tok.Metadata["order_id"] = "ORD-12345"
	ctx := context.NewExecutionContext(stdContext.Background(), clk, tok)

	// Create channel interaction
	interaction := NewChannelHumanInteraction()

	// Create human task with dynamic request
	task := &HumanTask{
		Name:        "test-dynamic",
		Interaction: interaction,
		Mode:        HumanModeApproval,
		Timeout:     10 * time.Second,
		RequestFunc: func(ctx *context.ExecutionContext) (*ApprovalRequest, error) {
			orderID := ctx.Token.Metadata["order_id"].(string)
			return &ApprovalRequest{
				Title:       "Approve Order " + orderID,
				Description: "Please review and approve order",
			}, nil
		},
	}

	// Execute in goroutine
	done := make(chan error)
	go func() {
		done <- task.Execute(ctx)
	}()

	// Wait for request, then approve
	time.Sleep(10 * time.Millisecond)
	err := interaction.SimulateApproval("approval-1", true, "approver", "OK")
	if err != nil {
		t.Fatalf("failed to simulate approval: %v", err)
	}

	// Wait for completion
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("test timed out")
	}
}

func TestHumanTask_Timeout(t *testing.T) {
	// Create virtual clock
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(start)

	// Create execution context
	tok := &token.Token{ID: "test-token", Metadata: make(map[string]interface{})}
	ctx := context.NewExecutionContext(stdContext.Background(), clk, tok)

	// Create channel interaction
	interaction := NewChannelHumanInteraction()

	// Create human task with short timeout
	var timeoutCalled bool
	task := &HumanTask{
		Name:        "test-timeout",
		Interaction: interaction,
		Mode:        HumanModeApproval,
		Timeout:     5 * time.Second,
		Request: &ApprovalRequest{
			Title: "Test Timeout",
		},
		OnTimeout: func(ctx *context.ExecutionContext) error {
			timeoutCalled = true
			return nil
		},
	}

	// Execute in goroutine
	done := make(chan error)
	go func() {
		done <- task.Execute(ctx)
	}()

	// Advance time past timeout
	time.Sleep(10 * time.Millisecond)
	clk.AdvanceBy(10 * time.Second)

	// Wait for completion
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("test timed out")
	}

	if !timeoutCalled {
		t.Fatal("OnTimeout callback not called")
	}
}

func TestHumanTask_ReviewMode(t *testing.T) {
	// Create virtual clock
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(start)

	// Create execution context
	tok := &token.Token{ID: "test-token", Metadata: make(map[string]interface{})}
	ctx := context.NewExecutionContext(stdContext.Background(), clk, tok)

	// Create channel interaction
	interaction := NewChannelHumanInteraction()

	// Create human task in review mode (non-blocking)
	task := &HumanTask{
		Name:        "test-review",
		Interaction: interaction,
		Mode:        HumanModeReview,
		Request: &ApprovalRequest{
			Title:       "Review This",
			Description: "Non-blocking review request",
		},
	}

	// Execute - should complete immediately without waiting
	err := task.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHumanTask_ValidationErrors(t *testing.T) {
	// Create virtual clock
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(start)

	tok := &token.Token{ID: "test-token", Metadata: make(map[string]interface{})}
	ctx := context.NewExecutionContext(stdContext.Background(), clk, tok)

	interaction := NewChannelHumanInteraction()

	// Test missing name
	task := &HumanTask{
		Interaction: interaction,
		Mode:        HumanModeApproval,
	}
	err := task.Execute(ctx)
	if err == nil || err.Error() != "human task: name is required" {
		t.Fatalf("expected name required error, got: %v", err)
	}

	// Test missing interaction
	task = &HumanTask{
		Name: "test",
		Mode: HumanModeApproval,
	}
	err = task.Execute(ctx)
	if err == nil || err.Error() != "human task: interaction is required" {
		t.Fatalf("expected interaction required error, got: %v", err)
	}

	// Test missing request in approval mode
	task = &HumanTask{
		Name:        "test",
		Interaction: interaction,
		Mode:        HumanModeApproval,
	}
	err = task.Execute(ctx)
	if err == nil {
		t.Fatal("expected request required error")
	}
}

func TestChannelHumanInteraction_UnknownIDs(t *testing.T) {
	// Create virtual clock
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(start)

	tok := &token.Token{ID: "test-token", Metadata: make(map[string]interface{})}
	ctx := context.NewExecutionContext(stdContext.Background(), clk, tok)

	interaction := NewChannelHumanInteraction()

	// Test unknown approval ID
	_, err := interaction.WaitForDecision(ctx, "unknown-approval", 0)
	if err == nil {
		t.Fatal("expected error for unknown approval ID")
	}

	// Test unknown data entry ID
	_, err = interaction.WaitForData(ctx, "unknown-request", 0)
	if err == nil {
		t.Fatal("expected error for unknown request ID")
	}

	// Test simulate approval for unknown ID
	err = interaction.SimulateApproval("unknown", true, "user", "")
	if err == nil {
		t.Fatal("expected error for unknown approval ID")
	}

	// Test simulate data entry for unknown ID
	err = interaction.SimulateDataEntry("unknown", nil, "user")
	if err == nil {
		t.Fatal("expected error for unknown request ID")
	}
}
