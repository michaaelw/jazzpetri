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

package state

import (
	"testing"
	"time"

	"github.com/jazzpetri/engine/clock"
	"github.com/jazzpetri/engine/petri"
	"github.com/jazzpetri/engine/token"
)

// TestNewMarking_EmptyNet tests creating a marking from an empty Petri net
func TestNewMarking_EmptyNet(t *testing.T) {
	net := petri.NewPetriNet("net1", "Test Net")
	if err := net.Resolve(); err != nil {
		t.Fatalf("Failed to resolve net: %v", err)
	}

	clk := clock.NewVirtualClock(time.Now())
	marking, err := NewMarking(net, clk)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if marking == nil {
		t.Fatal("Expected marking, got nil")
	}
	if len(marking.Places) != 0 {
		t.Errorf("Expected 0 places, got %d", len(marking.Places))
	}
	if !marking.IsEmpty() {
		t.Error("Expected marking to be empty")
	}
	if marking.TokenCount() != 0 {
		t.Errorf("Expected token count 0, got %d", marking.TokenCount())
	}
}

// TestNewMarking_UnresolvedNet tests that unresolved net returns error
func TestNewMarking_UnresolvedNet(t *testing.T) {
	net := petri.NewPetriNet("net1", "Test Net")
	// Don't resolve the net

	clk := clock.NewVirtualClock(time.Now())
	_, err := NewMarking(net, clk)

	if err == nil {
		t.Fatal("Expected error for unresolved net, got nil")
	}
}

// TestNewMarking_WithTokens tests creating marking from net with tokens
func TestNewMarking_WithTokens(t *testing.T) {
	// Create a simple net: p1 -> t1 -> p2
	net := petri.NewPetriNet("net1", "Test Net")

	p1 := petri.NewPlace("p1", "Place 1", 10)
	p2 := petri.NewPlace("p2", "Place 2", 10)
	t1 := petri.NewTransition("t1", "Transition 1")

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddTransition(t1)

	arc1 := petri.NewArc("a1", "p1", "t1", 1)
	arc2 := petri.NewArc("a2", "t1", "p2", 1)
	net.AddArc(arc1)
	net.AddArc(arc2)

	if err := net.Resolve(); err != nil {
		t.Fatalf("Failed to resolve net: %v", err)
	}

	// Add tokens to p1
	tok1 := &token.Token{ID: "tok1", Data: &token.StringToken{Value: "data1"}}
	tok2 := &token.Token{ID: "tok2", Data: &token.StringToken{Value: "data2"}}
	p1.AddToken(tok1)
	p1.AddToken(tok2)

	// Create marking
	clk := clock.NewVirtualClock(time.Now())
	marking, err := NewMarking(net, clk)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if marking == nil {
		t.Fatal("Expected marking, got nil")
	}

	// Verify marking structure
	if len(marking.Places) != 2 {
		t.Errorf("Expected 2 places, got %d", len(marking.Places))
	}
	if marking.TokenCount() != 2 {
		t.Errorf("Expected total token count 2, got %d", marking.TokenCount())
	}
	if marking.PlaceTokenCount("p1") != 2 {
		t.Errorf("Expected p1 token count 2, got %d", marking.PlaceTokenCount("p1"))
	}
	if marking.PlaceTokenCount("p2") != 0 {
		t.Errorf("Expected p2 token count 0, got %d", marking.PlaceTokenCount("p2"))
	}
	if marking.IsEmpty() {
		t.Error("Expected marking to not be empty")
	}

	// Verify tokens in p1
	tokens := marking.GetTokens("p1")
	if len(tokens) != 2 {
		t.Errorf("Expected 2 tokens in p1, got %d", len(tokens))
	}

	// Verify place state
	p1State, exists := marking.Places["p1"]
	if !exists {
		t.Fatal("Expected p1 in marking")
	}
	if p1State.PlaceID != "p1" {
		t.Errorf("Expected place ID p1, got %s", p1State.PlaceID)
	}
	if p1State.Capacity != 10 {
		t.Errorf("Expected capacity 10, got %d", p1State.Capacity)
	}
}

