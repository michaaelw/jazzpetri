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

// Package verification provides formal verification for Petri net workflows.
// It uses state space exploration to prove safety properties about workflows
// BEFORE execution, catching structural errors like deadlocks and unbounded growth.
//
// The verifier builds a complete reachability graph (state space) from the
// initial marking of a Petri net, then checks safety properties over all
// reachable states.
//
// # Usage
//
//	v := verification.NewVerifier(net, 10000)
//	cert, err := v.GenerateCertificate()
//	if !cert.AllSatisfied() {
//	    // workflow has structural problems
//	}
//
// # State Space Exploration
//
// The verifier explores all reachable states using breadth-first search.
// A state is a Marking -- a snapshot of token counts at each place.
// The exploration stops when:
//   - All reachable states have been found (bounded net)
//   - The state count limit is reached (potentially unbounded net)
//
// # Safety Properties
//
// The verifier can check:
//   - Boundedness: no place accumulates unlimited tokens
//   - Deadlock freedom: every non-terminal state has at least one enabled transition
//   - Reachability: a target marking is reachable from the initial state
//   - Place invariants: linear inequality over place token counts holds in all states
//   - Mutual exclusion: two places never both have tokens simultaneously
package verification

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jazzpetri/engine/petri"
)

// Marking represents a snapshot of token counts at each place.
// This is a simplified representation for state space analysis:
// it captures token counts per place, not the actual token data.
// This abstraction enables efficient state deduplication.
type Marking map[string]int

// Key returns a canonical string key for this marking.
// The key is deterministic regardless of map iteration order,
// enabling reliable state deduplication in the state space.
func (m Marking) Key() string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s:%d", k, m[k])
	}
	return strings.Join(parts, ",")
}

