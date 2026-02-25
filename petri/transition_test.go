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
	"github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/token"
	"testing"
)

func TestNewTransition(t *testing.T) {
	trans := NewTransition("t1", "Transition1")
	if trans == nil {
		t.Fatal("NewTransition returned nil")
	}
	if trans.ID != "t1" {
		t.Errorf("expected ID t1, got %s", trans.ID)
	}
	if trans.Name != "Transition1" {
		t.Errorf("expected Name Transition1, got %s", trans.Name)
	}
	if trans.Guard != nil {
		t.Error("Guard should be nil by default")
	}
	if trans.InputArcIDs == nil {
		t.Error("InputArcIDs should be initialized")
	}
	if trans.OutputArcIDs == nil {
		t.Error("OutputArcIDs should be initialized")
	}
	if trans.resolved {
		t.Error("Transition should not be resolved initially")
	}
}

func TestTransition_WithGuard(t *testing.T) {
	trans := NewTransition("t1", "Transition1")

	// Create a guard that checks if all tokens have value "pass"
	guard := func(ctx *context.ExecutionContext, tokens []*token.Token) (bool, error) {
		for _, tok := range tokens {
			if data, ok := tok.Data.(*SimpleTokenData); ok {
				if data.Value != "pass" {
					return false, nil
				}
			} else {
				return false, nil
			}
		}
		return true, nil
	}

	result := trans.WithGuard(guard)
	if result != trans {
		t.Error("WithGuard should return the transition for chaining")
	}
	if trans.Guard == nil {
		t.Error("Guard should be set")
	}

	// Test the guard
	ctx := context.NewExecutionContext(nil, nil, nil)
	passingTokens := []*token.Token{
		{ID: "t1", Data: &SimpleTokenData{Value: "pass"}},
		{ID: "t2", Data: &SimpleTokenData{Value: "pass"}},
	}
	if enabled, err := trans.Guard(ctx, passingTokens); err != nil || !enabled {
		t.Error("Guard should pass for valid tokens")
	}

	failingTokens := []*token.Token{
		{ID: "t1", Data: &SimpleTokenData{Value: "pass"}},
		{ID: "t2", Data: &SimpleTokenData{Value: "fail"}},
	}
	if enabled, err := trans.Guard(ctx, failingTokens); err != nil || enabled {
		t.Error("Guard should fail for invalid tokens")
	}
}

func TestTransition_ArcAccessors(t *testing.T) {
	trans := NewTransition("t1", "Transition1")

	t.Run("input arcs before resolve", func(t *testing.T) {
		arcs := trans.InputArcs()
		if arcs == nil {
			t.Error("InputArcs should return empty slice, not nil")
		}
		if len(arcs) != 0 {
			t.Errorf("expected 0 input arcs, got %d", len(arcs))
		}
	})

	t.Run("output arcs before resolve", func(t *testing.T) {
		arcs := trans.OutputArcs()
		if arcs == nil {
			t.Error("OutputArcs should return empty slice, not nil")
		}
		if len(arcs) != 0 {
			t.Errorf("expected 0 output arcs, got %d", len(arcs))
		}
	})
}

