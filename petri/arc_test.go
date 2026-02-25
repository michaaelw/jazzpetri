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
	"github.com/jazzpetri/engine/token"
	"testing"
)

func TestNewArc(t *testing.T) {
	tests := []struct {
		name           string
		id             string
		sourceID       string
		targetID       string
		weight         int
		expectedWeight int
	}{
		{
			name:           "default weight when 0",
			id:             "a1",
			sourceID:       "p1",
			targetID:       "t1",
			weight:         0,
			expectedWeight: 1,
		},
		{
			name:           "default weight when negative",
			id:             "a2",
			sourceID:       "p2",
			targetID:       "t2",
			weight:         -5,
			expectedWeight: 1,
		},
		{
			name:           "custom weight",
			id:             "a3",
			sourceID:       "p3",
			targetID:       "t3",
			weight:         3,
			expectedWeight: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arc := NewArc(tt.id, tt.sourceID, tt.targetID, tt.weight)
			if arc == nil {
				t.Fatal("NewArc returned nil")
			}
			if arc.ID != tt.id {
				t.Errorf("expected ID %s, got %s", tt.id, arc.ID)
			}
			if arc.SourceID != tt.sourceID {
				t.Errorf("expected SourceID %s, got %s", tt.sourceID, arc.SourceID)
			}
			if arc.TargetID != tt.targetID {
				t.Errorf("expected TargetID %s, got %s", tt.targetID, arc.TargetID)
			}
			if arc.Weight != tt.expectedWeight {
				t.Errorf("expected Weight %d, got %d", tt.expectedWeight, arc.Weight)
			}
			if arc.resolved {
				t.Error("Arc should not be resolved initially")
			}
		})
	}
}

func TestArc_WithExpression(t *testing.T) {
	t.Run("set expression", func(t *testing.T) {
		arc := NewArc("a1", "p1", "t1", 1)

		// Create a simple expression that doubles a value
		expr := func(input *token.Token) *token.Token {
			if data, ok := input.Data.(*SimpleTokenData); ok {
				return &token.Token{
					ID:   input.ID + "_doubled",
					Data: &SimpleTokenData{Value: data.Value + data.Value},
				}
			}
			return input
		}

		result := arc.WithExpression(expr)
		if result != arc {
			t.Error("WithExpression should return the arc for chaining")
		}
		if arc.Expression == nil {
			t.Error("Expression should be set")
		}

		// Test the expression
		inputToken := &token.Token{
			ID:   "t1",
			Data: &SimpleTokenData{Value: "test"},
		}
		outputToken := arc.Expression(inputToken)
		if outputToken.ID != "t1_doubled" {
			t.Errorf("expected token ID t1_doubled, got %s", outputToken.ID)
		}
		if data, ok := outputToken.Data.(*SimpleTokenData); ok {
			if data.Value != "testtest" {
				t.Errorf("expected value testtest, got %s", data.Value)
			}
		} else {
			t.Error("output token data is not SimpleTokenData")
		}
	})

	t.Run("nil expression is identity", func(t *testing.T) {
		arc := NewArc("a1", "p1", "t1", 1)
		if arc.Expression != nil {
			t.Error("Expression should be nil by default")
		}
	})
}

func TestArc_Unresolved(t *testing.T) {
	arc := NewArc("a1", "p1", "t1", 1)

	if arc.Source != nil {
		t.Error("Source should be nil before resolve")
	}
	if arc.Target != nil {
		t.Error("Target should be nil before resolve")
	}
	if arc.resolved {
		t.Error("Arc should not be resolved")
	}
}

// IntTokenData is a test token type with integer value
type IntTokenData struct {
	Value int
}

func (i *IntTokenData) Type() string {
	return "int"
}

func TestArc_ExpressionChaining(t *testing.T) {
	t.Run("multiple arcs with different expressions", func(t *testing.T) {
		// Arc 1: Increment value
		arc1 := NewArc("a1", "p1", "t1", 1)
		arc1.WithExpression(func(input *token.Token) *token.Token {
			if data, ok := input.Data.(*IntTokenData); ok {
				return &token.Token{
					ID:   input.ID,
					Data: &IntTokenData{Value: data.Value + 1},
				}
			}
			return input
		})

		// Arc 2: Double value
		arc2 := NewArc("a2", "t1", "p2", 1)
		arc2.WithExpression(func(input *token.Token) *token.Token {
			if data, ok := input.Data.(*IntTokenData); ok {
				return &token.Token{
					ID:   input.ID,
					Data: &IntTokenData{Value: data.Value * 2},
				}
			}
			return input
		})

		// Test the chain: 5 -> +1 = 6 -> *2 = 12
		inputToken := &token.Token{
			ID:   "t1",
			Data: &IntTokenData{Value: 5},
		}

		// Apply arc1 expression
		tok1 := arc1.Expression(inputToken)
		if data, ok := tok1.Data.(*IntTokenData); ok {
			if data.Value != 6 {
				t.Errorf("after arc1: expected value 6, got %d", data.Value)
			}
		}

		// Apply arc2 expression
		tok2 := arc2.Expression(tok1)
		if data, ok := tok2.Data.(*IntTokenData); ok {
			if data.Value != 12 {
				t.Errorf("after arc2: expected value 12, got %d", data.Value)
			}
		}
	})
}