// TestMarking_TokenQueries tests various token query methods
func TestMarking_TokenQueries(t *testing.T) {
	net := petri.NewPetriNet("net1", "Test Net")
	p1 := petri.NewPlace("p1", "Place 1", 10)
	p2 := petri.NewPlace("p2", "Place 2", 10)
	p3 := petri.NewPlace("p3", "Place 3", 10)

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddPlace(p3)
	net.Resolve()

	// Add different number of tokens to each place
	p1.AddToken(&token.Token{ID: "t1", Data: &token.StringToken{Value: "data1"}})
	p1.AddToken(&token.Token{ID: "t2", Data: &token.StringToken{Value: "data2"}})
	p2.AddToken(&token.Token{ID: "t3", Data: &token.StringToken{Value: "data3"}})
	// p3 remains empty

	clk := clock.NewVirtualClock(time.Now())
	marking, _ := NewMarking(net, clk)

	// Test TokenCount
	if marking.TokenCount() != 3 {
		t.Errorf("Expected total tokens 3, got %d", marking.TokenCount())
	}

	// Test PlaceTokenCount
	if marking.PlaceTokenCount("p1") != 2 {
		t.Errorf("Expected p1 tokens 2, got %d", marking.PlaceTokenCount("p1"))
	}
	if marking.PlaceTokenCount("p2") != 1 {
		t.Errorf("Expected p2 tokens 1, got %d", marking.PlaceTokenCount("p2"))
	}
	if marking.PlaceTokenCount("p3") != 0 {
		t.Errorf("Expected p3 tokens 0, got %d", marking.PlaceTokenCount("p3"))
	}
	if marking.PlaceTokenCount("nonexistent") != 0 {
		t.Errorf("Expected nonexistent place tokens 0, got %d", marking.PlaceTokenCount("nonexistent"))
	}

	// Test GetTokens
	p1Tokens := marking.GetTokens("p1")
	if len(p1Tokens) != 2 {
		t.Errorf("Expected 2 tokens from p1, got %d", len(p1Tokens))
	}

	p3Tokens := marking.GetTokens("p3")
	if len(p3Tokens) != 0 {
		t.Errorf("Expected 0 tokens from p3, got %d", len(p3Tokens))
	}

	nonexistentTokens := marking.GetTokens("nonexistent")
	if nonexistentTokens != nil {
		t.Error("Expected nil for nonexistent place")
	}

	// Test IsEmpty
	if marking.IsEmpty() {
		t.Error("Expected marking not to be empty")
	}
}

// TestMarking_Clone tests deep copying of markings
func TestMarking_Clone(t *testing.T) {
	net := petri.NewPetriNet("net1", "Test Net")
	p1 := petri.NewPlace("p1", "Place 1", 10)
	net.AddPlace(p1)
	net.Resolve()

	tok := &token.Token{ID: "tok1", Data: &token.StringToken{Value: "data1"}}
	p1.AddToken(tok)

	clk := clock.NewVirtualClock(time.Now())
	original, _ := NewMarking(net, clk)
	clone := original.Clone()

	// Verify clone is not the same instance
	if clone == original {
		t.Error("Clone should be a different instance")
	}

	// Verify clone has same content
	if clone.TokenCount() != original.TokenCount() {
		t.Error("Clone should have same token count")
	}
	if len(clone.Places) != len(original.Places) {
		t.Error("Clone should have same number of places")
	}

	// Verify deep copy of places map
	if &clone.Places == &original.Places {
		t.Error("Clone should have independent Places map")
	}

	// Verify timestamps are preserved
	if !clone.Timestamp.Equal(original.Timestamp) {
		t.Error("Clone should preserve timestamp")
	}

	// Modify clone and verify original is unchanged
	cloneP1 := clone.Places["p1"]
	cloneP1.Capacity = 999
	if original.Places["p1"].Capacity == 999 {
		t.Error("Modifying clone should not affect original")
	}
}

