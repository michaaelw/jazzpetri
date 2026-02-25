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

package petri_test

import (
	"fmt"
	"github.com/jazzpetri/engine/petri"
	"github.com/jazzpetri/engine/token"
	execContext "github.com/jazzpetri/engine/context"
)

// ExamplePetriNet_simpleWorkflow demonstrates a complete workflow using the petri package
func ExamplePetriNet_simpleWorkflow() {
	// Create a Petri net: Input -> Process -> Output
	net := petri.NewPetriNet("workflow1", "Simple Workflow")

	// Create places
	input := petri.NewPlace("p1", "Input", 10)
	output := petri.NewPlace("p2", "Output", 10)

	// Create transition
	process := petri.NewTransition("t1", "Process")

	// Create arcs
	inputArc := petri.NewArc("a1", "p1", "t1", 1)
	outputArc := petri.NewArc("a2", "t1", "p2", 1)

	// Set up ID references for serialization
	input.OutgoingArcIDs = []string{"a1"}
	process.InputArcIDs = []string{"a1"}
	process.OutputArcIDs = []string{"a2"}
	output.IncomingArcIDs = []string{"a2"}

	// Add to net
	net.AddPlace(input)
	net.AddPlace(output)
	net.AddTransition(process)
	net.AddArc(inputArc)
	net.AddArc(outputArc)

	// Validate structure
	if err := net.Validate(); err != nil {
		panic(err)
	}

	// Resolve ID references to pointers
	if err := net.Resolve(); err != nil {
		panic(err)
	}

	// Add token to input
	tok := &token.Token{
		ID:   "tok1",
		Data: &token.IntToken{Value: 42},
	}
	input.AddToken(tok)

	// Check transition state
	fmt.Printf("Input tokens: %d\n", input.TokenCount())
	fmt.Printf("Output tokens: %d\n", output.TokenCount())
	ctx := &execContext.ExecutionContext{}
	enabled, _ := process.IsEnabled(ctx)
	fmt.Printf("Transition enabled: %v\n", enabled)

	// Output:
	// Input tokens: 1
	// Output tokens: 0
	// Transition enabled: true
}

// ExampleTransition_withGuard demonstrates using guards for conditional firing
func ExampleTransition_withGuard() {
	// Create a place and transition with guard
	place := petri.NewPlace("p1", "Input", 10)
	trans := petri.NewTransition("t1", "Process")

	// Set guard that only allows even numbers
	trans.WithGuard(func(ctx *execContext.ExecutionContext, tokens []*token.Token) (bool, error) {
		for _, tok := range tokens {
			if intData, ok := tok.Data.(*token.IntToken); ok {
				if intData.Value%2 != 0 {
					return false, nil // Reject odd numbers
				}
			}
		}
		return true, nil
	})

	// Create arc and set up IDs
	arc := petri.NewArc("a1", "p1", "t1", 1)
	place.OutgoingArcIDs = []string{"a1"}
	trans.InputArcIDs = []string{"a1"}

	// Create net and resolve
	net := petri.NewPetriNet("net1", "Guard Example")
	net.AddPlace(place)
	net.AddTransition(trans)
	net.AddArc(arc)
	net.Resolve()

	// Add even number - should be enabled
	evenToken := &token.Token{ID: "t1", Data: &token.IntToken{Value: 42}}
	place.AddToken(evenToken)
	ctx := &execContext.ExecutionContext{}
	enabled1, _ := trans.IsEnabled(ctx)
	fmt.Printf("With even number: %v\n", enabled1)

	// Remove and add odd number - should not be enabled
	place.RemoveToken()
	oddToken := &token.Token{ID: "t2", Data: &token.IntToken{Value: 43}}
	place.AddToken(oddToken)
	enabled2, _ := trans.IsEnabled(ctx)
	fmt.Printf("With odd number: %v\n", enabled2)

	// Output:
	// With even number: true
	// With odd number: false
}

// ExampleArc_withExpression demonstrates token transformation via arc expressions
func ExampleArc_withExpression() {
	// Create arc with expression that doubles the value
	arc := petri.NewArc("a1", "p1", "t1", 1)
	arc.WithExpression(func(tok *token.Token) *token.Token {
		if intData, ok := tok.Data.(*token.IntToken); ok {
			return &token.Token{
				ID:   tok.ID + "_doubled",
				Data: &token.IntToken{Value: intData.Value * 2},
			}
		}
		return tok
	})

	// Apply expression
	inputToken := &token.Token{
		ID:   "tok1",
		Data: &token.IntToken{Value: 21},
	}
	outputToken := arc.Expression(inputToken)

	if intData, ok := outputToken.Data.(*token.IntToken); ok {
		fmt.Printf("Input: %d, Output: %d\n", 21, intData.Value)
	}

	// Output:
	// Input: 21, Output: 42
}

// ExamplePlace_concurrency demonstrates thread-safe token operations
func ExamplePlace_concurrency() {
	place := petri.NewPlace("p1", "Concurrent Place", 100)

	// Add tokens
	for i := 0; i < 5; i++ {
		tok := &token.Token{
			ID:   fmt.Sprintf("tok%d", i),
			Data: &token.IntToken{Value: i},
		}
		place.AddToken(tok)
	}

	fmt.Printf("Tokens in place: %d\n", place.TokenCount())

	// Remove tokens (FIFO order)
	for i := 0; i < 3; i++ {
		tok, _ := place.RemoveToken()
		if intData, ok := tok.Data.(*token.IntToken); ok {
			fmt.Printf("Removed token with value: %d\n", intData.Value)
		}
	}

	fmt.Printf("Remaining tokens: %d\n", place.TokenCount())

	// Output:
	// Tokens in place: 5
	// Removed token with value: 0
	// Removed token with value: 1
	// Removed token with value: 2
	// Remaining tokens: 2
}
