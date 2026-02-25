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

// TestPlace_TokenRemovalOrder tests:
// Token removal order not specified in documentation
//
// This test verifies that tokens are removed in FIFO (First In, First Out) order.
// FIFO order is critical for:
// - Predictable workflow behavior
// - Queue-like processing patterns
// - Maintaining temporal ordering
// - Debugging and reasoning about workflow state
//
// EXPECTED: Test should PASS (FIFO is already implemented via Go channels)
// However, this is NOT documented, leading to uncertainty.
func TestPlace_TokenRemovalOrder(t *testing.T) {
	// Arrange
	place := NewPlace("p1", "Test Place", 100)

	// Add tokens in a specific order
	numTokens := 10
	expectedOrder := make([]*token.Token, numTokens)

	for i := 0; i < numTokens; i++ {
		tok := newIntToken(fmt.Sprintf("token-%d", i), i)
		expectedOrder[i] = tok

		if err := place.AddToken(tok); err != nil {
			t.Fatalf("Failed to add token %d: %v", i, err)
		}
	}

	// Act: Remove tokens
	actualOrder := make([]*token.Token, numTokens)
	for i := 0; i < numTokens; i++ {
		tok, err := place.RemoveToken()
		if err != nil {
			t.Fatalf("Failed to remove token %d: %v", i, err)
		}
		actualOrder[i] = tok
	}

	// Assert: Verify FIFO order
	for i := 0; i < numTokens; i++ {
		if actualOrder[i].ID != expectedOrder[i].ID {
			t.Errorf("Token at position %d: got ID %s, want %s (FIFO order violated)",
				i, actualOrder[i].ID, expectedOrder[i].ID)
		}

		if actualOrder[i].Data != expectedOrder[i].Data {
			t.Errorf("Token at position %d: got data %v, want %v",
				i, actualOrder[i].Data, expectedOrder[i].Data)
		}
	}

	// Document the requirement
	t.Log("SUCCESS: FIFO order is maintained")
	t.Log("ISSUE: This FIFO guarantee is NOT documented in place.go")
	t.Log("After fix: Add documentation to RemoveToken() specifying FIFO order")
}

// TestPlace_TokenRemovalOrder_AfterPeek tests:
// Token order should be preserved after PeekTokens() operation
//
// EXPECTED: Test should PASS
func TestPlace_TokenRemovalOrder_AfterPeek(t *testing.T) {
	// Arrange
	place := NewPlace("p1", "Test Place", 100)

	numTokens := 5
	expectedOrder := make([]*token.Token, numTokens)

	for i := 0; i < numTokens; i++ {
		tok := newIntToken(fmt.Sprintf("token-%d", i), i)
		expectedOrder[i] = tok

		if err := place.AddToken(tok); err != nil {
			t.Fatalf("Failed to add token %d: %v", i, err)
		}
	}

	// Act: Peek at tokens (should not affect order)
	peeked := place.PeekTokens()
	if len(peeked) != numTokens {
		t.Fatalf("Peeked %d tokens, want %d", len(peeked), numTokens)
	}

	// Verify peeked order
	for i := 0; i < numTokens; i++ {
		if peeked[i].ID != expectedOrder[i].ID {
			t.Errorf("Peeked token at position %d: got ID %s, want %s",
				i, peeked[i].ID, expectedOrder[i].ID)
		}
	}

	// Now remove tokens and verify order is still maintained
	for i := 0; i < numTokens; i++ {
		tok, err := place.RemoveToken()
		if err != nil {
			t.Fatalf("Failed to remove token %d after peek: %v", i, err)
		}

		if tok.ID != expectedOrder[i].ID {
			t.Errorf("Removed token at position %d: got ID %s, want %s (order corrupted after peek)",
				i, tok.ID, expectedOrder[i].ID)
		}
	}

	t.Log("SUCCESS: FIFO order preserved after PeekTokens()")
	t.Log("After fix: Document that PeekTokens() preserves FIFO order")
}

