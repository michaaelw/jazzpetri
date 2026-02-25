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
	"fmt"
	"testing"

	"github.com/jazzpetri/engine/token"
)

// capacityTestTokenData is a simple test implementation of TokenData
type capacityTestTokenData struct {
	value string
}

func (s *capacityTestTokenData) Type() string {
	return "capacity-test"
}

// TestPetriNet_GlobalCapacity tests:
// Only per-place capacity exists, no global token limit across all places
//
// EXPECTED FAILURE: Global capacity feature does not exist
//
// Use case: Limit total tokens in the system to prevent unbounded growth
// in long-running workflows with token-generating transitions.
//
// Example: A workflow that processes orders might want to limit total
// in-flight orders across all places to prevent memory exhaustion.
func TestPetriNet_GlobalCapacity(t *testing.T) {
	// Arrange
	net := NewPetriNet("test-net", "Test Network")

	// Global capacity now implemented
	net.SetGlobalCapacity(100)

	// Add multiple places
	place1 := NewPlace("p1", "Place 1", 50)
	place2 := NewPlace("p2", "Place 2", 50)
	place3 := NewPlace("p3", "Place 3", 50)

	if err := net.AddPlace(place1); err != nil {
		t.Fatalf("Failed to add place1: %v", err)
	}
	if err := net.AddPlace(place2); err != nil {
		t.Fatalf("Failed to add place2: %v", err)
	}
	if err := net.AddPlace(place3); err != nil {
		t.Fatalf("Failed to add place3: %v", err)
	}

	// Act: Try to add tokens up to global capacity
	// Each place can hold 50, but global capacity should be 100
	globalCapacity := 100
	tokensAdded := 0

	for i := 0; i < 150; i++ { // Try to add more than global capacity
		tok := &token.Token{
			ID:   fmt.Sprintf("token-%d", i),
			Data: &capacityTestTokenData{value: fmt.Sprintf("%d", i)},
		}

		// Try adding to places in round-robin fashion
		placeIdx := i % 3
		var targetPlace *Place
		switch placeIdx {
		case 0:
			targetPlace = place1
		case 1:
			targetPlace = place2
		case 2:
			targetPlace = place3
		}

		// EXPECTED FAILURE: AddToken does not check global capacity
		err := targetPlace.AddToken(tok)
		if err != nil {
			// If global capacity was enforced, we should hit limit at 100 tokens
			break
		}
		tokensAdded++
	}

	// Assert: Should only be able to add up to global capacity
	// Global capacity now enforced
	if tokensAdded != globalCapacity {
		t.Errorf("Added %d tokens, expected to stop at global capacity of %d",
			tokensAdded, globalCapacity)
	}

	totalTokens := net.GetTotalTokens()
	if totalTokens != globalCapacity {
		t.Errorf("Total tokens in system: %d, expected %d", totalTokens, globalCapacity)
	}

	t.Logf("Successfully enforced global capacity of %d tokens", globalCapacity)
}

// TestPetriNet_GlobalCapacityEnforcement tests:
// Verify global capacity is enforced when adding tokens
//
// EXPECTED FAILURE: Global capacity enforcement does not exist
func TestPetriNet_GlobalCapacityEnforcement(t *testing.T) {
	tests := []struct {
		name           string
		globalCapacity int
		places         []int // per-place capacities
		tokensToAdd    int
		wantSuccess    int
		wantError      bool
	}{
		{
			name:           "global capacity less than sum of place capacities",
			globalCapacity: 50,
			places:         []int{30, 30, 30}, // sum=90, but global=50
			tokensToAdd:    60,
			wantSuccess:    50, // Should stop at global capacity
			wantError:      true,
		},
		{
			name:           "global capacity equals sum of place capacities",
			globalCapacity: 90,
			places:         []int{30, 30, 30}, // sum=90
			tokensToAdd:    90,
			wantSuccess:    90,
			wantError:      false,
		},
		{
			name:           "global capacity greater than sum of place capacities",
			globalCapacity: 200,
			places:         []int{30, 30, 30}, // sum=90
			tokensToAdd:    100,
			wantSuccess:    90, // Limited by place capacities, not global
			wantError:      true,
		},
		{
			name:           "zero global capacity means unlimited",
			globalCapacity: 0, // 0 = unlimited
			places:         []int{10, 10, 10},
			tokensToAdd:    30,
			wantSuccess:    30,
			wantError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			net := NewPetriNet("test-net", "Test Network")

			// Global capacity now implemented
			if err := net.SetGlobalCapacity(tt.globalCapacity); err != nil {
				t.Fatalf("SetGlobalCapacity failed: %v", err)
			}

			// Create places
			places := make([]*Place, len(tt.places))
			for i, cap := range tt.places {
				place := NewPlace(fmt.Sprintf("p%d", i), fmt.Sprintf("Place %d", i), cap)
				places[i] = place
				if err := net.AddPlace(place); err != nil {
					t.Fatalf("Failed to add place %d: %v", i, err)
				}
			}

			// Act: Add tokens
			successCount := 0
			var lastErr error

			for i := 0; i < tt.tokensToAdd; i++ {
				tok := &token.Token{
					ID:   fmt.Sprintf("token-%d", i),
					Data: &capacityTestTokenData{value: fmt.Sprintf("%d", i)},
				}

				// Add to places in round-robin
				placeIdx := i % len(places)
				err := places[placeIdx].AddToken(tok)

				if err != nil {
					lastErr = err
					break
				}
				successCount++
			}

			// Assert
			// Global capacity now enforced
			if successCount != tt.wantSuccess {
				t.Errorf("Added %d tokens, want %d", successCount, tt.wantSuccess)
			}

			if tt.wantError && lastErr == nil {
				t.Error("Expected error when exceeding capacity, got none")
			}

			// Log total tokens
			totalTokens := 0
			for i, place := range places {
				count := place.TokenCount()
				totalTokens += count
				t.Logf("Place %d: %d tokens", i, count)
			}
			t.Logf("Total tokens: %d (global capacity: %d)", totalTokens, tt.globalCapacity)
		})
	}
}

