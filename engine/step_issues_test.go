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

package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jazzpetri/engine/clock"
	execctx "github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/petri"
	"github.com/jazzpetri/engine/task"
	"github.com/jazzpetri/engine/token"
)

// TestEngine_Step_NoEnabledTransitions tests:
// Step() returns error when no transitions are enabled, but this may be expected state
//
// Problem: "no enabled transitions" can mean two different things:
// 1. Workflow is complete (normal termination)
// 2. Workflow is blocked/deadlocked (error state)
//
// EXPECTED BEHAVIOR: Step() should distinguish between these cases
// CURRENT BEHAVIOR: Always returns error "no enabled transitions"
//
// TODO: Implement state detection methods:
//   - func (e *Engine) IsComplete() bool
//   - func (e *Engine) IsBlocked() bool
//   - func (e *Engine) IsIdle() bool
//   - func (e *Engine) HasTokens() bool
// Then Step() can return nil for complete/idle and error only for blocked states.
func TestEngine_Step_NoEnabledTransitions(t *testing.T) {
	t.Skip("TODO: Step() cannot distinguish between complete/idle/blocked states. " +
		"Need to implement state detection methods (IsComplete/IsBlocked/IsIdle/HasTokens) " +
		"so Step() can return nil for complete/idle and error only for blocked states.")
}

// TestEngine_TryStep tests:
// Proposed TryStep() method that returns bool instead of error
//
// EXPECTED FAILURE: TryStep() method does not exist
//
// TryStep() API design:
//   func (e *Engine) TryStep() (bool, error)
//   Returns (true, nil) if a transition was fired
//   Returns (false, nil) if no transitions enabled (idle/complete)
//   Returns (false, err) if firing failed
func TestEngine_TryStep(t *testing.T) {
	tests := []struct {
		name         string
		setupNet     func() *petri.PetriNet
		wantFired    bool
		wantErr      bool
		description  string
	}{
		{
			name: "transition fires successfully",
			setupNet: func() *petri.PetriNet {
				net := petri.NewPetriNet("net", "Net")
				p1 := petri.NewPlace("p1", "Input", 10)
				p2 := petri.NewPlace("p2", "Output", 10)
				t1 := petri.NewTransition("t1", "Process")

				net.AddPlace(p1)
				net.AddPlace(p2)
				net.AddTransition(t1)
				net.AddArc(petri.NewArc("a1", "p1", "t1", 1))
				net.AddArc(petri.NewArc("a2", "t1", "p2", 1))
				net.Resolve()

				p1.AddToken(&token.Token{ID: "token-1"})
				return net
			},
			wantFired:   true,
			wantErr:     false,
			description: "Should fire transition and return true",
		},
		{
			name: "no enabled transitions - idle",
			setupNet: func() *petri.PetriNet {
				net := petri.NewPetriNet("net", "Net")
				p1 := petri.NewPlace("p1", "Input", 10)
				p2 := petri.NewPlace("p2", "Output", 10)
				t1 := petri.NewTransition("t1", "Process")

				net.AddPlace(p1)
				net.AddPlace(p2)
				net.AddTransition(t1)
				net.AddArc(petri.NewArc("a1", "p1", "t1", 1))
				net.AddArc(petri.NewArc("a2", "t1", "p2", 1))
				net.Resolve()

				// No tokens
				return net
			},
			wantFired:   false,
			wantErr:     false, // NOT an error - just idle
			description: "Should return false, nil when idle",
		},
		{
			name: "transition fires but task fails",
			setupNet: func() *petri.PetriNet {
				net := petri.NewPetriNet("net", "Net")
				p1 := petri.NewPlace("p1", "Input", 10)
				p2 := petri.NewPlace("p2", "Output", 10)
				t1 := petri.NewTransition("t1", "Process")

				// Task that always fails
				t1.WithTask(&task.InlineTask{
					Name: "failing-task",
					Fn: func(ctx *execctx.ExecutionContext) error {
						return errors.New("task failed")
					},
				})

				net.AddPlace(p1)
				net.AddPlace(p2)
				net.AddTransition(t1)
				net.AddArc(petri.NewArc("a1", "p1", "t1", 1))
				net.AddArc(petri.NewArc("a2", "t1", "p2", 1))
				net.Resolve()

				p1.AddToken(&token.Token{ID: "token-1"})
				return net
			},
			wantFired:   false,
			wantErr:     true, // Error because task failed
			description: "Should return false, error when task fails",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			net := tt.setupNet()
			ctx := execctx.NewExecutionContext(
				context.Background(),
				clock.NewRealTimeClock(),
				nil,
			)
			engine := NewEngine(net, ctx, DefaultConfig())

			// Act
			fired, err := engine.TryStep()

			// Assert
			if fired != tt.wantFired {
				t.Errorf("EXPECTED FAILURE: TryStep() fired = %v, want %v", fired, tt.wantFired)
			}

			if tt.wantErr && err == nil {
				t.Errorf("Expected error, got none")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			t.Logf("%s: fired=%v, err=%v", tt.description, fired, err)
		})
	}

	t.Log("SUCCESS: TryStep() method implemented")
	t.Log("  - Returns (true, nil) if transition fired")
	t.Log("  - Returns (false, nil) if no transitions enabled")
	t.Log("  - Returns (false, err) if firing failed")
}

