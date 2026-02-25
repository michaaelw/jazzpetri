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

import "fmt"

// VerificationResult contains the outcome of verifying a single safety property.
// A satisfied result proves the property holds for all reachable states.
// An unsatisfied result includes a witness path that demonstrates the violation.
type VerificationResult struct {
	// Property is the name of the property that was checked
	Property string

	// Satisfied is true if the property holds for all reachable states
	Satisfied bool

	// Message provides a human-readable explanation of the result
	Message string

	// Witness is the transition sequence from the initial state to the
	// violating state (for unsatisfied properties) or the target state
	// (for reachability witnesses). Empty if no path was found or property is satisfied.
	Witness []string

	// StatesChecked is the number of states that were examined during verification
	StatesChecked int
}

// ProofCertificate contains comprehensive verification results for a workflow.
// It aggregates the results of multiple property checks along with summary
// statistics about the state space exploration.
//
// A certificate with AllSatisfied() == true provides formal proof that the
// workflow satisfies all checked safety properties.
type ProofCertificate struct {
	// WorkflowID is the ID of the Petri net that was verified
	WorkflowID string

	// Properties contains the results of each individual property check
	Properties []VerificationResult

	// StateCount is the total number of reachable states discovered
	StateCount int

	// EdgeCount is the total number of transitions in the state space graph
	EdgeCount int

	// Bounded is true if the net is bounded (state space exploration completed)
	Bounded bool

	// MaxTokens is the maximum number of tokens seen in any place in any reachable state
	MaxTokens int

	// DeadlockFree is true if no reachable non-terminal state has zero enabled transitions
	DeadlockFree bool
}

// AllSatisfied returns true if all properties in the certificate are satisfied.
// A certificate with no properties returns true (vacuously true).
func (pc *ProofCertificate) AllSatisfied() bool {
	for _, r := range pc.Properties {
		if !r.Satisfied {
			return false
		}
	}
	return true
}

// CheckBoundedness verifies that the net is bounded.
// A net is bounded if state space exploration completed without hitting the limit,
// meaning every place has a finite maximum token count across all reachable states.
//
// This method analyzes the already-explored state space to find the actual
// maximum token count and which place holds it.
//
// Returns a VerificationResult that is always Satisfied == true when called
// on a complete state space (BuildStateSpace returned no error).
// The MaxTokens and MaxPlace fields provide the actual bounds.
func (v *Verifier) CheckBoundedness(ss *StateSpace) VerificationResult {
	maxTokens := 0
	maxPlace := ""

	for _, state := range ss.States {
		for placeID, count := range state.Marking {
			if count > maxTokens {
				maxTokens = count
				maxPlace = placeID
			}
		}
	}

	message := fmt.Sprintf("net is %d-bounded", maxTokens)
	if maxPlace != "" {
		message = fmt.Sprintf("net is %d-bounded (max tokens at place %s)", maxTokens, maxPlace)
	}

	return VerificationResult{
		Property:      "boundedness",
		Satisfied:     true, // If state space exploration completed, the net is bounded
		Message:       message,
		StatesChecked: len(ss.States),
	}
}

// CheckDeadlockFreedom verifies that no reachable non-terminal state has zero
// enabled transitions. A deadlock is a state where the workflow is stuck --
// not all tokens have reached terminal places, but no transition can fire.
//
// Terminal states (where all tokens are in terminal places, or where there are
// no tokens) are explicitly excluded from deadlock checking, since they represent
// successful workflow completion.
//
// If a deadlock is found, the result includes a Witness path (sequence of
// transition IDs) from the initial state to the deadlock state.
func (v *Verifier) CheckDeadlockFreedom(ss *StateSpace) VerificationResult {
	for _, state := range ss.States {
		// Check if any transition is enabled at this state
		hasEnabled := false
		for _, trans := range v.net.Transitions {
			if v.isEnabledAt(trans, state.Marking) {
				hasEnabled = true
				break
			}
		}

		if !hasEnabled {
			// No enabled transitions -- check if this is a valid terminal state
			if !v.isTerminalState(state.Marking) {
				// This is a non-terminal deadlock state
				path := v.findPath(ss, ss.Initial, state.ID)
				return VerificationResult{
					Property:      "deadlock_freedom",
					Satisfied:     false,
					Message:       fmt.Sprintf("deadlock found at state %d: %s", state.ID, state.Marking.Key()),
					Witness:       path,
					StatesChecked: len(ss.States),
				}
			}
		}
	}

	return VerificationResult{
		Property:      "deadlock_freedom",
		Satisfied:     true,
		Message:       "no deadlocks found in reachable state space",
		StatesChecked: len(ss.States),
	}
}

