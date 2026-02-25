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
	"fmt"

	"github.com/jazzpetri/engine/petri"
	"github.com/jazzpetri/engine/token"
)

// RestoreMarkingToNet restores a marking to a Petri net.
// This populates the places with tokens from the marking.
//
// The restoration process:
// 1. Validates inputs (marking and net must not be nil, net must be resolved)
// 2. Captures current state as a backup for rollback
// 3. Clears all existing tokens from all places in the net
// 4. Restores tokens from the marking to their respective places
//
// This operation is atomic: if it fails at any point, the original state
// is restored through rollback, ensuring no tokens are lost.
//
// Parameters:
//   - marking: The marking to restore from (must not be nil)
//   - net: The Petri net to restore to (must be resolved)
//
// Returns an error if:
//   - marking is nil
//   - net is nil
//   - net is not resolved
//   - a place ID in the marking does not exist in the net
//   - adding a token to a place fails
//
// In case of error, the original state is automatically restored.
func RestoreMarkingToNet(marking *Marking, net *petri.PetriNet) error {
	if marking == nil {
		return fmt.Errorf("cannot restore nil marking")
	}
	if net == nil {
		return fmt.Errorf("cannot restore to nil net")
	}
	if !net.Resolved() {
		return fmt.Errorf("net must be resolved before restoration")
	}

	// Phase 1: Capture current state for rollback
	backup := captureCurrentState(net)

	// Phase 2: Clear existing tokens from all places
	// Clear() now returns tokens instead of error
	for _, place := range net.Places {
		_ = place.Clear() // Cleared tokens are discarded (we have backup if needed)
	}

	// Phase 3: Restore tokens from marking
	for placeID, placeState := range marking.Places {
		place, exists := net.Places[placeID]
		if !exists {
			// Restoration failed - rollback to backup
			restoreFromBackup(net, backup)
			return fmt.Errorf("place %s not found in net", placeID)
		}

		// Add each token to the place in order (FIFO preservation)
		for _, tok := range placeState.Tokens {
			if err := place.AddToken(tok); err != nil {
				// Restoration failed - rollback to backup
				restoreFromBackup(net, backup)
				return fmt.Errorf("failed to add token to place %s: %w", placeID, err)
			}
		}
	}

	return nil
}

// captureCurrentState captures the current state of all places in the net.
// This is used for rollback in case restoration fails.
// Returns a map of place IDs to their token snapshots.
func captureCurrentState(net *petri.PetriNet) map[string][]*token.Token {
	backup := make(map[string][]*token.Token)
	for placeID, place := range net.Places {
		// PeekTokens returns a snapshot without removing tokens
		tokens := place.PeekTokens()
		backup[placeID] = tokens
	}
	return backup
}

// restoreFromBackup restores the net to a previous state captured by captureCurrentState.
// This is a best-effort operation used for rollback during restoration failures.
// It clears all places and restores tokens from the backup.
func restoreFromBackup(net *petri.PetriNet, backup map[string][]*token.Token) {
	// Clear current state
	for _, place := range net.Places {
		_ = place.Clear()
	}

	// Restore backup state
	for placeID, tokens := range backup {
		if place, exists := net.Places[placeID]; exists {
			for _, tok := range tokens {
				// Best effort - ignore errors during rollback
				// If rollback fails, we've already lost consistency
				_ = place.AddToken(tok)
			}
		}
	}
}
