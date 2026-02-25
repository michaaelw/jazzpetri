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

package verification

import (
	"fmt"
	"testing"

	"github.com/jazzpetri/engine/petri"
	"github.com/jazzpetri/engine/token"
)

// makeSequentialNet creates a simple sequential workflow: p1 -> t1 -> p2
// p1 starts with one token; p2 is terminal.
// Reachable states: {p1:1, p2:0}, {p1:0, p2:1}
func makeSequentialNet() *petri.PetriNet {
	net := petri.NewPetriNet("seq", "Sequential")

	p1 := petri.NewPlace("p1", "Start", 10)
	p2 := petri.NewPlace("p2", "End", 10)
	p2.IsTerminal = true

	t1 := petri.NewTransition("t1", "Process")

	_ = net.AddPlace(p1)
	_ = net.AddPlace(p2)
	_ = net.AddTransition(t1)

	_ = net.AddArc(petri.NewArc("a1", "p1", "t1", 1))
	_ = net.AddArc(petri.NewArc("a2", "t1", "p2", 1))

	_ = net.Resolve()

	_ = p1.AddToken(&token.Token{ID: "tok1", Data: &token.StringToken{Value: "test"}})

	return net
}

// makeForkJoinNet creates a fork-join workflow:
//
//	p1 -> fork -> p2, p3 -> join -> p4
//
// p1 starts with one token; p4 is terminal.
// Reachable states: {p1:1,p2:0,p3:0,p4:0}, {p1:0,p2:1,p3:1,p4:0}, {p1:0,p2:0,p3:0,p4:1}
func makeForkJoinNet() *petri.PetriNet {
	net := petri.NewPetriNet("forkjoin", "Fork-Join")

	p1 := petri.NewPlace("p1", "Start", 10)
	p2 := petri.NewPlace("p2", "Branch A", 10)
	p3 := petri.NewPlace("p3", "Branch B", 10)
	p4 := petri.NewPlace("p4", "End", 10)
	p4.IsTerminal = true

	tFork := petri.NewTransition("fork", "Fork")
	tJoin := petri.NewTransition("join", "Join")

	_ = net.AddPlace(p1)
	_ = net.AddPlace(p2)
	_ = net.AddPlace(p3)
	_ = net.AddPlace(p4)
	_ = net.AddTransition(tFork)
	_ = net.AddTransition(tJoin)

	_ = net.AddArc(petri.NewArc("a1", "p1", "fork", 1))
	_ = net.AddArc(petri.NewArc("a2", "fork", "p2", 1))
	_ = net.AddArc(petri.NewArc("a3", "fork", "p3", 1))
	_ = net.AddArc(petri.NewArc("a4", "p2", "join", 1))
	_ = net.AddArc(petri.NewArc("a5", "p3", "join", 1))
	_ = net.AddArc(petri.NewArc("a6", "join", "p4", 1))

	_ = net.Resolve()

	_ = p1.AddToken(&token.Token{ID: "tok1", Data: &token.StringToken{Value: "test"}})

	return net
}

// makeDeadlockNet creates a workflow with a deadlock:
//
//	p1 -> t1 -> p2 (no transition out of p2, p2 is not terminal)
func makeDeadlockNet() *petri.PetriNet {
	net := petri.NewPetriNet("deadlock", "Deadlock")

	p1 := petri.NewPlace("p1", "Start", 10)
	p2 := petri.NewPlace("p2", "Middle", 10) // NOT terminal -- creates deadlock

	t1 := petri.NewTransition("t1", "Step1")

	_ = net.AddPlace(p1)
	_ = net.AddPlace(p2)
	_ = net.AddTransition(t1)

	_ = net.AddArc(petri.NewArc("a1", "p1", "t1", 1))
	_ = net.AddArc(petri.NewArc("a2", "t1", "p2", 1))

	_ = net.Resolve()

	_ = p1.AddToken(&token.Token{ID: "tok1", Data: &token.StringToken{Value: "test"}})

	return net
}

// TestBuildStateSpace_Sequential verifies state space for a 2-state sequential net.
func TestBuildStateSpace_Sequential(t *testing.T) {
	net := makeSequentialNet()
	v := NewVerifier(net, 100)

	ss, err := v.BuildStateSpace()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Sequential: p1(1) -> fire t1 -> p2(1). Should produce exactly 2 states.
	if len(ss.States) != 2 {
		t.Errorf("expected 2 states, got %d", len(ss.States))
	}

	// Initial state should have token in p1
	initialState := ss.States[ss.Initial]
	if initialState.Marking["p1"] != 1 {
		t.Errorf("initial state should have p1=1, got p1=%d", initialState.Marking["p1"])
	}
	if initialState.Marking["p2"] != 0 {
		t.Errorf("initial state should have p2=0, got p2=%d", initialState.Marking["p2"])
	}
}

