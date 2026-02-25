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
	"sync"
	"testing"
)

// SimpleTokenData is a test token type
type SimpleTokenData struct {
	Value string
}

func (s *SimpleTokenData) Type() string {
	return "simple"
}

func TestNewPlace(t *testing.T) {
	tests := []struct {
		name             string
		id               string
		placeName        string
		capacity         int
		expectedCapacity int
	}{
		{
			name:             "default capacity when 0",
			id:               "p1",
			placeName:        "Place1",
			capacity:         0,
			expectedCapacity: 100,
		},
		{
			name:             "default capacity when negative",
			id:               "p2",
			placeName:        "Place2",
			capacity:         -10,
			expectedCapacity: 100,
		},
		{
			name:             "custom capacity",
			id:               "p3",
			placeName:        "Place3",
			capacity:         50,
			expectedCapacity: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPlace(tt.id, tt.placeName, tt.capacity)
			if p == nil {
				t.Fatal("NewPlace returned nil")
			}
			if p.ID != tt.id {
				t.Errorf("expected ID %s, got %s", tt.id, p.ID)
			}
			if p.Name != tt.placeName {
				t.Errorf("expected Name %s, got %s", tt.placeName, p.Name)
			}
			if p.Capacity != tt.expectedCapacity {
				t.Errorf("expected Capacity %d, got %d", tt.expectedCapacity, p.Capacity)
			}
			if p.IncomingArcIDs == nil {
				t.Error("IncomingArcIDs should be initialized")
			}
			if p.OutgoingArcIDs == nil {
				t.Error("OutgoingArcIDs should be initialized")
			}
			if p.tokens == nil {
				t.Error("tokens channel should be initialized")
			}
		})
	}
}

func TestPlace_IsInitial(t *testing.T) {
	t.Run("default is false", func(t *testing.T) {
		p := NewPlace("p1", "Place1", 10)
		if p.IsInitial {
			t.Error("expected IsInitial to be false by default")
		}
	})

	t.Run("set to true", func(t *testing.T) {
		p := NewPlace("p1", "Place1", 10)
		p.IsInitial = true
		if !p.IsInitial {
			t.Error("expected IsInitial to be true after setting")
		}
	})

	t.Run("set to false after true", func(t *testing.T) {
		p := NewPlace("p1", "Place1", 10)
		p.IsInitial = true
		p.IsInitial = false
		if p.IsInitial {
			t.Error("expected IsInitial to be false after unsetting")
		}
	})
}

func TestPlace_AddToken(t *testing.T) {
	t.Run("add token to empty place", func(t *testing.T) {
		p := NewPlace("p1", "Place1", 10)
		tok := &token.Token{
			ID:   "t1",
			Data: &SimpleTokenData{Value: "test"},
		}

		err := p.AddToken(tok)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if p.TokenCount() != 1 {
			t.Errorf("expected 1 token, got %d", p.TokenCount())
		}
	})

	t.Run("add multiple tokens", func(t *testing.T) {
		p := NewPlace("p1", "Place1", 10)

		for i := 0; i < 5; i++ {
			tok := &token.Token{
				ID:   string(rune('a' + i)),
				Data: &SimpleTokenData{Value: "test"},
			}
			err := p.AddToken(tok)
			if err != nil {
				t.Errorf("unexpected error adding token %d: %v", i, err)
			}
		}

		if p.TokenCount() != 5 {
			t.Errorf("expected 5 tokens, got %d", p.TokenCount())
		}
	})

	t.Run("add token when at capacity", func(t *testing.T) {
		p := NewPlace("p1", "Place1", 2)

		// Fill to capacity
		for i := 0; i < 2; i++ {
			tok := &token.Token{
				ID:   string(rune('a' + i)),
				Data: &SimpleTokenData{Value: "test"},
			}
			err := p.AddToken(tok)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}

		// Try to exceed capacity
		tok := &token.Token{
			ID:   "overflow",
			Data: &SimpleTokenData{Value: "test"},
		}
		err := p.AddToken(tok)
		if err == nil {
			t.Error("expected error when adding token at capacity")
		}

		if p.TokenCount() != 2 {
			t.Errorf("expected 2 tokens, got %d", p.TokenCount())
		}
	})
}

