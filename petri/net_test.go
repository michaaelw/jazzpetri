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
	"github.com/jazzpetri/engine/token"
	"strings"
	"testing"

	execContext "github.com/jazzpetri/engine/context"
)

func TestNewPetriNet(t *testing.T) {
	net := NewPetriNet("net1", "TestNet")
	if net == nil {
		t.Fatal("NewPetriNet returned nil")
	}
	if net.ID != "net1" {
		t.Errorf("expected ID net1, got %s", net.ID)
	}
	if net.Name != "TestNet" {
		t.Errorf("expected Name TestNet, got %s", net.Name)
	}
	if net.Places == nil {
		t.Error("Places map should be initialized")
	}
	if net.Transitions == nil {
		t.Error("Transitions map should be initialized")
	}
	if net.Arcs == nil {
		t.Error("Arcs map should be initialized")
	}
	if net.resolved {
		t.Error("PetriNet should not be resolved initially")
	}
}

func TestPetriNet_AddPlace(t *testing.T) {
	net := NewPetriNet("net1", "TestNet")

	t.Run("add new place", func(t *testing.T) {
		p := NewPlace("p1", "Place1", 10)
		err := net.AddPlace(p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(net.Places) != 1 {
			t.Errorf("expected 1 place, got %d", len(net.Places))
		}
		if net.Places["p1"] != p {
			t.Error("place not stored correctly")
		}
	})

	t.Run("add duplicate place", func(t *testing.T) {
		p1 := NewPlace("p2", "Place2", 10)
		p2 := NewPlace("p2", "Place2Duplicate", 10)

		err := net.AddPlace(p1)
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		err = net.AddPlace(p2)
		if err == nil {
			t.Error("expected error when adding duplicate place")
		}
		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("expected 'already exists' in error, got: %v", err)
		}
	})
}

func TestPetriNet_AddTransition(t *testing.T) {
	net := NewPetriNet("net1", "TestNet")

	t.Run("add new transition", func(t *testing.T) {
		trans := NewTransition("t1", "Transition1")
		err := net.AddTransition(trans)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(net.Transitions) != 1 {
			t.Errorf("expected 1 transition, got %d", len(net.Transitions))
		}
		if net.Transitions["t1"] != trans {
			t.Error("transition not stored correctly")
		}
	})

	t.Run("add duplicate transition", func(t *testing.T) {
		t1 := NewTransition("t2", "Transition2")
		t2 := NewTransition("t2", "Transition2Duplicate")

		err := net.AddTransition(t1)
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		err = net.AddTransition(t2)
		if err == nil {
			t.Error("expected error when adding duplicate transition")
		}
		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("expected 'already exists' in error, got: %v", err)
		}
	})
}

func TestPetriNet_AddArc(t *testing.T) {
	net := NewPetriNet("net1", "TestNet")

	t.Run("add new arc", func(t *testing.T) {
		arc := NewArc("a1", "p1", "t1", 1)
		err := net.AddArc(arc)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(net.Arcs) != 1 {
			t.Errorf("expected 1 arc, got %d", len(net.Arcs))
		}
		if net.Arcs["a1"] != arc {
			t.Error("arc not stored correctly")
		}
	})

	t.Run("add duplicate arc", func(t *testing.T) {
		a1 := NewArc("a2", "p1", "t1", 1)
		a2 := NewArc("a2", "p2", "t2", 1)

		err := net.AddArc(a1)
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		err = net.AddArc(a2)
		if err == nil {
			t.Error("expected error when adding duplicate arc")
		}
		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("expected 'already exists' in error, got: %v", err)
		}
	})
}

