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
	"fmt"
	"sync"

	"github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/task"
	"github.com/jazzpetri/engine/token"
)

// TransitionType categorizes transitions by their execution semantics.
// Each type is a composition of Guard/Trigger/Task primitives:
//   - AgentTransition: Executes an autonomous agent task
//   - ToolTransition: Executes a deterministic tool/function
//   - HumanTransition: Requires human approval or input (uses ManualTrigger)
//   - GateTransition: Pure control flow (guard-only, no task)
type TransitionType int

const (
	// AutoTransition is the default type (backward-compatible).
	// Behavior is determined entirely by the Guard/Trigger/Task configuration.
	AutoTransition TransitionType = iota

	// AgentTransition executes an AI agent with a reasoning loop.
	// Typically has a task with output validation guards.
	AgentTransition

	// ToolTransition executes a deterministic tool or function.
	// Has a Task (inline, HTTP, command) but no AI reasoning.
	ToolTransition

	// HumanTransition requires human approval or input.
	// Uses ManualTrigger for approval flow.
	HumanTransition

	// GateTransition is pure control flow with guard-only logic.
	// Has a Guard but no Task -- just routes tokens based on conditions.
	GateTransition
)

// String returns a human-readable name for the transition type.
func (tt TransitionType) String() string {
	switch tt {
	case AutoTransition:
		return "auto"
	case AgentTransition:
		return "agent"
	case ToolTransition:
		return "tool"
	case HumanTransition:
		return "human"
	case GateTransition:
		return "gate"
	default:
		return "unknown"
	}
}

// GuardFunc evaluates whether a transition can fire given input tokens.
// The function receives ExecutionContext for access to capabilities (Clock, Tracer,
// Metrics, Logger) and a slice of tokens that would be consumed by the transition.
//
// Returns:
//   - bool: true if the transition should be allowed to fire, false otherwise
//   - error: validation error if evaluation failed (treated as disabled)
//
// If no guard is set (nil), the transition is always enabled (given sufficient tokens).
//
// Error Handling
// Guards can now return errors to distinguish between "condition not met" (false, nil)
// and "validation error" (false, error). Errors are logged and the transition is disabled.
//
// Context Access
// Guards receive ExecutionContext to access capabilities like virtual clock, metrics,
// tracing, and logging. This enables time-based guards, observable guard logic, and
// guards that query external systems with cancellation support (via ctx.Context).
//
// Example - Simple guard:
//
//	trans.WithGuard(func(ctx *context.ExecutionContext, tokens []*token.Token) (bool, error) {
//	    app := tokens[0].Data.(*LoanApplication)
//	    return app.CreditScore >= 700, nil
//	})
//
// Example - Advanced guard with capabilities:
//
//	trans.WithGuard(func(ctx *context.ExecutionContext, tokens []*token.Token) (bool, error) {
//	    span := ctx.Tracer.StartSpan("guard.check_deadline")
//	    defer span.End()
//
//	    app := tokens[0].Data.(*LoanApplication)
//
//	    // Use virtual clock for testable time-based logic
//	    if ctx.Clock.Now().After(app.Deadline) {
//	        ctx.Logger.Info("application expired", map[string]interface{}{
//	            "app_id": app.ID,
//	            "deadline": app.Deadline,
//	        })
//	        return false, nil
//	    }
//
//	    // Query external system with cancellation support
//	    score, err := creditService.GetScore(ctx.Context, app.SSN)
//	    if err != nil {
//	        return false, fmt.Errorf("failed to get credit score: %w", err)
//	    }
//
//	    ctx.Metrics.Inc("guard.credit_check_total")
//	    return score >= 700, nil
//	})
type GuardFunc func(ctx *context.ExecutionContext, tokens []*token.Token) (bool, error)

