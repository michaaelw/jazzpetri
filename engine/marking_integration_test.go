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
	"testing"
	"time"

	"github.com/jazzpetri/engine/clock"
	execctx "github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/petri"
	"github.com/jazzpetri/engine/state"
	"github.com/jazzpetri/engine/task"
	"github.com/jazzpetri/engine/token"
)

// TestMarking_BeforeAfterTransitionFiring tests capturing marking before and after transition fires
func TestMarking_BeforeAfterTransitionFiring(t *testing.T) {
	// Create a simple workflow: p1 -> t1 -> p2
	net := petri.NewPetriNet("net1", "Test Net")

	p1 := petri.NewPlace("p1", "Place 1", 10)
	p2 := petri.NewPlace("p2", "Place 2", 10)
	t1 := petri.NewTransition("t1", "Transition 1").WithTask(&task.InlineTask{
		Name: "noop",
		Fn:   func(ctx *execctx.ExecutionContext) error { return nil },
	})

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddTransition(t1)

	arc1 := petri.NewArc("a1", "p1", "t1", 1)
	arc2 := petri.NewArc("a2", "t1", "p2", 1)
	net.AddArc(arc1)
	net.AddArc(arc2)
	net.Resolve()

	// Add token to p1
	tok := &token.Token{ID: "tok1", Data: &token.StringToken{Value: "test"}}
	p1.AddToken(tok)

	// Capture marking before transition
	clk := clock.NewVirtualClock(time.Now())
	before, err := state.NewMarking(net, clk)
	if err != nil {
		t.Fatalf("Failed to capture marking before: %v", err)
	}

	// Verify initial state
	if before.PlaceTokenCount("p1") != 1 {
		t.Errorf("Expected 1 token in p1 before, got %d", before.PlaceTokenCount("p1"))
	}
	if before.PlaceTokenCount("p2") != 0 {
		t.Errorf("Expected 0 tokens in p2 before, got %d", before.PlaceTokenCount("p2"))
	}

	// Fire transition
	ctx := execctx.NewExecutionContext(context.Background(), clk, nil)
	if err := t1.Fire(ctx); err != nil {
		t.Fatalf("Failed to fire transition: %v", err)
	}

	// Capture marking after transition
	after, err := state.NewMarking(net, clk)
	if err != nil {
		t.Fatalf("Failed to capture marking after: %v", err)
	}

	// Verify final state
	if after.PlaceTokenCount("p1") != 0 {
		t.Errorf("Expected 0 tokens in p1 after, got %d", after.PlaceTokenCount("p1"))
	}
	if after.PlaceTokenCount("p2") != 1 {
		t.Errorf("Expected 1 token in p2 after, got %d", after.PlaceTokenCount("p2"))
	}

	// Verify markings are different
	if before.Equal(after) {
		t.Error("Markings before and after should be different")
	}

	// Verify original marking unchanged (immutability)
	if before.PlaceTokenCount("p1") != 1 {
		t.Error("Original marking should be unchanged")
	}
}

// TestMarking_NonDestructiveDuringExecution tests that marking capture doesn't affect execution
func TestMarking_NonDestructiveDuringExecution(t *testing.T) {
	// Create workflow
	net := petri.NewPetriNet("net1", "Test Net")

	p1 := petri.NewPlace("p1", "Place 1", 10)
	p2 := petri.NewPlace("p2", "Place 2", 10)
	t1 := petri.NewTransition("t1", "Transition 1").WithTask(&task.InlineTask{
		Name: "noop",
		Fn:   func(ctx *execctx.ExecutionContext) error { return nil },
	})

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddTransition(t1)

	arc1 := petri.NewArc("a1", "p1", "t1", 1)
	arc2 := petri.NewArc("a2", "t1", "p2", 1)
	net.AddArc(arc1)
	net.AddArc(arc2)
	net.Resolve()

	// Add tokens
	tok1 := &token.Token{ID: "tok1", Data: &token.StringToken{Value: "test1"}}
	tok2 := &token.Token{ID: "tok2", Data: &token.StringToken{Value: "test2"}}
	p1.AddToken(tok1)
	p1.AddToken(tok2)

	clk := clock.NewVirtualClock(time.Now())

	// Capture marking multiple times
	m1, _ := state.NewMarking(net, clk)
	m2, _ := state.NewMarking(net, clk)
	m3, _ := state.NewMarking(net, clk)

	// All markings should be equal (same state)
	if !m1.Equal(m2) || !m2.Equal(m3) {
		t.Error("Multiple captures should produce equal markings")
	}

	// Verify tokens still available for execution
	ctx := execctx.NewExecutionContext(context.Background(), clk, nil)
	if err := t1.Fire(ctx); err != nil {
		t.Errorf("Should be able to fire transition after marking capture: %v", err)
	}

	// Verify state changed
	m4, _ := state.NewMarking(net, clk)
	if m3.Equal(m4) {
		t.Error("Marking after execution should differ from before")
	}
}

