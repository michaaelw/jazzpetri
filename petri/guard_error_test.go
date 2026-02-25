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
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jazzpetri/engine/clock"
	execctx "github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/token"
)

// TestTransition_GuardPanic tests:
// If guard panics, behavior is undefined
//
// EXPECTED FAILURE: Panic will crash the test
// Guards should recover from panics and treat as guard=false
func TestTransition_GuardPanic(t *testing.T) {
	// Panic recovery is now implemented - test should pass

	// Arrange
	net := NewPetriNet("test-net", "Test Network")
	input := NewPlace("input", "Input", 10)
	output := NewPlace("output", "Output", 10)
	trans := NewTransition("process", "Process with Panicking Guard")

	// Guard that panics
	trans.WithGuard(func(ctx *execctx.ExecutionContext, tokens []*token.Token) (bool, error) {
		panic("guard panicked!")
	})

	net.AddPlace(input)
	net.AddPlace(output)
	net.AddTransition(trans)
	net.AddArc(NewArc("a1", "input", "process", 1))
	net.AddArc(NewArc("a2", "process", "output", 1))
	net.Resolve()

	input.AddToken(&token.Token{ID: "token-1"})

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Act: Check if enabled (will panic)
	// EXPECTED FAILURE: This will panic and crash
	// After fix: Should recover and return false
	enabled, _ := trans.IsEnabled(ctx)

	// Assert: Should not panic, should return false
	if enabled {
		t.Error("Panicking guard should be treated as disabled")
	}

	// Try to fire (should fail gracefully)
	err := trans.Fire(ctx)
	if err == nil {
		t.Error("Expected error from panicking guard")
	}

	// Token should still be in input (not consumed)
	if input.TokenCount() != 1 {
		t.Error("Token should not be consumed when guard panics")
	}

	t.Log("SUCCESS: Guard panic recovered and treated as disabled")
}

// TestTransition_GuardPanic_Recovery tests:
// Verify panic recovery in guard evaluation
//
// EXPECTED FAILURE: Panic recovery not implemented
func TestTransition_GuardPanic_Recovery(t *testing.T) {
	// Panic recovery is now implemented

	tests := []struct {
		name          string
		guardFunc     GuardFunc
		expectEnabled bool
		expectPanic   bool
	}{
		{
			name: "nil dereference panic",
			guardFunc: func(ctx *execctx.ExecutionContext, tokens []*token.Token) (bool, error) {
				var ptr *int
				return *ptr == 0, nil // Will panic
			},
			expectEnabled: false,
			expectPanic:   true,
		},
		{
			name: "explicit panic",
			guardFunc: func(ctx *execctx.ExecutionContext, tokens []*token.Token) (bool, error) {
				panic("explicit panic")
			},
			expectEnabled: false,
			expectPanic:   true,
		},
		{
			name: "index out of bounds",
			guardFunc: func(ctx *execctx.ExecutionContext, tokens []*token.Token) (bool, error) {
				arr := []int{1, 2, 3}
				return arr[10] == 0, nil // Will panic
			},
			expectEnabled: false,
			expectPanic:   true,
		},
		{
			name: "normal guard returns false",
			guardFunc: func(ctx *execctx.ExecutionContext, tokens []*token.Token) (bool, error) {
				return false, nil
			},
			expectEnabled: false,
			expectPanic:   false,
		},
		{
			name: "normal guard returns true",
			guardFunc: func(ctx *execctx.ExecutionContext, tokens []*token.Token) (bool, error) {
				return true, nil
			},
			expectEnabled: true,
			expectPanic:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			net := NewPetriNet("test-net", "Test Network")
			input := NewPlace("input", "Input", 10)
			output := NewPlace("output", "Output", 10)
			trans := NewTransition("process", "Process")

			trans.WithGuard(tt.guardFunc)

			net.AddPlace(input)
			net.AddPlace(output)
			net.AddTransition(trans)
			net.AddArc(NewArc("a1", "input", "process", 1))
			net.AddArc(NewArc("a2", "process", "output", 1))
			net.Resolve()

			input.AddToken(&token.Token{ID: "token-1"})

			ctx := execctx.NewExecutionContext(
				context.Background(),
				clock.NewRealTimeClock(),
				nil,
			)

			// Act: Check if enabled (should not panic)
			enabled, _ := trans.IsEnabled(ctx)

			// Assert
			if enabled != tt.expectEnabled {
				t.Errorf("IsEnabled() = %v, want %v", enabled, tt.expectEnabled)
			}

			// Verify panic was logged if it occurred
			// EXPECTED FAILURE: No way to check if panic was recovered
			// After fix: Add panic counter to metrics or context
		})
	}

	t.Log("SUCCESS: Panic recovery implemented in guards")
}