// Transition consumes tokens from input places and produces tokens to output places.
// Transitions can have guards (conditional firing) and tasks (work execution).
//
// # Routing Patterns
//
// Transitions are the building blocks for workflow routing patterns:
//
// AND-split: One transition with multiple output arcs creates parallel flows.
//
//	t := NewTransition("split", "Create Parallel Tasks")
//	net.AddArc(NewArc("in", "input", "split", 1))
//	net.AddArc(NewArc("out1", "split", "branch_a", 1))  // Parallel branch 1
//	net.AddArc(NewArc("out2", "split", "branch_b", 1))  // Parallel branch 2
//
// AND-join: One transition with multiple input arcs synchronizes parallel flows.
//
//	t := NewTransition("join", "Synchronize")
//	net.AddArc(NewArc("in1", "branch_a", "join", 1))  // Wait for branch 1
//	net.AddArc(NewArc("in2", "branch_b", "join", 1))  // Wait for branch 2
//	net.AddArc(NewArc("out", "join", "merged", 1))
//
// OR-split: Multiple transitions with guards create exclusive choice.
//
//	t1 := NewTransition("high", "High Priority")
//	t1.WithGuard(func(ctx *context.ExecutionContext, tokens []*token.Token) (bool, error) {
//	    return priority(tokens) > 8, nil
//	})
//
//	t2 := NewTransition("low", "Low Priority")
//	t2.WithGuard(func(ctx *context.ExecutionContext, tokens []*token.Token) (bool, error) {
//	    return priority(tokens) <= 8, nil
//	})
//
//	// Both read from same place, but only one fires (exclusive choice)
//	net.AddArc(NewArc("a1", "decision", "high", 1))
//	net.AddArc(NewArc("a2", "decision", "low", 1))
//
// OR-join: Multiple transitions to the same place merge alternative paths.
//
//	t1 := NewTransition("merge_high", "Merge High")
//	net.AddArc(NewArc("a1", "high_path", "merge_high", 1))
//	net.AddArc(NewArc("a2", "merge_high", "output", 1))
//
//	t2 := NewTransition("merge_low", "Merge Low")
//	net.AddArc(NewArc("a3", "low_path", "merge_low", 1))
//	net.AddArc(NewArc("a4", "merge_low", "output", 1))  // Same destination
//
// See pkg/petri/ROUTING.md for comprehensive routing patterns guide.
//
// The hybrid structure stores arc ID references for serialization and direct
// arc pointers for execution. The Resolve() method converts IDs to pointers.
type Transition struct {
	// ID is the unique identifier for this transition
	ID string

	// Name is the human-readable name
	Name string

	// Type categorizes this transition's execution semantics.
	// Default is AutoTransition (backward-compatible).
	Type TransitionType

	// Guard is an optional function that determines if the transition can fire
	// nil = always enabled (given sufficient input tokens)
	Guard GuardFunc

	// Trigger determines WHEN this transition fires (separate from Guard which determines IF it CAN fire)
	// nil = AutomaticTrigger (fire immediately when enabled - default behavior)
	// See Trigger interface documentation for built-in trigger types
	Trigger Trigger

	// Task is the executable work to perform when this transition fires
	// nil = no task (just move tokens through the net)
	Task task.Task

	// InputArcIDs stores input arc IDs for serialization
	// These are resolved to direct pointers by PetriNet.Resolve()
	InputArcIDs []string

	// OutputArcIDs stores output arc IDs for serialization
	// These are resolved to direct pointers by PetriNet.Resolve()
	OutputArcIDs []string

	// inputArcs contains the resolved input arc pointers (runtime only)
	inputArcs []*Arc

	// outputArcs contains the resolved output arc pointers (runtime only)
	outputArcs []*Arc

	// netID is the ID of the Petri net this transition belongs to (runtime only)
	// Set during PetriNet.Resolve()
	netID string

	// resolved indicates whether ID references have been converted to pointers
	resolved bool

	// fireMu protects the firing process to prevent race conditions when multiple
	// goroutines try to fire the same transition concurrently
	fireMu sync.Mutex
}