func TestPlace_RemoveToken(t *testing.T) {
	t.Run("remove token from place with tokens", func(t *testing.T) {
		p := NewPlace("p1", "Place1", 10)
		originalToken := &token.Token{
			ID:   "t1",
			Data: &SimpleTokenData{Value: "test"},
		}

		err := p.AddToken(originalToken)
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		tok, err := p.RemoveToken()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if tok == nil {
			t.Fatal("expected token, got nil")
		}
		if tok.ID != "t1" {
			t.Errorf("expected token ID t1, got %s", tok.ID)
		}

		if p.TokenCount() != 0 {
			t.Errorf("expected 0 tokens, got %d", p.TokenCount())
		}
	})

	t.Run("remove token from empty place", func(t *testing.T) {
		p := NewPlace("p1", "Place1", 10)

		tok, err := p.RemoveToken()
		if err == nil {
			t.Error("expected error when removing from empty place")
		}
		if tok != nil {
			t.Error("expected nil token")
		}
	})

	t.Run("FIFO order", func(t *testing.T) {
		p := NewPlace("p1", "Place1", 10)

		// Add tokens in order
		ids := []string{"t1", "t2", "t3"}
		for _, id := range ids {
			tok := &token.Token{
				ID:   id,
				Data: &SimpleTokenData{Value: "test"},
			}
			err := p.AddToken(tok)
			if err != nil {
				t.Fatalf("setup failed: %v", err)
			}
		}

		// Remove and verify FIFO order
		for _, expectedID := range ids {
			tok, err := p.RemoveToken()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tok.ID != expectedID {
				t.Errorf("expected token ID %s, got %s", expectedID, tok.ID)
			}
		}
	})
}

func TestPlace_TokenCount(t *testing.T) {
	p := NewPlace("p1", "Place1", 10)

	if p.TokenCount() != 0 {
		t.Errorf("expected 0 tokens initially, got %d", p.TokenCount())
	}

	// Add 3 tokens
	for i := 0; i < 3; i++ {
		tok := &token.Token{
			ID:   string(rune('a' + i)),
			Data: &SimpleTokenData{Value: "test"},
		}
		p.AddToken(tok)
	}

	if p.TokenCount() != 3 {
		t.Errorf("expected 3 tokens, got %d", p.TokenCount())
	}

	// Remove 1 token
	p.RemoveToken()

	if p.TokenCount() != 2 {
		t.Errorf("expected 2 tokens after removal, got %d", p.TokenCount())
	}
}

func TestPlace_ArcAccessors(t *testing.T) {
	p := NewPlace("p1", "Place1", 10)

	t.Run("incoming arcs before resolve", func(t *testing.T) {
		arcs := p.IncomingArcs()
		if arcs == nil {
			t.Error("IncomingArcs should return empty slice, not nil")
		}
		if len(arcs) != 0 {
			t.Errorf("expected 0 incoming arcs, got %d", len(arcs))
		}
	})

	t.Run("outgoing arcs before resolve", func(t *testing.T) {
		arcs := p.OutgoingArcs()
		if arcs == nil {
			t.Error("OutgoingArcs should return empty slice, not nil")
		}
		if len(arcs) != 0 {
			t.Errorf("expected 0 outgoing arcs, got %d", len(arcs))
		}
	})
}

func TestPlace_ConcurrentAccess(t *testing.T) {
	p := NewPlace("p1", "Place1", 100)

	var wg sync.WaitGroup
	producers := 10
	tokensPerProducer := 10

	// Producers - add all tokens first
	for i := 0; i < producers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < tokensPerProducer; j++ {
				tok := &token.Token{
					ID:   string(rune('a' + id)),
					Data: &SimpleTokenData{Value: "test"},
				}
				p.AddToken(tok)
			}
		}(i)
	}

	wg.Wait()

	// Verify all tokens were added
	expectedTokens := producers * tokensPerProducer
	if p.TokenCount() != expectedTokens {
		t.Errorf("expected %d tokens after production, got %d", expectedTokens, p.TokenCount())
	}

	// Consumers - now remove all tokens
	consumers := 10
	tokensPerConsumer := expectedTokens / consumers
	for i := 0; i < consumers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < tokensPerConsumer; j++ {
				_, _ = p.RemoveToken()
			}
		}()
	}

	wg.Wait()

	// Final token count should be 0 (all produced tokens consumed)
	if p.TokenCount() != 0 {
		t.Errorf("expected 0 tokens after concurrent operations, got %d", p.TokenCount())
	}
}

