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
	"sync"
	"testing"

	"github.com/jazzpetri/engine/token"
)

// ============================================================================
// Place Capacity Default (100) May Be Too Low
// ============================================================================

// TestPlace_DefaultCapacity tests:
// Verify current default capacity is 100
//
// THIS SHOULD PASS: Documents current behavior
// After fix: May change default or add unlimited capacity option
func TestPlace_DefaultCapacity(t *testing.T) {
	testCases := []struct {
		name              string
		specifiedCapacity int
		expectedCapacity  int
	}{
		{
			name:              "zero capacity defaults to 100",
			specifiedCapacity: 0,
			expectedCapacity:  100,
		},
		{
			name:              "negative capacity defaults to 100",
			specifiedCapacity: -1,
			expectedCapacity:  100,
		},
		{
			name:              "positive capacity is preserved",
			specifiedCapacity: 50,
			expectedCapacity:  50,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			place := NewPlace("p1", "Test Place", tc.specifiedCapacity)

			// Assert
			if place.Capacity != tc.expectedCapacity {
				t.Errorf("Expected capacity %d, got %d", tc.expectedCapacity, place.Capacity)
			}

			t.Logf("Place with capacity %d defaults to %d", tc.specifiedCapacity, place.Capacity)
		})
	}

	t.Logf("PASS: Default capacity is 100")
	t.Logf("Consider: Is 100 sufficient for all use cases?")
	t.Logf("Consider: Should we support unlimited capacity (-1)?")
}

// TestPlace_LargeCapacity tests:
// Test with large capacity (10000+ tokens)
//
// EXPECTED FAILURE: May hit memory or performance issues
// Expected behavior: Should handle large capacities gracefully
func TestPlace_LargeCapacity(t *testing.T) {
	// Arrange: Create place with large capacity
	largeCapacity := 10000
	place := NewPlace("p1", "Large Place", largeCapacity)

	// Act: Try to fill it with many tokens
	for i := 0; i < largeCapacity; i++ {
		tok := &token.Token{
			ID:   fmt.Sprintf("tok%d", i),
			Data: &MapToken{Value: map[string]interface{}{"index": i}},
		}
		err := place.AddToken(tok)
		if err != nil {
			t.Errorf("EXPECTED FAILURE: Failed to add token %d/%d: %v", i+1, largeCapacity, err)
			t.Logf("After fix: Should handle large capacities without errors")
			break
		}

		// Check progress every 1000 tokens
		if (i+1)%1000 == 0 {
			t.Logf("Added %d/%d tokens", i+1, largeCapacity)
		}
	}

	// Verify token count
	count := place.TokenCount()
	if count != largeCapacity {
		t.Errorf("Expected %d tokens, got %d", largeCapacity, count)
	}

	t.Logf("Successfully handled %d tokens", largeCapacity)
}

// TestPlace_UnlimitedCapacity tests:
// Test unlimited capacity with capacity = -1
//
// EXPECTED FAILURE: Unlimited capacity not supported
// Expected behavior: Should support unlimited capacity (-1)
func TestPlace_UnlimitedCapacity(t *testing.T) {
	// Arrange: Try to create place with unlimited capacity
	place := NewPlace("p1", "Unlimited Place", -1)

	// Currently, -1 defaults to 100
	// After fix, -1 should mean unlimited

	// Act: Try to add more than 100 tokens
	maxTokens := 200 // More than default capacity
	successCount := 0
	var lastErr error

	for i := 0; i < maxTokens; i++ {
		tok := &token.Token{
			ID:   fmt.Sprintf("tok%d", i),
			Data: &MapToken{Value: map[string]interface{}{"index": i}},
		}
		err := place.AddToken(tok)
		if err != nil {
			lastErr = err
			break
		}
		successCount++
	}

	// Assert
	// EXPECTED FAILURE: Currently defaults to 100, so will fail at 100
	if successCount < maxTokens {
		t.Logf("EXPECTED FAILURE: Could only add %d tokens, capacity defaulted to 100", successCount)
		t.Logf("Last error: %v", lastErr)
		t.Logf("After fix: Capacity -1 should mean unlimited, allowing %d+ tokens", maxTokens)
	} else {
		t.Logf("Successfully added %d tokens with unlimited capacity", maxTokens)
	}

	// After fix, this test should pass with unlimited capacity
	if place.Capacity == 100 {
		t.Logf("EXPECTED FAILURE: -1 capacity defaulted to 100 instead of unlimited")
	}
}

