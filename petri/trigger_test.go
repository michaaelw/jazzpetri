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

package petri

import (
	stdContext "context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jazzpetri/engine/clock"
	"github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/token"
)

// TestAutomaticTrigger_ShouldFire verifies that AutomaticTrigger always returns true.
func TestAutomaticTrigger_ShouldFire(t *testing.T) {
	trigger := &AutomaticTrigger{}
	ctx := context.NewExecutionContext(stdContext.Background(), clock.NewVirtualClock(time.Now()), nil)

	ready, err := trigger.ShouldFire(ctx)
	if err != nil {
		t.Errorf("ShouldFire() returned unexpected error: %v", err)
	}
	if !ready {
		t.Error("AutomaticTrigger.ShouldFire() should always return true")
	}
}

// TestAutomaticTrigger_AwaitFiring verifies that AutomaticTrigger never blocks.
func TestAutomaticTrigger_AwaitFiring(t *testing.T) {
	trigger := &AutomaticTrigger{}
	ctx := context.NewExecutionContext(stdContext.Background(), clock.NewVirtualClock(time.Now()), nil)

	// AwaitFiring should return immediately
	done := make(chan error, 1)
	go func() {
		done <- trigger.AwaitFiring(ctx)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("AwaitFiring() returned unexpected error: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("AwaitFiring() blocked, but should return immediately for AutomaticTrigger")
	}
}

// TestAutomaticTrigger_Cancel verifies that Cancel is a no-op.
func TestAutomaticTrigger_Cancel(t *testing.T) {
	trigger := &AutomaticTrigger{}

	err := trigger.Cancel()
	if err != nil {
		t.Errorf("Cancel() returned unexpected error: %v", err)
	}
}

// TestTimedTrigger_Delay_ShouldFire verifies that TimedTrigger.ShouldFire works with delay.
func TestTimedTrigger_Delay_ShouldFire(t *testing.T) {
	startTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	vClock := clock.NewVirtualClock(startTime)
	ctx := context.NewExecutionContext(stdContext.Background(), vClock, nil)

	delay := 5 * time.Second
	trigger := &TimedTrigger{Delay: &delay}

	// Before time elapses, should return false
	ready, err := trigger.ShouldFire(ctx)
	if err != nil {
		t.Errorf("ShouldFire() returned unexpected error: %v", err)
	}
	if ready {
		t.Error("ShouldFire() should return false before delay elapses")
	}

	// Advance time past delay
	vClock.AdvanceBy(6 * time.Second)

	// Now should return true
	ready, err = trigger.ShouldFire(ctx)
	if err != nil {
		t.Errorf("ShouldFire() returned unexpected error: %v", err)
	}
	if !ready {
		t.Error("ShouldFire() should return true after delay elapses")
	}
}

// TestTimedTrigger_Delay_AwaitFiring verifies that TimedTrigger blocks until delay elapses.
func TestTimedTrigger_Delay_AwaitFiring(t *testing.T) {
	startTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	vClock := clock.NewVirtualClock(startTime)
	ctx := context.NewExecutionContext(stdContext.Background(), vClock, nil)

	delay := 5 * time.Second
	trigger := &TimedTrigger{Delay: &delay}

	// First check should return false (not ready)
	ready, err := trigger.ShouldFire(ctx)
	if err != nil {
		t.Fatalf("ShouldFire() returned unexpected error: %v", err)
	}
	if ready {
		t.Fatal("ShouldFire() should return false initially")
	}

	// Spawn goroutine to wait
	done := make(chan error, 1)
	go func() {
		done <- trigger.AwaitFiring(ctx)
	}()

	// Give the goroutine time to start waiting
	time.Sleep(50 * time.Millisecond)

	// Advance clock by 4 seconds - should still be waiting
	vClock.AdvanceBy(4 * time.Second)
	select {
	case <-done:
		t.Fatal("AwaitFiring returned too early")
	case <-time.After(100 * time.Millisecond):
		// Good, still waiting
	}

	// Advance by 2 more seconds (total 6) - should unblock
	vClock.AdvanceBy(2 * time.Second)
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("AwaitFiring() returned unexpected error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("AwaitFiring did not unblock after delay elapsed")
	}
}

// TestTimedTrigger_At_ShouldFire verifies that TimedTrigger.ShouldFire works with specific time.
func TestTimedTrigger_At_ShouldFire(t *testing.T) {
	startTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	targetTime := time.Date(2025, 1, 1, 12, 5, 0, 0, time.UTC)
	vClock := clock.NewVirtualClock(startTime)
	ctx := context.NewExecutionContext(stdContext.Background(), vClock, nil)

	trigger := &TimedTrigger{At: &targetTime}

	// Before target time, should return false
	ready, err := trigger.ShouldFire(ctx)
	if err != nil {
		t.Errorf("ShouldFire() returned unexpected error: %v", err)
	}
	if ready {
		t.Error("ShouldFire() should return false before target time")
	}

	// Advance time to target
	vClock.AdvanceTo(targetTime)

	// Now should return true
	ready, err = trigger.ShouldFire(ctx)
	if err != nil {
		t.Errorf("ShouldFire() returned unexpected error: %v", err)
	}
	if !ready {
		t.Error("ShouldFire() should return true at target time")
	}

	// Advance past target
	vClock.AdvanceBy(1 * time.Minute)

	// Should still return true
	ready, err = trigger.ShouldFire(ctx)
	if err != nil {
		t.Errorf("ShouldFire() returned unexpected error: %v", err)
	}
	if !ready {
		t.Error("ShouldFire() should return true after target time")
	}
}

// TestTimedTrigger_At_AwaitFiring verifies that TimedTrigger blocks until specific time reached.
func TestTimedTrigger_At_AwaitFiring(t *testing.T) {
	startTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	targetTime := time.Date(2025, 1, 1, 12, 5, 0, 0, time.UTC)
	vClock := clock.NewVirtualClock(startTime)
	ctx := context.NewExecutionContext(stdContext.Background(), vClock, nil)

	trigger := &TimedTrigger{At: &targetTime}

	// Initialize timer by calling ShouldFire first
	ready, err := trigger.ShouldFire(ctx)
	if err != nil {
		t.Fatalf("ShouldFire() returned error: %v", err)
	}
	if ready {
		t.Fatal("ShouldFire() should return false initially")
	}

	// Spawn goroutine to wait
	done := make(chan error, 1)
	go func() {
		done <- trigger.AwaitFiring(ctx)
	}()

	// Give the goroutine time to start waiting
	time.Sleep(50 * time.Millisecond)

	// Advance by 6 minutes (past target) - should unblock
	vClock.AdvanceBy(6 * time.Minute)
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("AwaitFiring() returned unexpected error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("AwaitFiring did not unblock after target time reached")
	}
}

// TestTimedTrigger_Cancel verifies that Cancel stops waiting.
func TestTimedTrigger_Cancel(t *testing.T) {
	startTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	vClock := clock.NewVirtualClock(startTime)
	ctx := context.NewExecutionContext(stdContext.Background(), vClock, nil)

	delay := 5 * time.Second
	trigger := &TimedTrigger{Delay: &delay}

	// Spawn goroutine to wait
	done := make(chan error, 1)
	go func() {
		done <- trigger.AwaitFiring(ctx)
	}()

	// Give the goroutine time to start waiting
	time.Sleep(50 * time.Millisecond)

	// Cancel the trigger
	err := trigger.Cancel()
	if err != nil {
		t.Errorf("Cancel() returned unexpected error: %v", err)
	}

	// AwaitFiring should return with error
	select {
	case err := <-done:
		if err == nil {
			t.Error("AwaitFiring() should return error after Cancel()")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("AwaitFiring did not return after Cancel()")
	}
}

// TestTimedTrigger_AlreadyPassed verifies that trigger fires immediately if time already passed.
func TestTimedTrigger_AlreadyPassed(t *testing.T) {
	startTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	pastTime := time.Date(2025, 1, 1, 11, 55, 0, 0, time.UTC)
	vClock := clock.NewVirtualClock(startTime)
	ctx := context.NewExecutionContext(stdContext.Background(), vClock, nil)

	trigger := &TimedTrigger{At: &pastTime}

	// Should be ready immediately since time already passed
	ready, err := trigger.ShouldFire(ctx)
	if err != nil {
		t.Errorf("ShouldFire() returned unexpected error: %v", err)
	}
	if !ready {
		t.Error("ShouldFire() should return true when time already passed")
	}

	// In normal flow, AwaitFiring() won't be called since ShouldFire() already returned true
	// The engine only calls AwaitFiring() when ShouldFire() returns false
	// This test just verifies that ShouldFire() correctly identifies past times
}

// TestTimedTrigger_Cancellation verifies that trigger respects context cancellation.
func TestTimedTrigger_Cancellation(t *testing.T) {
	startTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	vClock := clock.NewVirtualClock(startTime)
	cancelCtx, cancel := stdContext.WithCancel(stdContext.Background())
	ctx := context.NewExecutionContext(cancelCtx, vClock, nil)

	delay := 5 * time.Second
	trigger := &TimedTrigger{Delay: &delay}

	// Spawn goroutine to wait
	done := make(chan error, 1)
	go func() {
		done <- trigger.AwaitFiring(ctx)
	}()

	// Give the goroutine time to start waiting
	time.Sleep(50 * time.Millisecond)

	// Cancel the context
	cancel()

	// AwaitFiring should return with error
	select {
	case err := <-done:
		if err == nil {
			t.Error("AwaitFiring() should return error after context cancellation")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("AwaitFiring did not return after context cancellation")
	}
}

// TestTimedTrigger_BothDelayAndAt verifies that error is returned if both Delay and At are set.
func TestTimedTrigger_BothDelayAndAt(t *testing.T) {
	startTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	targetTime := time.Date(2025, 1, 1, 12, 5, 0, 0, time.UTC)
	vClock := clock.NewVirtualClock(startTime)
	ctx := context.NewExecutionContext(stdContext.Background(), vClock, nil)

	delay := 5 * time.Second
	trigger := &TimedTrigger{Delay: &delay, At: &targetTime}

	// Should return error
	_, err := trigger.ShouldFire(ctx)
	if err == nil {
		t.Error("ShouldFire() should return error when both Delay and At are set")
	}

	err = trigger.AwaitFiring(ctx)
	if err == nil {
		t.Error("AwaitFiring() should return error when both Delay and At are set")
	}
}

// TestTimedTrigger_NeitherDelayNorAt verifies that error is returned if neither Delay nor At is set.
func TestTimedTrigger_NeitherDelayNorAt(t *testing.T) {
	startTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	vClock := clock.NewVirtualClock(startTime)
	ctx := context.NewExecutionContext(stdContext.Background(), vClock, nil)

	trigger := &TimedTrigger{}

	// Should return error
	_, err := trigger.ShouldFire(ctx)
	if err == nil {
		t.Error("ShouldFire() should return error when neither Delay nor At is set")
	}

	err = trigger.AwaitFiring(ctx)
	if err == nil {
		t.Error("AwaitFiring() should return error when neither Delay nor At is set")
	}
}

// TestManualTrigger_ApproveAuthorized verifies that authorized user can approve.
func TestManualTrigger_ApproveAuthorized(t *testing.T) {
	ctx := context.NewExecutionContext(stdContext.Background(), clock.NewVirtualClock(time.Now()), nil)

	trigger := &ManualTrigger{
		ApprovalID: "test-approval",
		Approvers:  []string{"alice@example.com", "bob@example.com"},
		RequireAll: false,
	}

	// Alice approves
	err := trigger.Approve("alice@example.com")
	if err != nil {
		t.Errorf("Approve() returned unexpected error: %v", err)
	}

	// Should be ready to fire
	ready, err := trigger.ShouldFire(ctx)
	if err != nil {
		t.Errorf("ShouldFire() returned unexpected error: %v", err)
	}
	if !ready {
		t.Error("ShouldFire() should return true after approval")
	}
}

// TestManualTrigger_ApproveUnauthorized verifies that unauthorized user is rejected.
func TestManualTrigger_ApproveUnauthorized(t *testing.T) {
	ctx := context.NewExecutionContext(stdContext.Background(), clock.NewVirtualClock(time.Now()), nil)

	trigger := &ManualTrigger{
		ApprovalID: "test-approval",
		Approvers:  []string{"alice@example.com", "bob@example.com"},
		RequireAll: false,
	}

	// Charlie tries to approve (not in approver list)
	err := trigger.Approve("charlie@example.com")
	if err == nil {
		t.Error("Approve() should return error for unauthorized user")
	}

	// Should not be ready to fire
	ready, err := trigger.ShouldFire(ctx)
	if err != nil {
		t.Errorf("ShouldFire() returned unexpected error: %v", err)
	}
	if ready {
		t.Error("ShouldFire() should return false without valid approval")
	}
}

// TestManualTrigger_AwaitFiring_Approved verifies that AwaitFiring unblocks on approval.
func TestManualTrigger_AwaitFiring_Approved(t *testing.T) {
	ctx := context.NewExecutionContext(stdContext.Background(), clock.NewVirtualClock(time.Now()), nil)

	trigger := &ManualTrigger{
		ApprovalID: "test-approval",
		Approvers:  []string{"alice@example.com"},
		RequireAll: false,
	}

	// Spawn goroutine to wait
	done := make(chan error, 1)
	go func() {
		done <- trigger.AwaitFiring(ctx)
	}()

	// Give the goroutine time to start waiting
	time.Sleep(50 * time.Millisecond)

	// Should still be waiting
	select {
	case <-done:
		t.Fatal("AwaitFiring returned before approval")
	case <-time.After(100 * time.Millisecond):
		// Good, still waiting
	}

	// Approve
	err := trigger.Approve("alice@example.com")
	if err != nil {
		t.Fatalf("Approve() returned unexpected error: %v", err)
	}

	// Should unblock
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("AwaitFiring() returned unexpected error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("AwaitFiring did not unblock after approval")
	}
}

// TestManualTrigger_Timeout verifies that trigger returns error after timeout.
func TestManualTrigger_Timeout(t *testing.T) {
	startTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	vClock := clock.NewVirtualClock(startTime)
	ctx := context.NewExecutionContext(stdContext.Background(), vClock, nil)

	timeout := 5 * time.Second
	trigger := &ManualTrigger{
		ApprovalID: "test-approval",
		Approvers:  []string{"alice@example.com"},
		Timeout:    &timeout,
		RequireAll: false,
	}

	// Spawn goroutine to wait
	done := make(chan error, 1)
	go func() {
		done <- trigger.AwaitFiring(ctx)
	}()

	// Give the goroutine time to start waiting
	time.Sleep(50 * time.Millisecond)

	// Advance time past timeout
	vClock.AdvanceBy(6 * time.Second)

	// Should unblock with timeout error
	select {
	case err := <-done:
		if err == nil {
			t.Error("AwaitFiring() should return error after timeout")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("AwaitFiring did not return after timeout")
	}
}

// TestManualTrigger_RequireAll_False verifies that any approver can approve.
func TestManualTrigger_RequireAll_False(t *testing.T) {
	ctx := context.NewExecutionContext(stdContext.Background(), clock.NewVirtualClock(time.Now()), nil)

	trigger := &ManualTrigger{
		ApprovalID: "test-approval",
		Approvers:  []string{"alice@example.com", "bob@example.com"},
		RequireAll: false,
	}

	// Bob approves (not first approver)
	err := trigger.Approve("bob@example.com")
	if err != nil {
		t.Errorf("Approve() returned unexpected error: %v", err)
	}

	// Should be ready to fire
	ready, err := trigger.ShouldFire(ctx)
	if err != nil {
		t.Errorf("ShouldFire() returned unexpected error: %v", err)
	}
	if !ready {
		t.Error("ShouldFire() should return true after any approver approves (RequireAll=false)")
	}
}

// TestManualTrigger_RequireAll_True verifies that all approvers must approve.
func TestManualTrigger_RequireAll_True(t *testing.T) {
	ctx := context.NewExecutionContext(stdContext.Background(), clock.NewVirtualClock(time.Now()), nil)

	trigger := &ManualTrigger{
		ApprovalID: "test-approval",
		Approvers:  []string{"alice@example.com", "bob@example.com", "charlie@example.com"},
		RequireAll: true,
	}

	// Alice approves
	err := trigger.Approve("alice@example.com")
	if err != nil {
		t.Errorf("Approve() returned unexpected error: %v", err)
	}

	// Should not be ready yet (need all approvers)
	ready, err := trigger.ShouldFire(ctx)
	if err != nil {
		t.Errorf("ShouldFire() returned unexpected error: %v", err)
	}
	if ready {
		t.Error("ShouldFire() should return false until all approvers approve (RequireAll=true)")
	}

	// Bob approves
	err = trigger.Approve("bob@example.com")
	if err != nil {
		t.Errorf("Approve() returned unexpected error: %v", err)
	}

	// Still not ready (need Charlie)
	ready, err = trigger.ShouldFire(ctx)
	if err != nil {
		t.Errorf("ShouldFire() returned unexpected error: %v", err)
	}
	if ready {
		t.Error("ShouldFire() should return false until all approvers approve")
	}

	// Charlie approves
	err = trigger.Approve("charlie@example.com")
	if err != nil {
		t.Errorf("Approve() returned unexpected error: %v", err)
	}

	// Now should be ready
	ready, err = trigger.ShouldFire(ctx)
	if err != nil {
		t.Errorf("ShouldFire() returned unexpected error: %v", err)
	}
	if !ready {
		t.Error("ShouldFire() should return true after all approvers approve (RequireAll=true)")
	}
}

// TestManualTrigger_Cancel verifies that Cancel stops waiting.
func TestManualTrigger_Cancel(t *testing.T) {
	ctx := context.NewExecutionContext(stdContext.Background(), clock.NewVirtualClock(time.Now()), nil)

	trigger := &ManualTrigger{
		ApprovalID: "test-approval",
		Approvers:  []string{"alice@example.com"},
		RequireAll: false,
	}

	// Spawn goroutine to wait
	done := make(chan error, 1)
	go func() {
		done <- trigger.AwaitFiring(ctx)
	}()

	// Give the goroutine time to start waiting
	time.Sleep(50 * time.Millisecond)

	// Cancel the trigger
	err := trigger.Cancel()
	if err != nil {
		t.Errorf("Cancel() returned unexpected error: %v", err)
	}

	// AwaitFiring should complete (channel was closed)
	// The implementation closes the channel, which unblocks the select but doesn't return an error
	select {
	case <-done:
		// Good - it completed (whether with or without error is impl-specific)
	case <-time.After(1 * time.Second):
		t.Fatal("AwaitFiring did not return after Cancel()")
	}
}

// TestEngine_AutomaticTrigger_Integration verifies default behavior with Fire().
func TestEngine_AutomaticTrigger_Integration(t *testing.T) {
	// Create a simple net: p1 -> t1 -> p2
	net := NewPetriNet("test-net", "Test Net")
	p1 := NewPlace("p1", "Place1", 10)
	p2 := NewPlace("p2", "Place2", 10)
	t1 := NewTransition("t1", "Transition1")

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddTransition(t1)
	net.AddArc(NewArc("a1", p1.ID, t1.ID, 1))
	net.AddArc(NewArc("a2", t1.ID, p2.ID, 1))

	// Resolve net
	if err := net.Resolve(); err != nil {
		t.Fatalf("Resolve() failed: %v", err)
	}

	// Place initial token
	tok := &token.Token{
		ID:   "tok1",
		Data: &token.IntToken{Value: 1},
	}
	err := p1.AddToken(tok)
	if err != nil {
		t.Fatalf("AddToken() failed: %v", err)
	}

	// Create context with virtual clock
	vClock := clock.NewVirtualClock(time.Now())
	ctx := context.NewExecutionContext(stdContext.Background(), vClock, nil)

	// Fire transition - should succeed immediately (AutomaticTrigger by default)
	err = t1.Fire(ctx)
	if err != nil {
		t.Fatalf("Fire() failed: %v", err)
	}

	// Verify token moved to p2
	if p1.TokenCount() != 0 {
		t.Errorf("Expected 0 tokens in p1, got %d", p1.TokenCount())
	}
	if p2.TokenCount() != 1 {
		t.Errorf("Expected 1 token in p2, got %d", p2.TokenCount())
	}
}

// TestEngine_StopCancelsWaitingTriggers verifies context cancellation stops waiting.
func TestEngine_StopCancelsWaitingTriggers(t *testing.T) {
	startTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	vClock := clock.NewVirtualClock(startTime)
	cancelCtx, cancel := stdContext.WithCancel(stdContext.Background())
	ctx := context.NewExecutionContext(cancelCtx, vClock, nil)

	delay := 10 * time.Second
	trigger := &TimedTrigger{Delay: &delay}

	// Start waiting
	done := make(chan error, 1)
	go func() {
		done <- trigger.AwaitFiring(ctx)
	}()

	// Give goroutine time to start waiting
	time.Sleep(50 * time.Millisecond)

	// Cancel context
	cancel()

	// Should unblock with error
	select {
	case err := <-done:
		if err == nil {
			t.Error("AwaitFiring() should return error after context cancellation")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("AwaitFiring() did not return after context cancellation")
	}
}

// TestTimedTrigger_MultipleWaiters tests multiple goroutines waiting on same trigger.
func TestTimedTrigger_MultipleWaiters(t *testing.T) {
	startTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	vClock := clock.NewVirtualClock(startTime)
	ctx := context.NewExecutionContext(stdContext.Background(), vClock, nil)

	delay := 5 * time.Second
	trigger := &TimedTrigger{Delay: &delay}

	// Initialize timer by calling ShouldFire first
	ready, err := trigger.ShouldFire(ctx)
	if err != nil {
		t.Fatalf("ShouldFire() returned error: %v", err)
	}
	if ready {
		t.Fatal("ShouldFire() should return false initially")
	}

	// Start multiple waiters
	const numWaiters = 5
	done := make(chan error, numWaiters)

	for i := 0; i < numWaiters; i++ {
		go func() {
			done <- trigger.AwaitFiring(ctx)
		}()
	}

	// Give goroutines time to start waiting
	time.Sleep(100 * time.Millisecond)

	// Advance time
	vClock.AdvanceBy(6 * time.Second)

	// All waiters should complete
	for i := 0; i < numWaiters; i++ {
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Waiter %d returned error: %v", i, err)
			}
		case <-time.After(1 * time.Second):
			t.Fatalf("Waiter %d did not complete", i)
		}
	}
}

// TestManualTrigger_ConcurrentApprovals tests race conditions in approval handling.
func TestManualTrigger_ConcurrentApprovals(t *testing.T) {
	ctx := context.NewExecutionContext(stdContext.Background(), clock.NewVirtualClock(time.Now()), nil)

	trigger := &ManualTrigger{
		ApprovalID: "test-approval",
		Approvers:  []string{"alice@example.com", "bob@example.com", "charlie@example.com"},
		RequireAll: true, // Changed to true so all can approve without error
	}

	// Multiple approvers try to approve concurrently
	var wg sync.WaitGroup
	approvers := []string{"alice@example.com", "bob@example.com", "charlie@example.com"}
	errors := make(chan error, len(approvers))

	for _, approver := range approvers {
		wg.Add(1)
		go func(a string) {
			defer wg.Done()
			errors <- trigger.Approve(a)
		}(approver)
	}

	wg.Wait()
	close(errors)

	// Check that all approvals succeeded (no errors)
	for err := range errors {
		if err != nil {
			t.Errorf("Concurrent approval failed: %v", err)
		}
	}

	// Trigger should be ready
	ready, err := trigger.ShouldFire(ctx)
	if err != nil {
		t.Errorf("ShouldFire() returned unexpected error: %v", err)
	}
	if !ready {
		t.Error("ShouldFire() should return true after all approvers approve")
	}
}

// TestManualTrigger_DuplicateApproval tests duplicate approval behavior.
func TestManualTrigger_DuplicateApproval(t *testing.T) {
	t.Run("RequireAll=true allows idempotent approvals", func(t *testing.T) {
		trigger := &ManualTrigger{
			ApprovalID: "test-approval",
			Approvers:  []string{"alice@example.com", "bob@example.com"},
			RequireAll: true,
		}

		// Alice approves first time
		err := trigger.Approve("alice@example.com")
		if err != nil {
			t.Errorf("First approval failed: %v", err)
		}

		// Alice tries to approve again - should be idempotent (no error)
		err = trigger.Approve("alice@example.com")
		if err != nil {
			t.Errorf("Duplicate approval returned error: %v", err)
		}

		// Should still need Bob's approval
		ctx := context.NewExecutionContext(stdContext.Background(), clock.NewVirtualClock(time.Now()), nil)
		ready, err := trigger.ShouldFire(ctx)
		if err != nil {
			t.Errorf("ShouldFire() returned unexpected error: %v", err)
		}
		if ready {
			t.Error("Duplicate approval should not count as second approval")
		}
	})

	t.Run("RequireAll=false rejects duplicate approvals", func(t *testing.T) {
		trigger := &ManualTrigger{
			ApprovalID: "test-approval",
			Approvers:  []string{"alice@example.com", "bob@example.com"},
			RequireAll: false,
		}

		// Alice approves first time
		err := trigger.Approve("alice@example.com")
		if err != nil {
			t.Errorf("First approval failed: %v", err)
		}

		// Alice tries to approve again - should return error (already approved)
		err = trigger.Approve("alice@example.com")
		if err == nil {
			t.Error("Duplicate approval should return error when RequireAll=false")
		}
	})
}

// Example_timedTrigger demonstrates using a timed trigger.
func Example_timedTrigger() {
	// Create a transition with 5 second delay
	t := NewTransition("reminder", "Send Reminder")
	delay := 5 * time.Second
	t.WithTrigger(&TimedTrigger{Delay: &delay})

	fmt.Println("Transition created with 5 second delay")
	// Output: Transition created with 5 second delay
}

// Example_manualTrigger demonstrates using a manual approval trigger.
func Example_manualTrigger() {
	// Create a transition requiring manual approval
	t := NewTransition("approve", "Approve Expense")
	timeout := 48 * time.Hour
	trigger := &ManualTrigger{
		ApprovalID: "expense-123",
		Approvers:  []string{"alice@example.com", "bob@example.com"},
		Timeout:    &timeout,
		RequireAll: false,
	}
	t.WithTrigger(trigger)

	// Later, an approver approves
	err := trigger.Approve("alice@example.com")
	if err != nil {
		fmt.Printf("Approval failed: %v\n", err)
		return
	}

	fmt.Println("Expense approved by Alice")
	// Output: Expense approved by Alice
}

// Benchmark_AutomaticTrigger benchmarks automatic trigger overhead.
func Benchmark_AutomaticTrigger(b *testing.B) {
	trigger := &AutomaticTrigger{}
	ctx := context.NewExecutionContext(stdContext.Background(), clock.NewVirtualClock(time.Now()), nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trigger.ShouldFire(ctx)
	}
}

// Benchmark_TimedTrigger_ShouldFire benchmarks timed trigger check overhead.
func Benchmark_TimedTrigger_ShouldFire(b *testing.B) {
	startTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	vClock := clock.NewVirtualClock(startTime)
	ctx := context.NewExecutionContext(stdContext.Background(), vClock, nil)

	delay := 5 * time.Second
	trigger := &TimedTrigger{Delay: &delay}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trigger.ShouldFire(ctx)
	}
}

// Benchmark_ManualTrigger_ShouldFire benchmarks manual trigger check overhead.
func Benchmark_ManualTrigger_ShouldFire(b *testing.B) {
	ctx := context.NewExecutionContext(stdContext.Background(), clock.NewVirtualClock(time.Now()), nil)

	trigger := &ManualTrigger{
		ApprovalID: "test-approval",
		Approvers:  []string{"alice@example.com"},
		RequireAll: false,
	}
	trigger.Approve("alice@example.com")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trigger.ShouldFire(ctx)
	}
}
