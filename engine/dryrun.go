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

package engine

import (
	"fmt"

	"github.com/jazzpetri/engine/petri"
)

// DryRun validates the workflow without executing it.
// Full Dry-Run Mode Implementation
//
// This method performs comprehensive structural validation:
//   - Checks for unreachable transitions (no input arcs or unreachable input places)
//   - Checks for unreachable places (no incoming arcs from reachable transitions)
//   - Detects potential deadlocks (places with no outgoing arcs that aren't explicitly end states)
//   - Checks for disconnected components
//   - Validates all arcs have valid source/target
//   - Checks place capacities are configured
//
// Returns a DryRunResult with:
//   - Valid: true if no critical issues found
//   - Issues: List of critical problems that prevent execution
//   - Warnings: List of potential problems that may cause issues
//   - Analysis: Structural metrics about the workflow
//   - Reachability: Map of which places/transitions are reachable
func (e *Engine) DryRun() *DryRunResult {
	result := &DryRunResult{
		Valid:        true,
		Issues:       []DryRunIssue{},
		Warnings:     []DryRunIssue{},
		Analysis:     make(map[string]interface{}),
		Reachability: make(map[string]bool),
	}

	net := e.net

	// Structural analysis
	result.Analysis["place_count"] = len(net.Places)
	result.Analysis["transition_count"] = len(net.Transitions)
	result.Analysis["arc_count"] = len(net.Arcs)

	// Check if net is resolved
	if !net.Resolved() {
		result.Valid = false
		result.Issues = append(result.Issues, DryRunIssue{
			Type:        "unresolved_net",
			Severity:    "error",
			Component:   "net",
			ComponentID: net.ID,
			Message:     "Petri net is not resolved - call net.Resolve() before validation",
		})
		return result
	}

	// 1. Check for unreachable transitions
	e.checkUnreachableTransitions(result)

	// 2. Check for unreachable places
	e.checkUnreachablePlaces(result)

	// 3. Detect potential deadlocks
	e.checkPotentialDeadlocks(result)

	// 4. Check for disconnected components
	e.checkDisconnectedComponents(result)

	// 5. Validate arcs
	e.validateArcs(result)

	// 6. Check place capacities
	e.checkPlaceCapacities(result)

	// Determine overall validity
	if len(result.Issues) > 0 {
		result.Valid = false
	}

	return result
}

// checkUnreachableTransitions identifies transitions that can never fire
func (e *Engine) checkUnreachableTransitions(result *DryRunResult) {
	for _, t := range e.net.Transitions {
		// A transition is unreachable if it has no input arcs
		if len(t.InputArcIDs) == 0 {
			result.Issues = append(result.Issues, DryRunIssue{
				Type:        "unreachable_transition",
				Severity:    "error",
				Component:   "transition",
				ComponentID: t.ID,
				Message:     fmt.Sprintf("Transition '%s' has no input arcs and can never fire", t.Name),
			})
		}

		// Check if all input places are unreachable
		allInputsUnreachable := true
		inputArcs := t.InputArcs()
		for _, arc := range inputArcs {
			place := arc.Source.(*petri.Place)
			// If any input place is reachable (has incoming arcs or initial tokens), transition might be reachable
			if len(place.IncomingArcIDs) > 0 || place.TokenCount() > 0 {
				allInputsUnreachable = false
				break
			}
		}

		if allInputsUnreachable && len(inputArcs) > 0 {
			result.Warnings = append(result.Warnings, DryRunIssue{
				Type:        "potentially_unreachable_transition",
				Severity:    "warning",
				Component:   "transition",
				ComponentID: t.ID,
				Message:     fmt.Sprintf("Transition '%s' may be unreachable (all input places have no incoming arcs and no initial tokens)", t.Name),
			})
		}
	}
}