func TestPlace_PeekTokens(t *testing.T) {
	t.Run("peek empty place", func(t *testing.T) {
		p := NewPlace("p1", "Place1", 10)
		tokens := p.PeekTokens()

		if tokens == nil {
			t.Error("PeekTokens should return empty slice, not nil")
		}
		if len(tokens) != 0 {
			t.Errorf("expected 0 tokens, got %d", len(tokens))
		}
	})

	t.Run("peek place with tokens", func(t *testing.T) {
		p := NewPlace("p1", "Place1", 10)

		// Add tokens
		tok1 := &token.Token{ID: "t1", Data: &SimpleTokenData{Value: "data1"}}
		tok2 := &token.Token{ID: "t2", Data: &SimpleTokenData{Value: "data2"}}
		tok3 := &token.Token{ID: "t3", Data: &SimpleTokenData{Value: "data3"}}
		p.AddToken(tok1)
		p.AddToken(tok2)
		p.AddToken(tok3)

		// Peek tokens
		tokens := p.PeekTokens()

		// Verify we got all tokens
		if len(tokens) != 3 {
			t.Errorf("expected 3 tokens, got %d", len(tokens))
		}

		// Verify tokens are in FIFO order
		if tokens[0].ID != "t1" {
			t.Errorf("expected first token t1, got %s", tokens[0].ID)
		}
		if tokens[1].ID != "t2" {
			t.Errorf("expected second token t2, got %s", tokens[1].ID)
		}
		if tokens[2].ID != "t3" {
			t.Errorf("expected third token t3, got %s", tokens[2].ID)
		}
	})

	t.Run("peek is non-destructive", func(t *testing.T) {
		p := NewPlace("p1", "Place1", 10)

		// Add tokens
		tok1 := &token.Token{ID: "t1", Data: &SimpleTokenData{Value: "data1"}}
		tok2 := &token.Token{ID: "t2", Data: &SimpleTokenData{Value: "data2"}}
		p.AddToken(tok1)
		p.AddToken(tok2)

		// Peek tokens
		tokens := p.PeekTokens()
		if len(tokens) != 2 {
			t.Fatalf("expected 2 tokens from peek, got %d", len(tokens))
		}

		// Verify place still has tokens
		if p.TokenCount() != 2 {
			t.Errorf("expected place to still have 2 tokens after peek, got %d", p.TokenCount())
		}

		// Verify we can remove tokens normally
		removed, err := p.RemoveToken()
		if err != nil {
			t.Errorf("should be able to remove token after peek: %v", err)
		}
		if removed.ID != "t1" {
			t.Errorf("expected to remove t1, got %s", removed.ID)
		}

		// Verify FIFO order preserved
		if p.TokenCount() != 1 {
			t.Errorf("expected 1 token remaining, got %d", p.TokenCount())
		}
	})

	t.Run("peek multiple times", func(t *testing.T) {
		p := NewPlace("p1", "Place1", 10)

		// Add tokens
		tok1 := &token.Token{ID: "t1", Data: &SimpleTokenData{Value: "data1"}}
		tok2 := &token.Token{ID: "t2", Data: &SimpleTokenData{Value: "data2"}}
		p.AddToken(tok1)
		p.AddToken(tok2)

		// Peek multiple times
		tokens1 := p.PeekTokens()
		tokens2 := p.PeekTokens()
		tokens3 := p.PeekTokens()

		// Each peek should return same tokens
		if len(tokens1) != 2 || len(tokens2) != 2 || len(tokens3) != 2 {
			t.Error("each peek should return same number of tokens")
		}

		// Place should still have tokens
		if p.TokenCount() != 2 {
			t.Errorf("expected place to still have 2 tokens, got %d", p.TokenCount())
		}
	})

	t.Run("concurrent peek", func(t *testing.T) {
		p := NewPlace("p1", "Place1", 50)

		// Add tokens
		for i := 0; i < 10; i++ {
			tok := &token.Token{ID: string(rune('a' + i)), Data: &SimpleTokenData{Value: "test"}}
			p.AddToken(tok)
		}

		// Concurrent peeks
		var wg sync.WaitGroup
		peekCount := 20
		for i := 0; i < peekCount; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				tokens := p.PeekTokens()
				if len(tokens) != 10 {
					t.Errorf("expected 10 tokens from concurrent peek, got %d", len(tokens))
				}
			}()
		}

		wg.Wait()

		// Verify place still has all tokens
		if p.TokenCount() != 10 {
			t.Errorf("expected place to still have 10 tokens after concurrent peeks, got %d", p.TokenCount())
		}
	})
}