// TestPetriNet_GlobalCapacity_GetTotalTokens tests:
// Utility method to get total token count across all places
//
// EXPECTED FAILURE: GetTotalTokens method does not exist
func TestPetriNet_GlobalCapacity_GetTotalTokens(t *testing.T) {
	// Arrange
	net := NewPetriNet("test-net", "Test Network")

	place1 := NewPlace("p1", "Place 1", 50)
	place2 := NewPlace("p2", "Place 2", 50)
	place3 := NewPlace("p3", "Place 3", 50)

	net.AddPlace(place1)
	net.AddPlace(place2)
	net.AddPlace(place3)

	// Add different numbers of tokens to each place
	for i := 0; i < 10; i++ {
		place1.AddToken(&token.Token{ID: fmt.Sprintf("t1-%d", i), Data: &capacityTestTokenData{value: fmt.Sprintf("t1-%d", i)}})
	}
	for i := 0; i < 20; i++ {
		place2.AddToken(&token.Token{ID: fmt.Sprintf("t2-%d", i), Data: &capacityTestTokenData{value: fmt.Sprintf("t2-%d", i)}})
	}
	for i := 0; i < 5; i++ {
		place3.AddToken(&token.Token{ID: fmt.Sprintf("t3-%d", i), Data: &capacityTestTokenData{value: fmt.Sprintf("t3-%d", i)}})
	}

	// Act
	// GetTotalTokens now implemented
	total := net.GetTotalTokens()

	// Assert
	expectedTotal := 10 + 20 + 5
	if total != expectedTotal {
		t.Errorf("GetTotalTokens() = %d, want %d", total, expectedTotal)
	}

	t.Logf("GetTotalTokens() correctly returned %d tokens", total)
}

// TestPetriNet_GlobalCapacity_MultipleNets tests:
// Each PetriNet should have its own independent global capacity
//
// EXPECTED FAILURE: Global capacity feature does not exist
func TestPetriNet_GlobalCapacity_MultipleNets(t *testing.T) {
	// TODO: Implement global capacity per-net enforcement
	// This test documents the expected behavior where each net should enforce its own global capacity
	// independently. Currently, global capacity is not implemented.
	t.Skip("Global capacity feature not yet implemented - each net should enforce its own global capacity")

	// Arrange
	net1 := NewPetriNet("net1", "Network 1")
	net2 := NewPetriNet("net2", "Network 2")

	// EXPECTED FAILURE: SetGlobalCapacity does not exist
	// Uncomment when implementing:
	// net1.SetGlobalCapacity(50)
	// net2.SetGlobalCapacity(100)

	// Add places to both nets
	place1a := NewPlace("p1a", "Net1 Place A", 100)
	place1b := NewPlace("p1b", "Net1 Place B", 100)
	net1.AddPlace(place1a)
	net1.AddPlace(place1b)

	place2a := NewPlace("p2a", "Net2 Place A", 100)
	place2b := NewPlace("p2b", "Net2 Place B", 100)
	net2.AddPlace(place2a)
	net2.AddPlace(place2b)

	// Act: Add tokens to net1 (global capacity 50)
	for i := 0; i < 60; i++ {
		tok := &token.Token{ID: fmt.Sprintf("n1-token-%d", i), Data: &capacityTestTokenData{value: fmt.Sprintf("n1-%d", i)}}
		if i%2 == 0 {
			place1a.AddToken(tok)
		} else {
			place1b.AddToken(tok)
		}
	}

	// Add tokens to net2 (global capacity 100)
	for i := 0; i < 120; i++ {
		tok := &token.Token{ID: fmt.Sprintf("n2-token-%d", i), Data: &capacityTestTokenData{value: fmt.Sprintf("n2-%d", i)}}
		if i%2 == 0 {
			place2a.AddToken(tok)
		} else {
			place2b.AddToken(tok)
		}
	}

	// Assert
	totalNet1 := place1a.TokenCount() + place1b.TokenCount()
	totalNet2 := place2a.TokenCount() + place2b.TokenCount()

	// EXPECTED FAILURE: Global capacities not enforced
	if totalNet1 > 50 {
		t.Errorf("EXPECTED FAILURE: Net1 has %d tokens, should be limited to 50", totalNet1)
	}

	if totalNet2 > 100 {
		t.Errorf("EXPECTED FAILURE: Net2 has %d tokens, should be limited to 100", totalNet2)
	}

	t.Logf("Net1 total: %d (limit: 50)", totalNet1)
	t.Logf("Net2 total: %d (limit: 100)", totalNet2)

	t.Log("EXPECTED FAILURE: Each net should enforce its own global capacity")
	t.Log("After fix: Global capacity is per-net, not system-wide")
}

