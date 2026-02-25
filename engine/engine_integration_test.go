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
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jazzpetri/engine/clock"
	execContext "github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/petri"
	"github.com/jazzpetri/engine/task"
	"github.com/jazzpetri/engine/token"
)

// TestCompleteWorkflow tests a complete workflow from start to finish
func TestCompleteWorkflow(t *testing.T) {
	// Create a linear workflow: p1 -> t1 -> p2 -> t2 -> p3
	net := petri.NewPetriNet("complete-workflow", "Complete Workflow")

	p1 := petri.NewPlace("p1", "Start", 10)
	p2 := petri.NewPlace("p2", "Middle", 10)
	p3 := petri.NewPlace("p3", "End", 10)

	var step1Done, step2Done int32

	t1 := petri.NewTransition("t1", "Step 1").WithTask(&task.InlineTask{
		Name: "step1",
		Fn: func(ctx *execContext.ExecutionContext) error {
			atomic.StoreInt32(&step1Done, 1)
			return nil
		},
	})

	t2 := petri.NewTransition("t2", "Step 2").WithTask(&task.InlineTask{
		Name: "step2",
		Fn: func(ctx *execContext.ExecutionContext) error {
			atomic.StoreInt32(&step2Done, 1)
			return nil
		},
	})

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddPlace(p3)
	net.AddTransition(t1)
	net.AddTransition(t2)

	net.AddArc(petri.NewArc("arc1", "p1", "t1", 1))
	net.AddArc(petri.NewArc("arc2", "t1", "p2", 1))
	net.AddArc(petri.NewArc("arc3", "p2", "t2", 1))
	net.AddArc(petri.NewArc("arc4", "t2", "p3", 1))

	if err := net.Resolve(); err != nil {
		t.Fatalf("Failed to resolve net: %v", err)
	}

	// Add initial token
	tok := &token.Token{
		ID:   "tok1",
		Data: &petri.MapToken{Value: map[string]interface{}{"workflow": "complete"}},
	}
	p1.AddToken(tok)

	ctx := execContext.NewExecutionContext(
		context.Background(),
		clock.NewVirtualClock(time.Now()),
		nil,
	)

	engine := NewEngine(net, ctx, DefaultConfig())

	if err := engine.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if err := engine.Wait(); err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	if err := engine.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Verify workflow completed
	if atomic.LoadInt32(&step1Done) != 1 {
		t.Error("Step 1 did not execute")
	}

	if atomic.LoadInt32(&step2Done) != 1 {
		t.Error("Step 2 did not execute")
	}

	if p3.TokenCount() != 1 {
		t.Errorf("Final place token count = %d, want 1", p3.TokenCount())
	}
}

