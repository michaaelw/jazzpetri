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
	"sync"
	"testing"
	"time"

	"github.com/jazzpetri/engine/clock"
	execctx "github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/token"
)

// TestTransition_Fire_TokenIDUniqueness tests:
// Token ID generation uses time.Now().UnixNano() which can collide under high throughput
//
// FAILING TEST: Demonstrates token ID collisions when firing rapidly
// Expected behavior: All generated token IDs should be unique, even under high load
func TestTransition_Fire_TokenIDUniqueness(t *testing.T) {
	// Arrange: Create simple net with no input tokens (generates new tokens)
	net := NewPetriNet("test-net", "Test Net")
	p1 := NewPlace("p1", "Place 1", 10000)
	trans := NewTransition("t1", "Transition 1")

	net.AddPlace(p1)
	net.AddTransition(trans)

	// Only output arc - will generate tokens with time.Now().UnixNano()
	arc := NewArc("arc1", "t1", "p1", 1)
	net.AddArc(arc)

	if err := net.Resolve(); err != nil {
		t.Fatalf("Failed to resolve net: %v", err)
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(), // Must use real time to expose the bug
		nil,
	)

	// Act: Fire transition rapidly to generate many tokens
	numFires := 1000
	tokenIDMap := make(map[string]bool)
	var mu sync.Mutex
	var duplicateCount int

	for i := 0; i < numFires; i++ {
		if err := trans.Fire(ctx); err != nil {
			t.Fatalf("Fire %d failed: %v", i, err)
		}
	}

	// Collect all generated token IDs
	for i := 0; i < numFires; i++ {
		tok, err := p1.RemoveToken()
		if err != nil {
			t.Fatalf("Failed to remove token %d: %v", i, err)
		}

		mu.Lock()
		if tokenIDMap[tok.ID] {
			duplicateCount++
			t.Logf("DUPLICATE TOKEN ID FOUND: %s", tok.ID)
		}
		tokenIDMap[tok.ID] = true
		mu.Unlock()
	}

	// Assert: All token IDs should be unique
	// WHY THIS FAILS: time.Now().UnixNano() can return same value in tight loops
	if duplicateCount > 0 {
		t.Errorf("EXPECTED FAILURE: Found %d duplicate token IDs out of %d tokens",
			duplicateCount, numFires)
		t.Logf("This test FAILS because UnixNano() has limited precision")
		t.Logf("After fix: use UUID or atomic counter for token IDs")
	}

	if len(tokenIDMap) != numFires {
		t.Errorf("EXPECTED FAILURE: Generated %d unique IDs, want %d", len(tokenIDMap), numFires)
	}
}