// TestMarking_EngineIntegration tests Engine.GetMarking()
func TestMarking_EngineIntegration(t *testing.T) {
	// Create workflow
	net := petri.NewPetriNet("net1", "Test Net")

	p1 := petri.NewPlace("p1", "Place 1", 10)
	p2 := petri.NewPlace("p2", "Place 2", 10)
	t1 := petri.NewTransition("t1", "Transition 1").WithTask(&task.InlineTask{
		Name: "noop",
		Fn:   func(ctx *execctx.ExecutionContext) error { return nil },
	})

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddTransition(t1)

	arc1 := petri.NewArc("a1", "p1", "t1", 1)
	arc2 := petri.NewArc("a2", "t1", "p2", 1)
	net.AddArc(arc1)
	net.AddArc(arc2)
	net.Resolve()

	// Add token
	tok := &token.Token{ID: "tok1", Data: &token.StringToken{Value: "test"}}
	p1.AddToken(tok)

	// Create engine
	clk := clock.NewVirtualClock(time.Now())
	ctx := execctx.NewExecutionContext(context.Background(), clk, nil)
	eng := NewEngine(net, ctx, Config{
		Mode:         ModeSequential,
		PollInterval: 50 * time.Millisecond,
		StepMode:     true, // Manual stepping
	})

	// Get marking before execution
	before, err := eng.GetMarking()
	if err != nil {
		t.Fatalf("Failed to get marking: %v", err)
	}

	if before.PlaceTokenCount("p1") != 1 {
		t.Errorf("Expected 1 token in p1, got %d", before.PlaceTokenCount("p1"))
	}

	// Execute one step
	if err := eng.Step(); err != nil {
		t.Fatalf("Failed to step: %v", err)
	}

	// Get marking after execution
	after, err := eng.GetMarking()
	if err != nil {
		t.Fatalf("Failed to get marking after: %v", err)
	}

	if after.PlaceTokenCount("p2") != 1 {
		t.Errorf("Expected 1 token in p2 after step, got %d", after.PlaceTokenCount("p2"))
	}

	// Verify markings differ
	if before.Equal(after) {
		t.Error("Markings should differ after execution")
	}
}

// TestMarking_ConcurrentCapture tests concurrent marking captures
func TestMarking_ConcurrentCapture(t *testing.T) {
	// Create workflow with multiple places
	net := petri.NewPetriNet("net1", "Test Net")

	for i := 0; i < 5; i++ {
		p := petri.NewPlace(string(rune('a'+i)), "Place", 10)
		net.AddPlace(p)

		// Add tokens to each place
		for j := 0; j < 3; j++ {
			tok := &token.Token{
				ID:   string(rune('a'+i)) + string(rune('0'+j)),
				Data: &token.StringToken{Value: "test"},
			}
			p.AddToken(tok)
		}
	}
	net.Resolve()

	clk := clock.NewVirtualClock(time.Now())

	// Capture markings concurrently
	done := make(chan *state.Marking, 10)
	for i := 0; i < 10; i++ {
		go func() {
			m, err := state.NewMarking(net, clk)
			if err != nil {
				t.Errorf("Concurrent capture failed: %v", err)
			}
			done <- m
		}()
	}

	// Collect all markings
	markings := make([]*state.Marking, 10)
	for i := 0; i < 10; i++ {
		markings[i] = <-done
	}

	// All markings should be equal (same state)
	for i := 1; i < len(markings); i++ {
		if !markings[0].Equal(markings[i]) {
			t.Errorf("Concurrent markings should be equal, marking %d differs", i)
		}
	}

	// Each marking should have correct token count
	for i, m := range markings {
		if m.TokenCount() != 15 { // 5 places * 3 tokens
			t.Errorf("Marking %d has wrong token count: %d", i, m.TokenCount())
		}
	}
}

// TestMarking_MultiStageWorkflow tests marking capture at different workflow stages
func TestMarking_MultiStageWorkflow(t *testing.T) {
	// Create a 3-stage workflow: p1 -> t1 -> p2 -> t2 -> p3
	net := petri.NewPetriNet("net1", "Multi-Stage Workflow")

	p1 := petri.NewPlace("p1", "Start", 10)
	p2 := petri.NewPlace("p2", "Middle", 10)
	p3 := petri.NewPlace("p3", "End", 10)

	t1 := petri.NewTransition("t1", "Stage 1").WithTask(&task.InlineTask{
		Name: "noop1",
		Fn:   func(ctx *execctx.ExecutionContext) error { return nil },
	})
	t2 := petri.NewTransition("t2", "Stage 2").WithTask(&task.InlineTask{
		Name: "noop2",
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

	// Stage 0: Initial state
	m0, _ := state.NewMarking(net, clk)
	if m0.PlaceTokenCount("p1") != 1 || m0.PlaceTokenCount("p2") != 0 || m0.PlaceTokenCount("p3") != 0 {
		t.Error("Stage 0 marking incorrect")
	}

	// Stage 1: After first transition
	t1.Fire(ctx)
	m1, _ := state.NewMarking(net, clk)
	if m1.PlaceTokenCount("p1") != 0 || m1.PlaceTokenCount("p2") != 1 || m1.PlaceTokenCount("p3") != 0 {
		t.Error("Stage 1 marking incorrect")
	}

	// Stage 2: After second transition
	t2.Fire(ctx)
	m2, _ := state.NewMarking(net, clk)
	if m2.PlaceTokenCount("p1") != 0 || m2.PlaceTokenCount("p2") != 0 || m2.PlaceTokenCount("p3") != 1 {
		t.Error("Stage 2 marking incorrect")
	}

	// All markings should be different
	if m0.Equal(m1) || m1.Equal(m2) || m0.Equal(m2) {
		t.Error("Markings at different stages should differ")
	}

	// Total tokens should be conserved
	if m0.TokenCount() != 1 || m1.TokenCount() != 1 || m2.TokenCount() != 1 {
		t.Error("Token count should be conserved across stages")
	}
}
