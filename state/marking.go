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

// Package state provides state capture and persistence for Jazz3 workflow engine.
// It implements marking (state snapshots) and will support event logging for recovery.
package state

import (
	"fmt"
	"time"

	"github.com/jazzpetri/engine/clock"
	"github.com/jazzpetri/engine/petri"
	"github.com/jazzpetri/engine/token"
)

// Marking represents a snapshot of the current state of a Petri net.
// In Petri net terminology, a "marking" is the distribution of tokens across places.
// This represents the complete workflow state at a specific point in time.
type Marking struct {
	// Timestamp records when this snapshot was captured
	Timestamp time.Time

	// Places maps place IDs to their state at the time of capture
	Places map[string]PlaceState
}

// PlaceState represents the state of a single place at a point in time.
// It captures the tokens currently in the place and the place's capacity.
type PlaceState struct {
	// PlaceID is the unique identifier of the place
	PlaceID string

	// Tokens are the tokens currently in this place
	// This is a snapshot copy, not live references
	Tokens []*token.Token

	// Capacity is the maximum number of tokens this place can hold
	Capacity int
}

// NewMarking creates a marking from the current state of a Petri net.
// This performs a non-destructive snapshot of all places and their tokens.
// The net must be resolved before calling this function.
//
// Parameters:
//   - net: The Petri net to capture state from (must be resolved)
//   - clock: Clock for timestamp generation
//
// Returns an error if the net is not resolved.
func NewMarking(net *petri.PetriNet, clk clock.Clock) (*Marking, error) {
	// Check if net is resolved
	if !net.Resolved() {
		return nil, fmt.Errorf("net not resolved")
	}

	marking := &Marking{
		Timestamp: clk.Now(),
		Places:    make(map[string]PlaceState),
	}

	// Capture state from each place
	for placeID, place := range net.Places {
		// Get tokens non-destructively
		tokens := place.PeekTokens()

		marking.Places[placeID] = PlaceState{
			PlaceID:  placeID,
			Tokens:   tokens,
			Capacity: place.Capacity,
		}
	}

	return marking, nil
}

// TokenCount returns the total number of tokens across all places.
func (m *Marking) TokenCount() int {
	count := 0
	for _, placeState := range m.Places {
		count += len(placeState.Tokens)
	}
	return count
}

// PlaceTokenCount returns the number of tokens in a specific place.
// Returns 0 if the place doesn't exist in the marking.
func (m *Marking) PlaceTokenCount(placeID string) int {
	if placeState, exists := m.Places[placeID]; exists {
		return len(placeState.Tokens)
	}
	return 0
}

// GetTokens returns the tokens in a specific place.
// Returns nil if the place doesn't exist in the marking.
func (m *Marking) GetTokens(placeID string) []*token.Token {
	if placeState, exists := m.Places[placeID]; exists {
		return placeState.Tokens
	}
	return nil
}

// IsEmpty returns true if there are no tokens in any place.
func (m *Marking) IsEmpty() bool {
	return m.TokenCount() == 0
}

// Clone creates a deep copy of the marking.
// This creates an independent copy that can be modified without affecting the original.
func (m *Marking) Clone() *Marking {
	clone := &Marking{
		Timestamp: m.Timestamp,
		Places:    make(map[string]PlaceState),
	}

	// Deep copy each place state
	for placeID, placeState := range m.Places {
		// Copy tokens slice
		tokensCopy := make([]*token.Token, len(placeState.Tokens))
		copy(tokensCopy, placeState.Tokens)

		clone.Places[placeID] = PlaceState{
			PlaceID:  placeState.PlaceID,
			Tokens:   tokensCopy,
			Capacity: placeState.Capacity,
		}
	}

	return clone
}

// Equal returns true if two markings are equivalent.
// Markings are considered equal if they have the same places, capacities,
// and tokens (compared by ID).
func (m *Marking) Equal(other *Marking) bool {
	// Check number of places
	if len(m.Places) != len(other.Places) {
		return false
	}

	// Check each place
	for placeID, placeState := range m.Places {
		otherPlaceState, exists := other.Places[placeID]
		if !exists {
			return false
		}

		// Check place metadata
		if placeState.PlaceID != otherPlaceState.PlaceID {
			return false
		}
		if placeState.Capacity != otherPlaceState.Capacity {
			return false
		}

		// Check token count
		if len(placeState.Tokens) != len(otherPlaceState.Tokens) {
			return false
		}

		// Check each token (by ID)
		for i, tok := range placeState.Tokens {
			if tok.ID != otherPlaceState.Tokens[i].ID {
				return false
			}
		}
	}

	return true
}