// TestBuildStateSpace_ForkJoin verifies state space for a fork-join net.
func TestBuildStateSpace_ForkJoin(t *testing.T) {
	net := makeForkJoinNet()
	v := NewVerifier(net, 100)

	ss, err := v.BuildStateSpace()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Fork-join has 3 states:
	// State 0: p1=1, p2=0, p3=0, p4=0 (initial)
	// State 1: p1=0, p2=1, p3=1, p4=0 (after fork)
	// State 2: p1=0, p2=0, p3=0, p4=1 (after join)
	if len(ss.States) != 3 {
		t.Errorf("expected 3 states, got %d", len(ss.States))
	}
}

// TestBuildStateSpace_EmptyNet verifies that an empty net (no tokens) has one state.
func TestBuildStateSpace_EmptyNet(t *testing.T) {
	net := petri.NewPetriNet("empty", "Empty")

	p1 := petri.NewPlace("p1", "Start", 10)
	t1 := petri.NewTransition("t1", "Step")
	p2 := petri.NewPlace("p2", "End", 10)
	p2.IsTerminal = true

	_ = net.AddPlace(p1)
	_ = net.AddPlace(p2)
	_ = net.AddTransition(t1)
	_ = net.AddArc(petri.NewArc("a1", "p1", "t1", 1))
	_ = net.AddArc(petri.NewArc("a2", "t1", "p2", 1))
	_ = net.Resolve()

	// No tokens added -- initial marking is all zeros
	v := NewVerifier(net, 100)
	ss, err := v.BuildStateSpace()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only one state: the empty initial marking
	if len(ss.States) != 1 {
		t.Errorf("expected 1 state for empty net, got %d", len(ss.States))
	}
}

// TestCheckBoundedness_Sequential verifies sequential net is bounded.
func TestCheckBoundedness_Sequential(t *testing.T) {
	net := makeSequentialNet()
	v := NewVerifier(net, 100)

	ss, _ := v.BuildStateSpace()
	result := v.CheckBoundedness(ss)

	if !result.Satisfied {
		t.Error("sequential net should be bounded")
	}
	if result.Property != "boundedness" {
		t.Errorf("expected property name 'boundedness', got '%s'", result.Property)
	}
	if result.StatesChecked != len(ss.States) {
		t.Errorf("StatesChecked should be %d, got %d", len(ss.States), result.StatesChecked)
	}
}

// TestCheckDeadlockFreedom_NoDeadlock verifies sequential net is deadlock-free.
func TestCheckDeadlockFreedom_NoDeadlock(t *testing.T) {
	net := makeSequentialNet()
	v := NewVerifier(net, 100)

	ss, _ := v.BuildStateSpace()
	result := v.CheckDeadlockFreedom(ss)

	if !result.Satisfied {
		t.Errorf("sequential net should be deadlock-free: %s", result.Message)
	}
}

// TestCheckDeadlockFreedom_WithDeadlock verifies deadlock detection.
func TestCheckDeadlockFreedom_WithDeadlock(t *testing.T) {
	net := makeDeadlockNet()
	v := NewVerifier(net, 100)

	ss, _ := v.BuildStateSpace()
	result := v.CheckDeadlockFreedom(ss)

	if result.Satisfied {
		t.Error("deadlock net should have deadlock detected")
	}
	if len(result.Witness) == 0 {
		t.Error("should provide witness path to deadlock")
	}
}

// TestCheckDeadlockFreedom_ForkJoin verifies fork-join net is deadlock-free.
func TestCheckDeadlockFreedom_ForkJoin(t *testing.T) {
	net := makeForkJoinNet()
	v := NewVerifier(net, 100)

	ss, _ := v.BuildStateSpace()
	result := v.CheckDeadlockFreedom(ss)

	if !result.Satisfied {
		t.Errorf("fork-join net should be deadlock-free: %s", result.Message)
	}
}