func TestPlace_Clear(t *testing.T) {
	t.Run("clear empty place", func(t *testing.T) {
		p := NewPlace("p1", "Place1", 10)

		clearedTokens := p.Clear()
		if len(clearedTokens) != 0 {
			t.Errorf("expected 0 cleared tokens from empty place, got %d", len(clearedTokens))
		}

		if p.TokenCount() != 0 {
			t.Errorf("expected 0 tokens after clear, got %d", p.TokenCount())
		}
	})

	t.Run("clear place with tokens", func(t *testing.T) {
		p := NewPlace("p1", "Place1", 10)

		// Add tokens
		for i := 0; i < 5; i++ {
			tok := &token.Token{
				ID:   string(rune('a' + i)),
				Data: &SimpleTokenData{Value: "test"},
			}
			err := p.AddToken(tok)
			if err != nil {
				t.Fatalf("setup failed: %v", err)
			}
		}

		// Verify setup
		if p.TokenCount() != 5 {
			t.Fatalf("expected 5 tokens before clear, got %d", p.TokenCount())
		}

		// Clear
		clearedTokens := p.Clear()
		if len(clearedTokens) != 5 {
			t.Errorf("expected 5 cleared tokens, got %d", len(clearedTokens))
		}

		// Verify cleared
		if p.TokenCount() != 0 {
			t.Errorf("expected 0 tokens after clear, got %d", p.TokenCount())
		}

		// Verify can add new tokens after clear
		tok := &token.Token{
			ID:   "new",
			Data: &SimpleTokenData{Value: "test"},
		}
		err := p.AddToken(tok)
		if err != nil {
			t.Errorf("should be able to add token after clear: %v", err)
		}

		if p.TokenCount() != 1 {
			t.Errorf("expected 1 token after adding, got %d", p.TokenCount())
		}
	})

	t.Run("clear at capacity", func(t *testing.T) {
		p := NewPlace("p1", "Place1", 3)

		// Fill to capacity
		for i := 0; i < 3; i++ {
			tok := &token.Token{
				ID:   string(rune('a' + i)),
				Data: &SimpleTokenData{Value: "test"},
			}
			err := p.AddToken(tok)
			if err != nil {
				t.Fatalf("setup failed: %v", err)
			}
		}

		// Clear
		clearedTokens := p.Clear()
		if len(clearedTokens) != 3 {
			t.Errorf("expected 3 cleared tokens, got %d", len(clearedTokens))
		}

		// Verify can fill again
		for i := 0; i < 3; i++ {
			tok := &token.Token{
				ID:   string(rune('x' + i)),
				Data: &SimpleTokenData{Value: "new"},
			}
			err := p.AddToken(tok)
			if err != nil {
				t.Errorf("should be able to refill after clear: %v", err)
			}
		}

		if p.TokenCount() != 3 {
			t.Errorf("expected 3 tokens after refill, got %d", p.TokenCount())
		}
	})

	t.Run("concurrent clear", func(t *testing.T) {
		p := NewPlace("p1", "Place1", 100)

		// Add tokens
		for i := 0; i < 50; i++ {
			tok := &token.Token{
				ID:   string(rune('a' + i)),
				Data: &SimpleTokenData{Value: "test"},
			}
			p.AddToken(tok)
		}

		// Multiple concurrent clears should be safe
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = p.Clear()
			}()
		}

		wg.Wait()

		// Should be cleared
		if p.TokenCount() != 0 {
			t.Errorf("expected 0 tokens after concurrent clear, got %d", p.TokenCount())
		}
	})
}