// TestParallelBranchesWorkflow tests a workflow with parallel execution
func TestParallelBranchesWorkflow(t *testing.T) {
	// Create workflow with parallel branches
	// p1 -> split -> (t1 and t2) -> join -> p4
	net := petri.NewPetriNet("parallel-workflow", "Parallel Workflow")

	p1 := petri.NewPlace("p1", "Start", 10)
	p2a := petri.NewPlace("p2a", "Branch A", 10)
	p2b := petri.NewPlace("p2b", "Branch B", 10)
	p3a := petri.NewPlace("p3a", "Branch A Done", 10)
	p3b := petri.NewPlace("p3b", "Branch B Done", 10)
	p4 := petri.NewPlace("p4", "Join", 10)

	var execA, execB, execJoin int32

	// Split transition
	tSplit := petri.NewTransition("tSplit", "Split")

	// Branch A
	tA := petri.NewTransition("tA", "Process A").WithTask(&task.InlineTask{
		Name: "processA",
		Fn: func(ctx *execContext.ExecutionContext) error {
			atomic.AddInt32(&execA, 1)
			time.Sleep(20 * time.Millisecond)
			return nil
		},
	})

	// Branch B
	tB := petri.NewTransition("tB", "Process B").WithTask(&task.InlineTask{
		Name: "processB",
		Fn: func(ctx *execContext.ExecutionContext) error {
			atomic.AddInt32(&execB, 1)
			time.Sleep(20 * time.Millisecond)
			return nil
		},
	})

	// Join transition
	tJoin := petri.NewTransition("tJoin", "Join").WithTask(&task.InlineTask{
		Name: "join",
		Fn: func(ctx *execContext.ExecutionContext) error {
			atomic.AddInt32(&execJoin, 1)
			return nil
		},
	})

	net.AddPlace(p1)
	net.AddPlace(p2a)
	net.AddPlace(p2b)
	net.AddPlace(p3a)
	net.AddPlace(p3b)
	net.AddPlace(p4)
	net.AddTransition(tSplit)
	net.AddTransition(tA)
	net.AddTransition(tB)
	net.AddTransition(tJoin)

	// Split: p1 -> tSplit -> p2a and p2b
	net.AddArc(petri.NewArc("arc-split-in", "p1", "tSplit", 1))
	net.AddArc(petri.NewArc("arc-split-a", "tSplit", "p2a", 1))
	net.AddArc(petri.NewArc("arc-split-b", "tSplit", "p2b", 1))

	// Branch A: p2a -> tA -> p3a
	net.AddArc(petri.NewArc("arc-a-in", "p2a", "tA", 1))
	net.AddArc(petri.NewArc("arc-a-out", "tA", "p3a", 1))

	// Branch B: p2b -> tB -> p3b
	net.AddArc(petri.NewArc("arc-b-in", "p2b", "tB", 1))
	net.AddArc(petri.NewArc("arc-b-out", "tB", "p3b", 1))

	// Join: p3a + p3b -> tJoin -> p4
	net.AddArc(petri.NewArc("arc-join-a", "p3a", "tJoin", 1))
	net.AddArc(petri.NewArc("arc-join-b", "p3b", "tJoin", 1))
	net.AddArc(petri.NewArc("arc-join-out", "tJoin", "p4", 1))

	if err := net.Resolve(); err != nil {
		t.Fatalf("Failed to resolve net: %v", err)
	}

	// Add initial token
	tok := &token.Token{
		ID:   "tok1",
		Data: &petri.MapToken{Value: map[string]interface{}{"parallel": true}},
	}
	p1.AddToken(tok)

	ctx := execContext.NewExecutionContext(
		context.Background(),
		clock.NewVirtualClock(time.Now()),
		nil,
	)

	config := Config{
		Mode:                     ModeConcurrent,
		MaxConcurrentTransitions: 5,
		PollInterval:             10 * time.Millisecond,
	}

	engine := NewEngine(net, ctx, config)

	if err := engine.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if err := engine.Wait(); err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	if err := engine.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Verify all branches executed
	if atomic.LoadInt32(&execA) != 1 {
		t.Errorf("Branch A exec count = %d, want 1", execA)
	}

	if atomic.LoadInt32(&execB) != 1 {
		t.Errorf("Branch B exec count = %d, want 1", execB)
	}

	if atomic.LoadInt32(&execJoin) != 1 {
		t.Errorf("Join exec count = %d, want 1", execJoin)
	}

	if p4.TokenCount() != 1 {
		t.Errorf("Final place token count = %d, want 1", p4.TokenCount())
	}
}