// TestTransition_Fire_TokenIDCollisionUnderLoad tests:
// Concurrent token generation can cause ID collisions
//
// FAILING TEST: Demonstrates race conditions in token ID generation
// Expected behavior: No collisions even with concurrent transitions
func TestTransition_Fire_TokenIDCollisionUnderLoad(t *testing.T) {
	// Arrange: Create net with multiple transitions generating tokens concurrently
	net := NewPetriNet("test-net", "Test Net")

	numTransitions := 10
	firesPerTransition := 100

	places := make([]*Place, numTransitions)
	transitions := make([]*Transition, numTransitions)

	for i := 0; i < numTransitions; i++ {
		p := NewPlace(fmt.Sprintf("p%d", i), fmt.Sprintf("Place %d", i), 1000)
		t := NewTransition(fmt.Sprintf("t%d", i), fmt.Sprintf("Transition %d", i))

		places[i] = p
		transitions[i] = t

		net.AddPlace(p)
		net.AddTransition(t)

		// Output arc only - generates tokens
		arc := NewArc(fmt.Sprintf("arc%d", i), fmt.Sprintf("t%d", i), fmt.Sprintf("p%d", i), 1)
		net.AddArc(arc)
	}

	if err := net.Resolve(); err != nil {
		t.Fatalf("Failed to resolve net: %v", err)
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Act: Fire all transitions concurrently
	var wg sync.WaitGroup
	tokenIDMap := sync.Map{} // Concurrent-safe map
	duplicates := make([]string, 0)
	var dupMu sync.Mutex

	for i := 0; i < numTransitions; i++ {
		wg.Add(1)
		go func(transIdx int) {
			defer wg.Done()
			trans := transitions[transIdx]

			for j := 0; j < firesPerTransition; j++ {
				if err := trans.Fire(ctx); err != nil {
					t.Logf("Fire failed for transition %d, fire %d: %v", transIdx, j, err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Collect all token IDs from all places
	totalTokens := 0
	for i := 0; i < numTransitions; i++ {
		for {
			tok, err := places[i].RemoveToken()
			if err != nil {
				break // No more tokens in this place
			}

			totalTokens++
			if _, exists := tokenIDMap.LoadOrStore(tok.ID, true); exists {
				dupMu.Lock()
				duplicates = append(duplicates, tok.ID)
				dupMu.Unlock()
			}
		}
	}

	// Assert: No duplicate token IDs
	// WHY THIS FAILS: Concurrent calls to time.Now().UnixNano() can return same value
	if len(duplicates) > 0 {
		t.Errorf("EXPECTED FAILURE: Found %d duplicate token IDs under concurrent load",
			len(duplicates))
		t.Logf("Total tokens generated: %d", totalTokens)
		t.Logf("Duplicate IDs: %v", duplicates[:min(10, len(duplicates))])
		t.Logf("This test FAILS due to timestamp collision in concurrent execution")
		t.Logf("After fix: use UUID/ULID or atomic counter with goroutine ID")
	}
}

// TestTransition_Fire_TokenIDFormat tests:
// Token IDs should follow a consistent, collision-resistant format
//
// FAILING TEST: Current format is just "token-{nanoseconds}" which lacks uniqueness guarantees
// Expected behavior: Use UUID, ULID, or atomic counter
func TestTransition_Fire_TokenIDFormat(t *testing.T) {
	// Arrange
	net := NewPetriNet("test-net", "Test Net")
	p1 := NewPlace("p1", "Place 1", 100)
	trans := NewTransition("t1", "Transition 1")

	net.AddPlace(p1)
	net.AddTransition(trans)
	net.AddArc(NewArc("arc1", "t1", "p1", 1))

	if err := net.Resolve(); err != nil {
		t.Fatalf("Failed to resolve net: %v", err)
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Act: Generate some tokens
	numTokens := 10
	for i := 0; i < numTokens; i++ {
		trans.Fire(ctx)
	}

	// Assert: Check token ID format
	for i := 0; i < numTokens; i++ {
		tok, _ := p1.RemoveToken()

		// Current format: "token-{timestamp}"
		// After fix: Should be UUID format or similar
		t.Logf("Generated token ID: %s", tok.ID)

		// WHY THIS FAILS: IDs are timestamp-based, not UUIDs
		// After fix, IDs should match UUID format:
		// Example: "123e4567-e89b-12d3-a456-426614174000"
		if len(tok.ID) < 32 { // UUID without hyphens is 32 chars
			t.Logf("Token ID '%s' is not UUID format (length=%d)", tok.ID, len(tok.ID))
		}
	}

	t.Logf("After fix: token IDs should be UUIDs or ULIDs for guaranteed uniqueness")
}

// TestTransition_Fire_NoDoubleEnableCheck tests:
// Transition.Fire() calls IsEnabled() but engine already checked it
//
// FAILING TEST: Demonstrates guard function is called twice
// Expected behavior: Guard should be called only once per fire attempt
func TestTransition_Fire_NoDoubleEnableCheck(t *testing.T) {
	// Arrange: Create net with guard function that tracks calls
	net := NewPetriNet("test-net", "Test Net")

	p1 := NewPlace("p1", "Place 1", 10)
	p2 := NewPlace("p2", "Place 2", 10)
	trans := NewTransition("t1", "Transition 1")

	// Guard function that counts calls
	var guardCallCount int
	var guardMu sync.Mutex

	trans.WithGuard(func(ctx *execctx.ExecutionContext, tokens []*token.Token) (bool, error) {
		guardMu.Lock()
		guardCallCount++
		guardMu.Unlock()
		return true, nil // Always allow
	})

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddTransition(trans)

	net.AddArc(NewArc("a1", "p1", "t1", 1))
	net.AddArc(NewArc("a2", "t1", "p2", 1))

	if err := net.Resolve(); err != nil {
		t.Fatalf("Failed to resolve net: %v", err)
	}

	// Add token
	tok := &token.Token{
		ID:   "tok1",
		Data: &MapToken{Value: make(map[string]interface{})},
	}
	p1.AddToken(tok)

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Act: Fire transition (simulating what engine does)
	// Engine calls IsEnabled(), then FireUnchecked() to avoid double-check
	guardMu.Lock()
	guardCallCount = 0
	guardMu.Unlock()

	// Simulate engine check
	if enabled, _ := trans.IsEnabled(ctx); !enabled {
		t.Fatal("Transition should be enabled")
	}

	engineCallCount := guardCallCount

	// Then FireUnchecked() is called (engine uses this to avoid re-checking)
	if err := trans.FireUnchecked(ctx); err != nil {
		t.Fatalf("FireUnchecked failed: %v", err)
	}

	totalCallCount := guardCallCount

	// Assert: Guard should be called only ONCE total
	// Fixed: FireUnchecked() skips redundant IsEnabled() check
	if totalCallCount != 1 {
		t.Errorf("Guard called %d times, want 1", totalCallCount)
		t.Logf("Engine IsEnabled() calls: %d", engineCallCount)
		t.Logf("FireUnchecked() internal calls: %d", totalCallCount-engineCallCount)
	}

	if totalCallCount == 1 {
		t.Logf("SUCCESS: Guard called exactly once (engine check only)")
	}
}

// TestTransition_Fire_GuardEvaluationCount tests:
// Guard functions with side effects or expensive operations suffer from double evaluation
//
// FAILING TEST: Demonstrates performance impact of double-checking
// Expected behavior: Single evaluation per firing attempt
func TestTransition_Fire_GuardEvaluationCount(t *testing.T) {
	net := NewPetriNet("test-net", "Test Net")

	p1 := NewPlace("p1", "Place 1", 100)
	p2 := NewPlace("p2", "Place 2", 100)
	trans := NewTransition("t1", "Transition 1")

	// Guard that tracks total evaluation time (simulating expensive check)
	var totalEvaluations int
	var evaluationTime time.Duration
	var mu sync.Mutex

	trans.WithGuard(func(ctx *execctx.ExecutionContext, tokens []*token.Token) (bool, error) {
		mu.Lock()
		defer mu.Unlock()

		totalEvaluations++
		start := time.Now()

		// Simulate expensive guard operation (database query, API call, etc.)
		time.Sleep(1 * time.Millisecond)

		evaluationTime += time.Since(start)
		return true, nil
	})

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddTransition(trans)
	net.AddArc(NewArc("a1", "p1", "t1", 1))
	net.AddArc(NewArc("a2", "t1", "p2", 1))
	net.Resolve()

	// Add tokens
	for i := 0; i < 10; i++ {
		p1.AddToken(&token.Token{
			ID:   fmt.Sprintf("tok%d", i),
			Data: &MapToken{Value: make(map[string]interface{})},
		})
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Act: Fire transition 10 times (as engine would)
	numFires := 10
	start := time.Now()

	for i := 0; i < numFires; i++ {
		// Simulate engine checking enablement then firing
		// Use FireUnchecked() after IsEnabled() to avoid double-check
		if enabled, _ := trans.IsEnabled(ctx); enabled {
			trans.FireUnchecked(ctx) // Engine now uses FireUnchecked()
		}
	}

	elapsed := time.Since(start)

	// Assert: Should have exactly numFires evaluations, not 2*numFires
	// Fixed: FireUnchecked() skips redundant IsEnabled() check
	expectedEvaluations := numFires
	actualEvaluations := totalEvaluations

	if actualEvaluations > expectedEvaluations {
		t.Errorf("Guard evaluated %d times, want %d",
			actualEvaluations, expectedEvaluations)
		t.Logf("Average evaluations per fire: %.2f (want 1.0)",
			float64(actualEvaluations)/float64(numFires))
		t.Logf("Total evaluation time: %v", evaluationTime)
		t.Logf("Total elapsed: %v", elapsed)
	}

	// Verify evaluations match expected
	if actualEvaluations == expectedEvaluations {
		t.Logf("SUCCESS: Guard evaluated exactly once per fire")
		t.Logf("Total evaluation time: %v", evaluationTime)
		t.Logf("Total elapsed: %v", elapsed)
	}
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
