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

package engine_test

import (
	"context"
	"fmt"
	"time"

	"github.com/jazzpetri/engine/clock"
	execContext "github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/engine"
	"github.com/jazzpetri/engine/petri"
	"github.com/jazzpetri/engine/token"
)

// Example_workflowState demonstrates state detection in a workflow
func Example_workflowState() {
	// Create a simple workflow: start -> process -> end
	net := petri.NewPetriNet("workflow", "Simple Workflow")

	// Places
	start := petri.NewPlace("start", "Start", 10)
	processing := petri.NewPlace("processing", "Processing", 10)
	end := petri.NewPlace("end", "End", 10)
	end.IsTerminal = true // Mark as terminal

	net.AddPlace(start)
	net.AddPlace(processing)
	net.AddPlace(end)

	// Transitions
	t1 := petri.NewTransition("t1", "Begin Processing")
	t2 := petri.NewTransition("t2", "Complete")

	net.AddTransition(t1)
	net.AddTransition(t2)

	// Arcs
	net.AddArc(petri.NewArc("a1", "start", "t1", 1))
	net.AddArc(petri.NewArc("a2", "t1", "processing", 1))
	net.AddArc(petri.NewArc("a3", "processing", "t2", 1))
	net.AddArc(petri.NewArc("a4", "t2", "end", 1))

	net.Resolve()

	// Create engine
	vClock := clock.NewVirtualClock(time.Now())
	ctx := execContext.NewExecutionContext(context.Background(), vClock, nil)
	config := engine.DefaultConfig()
	config.StepMode = true
	eng := engine.NewEngine(net, ctx, config)

	// Initially blocked (no tokens)
	fmt.Printf("Initial state: %s\n", eng.State())

	// Add token to start place
	tok := token.NewToken("work-1", "data")
	start.AddToken(tok)

	// Now running (t1 is enabled)
	fmt.Printf("After adding token: %s\n", eng.State())

	// Fire t1 (moves token to processing)
	t1.Fire(ctx)
	fmt.Printf("After first transition: %s\n", eng.State())

	// Fire t2 (moves token to terminal place)
	t2.Fire(ctx)
	fmt.Printf("After second transition: %s\n", eng.State())

	// Output:
	// Initial state: Blocked
	// After adding token: Running
	// After first transition: Running
	// After second transition: Complete
}

// Example_workflowStateWithTimer demonstrates idle state with timer trigger
func Example_workflowStateWithTimer() {
	// Create workflow with delayed transition
	net := petri.NewPetriNet("delayed", "Delayed Workflow")

	input := petri.NewPlace("input", "Input", 10)
	output := petri.NewPlace("output", "Output", 10)

	net.AddPlace(input)
	net.AddPlace(output)

	// Transition with 1 second delay
	delay := 1 * time.Second
	t1 := petri.NewTransition("t1", "Delayed Process")
	t1.WithTrigger(&petri.TimedTrigger{Delay: &delay})

	net.AddTransition(t1)

	net.AddArc(petri.NewArc("a1", "input", "t1", 1))
	net.AddArc(petri.NewArc("a2", "t1", "output", 1))

	net.Resolve()

	// Create engine with virtual clock
	startTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	vClock := clock.NewVirtualClock(startTime)
	ctx := execContext.NewExecutionContext(context.Background(), vClock, nil)
	config := engine.DefaultConfig()
	config.PollInterval = 10 * time.Millisecond
	eng := engine.NewEngine(net, ctx, config)

	// Add token and start engine
	tok := token.NewToken("work-1", 42)
	input.AddToken(tok)

	eng.Start()
	time.Sleep(50 * time.Millisecond) // Let engine spawn waiter

	// Should be idle (waiting for timer)
	fmt.Printf("Waiting for timer: %s\n", eng.State())

	// Advance clock past delay
	vClock.AdvanceBy(2 * time.Second)
	time.Sleep(50 * time.Millisecond) // Let transition fire

	// Should be blocked (no more enabled transitions)
	fmt.Printf("After timer fired: %s\n", eng.State())

	eng.Stop()

	// Output:
	// Waiting for timer: Idle
	// After timer fired: Blocked
}
