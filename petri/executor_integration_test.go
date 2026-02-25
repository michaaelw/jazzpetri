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
	"context"
	"errors"
	"testing"

	"github.com/jazzpetri/engine/clock"
	execctx "github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/task"
	"github.com/jazzpetri/engine/token"
)

// TestIntegration_SequentialWorkflow tests a sequential workflow (A -> B -> C)
func TestIntegration_SequentialWorkflow(t *testing.T) {
	// Create net: p1 -> t1 -> p2 -> t2 -> p3
	net := NewPetriNet("sequential-net", "Sequential Workflow")

	p1 := NewPlace("p1", "Start", 10)
	p2 := NewPlace("p2", "Middle", 10)
	p3 := NewPlace("p3", "End", 10)

	t1 := NewTransition("t1", "Step 1")
	t2 := NewTransition("t2", "Step 2")

	// Add task to track execution order
	var executionOrder []string
	t1.WithTask(&task.InlineTask{
		Name: "task1",
		Fn: func(ctx *execctx.ExecutionContext) error {
			executionOrder = append(executionOrder, "task1")
			return nil
		},
	})
	t2.WithTask(&task.InlineTask{
		Name: "task2",
		Fn: func(ctx *execctx.ExecutionContext) error {
			executionOrder = append(executionOrder, "task2")
			return nil
		},
	})

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddPlace(p3)
	net.AddTransition(t1)
	net.AddTransition(t2)

	// Connect: p1 -> t1 -> p2 -> t2 -> p3
	net.AddArc(NewArc("a1", "p1", "t1", 1))
	net.AddArc(NewArc("a2", "t1", "p2", 1))
	net.AddArc(NewArc("a3", "p2", "t2", 1))
	net.AddArc(NewArc("a4", "t2", "p3", 1))

	if err := net.Resolve(); err != nil {
		t.Fatalf("Failed to resolve net: %v", err)
	}

	// Add initial token
	tok := &token.Token{ID: "tok1", Data: &MapToken{Value: make(map[string]interface{})}}
	if err := p1.AddToken(tok); err != nil {
		t.Fatalf("Failed to add token: %v", err)
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Execute workflow
	if err := t1.Fire(ctx); err != nil {
		t.Fatalf("t1 fire failed: %v", err)
	}

	if err := t2.Fire(ctx); err != nil {
		t.Fatalf("t2 fire failed: %v", err)
	}

	// Verify execution order
	if len(executionOrder) != 2 {
		t.Errorf("Expected 2 tasks executed, got %d", len(executionOrder))
	}
	if executionOrder[0] != "task1" {
		t.Errorf("Expected task1 first, got %s", executionOrder[0])
	}
	if executionOrder[1] != "task2" {
		t.Errorf("Expected task2 second, got %s", executionOrder[1])
	}

	// Verify final state
	if p1.TokenCount() != 0 {
		t.Errorf("Expected p1 empty, got %d tokens", p1.TokenCount())
	}
	if p2.TokenCount() != 0 {
		t.Errorf("Expected p2 empty, got %d tokens", p2.TokenCount())
	}
	if p3.TokenCount() != 1 {
		t.Errorf("Expected p3 to have 1 token, got %d", p3.TokenCount())
	}
}

// TestIntegration_ParallelSplitJoin tests parallel execution with split/join
func TestIntegration_ParallelSplitJoin(t *testing.T) {
	// Create net: p1 -> t_split -> p2 + p3 -> t_join -> p4
	net := NewPetriNet("parallel-net", "Parallel Workflow")

	p1 := NewPlace("p1", "Start", 10)
	p2 := NewPlace("p2", "Branch A", 10)
	p3 := NewPlace("p3", "Branch B", 10)
	p4 := NewPlace("p4", "End", 10)

	tSplit := NewTransition("t_split", "Split")
	tJoin := NewTransition("t_join", "Join")

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddPlace(p3)
	net.AddPlace(p4)
	net.AddTransition(tSplit)
	net.AddTransition(tJoin)

	// Connect: p1 -> t_split -> (p2, p3) -> t_join -> p4
	net.AddArc(NewArc("a1", "p1", "t_split", 1))
	net.AddArc(NewArc("a2", "t_split", "p2", 1))
	net.AddArc(NewArc("a3", "t_split", "p3", 1))
	net.AddArc(NewArc("a4", "p2", "t_join", 1))
	net.AddArc(NewArc("a5", "p3", "t_join", 1))
	net.AddArc(NewArc("a6", "t_join", "p4", 1))

	if err := net.Resolve(); err != nil {
		t.Fatalf("Failed to resolve net: %v", err)
	}

	// Add initial token
	tok := &token.Token{ID: "tok1", Data: &MapToken{Value: make(map[string]interface{})}}
	if err := p1.AddToken(tok); err != nil {
		t.Fatalf("Failed to add token: %v", err)
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Execute workflow
	if err := tSplit.Fire(ctx); err != nil {
		t.Fatalf("tSplit fire failed: %v", err)
	}

	// Verify split worked - tokens in both branches
	if p2.TokenCount() != 1 || p3.TokenCount() != 1 {
		t.Errorf("Split failed: p2=%d, p3=%d", p2.TokenCount(), p3.TokenCount())
	}

	// tJoin should be enabled now that both branches have tokens
	if enabled, _ := tJoin.IsEnabled(ctx); !enabled {
		t.Error("tJoin should be enabled when both branches have tokens")
	}

	// Fire join to synchronize both branches
	if err := tJoin.Fire(ctx); err != nil {
		t.Fatalf("tJoin fire failed: %v", err)
	}

	// Verify final state
	if p1.TokenCount() != 0 {
		t.Errorf("Expected p1 empty, got %d tokens", p1.TokenCount())
	}
	if p2.TokenCount() != 0 {
		t.Errorf("Expected p2 empty, got %d tokens", p2.TokenCount())
	}
	if p3.TokenCount() != 0 {
		t.Errorf("Expected p3 empty, got %d tokens", p3.TokenCount())
	}
	if p4.TokenCount() != 1 {
		t.Errorf("Expected p4 to have 1 token, got %d", p4.TokenCount())
	}
}

// TestIntegration_ConditionalRouting tests conditional routing with guards
func TestIntegration_ConditionalRouting(t *testing.T) {
	// Create net: p1 -> (t_even -> p2) or (t_odd -> p3)
	net := NewPetriNet("conditional-net", "Conditional Routing")

	p1 := NewPlace("p1", "Start", 10)
	p2 := NewPlace("p2", "Even Path", 10)
	p3 := NewPlace("p3", "Odd Path", 10)

	tEven := NewTransition("t_even", "Even").WithGuard(func(ctx *execctx.ExecutionContext, tokens []*token.Token) (bool, error) {
		if len(tokens) == 0 {
			return false, nil
		}
		intData, ok := tokens[0].Data.(*intTokenData)
		return ok && intData.value%2 == 0, nil
	})

	tOdd := NewTransition("t_odd", "Odd").WithGuard(func(ctx *execctx.ExecutionContext, tokens []*token.Token) (bool, error) {
		if len(tokens) == 0 {
			return false, nil
		}
		intData, ok := tokens[0].Data.(*intTokenData)
		return ok && intData.value%2 == 1, nil
	})

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddPlace(p3)
	net.AddTransition(tEven)
	net.AddTransition(tOdd)

	// Connect
	net.AddArc(NewArc("a1", "p1", "t_even", 1))
	net.AddArc(NewArc("a2", "t_even", "p2", 1))
	net.AddArc(NewArc("a3", "p1", "t_odd", 1))
	net.AddArc(NewArc("a4", "t_odd", "p3", 1))

	if err := net.Resolve(); err != nil {
		t.Fatalf("Failed to resolve net: %v", err)
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Test even number
	tokEven := &token.Token{ID: "tok_even", Data: &intTokenData{value: 42}}
	if err := p1.AddToken(tokEven); err != nil {
		t.Fatalf("Failed to add token: %v", err)
	}

	if enabled, _ := tEven.IsEnabled(ctx); !enabled {
		t.Error("tEven should be enabled for even number")
	}
	if enabled, _ := tOdd.IsEnabled(ctx); enabled {
		t.Error("tOdd should not be enabled for even number")
	}

	if err := tEven.Fire(ctx); err != nil {
		t.Fatalf("tEven fire failed: %v", err)
	}

	if p2.TokenCount() != 1 {
		t.Errorf("Expected token in p2 (even path), got %d", p2.TokenCount())
	}
	if p3.TokenCount() != 0 {
		t.Errorf("Expected no tokens in p3 (odd path), got %d", p3.TokenCount())
	}

	// Clear p2
	p2.RemoveToken()

	// Test odd number
	tokOdd := &token.Token{ID: "tok_odd", Data: &intTokenData{value: 13}}
	if err := p1.AddToken(tokOdd); err != nil {
		t.Fatalf("Failed to add token: %v", err)
	}

	if enabled, _ := tEven.IsEnabled(ctx); enabled {
		t.Error("tEven should not be enabled for odd number")
	}
	if enabled, _ := tOdd.IsEnabled(ctx); !enabled {
		t.Error("tOdd should be enabled for odd number")
	}

	if err := tOdd.Fire(ctx); err != nil {
		t.Fatalf("tOdd fire failed: %v", err)
	}

	if p2.TokenCount() != 0 {
		t.Errorf("Expected no tokens in p2 (even path), got %d", p2.TokenCount())
	}
	if p3.TokenCount() != 1 {
		t.Errorf("Expected token in p3 (odd path), got %d", p3.TokenCount())
	}
}

// TestIntegration_ErrorHandlingWithCompensation tests error handling and rollback
func TestIntegration_ErrorHandlingWithCompensation(t *testing.T) {
	// Create net: p1 -> t_main -> p2 -> t_fail -> p3
	net := NewPetriNet("error-net", "Error Handling")

	p1 := NewPlace("p1", "Start", 10)
	p2 := NewPlace("p2", "Middle", 10)
	p3 := NewPlace("p3", "End", 10)

	tMain := NewTransition("t_main", "Main Task")
	tFail := NewTransition("t_fail", "Failing Task").WithTask(&task.InlineTask{
		Name: "failing-task",
		Fn: func(ctx *execctx.ExecutionContext) error {
			return errors.New("simulated failure")
		},
	})

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddPlace(p3)
	net.AddTransition(tMain)
	net.AddTransition(tFail)

	// Connect
	net.AddArc(NewArc("a1", "p1", "t_main", 1))
	net.AddArc(NewArc("a2", "t_main", "p2", 1))
	net.AddArc(NewArc("a3", "p2", "t_fail", 1))
	net.AddArc(NewArc("a4", "t_fail", "p3", 1))

	if err := net.Resolve(); err != nil {
		t.Fatalf("Failed to resolve net: %v", err)
	}

	// Add initial token
	tok := &token.Token{ID: "tok1", Data: &MapToken{Value: make(map[string]interface{})}}
	if err := p1.AddToken(tok); err != nil {
		t.Fatalf("Failed to add token: %v", err)
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Execute main task (should succeed)
	if err := tMain.Fire(ctx); err != nil {
		t.Fatalf("tMain fire failed: %v", err)
	}

	if p2.TokenCount() != 1 {
		t.Errorf("Expected token in p2, got %d", p2.TokenCount())
	}

	// Execute failing task (should rollback)
	err := tFail.Fire(ctx)
	if err == nil {
		t.Fatal("Expected error from failing task")
	}

	// Verify rollback - token should still be in p2
	if p2.TokenCount() != 1 {
		t.Errorf("Expected token in p2 after rollback, got %d", p2.TokenCount())
	}
	if p3.TokenCount() != 0 {
		t.Errorf("Expected no tokens in p3 after rollback, got %d", p3.TokenCount())
	}
}

// TestIntegration_TokenTransformation tests token transformation through arc expressions
func TestIntegration_TokenTransformation(t *testing.T) {
	// Create a pipeline that transforms data: p1 -> t1 -> p2 -> t2 -> p3
	net := NewPetriNet("transform-net", "Token Transformation")

	p1 := NewPlace("p1", "Input", 10)
	p2 := NewPlace("p2", "Middle", 10)
	p3 := NewPlace("p3", "Output", 10)

	t1 := NewTransition("t1", "Transform 1")
	t2 := NewTransition("t2", "Transform 2")

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddPlace(p3)
	net.AddTransition(t1)
	net.AddTransition(t2)

	// Connect with transformations
	net.AddArc(NewArc("a1", "p1", "t1", 1))
	net.AddArc(NewArc("a2", "t1", "p2", 1).WithExpression(func(tok *token.Token) *token.Token {
		// First transformation: multiply by 2
		intData, ok := tok.Data.(*intTokenData)
		if !ok {
			return tok
		}
		newTok := &token.Token{ID: tok.ID + "-x2", Data: &intTokenData{value: intData.value * 2}}
		return newTok
	}))
	net.AddArc(NewArc("a3", "p2", "t2", 1))
	net.AddArc(NewArc("a4", "t2", "p3", 1).WithExpression(func(tok *token.Token) *token.Token {
		// Second transformation: add 10
		intData, ok := tok.Data.(*intTokenData)
		if !ok {
			return tok
		}
		newTok := &token.Token{ID: tok.ID + "+10", Data: &intTokenData{value: intData.value + 10}}
		return newTok
	}))

	if err := net.Resolve(); err != nil {
		t.Fatalf("Failed to resolve net: %v", err)
	}

	// Add initial token with value 5
	tok := &token.Token{ID: "tok1", Data: &intTokenData{value: 5}}
	if err := p1.AddToken(tok); err != nil {
		t.Fatalf("Failed to add token: %v", err)
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Execute workflow
	if err := t1.Fire(ctx); err != nil {
		t.Fatalf("t1 fire failed: %v", err)
	}

	if err := t2.Fire(ctx); err != nil {
		t.Fatalf("t2 fire failed: %v", err)
	}

	// Verify transformation: 5 * 2 + 10 = 20
	resultTok, err := p3.RemoveToken()
	if err != nil {
		t.Fatalf("Failed to get result token: %v", err)
	}

	resultData, ok := resultTok.Data.(*intTokenData)
	if !ok {
		t.Fatalf("Result token data is not intTokenData: %T", resultTok.Data)
	}

	if resultData.value != 20 {
		t.Errorf("Expected result 20, got %d", resultData.value)
	}
}

// TestIntegration_ComplexWorkflowWithComposite tests integration with composite tasks
func TestIntegration_ComplexWorkflowWithComposite(t *testing.T) {
	// Create a workflow with composite tasks (sequence of inline tasks)
	net := NewPetriNet("composite-net", "Composite Task Workflow")

	p1 := NewPlace("p1", "Start", 10)
	p2 := NewPlace("p2", "End", 10)

	var executionLog []string

	// Create a sequence of tasks
	compositeTask := &task.SequentialTask{
		Name: "composite-sequence",
		Tasks: []task.Task{
			&task.InlineTask{
				Name: "step1",
				Fn: func(ctx *execctx.ExecutionContext) error {
					executionLog = append(executionLog, "step1")
					return nil
				},
			},
			&task.InlineTask{
				Name: "step2",
				Fn: func(ctx *execctx.ExecutionContext) error {
					executionLog = append(executionLog, "step2")
					return nil
				},
			},
			&task.InlineTask{
				Name: "step3",
				Fn: func(ctx *execctx.ExecutionContext) error {
					executionLog = append(executionLog, "step3")
					return nil
				},
			},
		},
	}

	t1 := NewTransition("t1", "Composite").WithTask(compositeTask)

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddTransition(t1)

	net.AddArc(NewArc("a1", "p1", "t1", 1))
	net.AddArc(NewArc("a2", "t1", "p2", 1))

	if err := net.Resolve(); err != nil {
		t.Fatalf("Failed to resolve net: %v", err)
	}

	// Add initial token
	tok := &token.Token{ID: "tok1", Data: &MapToken{Value: make(map[string]interface{})}}
	if err := p1.AddToken(tok); err != nil {
		t.Fatalf("Failed to add token: %v", err)
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Execute workflow
	if err := t1.Fire(ctx); err != nil {
		t.Fatalf("t1 fire failed: %v", err)
	}

	// Verify all steps executed in order
	expectedLog := []string{"step1", "step2", "step3"}
	if len(executionLog) != len(expectedLog) {
		t.Fatalf("Expected %d steps, got %d", len(expectedLog), len(executionLog))
	}

	for i, step := range expectedLog {
		if executionLog[i] != step {
			t.Errorf("Step %d: expected %s, got %s", i, step, executionLog[i])
		}
	}

	// Verify token moved
	if p1.TokenCount() != 0 {
		t.Errorf("Expected p1 empty, got %d tokens", p1.TokenCount())
	}
	if p2.TokenCount() != 1 {
		t.Errorf("Expected p2 to have 1 token, got %d", p2.TokenCount())
	}
}