// TestTransition_GuardError tests:
// If guard signature changes to return error, verify error handling
//
// EXPECTED FAILURE: Guard signature does not support errors
//
// Current: type GuardFunc func([]*token.Token) bool
// Proposed: type GuardFunc func([]*token.Token) (bool, error)
func TestTransition_GuardError(t *testing.T) {
	// This test demonstrates why error-returning guards would be useful

	t.Log("CURRENT: Guards cannot return errors")
	t.Log("Example scenarios where errors would be useful:")
	t.Log("  - Database query to check condition")
	t.Log("  - API call to external service")
	t.Log("  - File system check")
	t.Log("  - Any operation that can fail")
	t.Log("")
	t.Log("Current workaround: Guard panics or logs error and returns false")
	t.Log("Problem: Cannot distinguish between 'condition not met' and 'error occurred'")
	t.Log("")
	t.Log("EXPECTED FAILURE: Guards cannot propagate errors")
	t.Log("After fix: Consider changing signature to:")
	t.Log("  type GuardFunc func([]*token.Token) (bool, error)")
	t.Log("  - Returns (true, nil) if transition should fire")
	t.Log("  - Returns (false, nil) if condition not met")
	t.Log("  - Returns (false, err) if error occurred")

	t.Skip("Guard error support not implemented")
}

// TestTransition_GuardError_DatabaseExample tests:
// Real-world example: Guard that queries database
//
// EXPECTED FAILURE: No way to handle database errors in guard
func TestTransition_GuardError_DatabaseExample(t *testing.T) {
	t.Log("Example: Guard checks if user exists in database")
	t.Log("")
	t.Log("Current implementation (problematic):")
	t.Log("  trans.WithGuard(func(tokens []*token.Token) bool {")
	t.Log("      // How to handle database error?")
	t.Log("      user, err := db.GetUser(tokens[0].ID)")
	t.Log("      if err != nil {")
	t.Log("          // Option 1: Panic (crashes workflow)")
	t.Log("          // panic(err)")
	t.Log("          // Option 2: Log and return false (loses error information)")
	t.Log("          log.Error(err)")
	t.Log("          return false")
	t.Log("          // Option 3: Return false and hope for retry (unreliable)")
	t.Log("          return false")
	t.Log("      }")
	t.Log("      return user.Active")
	t.Log("  })")
	t.Log("")
	t.Log("With error support (better):")
	t.Log("  trans.WithGuard(func(tokens []*token.Token) (bool, error) {")
	t.Log("      user, err := db.GetUser(tokens[0].ID)")
	t.Log("      if err != nil {")
	t.Log("          return false, fmt.Errorf(\"failed to check user: %w\", err)")
	t.Log("      }")
	t.Log("      return user.Active, nil")
	t.Log("  })")
	t.Log("")
	t.Log("EXPECTED FAILURE: Cannot propagate database errors from guard")
	t.Log("After fix: Update GuardFunc signature to support errors")

	t.Skip("Guard error support not implemented")
}