// TestPlace_PeekTokensWithCount tests the new PeekTokensWithCount method
// used for non-consuming guard evaluation
func TestPlace_PeekTokensWithCount(t *testing.T) {
	t.Run("peek exact count", func(t *testing.T) {
		p := NewPlace("p1", "Place1", 10)

		// Add 5 tokens
		for i := 0; i < 5; i++ {
			p.AddToken(&token.Token{
				ID:   string(rune('a' + i)),
				Data: &SimpleTokenData{Value: "test"},
			})
		}

		// Peek 3 tokens
		tokens, err := p.PeekTokensWithCount(3)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should get exactly 3 tokens
		if len(tokens) != 3 {
			t.Errorf("expected 3 tokens, got %d", len(tokens))
		}

		// Verify FIFO order
		if tokens[0].ID != "a" {
			t.Errorf("expected first token 'a', got %s", tokens[0].ID)
		}
		if tokens[1].ID != "b" {
			t.Errorf("expected second token 'b', got %s", tokens[1].ID)
		}
		if tokens[2].ID != "c" {
			t.Errorf("expected third token 'c', got %s", tokens[2].ID)
		}

		// All tokens should still be in place
		if p.TokenCount() != 5 {
			t.Errorf("expected 5 tokens after peek, got %d", p.TokenCount())
		}
	})

	t.Run("peek insufficient tokens", func(t *testing.T) {
		p := NewPlace("p1", "Place1", 10)

		// Add only 2 tokens
		p.AddToken(&token.Token{ID: "a", Data: &SimpleTokenData{Value: "test"}})
		p.AddToken(&token.Token{ID: "b", Data: &SimpleTokenData{Value: "test"}})

		// Try to peek 3 tokens (more than available)
		tokens, err := p.PeekTokensWithCount(3)
		if err == nil {
			t.Error("expected error when peeking more tokens than available")
		}
		if tokens != nil {
			t.Errorf("expected nil tokens on error, got %v", tokens)
		}

		// Tokens should still be in place
		if p.TokenCount() != 2 {
			t.Errorf("expected 2 tokens after failed peek, got %d", p.TokenCount())
		}
	})

	t.Run("peek zero tokens", func(t *testing.T) {
		p := NewPlace("p1", "Place1", 10)
		p.AddToken(&token.Token{ID: "a", Data: &SimpleTokenData{Value: "test"}})

		// Peek 0 tokens
		tokens, err := p.PeekTokensWithCount(0)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(tokens) != 0 {
			t.Errorf("expected 0 tokens, got %d", len(tokens))
		}

		// Token should still be there
		if p.TokenCount() != 1 {
			t.Errorf("expected 1 token, got %d", p.TokenCount())
		}
	})

	t.Run("peek is non-destructive", func(t *testing.T) {
		p := NewPlace("p1", "Place1", 10)

		// Add tokens
		p.AddToken(&token.Token{ID: "a", Data: &SimpleTokenData{Value: "test"}})
		p.AddToken(&token.Token{ID: "b", Data: &SimpleTokenData{Value: "test"}})
		p.AddToken(&token.Token{ID: "c", Data: &SimpleTokenData{Value: "test"}})

		// Peek multiple times
		for i := 0; i < 5; i++ {
			tokens, err := p.PeekTokensWithCount(2)
			if err != nil {
				t.Errorf("iteration %d: unexpected error: %v", i, err)
			}
			if len(tokens) != 2 {
				t.Errorf("iteration %d: expected 2 tokens, got %d", i, len(tokens))
			}
			// Should always get same tokens in same order
			if tokens[0].ID != "a" || tokens[1].ID != "b" {
				t.Errorf("iteration %d: expected tokens a,b, got %s,%s", i, tokens[0].ID, tokens[1].ID)
			}
		}

		// All tokens should still be present
		if p.TokenCount() != 3 {
			t.Errorf("expected 3 tokens after multiple peeks, got %d", p.TokenCount())
		}
	})

	t.Run("concurrent peek operations", func(t *testing.T) {
		p := NewPlace("p1", "Place1", 10)

		// Add tokens
		for i := 0; i < 5; i++ {
			p.AddToken(&token.Token{
				ID:   string(rune('a' + i)),
				Data: &SimpleTokenData{Value: "test"},
			})
		}

		// Run concurrent peek operations
		var wg sync.WaitGroup
		errors := make(chan error, 20)
		tokenCounts := make(chan int, 20)

		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				tokens, err := p.PeekTokensWithCount(3)
				if err != nil {
					errors <- err
					return
				}
				tokenCounts <- len(tokens)
			}()
		}

		wg.Wait()
		close(errors)
		close(tokenCounts)

		// Check for errors
		for err := range errors {
			t.Errorf("concurrent peek error: %v", err)
		}

		// All peeks should have succeeded with 3 tokens
		for count := range tokenCounts {
			if count != 3 {
				t.Errorf("expected 3 tokens from concurrent peek, got %d", count)
			}
		}

		// All tokens should still be present
		if p.TokenCount() != 5 {
			t.Errorf("expected 5 tokens after concurrent peeks, got %d", p.TokenCount())
		}
	})
}

