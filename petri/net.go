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
	"sync"
)

// PetriNet represents a complete Petri net graph.
// It contains places, transitions, and arcs, and provides methods for
// validation and resolution of ID references to direct pointers.
//
// The hybrid structure approach:
// - ID references (strings) are stored for serialization
// - Direct pointers are created by Resolve() for efficient execution
// - Validate() checks structural integrity
//
// Global Capacity Support
// Added GlobalCapacity and totalTokens fields to support limiting
// total tokens across all places in the network.
type PetriNet struct {
	// ID is the unique identifier for this Petri net
	ID string

	// Name is the human-readable name
	Name string

	// Places maps place IDs to place instances
	Places map[string]*Place

	// Transitions maps transition IDs to transition instances
	Transitions map[string]*Transition

	// Arcs maps arc IDs to arc instances
	Arcs map[string]*Arc

	// resolved indicates whether ID references have been converted to pointers
	resolved bool

	// GlobalCapacity is the maximum number of tokens allowed across all places.
	// 0 means unlimited (no global capacity limit).
	// Global capacity support
	GlobalCapacity int

	// totalTokens tracks the current number of tokens across all places.
	// Protected by tokenMutex for thread-safe access.
	// Global capacity support
	totalTokens int

	// tokenMutex protects totalTokens for concurrent access
	tokenMutex sync.Mutex

	// Metadata stores arbitrary key-value pairs for workflow metadata.
	// Workflow metadata support
	Metadata map[string]interface{}
}

// NewPetriNet creates a new Petri net with the specified ID and name.
// The net is initialized with empty maps for places, transitions, and arcs.
// Initializes Metadata map.
func NewPetriNet(id, name string) *PetriNet {
	return &PetriNet{
		ID:          id,
		Name:        name,
		Places:      make(map[string]*Place),
		Transitions: make(map[string]*Transition),
		Arcs:        make(map[string]*Arc),
		Metadata:    make(map[string]interface{}),
	}
}

// AddPlace adds a place to the net.
// Returns an error if a place with the same ID already exists.
// Sets the place's net reference for global capacity support.
func (pn *PetriNet) AddPlace(p *Place) error {
	if _, exists := pn.Places[p.ID]; exists {
		return fmt.Errorf("place %s already exists", p.ID)
	}
	p.net = pn // Set parent net reference
	pn.Places[p.ID] = p
	return nil
}

// AddTransition adds a transition to the net.
// Returns an error if a transition with the same ID already exists.
func (pn *PetriNet) AddTransition(t *Transition) error {
	if _, exists := pn.Transitions[t.ID]; exists {
		return fmt.Errorf("transition %s already exists", t.ID)
	}
	pn.Transitions[t.ID] = t
	return nil
}

// AddArc adds an arc to the net and updates the arc ID lists on places/transitions.
// Returns an error if an arc with the same ID already exists.
func (pn *PetriNet) AddArc(a *Arc) error {
	if _, exists := pn.Arcs[a.ID]; exists {
		return fmt.Errorf("arc %s already exists", a.ID)
	}
	pn.Arcs[a.ID] = a

	// Update arc ID lists on places and transitions (avoid duplicates)
	// For arcs from Place to Transition (P->T):
	if place, ok := pn.Places[a.SourceID]; ok {
		if trans, ok := pn.Transitions[a.TargetID]; ok {
			// Add to place's outgoing arcs if not already present
			if !contains(place.OutgoingArcIDs, a.ID) {
				place.OutgoingArcIDs = append(place.OutgoingArcIDs, a.ID)
			}
			// Add to transition's input arcs if not already present
			if !contains(trans.InputArcIDs, a.ID) {
				trans.InputArcIDs = append(trans.InputArcIDs, a.ID)
			}
		}
	}

	// For arcs from Transition to Place (T->P):
	if trans, ok := pn.Transitions[a.SourceID]; ok {
		if place, ok := pn.Places[a.TargetID]; ok {
			// Add to transition's output arcs if not already present
			if !contains(trans.OutputArcIDs, a.ID) {
				trans.OutputArcIDs = append(trans.OutputArcIDs, a.ID)
			}
			// Add to place's incoming arcs if not already present
			if !contains(place.IncomingArcIDs, a.ID) {
				place.IncomingArcIDs = append(place.IncomingArcIDs, a.ID)
			}
		}
	}

	return nil
}

// contains checks if a string slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Validate checks that the Petri net is well-formed.
// A net is valid if all arcs connect to existing places or transitions.
// Returns an error if validation fails.
func (pn *PetriNet) Validate() error {
	// Check that all arcs connect to existing places/transitions
	for _, arc := range pn.Arcs {
		sourceExists := false
		targetExists := false

		// Check if source exists
		if _, ok := pn.Places[arc.SourceID]; ok {
			sourceExists = true
		} else if _, ok := pn.Transitions[arc.SourceID]; ok {
			sourceExists = true
		}

		// Check if target exists
		if _, ok := pn.Places[arc.TargetID]; ok {
			targetExists = true
		} else if _, ok := pn.Transitions[arc.TargetID]; ok {
			targetExists = true
		}

		if !sourceExists {
			return fmt.Errorf("arc %s: source %s does not exist", arc.ID, arc.SourceID)
		}
		if !targetExists {
			return fmt.Errorf("arc %s: target %s does not exist", arc.ID, arc.TargetID)
		}
	}

	return nil
}

