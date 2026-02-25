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

package state_test

import (
	"context"
	"fmt"
	"time"

	"github.com/jazzpetri/engine/clock"
	execctx "github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/engine"
	"github.com/jazzpetri/engine/petri"
	"github.com/jazzpetri/engine/state"
	"github.com/jazzpetri/engine/task"
	"github.com/jazzpetri/engine/token"
)

// Example_workflowStateInspection demonstrates using markings to inspect workflow state
func Example_workflowStateInspection() {
	// Create a simple order processing workflow
	net := petri.NewPetriNet("order-workflow", "Order Processing")

	// Places
	submitted := petri.NewPlace("submitted", "Order Submitted", 10)
	validated := petri.NewPlace("validated", "Order Validated", 10)
	fulfilled := petri.NewPlace("fulfilled", "Order Fulfilled", 10)

	// Transitions
	validate := petri.NewTransition("validate", "Validate Order").WithTask(&task.InlineTask{
		Name: "validate-order",
		Fn:   func(ctx *execctx.ExecutionContext) error { return nil },
	})
	fulfill := petri.NewTransition("fulfill", "Fulfill Order").WithTask(&task.InlineTask{
		Name: "fulfill-order",
		Fn:   func(ctx *execctx.ExecutionContext) error { return nil },
	})

	// Build workflow
	net.AddPlace(submitted)
	net.AddPlace(validated)
	net.AddPlace(fulfilled)
	net.AddTransition(validate)
	net.AddTransition(fulfill)

	net.AddArc(petri.NewArc("a1", "submitted", "validate", 1))
	net.AddArc(petri.NewArc("a2", "validate", "validated", 1))
	net.AddArc(petri.NewArc("a3", "validated", "fulfill", 1))
	net.AddArc(petri.NewArc("a4", "fulfill", "fulfilled", 1))
	net.Resolve()

	// Start with an order token
	order := &token.Token{ID: "order-123", Data: &token.StringToken{Value: "Customer order"}}
	submitted.AddToken(order)

	// Create execution context
	clk := clock.NewVirtualClock(time.Now())
	ctx := execctx.NewExecutionContext(context.Background(), clk, nil)

	// Capture initial state
	initial, _ := state.NewMarking(net, clk)
	fmt.Printf("Initial state: %d token(s) in 'submitted'\n", initial.PlaceTokenCount("submitted"))

	// Process order through validation
	validate.Fire(ctx)
	afterValidation, _ := state.NewMarking(net, clk)
	fmt.Printf("After validation: %d token(s) in 'validated'\n", afterValidation.PlaceTokenCount("validated"))

	// Complete fulfillment
	fulfill.Fire(ctx)
	final, _ := state.NewMarking(net, clk)
	fmt.Printf("Final state: %d token(s) in 'fulfilled'\n", final.PlaceTokenCount("fulfilled"))

	// Output:
	// Initial state: 1 token(s) in 'submitted'
	// After validation: 1 token(s) in 'validated'
	// Final state: 1 token(s) in 'fulfilled'
}

// Example_debuggingWorkflowProgress shows using markings to debug stuck workflows
func Example_debuggingWorkflowProgress() {
	// Create workflow that might get stuck
	net := petri.NewPetriNet("debug-workflow", "Debug Example")

	p1 := petri.NewPlace("p1", "Start", 10)
	p2 := petri.NewPlace("p2", "Middle", 10)
	p3 := petri.NewPlace("p3", "End", 10)

	t1 := petri.NewTransition("t1", "Step 1").WithTask(&task.InlineTask{
		Name: "step1",
		Fn:   func(ctx *execctx.ExecutionContext) error { return nil },
	})
	t2 := petri.NewTransition("t2", "Step 2").WithTask(&task.InlineTask{
		Name: "step2",
		Fn:   func(ctx *execctx.ExecutionContext) error { return nil },
	})

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddPlace(p3)
	net.AddTransition(t1)
	net.AddTransition(t2)

	net.AddArc(petri.NewArc("a1", "p1", "t1", 1))
	net.AddArc(petri.NewArc("a2", "t1", "p2", 1))
	net.AddArc(petri.NewArc("a3", "p2", "t2", 1))
	net.AddArc(petri.NewArc("a4", "t2", "p3", 1))
	net.Resolve()

	// Add initial token
	p1.AddToken(&token.Token{ID: "tok1", Data: &token.StringToken{Value: "data"}})

	clk := clock.NewVirtualClock(time.Now())
	ctx := execctx.NewExecutionContext(context.Background(), clk, nil)

	// Process first step
	t1.Fire(ctx)

	// Capture current state for debugging
	current, _ := state.NewMarking(net, clk)

	// Inspect where tokens are
	for placeID, placeState := range current.Places {
		if len(placeState.Tokens) > 0 {
			fmt.Printf("Found %d token(s) in place '%s'\n", len(placeState.Tokens), placeID)
			for _, tok := range placeState.Tokens {
				fmt.Printf("  - Token ID: %s\n", tok.ID)
			}
		}
	}

	// Output:
	// Found 1 token(s) in place 'p2'
	//   - Token ID: tok1
}