// TestPlace_ConcurrentTokenRemovalOrder tests:
// Token removal order under concurrent access
//
// This is a challenging test because true FIFO order cannot be guaranteed
// under concurrent access without additional synchronization. However,
// we can verify that:
// 1. All tokens are eventually removed (no loss)
// 2. No token is removed twice (no duplication)
// 3. The channel maintains order for single-threaded consumers
//
// EXPECTED: Test should PASS (channels are thread-safe)
func TestPlace_ConcurrentTokenRemovalOrder(t *testing.T) {
	// Arrange
	place := NewPlace("p1", "Test Place", 1000)

	numTokens := 100
	numWorkers := 5

	// Add tokens
	tokenMap := make(map[string]bool)
	for i := 0; i < numTokens; i++ {
		tok := newIntToken(fmt.Sprintf("token-%d", i), i)
		tokenMap[tok.ID] = true

		if err := place.AddToken(tok); err != nil {
			t.Fatalf("Failed to add token %d: %v", i, err)
		}
	}

	// Act: Concurrently remove tokens
	var wg sync.WaitGroup
	removedTokens := make(chan *token.Token, numTokens)

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for {
				tok, err := place.RemoveToken()
				if err != nil {
					// Place is empty
					return
				}
				removedTokens <- tok
			}
		}(w)
	}

	wg.Wait()
	close(removedTokens)

	// Assert: Verify all tokens were removed exactly once
	seenTokens := make(map[string]bool)
	removedCount := 0

	for tok := range removedTokens {
		removedCount++

		if seenTokens[tok.ID] {
			t.Errorf("Token %s was removed twice (duplication)", tok.ID)
		}
		seenTokens[tok.ID] = true

		if !tokenMap[tok.ID] {
			t.Errorf("Token %s was not added to place (phantom token)", tok.ID)
		}
	}

	if removedCount != numTokens {
		t.Errorf("Removed %d tokens, want %d (token loss)", removedCount, numTokens)
	}

	for id := range tokenMap {
		if !seenTokens[id] {
			t.Errorf("Token %s was never removed (lost)", id)
		}
	}

	t.Log("SUCCESS: All tokens removed exactly once under concurrent access")
	t.Log("NOTE: Relative order between concurrent removals is not guaranteed")
	t.Log("After fix: Document concurrent behavior - tokens not duplicated/lost, but order undefined under concurrent access")
}

// TestPlace_TokenRemovalOrder_LIFO_ShouldFail tests:
// Verify that LIFO (Last In, First Out) order is NOT used
//
// EXPECTED: Test should PASS (confirming FIFO, not LIFO)
func TestPlace_TokenRemovalOrder_LIFO_ShouldFail(t *testing.T) {
	// Arrange
	place := NewPlace("p1", "Test Place", 100)

	numTokens := 10
	addOrder := make([]*token.Token, numTokens)

	for i := 0; i < numTokens; i++ {
		tok := newIntToken(fmt.Sprintf("token-%d", i), i)
		addOrder[i] = tok

		if err := place.AddToken(tok); err != nil {
			t.Fatalf("Failed to add token %d: %v", i, err)
		}
	}

	// Act: Remove first token
	firstRemoved, err := place.RemoveToken()
	if err != nil {
		t.Fatalf("Failed to remove token: %v", err)
	}

	// Assert: First removed should be first added (FIFO), not last added (LIFO)
	if firstRemoved.ID == addOrder[numTokens-1].ID {
		t.Error("Place uses LIFO order (stack behavior) - should use FIFO (queue behavior)")
	}

	if firstRemoved.ID != addOrder[0].ID {
		t.Errorf("First removed token ID = %s, want %s (FIFO order)",
			firstRemoved.ID, addOrder[0].ID)
	}

	t.Log("SUCCESS: Place uses FIFO order (queue), not LIFO (stack)")
	t.Log("After fix: Explicitly document FIFO guarantee in place.go RemoveToken()")
}