// Copy returns a deep copy of the marking.
// The returned marking is independent of the original --
// modifications to one do not affect the other.
func (m Marking) Copy() Marking {
	c := make(Marking, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

// State represents a node in the state space graph.
// Each state corresponds to a unique reachable marking.
type State struct {
	// ID is the unique state identifier (assigned sequentially during exploration)
	ID int

	// Marking is the token count snapshot at each place for this state
	Marking Marking
}

// StateSpace represents the reachability graph of a Petri net.
// It is the complete set of reachable states and the transitions between them.
type StateSpace struct {
	// States is the list of all discovered states (indexed by State.ID)
	States []*State

	// Edges maps from state ID to the list of outgoing edges
	Edges map[int][]Edge

	// Initial is the ID of the initial state (always 0)
	Initial int

	// stateIndex maps marking key to state ID for deduplication
	stateIndex map[string]int
}

// Edge represents a transition firing in the state space.
// An edge connects two states and is labeled with the transition that was fired.
type Edge struct {
	// From is the source state ID
	From int

	// To is the target state ID
	To int

	// TransitionID is the ID of the petri net transition that was fired
	TransitionID string
}

// Verifier performs formal verification on Petri net workflows.
// It explores the complete reachable state space and checks safety properties.
//
// The verifier works on a snapshot of the net's initial marking at the time
// BuildStateSpace() is called. Subsequent token changes to the net do not
// affect the verification results.
type Verifier struct {
	// net is the Petri net to verify
	net *petri.PetriNet

	// maxStates limits state space exploration to prevent infinite loops
	// on unbounded or very large nets
	maxStates int
}

// NewVerifier creates a new verifier for the given Petri net.
// The net must be resolved (net.Resolve() called) before verification.
//
// Parameters:
//   - net: The Petri net to verify (must be resolved)
//   - maxStates: Maximum number of states to explore (0 = default 10000)
//
// Returns a configured Verifier ready for state space exploration.
func NewVerifier(net *petri.PetriNet, maxStates int) *Verifier {
	if maxStates <= 0 {
		maxStates = 10000
	}
	return &Verifier{
		net:       net,
		maxStates: maxStates,
	}
}

// BuildStateSpace explores all reachable states from the initial marking.
// Uses breadth-first search to build the complete reachability graph.
//
// The initial marking is captured from the net's current token counts.
// All reachable states are explored by firing enabled transitions and
// computing successor markings.
//
// Returns:
//   - *StateSpace: The reachability graph (may be partial if limit is hit)
//   - error: Non-nil if the state space limit was reached (net may be unbounded)
//
// The returned StateSpace is always non-nil, even if an error occurs.
// Partial state spaces can still be used for property checking, but results
// will only cover the explored portion of the state space.
func (v *Verifier) BuildStateSpace() (*StateSpace, error) {
	// Capture initial marking from current net state
	initial := v.currentMarking()

	ss := &StateSpace{
		States:     make([]*State, 0),
		Edges:      make(map[int][]Edge),
		stateIndex: make(map[string]int),
	}

	// Add initial state
	initialState := &State{ID: 0, Marking: initial}
	ss.States = append(ss.States, initialState)
	ss.stateIndex[initial.Key()] = 0
	ss.Initial = 0

	// BFS exploration queue (holds state IDs to process)
	queue := []int{0}

	for len(queue) > 0 {
		if len(ss.States) >= v.maxStates {
			return ss, fmt.Errorf("state space limit reached (%d states); net may be unbounded", v.maxStates)
		}

		currentID := queue[0]
		queue = queue[1:]
		current := ss.States[currentID]

		// Find all transitions enabled at this marking and compute successors
		for _, trans := range v.net.Transitions {
			if v.isEnabledAt(trans, current.Marking) {
				// Compute successor marking after firing this transition
				successor := v.fire(trans, current.Marking)

				key := successor.Key()
				if existingID, exists := ss.stateIndex[key]; exists {
					// State already discovered -- add edge to existing state
					ss.Edges[currentID] = append(ss.Edges[currentID], Edge{
						From:         currentID,
						To:           existingID,
						TransitionID: trans.ID,
					})
				} else {
					// New state discovered
					newID := len(ss.States)
					newState := &State{ID: newID, Marking: successor}
					ss.States = append(ss.States, newState)
					ss.stateIndex[key] = newID

					ss.Edges[currentID] = append(ss.Edges[currentID], Edge{
						From:         currentID,
						To:           newID,
						TransitionID: trans.ID,
					})

					queue = append(queue, newID)
				}
			}
		}
	}

	return ss, nil
}

// currentMarking captures the current token counts from the net.
// This is called once at the start of BuildStateSpace() to establish
// the initial marking for state space exploration.
func (v *Verifier) currentMarking() Marking {
	m := make(Marking, len(v.net.Places))
	for id, place := range v.net.Places {
		m[id] = place.TokenCount()
	}
	return m
}

// isEnabledAt checks if a transition is structurally enabled at a given marking.
// Only checks token availability vs arc weights (structural enablement).
// Does NOT evaluate guard functions -- guards may involve runtime state
// not available during static analysis.
//
// A transition is enabled at a marking if:
//  1. It has at least one input arc
//  2. Every input place has at least arc.Weight tokens
func (v *Verifier) isEnabledAt(t *petri.Transition, m Marking) bool {
	inputArcs := t.InputArcs()
	if len(inputArcs) == 0 {
		// Transitions without input arcs are source transitions (generators).
		// They are always enabled structurally, but we exclude them from
		// state space exploration since they would create infinite states.
		return false
	}

	for _, arc := range inputArcs {
		sourcePlace, ok := arc.Source.(*petri.Place)
		if !ok {
			// Source is not a place (should not happen in valid net)
			return false
		}
		if m[sourcePlace.ID] < arc.Weight {
			return false
		}
	}
	return true
}

// fire computes the successor marking after firing a transition at a given marking.
// The original marking is not modified -- a new Marking is returned.
//
// Firing semantics:
//  1. Remove arc.Weight tokens from each input place
//  2. Add arc.Weight tokens to each output place
func (v *Verifier) fire(t *petri.Transition, m Marking) Marking {
	result := m.Copy()

	// Remove tokens from input places
	for _, arc := range t.InputArcs() {
		if place, ok := arc.Source.(*petri.Place); ok {
			result[place.ID] -= arc.Weight
		}
	}

	// Add tokens to output places
	for _, arc := range t.OutputArcs() {
		if place, ok := arc.Target.(*petri.Place); ok {
			result[place.ID] += arc.Weight
		}
	}

	return result
}
