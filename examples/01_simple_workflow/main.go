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

// Package main demonstrates the simplest possible JazzPetri workflow: a
// three-place linear order-processing pipeline.
//
// The workflow models the lifecycle of a customer order:
//
//	[order_received] --> (process) --> [order_processing] --> (ship) --> [order_complete]
//
// Each place holds tokens (data) and each transition performs a task when it
// fires. The engine polls for enabled transitions — a transition is enabled
// when its input place contains at least one token — and fires them in order.
//
// Run with:
//
//	go run ./examples/01_simple_workflow
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jazzpetri/engine/clock"
	execContext "github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/engine"
	"github.com/jazzpetri/engine/petri"
	"github.com/jazzpetri/engine/task"
	"github.com/jazzpetri/engine/token"
)

func main() {
	fmt.Println("=== Example 01: Simple Order Processing Workflow ===")
	fmt.Println()

	// ------------------------------------------------------------------
	// Step 1: Create the Petri net container.
	// A PetriNet is the graph that holds places, transitions, and arcs.
	// ------------------------------------------------------------------
	net := petri.NewPetriNet("order-workflow", "Order Processing Workflow")

	// ------------------------------------------------------------------
	// Step 2: Define places.
	// Places hold tokens. Capacity limits how many tokens can be in a
	// place at once. Setting IsTerminal = true on the final place tells
	// the engine that tokens here represent completed work.
	// ------------------------------------------------------------------
	pReceived := petri.NewPlace("order_received", "Order Received", 10)
	pReceived.IsInitial = true // Tokens start here

	pProcessing := petri.NewPlace("order_processing", "Order Processing", 10)

	pComplete := petri.NewPlace("order_complete", "Order Complete", 10)
	pComplete.IsTerminal = true // Workflow ends here

	// ------------------------------------------------------------------
	// Step 3: Define transitions with InlineTask functions.
	// An InlineTask is the simplest way to attach work to a transition.
	// The function is called when the transition fires.
	// ------------------------------------------------------------------
	tProcess := petri.NewTransition("process", "Process Order").
		WithTask(&task.InlineTask{
			Name: "process-order",
			Fn: func(ctx *execContext.ExecutionContext) error {
				fmt.Println("  [process] Validating order details...")
				fmt.Println("  [process] Charging payment for order #123...")
				fmt.Println("  [process] Order #123 processed successfully.")
				return nil
			},
		})

	tShip := petri.NewTransition("ship", "Ship Order").
		WithTask(&task.InlineTask{
			Name: "ship-order",
			Fn: func(ctx *execContext.ExecutionContext) error {
				fmt.Println("  [ship] Allocating warehouse stock...")
				fmt.Println("  [ship] Printing shipping label for order #123...")
				fmt.Println("  [ship] Order #123 dispatched to carrier.")
				return nil
			},
		})

	// ------------------------------------------------------------------
	// Step 4: Add all elements to the net.
	// ------------------------------------------------------------------
	net.AddPlace(pReceived)
	net.AddPlace(pProcessing)
	net.AddPlace(pComplete)
	net.AddTransition(tProcess)
	net.AddTransition(tShip)

	// ------------------------------------------------------------------
	// Step 5: Connect elements with arcs.
	// An arc from a place to a transition is an INPUT arc (consumes tokens).
	// An arc from a transition to a place is an OUTPUT arc (produces tokens).
	// The weight (last argument) is how many tokens are consumed/produced.
	// ------------------------------------------------------------------
	// order_received --> process --> order_processing
	net.AddArc(petri.NewArc("arc-recv-process", "order_received", "process", 1))
	net.AddArc(petri.NewArc("arc-process-proc", "process", "order_processing", 1))
	// order_processing --> ship --> order_complete
	net.AddArc(petri.NewArc("arc-proc-ship", "order_processing", "ship", 1))
	net.AddArc(petri.NewArc("arc-ship-done", "ship", "order_complete", 1))

	// ------------------------------------------------------------------
	// Step 6: Resolve the net.
	// Resolve converts string ID references in arcs to direct pointers,
	// enabling efficient graph traversal during execution.
	// ------------------------------------------------------------------
	if err := net.Resolve(); err != nil {
		log.Fatalf("failed to resolve net: %v", err)
	}

	// ------------------------------------------------------------------
	// Step 7: Add an initial token to kick off the workflow.
	// Tokens carry data (MapToken, IntToken, StringToken, etc.).
	// Here we use a MapToken to hold arbitrary order fields.
	// ------------------------------------------------------------------
	orderToken := &token.Token{
		ID: "order-123",
		Data: &petri.MapToken{Value: map[string]interface{}{
			"orderId":  "123",
			"customer": "Alice",
			"total":    49.99,
		}},
	}
	pReceived.AddToken(orderToken)
	fmt.Printf("Token 'order-123' placed in '%s'.\n\n", pReceived.Name)

	// ------------------------------------------------------------------
	// Step 8: Create an ExecutionContext.
	// The context carries a clock (real or virtual), a Go context for
	// cancellation, and optionally a logger, tracer, and metrics.
	// ------------------------------------------------------------------
	ctx := execContext.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil, // no pre-selected token; the engine selects tokens
	)

	// ------------------------------------------------------------------
	// Step 9: Configure and start the engine.
	// ModeSequential fires one transition at a time — ideal for simple
	// linear workflows. PollInterval controls how often the engine checks
	// for enabled transitions.
	// ------------------------------------------------------------------
	cfg := engine.Config{
		Mode:         engine.ModeSequential,
		PollInterval: 10 * time.Millisecond,
	}
	eng := engine.NewEngine(net, ctx, cfg)

	fmt.Println("Starting engine...")
	eng.Start()

	// Wait blocks until no enabled transitions remain (workflow done).
	eng.Wait()

	// Stop cleans up goroutines and resources.
	eng.Stop()

	// ------------------------------------------------------------------
	// Step 10: Inspect the final state.
	// ------------------------------------------------------------------
	fmt.Printf("\nTokens in '%s': %d\n", pComplete.Name, pComplete.TokenCount())
	fmt.Println("\nWorkflow complete! Order #123 has been processed and shipped.")
}
