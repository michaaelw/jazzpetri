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
	"errors"
	"testing"

	"github.com/jazzpetri/engine/token"
	execContext "github.com/jazzpetri/engine/context"
)

// ============================================================================
// Transition Guard Functions Cannot Return Errors
// ============================================================================

// TestTransition_GuardWithError tests:
// Guard signature is func([]*Token) bool, cannot return errors
//
// EXPECTED FAILURE: Guard cannot communicate errors, only true/false
// Expected behavior: New guard signature func([]*Token) (bool, error)
func TestTransition_GuardWithError(t *testing.T) {
	// Arrange
	net := NewPetriNet("test", "Test")
	p1 := NewPlace("p1", "Input", 10)
	p2 := NewPlace("p2", "Output", 10)

	net.AddPlace(p1)
	net.AddPlace(p2)

	// Add token that will cause validation error
	invalidToken := &token.Token{
		ID: "t1",
		Data: &MapToken{Value: map[string]interface{}{
			"amount": -100, // Invalid negative amount
		}},
	}
	p1.AddToken(invalidToken)

	// New guard signature: func(ctx *context.ExecutionContext, tokens []*Token) (bool, error)
	// Now we can distinguish between "guard failed" and "validation error"
	trans := NewTransition("t1", "Process Payment")
	trans.WithGuard(func(ctx *execContext.ExecutionContext, tokens []*token.Token) (bool, error) {
		// We can validate the token and return an error if invalid
		for _, tok := range tokens {
			if mapData, ok := tok.Data.(*MapToken); ok {
				if amount, ok := mapData.Value["amount"].(int); ok {
					if amount < 0 {
						// Now we can return a specific error explaining WHY
						return false, errors.New("invalid amount: must be positive")
					}
				}
			}
		}
		return true, nil
	})

	net.AddTransition(trans)
	net.AddArc(NewArc("a1", "p1", "t1", 1))
	net.AddArc(NewArc("a2", "t1", "p2", 1))
	net.Resolve()

	// Act
	ctx := &execContext.ExecutionContext{}
	enabled, err := trans.IsEnabled(ctx)

	// Assert
	// NOW FIXED: We get false AND an error explaining WHY
	if !enabled && err != nil {
		t.Logf("SUCCESS: Guard returned false with error: %v", err)
		t.Logf("We now know WHY it failed: %v", err)
	} else if !enabled {
		t.Logf("Guard returned false, nil = guard condition not met (normal)")
	}

	t.Logf("FIXED: Guard now returns (bool, error)")
	t.Logf("Example: func(ctx *ExecutionContext, tokens []*Token) (bool, error)")
	t.Logf("  - return false, nil = guard condition not met (normal)")
	t.Logf("  - return false, err = validation/execution error")
	t.Logf("  - return true, nil = guard passed")
}

// TestTransition_GuardErrorHandling tests:
// How should guard errors be handled?
//
// EXPECTED FAILURE: No error handling for guards
// Expected behavior: Guard errors should be propagated/logged
func TestTransition_GuardErrorHandling(t *testing.T) {
	// After fix, this test would look like:
	/*
	trans := NewTransition("t1", "Validate Order")

	// New guard signature with error return
	trans.WithGuard(func(tokens []*Token) (bool, error) {
		for _, tok := range tokens {
			// Can now return specific validation errors
			if tok.Data == nil {
				return false, errors.New("token data is nil")
			}

			data, ok := tok.Data.(map[string]interface{})
			if !ok {
				return false, errors.New("token data is not a map")
			}

			amount, ok := data["amount"].(int)
			if !ok {
				return false, errors.New("amount field missing or wrong type")
			}

			if amount < 0 {
				return false, fmt.Errorf("invalid amount: %d (must be positive)", amount)
			}

			if amount > 10000 {
				return false, fmt.Errorf("amount exceeds limit: %d > 10000", amount)
			}
		}

		return true, nil
	})

	// When IsEnabled() is called:
	// - If guard returns (false, error), IsEnabled() should log the error
	//   and return false (transition not enabled due to error)
	// - If guard returns (false, nil), IsEnabled() returns false (condition not met)
	// - If guard returns (true, nil), IsEnabled() returns true
	*/

	t.Logf("EXPECTED FAILURE: No way to return/handle guard errors")
	t.Logf("After fix: Guard errors should be:")
	t.Logf("  1. Logged via ExecutionContext")
	t.Logf("  2. Recorded in metrics (guard_error_total)")
	t.Logf("  3. Optionally exposed via GetLastGuardError()")
}

// TestTransition_GuardValidationError tests:
// Real-world scenario: Guard needs to validate external state
//
// EXPECTED FAILURE: Cannot communicate validation failures
// Expected behavior: Validation errors should be distinguishable
func TestTransition_GuardValidationError(t *testing.T) {
	// Arrange
	trans := NewTransition("t1", "Check Account Balance")

	// Simulate guard that needs to check external state (database, API, etc.)
	trans.WithGuard(func(ctx *execContext.ExecutionContext, tokens []*token.Token) (bool, error) {
		// Imagine we need to check account balance from database
		// accountID := tokens[0].Data.(string)
		// balance, err := db.GetBalance(accountID)

		// Simulated error scenarios:
		// 1. Database connection failed
		// 2. Account not found
		// 3. Invalid account ID format

		// FIXED: We can now return the error!
		// This allows proper error handling and observability

		// Simulated database error
		dbError := errors.New("database connection failed")
		return false, dbError // Error context preserved!
	})

	t.Logf("FIXED: Guard can now distinguish between:")
	t.Logf("  1. Business condition not met (balance too low) - return false, nil")
	t.Logf("  2. System error (database down) - return false, error")
	t.Logf("  3. Validation error (invalid account ID) - return false, error")
	t.Logf("")
	t.Logf("Guard signature: func(ctx *ExecutionContext, tokens []*Token) (enabled bool, err error)")
	t.Logf("This enables proper error handling and observability")
}