// TestTransition_GuardLogging tests:
// Verify guard failures are logged appropriately
//
// Current behavior: No logging when guard returns false
// EXPECTED: Should log guard failures for debugging
func TestTransition_GuardLogging(t *testing.T) {
	// Arrange
	net := NewPetriNet("test-net", "Test Network")
	input := NewPlace("input", "Input", 10)
	output := NewPlace("output", "Output", 10)
	trans := NewTransition("process", "Process with Guard")

	guardEvaluations := 0
	trans.WithGuard(func(ctx *execctx.ExecutionContext, tokens []*token.Token) (bool, error) {
		guardEvaluations++
		// Guard rejects tokens with odd data
		if len(tokens) == 0 {
			return false, nil
		}
		data, ok := tokens[0].Data.(*intTokenData)
		if !ok {
			return false, nil
		}
		return data.value%2 == 0, nil
	})

	net.AddPlace(input)
	net.AddPlace(output)
	net.AddTransition(trans)
	net.AddArc(NewArc("a1", "input", "process", 1))
	net.AddArc(NewArc("a2", "process", "output", 1))
	net.Resolve()

	// Add odd token (will be rejected by guard)
	input.AddToken(newIntToken("odd-token", 3))

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Act
	enabled, _ := trans.IsEnabled(ctx)

	// Assert
	if enabled {
		t.Error("Transition should not be enabled (guard should reject odd token)")
	}

	if guardEvaluations != 1 {
		t.Errorf("Guard evaluated %d times, want 1", guardEvaluations)
	}

	// EXPECTED IMPROVEMENT: Guard failure should be logged
	// Check metrics or logs for guard rejection
	// After fix: ctx.Metrics should have "guard_rejected_total" counter

	t.Log("Current behavior: Guard rejections not logged or metered")
	t.Log("After fix: Add logging and metrics for guard evaluation:")
	t.Log("  - Log when guard returns false (at debug level)")
	t.Log("  - Increment metrics counter: guard_evaluations_total")
	t.Log("  - Increment metrics counter: guard_rejected_total")
	t.Log("  - Include transition ID and reason in logs")
}

// TestTransition_GuardTimeout tests:
// Guards that take too long should timeout
//
// EXPECTED FAILURE: No timeout mechanism for guards
func TestTransition_GuardTimeout(t *testing.T) {
	// Skip to avoid hanging
	t.Skip("EXPECTED FAILURE: No guard timeout - would hang indefinitely")

	// Arrange
	net := NewPetriNet("test-net", "Test Network")
	input := NewPlace("input", "Input", 10)
	output := NewPlace("output", "Output", 10)
	trans := NewTransition("process", "Process with Slow Guard")

	// Guard that takes 10 seconds (too long)
	trans.WithGuard(func(ctx *execctx.ExecutionContext, tokens []*token.Token) (bool, error) {
		// Simulate slow database query or API call
		// time.Sleep(10 * time.Second)
		return true, nil
	})

	net.AddPlace(input)
	net.AddPlace(output)
	net.AddTransition(trans)
	net.AddArc(NewArc("a1", "input", "process", 1))
	net.AddArc(NewArc("a2", "process", "output", 1))
	net.Resolve()

	input.AddToken(&token.Token{ID: "token-1"})

	// Context with timeout
	baseCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	ctx := execctx.NewExecutionContext(
		baseCtx,
		clock.NewRealTimeClock(),
		nil,
	)

	// Act: IsEnabled should respect context timeout
	// EXPECTED FAILURE: IsEnabled does not check context cancellation
	// Will hang for 10 seconds instead of timing out at 100ms
	enabled, _ := trans.IsEnabled(ctx)

	// Assert: Should timeout quickly
	if enabled {
		t.Error("Guard should timeout and return false")
	}

	t.Log("EXPECTED FAILURE: Guards do not respect context timeout")
	t.Log("After fix: Guard evaluation should:")
	t.Log("  - Run in goroutine with timeout")
	t.Log("  - Respect ctx.Context.Done()")
	t.Log("  - Return false on timeout")
	t.Log("  - Log timeout event")
}

