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
	stdContext "context"
	"testing"
	"time"

	"github.com/jazzpetri/engine/clock"
	"github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/petri"
	"github.com/jazzpetri/engine/token"
)

// TestWorkflowState_String verifies string representation of workflow states
func TestWorkflowState_String(t *testing.T) {
	tests := []struct {
		state    WorkflowState
		expected string
	}{
		{StateRunning, "Running"},
		{StateIdle, "Idle"},
		{StateBlocked, "Blocked"},
		{StateComplete, "Complete"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.state.String(); got != tt.expected {
				t.Errorf("String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestState_Running verifies StateRunning detection when transitions are enabled
func TestState_Running(t *testing.T) {
	// Create a simple workflow: input -> t1 -> output
	net := petri.NewPetriNet("test-net", "Test Net")

	input := petri.NewPlace("input", "Input", 10)
	output := petri.NewPlace("output", "Output", 10)

	net.AddPlace(input)
	net.AddPlace(output)

	t1 := petri.NewTransition("t1", "Process")
	net.AddTransition(t1)

	net.AddArc(petri.NewArc("a1", "input", "t1", 1))
	net.AddArc(petri.NewArc("a2", "t1", "output", 1))

	net.Resolve()

	// Add token to input place
	tok := token.NewToken("tok1", 42)
	if err := input.AddToken(tok); err != nil {
		t.Fatalf("Failed to add token: %v", err)
	}

	// Create engine
	vClock := clock.NewVirtualClock(time.Now())
	ctx := context.NewExecutionContext(stdContext.Background(), vClock, nil)
	config := DefaultConfig()
	config.StepMode = true // Use step mode for deterministic testing
	engine := NewEngine(net, ctx, config)

	// Check state - should be RUNNING (transition enabled)
	state := engine.State()
	if state != StateRunning {
		t.Errorf("State() = %v, want %v", state, StateRunning)
	}
}

// TestState_Complete verifies StateComplete detection when token in terminal place
func TestState_Complete(t *testing.T) {
	// Create workflow with terminal place
	net := petri.NewPetriNet("test-net", "Test Net")

	input := petri.NewPlace("input", "Input", 10)
	terminal := petri.NewPlace("terminal", "Terminal", 10)
	terminal.IsTerminal = true // Mark as terminal

	net.AddPlace(input)
	net.AddPlace(terminal)

	t1 := petri.NewTransition("t1", "Complete")
	net.AddTransition(t1)

	net.AddArc(petri.NewArc("a1", "input", "t1", 1))
	net.AddArc(petri.NewArc("a2", "t1", "terminal", 1))

	net.Resolve()

	// Create engine
	vClock := clock.NewVirtualClock(time.Now())
	ctx := context.NewExecutionContext(stdContext.Background(), vClock, nil)
	config := DefaultConfig()
	config.StepMode = true
	engine := NewEngine(net, ctx, config)

	// Add token to input and fire transition
	tok := token.NewToken("tok1", 42)
	if err := input.AddToken(tok); err != nil {
		t.Fatalf("Failed to add token: %v", err)
	}

	// Fire transition to move token to terminal place
	if err := t1.Fire(ctx); err != nil {
		t.Fatalf("Failed to fire transition: %v", err)
	}

	// Check state - should be COMPLETE (token in terminal place)
	state := engine.State()
	if state != StateComplete {
		t.Errorf("State() = %v, want %v", state, StateComplete)
	}
}

// TestState_Blocked verifies StateBlocked detection when no progress possible
func TestState_Blocked(t *testing.T) {
	// Create workflow with no tokens
	net := petri.NewPetriNet("test-net", "Test Net")

	input := petri.NewPlace("input", "Input", 10)
	output := petri.NewPlace("output", "Output", 10)

	net.AddPlace(input)
	net.AddPlace(output)

	t1 := petri.NewTransition("t1", "Process")
	net.AddTransition(t1)

	net.AddArc(petri.NewArc("a1", "input", "t1", 1))
	net.AddArc(petri.NewArc("a2", "t1", "output", 1))

	net.Resolve()

	// Create engine - no tokens added
	vClock := clock.NewVirtualClock(time.Now())
	ctx := context.NewExecutionContext(stdContext.Background(), vClock, nil)
	config := DefaultConfig()
	config.StepMode = true
	engine := NewEngine(net, ctx, config)

	// Check state - should be BLOCKED (no tokens, no enabled transitions, no pending triggers)
	state := engine.State()
	if state != StateBlocked {
		t.Errorf("State() = %v, want %v", state, StateBlocked)
	}
}

// TestState_Idle_WithTimer verifies StateIdle detection with pending timer trigger
func TestState_Idle_WithTimer(t *testing.T) {
	// Create workflow with timed transition
	net := petri.NewPetriNet("test-net", "Test Net")

	input := petri.NewPlace("input", "Input", 10)
	output := petri.NewPlace("output", "Output", 10)

	net.AddPlace(input)
	net.AddPlace(output)

	// Transition with 5 second delay
	delay := 5 * time.Second
	t1 := petri.NewTransition("t1", "Delayed Process")
	t1.WithTrigger(&petri.TimedTrigger{Delay: &delay})
	net.AddTransition(t1)

	net.AddArc(petri.NewArc("a1", "input", "t1", 1))
	net.AddArc(petri.NewArc("a2", "t1", "output", 1))

	net.Resolve()

	// Create engine with virtual clock
	startTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	vClock := clock.NewVirtualClock(startTime)
	ctx := context.NewExecutionContext(stdContext.Background(), vClock, nil)
	config := DefaultConfig()
	config.PollInterval = 10 * time.Millisecond
	engine := NewEngine(net, ctx, config)

	// Add token to input
	tok := token.NewToken("tok1", 42)
	if err := input.AddToken(tok); err != nil {
		t.Fatalf("Failed to add token: %v", err)
	}

	// Start engine
	if err := engine.Start(); err != nil {
		t.Fatalf("Failed to start engine: %v", err)
	}

	// Give engine time to detect transition and spawn waiter
	time.Sleep(100 * time.Millisecond)

	// Check state - should be IDLE (waiting for timer)
	state := engine.State()
	if state != StateIdle {
		t.Errorf("State() = %v, want %v (waiting for timer)", state, StateIdle)
	}

	// Advance clock past delay
	vClock.AdvanceBy(6 * time.Second)

	// Give engine time to fire transition
	time.Sleep(100 * time.Millisecond)

	// Check state - should be COMPLETE or BLOCKED depending on terminal place
	state = engine.State()
	if state != StateBlocked {
		t.Errorf("State() = %v, want %v (after transition fired)", state, StateBlocked)
	}

	// Stop engine
	if err := engine.Stop(); err != nil {
		t.Fatalf("Failed to stop engine: %v", err)
	}
}

// TestState_Idle_WithManualTrigger verifies StateIdle detection with manual approval
func TestState_Idle_WithManualTrigger(t *testing.T) {
	// Create workflow with manual trigger
	net := petri.NewPetriNet("test-net", "Test Net")

	input := petri.NewPlace("input", "Input", 10)
	output := petri.NewPlace("output", "Output", 10)

	net.AddPlace(input)
	net.AddPlace(output)

	// Transition with manual approval
	t1 := petri.NewTransition("t1", "Manual Approval")
	manualTrigger := &petri.ManualTrigger{
		ApprovalID: "approval-1",
		Approvers:  []string{"alice@example.com"},
	}
	t1.WithTrigger(manualTrigger)
	net.AddTransition(t1)

	net.AddArc(petri.NewArc("a1", "input", "t1", 1))
	net.AddArc(petri.NewArc("a2", "t1", "output", 1))

	net.Resolve()

	// Create engine
	vClock := clock.NewVirtualClock(time.Now())
	ctx := context.NewExecutionContext(stdContext.Background(), vClock, nil)
	config := DefaultConfig()
	config.PollInterval = 10 * time.Millisecond
	engine := NewEngine(net, ctx, config)

	// Add token to input
	tok := token.NewToken("tok1", 42)
	if err := input.AddToken(tok); err != nil {
		t.Fatalf("Failed to add token: %v", err)
	}

	// Start engine
	if err := engine.Start(); err != nil {
		t.Fatalf("Failed to start engine: %v", err)
	}

	// Give engine time to detect transition and spawn waiter
	time.Sleep(100 * time.Millisecond)

	// Check state - should be IDLE (waiting for manual approval)
	state := engine.State()
	if state != StateIdle {
		t.Errorf("State() = %v, want %v (waiting for approval)", state, StateIdle)
	}

	// Approve the transition
	manualTrigger.Approve("alice@example.com")

	// Give engine time to fire transition
	time.Sleep(100 * time.Millisecond)

	// Check state - should be BLOCKED (no more enabled transitions)
	state = engine.State()
	if state != StateBlocked {
		t.Errorf("State() = %v, want %v (after approval)", state, StateBlocked)
	}

	// Stop engine
	if err := engine.Stop(); err != nil {
		t.Fatalf("Failed to stop engine: %v", err)
	}
}

// TestState_CompleteWithMultiplePlaces verifies StateComplete with multiple terminal places
func TestState_CompleteWithMultiplePlaces(t *testing.T) {
	// Create workflow with multiple terminal places
	net := petri.NewPetriNet("test-net", "Test Net")

	input := petri.NewPlace("input", "Input", 10)
	terminal1 := petri.NewPlace("terminal1", "Terminal 1", 10)
	terminal1.IsTerminal = true
	terminal2 := petri.NewPlace("terminal2", "Terminal 2", 10)
	terminal2.IsTerminal = true

	net.AddPlace(input)
	net.AddPlace(terminal1)
	net.AddPlace(terminal2)

	// Two transitions to different terminal places
	t1 := petri.NewTransition("t1", "Path 1")
	t2 := petri.NewTransition("t2", "Path 2")
	net.AddTransition(t1)
	net.AddTransition(t2)

	// Guard for exclusive choice
	t1.WithGuard(func(ctx *context.ExecutionContext, tokens []*token.Token) (bool, error) {
		intToken := tokens[0].Data.(*token.IntToken)
		return intToken.Value > 50, nil
	})

	t2.WithGuard(func(ctx *context.ExecutionContext, tokens []*token.Token) (bool, error) {
		intToken := tokens[0].Data.(*token.IntToken)
		return intToken.Value <= 50, nil
	})

	net.AddArc(petri.NewArc("a1", "input", "t1", 1))
	net.AddArc(petri.NewArc("a2", "t1", "terminal1", 1))
	net.AddArc(petri.NewArc("a3", "input", "t2", 1))
	net.AddArc(petri.NewArc("a4", "t2", "terminal2", 1))

	net.Resolve()

	// Create engine
	vClock := clock.NewVirtualClock(time.Now())
	ctx := context.NewExecutionContext(stdContext.Background(), vClock, nil)
	config := DefaultConfig()
	config.StepMode = true
	engine := NewEngine(net, ctx, config)

	// Test with value > 50 (goes to terminal1)
	tok := token.NewToken("tok1", 75)
	if err := input.AddToken(tok); err != nil {
		t.Fatalf("Failed to add token: %v", err)
	}

	// Fire enabled transition
	if err := t1.Fire(ctx); err != nil {
		t.Fatalf("Failed to fire t1: %v", err)
	}

	// Check state - should be COMPLETE
	state := engine.State()
	if state != StateComplete {
		t.Errorf("State() = %v, want %v (token in terminal1)", state, StateComplete)
	}
}

// TestState_TransitionPriority verifies state priority: Complete > Running > Idle > Blocked
func TestState_TransitionPriority(t *testing.T) {
	// Create workflow where we can test state priority
	net := petri.NewPetriNet("test-net", "Test Net")

	// Places
	input1 := petri.NewPlace("input1", "Input 1", 10)
	input2 := petri.NewPlace("input2", "Input 2", 10)
	terminal := petri.NewPlace("terminal", "Terminal", 10)
	terminal.IsTerminal = true
	output := petri.NewPlace("output", "Output", 10)

	net.AddPlace(input1)
	net.AddPlace(input2)
	net.AddPlace(terminal)
	net.AddPlace(output)

	// Transitions
	t1 := petri.NewTransition("t1", "To Terminal") // Goes to terminal
	delay := 5 * time.Second
	t2 := petri.NewTransition("t2", "Delayed")
	t2.WithTrigger(&petri.TimedTrigger{Delay: &delay})

	net.AddTransition(t1)
	net.AddTransition(t2)

	net.AddArc(petri.NewArc("a1", "input1", "t1", 1))
	net.AddArc(petri.NewArc("a2", "t1", "terminal", 1))
	net.AddArc(petri.NewArc("a3", "input2", "t2", 1))
	net.AddArc(petri.NewArc("a4", "t2", "output", 1))

	net.Resolve()

	// Create engine
	vClock := clock.NewVirtualClock(time.Now())
	ctx := context.NewExecutionContext(stdContext.Background(), vClock, nil)
	config := DefaultConfig()
	config.StepMode = true
	engine := NewEngine(net, ctx, config)

	// Add token to terminal place directly (simulating completed workflow)
	tokTerminal := token.NewToken("tok-terminal", 1)
	if err := terminal.AddToken(tokTerminal); err != nil {
		t.Fatalf("Failed to add token to terminal: %v", err)
	}

	// Add token to input1 (t1 enabled)
	tok1 := token.NewToken("tok1", 2)
	if err := input1.AddToken(tok1); err != nil {
		t.Fatalf("Failed to add token to input1: %v", err)
	}

	// State should be COMPLETE (highest priority) even though t1 is enabled
	state := engine.State()
	if state != StateComplete {
		t.Errorf("State() = %v, want %v (complete has priority over running)", state, StateComplete)
	}
}

// TestState_ConcurrentAccess verifies State() is thread-safe
func TestState_ConcurrentAccess(t *testing.T) {
	// Create simple workflow
	net := petri.NewPetriNet("test-net", "Test Net")

	input := petri.NewPlace("input", "Input", 10)
	output := petri.NewPlace("output", "Output", 10)

	net.AddPlace(input)
	net.AddPlace(output)

	t1 := petri.NewTransition("t1", "Process")
	net.AddTransition(t1)

	net.AddArc(petri.NewArc("a1", "input", "t1", 1))
	net.AddArc(petri.NewArc("a2", "t1", "output", 1))

	net.Resolve()

	// Add token
	tok := token.NewToken("tok1", 42)
	if err := input.AddToken(tok); err != nil {
		t.Fatalf("Failed to add token: %v", err)
	}

	// Create engine
	vClock := clock.NewVirtualClock(time.Now())
	ctx := context.NewExecutionContext(stdContext.Background(), vClock, nil)
	config := DefaultConfig()
	engine := NewEngine(net, ctx, config)

	// Start engine
	if err := engine.Start(); err != nil {
		t.Fatalf("Failed to start engine: %v", err)
	}

	// Call State() from multiple goroutines concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = engine.State() // Should not panic
			}
			done <- true
		}()
	}

	// Wait for all goroutines to finish
	for i := 0; i < 10; i++ {
		<-done
	}

	// Stop engine
	if err := engine.Stop(); err != nil {
		t.Fatalf("Failed to stop engine: %v", err)
	}
}