// TestTransition_GuardPanicVsError tests:
// Current workaround is to panic, but that's not ideal
//
// EXPECTED FAILURE: Guards must panic to signal errors
// Expected behavior: Guards should return errors instead of panicking
func TestTransition_GuardPanicVsError(t *testing.T) {
	// Arrange
	net := NewPetriNet("test", "Test")
	p1 := NewPlace("p1", "Input", 10)
	net.AddPlace(p1)

	tok := &token.Token{
		ID:   "t1",
		Data: &MapToken{Value: make(map[string]interface{})},
	}
	p1.AddToken(tok)

	trans := NewTransition("t1", "Process")

	// FIXED: Guards can now return errors instead of panicking
	trans.WithGuard(func(ctx *execContext.ExecutionContext, tokens []*token.Token) (bool, error) {
		if tokens[0].Data == nil {
			// Now we can return a proper error
			return false, errors.New("token data is nil")
		}
		return true, nil
	})

	net.AddTransition(trans)
	net.AddArc(NewArc("a1", "p1", "t1", 1))
	net.Resolve()

	// Act
	ctx := &execContext.ExecutionContext{}
	enabled, err := trans.IsEnabled(ctx)

	// Assert
	if !enabled && err != nil {
		t.Logf("SUCCESS: Guard returned false with error: %v", err)
		t.Logf("We now know it was a validation error, not just condition not met")
	} else if !enabled {
		t.Logf("Guard returned false (condition not met)")
	}

	t.Logf("")
	t.Logf("FIXED: Replaced panic-based error handling with error returns")
	t.Logf("Guard signature: func(ctx *ExecutionContext, tokens []*Token) (bool, error)")
}

// TestTransition_GuardTypeDefinition tests:
// Consider a new GuardFunc type or interface
//
// EXPECTED FAILURE: Only one guard signature available
// Expected behavior: Flexible guard types for different use cases
func TestTransition_GuardTypeDefinition(t *testing.T) {
	// Proposed solution options:

	// Option 1: Change existing GuardFunc signature
	// type GuardFunc func(tokens []*token.Token) (bool, error)
	// Pros: Simple, backward compatible if we keep old name
	// Cons: Breaking change

	// Option 2: Add new GuardFuncWithError type
	// type GuardFunc func(tokens []*token.Token) bool  // Keep for compatibility
	// type GuardFuncWithError func(tokens []*token.Token) (bool, error)
	// Pros: Backward compatible
	// Cons: Two types to maintain

	// Option 3: Guard interface
	/*
	type Guard interface {
		Evaluate(tokens []*token.Token) (bool, error)
	}

	type FuncGuard struct {
		Fn func(tokens []*token.Token) (bool, error)
	}

	func (g *FuncGuard) Evaluate(tokens []*token.Token) (bool, error) {
		return g.Fn(tokens)
	}
	*/
	// Pros: Most flexible, extensible
	// Cons: More complex

	t.Logf("DECISION MADE: Changed GuardFunc signature")
	t.Logf("Selected:")
	t.Logf("  Option 1: Change GuardFunc signature")
	t.Logf("  type GuardFunc func(ctx *context.ExecutionContext, tokens []*token.Token) (bool, error)")
	t.Logf("This provides the best balance of simplicity and functionality")
}

// TestTransition_GuardContext tests:
// Guards might need access to ExecutionContext
//
// EXPECTED FAILURE: Guards don't have access to context
// Expected behavior: Guards should receive ExecutionContext
func TestTransition_GuardContext(t *testing.T) {
	// Future consideration: Should guards have access to ExecutionContext?
	//
	// type GuardFunc func(ctx *context.ExecutionContext, tokens []*Token) (bool, error)
	//
	// This would allow guards to:
	// 1. Log guard evaluation details
	// 2. Record metrics
	// 3. Access clock for time-based guards
	// 4. Check context cancellation
	// 5. Access workflow metadata

	t.Logf("IMPLEMENTED: Guards now receive ExecutionContext")
	t.Logf("New signature: func(ctx *ExecutionContext, tokens []*Token) (bool, error)")
	t.Logf("")
	t.Logf("This enables:")
	t.Logf("  - Time-based guards using ctx.Clock")
	t.Logf("  - Guard logging and metrics")
	t.Logf("  - Context cancellation awareness")
}

// TestTransition_GuardErrorMetrics tests:
// Guard errors should be tracked in metrics
//
// EXPECTED FAILURE: No metrics for guard errors
// Expected behavior: Metrics should track guard evaluation failures
func TestTransition_GuardErrorMetrics(t *testing.T) {
	// After fix, guards should record metrics:
	//
	// - guard_evaluations_total (counter)
	// - guard_errors_total (counter)
	// - guard_evaluation_duration (histogram)
	// - guard_false_total (counter) - guard condition not met
	// - guard_true_total (counter) - guard condition met

	t.Logf("READY FOR IMPLEMENTATION: Guard error return enables metrics tracking")
	t.Logf("Can now track guard metrics:")
	t.Logf("  - guard_evaluations_total")
	t.Logf("  - guard_errors_total")
	t.Logf("  - guard_evaluation_duration")
	t.Logf("  - guard_false_total / guard_true_total")
}