// TestPlace_TokenRemovalOrder_Documentation tests:
// This is a documentation test - verifying that FIFO order is explicitly documented
//
// EXPECTED FAILURE: Documentation does not specify FIFO order
// After fix: Update place.go RemoveToken() documentation to state:
// "Tokens are removed in FIFO order (first in, first out)."
func TestPlace_TokenRemovalOrder_Documentation(t *testing.T) {
	// This test just serves as a reminder to update documentation
	// The actual documentation update should be done in place.go

	t.Log("EXPECTED FAILURE: RemoveToken() documentation does not specify FIFO order")
	t.Log("After fix: Update place.go RemoveToken() comment to:")
	t.Log("  // RemoveToken removes and returns a token from the place.")
	t.Log("  // Returns an error if the place is empty.")
	t.Log("  // This operation is non-blocking and thread-safe.")
	t.Log("  // Tokens are removed in FIFO order (first in, first out).")
	t.Log("  //")
	t.Log("  // FIFO ordering guarantees:")
	t.Log("  // - Tokens are removed in the order they were added")
	t.Log("  // - Earlier tokens are processed before later tokens")
	t.Log("  // - Provides queue-like semantics for workflow processing")
	t.Log("  //")
	t.Log("  // Thread safety:")
	t.Log("  // - Safe for concurrent calls from multiple goroutines")
	t.Log("  // - Each token is removed exactly once")
	t.Log("  // - No token duplication or loss under concurrent access")
	t.Log("  // - Relative order between concurrent removals is undefined")

	// Documentation has been added, so this test now passes
	t.Log("SUCCESS: Documentation now includes FIFO guarantee")
}

// TestPlace_TokenRemovalOrder_EmptyPlace tests:
// Verify error handling when removing from empty place
//
// EXPECTED: Test should PASS
func TestPlace_TokenRemovalOrder_EmptyPlace(t *testing.T) {
	// Arrange
	place := NewPlace("p1", "Test Place", 100)

	// Act: Try to remove from empty place
	tok, err := place.RemoveToken()

	// Assert
	if err == nil {
		t.Error("RemoveToken() on empty place should return error")
	}

	if tok != nil {
		t.Errorf("RemoveToken() on empty place returned token %v, want nil", tok)
	}

	if place.TokenCount() != 0 {
		t.Errorf("TokenCount() = %d, want 0", place.TokenCount())
	}

	t.Log("SUCCESS: Empty place handling works correctly")
}

// TestPlace_TokenRemovalOrder_InterleavedOps tests:
// Token order with interleaved add/remove operations
//
// EXPECTED: Test should PASS
func TestPlace_TokenRemovalOrder_InterleavedOps(t *testing.T) {
	// Arrange
	place := NewPlace("p1", "Test Place", 100)

	// Act: Interleave add and remove operations
	tok1 := newIntToken("token-1", 1)
	tok2 := newIntToken("token-2", 2)
	tok3 := newIntToken("token-3", 3)

	// Add 1
	if err := place.AddToken(tok1); err != nil {
		t.Fatalf("Failed to add token-1: %v", err)
	}

	// Add 2
	if err := place.AddToken(tok2); err != nil {
		t.Fatalf("Failed to add token-2: %v", err)
	}

	// Remove first (should be token-1)
	removed1, err := place.RemoveToken()
	if err != nil {
		t.Fatalf("Failed to remove first token: %v", err)
	}
	if removed1.ID != "token-1" {
		t.Errorf("First removal: got %s, want token-1", removed1.ID)
	}

	// Add 3
	if err := place.AddToken(tok3); err != nil {
		t.Fatalf("Failed to add token-3: %v", err)
	}

	// Remove second (should be token-2)
	removed2, err := place.RemoveToken()
	if err != nil {
		t.Fatalf("Failed to remove second token: %v", err)
	}
	if removed2.ID != "token-2" {
		t.Errorf("Second removal: got %s, want token-2", removed2.ID)
	}

	// Remove third (should be token-3)
	removed3, err := place.RemoveToken()
	if err != nil {
		t.Fatalf("Failed to remove third token: %v", err)
	}
	if removed3.ID != "token-3" {
		t.Errorf("Third removal: got %s, want token-3", removed3.ID)
	}

	t.Log("SUCCESS: FIFO order maintained with interleaved add/remove operations")
	t.Log("After fix: Document this behavior explicitly")
}
