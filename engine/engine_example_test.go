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
	"log"
	"time"

	"github.com/jazzpetri/engine/clock"
	execContext "github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/engine"
	"github.com/jazzpetri/engine/petri"
	"github.com/jazzpetri/engine/task"
	"github.com/jazzpetri/engine/token"
)

// ExampleEngine_sequential demonstrates a simple sequential workflow
func ExampleEngine_sequential() {
	// Create a simple workflow: Start -> Process -> End
	net := petri.NewPetriNet("order-workflow", "Order Processing")

	// Define places
	pStart := petri.NewPlace("start", "Order Received", 10)
	pComplete := petri.NewPlace("complete", "Order Complete", 10)

	// Define transition with task
	tProcess := petri.NewTransition("process", "Process Order").WithTask(&task.InlineTask{
		Name: "process-order",
		Fn: func(ctx *execContext.ExecutionContext) error {
			fmt.Println("Processing order...")
			fmt.Println("Order completed!")
			return nil
		},
	})

	// Build the net
	net.AddPlace(pStart)
	net.AddPlace(pComplete)
	net.AddTransition(tProcess)

	// Connect with arcs: start -> process -> complete
	net.AddArc(petri.NewArc("arc1", "start", "process", 1))
	net.AddArc(petri.NewArc("arc2", "process", "complete", 1))

	// Resolve references
	if err := net.Resolve(); err != nil {
		log.Fatalf("Failed to resolve net: %v", err)
	}

	// Add initial token (incoming order)
	order := &token.Token{
		ID:   "order-123",
		Data: &petri.MapToken{Value: map[string]interface{}{"orderId": "123"}},
	}
	pStart.AddToken(order)

	// Create execution context
	ctx := execContext.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Create engine with sequential mode
	config := engine.Config{
		Mode:         engine.ModeSequential,
		PollInterval: 10 * time.Millisecond,
	}
	eng := engine.NewEngine(net, ctx, config)

	// Run the workflow
	eng.Start()
	eng.Wait()
	eng.Stop()

	// Output:
	// Processing order...
	// Order completed!
}

// ExampleEngine_parallel demonstrates concurrent workflow execution
func ExampleEngine_parallel() {
	// Create workflow with parallel processing
	net := petri.NewPetriNet("parallel-workflow", "Parallel Processing")

	// Define places
	pStart := petri.NewPlace("start", "Start", 10)
	pBranchA := petri.NewPlace("branchA", "Branch A", 10)
	pBranchB := petri.NewPlace("branchB", "Branch B", 10)
	pJoin := petri.NewPlace("join", "Join", 10)

	// Split transition
	tSplit := petri.NewTransition("split", "Split")

	// Parallel tasks
	tTaskA := petri.NewTransition("taskA", "Task A").WithTask(&task.InlineTask{
		Name: "task-a",
		Fn: func(ctx *execContext.ExecutionContext) error {
			fmt.Println("Executing Task A")
			return nil
		},
	})

	tTaskB := petri.NewTransition("taskB", "Task B").WithTask(&task.InlineTask{
		Name: "task-b",
		Fn: func(ctx *execContext.ExecutionContext) error {
			fmt.Println("Executing Task B")
			return nil
		},
	})

	// Build the net
	net.AddPlace(pStart)
	net.AddPlace(pBranchA)
	net.AddPlace(pBranchB)
	net.AddPlace(pJoin)
	net.AddTransition(tSplit)
	net.AddTransition(tTaskA)
	net.AddTransition(tTaskB)

	// Split: start -> split -> branchA and branchB
	net.AddArc(petri.NewArc("arc-split-in", "start", "split", 1))
	net.AddArc(petri.NewArc("arc-split-a", "split", "branchA", 1))
	net.AddArc(petri.NewArc("arc-split-b", "split", "branchB", 1))

	// Parallel execution
	net.AddArc(petri.NewArc("arc-a-in", "branchA", "taskA", 1))
	net.AddArc(petri.NewArc("arc-a-out", "taskA", "join", 1))
	net.AddArc(petri.NewArc("arc-b-in", "branchB", "taskB", 1))
	net.AddArc(petri.NewArc("arc-b-out", "taskB", "join", 1))

	// Resolve references
	if err := net.Resolve(); err != nil {
		log.Fatalf("Failed to resolve net: %v", err)
	}

	// Add initial token
	tok := &token.Token{
		ID:   "tok1",
		Data: &petri.MapToken{Value: map[string]interface{}{}},
	}
	pStart.AddToken(tok)

	// Create execution context
	ctx := execContext.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Create engine with concurrent mode
	config := engine.Config{
		Mode:                     engine.ModeConcurrent,
		MaxConcurrentTransitions: 5,
		PollInterval:             10 * time.Millisecond,
	}
	eng := engine.NewEngine(net, ctx, config)

	// Run the workflow
	eng.Start()
	eng.Wait()
	eng.Stop()

	// Output may be in either order due to concurrency:
	// Executing Task A
	// Executing Task B
	// Or:
	// Executing Task B
	// Executing Task A
}