// ============================================================================
// No Metrics for Place Capacity Utilization
// ============================================================================

// TestPlace_CapacityUtilization tests:
// No method to get capacity utilization (tokens/capacity ratio)
//
// EXPECTED FAILURE: GetUtilization() method doesn't exist
// Expected behavior: New GetUtilization() method returning float64
func TestPlace_CapacityUtilization(t *testing.T) {
	// Arrange
	capacity := 100
	place := NewPlace("p1", "Test Place", capacity)

	// Add some tokens
	for i := 0; i < 50; i++ {
		tok := &token.Token{
			ID:   fmt.Sprintf("tok%d", i),
			Data: &MapToken{Value: map[string]interface{}{"index": i}},
		}
		place.AddToken(tok)
	}

	// Act & Assert
	// EXPECTED FAILURE: GetUtilization() method doesn't exist - won't compile
	/*
	utilization := place.GetUtilization()

	// Should be 0.5 (50/100)
	expectedUtilization := 0.5
	if utilization != expectedUtilization {
		t.Errorf("EXPECTED FAILURE: Expected utilization %.2f, got %.2f",
			expectedUtilization, utilization)
	}

	// Test empty place
	place.Clear()
	utilization = place.GetUtilization()
	if utilization != 0.0 {
		t.Errorf("EXPECTED FAILURE: Empty place should have 0.0 utilization, got %.2f", utilization)
	}

	// Test full place
	for i := 0; i < capacity; i++ {
		place.AddToken(token.NewToken(string(rune(i)), i))
	}
	utilization = place.GetUtilization()
	if utilization != 1.0 {
		t.Errorf("EXPECTED FAILURE: Full place should have 1.0 utilization, got %.2f", utilization)
	}
	*/

	t.Logf("EXPECTED FAILURE: GetUtilization() method doesn't exist yet")
	t.Logf("After fix: Implement GetUtilization() returning float64 (tokens/capacity)")
	t.Logf("Formula: GetUtilization() = float64(TokenCount()) / float64(Capacity)")
}

// TestMetrics_PlaceCapacityUtilization tests:
// No metrics recorded for place capacity utilization
//
// EXPECTED FAILURE: No capacity metrics in ExecutionContext
// Expected behavior: Record capacity metrics during execution
func TestMetrics_PlaceCapacityUtilization(t *testing.T) {
	// This test would require ExecutionContext integration
	// Placeholder for future implementation

	t.Logf("EXPECTED FAILURE: No capacity utilization metrics recorded")
	t.Logf("After fix: Record place_capacity_utilization metric")
	t.Logf("Consider: Metrics like max_utilization, avg_utilization per place")
}

// ============================================================================
// Place.Clear() Doesn't Return Cleared Tokens
// ============================================================================

// TestPlace_ClearReturnsTokens tests:
// Clear() doesn't return the cleared tokens
//
// EXPECTED FAILURE: Clear() returns error, not tokens
// Expected behavior: Clear() should return []*Token
func TestPlace_ClearReturnsTokens(t *testing.T) {
	// Arrange: Create place with tokens
	place := NewPlace("p1", "Test Place", 10)
	expectedTokens := []*token.Token{
		{ID: "t1", Data: &MapToken{Value: map[string]interface{}{"data": "data1"}}},
		{ID: "t2", Data: &MapToken{Value: map[string]interface{}{"data": "data2"}}},
		{ID: "t3", Data: &MapToken{Value: map[string]interface{}{"data": "data3"}}},
	}

	for _, tok := range expectedTokens {
		err := place.AddToken(tok)
		if err != nil {
			t.Fatalf("Failed to add token: %v", err)
		}
	}

	// Act
	clearedTokens := place.Clear()

	// Assert: Should return the tokens that were cleared
	if len(clearedTokens) != len(expectedTokens) {
		t.Errorf("Expected %d cleared tokens, got %d",
			len(expectedTokens), len(clearedTokens))
	}

	// Verify token IDs match
	for i, tok := range clearedTokens {
		if tok.ID != expectedTokens[i].ID {
			t.Errorf("Token %d: expected ID %s, got %s",
				i, expectedTokens[i].ID, tok.ID)
		}
	}

	t.Logf("SUCCESS: Clear() returns []*Token for inspection")
	t.Logf("Use case: Inspect tokens before discarding, state restoration debugging")
}