func TestPetriNet_Validate(t *testing.T) {
	t.Run("valid net", func(t *testing.T) {
		net := NewPetriNet("net1", "TestNet")
		p1 := NewPlace("p1", "Place1", 10)
		trans := NewTransition("t1", "Transition1")
		arc := NewArc("a1", "p1", "t1", 1)

		net.AddPlace(p1)
		net.AddTransition(trans)
		net.AddArc(arc)

		err := net.Validate()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("arc with nonexistent source", func(t *testing.T) {
		net := NewPetriNet("net1", "TestNet")
		trans := NewTransition("t1", "Transition1")
		arc := NewArc("a1", "nonexistent", "t1", 1)

		net.AddTransition(trans)
		net.AddArc(arc)

		err := net.Validate()
		if err == nil {
			t.Error("expected error for arc with nonexistent source")
		}
		if !strings.Contains(err.Error(), "does not exist") {
			t.Errorf("expected 'does not exist' in error, got: %v", err)
		}
	})

	t.Run("arc with nonexistent target", func(t *testing.T) {
		net := NewPetriNet("net1", "TestNet")
		p1 := NewPlace("p1", "Place1", 10)
		arc := NewArc("a1", "p1", "nonexistent", 1)

		net.AddPlace(p1)
		net.AddArc(arc)

		err := net.Validate()
		if err == nil {
			t.Error("expected error for arc with nonexistent target")
		}
		if !strings.Contains(err.Error(), "does not exist") {
			t.Errorf("expected 'does not exist' in error, got: %v", err)
		}
	})

	t.Run("empty net is valid", func(t *testing.T) {
		net := NewPetriNet("net1", "TestNet")
		err := net.Validate()
		if err != nil {
			t.Errorf("empty net should be valid, got error: %v", err)
		}
	})
}

func TestPetriNet_Resolve(t *testing.T) {
	t.Run("simple place to transition arc", func(t *testing.T) {
		net := NewPetriNet("net1", "TestNet")
		p1 := NewPlace("p1", "Place1", 10)
		trans := NewTransition("t1", "Transition1")
		arc := NewArc("a1", "p1", "t1", 1)

		// Set up ID references
		p1.OutgoingArcIDs = []string{"a1"}
		trans.InputArcIDs = []string{"a1"}

		net.AddPlace(p1)
		net.AddTransition(trans)
		net.AddArc(arc)

		err := net.Resolve()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Check arc pointers
		if arc.Source != p1 {
			t.Error("arc source not resolved to place")
		}
		if arc.Target != trans {
			t.Error("arc target not resolved to transition")
		}
		if !arc.resolved {
			t.Error("arc not marked as resolved")
		}

		// Check place pointers
		if len(p1.OutgoingArcs()) != 1 {
			t.Errorf("expected 1 outgoing arc, got %d", len(p1.OutgoingArcs()))
		}
		if p1.OutgoingArcs()[0] != arc {
			t.Error("place outgoing arc not resolved")
		}
		if !p1.resolved {
			t.Error("place not marked as resolved")
		}

		// Check transition pointers
		if len(trans.InputArcs()) != 1 {
			t.Errorf("expected 1 input arc, got %d", len(trans.InputArcs()))
		}
		if trans.InputArcs()[0] != arc {
			t.Error("transition input arc not resolved")
		}
		if !trans.resolved {
			t.Error("transition not marked as resolved")
		}

		// Check net marked as resolved
		if !net.resolved {
			t.Error("net not marked as resolved")
		}
	})

	t.Run("transition to place arc", func(t *testing.T) {
		net := NewPetriNet("net1", "TestNet")
		trans := NewTransition("t1", "Transition1")
		p1 := NewPlace("p1", "Place1", 10)
		arc := NewArc("a1", "t1", "p1", 1)

		// Set up ID references
		trans.OutputArcIDs = []string{"a1"}
		p1.IncomingArcIDs = []string{"a1"}

		net.AddTransition(trans)
		net.AddPlace(p1)
		net.AddArc(arc)

		err := net.Resolve()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Check arc pointers
		if arc.Source != trans {
			t.Error("arc source not resolved to transition")
		}
		if arc.Target != p1 {
			t.Error("arc target not resolved to place")
		}

		// Check transition pointers
		if len(trans.OutputArcs()) != 1 {
			t.Errorf("expected 1 output arc, got %d", len(trans.OutputArcs()))
		}
		if trans.OutputArcs()[0] != arc {
			t.Error("transition output arc not resolved")
		}

		// Check place pointers
		if len(p1.IncomingArcs()) != 1 {
			t.Errorf("expected 1 incoming arc, got %d", len(p1.IncomingArcs()))
		}
		if p1.IncomingArcs()[0] != arc {
			t.Error("place incoming arc not resolved")
		}
	})

	t.Run("resolve multiple times is idempotent", func(t *testing.T) {
		net := NewPetriNet("net1", "TestNet")
		p1 := NewPlace("p1", "Place1", 10)
		trans := NewTransition("t1", "Transition1")
		arc := NewArc("a1", "p1", "t1", 1)

		p1.OutgoingArcIDs = []string{"a1"}
		trans.InputArcIDs = []string{"a1"}

		net.AddPlace(p1)
		net.AddTransition(trans)
		net.AddArc(arc)

		// Resolve multiple times
		err := net.Resolve()
		if err != nil {
			t.Fatalf("first resolve failed: %v", err)
		}

		err = net.Resolve()
		if err != nil {
			t.Errorf("second resolve failed: %v", err)
		}

		// Should still be correctly resolved
		if arc.Source != p1 {
			t.Error("arc source changed after second resolve")
		}
	})

	t.Run("error on nonexistent arc in place", func(t *testing.T) {
		net := NewPetriNet("net1", "TestNet")
		p1 := NewPlace("p1", "Place1", 10)
		p1.OutgoingArcIDs = []string{"nonexistent"}

		net.AddPlace(p1)

		err := net.Resolve()
		if err == nil {
			t.Error("expected error for nonexistent arc")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected 'not found' in error, got: %v", err)
		}
	})

	t.Run("error on nonexistent arc in transition", func(t *testing.T) {
		net := NewPetriNet("net1", "TestNet")
		trans := NewTransition("t1", "Transition1")
		trans.InputArcIDs = []string{"nonexistent"}

		net.AddTransition(trans)

		err := net.Resolve()
		if err == nil {
			t.Error("expected error for nonexistent arc")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected 'not found' in error, got: %v", err)
		}
	})

	t.Run("error on nonexistent source in arc", func(t *testing.T) {
		net := NewPetriNet("net1", "TestNet")
		trans := NewTransition("t1", "Transition1")
		arc := NewArc("a1", "nonexistent", "t1", 1)

		net.AddTransition(trans)
		net.AddArc(arc)

		err := net.Resolve()
		if err == nil {
			t.Error("expected error for nonexistent source")
		}
		if !strings.Contains(err.Error(), "does not exist") {
			t.Errorf("expected 'does not exist' in error, got: %v", err)
		}
	})

	t.Run("error on nonexistent target in arc", func(t *testing.T) {
		net := NewPetriNet("net1", "TestNet")
		p1 := NewPlace("p1", "Place1", 10)
		arc := NewArc("a1", "p1", "nonexistent", 1)

		net.AddPlace(p1)
		net.AddArc(arc)

		err := net.Resolve()
		if err == nil {
			t.Error("expected error for nonexistent target")
		}
		if !strings.Contains(err.Error(), "does not exist") {
			t.Errorf("expected 'does not exist' in error, got: %v", err)
		}
	})
}

func TestPetriNet_CompleteWorkflow(t *testing.T) {
	// Build a complete workflow: p1 -> t1 -> p2
	net := NewPetriNet("workflow1", "Simple Workflow")

	// Create places
	p1 := NewPlace("p1", "Input", 10)
	p2 := NewPlace("p2", "Output", 10)

	// Create transition
	trans := NewTransition("t1", "Process")

	// Create arcs
	arc1 := NewArc("a1", "p1", "t1", 1)
	arc2 := NewArc("a2", "t1", "p2", 1)

	// Set up ID references
	p1.OutgoingArcIDs = []string{"a1"}
	trans.InputArcIDs = []string{"a1"}
	trans.OutputArcIDs = []string{"a2"}
	p2.IncomingArcIDs = []string{"a2"}

	// Add to net
	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddTransition(trans)
	net.AddArc(arc1)
	net.AddArc(arc2)

	// Validate
	err := net.Validate()
	if err != nil {
		t.Fatalf("validation failed: %v", err)
	}

	// Resolve
	err = net.Resolve()
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}

	// Add token to input place
	inputToken := &token.Token{
		ID:   "tok1",
		Data: &SimpleTokenData{Value: "test"},
	}
	err = p1.AddToken(inputToken)
	if err != nil {
		t.Fatalf("failed to add token: %v", err)
	}

	// Check transition is enabled
	ctx := &execContext.ExecutionContext{}
	if enabled, _ := trans.IsEnabled(ctx); !enabled {
		t.Error("transition should be enabled with input token")
	}

	// Verify structure
	if len(p1.OutgoingArcs()) != 1 {
		t.Errorf("p1 should have 1 outgoing arc, got %d", len(p1.OutgoingArcs()))
	}
	if len(trans.InputArcs()) != 1 {
		t.Errorf("t1 should have 1 input arc, got %d", len(trans.InputArcs()))
	}
	if len(trans.OutputArcs()) != 1 {
		t.Errorf("t1 should have 1 output arc, got %d", len(trans.OutputArcs()))
	}
	if len(p2.IncomingArcs()) != 1 {
		t.Errorf("p2 should have 1 incoming arc, got %d", len(p2.IncomingArcs()))
	}
}