// Resolve converts ID references to direct pointers for execution.
// This method must be called after deserialization and before execution.
// It performs lazy resolution of all ID references to object pointers:
//  1. Validates arc endpoints exist 
//  2. Resolves arc source/target IDs to place/transition pointers
//  3. Resolves place incoming/outgoing arc IDs to arc pointers
//  4. Resolves transition input/output arc IDs to arc pointers
//
// Returns an error if any reference cannot be resolved or validation fails.
// Calling Resolve() multiple times is safe (idempotent).
func (pn *PetriNet) Resolve() error {
	if pn.resolved {
		return nil // Already resolved
	}

	// Validate arc endpoints before resolution
	// This catches structural errors early with clear error messages
	if err := pn.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Resolve arcs (convert SourceID/TargetID to Source/Target pointers)
	for _, arc := range pn.Arcs {
		// Resolve source
		if place, ok := pn.Places[arc.SourceID]; ok {
			arc.Source = place
		} else if trans, ok := pn.Transitions[arc.SourceID]; ok {
			arc.Source = trans
		} else {
			return fmt.Errorf("arc %s: source %s not found", arc.ID, arc.SourceID)
		}

		// Resolve target
		if place, ok := pn.Places[arc.TargetID]; ok {
			arc.Target = place
		} else if trans, ok := pn.Transitions[arc.TargetID]; ok {
			arc.Target = trans
		} else {
			return fmt.Errorf("arc %s: target %s not found", arc.ID, arc.TargetID)
		}

		arc.resolved = true
	}

	// Resolve places (convert arc IDs to arc pointers)
	for _, place := range pn.Places {
		// Resolve incoming arcs
		for _, arcID := range place.IncomingArcIDs {
			if arc, ok := pn.Arcs[arcID]; ok {
				place.incomingArcs = append(place.incomingArcs, arc)
			} else {
				return fmt.Errorf("place %s: incoming arc %s not found", place.ID, arcID)
			}
		}

		// Resolve outgoing arcs
		for _, arcID := range place.OutgoingArcIDs {
			if arc, ok := pn.Arcs[arcID]; ok {
				place.outgoingArcs = append(place.outgoingArcs, arc)
			} else {
				return fmt.Errorf("place %s: outgoing arc %s not found", place.ID, arcID)
			}
		}

		place.resolved = true
	}

	// Resolve transitions (convert arc IDs to arc pointers)
	for _, trans := range pn.Transitions {
		// Resolve input arcs
		for _, arcID := range trans.InputArcIDs {
			if arc, ok := pn.Arcs[arcID]; ok {
				trans.inputArcs = append(trans.inputArcs, arc)
			} else {
				return fmt.Errorf("transition %s: input arc %s not found", trans.ID, arcID)
			}
		}

		// Resolve output arcs
		for _, arcID := range trans.OutputArcIDs {
			if arc, ok := pn.Arcs[arcID]; ok {
				trans.outputArcs = append(trans.outputArcs, arc)
			} else {
				return fmt.Errorf("transition %s: output arc %s not found", trans.ID, arcID)
			}
		}

		// Set net ID for event logging
		trans.netID = pn.ID

		trans.resolved = true
	}

	pn.resolved = true
	return nil
}

// Resolved returns true if the Petri net has been resolved.
// A resolved net has all ID references converted to direct pointers.
func (pn *PetriNet) Resolved() bool {
	return pn.resolved
}

// GetPlace retrieves a place by its ID.
// Returns the place and true if found, nil and false otherwise.
// Returns an error if the net is nil or the ID is empty.
//
// This method provides safe access to places for:
//   - Dynamic workflow inspection
//   - State queries
//   - External integrations
//   - Debugging and tooling
func (pn *PetriNet) GetPlace(id string) (*Place, error) {
	if pn == nil {
		return nil, fmt.Errorf("cannot get place from nil net")
	}
	if id == "" {
		return nil, fmt.Errorf("place ID cannot be empty")
	}

	place, exists := pn.Places[id]
	if !exists {
		return nil, fmt.Errorf("place %s not found", id)
	}

	return place, nil
}

// GetTransition retrieves a transition by its ID.
// Returns the transition and nil error if found, nil and error otherwise.
// Returns an error if the net is nil or the ID is empty.
//
// This method provides safe access to transitions for:
//   - Dynamic workflow inspection
//   - Transition state queries
//   - External integrations
//   - Debugging and tooling
func (pn *PetriNet) GetTransition(id string) (*Transition, error) {
	if pn == nil {
		return nil, fmt.Errorf("cannot get transition from nil net")
	}
	if id == "" {
		return nil, fmt.Errorf("transition ID cannot be empty")
	}

	transition, exists := pn.Transitions[id]
	if !exists {
		return nil, fmt.Errorf("transition %s not found", id)
	}

	return transition, nil
}