// checkUnreachablePlaces identifies places that can never receive tokens
func (e *Engine) checkUnreachablePlaces(result *DryRunResult) {
	for _, p := range e.net.Places {
		// A place is unreachable if:
		// - It has no incoming arcs AND
		// - It has no initial tokens
		if len(p.IncomingArcIDs) == 0 && p.TokenCount() == 0 {
			result.Warnings = append(result.Warnings, DryRunIssue{
				Type:        "unreachable_place",
				Severity:    "warning",
				Component:   "place",
				ComponentID: p.ID,
				Message:     fmt.Sprintf("Place '%s' has no incoming arcs and no initial tokens - it will always be empty", p.Name),
			})
		}
	}
}

// checkPotentialDeadlocks identifies places that could cause deadlocks
func (e *Engine) checkPotentialDeadlocks(result *DryRunResult) {
	for _, p := range e.net.Places {
		// A place could cause deadlock if:
		// - It has outgoing arcs (so it's used as input) AND
		// - It has no incoming arcs (so it can't be replenished) AND
		// - It has initial tokens (so it's not just unreachable)
		// This means tokens can be consumed but never replenished
		if len(p.OutgoingArcIDs) > 0 && len(p.IncomingArcIDs) == 0 && p.TokenCount() > 0 {
			result.Issues = append(result.Issues, DryRunIssue{
				Type:        "potential_deadlock",
				Severity:    "error",
				Component:   "place",
				ComponentID: p.ID,
				Message:     fmt.Sprintf("Place '%s' has initial tokens but no incoming arcs - workflow may deadlock when tokens are consumed", p.Name),
			})
		}

		// Also check for places that accumulate tokens
		if len(p.IncomingArcIDs) > 0 && len(p.OutgoingArcIDs) == 0 {
			result.Warnings = append(result.Warnings, DryRunIssue{
				Type:        "token_sink",
				Severity:    "warning",
				Component:   "place",
				ComponentID: p.ID,
				Message:     fmt.Sprintf("Place '%s' has no outgoing arcs - tokens accumulate here and are never consumed (might be an end place)", p.Name),
			})
		}

		// Check for unbounded place growth
		// A place can grow unbounded if:
		// - It has no capacity limit (or very large capacity)
		// - Total incoming arc weight > total outgoing arc weight
		totalIncomingWeight := 0
		for _, arc := range p.IncomingArcs() {
			totalIncomingWeight += arc.Weight
		}

		totalOutgoingWeight := 0
		for _, arc := range p.OutgoingArcs() {
			totalOutgoingWeight += arc.Weight
		}

		if totalIncomingWeight > totalOutgoingWeight && len(p.OutgoingArcIDs) > 0 {
			result.Issues = append(result.Issues, DryRunIssue{
				Type:        "unbounded_place",
				Severity:    "error",
				Component:   "place",
				ComponentID: p.ID,
				Message:     fmt.Sprintf("Place '%s' may accumulate tokens unbounded (incoming weight %d > outgoing weight %d)", p.Name, totalIncomingWeight, totalOutgoingWeight),
			})
		}
	}
}