// TestCheckReachability_Reachable verifies that the end place is reachable.
func TestCheckReachability_Reachable(t *testing.T) {
	net := makeSequentialNet()
	v := NewVerifier(net, 100)

	ss, _ := v.BuildStateSpace()

	// p2 should be reachable (the terminal place)
	result := v.CheckReachability(ss, Marking{"p2": 1})
	if !result.Satisfied {
		t.Error("p2 should be reachable")
	}
	if result.Witness == nil {
		t.Error("should have witness path to p2")
	}
}

// TestCheckReachability_NotReachable verifies unreachable markings are detected.
func TestCheckReachability_NotReachable(t *testing.T) {
	net := makeSequentialNet()
	v := NewVerifier(net, 100)

	ss, _ := v.BuildStateSpace()

	// p1 with 2 tokens should NOT be reachable (only starts with 1)
	result := v.CheckReachability(ss, Marking{"p1": 2})
	if result.Satisfied {
		t.Error("p1 with 2 tokens should not be reachable")
	}
}

// TestCheckMutualExclusion_Sequential verifies p1 and p2 are mutually exclusive.
func TestCheckMutualExclusion_Sequential(t *testing.T) {
	net := makeSequentialNet()
	v := NewVerifier(net, 100)

	ss, _ := v.BuildStateSpace()

	// In sequential net, token is either in p1 or p2, never both
	result := v.CheckMutualExclusion(ss, "p1", "p2")
	if !result.Satisfied {
		t.Error("p1 and p2 should be mutually exclusive in sequential net")
	}
}

// TestCheckMutualExclusion_Violated verifies violated mutual exclusion is detected.
func TestCheckMutualExclusion_Violated(t *testing.T) {
	net := makeForkJoinNet()
	v := NewVerifier(net, 100)

	ss, _ := v.BuildStateSpace()

	// After fork, p2 and p3 both have tokens -- mutual exclusion violated
	result := v.CheckMutualExclusion(ss, "p2", "p3")
	if result.Satisfied {
		t.Error("p2 and p3 should NOT be mutually exclusive after fork")
	}
	if len(result.Witness) == 0 {
		t.Error("should provide witness path to violation")
	}
}

// TestCheckPlaceInvariant_TokenConservation verifies token conservation invariant.
func TestCheckPlaceInvariant_TokenConservation(t *testing.T) {
	net := makeSequentialNet()
	v := NewVerifier(net, 100)

	ss, _ := v.BuildStateSpace()

	// Token conservation: p1 + p2 = 1 always
	// This means sum of tokens in p1 and p2 is always <= 1
	result := v.CheckPlaceInvariant(ss, map[string]int{"p1": 1, "p2": 1}, 1)
	if !result.Satisfied {
		t.Errorf("token conservation invariant should hold: %s", result.Message)
	}
}

// TestCheckPlaceInvariant_Violated verifies invariant violations are detected.
func TestCheckPlaceInvariant_Violated(t *testing.T) {
	net := makeForkJoinNet()
	v := NewVerifier(net, 100)

	ss, _ := v.BuildStateSpace()

	// After fork: p2=1 and p3=1, so p2+p3=2 > 1
	// This invariant will be violated in the fork state
	result := v.CheckPlaceInvariant(ss, map[string]int{"p2": 1, "p3": 1}, 1)
	if result.Satisfied {
		t.Error("invariant p2+p3<=1 should be violated in fork-join net")
	}
	if len(result.Witness) == 0 {
		t.Error("should provide witness path to invariant violation")
	}
}

// TestGenerateCertificate_Sequential verifies certificate generation for sequential net.
func TestGenerateCertificate_Sequential(t *testing.T) {
	net := makeSequentialNet()
	v := NewVerifier(net, 100)

	cert, err := v.GenerateCertificate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cert.WorkflowID != "seq" {
		t.Errorf("expected workflow ID 'seq', got '%s'", cert.WorkflowID)
	}
	if !cert.AllSatisfied() {
		for _, r := range cert.Properties {
			if !r.Satisfied {
				t.Errorf("property %s failed: %s", r.Property, r.Message)
			}
		}
	}
	if !cert.Bounded {
		t.Error("sequential net should be bounded")
	}
	if !cert.DeadlockFree {
		t.Error("sequential net should be deadlock-free")
	}
	if cert.StateCount != 2 {
		t.Errorf("expected 2 states, got %d", cert.StateCount)
	}
	if cert.EdgeCount == 0 {
		t.Error("expected non-zero edge count")
	}
}