func TestTransition_IsEnabled(t *testing.T) {
	ctx := context.NewExecutionContext(nil, nil, nil)

	t.Run("not enabled before resolve", func(t *testing.T) {
		trans := NewTransition("t1", "Transition1")
		if enabled, _ := trans.IsEnabled(ctx); enabled {
			t.Error("Transition should not be enabled before resolve")
		}
	})

	t.Run("enabled with sufficient tokens and no guard", func(t *testing.T) {
		// Create a simple net: p1 -> t1
		p1 := NewPlace("p1", "Place1", 10)
		trans := NewTransition("t1", "Transition1")
		arc := NewArc("a1", "p1", "t1", 2)

		// Add tokens to p1
		for i := 0; i < 3; i++ {
			p1.AddToken(&token.Token{
				ID:   string(rune('a' + i)),
				Data: &SimpleTokenData{Value: "test"},
			})
		}

		// Manually resolve (simulate what PetriNet.Resolve() would do)
		arc.Source = p1
		arc.Target = trans
		arc.resolved = true
		trans.inputArcs = []*Arc{arc}
		trans.resolved = true

		if enabled, _ := trans.IsEnabled(ctx); !enabled {
			t.Error("Transition should be enabled with sufficient tokens")
		}
	})

	t.Run("not enabled with insufficient tokens", func(t *testing.T) {
		// Create a simple net: p1 -> t1 (arc weight 3)
		p1 := NewPlace("p1", "Place1", 10)
		trans := NewTransition("t1", "Transition1")
		arc := NewArc("a1", "p1", "t1", 3)

		// Add only 2 tokens to p1 (need 3)
		for i := 0; i < 2; i++ {
			p1.AddToken(&token.Token{
				ID:   string(rune('a' + i)),
				Data: &SimpleTokenData{Value: "test"},
			})
		}

		// Manually resolve
		arc.Source = p1
		arc.Target = trans
		arc.resolved = true
		trans.inputArcs = []*Arc{arc}
		trans.resolved = true

		if enabled, _ := trans.IsEnabled(ctx); enabled {
			t.Error("Transition should not be enabled with insufficient tokens")
		}
	})

	t.Run("enabled with guard that passes", func(t *testing.T) {
		// Create a simple net: p1 -> t1
		p1 := NewPlace("p1", "Place1", 10)
		trans := NewTransition("t1", "Transition1")
		arc := NewArc("a1", "p1", "t1", 2)

		// Add passing tokens
		for i := 0; i < 2; i++ {
			p1.AddToken(&token.Token{
				ID:   string(rune('a' + i)),
				Data: &SimpleTokenData{Value: "pass"},
			})
		}

		// Set guard that checks for "pass" value
		trans.WithGuard(func(ctx *context.ExecutionContext, tokens []*token.Token) (bool, error) {
			for _, tok := range tokens {
				if data, ok := tok.Data.(*SimpleTokenData); ok {
					if data.Value != "pass" {
						return false, nil
					}
				} else {
					return false, nil
				}
			}
			return true, nil
		})

		// Manually resolve
		arc.Source = p1
		arc.Target = trans
		arc.resolved = true
		trans.inputArcs = []*Arc{arc}
		trans.resolved = true

		if enabled, _ := trans.IsEnabled(ctx); !enabled {
			t.Error("Transition should be enabled when guard passes")
		}

		// Verify tokens are put back after guard check
		if p1.TokenCount() != 2 {
			t.Errorf("expected 2 tokens after guard check, got %d", p1.TokenCount())
		}
	})

	t.Run("not enabled with guard that fails", func(t *testing.T) {
		// Create a simple net: p1 -> t1
		p1 := NewPlace("p1", "Place1", 10)
		trans := NewTransition("t1", "Transition1")
		arc := NewArc("a1", "p1", "t1", 2)

		// Add failing tokens
		p1.AddToken(&token.Token{
			ID:   "a",
			Data: &SimpleTokenData{Value: "pass"},
		})
		p1.AddToken(&token.Token{
			ID:   "b",
			Data: &SimpleTokenData{Value: "fail"},
		})

		// Set guard that checks for "pass" value
		trans.WithGuard(func(ctx *context.ExecutionContext, tokens []*token.Token) (bool, error) {
			for _, tok := range tokens {
				if data, ok := tok.Data.(*SimpleTokenData); ok {
					if data.Value != "pass" {
						return false, nil
					}
				} else {
					return false, nil
				}
			}
			return true, nil
		})

		// Manually resolve
		arc.Source = p1
		arc.Target = trans
		arc.resolved = true
		trans.inputArcs = []*Arc{arc}
		trans.resolved = true

		if enabled, _ := trans.IsEnabled(ctx); enabled {
			t.Error("Transition should not be enabled when guard fails")
		}

		// Verify tokens are put back after guard check
		if p1.TokenCount() != 2 {
			t.Errorf("expected 2 tokens after guard check, got %d", p1.TokenCount())
		}
	})

	t.Run("multiple input arcs all need sufficient tokens", func(t *testing.T) {
		// Create a net: p1 -> t1 <- p2
		p1 := NewPlace("p1", "Place1", 10)
		p2 := NewPlace("p2", "Place2", 10)
		trans := NewTransition("t1", "Transition1")
		arc1 := NewArc("a1", "p1", "t1", 1)
		arc2 := NewArc("a2", "p2", "t1", 1)

		// Add token to p1 but not p2
		p1.AddToken(&token.Token{
			ID:   "a",
			Data: &SimpleTokenData{Value: "test"},
		})

		// Manually resolve
		arc1.Source = p1
		arc1.Target = trans
		arc1.resolved = true
		arc2.Source = p2
		arc2.Target = trans
		arc2.resolved = true
		trans.inputArcs = []*Arc{arc1, arc2}
		trans.resolved = true

		if enabled, _ := trans.IsEnabled(ctx); enabled {
			t.Error("Transition should not be enabled when any input place lacks tokens")
		}

		// Add token to p2
		p2.AddToken(&token.Token{
			ID:   "b",
			Data: &SimpleTokenData{Value: "test"},
		})

		if enabled, _ := trans.IsEnabled(ctx); !enabled {
			t.Error("Transition should be enabled when all input places have tokens")
		}
	})
}