// TestConditionalRoutingWorkflow tests workflow with guard-based routing
func TestConditionalRoutingWorkflow(t *testing.T) {
	// Create workflow with conditional routing
	// p1 -> t1 (guard: value > 10) -> p2
	// p1 -> t2 (guard: value <= 10) -> p3
	net := petri.NewPetriNet("conditional-workflow", "Conditional Workflow")

	p1 := petri.NewPlace("p1", "Input", 10)
	p2 := petri.NewPlace("p2", "High Value", 10)
	p3 := petri.NewPlace("p3", "Low Value", 10)

	// High value path
	tHigh := petri.NewTransition("tHigh", "High Value").WithGuard(func(ctx *execContext.ExecutionContext, tokens []*token.Token) (bool, error) {
		if len(tokens) == 0 {
			return false, nil
		}
		data, ok := tokens[0].Data.(*petri.MapToken)
		if !ok {
			return false, nil
		}
		val, ok := data.Value["value"].(int)
		return ok && val > 10, nil
	})

	// Low value path
	tLow := petri.NewTransition("tLow", "Low Value").WithGuard(func(ctx *execContext.ExecutionContext, tokens []*token.Token) (bool, error) {
		if len(tokens) == 0 {
			return false, nil
		}
		data, ok := tokens[0].Data.(*petri.MapToken)
		if !ok {
			return false, nil
		}
		val, ok := data.Value["value"].(int)
		return ok && val <= 10, nil
	})

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddPlace(p3)
	net.AddTransition(tHigh)
	net.AddTransition(tLow)

	net.AddArc(petri.NewArc("arc-high-in", "p1", "tHigh", 1))
	net.AddArc(petri.NewArc("arc-high-out", "tHigh", "p2", 1))
	net.AddArc(petri.NewArc("arc-low-in", "p1", "tLow", 1))
	net.AddArc(petri.NewArc("arc-low-out", "tLow", "p3", 1))

	if err := net.Resolve(); err != nil {
		t.Fatalf("Failed to resolve net: %v", err)
	}

	tests := []struct {
		name       string
		value      int
		expectHigh int
		expectLow  int
	}{
		{"high value", 20, 1, 0},
		{"low value", 5, 0, 1},
		{"boundary", 10, 0, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset places
			for p1.TokenCount() > 0 {
				p1.RemoveToken()
			}
			for p2.TokenCount() > 0 {
				p2.RemoveToken()
			}
			for p3.TokenCount() > 0 {
				p3.RemoveToken()
			}

			// Add token with test value
			tok := &token.Token{
				ID:   "tok1",
				Data: &petri.MapToken{Value: map[string]interface{}{"value": tt.value}},
			}
			p1.AddToken(tok)

			ctx := execContext.NewExecutionContext(
				context.Background(),
				clock.NewVirtualClock(time.Now()),
				nil,
			)

			engine := NewEngine(net, ctx, DefaultConfig())

			if err := engine.Start(); err != nil {
				t.Fatalf("Start failed: %v", err)
			}

			if err := engine.Wait(); err != nil {
				t.Fatalf("Wait failed: %v", err)
			}

			if err := engine.Stop(); err != nil {
				t.Fatalf("Stop failed: %v", err)
			}

			if p2.TokenCount() != tt.expectHigh {
				t.Errorf("High value place count = %d, want %d", p2.TokenCount(), tt.expectHigh)
			}

			if p3.TokenCount() != tt.expectLow {
				t.Errorf("Low value place count = %d, want %d", p3.TokenCount(), tt.expectLow)
			}
		})
	}
}

// TestErrorRecoveryWorkflow tests error handling and recovery
func TestErrorRecoveryWorkflow(t *testing.T) {
	// Create workflow with potential errors
	net := petri.NewPetriNet("error-workflow", "Error Recovery Workflow")

	p1 := petri.NewPlace("p1", "Input", 10)
	p2 := petri.NewPlace("p2", "Success", 10)
	p3 := petri.NewPlace("p3", "After Error", 10)

	var failCount int32

	// Task that fails once then succeeds
	tFailOnce := petri.NewTransition("tFailOnce", "Fail Once").WithTask(&task.InlineTask{
		Name: "fail-once",
		Fn: func(ctx *execContext.ExecutionContext) error {
			count := atomic.AddInt32(&failCount, 1)
			if count == 1 {
				return fmt.Errorf("intentional first failure")
			}
			return nil
		},
	})

	// Recovery transition
	tRecovery := petri.NewTransition("tRecovery", "Recovery")

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddPlace(p3)
	net.AddTransition(tFailOnce)
	net.AddTransition(tRecovery)

	net.AddArc(petri.NewArc("arc1", "p1", "tFailOnce", 1))
	net.AddArc(petri.NewArc("arc2", "tFailOnce", "p2", 1))
	net.AddArc(petri.NewArc("arc3", "p2", "tRecovery", 1))
	net.AddArc(petri.NewArc("arc4", "tRecovery", "p3", 1))

	if err := net.Resolve(); err != nil {
		t.Fatalf("Failed to resolve net: %v", err)
	}

	// Add token
	tok := &token.Token{
		ID:   "tok1",
		Data: &petri.MapToken{Value: map[string]interface{}{}},
	}
	p1.AddToken(tok)

	ctx := execContext.NewExecutionContext(
		context.Background(),
		clock.NewVirtualClock(time.Now()),
		nil,
	)

	config := Config{
		Mode:         ModeSequential,
		PollInterval: 10 * time.Millisecond,
	}

	engine := NewEngine(net, ctx, config)

	if err := engine.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for first attempt (which will fail) and retry
	time.Sleep(150 * time.Millisecond)

	if err := engine.Wait(); err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	if err := engine.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Verify recovery succeeded
	if atomic.LoadInt32(&failCount) < 2 {
		t.Errorf("Fail count = %d, want >= 2 (retry)", failCount)
	}

	if p3.TokenCount() != 1 {
		t.Errorf("Final place token count = %d, want 1", p3.TokenCount())
	}
}