// ExampleEngine_conditional demonstrates conditional routing with guards
func ExampleEngine_conditional() {
	// Create workflow with conditional routing
	net := petri.NewPetriNet("conditional-workflow", "Conditional Routing")

	// Define places
	pInput := petri.NewPlace("input", "Input", 10)
	pApproved := petri.NewPlace("approved", "Approved", 10)
	pRejected := petri.NewPlace("rejected", "Rejected", 10)

	// Transition with guard for approval (amount <= 1000)
	tApprove := petri.NewTransition("approve", "Auto Approve").
		WithGuard(func(ctx *execContext.ExecutionContext, tokens []*token.Token) (bool, error) {
			if len(tokens) == 0 {
				return false, nil
			}
			data, ok := tokens[0].Data.(*petri.MapToken)
			if !ok {
				return false, nil
			}
			amount, ok := data.Value["amount"].(float64)
			return ok && amount <= 1000.0, nil
		}).
		WithTask(&task.InlineTask{
			Name: "approve",
			Fn: func(ctx *execContext.ExecutionContext) error {
				fmt.Println("Order auto-approved")
				return nil
			},
		})

	// Transition with guard for rejection (amount > 1000)
	tReject := petri.NewTransition("reject", "Manual Review").
		WithGuard(func(ctx *execContext.ExecutionContext, tokens []*token.Token) (bool, error) {
			if len(tokens) == 0 {
				return false, nil
			}
			data, ok := tokens[0].Data.(*petri.MapToken)
			if !ok {
				return false, nil
			}
			amount, ok := data.Value["amount"].(float64)
			return ok && amount > 1000.0, nil
		}).
		WithTask(&task.InlineTask{
			Name: "reject",
			Fn: func(ctx *execContext.ExecutionContext) error {
				fmt.Println("Order requires manual review")
				return nil
			},
		})

	// Build the net
	net.AddPlace(pInput)
	net.AddPlace(pApproved)
	net.AddPlace(pRejected)
	net.AddTransition(tApprove)
	net.AddTransition(tReject)

	// Connect with arcs
	net.AddArc(petri.NewArc("arc-approve-in", "input", "approve", 1))
	net.AddArc(petri.NewArc("arc-approve-out", "approve", "approved", 1))
	net.AddArc(petri.NewArc("arc-reject-in", "input", "reject", 1))
	net.AddArc(petri.NewArc("arc-reject-out", "reject", "rejected", 1))

	// Resolve references
	if err := net.Resolve(); err != nil {
		log.Fatalf("Failed to resolve net: %v", err)
	}

	// Add token with small amount (will be auto-approved)
	smallOrder := &token.Token{
		ID:   "order-small",
		Data: &petri.MapToken{Value: map[string]interface{}{"amount": 500.0}},
	}
	pInput.AddToken(smallOrder)

	// Create execution context
	ctx := execContext.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Create and run engine
	eng := engine.NewEngine(net, ctx, engine.DefaultConfig())
	eng.Start()
	eng.Wait()
	eng.Stop()

	// Output:
	// Order auto-approved
}

// ExampleEngine_stepMode demonstrates step-by-step debugging
func ExampleEngine_stepMode() {
	// Create a simple 3-step workflow
	net := petri.NewPetriNet("step-workflow", "Step-by-Step Workflow")

	// Create places and transitions
	places := make([]*petri.Place, 4)
	for i := 0; i < 4; i++ {
		p := petri.NewPlace(fmt.Sprintf("p%d", i), fmt.Sprintf("Step %d", i), 10)
		places[i] = p
		net.AddPlace(p)
	}

	for i := 0; i < 3; i++ {
		taskName := fmt.Sprintf("Step %d", i+1)
		t := petri.NewTransition(fmt.Sprintf("t%d", i), taskName).WithTask(&task.InlineTask{
			Name: taskName,
			Fn: func(name string) func(*execContext.ExecutionContext) error {
				return func(ctx *execContext.ExecutionContext) error {
					fmt.Printf("Executing: %s\n", name)
					return nil
				}
			}(taskName),
		})
		net.AddTransition(t)

		net.AddArc(petri.NewArc(fmt.Sprintf("arc-in-%d", i), fmt.Sprintf("p%d", i), fmt.Sprintf("t%d", i), 1))
		net.AddArc(petri.NewArc(fmt.Sprintf("arc-out-%d", i), fmt.Sprintf("t%d", i), fmt.Sprintf("p%d", i+1), 1))
	}

	// Resolve references
	if err := net.Resolve(); err != nil {
		log.Fatalf("Failed to resolve net: %v", err)
	}

	// Add initial token
	tok := &token.Token{
		ID:   "tok1",
		Data: &petri.MapToken{Value: map[string]interface{}{}},
	}
	places[0].AddToken(tok)

	// Create execution context
	ctx := execContext.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Create engine in step mode
	config := engine.Config{
		Mode:     engine.ModeSequential,
		StepMode: true,
	}
	eng := engine.NewEngine(net, ctx, config)

	// Execute step by step
	fmt.Println("Starting step-by-step execution...")
	for i := 0; i < 3; i++ {
		fmt.Printf("\nStep %d:\n", i+1)
		if err := eng.Step(); err != nil {
			log.Fatalf("Step failed: %v", err)
		}
	}
	fmt.Println("\nWorkflow completed!")

	// Output:
	// Starting step-by-step execution...
	//
	// Step 1:
	// Executing: Step 1
	//
	// Step 2:
	// Executing: Step 2
	//
	// Step 3:
	// Executing: Step 3
	//
	// Workflow completed!
}