// checkDisconnectedComponents identifies isolated subgraphs
func (e *Engine) checkDisconnectedComponents(result *DryRunResult) {
	// Build connectivity map
	visited := make(map[string]bool)
	components := 0

	// DFS to find connected components
	var visit func(nodeID string, nodeType string)
	visit = func(nodeID string, nodeType string) {
		key := nodeType + ":" + nodeID
		if visited[key] {
			return
		}
		visited[key] = true

		if nodeType == "place" {
			// Visit connected transitions
			for _, p := range e.net.Places {
				if p.ID == nodeID {
					for _, arc := range p.OutgoingArcs() {
						t := arc.Target.(*petri.Transition)
						visit(t.ID, "transition")
					}
					for _, arc := range p.IncomingArcs() {
						t := arc.Source.(*petri.Transition)
						visit(t.ID, "transition")
					}
					break
				}
			}
		} else { // transition
			// Visit connected places
			for _, t := range e.net.Transitions {
				if t.ID == nodeID {
					for _, arc := range t.OutputArcs() {
						p := arc.Target.(*petri.Place)
						visit(p.ID, "place")
					}
					for _, arc := range t.InputArcs() {
						p := arc.Source.(*petri.Place)
						visit(p.ID, "place")
					}
					break
				}
			}
		}
	}

	// Find all connected components
	for _, p := range e.net.Places {
		key := "place:" + p.ID
		if !visited[key] {
			components++
			visit(p.ID, "place")
		}
	}

	for _, t := range e.net.Transitions {
		key := "transition:" + t.ID
		if !visited[key] {
			components++
			visit(t.ID, "transition")
		}
	}

	if components > 1 {
		result.Warnings = append(result.Warnings, DryRunIssue{
			Type:        "disconnected_components",
			Severity:    "warning",
			Component:   "net",
			ComponentID: e.net.ID,
			Message:     fmt.Sprintf("Workflow has %d disconnected components - some parts may be unreachable", components),
		})
	}

	result.Analysis["connected_components"] = components
}

// validateArcs checks that all arcs have valid source and target
func (e *Engine) validateArcs(result *DryRunResult) {
	for _, arc := range e.net.Arcs {
		// Check source exists
		if arc.Source == nil {
			result.Issues = append(result.Issues, DryRunIssue{
				Type:        "invalid_arc",
				Severity:    "error",
				Component:   "arc",
				ComponentID: arc.ID,
				Message:     "Arc has nil source",
			})
		}

		// Check target exists
		if arc.Target == nil {
			result.Issues = append(result.Issues, DryRunIssue{
				Type:        "invalid_arc",
				Severity:    "error",
				Component:   "arc",
				ComponentID: arc.ID,
				Message:     "Arc has nil target",
			})
		}

		// Check weight is positive
		if arc.Weight <= 0 {
			result.Issues = append(result.Issues, DryRunIssue{
				Type:        "invalid_arc",
				Severity:    "error",
				Component:   "arc",
				ComponentID: arc.ID,
				Message:     fmt.Sprintf("Arc has invalid weight: %d (must be > 0)", arc.Weight),
			})
		}
	}
}

// checkPlaceCapacities validates place capacity configuration
func (e *Engine) checkPlaceCapacities(result *DryRunResult) {
	for _, p := range e.net.Places {
		// Check capacity is set
		if p.Capacity <= 0 {
			result.Warnings = append(result.Warnings, DryRunIssue{
				Type:        "invalid_capacity",
				Severity:    "warning",
				Component:   "place",
				ComponentID: p.ID,
				Message:     fmt.Sprintf("Place '%s' has capacity <= 0 (%d) - will use default capacity", p.Name, p.Capacity),
			})
		}

		// Warn if current token count exceeds capacity
		if p.TokenCount() > p.Capacity {
			result.Issues = append(result.Issues, DryRunIssue{
				Type:        "capacity_exceeded",
				Severity:    "error",
				Component:   "place",
				ComponentID: p.ID,
				Message:     fmt.Sprintf("Place '%s' has %d tokens but capacity is only %d", p.Name, p.TokenCount(), p.Capacity),
			})
		}

		// Check if capacity seems too small for incoming arc weights
		totalIncomingWeight := 0
		for _, arc := range p.IncomingArcs() {
			totalIncomingWeight += arc.Weight
		}

		if totalIncomingWeight > p.Capacity {
			result.Warnings = append(result.Warnings, DryRunIssue{
				Type:        "small_capacity",
				Severity:    "warning",
				Component:   "place",
				ComponentID: p.ID,
				Message:     fmt.Sprintf("Place '%s' capacity (%d) is smaller than total incoming arc weight (%d) - may cause blocking", p.Name, p.Capacity, totalIncomingWeight),
			})
		}
	}
}