// GetArc retrieves an arc by its ID.
// Returns the arc and nil error if found, nil and error otherwise.
// Returns an error if the net is nil or the ID is empty.
//
// This method provides safe access to arcs for:
//   - Dynamic workflow inspection
//   - Arc weight queries
//   - External integrations
//   - Debugging and tooling
func (pn *PetriNet) GetArc(id string) (*Arc, error) {
	if pn == nil {
		return nil, fmt.Errorf("cannot get arc from nil net")
	}
	if id == "" {
		return nil, fmt.Errorf("arc ID cannot be empty")
	}

	arc, exists := pn.Arcs[id]
	if !exists {
		return nil, fmt.Errorf("arc %s not found", id)
	}

	return arc, nil
}

// SetGlobalCapacity sets the global capacity for the Petri net.
// Global capacity support
//
// Parameters:
//   - capacity: Maximum number of tokens allowed across all places.
//     0 means unlimited (no global capacity limit).
//     Negative values return an error.
//
// Returns:
//   - error if capacity is negative
func (pn *PetriNet) SetGlobalCapacity(capacity int) error {
	if capacity < 0 {
		return fmt.Errorf("global capacity cannot be negative: %d", capacity)
	}
	pn.tokenMutex.Lock()
	defer pn.tokenMutex.Unlock()
	pn.GlobalCapacity = capacity
	return nil
}

// GetTotalTokens returns the current number of tokens across all places.
// Global capacity support
//
// This method is thread-safe and can be called concurrently with
// token add/remove operations.
//
// Returns:
//   - int: Current total number of tokens in the network
func (pn *PetriNet) GetTotalTokens() int {
	pn.tokenMutex.Lock()
	defer pn.tokenMutex.Unlock()
	return pn.totalTokens
}

// incrementTokenCount increments the total token count by the specified delta.
// Global capacity support
//
// This is an internal method called by Place.AddToken and Place.RemoveToken.
// It is thread-safe.
//
// Parameters:
//   - delta: Number of tokens to add (positive) or remove (negative)
func (pn *PetriNet) incrementTokenCount(delta int) {
	pn.tokenMutex.Lock()
	defer pn.tokenMutex.Unlock()
	pn.totalTokens += delta
}

// checkGlobalCapacity checks if adding numTokens would exceed global capacity.
// Global capacity support
//
// This is an internal method called by Place.AddToken before adding tokens.
// It is thread-safe.
//
// Parameters:
//   - numTokens: Number of tokens to be added
//
// Returns:
//   - error if adding tokens would exceed global capacity, nil otherwise
func (pn *PetriNet) checkGlobalCapacity(numTokens int) error {
	pn.tokenMutex.Lock()
	defer pn.tokenMutex.Unlock()

	// 0 means unlimited capacity
	if pn.GlobalCapacity == 0 {
		return nil
	}

	if pn.totalTokens+numTokens > pn.GlobalCapacity {
		return fmt.Errorf("adding %d tokens would exceed global capacity of %d (current: %d)",
			numTokens, pn.GlobalCapacity, pn.totalTokens)
	}

	return nil
}

// String returns a human-readable representation of the Petri net for debugging.
// String() Methods for Debugging
func (pn *PetriNet) String() string {
	placeCount := len(pn.Places)
	transCount := len(pn.Transitions)
	arcCount := len(pn.Arcs)
	resolvedStr := "not resolved"
	if pn.resolved {
		resolvedStr = "resolved"
	}
	return fmt.Sprintf("PetriNet[%s: \"%s\" places=%d transitions=%d arcs=%d %s]",
		pn.ID, pn.Name, placeCount, transCount, arcCount, resolvedStr)
}

// SetMetadata sets a metadata value for the specified key.
// Workflow metadata support
//
// Metadata allows storing arbitrary workflow-related information such as:
//   - Author and creation date
//   - Version information
//   - Business process details
//   - Integration-specific data
//
// Parameters:
//   - key: The metadata key (cannot be empty)
//   - value: The metadata value (can be any type)
func (pn *PetriNet) SetMetadata(key string, value interface{}) {
	if pn.Metadata == nil {
		pn.Metadata = make(map[string]interface{})
	}
	pn.Metadata[key] = value
}

// GetMetadata retrieves a metadata value for the specified key.
// Workflow metadata support
//
// Parameters:
//   - key: The metadata key to retrieve
//
// Returns:
//   - value: The metadata value if found
//   - ok: true if the key exists, false otherwise
func (pn *PetriNet) GetMetadata(key string) (interface{}, bool) {
	if pn.Metadata == nil {
		return nil, false
	}
	value, ok := pn.Metadata[key]
	return value, ok
}