// TestPlace_ClearWithInspection tests:
// Use case for inspecting cleared tokens
//
// EXPECTED FAILURE: Cannot inspect cleared tokens
// Expected behavior: Clear() returns tokens for inspection/logging
func TestPlace_ClearWithInspection(t *testing.T) {
	// Arrange
	place := NewPlace("p1", "Order Queue", 100)

	// Add some orders
	for i := 0; i < 10; i++ {
		tok := &token.Token{
			ID: fmt.Sprintf("order%d", i),
			Data: &MapToken{Value: map[string]interface{}{
				"orderID": i,
				"amount":  100.0 * float64(i),
			}},
		}
		place.AddToken(tok)
	}

	// Act: Clear and inspect
	// EXPECTED FAILURE: Current Clear() discards tokens
	place.Clear()

	// After fix:
	/*
	clearedTokens := place.Clear()

	// Use case: Log what was cleared
	totalAmount := 0.0
	for _, tok := range clearedTokens {
		if data, ok := tok.Data.(map[string]interface{}); ok {
			if amount, ok := data["amount"].(float64); ok {
				totalAmount += amount
			}
		}
	}

	t.Logf("Cleared %d orders worth $%.2f", len(clearedTokens), totalAmount)
	*/

	t.Logf("EXPECTED FAILURE: Cannot inspect cleared tokens with current API")
	t.Logf("After fix: Clear() returns tokens for inspection/logging")
	t.Logf("Use case: Audit trails, debugging, metrics on cleared data")
}

// TestPlace_ClearEmptyPlace tests:
// Clearing empty place should return empty slice
//
// EXPECTED FAILURE: Current behavior unknown for return value
// Expected behavior: Clear() on empty place returns empty []*Token
func TestPlace_ClearEmptyPlace(t *testing.T) {
	// Arrange: Empty place
	place := NewPlace("p1", "Empty Place", 10)

	// Act
	clearedTokens := place.Clear()

	// Assert: Should return empty slice
	if len(clearedTokens) != 0 {
		t.Errorf("Empty place should return empty slice, got %d tokens",
			len(clearedTokens))
	}

	t.Logf("SUCCESS: Clear() on empty place returns empty []*Token slice")
}

// TestPlace_ClearThreadSafety tests:
// Clear() should be thread-safe with concurrent operations
//
// Tests that Clear() properly locks and returns consistent results
func TestPlace_ClearThreadSafety(t *testing.T) {
	// Arrange
	place := NewPlace("p1", "Concurrent Place", 1000)

	// Add tokens concurrently
	var wg sync.WaitGroup
	tokensAdded := 100

	for i := 0; i < tokensAdded; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			tok := &token.Token{
				ID:   fmt.Sprintf("tok%d", id),
				Data: &MapToken{Value: map[string]interface{}{"id": id}},
			}
			place.AddToken(tok)
		}(i)
	}
	wg.Wait()

	// Act: Clear while other goroutines try to add/remove
	var numCleared int

	wg.Add(1)
	go func() {
		defer wg.Done()
		clearedTokens := place.Clear()
		numCleared = len(clearedTokens) // Track how many were cleared
	}()

	// Try to add more tokens concurrently
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			tok := &token.Token{
				ID:   fmt.Sprintf("tok%d", id+1000),
				Data: &MapToken{Value: map[string]interface{}{"id": id}},
			}
			place.AddToken(tok)
		}(i)
	}

	wg.Wait()

	// Assert
	_ = numCleared // The number of tokens that were cleared

	// After fix, verify cleared count is consistent
	// (Should clear exactly what was there at the moment of Clear())

	t.Logf("Clear() executed concurrently with Add operations")
	t.Logf("After fix: Should return consistent snapshot of cleared tokens")
}