// TestGenerateCertificate_Deadlock verifies that deadlock nets are detected by certificate.
func TestGenerateCertificate_Deadlock(t *testing.T) {
	net := makeDeadlockNet()
	v := NewVerifier(net, 100)

	cert, _ := v.GenerateCertificate()

	if cert.DeadlockFree {
		t.Error("deadlock net should not be deadlock-free")
	}
	if cert.AllSatisfied() {
		t.Error("deadlock net should not have all properties satisfied")
	}
}

// TestVerifyProperties_CustomProperties tests the custom property suite.
func TestVerifyProperties_CustomProperties(t *testing.T) {
	net := makeSequentialNet()
	v := NewVerifier(net, 100)

	properties := []SafetyProperty{
		NewDeadlockFreedomProperty("no_deadlocks"),
		NewReachabilityProperty("end_reachable", "p2"),
		NewMutualExclusionProperty("mutex_start_end", "p1", "p2"),
		NewBoundednessProperty("start_bounded", "p1", 1),
	}

	cert, err := v.VerifyProperties(properties)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cert.AllSatisfied() {
		for _, r := range cert.Properties {
			if !r.Satisfied {
				t.Errorf("property %s failed: %s", r.Property, r.Message)
			}
		}
	}
}

// TestVerifyProperties_WithViolations tests that violations are correctly reported.
func TestVerifyProperties_WithViolations(t *testing.T) {
	net := makeDeadlockNet()
	v := NewVerifier(net, 100)

	properties := []SafetyProperty{
		NewDeadlockFreedomProperty("no_deadlocks"),
	}

	cert, err := v.VerifyProperties(properties)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cert.AllSatisfied() {
		t.Error("deadlock net properties should not all be satisfied")
	}
}

// TestStateSpaceLimit verifies that state space limit is enforced.
func TestStateSpaceLimit(t *testing.T) {
	// Create a cycling net: p1 -> t1 -> p2, p2 -> t2 -> p1
	// With many tokens this creates a very large state space
	net := petri.NewPetriNet("big", "Big Net")

	p1 := petri.NewPlace("p1", "A", 100)
	p2 := petri.NewPlace("p2", "B", 100)

	t1 := petri.NewTransition("t1", "A->B")
	t2 := petri.NewTransition("t2", "B->A")

	_ = net.AddPlace(p1)
	_ = net.AddPlace(p2)
	_ = net.AddTransition(t1)
	_ = net.AddTransition(t2)

	_ = net.AddArc(petri.NewArc("a1", "p1", "t1", 1))
	_ = net.AddArc(petri.NewArc("a2", "t1", "p2", 1))
	_ = net.AddArc(petri.NewArc("a3", "p2", "t2", 1))
	_ = net.AddArc(petri.NewArc("a4", "t2", "p1", 1))

	_ = net.Resolve()

	// Add many tokens to create large state space
	for i := 0; i < 20; i++ {
		_ = p1.AddToken(&token.Token{
			ID:   fmt.Sprintf("tok%d", i),
			Data: &token.StringToken{Value: "test"},
		})
	}

	v := NewVerifier(net, 10) // Very low limit
	_, err := v.BuildStateSpace()
	if err == nil {
		t.Error("expected state space limit error")
	}
}

// TestStateSpaceLimit_Default verifies default limit is applied when 0 is passed.
func TestStateSpaceLimit_Default(t *testing.T) {
	net := makeSequentialNet()
	v := NewVerifier(net, 0) // 0 should use default (10000)

	if v.maxStates != 10000 {
		t.Errorf("expected default maxStates of 10000, got %d", v.maxStates)
	}

	ss, err := v.BuildStateSpace()
	if err != nil {
		t.Fatalf("unexpected error with default limit: %v", err)
	}
	if len(ss.States) != 2 {
		t.Errorf("expected 2 states, got %d", len(ss.States))
	}
}

// TestMarking_Key verifies canonical key generation.
func TestMarking_Key(t *testing.T) {
	// Same marking expressed in different insertion orders should produce same key
	m1 := Marking{"p1": 1, "p2": 0}
	m2 := Marking{"p2": 0, "p1": 1}

	if m1.Key() != m2.Key() {
		t.Errorf("same markings with different key order should produce same key: %q vs %q",
			m1.Key(), m2.Key())
	}
}

// TestMarking_Key_Empty verifies that an empty marking produces a consistent key.
func TestMarking_Key_Empty(t *testing.T) {
	m := Marking{}
	key := m.Key()
	// Empty marking should produce empty string
	if key != "" {
		t.Errorf("empty marking key should be '', got %q", key)
	}
}