// TestPlace_PeekTokensRaceWithConcurrentAdd tests:
// PeekTokens() has a race condition when tokens are added concurrently
// The problem: Gap between len(p.tokens) check and channel drain causes incomplete snapshots
func TestPlace_PeekTokensRaceWithConcurrentAdd(t *testing.T) {
	t.Run("peek with concurrent adds", func(t *testing.T) {
		p := NewPlace("p1", "Place1", 100)

		// Pre-populate with some tokens
		for i := 0; i < 10; i++ {
			p.AddToken(&token.Token{
				ID:   string(rune('a' + i)),
				Data: &SimpleTokenData{Value: "initial"},
			})
		}

		var wg sync.WaitGroup
		iterations := 50

		// Continuously peek tokens
		wg.Add(1)
		peekResults := make(chan int, iterations)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				tokens := p.PeekTokens()
				peekResults <- len(tokens)
			}
			close(peekResults)
		}()

		// Concurrently add tokens during peek operations
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				p.AddToken(&token.Token{
					ID:   string(rune('k' + i)),
					Data: &SimpleTokenData{Value: "concurrent"},
				})
			}
		}()

		wg.Wait()

		// Check that all peeks returned consistent results
		// The issue is: PeekTokens() might return fewer tokens than actually present
		// due to the gap between len() check and drain
		minTokensSeen := 1000
		for count := range peekResults {
			if count < 10 {
				// Should never see fewer than the initial 10 tokens
				t.Errorf("PeekTokens() returned incomplete snapshot: %d tokens (expected at least 10)", count)
			}
			if count < minTokensSeen {
				minTokensSeen = count
			}
		}

		// Final verification: all tokens should be present
		finalCount := p.TokenCount()
		if finalCount < 10 {
			t.Errorf("expected at least 10 tokens remaining, got %d", finalCount)
		}
	})

	t.Run("peek drains all tokens even with concurrent adds", func(t *testing.T) {
		p := NewPlace("p1", "Place1", 100)

		// Add initial tokens
		for i := 0; i < 5; i++ {
			p.AddToken(&token.Token{
				ID:   string(rune('a' + i)),
				Data: &SimpleTokenData{Value: "initial"},
			})
		}

		var wg sync.WaitGroup
		started := make(chan struct{})

		// Start adding tokens concurrently
		wg.Add(1)
		go func() {
			defer wg.Done()
			close(started)
			for i := 0; i < 10; i++ {
				p.AddToken(&token.Token{
					ID:   string(rune('x' + i)),
					Data: &SimpleTokenData{Value: "concurrent"},
				})
			}
		}()

		// Wait for adder to start, then peek
		<-started
		tokens := p.PeekTokens()

		wg.Wait()

		// After concurrent operations, peek should have captured a consistent snapshot
		// The snapshot might not include all added tokens (timing dependent),
		// but it should never lose tokens that were present
		if len(tokens) < 5 {
			t.Errorf("PeekTokens() lost tokens during concurrent add: got %d, expected at least 5", len(tokens))
		}

		// Place should still have all tokens (peek is non-destructive)
		finalCount := p.TokenCount()
		if finalCount != 15 {
			t.Errorf("expected 15 total tokens after operations, got %d", finalCount)
		}
	})
}