// NewTransition creates a new transition with the specified ID and name.
// The transition is initialized with no guard (always enabled) and empty arc lists.
func NewTransition(id, name string) *Transition {
	return &Transition{
		ID:           id,
		Name:         name,
		InputArcIDs:  make([]string, 0),
		OutputArcIDs: make([]string, 0),
		inputArcs:    make([]*Arc, 0),
		outputArcs:   make([]*Arc, 0),
	}
}

// WithType sets the transition type.
// Returns the transition itself to allow method chaining.
// Example:
//
//	trans := NewTransition("t1", "Process").WithType(AgentTransition)
func (t *Transition) WithType(tt TransitionType) *Transition {
	t.Type = tt
	return t
}

// WithGuard sets a guard function for conditional firing.
// Returns the transition itself to allow method chaining.
// Example:
//
//	trans := NewTransition("t1", "Process").WithGuard(
//	    func(ctx *context.ExecutionContext, tokens []*token.Token) (bool, error) {
//	        // Check if tokens meet some condition
//	        return condition, nil
//	    })
func (t *Transition) WithGuard(guard GuardFunc) *Transition {
	t.Guard = guard
	return t
}

// WithTrigger sets a trigger that determines WHEN this transition fires.
// Returns the transition itself to allow method chaining.
//
// The trigger is evaluated after the guard passes, controlling the timing of execution:
//   - AutomaticTrigger: Fire immediately (default if nil)
//   - TimedTrigger: Fire after delay or at specific time
//   - ManualTrigger: Wait for external approval
//   - EventTrigger: Fire on external event
//
// Example - Timed delay:
//
//	delay := 24 * time.Hour
//	trans := NewTransition("reminder", "Send Reminder").WithTrigger(&TimedTrigger{
//	    Delay: &delay,
//	})
//
// Example - Manual approval:
//
//	trans := NewTransition("approve", "Approve Expense").WithTrigger(&ManualTrigger{
//	    ApprovalID: "expense-123",
//	    Approvers:  []string{"alice@example.com"},
//	    Timeout:    ptr(48 * time.Hour),
//	})
func (t *Transition) WithTrigger(trigger Trigger) *Transition {
	t.Trigger = trigger
	return t
}

// WithTask sets the task to execute when this transition fires.
// Returns the transition itself to allow method chaining.
// Example:
//
//	trans := NewTransition("t1", "Process").WithTask(&task.InlineTask{
//	    Name: "process-order",
//	    Fn: func(ctx *context.ExecutionContext) error {
//	        // Process the order
//	        return nil
//	    },
//	})
func (t *Transition) WithTask(task task.Task) *Transition {
	t.Task = task
	return t
}

// InputArcs returns the list of input arcs.
// Returns an empty slice if the transition has not been resolved yet.
// The arcs are only available after PetriNet.Resolve() has been called.
func (t *Transition) InputArcs() []*Arc {
	if t.inputArcs == nil {
		return make([]*Arc, 0)
	}
	return t.inputArcs
}

// OutputArcs returns the list of output arcs.
// Returns an empty slice if the transition has not been resolved yet.
// The arcs are only available after PetriNet.Resolve() has been called.
func (t *Transition) OutputArcs() []*Arc {
	if t.outputArcs == nil {
		return make([]*Arc, 0)
	}
	return t.outputArcs
}