// TestMarking_Copy verifies that copies are independent.
func TestMarking_Copy(t *testing.T) {
	m1 := Marking{"p1": 1, "p2": 2}
	m2 := m1.Copy()

	// Modify copy -- original should be unaffected
	m2["p1"] = 99
	if m1["p1"] == 99 {
		t.Error("copy should be independent of original")
	}

	// Modify original -- copy should be unaffected
	m1["p2"] = 77
	if m2["p2"] == 77 {
		t.Error("original should not affect copy")
	}
}

// TestFindPath_DirectPath verifies BFS path finding for adjacent states.
func TestFindPath_DirectPath(t *testing.T) {
	net := makeSequentialNet()
	v := NewVerifier(net, 100)

	ss, _ := v.BuildStateSpace()

	// There should be a path from state 0 to state 1
	path := v.findPath(ss, 0, 1)
	if len(path) == 0 {
		t.Error("should find path from state 0 to state 1")
	}
	if path[0] != "t1" {
		t.Errorf("path should start with 't1', got '%s'", path[0])
	}
}

// TestFindPath_SameState verifies that path from state to itself is nil.
func TestFindPath_SameState(t *testing.T) {
	net := makeSequentialNet()
	v := NewVerifier(net, 100)

	ss, _ := v.BuildStateSpace()

	path := v.findPath(ss, 0, 0)
	if path != nil {
		t.Error("path from state to itself should be nil")
	}
}

// TestFindPath_NoPath verifies that nil is returned when no path exists.
func TestFindPath_NoPath(t *testing.T) {
	net := makeSequentialNet()
	v := NewVerifier(net, 100)

	ss, _ := v.BuildStateSpace()

	// State 99 doesn't exist -- should return nil
	path := v.findPath(ss, 0, 99)
	if path != nil {
		t.Error("path to non-existent state should be nil")
	}
}

// TestProofCertificate_AllSatisfied verifies AllSatisfied logic.
func TestProofCertificate_AllSatisfied(t *testing.T) {
	cert := &ProofCertificate{
		Properties: []VerificationResult{
			{Property: "a", Satisfied: true},
			{Property: "b", Satisfied: true},
		},
	}
	if !cert.AllSatisfied() {
		t.Error("all satisfied should return true")
	}

	cert.Properties = append(cert.Properties, VerificationResult{
		Property:  "c",
		Satisfied: false,
	})
	if cert.AllSatisfied() {
		t.Error("should not be all satisfied with one failure")
	}
}

// TestProofCertificate_Empty verifies AllSatisfied returns true for empty properties.
func TestProofCertificate_Empty(t *testing.T) {
	cert := &ProofCertificate{}
	if !cert.AllSatisfied() {
		t.Error("empty certificate should have all properties satisfied (vacuously true)")
	}
}

// TestIsTerminalState verifies terminal state detection.
func TestIsTerminalState(t *testing.T) {
	net := makeSequentialNet()
	v := NewVerifier(net, 100)

	// All tokens in terminal place p2 -> terminal state
	if !v.isTerminalState(Marking{"p1": 0, "p2": 1}) {
		t.Error("state with all tokens in terminal place should be terminal")
	}

	// Token in non-terminal place p1 -> not terminal
	if v.isTerminalState(Marking{"p1": 1, "p2": 0}) {
		t.Error("state with token in non-terminal place should not be terminal")
	}

	// No tokens -> terminal (empty workflow)
	if !v.isTerminalState(Marking{"p1": 0, "p2": 0}) {
		t.Error("empty marking should be terminal")
	}
}

// TestWeightedArcs verifies that arc weights are correctly handled.
func TestWeightedArcs(t *testing.T) {
	// Net: p1 (2 tokens) -> t1 (weight 2) -> p2
	net := petri.NewPetriNet("weighted", "Weighted")

	p1 := petri.NewPlace("p1", "Start", 10)
	p2 := petri.NewPlace("p2", "End", 10)
	p2.IsTerminal = true

	t1 := petri.NewTransition("t1", "Process")

	_ = net.AddPlace(p1)
	_ = net.AddPlace(p2)
	_ = net.AddTransition(t1)

	// Arc with weight 2 -- requires 2 tokens to fire
	_ = net.AddArc(petri.NewArc("a1", "p1", "t1", 2))
	_ = net.AddArc(petri.NewArc("a2", "t1", "p2", 1))

	_ = net.Resolve()

	// Add 2 tokens to p1
	_ = p1.AddToken(&token.Token{ID: "tok1", Data: &token.StringToken{Value: "a"}})
	_ = p1.AddToken(&token.Token{ID: "tok2", Data: &token.StringToken{Value: "b"}})

	v := NewVerifier(net, 100)
	ss, err := v.BuildStateSpace()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// State 0: p1=2, p2=0 (initial -- t1 enabled)
	// State 1: p1=0, p2=1 (after firing t1 with weight 2)
	if len(ss.States) != 2 {
		t.Errorf("expected 2 states for weighted arc net, got %d", len(ss.States))
	}
}