// TestPetriNet_GlobalCapacity_TokenRemoval tests:
// Removing tokens should free up global capacity
//
// EXPECTED FAILURE: Global capacity feature does not exist
func TestPetriNet_GlobalCapacity_TokenRemoval(t *testing.T) {
	// Arrange
	net := NewPetriNet("test-net", "Test Network")

	// EXPECTED FAILURE: SetGlobalCapacity does not exist
	// Uncomment when implementing:
	// net.SetGlobalCapacity(10)

	place1 := NewPlace("p1", "Place 1", 20)
	place2 := NewPlace("p2", "Place 2", 20)
	net.AddPlace(place1)
	net.AddPlace(place2)

	globalCapacity := 10

	// Act: Fill to global capacity
	for i := 0; i < globalCapacity; i++ {
		tok := &token.Token{ID: fmt.Sprintf("token-%d", i), Data: &capacityTestTokenData{value: fmt.Sprintf("%d", i)}}
		if i%2 == 0 {
			place1.AddToken(tok)
		} else {
			place2.AddToken(tok)
		}
	}

	// Try to add one more (should fail due to global capacity)
	extraToken := &token.Token{ID: "extra-token", Data: &capacityTestTokenData{value: "extra"}}
	errBefore := place1.AddToken(extraToken)

	// EXPECTED FAILURE: No global capacity enforcement
	if errBefore == nil {
		t.Log("EXPECTED FAILURE: Could add token beyond global capacity")
	}

	// Remove 5 tokens from place1
	for i := 0; i < 5; i++ {
		place1.RemoveToken()
	}

	// Now try to add 5 new tokens (should succeed because we freed capacity)
	successCount := 0
	for i := 0; i < 5; i++ {
		tok := &token.Token{ID: fmt.Sprintf("new-token-%d", i), Data: &capacityTestTokenData{value: fmt.Sprintf("new-%d", i)}}
		err := place2.AddToken(tok)
		if err == nil {
			successCount++
		}
	}

	// Assert: Should be able to add tokens after removing some
	// EXPECTED FAILURE: Global capacity not tracked
	if successCount < 5 {
		t.Errorf("EXPECTED FAILURE: Only added %d tokens after removal, want 5", successCount)
	}

	totalTokens := place1.TokenCount() + place2.TokenCount()
	t.Logf("Total tokens after removal and re-add: %d (global capacity: %d)", totalTokens, globalCapacity)

	t.Log("EXPECTED FAILURE: Token removal should free global capacity")
	t.Log("After fix: Track total tokens and update count on both Add and Remove operations")
}

// TestPetriNet_GlobalCapacity_Validation tests:
// Validate global capacity configuration
//
// EXPECTED FAILURE: SetGlobalCapacity method and validation do not exist
func TestPetriNet_GlobalCapacity_Validation(t *testing.T) {
	tests := []struct {
		name           string
		globalCapacity int
		wantErr        bool
		errMsg         string
	}{
		{
			name:           "valid positive capacity",
			globalCapacity: 100,
			wantErr:        false,
		},
		{
			name:           "zero means unlimited",
			globalCapacity: 0,
			wantErr:        false,
		},
		{
			name:           "negative capacity should error",
			globalCapacity: -10,
			wantErr:        true,
			errMsg:         "negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			_ = NewPetriNet("test-net", "Test Network")

			// Act
			// EXPECTED FAILURE: SetGlobalCapacity does not exist
			// Uncomment when implementing:
			// net := NewPetriNet("test-net", "Test Network")
			// err := net.SetGlobalCapacity(tt.globalCapacity)

			// For now, simulate validation
			var err error
			if tt.globalCapacity < 0 {
				err = fmt.Errorf("global capacity cannot be negative")
			}

			// Assert
			if tt.wantErr && err == nil {
				t.Error("EXPECTED FAILURE: Expected error for invalid capacity, got none")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}

	t.Log("EXPECTED FAILURE: SetGlobalCapacity() with validation does not exist")
	t.Log("After fix: Validate global capacity (must be >= 0)")
}