// IsEnabled checks if the transition can fire.
// A transition is enabled if:
//  1. It has been resolved (IDs converted to pointers)
//  2. All input places have sufficient tokens (>= arc weight)
//  3. The guard function passes (if guard is set)
//
// This method is thread-safe and does not consume tokens during guard evaluation.
// It uses PeekTokensWithCount() to examine tokens without removing them, preventing
// race conditions when multiple goroutines call IsEnabled() concurrently.
//
// Race Condition in IsEnabled() Guard Evaluation
// Previously, tokens were temporarily removed and then restored, creating a race window
// where concurrent IsEnabled() calls could interfere with each other. Now we use
// non-consuming peek operations that are protected by a mutex in the Place.
//
// Guard Error Handling
// Guards can now return errors. If a guard returns an error, it's returned to the caller.
// This allows guards to distinguish between "condition not met" (false, nil) and
// "validation error" (false, error).
//
// Returns:
//   - (true, nil): Transition is enabled and guard passed
//   - (false, nil): Transition is disabled (insufficient tokens or guard condition not met)
//   - (false, error): Guard evaluation error (validation failure, system error, etc.)
func (t *Transition) IsEnabled(ctx *context.ExecutionContext) (bool, error) {
	if !t.resolved {
		return false, nil
	}

	// Collect input tokens for evaluation using non-consuming peek
	// PeekTokensWithCount() atomically checks token availability and returns
	// a snapshot, preventing race conditions with concurrent IsEnabled() calls.
	var inputTokens []*token.Token

	for _, arc := range t.inputArcs {
		place, ok := arc.Source.(*Place)
		if !ok {
			return false, nil
		}

		// Peek at arc.Weight tokens from this place
		// This atomically checks if sufficient tokens exist and returns them
		tokens, err := place.PeekTokensWithCount(arc.Weight)
		if err != nil {
			// Failed to peek tokens (insufficient tokens available)
			return false, nil
		}

		inputTokens = append(inputTokens, tokens...)
	}

	// If no guard is set, transition is enabled (we have sufficient tokens)
	if t.Guard == nil {
		return true, nil
	}

	// Evaluate guard with panic recovery and error handling
	// Returns (bool, error) from guard, with panic recovery
	return t.evaluateGuardSafely(ctx, inputTokens)
}

// evaluateGuardSafely wraps guard evaluation in panic recovery and error handling.
// Returns (enabled, error) where error is nil if guard passed or guard condition not met.
// This prevents guard panics from crashing the entire workflow engine.
//
// Guard Panic Recovery
// Guards that panic are treated as returning (false, error), not crashing the system.
// The panic is logged if a logger is available in the context.
//
// Guard Error Handling
// Guards can return (false, error) to provide meaningful error messages.
// These errors are propagated to callers for proper handling.
//
// Returns:
//   - (true, nil): Guard passed
//   - (false, nil): Guard condition not met (normal)
//   - (false, error): Guard evaluation error or panic
func (t *Transition) evaluateGuardSafely(ctx *context.ExecutionContext, tokens []*token.Token) (enabled bool, guardErr error) {
	defer func() {
		if r := recover(); r != nil {
			// Guard panicked - treat as error
			enabled = false
			guardErr = fmt.Errorf("guard panicked: %v", r)
			if ctx.Logger != nil {
				ctx.Logger.Error("guard panicked", map[string]interface{}{
					"net_id":          t.netID,
					"transition_id":   t.ID,
					"transition_name": t.Name,
					"panic":           r,
				})
			}
			if ctx.Metrics != nil {
				ctx.Metrics.Inc("guard_panic_total")
			}
		}
	}()

	// Call guard with new signature (ctx, tokens) -> (bool, error)
	result, err := t.Guard(ctx, tokens)

	// If guard returned an error, log it and propagate
	if err != nil {
		if ctx.Logger != nil {
			ctx.Logger.Error("guard evaluation error", map[string]interface{}{
				"net_id":          t.netID,
				"transition_id":   t.ID,
				"transition_name": t.Name,
				"error":           err.Error(),
			})
		}
		if ctx.Metrics != nil {
			ctx.Metrics.Inc("guard_error_total")
		}
		return false, err
	}

	// Guard succeeded - return its result
	return result, nil
}

// String returns a human-readable representation of the transition for debugging.
// String() Methods for Debugging
func (t *Transition) String() string {
	hasGuard := ""
	if t.Guard != nil {
		hasGuard = " guarded"
	}
	typeStr := ""
	if t.Type != AutoTransition {
		typeStr = fmt.Sprintf(" type=%s", t.Type)
	}
	inCount := len(t.InputArcIDs)
	outCount := len(t.OutputArcIDs)
	return fmt.Sprintf("Transition[%s: \"%s\" in=%d out=%d%s%s]", t.ID, t.Name, inCount, outCount, typeStr, hasGuard)
}