// TestInsufficientTokens verifies that transitions are disabled when tokens are insufficient.
func TestInsufficientTokens(t *testing.T) {
	// Net: p1 (1 token) -> t1 (weight 2) -> p2
	// t1 requires 2 tokens but only 1 is available -- deadlock
	net := petri.NewPetriNet("insufficient", "Insufficient")

	p1 := petri.NewPlace("p1", "Start", 10)
	p2 := petri.NewPlace("p2", "End", 10)
	p2.IsTerminal = true

	t1 := petri.NewTransition("t1", "Process")

	_ = net.AddPlace(p1)
	_ = net.AddPlace(p2)
	_ = net.AddTransition(t1)

	// Arc requires 2 tokens
	_ = net.AddArc(petri.NewArc("a1", "p1", "t1", 2))
	_ = net.AddArc(petri.NewArc("a2", "t1", "p2", 1))

	_ = net.Resolve()

	// Add only 1 token -- insufficient for t1 to fire
	_ = p1.AddToken(&token.Token{ID: "tok1", Data: &token.StringToken{Value: "test"}})

	v := NewVerifier(net, 100)
	ss, _ := v.BuildStateSpace()

	// Only 1 state -- t1 never fires
	if len(ss.States) != 1 {
		t.Errorf("expected 1 state when tokens insufficient, got %d", len(ss.States))
	}

	// Should detect deadlock (p1 has token, p2 is not reached, t1 can't fire)
	result := v.CheckDeadlockFreedom(ss)
	if result.Satisfied {
		t.Error("insufficient tokens should cause deadlock")
	}
}

// TestNewSafetyProperties verifies the safety property constructors.
func TestNewSafetyProperties(t *testing.T) {
	t.Run("BoundednessProperty", func(t *testing.T) {
		prop := NewBoundednessProperty("test", "p1", 5)
		if prop.Name != "test" {
			t.Errorf("expected name 'test', got '%s'", prop.Name)
		}
		if prop.Check == nil {
			t.Error("check function should not be nil")
		}
	})

	t.Run("ReachabilityProperty", func(t *testing.T) {
		prop := NewReachabilityProperty("test", "p2")
		if prop.Name != "test" {
			t.Errorf("expected name 'test', got '%s'", prop.Name)
		}
		if prop.Check == nil {
			t.Error("check function should not be nil")
		}
	})

	t.Run("MutualExclusionProperty", func(t *testing.T) {
		prop := NewMutualExclusionProperty("test", "p1", "p2")
		if prop.Name != "test" {
			t.Errorf("expected name 'test', got '%s'", prop.Name)
		}
		if prop.Check == nil {
			t.Error("check function should not be nil")
		}
	})

	t.Run("DeadlockFreedomProperty", func(t *testing.T) {
		prop := NewDeadlockFreedomProperty("test")
		if prop.Name != "test" {
			t.Errorf("expected name 'test', got '%s'", prop.Name)
		}
		if prop.Check == nil {
			t.Error("check function should not be nil")
		}
	})

	t.Run("PlaceInvariantProperty", func(t *testing.T) {
		prop := NewPlaceInvariantProperty("test", map[string]int{"p1": 1}, 5)
		if prop.Name != "test" {
			t.Errorf("expected name 'test', got '%s'", prop.Name)
		}
		if prop.Check == nil {
			t.Error("check function should not be nil")
		}
	})
}

// TestVerifyProperties_DeadlockFreeField verifies DeadlockFree field is set correctly
// when using VerifyProperties with the no_deadlocks property name.
func TestVerifyProperties_DeadlockFreeField(t *testing.T) {
	net := makeDeadlockNet()
	v := NewVerifier(net, 100)

	// Use the property name that triggers DeadlockFree summary
	properties := []SafetyProperty{
		NewDeadlockFreedomProperty("deadlock_freedom"),
	}

	cert, _ := v.VerifyProperties(properties)

	if cert.DeadlockFree {
		t.Error("DeadlockFree should be false for deadlock net")
	}
}