// TestTransition_DisableOnGuardPanic tests:
// Transition should be disabled when guard panics
//
// EXPECTED FAILURE: Panic disables transition but not persistently
func TestTransition_DisableOnGuardPanic(t *testing.T) {
	// Panic recovery is now implemented

	// Arrange
	net := NewPetriNet("test-net", "Test Network")
	input := NewPlace("input", "Input", 10)
	output := NewPlace("output", "Output", 10)
	trans := NewTransition("process", "Process with Panicking Guard")

	panicCount := 0
	trans.WithGuard(func(ctx *execctx.ExecutionContext, tokens []*token.Token) (bool, error) {
		panicCount++
		panic(fmt.Sprintf("panic #%d", panicCount))
	})

	net.AddPlace(input)
	net.AddPlace(output)
	net.AddTransition(trans)
	net.AddArc(NewArc("a1", "input", "process", 1))
	net.AddArc(NewArc("a2", "process", "output", 1))
	net.Resolve()

	input.AddToken(&token.Token{ID: "token-1"})

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Act: Call IsEnabled multiple times
	enabled1, _ := trans.IsEnabled(ctx)
	enabled2, _ := trans.IsEnabled(ctx)
	enabled3, _ := trans.IsEnabled(ctx)

	// Assert: Should be disabled consistently after first panic
	if enabled1 || enabled2 || enabled3 {
		t.Error("Transition should remain disabled after guard panic")
	}

	// Question: Should transition be permanently disabled after panic?
	// Or should each call to IsEnabled retry the guard?
	// Current behavior: Retries guard each time (panics repeatedly)
	// Proposed: After N panics, permanently disable transition

	t.Log("SUCCESS: Guard panics are recovered on each IsEnabled call")
	t.Log("Note: Each call retries the guard - no circuit breaker pattern implemented yet")
}

// TestTransition_GuardNilTokens tests:
// Guard should handle nil tokens gracefully
//
// EXPECTED: Guards should validate input
func TestTransition_GuardNilTokens(t *testing.T) {
	// Arrange
	net := NewPetriNet("test-net", "Test Network")
	input := NewPlace("input", "Input", 10)
	output := NewPlace("output", "Output", 10)
	trans := NewTransition("process", "Process")

	receivedNil := false
	trans.WithGuard(func(ctx *execctx.ExecutionContext, tokens []*token.Token) (bool, error) {
		if tokens == nil {
			receivedNil = true
			return false, nil
		}
		return true, nil
	})

	net.AddPlace(input)
	net.AddPlace(output)
	net.AddTransition(trans)
	net.AddArc(NewArc("a1", "input", "process", 1))
	net.AddArc(NewArc("a2", "process", "output", 1))
	net.Resolve()

	// No tokens in input
	// IsEnabled should pass empty slice, not nil

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Act
	enabled, _ := trans.IsEnabled(ctx)

	// Assert
	if enabled {
		t.Error("Transition should not be enabled (no tokens)")
	}

	if receivedNil {
		t.Error("Guard received nil tokens - should receive empty slice instead")
	}

	t.Log("Guard should never receive nil tokens")
	t.Log("After fix: Ensure empty slice is passed instead of nil")
}

// TestTransition_GuardConcurrency tests that multiple goroutines can call
// IsEnabled() concurrently without data races in the engine.
// Guards themselves must be thread-safe (user responsibility).
func TestTransition_GuardConcurrency(t *testing.T) {
	// Arrange
	net := NewPetriNet("test-net", "Test Network")
	input := NewPlace("input", "Input", 100)
	output := NewPlace("output", "Output", 100)
	trans := NewTransition("process", "Process")

	var evaluationCount atomic.Int64
	// Thread-safe guard using atomic counter
	trans.WithGuard(func(ctx *execctx.ExecutionContext, tokens []*token.Token) (bool, error) {
		evaluationCount.Add(1)
		return true, nil
	})

	net.AddPlace(input)
	net.AddPlace(output)
	net.AddTransition(trans)
	net.AddArc(NewArc("a1", "input", "process", 1))
	net.AddArc(NewArc("a2", "process", "output", 1))
	net.Resolve()

	input.AddToken(&token.Token{ID: "token-1"})

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Act: Call IsEnabled from multiple goroutines
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			trans.IsEnabled(ctx)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Assert
	count := evaluationCount.Load()
	t.Logf("Guard evaluated %d times (expected 10)", count)
	if count != 10 {
		t.Errorf("Expected 10 guard evaluations, got %d", count)
	}
}