// TestTransition_IsEnabled_ConcurrentGuardEvaluation tests for race conditions
// when multiple goroutines call IsEnabled() simultaneously.
// This test verifies Race Condition in IsEnabled() Guard Evaluation
func TestTransition_IsEnabled_ConcurrentGuardEvaluation(t *testing.T) {
	ctx := context.NewExecutionContext(nil, nil, nil)

	// Create a net: p1 -> t1
	p1 := NewPlace("p1", "Place1", 10)
	trans := NewTransition("t1", "Transition1")
	arc := NewArc("a1", "p1", "t1", 2)

	// Add exactly 2 tokens (minimum required)
	p1.AddToken(&token.Token{
		ID:   "tok1",
		Data: &SimpleTokenData{Value: "pass"},
	})
	p1.AddToken(&token.Token{
		ID:   "tok2",
		Data: &SimpleTokenData{Value: "pass"},
	})

	// Set a guard that checks token values
	trans.WithGuard(func(ctx *context.ExecutionContext, tokens []*token.Token) (bool, error) {
		// Simulate some work in the guard
		for _, tok := range tokens {
			if data, ok := tok.Data.(*SimpleTokenData); ok {
				if data.Value != "pass" {
					return false, nil
				}
			}
		}
		return true, nil
	})

	// Manually resolve
	arc.Source = p1
	arc.Target = trans
	arc.resolved = true
	trans.inputArcs = []*Arc{arc}
	trans.resolved = true

	// Run concurrent IsEnabled() calls
	const numGoroutines = 10
	results := make([]bool, numGoroutines)
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			// All goroutines should see the transition as enabled
			results[index], _ = trans.IsEnabled(ctx)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// All goroutines should have seen the transition as enabled
	// because tokens should not be consumed during guard evaluation
	for i, enabled := range results {
		if !enabled {
			t.Errorf("Goroutine %d saw transition as disabled (race condition)", i)
		}
	}

	// Tokens should still be present after all guard checks
	if p1.TokenCount() != 2 {
		t.Errorf("expected 2 tokens after concurrent guard checks, got %d - tokens were lost!", p1.TokenCount())
	}
}

// TestTransition_IsEnabled_NoTokenConsumption verifies that IsEnabled()
// does not consume tokens from places during guard evaluation.
// This test verifies the fix 
func TestTransition_IsEnabled_NoTokenConsumption(t *testing.T) {
	ctx := context.NewExecutionContext(nil, nil, nil)

	// Create a net: p1 -> t1
	p1 := NewPlace("p1", "Place1", 10)
	trans := NewTransition("t1", "Transition1")
	arc := NewArc("a1", "p1", "t1", 3)

	// Add exactly 3 tokens (minimum required)
	for i := 0; i < 3; i++ {
		p1.AddToken(&token.Token{
			ID:   string(rune('a' + i)),
			Data: &SimpleTokenData{Value: "pass"},
		})
	}

	// Set a guard
	trans.WithGuard(func(ctx *context.ExecutionContext, tokens []*token.Token) (bool, error) {
		return len(tokens) == 3, nil
	})

	// Manually resolve
	arc.Source = p1
	arc.Target = trans
	arc.resolved = true
	trans.inputArcs = []*Arc{arc}
	trans.resolved = true

	// Call IsEnabled() multiple times
	for i := 0; i < 5; i++ {
		enabled, _ := trans.IsEnabled(ctx)
		if !enabled {
			t.Errorf("Call %d: transition should be enabled", i)
		}

		// Tokens should remain in place
		if p1.TokenCount() != 3 {
			t.Errorf("Call %d: expected 3 tokens, got %d - IsEnabled() consumed tokens!", i, p1.TokenCount())
		}
	}
}