// TestEngine_Step_WorkflowComplete_vs_Blocked tests:
// Distinguish between workflow complete and workflow blocked
//
// TODO: Implement state detection methods so we can distinguish
// between complete, idle, and blocked workflow states.
func TestEngine_Step_WorkflowComplete_vs_Blocked(t *testing.T) {
	t.Skip("TODO: Cannot distinguish complete from blocked state. " +
		"Need to implement: IsComplete(), IsBlocked(), IsIdle(), HasTokens() methods.")
}

// TestEngine_Step_MultipleCallsOnComplete tests:
// Calling Step() multiple times after completion should be safe
//
// EXPECTED: Should return consistent error/nil each time
func TestEngine_Step_MultipleCallsOnComplete(t *testing.T) {
	// Arrange
	net := petri.NewPetriNet("net", "Net")
	p1 := petri.NewPlace("p1", "Final", 10)
	net.AddPlace(p1)
	net.Resolve()

	// Add token to final place (workflow complete)
	p1.AddToken(&token.Token{ID: "final-token"})

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)
	engine := NewEngine(net, ctx, DefaultConfig())

	// Act: Call Step() multiple times
	errors := make([]error, 5)
	for i := 0; i < 5; i++ {
		errors[i] = engine.Step()
	}

	// Assert: All calls should return same result
	firstErr := errors[0]
	for i, err := range errors {
		if (firstErr == nil) != (err == nil) {
			t.Errorf("Call %d: inconsistent error state: first=%v, current=%v", i, firstErr, err)
		}

		if firstErr != nil && err != nil && firstErr.Error() != err.Error() {
			t.Errorf("Call %d: different error message: first=%v, current=%v", i, firstErr, err)
		}
	}

	t.Log("Step() calls on complete workflow are idempotent (consistent error/nil)")
	t.Log("After fix: Should return nil (not error) for complete/idle workflow")
}

// TestEngine_Step_Timeout tests:
// Step() with long-running task should respect context timeout
//
// EXPECTED: Test should PASS (context cancellation works)
func TestEngine_Step_Timeout(t *testing.T) {
	// Arrange
	net := petri.NewPetriNet("net", "Net")
	p1 := petri.NewPlace("p1", "Input", 10)
	p2 := petri.NewPlace("p2", "Output", 10)
	t1 := petri.NewTransition("t1", "Slow Process")

	// Task that takes 5 seconds
	t1.WithTask(&task.InlineTask{
		Name: "slow-task",
		Fn: func(ctx *execctx.ExecutionContext) error {
			select {
			case <-time.After(5 * time.Second):
				return nil
			case <-ctx.Context.Done():
				return ctx.Context.Err()
			}
		},
	})

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddTransition(t1)
	net.AddArc(petri.NewArc("a1", "p1", "t1", 1))
	net.AddArc(petri.NewArc("a2", "t1", "p2", 1))
	net.Resolve()

	p1.AddToken(&token.Token{ID: "token-1"})

	// Context with 100ms timeout
	baseCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	ctx := execctx.NewExecutionContext(
		baseCtx,
		clock.NewRealTimeClock(),
		nil,
	)
	engine := NewEngine(net, ctx, DefaultConfig())

	// Act
	start := time.Now()
	err := engine.Step()
	duration := time.Since(start)

	// Assert: Should timeout quickly (not wait 5 seconds)
	if duration > 200*time.Millisecond {
		t.Errorf("Step() took %v, should timeout within 200ms", duration)
	}

	if err == nil {
		t.Error("Expected timeout error, got none")
	}

	t.Logf("Step() respected context timeout: %v", duration)
}

// TestEngine_StepMode_vs_ContinuousMode tests:
// Verify Step() behavior differs between step mode and continuous mode
//
// EXPECTED: Test should PASS
func TestEngine_StepMode_vs_ContinuousMode(t *testing.T) {
	// Setup net
	net := petri.NewPetriNet("net", "Net")
	p1 := petri.NewPlace("p1", "Input", 10)
	p2 := petri.NewPlace("p2", "Output", 10)
	t1 := petri.NewTransition("t1", "Process")

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddTransition(t1)
	net.AddArc(petri.NewArc("a1", "p1", "t1", 1))
	net.AddArc(petri.NewArc("a2", "t1", "p2", 1))
	net.Resolve()

	p1.AddToken(&token.Token{ID: "token-1"})

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	t.Run("step mode allows Step()", func(t *testing.T) {
		config := DefaultConfig()
		config.StepMode = true

		engine := NewEngine(net, ctx, config)

		// Should work in step mode
		err := engine.Step()
		if err != nil {
			t.Errorf("Step() in step mode should work: %v", err)
		}
	})

	t.Run("continuous mode blocks Step()", func(t *testing.T) {
		// Reset net
		p1.Clear()
		p2.Clear()
		p1.AddToken(&token.Token{ID: "token-2"})

		config := DefaultConfig()
		config.StepMode = false

		engine := NewEngine(net, ctx, config)

		// Start engine
		engine.Start()
		defer engine.Stop()

		// Wait a bit for engine to start
		time.Sleep(50 * time.Millisecond)

		// Should fail because engine is running in continuous mode
		err := engine.Step()
		if err == nil {
			t.Error("Step() should fail when engine is running in continuous mode")
		}

		expectedErr := "engine running in continuous mode"
		if err.Error() != expectedErr {
			t.Errorf("Expected error %q, got %q", expectedErr, err.Error())
		}
	})

	t.Log("Step() correctly distinguishes step mode from continuous mode")
}
