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

// TestTokenData is a simple token type for testing
type TestTokenData struct {
	Value string
}

func (t *TestTokenData) Type() string {
	return "test"
}

func TestRestoreMarkingToNet_NilInputs(t *testing.T) {
	t.Run("nil marking", func(t *testing.T) {
		net := createSimpleNet(t)
		net.Resolve()

		err := RestoreMarkingToNet(nil, net)
		if err == nil {
			t.Error("expected error for nil marking")
		}
		if err.Error() != "cannot restore nil marking" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("nil net", func(t *testing.T) {
		marking := &Marking{
			Places: make(map[string]PlaceState),
		}

		err := RestoreMarkingToNet(marking, nil)
		if err == nil {
			t.Error("expected error for nil net")
		}
		if err.Error() != "cannot restore to nil net" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("unresolved net", func(t *testing.T) {
		net := createSimpleNet(t)
		// Don't resolve

		marking := &Marking{
			Places: make(map[string]PlaceState),
		}

		err := RestoreMarkingToNet(marking, net)
		if err == nil {
			t.Error("expected error for unresolved net")
		}
		if err.Error() != "net must be resolved before restoration" {
			t.Errorf("unexpected error message: %v", err)
		}
	})
}

func TestRestoreMarkingToNet_EmptyMarking(t *testing.T) {
	net := createSimpleNet(t)
	net.Resolve()

	// Add some tokens to the net
	p1 := net.Places["p1"]
	tok := &token.Token{ID: "t1", Data: &TestTokenData{Value: "test"}}
	if err := p1.AddToken(tok); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Verify setup
	if p1.TokenCount() != 1 {
		t.Fatalf("expected 1 token before restore, got %d", p1.TokenCount())
	}

	// Restore empty marking
	marking := &Marking{
		Places: make(map[string]PlaceState),
	}

	err := RestoreMarkingToNet(marking, net)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all places are cleared
	if p1.TokenCount() != 0 {
		t.Errorf("expected 0 tokens after restore, got %d", p1.TokenCount())
	}
}

func TestRestoreMarkingToNet_WithTokens(t *testing.T) {
	net := createSimpleNet(t)
	net.Resolve()

	// Create marking with tokens
	tok1 := &token.Token{ID: "t1", Data: &TestTokenData{Value: "data1"}}
	tok2 := &token.Token{ID: "t2", Data: &TestTokenData{Value: "data2"}}

	marking := &Marking{
		Places: map[string]PlaceState{
			"p1": {
				PlaceID:  "p1",
				Tokens:   []*token.Token{tok1, tok2},
				Capacity: 10,
			},
			"p2": {
				PlaceID:  "p2",
				Tokens:   []*token.Token{},
				Capacity: 10,
			},
		},
	}

	// Restore
	err := RestoreMarkingToNet(marking, net)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify tokens restored
	p1 := net.Places["p1"]
	if p1.TokenCount() != 2 {
		t.Errorf("expected 2 tokens in p1, got %d", p1.TokenCount())
	}

	p2 := net.Places["p2"]
	if p2.TokenCount() != 0 {
		t.Errorf("expected 0 tokens in p2, got %d", p2.TokenCount())
	}

	// Verify token order (FIFO)
	removed1, err := p1.RemoveToken()
	if err != nil {
		t.Fatalf("failed to remove token: %v", err)
	}
	if removed1.ID != "t1" {
		t.Errorf("expected first token t1, got %s", removed1.ID)
	}

	removed2, err := p1.RemoveToken()
	if err != nil {
		t.Fatalf("failed to remove token: %v", err)
	}
	if removed2.ID != "t2" {
		t.Errorf("expected second token t2, got %s", removed2.ID)
	}
}

func TestRestoreMarkingToNet_PlaceMismatch(t *testing.T) {
	net := createSimpleNet(t)
	net.Resolve()

	// Create marking with unknown place
	marking := &Marking{
		Places: map[string]PlaceState{
			"unknown_place": {
				PlaceID:  "unknown_place",
				Tokens:   []*token.Token{},
				Capacity: 10,
			},
		},
	}

	err := RestoreMarkingToNet(marking, net)
	if err == nil {
		t.Error("expected error for unknown place")
	}
	if err.Error() != "place unknown_place not found in net" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRestoreMarkingToNet_ClearExisting(t *testing.T) {
	net := createSimpleNet(t)
	net.Resolve()

	// Add tokens to places
	p1 := net.Places["p1"]
	p2 := net.Places["p2"]

	tok1 := &token.Token{ID: "old1", Data: &TestTokenData{Value: "old"}}
	tok2 := &token.Token{ID: "old2", Data: &TestTokenData{Value: "old"}}
	p1.AddToken(tok1)
	p2.AddToken(tok2)

	// Verify setup
	if p1.TokenCount() != 1 || p2.TokenCount() != 1 {
		t.Fatalf("setup failed")
	}

	// Create new marking with different tokens
	newTok := &token.Token{ID: "new1", Data: &TestTokenData{Value: "new"}}
	marking := &Marking{
		Places: map[string]PlaceState{
			"p1": {
				PlaceID:  "p1",
				Tokens:   []*token.Token{newTok},
				Capacity: 10,
			},
			"p2": {
				PlaceID:  "p2",
				Tokens:   []*token.Token{},
				Capacity: 10,
			},
		},
	}

	// Restore
	err := RestoreMarkingToNet(marking, net)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify old tokens cleared and new tokens added
	if p1.TokenCount() != 1 {
		t.Errorf("expected 1 token in p1, got %d", p1.TokenCount())
	}
	if p2.TokenCount() != 0 {
		t.Errorf("expected 0 tokens in p2, got %d", p2.TokenCount())
	}

	removed, err := p1.RemoveToken()
	if err != nil {
		t.Fatalf("failed to remove token: %v", err)
	}
	if removed.ID != "new1" {
		t.Errorf("expected new token new1, got %s", removed.ID)
	}
}

func TestRestoreMarkingToNet_RoundTrip(t *testing.T) {
	// Test that we can capture and restore state
	net := createSimpleNet(t)
	net.Resolve()

	// Add tokens
	p1 := net.Places["p1"]
	tok1 := &token.Token{ID: "t1", Data: &TestTokenData{Value: "data1"}}
	tok2 := &token.Token{ID: "t2", Data: &TestTokenData{Value: "data2"}}
	p1.AddToken(tok1)
	p1.AddToken(tok2)

	// Capture marking
	clk := clock.NewVirtualClock(time.Now())
	marking, err := NewMarking(net, clk)
	if err != nil {
		t.Fatalf("failed to capture marking: %v", err)
	}

	// Clear the net
	p1.Clear()
	if p1.TokenCount() != 0 {
		t.Fatalf("expected 0 tokens after clear, got %d", p1.TokenCount())
	}

	// Restore
	err = RestoreMarkingToNet(marking, net)
	if err != nil {
		t.Fatalf("failed to restore: %v", err)
	}

	// Verify state matches original
	if p1.TokenCount() != 2 {
		t.Errorf("expected 2 tokens after restore, got %d", p1.TokenCount())
	}

	// Verify token order preserved
	removed1, _ := p1.RemoveToken()
	removed2, _ := p1.RemoveToken()

	if removed1.ID != "t1" || removed2.ID != "t2" {
		t.Error("token order not preserved after round trip")
	}
}

// TestRestoreMarkingToNet_FailureAfterClear_PreservesOriginalState tests that
// if restoration fails after clearing, the original state is preserved through rollback.
func TestRestoreMarkingToNet_FailureAfterClear_PreservesOriginalState(t *testing.T) {
	net := createSimpleNet(t)
	net.Resolve()

	// Add initial tokens
	p1 := net.Places["p1"]
	p2 := net.Places["p2"]
	tok1 := &token.Token{ID: "orig1", Data: &TestTokenData{Value: "original1"}}
	tok2 := &token.Token{ID: "orig2", Data: &TestTokenData{Value: "original2"}}
	if err := p1.AddToken(tok1); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if err := p2.AddToken(tok2); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Verify setup
	if p1.TokenCount() != 1 || p2.TokenCount() != 1 {
		t.Fatalf("setup failed: expected 1 token in each place")
	}

	// Create marking with unknown place (will fail during restoration)
	marking := &Marking{
		Places: map[string]PlaceState{
			"unknown_place": {
				PlaceID:  "unknown_place",
				Tokens:   []*token.Token{},
				Capacity: 10,
			},
		},
	}

	// Attempt restoration - should fail
	err := RestoreMarkingToNet(marking, net)
	if err == nil {
		t.Fatal("expected error for unknown place")
	}

	// CRITICAL: Original state should be preserved after failure
	if p1.TokenCount() != 1 {
		t.Errorf("expected original token in p1 to be preserved, got %d tokens", p1.TokenCount())
	}
	if p2.TokenCount() != 1 {
		t.Errorf("expected original token in p2 to be preserved, got %d tokens", p2.TokenCount())
	}

	// Verify original token IDs are preserved
	if p1.TokenCount() == 1 {
		tok, _ := p1.RemoveToken()
		if tok.ID != "orig1" {
			t.Errorf("expected original token orig1, got %s", tok.ID)
		}
		// Put it back
		p1.AddToken(tok)
	}

	if p2.TokenCount() == 1 {
		tok, _ := p2.RemoveToken()
		if tok.ID != "orig2" {
			t.Errorf("expected original token orig2, got %s", tok.ID)
		}
		// Put it back
		p2.AddToken(tok)
	}
}

// TestRestoreMarkingToNet_PlaceNotFound_RollsBack tests that
// when a place is not found in the net, the restoration rolls back to original state.
func TestRestoreMarkingToNet_PlaceNotFound_RollsBack(t *testing.T) {
	net := createSimpleNet(t)
	net.Resolve()

	// Add initial tokens
	p1 := net.Places["p1"]
	tok1 := &token.Token{ID: "orig1", Data: &TestTokenData{Value: "original"}}
	tok2 := &token.Token{ID: "orig2", Data: &TestTokenData{Value: "original"}}
	if err := p1.AddToken(tok1); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if err := p1.AddToken(tok2); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	originalCount := p1.TokenCount()
	if originalCount != 2 {
		t.Fatalf("setup failed: expected 2 tokens, got %d", originalCount)
	}

	// Create marking with unknown place
	marking := &Marking{
		Places: map[string]PlaceState{
			"p1": {
				PlaceID: "p1",
				Tokens: []*token.Token{
					{ID: "new1", Data: &TestTokenData{Value: "new"}},
				},
				Capacity: 10,
			},
			"nonexistent": {
				PlaceID:  "nonexistent",
				Tokens:   []*token.Token{},
				Capacity: 10,
			},
		},
	}

	// Attempt restoration - should fail
	err := RestoreMarkingToNet(marking, net)
	if err == nil {
		t.Fatal("expected error for nonexistent place")
	}

	// Original state should be restored
	if p1.TokenCount() != originalCount {
		t.Errorf("expected %d tokens after rollback, got %d", originalCount, p1.TokenCount())
	}

	// Verify original tokens are back (not new ones)
	tok, _ := p1.RemoveToken()
	if tok.ID != "orig1" {
		t.Errorf("expected original token orig1, got %s", tok.ID)
	}
}

// TestRestoreMarkingToNet_TokenAddFailure_RollsBack tests that
// when token addition fails (e.g., capacity exceeded), the restoration rolls back.
func TestRestoreMarkingToNet_TokenAddFailure_RollsBack(t *testing.T) {
	net := petri.NewPetriNet("test_net", "Test Net")

	// Create place with very small capacity
	p1 := petri.NewPlace("p1", "Place 1", 1) // Capacity of 1
	net.AddPlace(p1)
	net.Resolve()

	// Add initial token
	origTok := &token.Token{ID: "orig1", Data: &TestTokenData{Value: "original"}}
	if err := p1.AddToken(origTok); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Create marking with more tokens than capacity
	marking := &Marking{
		Places: map[string]PlaceState{
			"p1": {
				PlaceID: "p1",
				Tokens: []*token.Token{
					{ID: "new1", Data: &TestTokenData{Value: "new1"}},
					{ID: "new2", Data: &TestTokenData{Value: "new2"}},
				},
				Capacity: 1,
			},
		},
	}

	// Attempt restoration - should fail when adding second token
	err := RestoreMarkingToNet(marking, net)
	if err == nil {
		t.Fatal("expected error when exceeding capacity")
	}

	// Original state should be restored
	if p1.TokenCount() != 1 {
		t.Errorf("expected 1 token after rollback, got %d", p1.TokenCount())
	}

	// Verify original token is back
	tok, _ := p1.RemoveToken()
	if tok.ID != "orig1" {
		t.Errorf("expected original token orig1, got %s", tok.ID)
	}
}

// TestRestoreMarkingToNet_Success_StillWorks verifies that
// successful restoration still works correctly after implementing rollback.
func TestRestoreMarkingToNet_Success_StillWorks(t *testing.T) {
	net := createSimpleNet(t)
	net.Resolve()

	// Add initial tokens
	p1 := net.Places["p1"]
	origTok := &token.Token{ID: "orig1", Data: &TestTokenData{Value: "original"}}
	if err := p1.AddToken(origTok); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Create valid marking
	marking := &Marking{
		Places: map[string]PlaceState{
			"p1": {
				PlaceID: "p1",
				Tokens: []*token.Token{
					{ID: "new1", Data: &TestTokenData{Value: "new1"}},
					{ID: "new2", Data: &TestTokenData{Value: "new2"}},
				},
				Capacity: 10,
			},
			"p2": {
				PlaceID:  "p2",
				Tokens:   []*token.Token{},
				Capacity: 10,
			},
		},
	}

	// Restoration should succeed
	err := RestoreMarkingToNet(marking, net)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify new state
	if p1.TokenCount() != 2 {
		t.Errorf("expected 2 tokens, got %d", p1.TokenCount())
	}

	// Verify token IDs
	tok1, _ := p1.RemoveToken()
	tok2, _ := p1.RemoveToken()
	if tok1.ID != "new1" || tok2.ID != "new2" {
		t.Errorf("expected new tokens, got %s and %s", tok1.ID, tok2.ID)
	}
}

// Helper function to create a simple test net
func createSimpleNet(t *testing.T) *petri.PetriNet {
	t.Helper()

	net := petri.NewPetriNet("test_net", "Test Net")

	// Create places
	p1 := petri.NewPlace("p1", "Place 1", 10)
	p2 := petri.NewPlace("p2", "Place 2", 10)

	net.AddPlace(p1)
	net.AddPlace(p2)

	return net
}
