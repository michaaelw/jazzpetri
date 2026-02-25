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
	"github.com/jazzpetri/engine/context"
)

// Trigger determines WHEN and HOW a transition fires.
//
// Triggers are separate from Guards:
//   - Guard: Determines IF a transition CAN fire (business logic, preconditions)
//   - Trigger: Determines WHEN a transition fires (timing, approval, events)
//
// Execution Order:
//  1. Check structural enablement (sufficient tokens in input places)
//  2. Evaluate Guard (business logic - returns true if conditions met)
//  3. Check Trigger.ShouldFire() (non-blocking check if ready to fire)
//  4. If not ready, spawn goroutine to call Trigger.AwaitFiring() (blocks until ready)
//  5. When ready, re-check IsEnabled() (tokens may have been consumed)
//  6. Fire the transition
//
// Guards are evaluated BEFORE triggers to avoid wasteful waiting on transitions
// that would fail business logic anyway. For example, don't wait for approval
// on an order that doesn't meet minimum amount requirements.
//
// Built-in Trigger Types:
//   - AutomaticTrigger: Fires immediately when enabled (default behavior)
//   - TimedTrigger: Fires after a delay or at a specific time
//   - ManualTrigger: Waits for explicit external approval (human-in-the-loop)
//   - EventTrigger: Fires when an external event is received
//
// Example - Automatic (default):
//
//	t := NewTransition("process", "Process Order")
//	// No trigger = AutomaticTrigger (fires immediately when enabled)
//
// Example - Timed delay:
//
//	t := NewTransition("reminder", "Send Reminder")
//	delay := 24 * time.Hour
//	t.WithTrigger(&TimedTrigger{Delay: &delay})
//
// Example - Manual approval:
//
//	t := NewTransition("approve", "Approve Expense")
//	t.WithTrigger(&ManualTrigger{
//	    ApprovalID: "expense-123",
//	    Approvers:  []string{"alice@example.com"},
//	    Timeout:    ptr(48 * time.Hour),
//	})
//
// Example - Event-driven:
//
//	t := NewTransition("on_webhook", "Process Webhook")
//	t.WithTrigger(&EventTrigger{
//	    EventType: "webhook.github.push",
//	})
//
// Thread Safety:
// Trigger implementations must be safe for concurrent calls to ShouldFire().
// AwaitFiring() will be called from a dedicated goroutine per transition.
type Trigger interface {
	// ShouldFire returns true if the transition is ready to fire immediately.
	// This method is called after the guard passes and must not block.
	//
	// Returns:
	//   - (true, nil): Transition is ready to fire now
	//   - (false, nil): Transition is not ready, call AwaitFiring() to wait
	//   - (false, error): Trigger evaluation error
	//
	// Thread Safety: Must be safe for concurrent calls.
	ShouldFire(ctx *context.ExecutionContext) (bool, error)

	// AwaitFiring blocks until the transition is ready to fire.
	// Only called if ShouldFire() returned (false, nil).
	//
	// This method runs in a dedicated goroutine spawned by the engine.
	// It should respect ctx.Context for cancellation (engine stop, workflow cancel).
	//
	// Returns:
	//   - nil: Transition is now ready to fire
	//   - error: Waiting failed (timeout, cancellation, etc.)
	//
	// After AwaitFiring() returns nil, the engine will:
	//   1. Re-check IsEnabled() (tokens may have been consumed while waiting)
	//   2. Fire the transition if still enabled
	//
	// Thread Safety: Called from dedicated goroutine, one per transition.
	AwaitFiring(ctx *context.ExecutionContext) error

	// Cancel stops any pending waits and performs cleanup.
	// Called when the engine stops or the workflow is cancelled.
	//
	// Returns:
	//   - nil: Cleanup successful
	//   - error: Cleanup failed (logged but not fatal)
	//
	// Thread Safety: Must be safe to call concurrently with AwaitFiring().
	Cancel() error
}

// AutomaticTrigger fires immediately when the transition is enabled.
// This is the default behavior and maintains backward compatibility.
//
// An AutomaticTrigger never blocks - the transition fires as soon as
// sufficient tokens are available and the guard passes.
//
// Usage:
//
//	t := NewTransition("process", "Process Order")
//	// No trigger specified = AutomaticTrigger behavior (default)
//
// Or explicitly:
//
//	t.WithTrigger(&AutomaticTrigger{})
type AutomaticTrigger struct{}

// ShouldFire always returns true - automatic transitions are always ready.
func (t *AutomaticTrigger) ShouldFire(ctx *context.ExecutionContext) (bool, error) {
	return true, nil
}

// AwaitFiring never blocks - automatic transitions fire immediately.
func (t *AutomaticTrigger) AwaitFiring(ctx *context.ExecutionContext) error {
	return nil // Never called since ShouldFire() always returns true
}

// Cancel does nothing - automatic transitions have no state to clean up.
func (t *AutomaticTrigger) Cancel() error {
	return nil
}
