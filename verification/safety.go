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

// SafetyProperty represents a named safety property that can be verified
// against a state space. Properties are composable and can be combined
// into verification suites using VerifyProperties().
//
// SafetyProperty wraps a check function that receives the verifier and
// state space, enabling access to all verification methods.
type SafetyProperty struct {
	// Name is the unique identifier for this property
	Name string

	// Description is a human-readable explanation of what the property checks
	Description string

	// Check is the function that performs the actual verification
	Check func(v *Verifier, ss *StateSpace) VerificationResult
}

// NewBoundednessProperty creates a property that checks k-boundedness for a specific place.
// The property is satisfied if the named place never holds more than maxTokens tokens
// in any reachable state.
//
// This is useful for checking capacity constraints:
//   - Queue length limits
//   - Resource pool sizes
//   - Buffer capacity
//
// Example:
//
//	prop := NewBoundednessProperty("queue_bound", "request_queue", 10)
func NewBoundednessProperty(name, placeID string, maxTokens int) SafetyProperty {
	return SafetyProperty{
		Name:        name,
		Description: fmt.Sprintf("place %s never exceeds %d tokens", placeID, maxTokens),
		Check: func(v *Verifier, ss *StateSpace) VerificationResult {
			for _, state := range ss.States {
				if state.Marking[placeID] > maxTokens {
					path := v.findPath(ss, ss.Initial, state.ID)
					return VerificationResult{
						Property: name,
						Satisfied: false,
						Message: fmt.Sprintf("place %s has %d tokens (max allowed: %d) at state %d",
							placeID, state.Marking[placeID], maxTokens, state.ID),
						Witness:       path,
						StatesChecked: len(ss.States),
					}
				}
			}
			return VerificationResult{
				Property:      name,
				Satisfied:     true,
				Message:       fmt.Sprintf("place %s stays within %d tokens across all states", placeID, maxTokens),
				StatesChecked: len(ss.States),
			}
		},
	}
}

// NewReachabilityProperty creates a property that checks if a specific place
// can receive at least one token from the initial state.
//
// This is useful for checking that:
//   - Success paths are actually reachable
//   - Terminal states can be reached
//   - No workflow paths are structurally dead
//
// Example:
//
//	prop := NewReachabilityProperty("success_reachable", "success_place")
func NewReachabilityProperty(name, placeID string) SafetyProperty {
	return SafetyProperty{
		Name:        name,
		Description: fmt.Sprintf("place %s is reachable", placeID),
		Check: func(v *Verifier, ss *StateSpace) VerificationResult {
			target := Marking{placeID: 1}
			result := v.CheckReachability(ss, target)
			result.Property = name
			return result
		},
	}
}

// NewMutualExclusionProperty creates a property that checks two places never
// both have tokens simultaneously in any reachable state.
//
// This is useful for verifying:
//   - Critical section enforcement
//   - Exclusive resource ownership
//   - OR-split correctness (token follows only one branch)
//
// Example:
//
//	prop := NewMutualExclusionProperty("exclusive_branches", "branch_a", "branch_b")
func NewMutualExclusionProperty(name, placeA, placeB string) SafetyProperty {
	return SafetyProperty{
		Name:        name,
		Description: fmt.Sprintf("places %s and %s are mutually exclusive", placeA, placeB),
		Check: func(v *Verifier, ss *StateSpace) VerificationResult {
			result := v.CheckMutualExclusion(ss, placeA, placeB)
			result.Property = name
			return result
		},
	}
}

// NewDeadlockFreedomProperty creates a property that verifies the workflow
// has no deadlock states (non-terminal states with no enabled transitions).
//
// This is the most fundamental safety property for workflow correctness:
// a deadlocked workflow is stuck and can never complete.
//
// Example:
//
//	prop := NewDeadlockFreedomProperty("no_deadlocks")
func NewDeadlockFreedomProperty(name string) SafetyProperty {
	return SafetyProperty{
		Name:        name,
		Description: "workflow has no deadlocks",
		Check: func(v *Verifier, ss *StateSpace) VerificationResult {
			result := v.CheckDeadlockFreedom(ss)
			result.Property = name
			return result
		},
	}
}

// NewPlaceInvariantProperty creates a property that checks a linear inequality
// over place token counts holds in all reachable states.
//
// The invariant is: sum(coefficients[place] * tokens[place]) <= bound
//
// Example -- verify token conservation (exactly 1 token in the workflow):
//
//	prop := NewPlaceInvariantProperty(
//	    "token_conservation",
//	    map[string]int{"start": 1, "middle": 1, "end": 1},
//	    1,
//	)
func NewPlaceInvariantProperty(name string, coefficients map[string]int, bound int) SafetyProperty {
	return SafetyProperty{
		Name:        name,
		Description: fmt.Sprintf("place invariant holds (sum <= %d)", bound),
		Check: func(v *Verifier, ss *StateSpace) VerificationResult {
			result := v.CheckPlaceInvariant(ss, coefficients, bound)
			result.Property = name
			return result
		},
	}
}

// VerifyProperties runs a set of safety properties against the state space.
// It builds the state space once and then evaluates all properties against it.
// This is more efficient than calling individual Check methods separately
// when multiple properties need to be verified.
//
// The returned ProofCertificate contains results for all properties.
// The Bounded and DeadlockFree summary fields are derived from the properties:
//   - Bounded is true if the state space exploration completed without hitting the limit
//   - DeadlockFree is true if any property named "deadlock_freedom" is satisfied
//
// Returns the certificate and any error from state space exploration.
// A state space limit error does not prevent properties from being checked
// on the explored portion.
func (v *Verifier) VerifyProperties(properties []SafetyProperty) (*ProofCertificate, error) {
	ss, err := v.BuildStateSpace()

	cert := &ProofCertificate{
		WorkflowID: v.net.ID,
		StateCount: len(ss.States),
		Bounded:    err == nil, // Bounded only if exploration completed without limit error
	}

	// Count total edges
	edgeCount := 0
	for _, edges := range ss.Edges {
		edgeCount += len(edges)
	}
	cert.EdgeCount = edgeCount

	// Evaluate all properties against the state space
	for _, prop := range properties {
		result := prop.Check(v, ss)
		cert.Properties = append(cert.Properties, result)
	}

	// Derive summary fields from property results
	cert.DeadlockFree = true
	for _, r := range cert.Properties {
		if !r.Satisfied {
			// Check for deadlock-related property names
			if r.Property == "deadlock_freedom" || r.Property == "no_deadlocks" {
				cert.DeadlockFree = false
			}
		}
	}

	return cert, err
}