// TestMarking_Equal tests marking equality comparison
func TestMarking_Equal(t *testing.T) {
	net := petri.NewPetriNet("net1", "Test Net")
	p1 := petri.NewPlace("p1", "Place 1", 10)
	net.AddPlace(p1)
	net.Resolve()

	tok := &token.Token{ID: "tok1", Data: &token.StringToken{Value: "data1"}}
	p1.AddToken(tok)

	clk := clock.NewVirtualClock(time.Now())
	m1, _ := NewMarking(net, clk)
	m2, _ := NewMarking(net, clk)

	// Same content should be equal
	if !m1.Equal(m2) {
		t.Error("Markings with same content should be equal")
	}

	// Self-equality
	if !m1.Equal(m1) {
		t.Error("Marking should equal itself")
	}

	// Different token counts
	p1.AddToken(&token.Token{ID: "tok2", Data: &token.StringToken{Value: "data2"}})
	m3, _ := NewMarking(net, clk)
	if m1.Equal(m3) {
		t.Error("Markings with different token counts should not be equal")
	}

	// Different number of places
	net2 := petri.NewPetriNet("net2", "Test Net 2")
	p2 := petri.NewPlace("p2", "Place 2", 10)
	net2.AddPlace(p2)
	net2.Resolve()
	m4, _ := NewMarking(net2, clk)
	if m1.Equal(m4) {
		t.Error("Markings with different places should not be equal")
	}
}

// TestMarking_Timestamp tests timestamp capture
func TestMarking_Timestamp(t *testing.T) {
	net := petri.NewPetriNet("net1", "Test Net")
	net.Resolve()

	startTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(startTime)

	m1, _ := NewMarking(net, clk)
	if !m1.Timestamp.Equal(startTime) {
		t.Errorf("Expected timestamp %v, got %v", startTime, m1.Timestamp)
	}

	// Advance clock and create new marking
	laterTime := startTime.Add(5 * time.Minute)
	clk.AdvanceTo(laterTime)
	m2, _ := NewMarking(net, clk)

	if !m2.Timestamp.Equal(laterTime) {
		t.Errorf("Expected timestamp %v, got %v", laterTime, m2.Timestamp)
	}
	if m1.Timestamp.Equal(m2.Timestamp) {
		t.Error("Different markings should have different timestamps")
	}
}

// TestMarking_NonDestructiveCapture tests that marking capture doesn't remove tokens
func TestMarking_NonDestructiveCapture(t *testing.T) {
	net := petri.NewPetriNet("net1", "Test Net")
	p1 := petri.NewPlace("p1", "Place 1", 10)
	net.AddPlace(p1)
	net.Resolve()

	// Add tokens
	tok1 := &token.Token{ID: "tok1", Data: &token.StringToken{Value: "data1"}}
	tok2 := &token.Token{ID: "tok2", Data: &token.StringToken{Value: "data2"}}
	p1.AddToken(tok1)
	p1.AddToken(tok2)

	// Capture marking
	clk := clock.NewVirtualClock(time.Now())
	marking, _ := NewMarking(net, clk)

	// Verify tokens captured
	if marking.PlaceTokenCount("p1") != 2 {
		t.Errorf("Expected marking to capture 2 tokens, got %d", marking.PlaceTokenCount("p1"))
	}

	// Verify tokens still in place (non-destructive)
	if p1.TokenCount() != 2 {
		t.Errorf("Expected place to still have 2 tokens after capture, got %d", p1.TokenCount())
	}

	// Verify we can still remove tokens from place
	tok, err := p1.RemoveToken()
	if err != nil {
		t.Errorf("Should be able to remove token after capture: %v", err)
	}
	if tok == nil {
		t.Error("Expected to get token from place")
	}
}
