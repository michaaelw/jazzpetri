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

	"github.com/jazzpetri/engine/token"
)

// ArcExpression transforms tokens as they traverse an arc.
// The function receives an input token and returns a transformed token.
// If the expression is nil, the token passes through unchanged (identity transformation).
type ArcExpression func(input *token.Token) *token.Token

// Arc connects a place to a transition or vice versa in a Petri net.
// Arcs have a weight (number of tokens required/produced) and an optional
// expression for token transformation.
//
// The hybrid structure stores ID references for serialization and direct
// pointers for execution. The Resolve() method converts IDs to pointers.
type Arc struct {
	// ID is the unique identifier for this arc
	ID string

	// Weight is the number of tokens required (input arc) or produced (output arc)
	// 0 or negative = default weight of 1
	Weight int

	// Expression transforms tokens as they traverse this arc
	// nil = identity (token passes through unchanged)
	Expression ArcExpression

	// SourceID stores the source node ID for serialization
	SourceID string

	// TargetID stores the target node ID for serialization
	TargetID string

	// Source is the resolved source node (Place or Transition)
	// Only available after PetriNet.Resolve() has been called
	Source interface{}

	// Target is the resolved target node (Place or Transition)
	// Only available after PetriNet.Resolve() has been called
	Target interface{}

	// resolved indicates whether ID references have been converted to pointers
	resolved bool
}

// NewArc creates a new arc connecting source to target with the specified weight.
// If weight is 0 or negative, a default weight of 1 is used.
// The arc is initialized with nil expression (identity) and unresolved pointers.
func NewArc(id, sourceID, targetID string, weight int) *Arc {
	if weight <= 0 {
		weight = 1
	}
	return &Arc{
		ID:       id,
		SourceID: sourceID,
		TargetID: targetID,
		Weight:   weight,
	}
}

// WithExpression sets an arc expression for token transformation.
// Returns the arc itself to allow method chaining.
// Example:
//
//	arc := NewArc("a1", "p1", "t1", 1).WithExpression(func(tok *token.Token) *token.Token {
//	    // Transform token
//	    return transformedToken
//	})
func (a *Arc) WithExpression(expr ArcExpression) *Arc {
	a.Expression = expr
	return a
}

// String returns a human-readable representation of the arc for debugging.
// String() Methods for Debugging
func (a *Arc) String() string {
	return fmt.Sprintf("Arc[%s: %s -> %s weight=%d]", a.ID, a.SourceID, a.TargetID, a.Weight)
}