// Example_comparingStates shows comparing markings at different stages
func Example_comparingStates() {
	// Create workflow
	net := petri.NewPetriNet("compare-workflow", "State Comparison")

	p1 := petri.NewPlace("p1", "Place 1", 10)
	p2 := petri.NewPlace("p2", "Place 2", 10)
	t1 := petri.NewTransition("t1", "Transition 1").WithTask(&task.InlineTask{
		Name: "task1",
		Fn:   func(ctx *execctx.ExecutionContext) error { return nil },
	})

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddTransition(t1)
	net.AddArc(petri.NewArc("a1", "p1", "t1", 1))
	net.AddArc(petri.NewArc("a2", "t1", "p2", 1))
	net.Resolve()

	p1.AddToken(&token.Token{ID: "tok1", Data: &token.StringToken{Value: "data"}})

	clk := clock.NewVirtualClock(time.Now())
	ctx := execctx.NewExecutionContext(context.Background(), clk, nil)

	// Capture before
	before, _ := state.NewMarking(net, clk)

	// Execute
	t1.Fire(ctx)

	// Capture after
	after, _ := state.NewMarking(net, clk)

	// Compare states
	if before.Equal(after) {
		fmt.Println("States are the same")
	} else {
		fmt.Println("States differ")
		fmt.Printf("Before: %d tokens total\n", before.TokenCount())
		fmt.Printf("After: %d tokens total\n", after.TokenCount())
		fmt.Printf("Before p1: %d, After p1: %d\n", before.PlaceTokenCount("p1"), after.PlaceTokenCount("p1"))
		fmt.Printf("Before p2: %d, After p2: %d\n", before.PlaceTokenCount("p2"), after.PlaceTokenCount("p2"))
	}

	// Output:
	// States differ
	// Before: 1 tokens total
	// After: 1 tokens total
	// Before p1: 1, After p1: 0
	// Before p2: 0, After p2: 1
}

// Example_engineIntegration demonstrates using Engine.GetMarking()
func Example_engineIntegration() {
	// Create workflow
	net := petri.NewPetriNet("engine-workflow", "Engine Integration")

	p1 := petri.NewPlace("p1", "Input", 10)
	p2 := petri.NewPlace("p2", "Output", 10)
	t1 := petri.NewTransition("t1", "Process").WithTask(&task.InlineTask{
		Name: "process",
		Fn:   func(ctx *execctx.ExecutionContext) error { return nil },
	})

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddTransition(t1)
	net.AddArc(petri.NewArc("a1", "p1", "t1", 1))
	net.AddArc(petri.NewArc("a2", "t1", "p2", 1))
	net.Resolve()

	p1.AddToken(&token.Token{ID: "work-item", Data: &token.StringToken{Value: "task"}})

	// Create engine in step mode
	clk := clock.NewVirtualClock(time.Now())
	ctx := execctx.NewExecutionContext(context.Background(), clk, nil)
	eng := engine.NewEngine(net, ctx, engine.Config{
		Mode:     engine.ModeSequential,
		StepMode: true,
	})

	// Get marking before execution
	before, _ := eng.GetMarking()
	fmt.Printf("Before execution: %d token(s) ready for processing\n", before.PlaceTokenCount("p1"))

	// Execute one step
	eng.Step()

	// Get marking after execution
	after, _ := eng.GetMarking()
	fmt.Printf("After execution: %d token(s) processed\n", after.PlaceTokenCount("p2"))

	// Output:
	// Before execution: 1 token(s) ready for processing
	// After execution: 1 token(s) processed
}