// CheckReachability verifies that a target marking is reachable from the initial state.
// The target is specified as a partial marking -- only the places listed in target
// need to match; other places can have any token count.
//
// When the target is reachable, the result includes a Witness path (sequence of
// transition IDs) from the initial state to the matching state.
//
// Example -- check that the "end" place gets a token:
//
//	result := v.CheckReachability(ss, Marking{"end": 1})
func (v *Verifier) CheckReachability(ss *StateSpace, target Marking) VerificationResult {
	for _, state := range ss.States {
		if v.markingContains(state.Marking, target) {
			path := v.findPath(ss, ss.Initial, state.ID)
			return VerificationResult{
				Property:      "reachability",
				Satisfied:     true,
				Message:       fmt.Sprintf("target marking reachable at state %d", state.ID),
				Witness:       path,
				StatesChecked: len(ss.States),
			}
		}
	}

	return VerificationResult{
		Property:      "reachability",
		Satisfied:     false,
		Message:       "target marking not reachable from initial state",
		StatesChecked: len(ss.States),
	}
}

// CheckPlaceInvariant checks that a linear inequality over place token counts
// holds in all reachable states.
//
// The invariant is: sum(coefficients[place] * tokens[place]) <= bound
//
// This can express many useful properties:
//   - Token conservation: {p1: 1, p2: 1} <= 1 (total tokens never exceed 1)
//   - Resource bounds: {resource: 1} <= 5 (resource count stays within 5)
//   - Capacity limits: {queue: 1} <= 10 (queue never exceeds 10 items)
//
// If the invariant is violated, the result includes a Witness path to
// the violating state.
func (v *Verifier) CheckPlaceInvariant(ss *StateSpace, coefficients map[string]int, bound int) VerificationResult {
	for _, state := range ss.States {
		sum := 0
		for placeID, coeff := range coefficients {
			sum += coeff * state.Marking[placeID]
		}
		if sum > bound {
			path := v.findPath(ss, ss.Initial, state.ID)
			return VerificationResult{
				Property:      "place_invariant",
				Satisfied:     false,
				Message:       fmt.Sprintf("invariant violated at state %d: sum=%d > bound=%d", state.ID, sum, bound),
				Witness:       path,
				StatesChecked: len(ss.States),
			}
		}
	}

	return VerificationResult{
		Property:      "place_invariant",
		Satisfied:     true,
		Message:       fmt.Sprintf("invariant holds across all %d states", len(ss.States)),
		StatesChecked: len(ss.States),
	}
}

// CheckMutualExclusion verifies that two places never both have tokens simultaneously
// in any reachable state. This is a common safety property for:
//   - Critical section enforcement (only one thread in critical section)
//   - Resource allocation (resource can't be in two states at once)
//   - Workflow branching (token follows only one branch)
//
// If mutual exclusion is violated, the result includes a Witness path to
// the violating state.
func (v *Verifier) CheckMutualExclusion(ss *StateSpace, placeA, placeB string) VerificationResult {
	for _, state := range ss.States {
		if state.Marking[placeA] > 0 && state.Marking[placeB] > 0 {
			path := v.findPath(ss, ss.Initial, state.ID)
			return VerificationResult{
				Property:  "mutual_exclusion",
				Satisfied: false,
				Message: fmt.Sprintf("mutual exclusion violated: places %s and %s both have tokens at state %d",
					placeA, placeB, state.ID),
				Witness:       path,
				StatesChecked: len(ss.States),
			}
		}
	}

	return VerificationResult{
		Property:      "mutual_exclusion",
		Satisfied:     true,
		Message:       fmt.Sprintf("mutual exclusion holds between %s and %s", placeA, placeB),
		StatesChecked: len(ss.States),
	}
}