// TestSequentialVsConcurrent demonstrates performance difference between modes
// This test is mainly for demonstration - actual performance depends on workload
func TestSequentialVsConcurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance comparison in short mode")
	}

	// Create a simple workflow with two parallel tasks
	createWorkflow := func() *petri.PetriNet {
		net := petri.NewPetriNet("perf-workflow", "Performance Workflow")

		p1 := petri.NewPlace("p1", "Start", 10)
		p2 := petri.NewPlace("p2", "Task 1 Done", 10)
		p3 := petri.NewPlace("p3", "Task 2 Done", 10)

		t1 := petri.NewTransition("t1", "Task 1").WithTask(&task.InlineTask{
			Name: "task1",
			Fn: func(ctx *execContext.ExecutionContext) error {
				time.Sleep(20 * time.Millisecond)
				return nil
			},
		})

		t2 := petri.NewTransition("t2", "Task 2").WithTask(&task.InlineTask{
			Name: "task2",
			Fn: func(ctx *execContext.ExecutionContext) error {
				time.Sleep(20 * time.Millisecond)
				return nil
			},
		})

		net.AddPlace(p1)
		net.AddPlace(p2)
		net.AddPlace(p3)
		net.AddTransition(t1)
		net.AddTransition(t2)

		net.AddArc(petri.NewArc("arc1", "p1", "t1", 1))
		net.AddArc(petri.NewArc("arc2", "t1", "p2", 1))
		net.AddArc(petri.NewArc("arc3", "p1", "t2", 1))
		net.AddArc(petri.NewArc("arc4", "t2", "p3", 1))

		// Add two tokens
		p1.AddToken(&token.Token{ID: "tok1", Data: &petri.MapToken{Value: map[string]interface{}{}}})
		p1.AddToken(&token.Token{ID: "tok2", Data: &petri.MapToken{Value: map[string]interface{}{}}})

		net.Resolve()
		return net
	}

	// Test sequential mode
	t.Run("sequential", func(t *testing.T) {
		net := createWorkflow()
		ctx := execContext.NewExecutionContext(
			context.Background(),
			clock.NewVirtualClock(time.Now()),
			nil,
		)

		config := Config{
			Mode:         ModeSequential,
			PollInterval: 5 * time.Millisecond,
		}

		engine := NewEngine(net, ctx, config)
		start := time.Now()

		engine.Start()
		engine.Wait()
		engine.Stop()

		elapsed := time.Since(start)
		t.Logf("Sequential mode: %v (tasks run one at a time)", elapsed)
	})

	// Test concurrent mode
	t.Run("concurrent", func(t *testing.T) {
		net := createWorkflow()
		ctx := execContext.NewExecutionContext(
			context.Background(),
			clock.NewVirtualClock(time.Now()),
			nil,
		)

		config := Config{
			Mode:                     ModeConcurrent,
			MaxConcurrentTransitions: 5,
			PollInterval:             5 * time.Millisecond,
		}

		engine := NewEngine(net, ctx, config)
		start := time.Now()

		engine.Start()
		engine.Wait()
		engine.Stop()

		elapsed := time.Since(start)
		t.Logf("Concurrent mode: %v (tasks run in parallel)", elapsed)
	})
}