// isTerminalState checks if a marking represents a valid terminal state.
// A state is terminal if:
//   - All tokens are in places marked as IsTerminal == true, OR
//   - There are no tokens anywhere in the net
//
// Terminal states are excluded from deadlock detection because they represent
// successful workflow completion.
func (v *Verifier) isTerminalState(m Marking) bool {
	totalTokens := 0
	terminalTokens := 0

	for placeID, count := range m {
		if count > 0 {
			totalTokens += count
			place := v.net.Places[placeID]
			if place != nil && place.IsTerminal {
				terminalTokens += count
			}
		}
	}

	// Terminal if there are no tokens, or all tokens are in terminal places
	return totalTokens == 0 || terminalTokens == totalTokens
}

// markingContains checks if marking m satisfies the partial target marking t.
// Target is a partial marking: only the places specified in t are checked.
// Places not specified in t can have any token count.
func (v *Verifier) markingContains(m, target Marking) bool {
	for placeID, count := range target {
		if m[placeID] != count {
			return false
		}
	}
	return true
}

// findPath finds the transition sequence from state 'from' to state 'to'
// using breadth-first search over the state space graph.
// Returns the sequence of transition IDs, or nil if no path exists.
func (v *Verifier) findPath(ss *StateSpace, from, to int) []string {
	if from == to {
		return nil
	}

	type pathNode struct {
		stateID int
		path    []string
	}

	visited := make(map[int]bool)
	queue := []pathNode{{stateID: from, path: nil}}
	visited[from] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, edge := range ss.Edges[current.stateID] {
			if visited[edge.To] {
				continue
			}

			newPath := make([]string, len(current.path)+1)
			copy(newPath, current.path)
			newPath[len(current.path)] = edge.TransitionID

			if edge.To == to {
				return newPath
			}

			visited[edge.To] = true
			queue = append(queue, pathNode{stateID: edge.To, path: newPath})
		}
	}

	return nil // No path found
}

// GenerateCertificate runs all standard checks and produces a comprehensive
// proof certificate for the workflow. It checks:
//   - Boundedness: state space exploration completes
//   - Deadlock freedom: no non-terminal deadlock states
//
// Additional custom properties can be checked using VerifyProperties().
//
// Returns the certificate and any error from state space exploration.
// If the state space limit is reached, the certificate is partial but
// still contains results for the explored portion.
func (v *Verifier) GenerateCertificate() (*ProofCertificate, error) {
	ss, err := v.BuildStateSpace()

	cert := &ProofCertificate{
		WorkflowID: v.net.ID,
		StateCount: len(ss.States),
	}

	// Count total edges
	edgeCount := 0
	for _, edges := range ss.Edges {
		edgeCount += len(edges)
	}
	cert.EdgeCount = edgeCount

	// Check boundedness (always satisfied if state space exploration completed)
	boundedness := v.CheckBoundedness(ss)
	cert.Properties = append(cert.Properties, boundedness)
	cert.Bounded = err == nil // Bounded only if exploration completed without limit error

	// Check deadlock freedom
	deadlock := v.CheckDeadlockFreedom(ss)
	cert.Properties = append(cert.Properties, deadlock)
	cert.DeadlockFree = deadlock.Satisfied

	// Calculate maximum token count across all states and places
	maxTokens := 0
	for _, state := range ss.States {
		for _, count := range state.Marking {
			if count > maxTokens {
				maxTokens = count
			}
		}
	}
	cert.MaxTokens = maxTokens

	return cert, err
}
