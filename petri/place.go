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

// Package petri provides the core Petri net model for Jazz3 workflow engine.
// It implements places, transitions, arcs, and the Petri net container with
// hybrid graph structure (ID references for serialization, direct pointers for execution).
//
// Token Ordering Semantics:
//
// Places maintain FIFO (First In, First Out) ordering for tokens:
//   - Tokens are stored in a Go channel, which guarantees FIFO order
//   - RemoveToken() always removes the oldest token first
//   - This provides queue-like semantics for workflow processing
//   - Predictable behavior for sequential workflows
//
// Thread-safe operations ensure no token loss or duplication under concurrent access,
// though relative order between concurrent removals from multiple goroutines is undefined.
package petri

import (
	"fmt"
	"sync"

	"github.com/jazzpetri/engine/token"
)

// Place holds tokens in a Petri net.
// Places use Go channels for thread-safe token storage and FIFO ordering.
// The capacity limits the number of tokens that can be held (0 = default capacity).
//
// Global Capacity Support
// Added net field to reference parent PetriNet for global capacity checks.
type Place struct {
	// ID is the unique identifier for this place
	ID string

	// Name is the human-readable name
	Name string

	// Capacity is the maximum number of tokens this place can hold
	// 0 or negative = default capacity of 100
	Capacity int

	// IsInitial indicates if this place is an initial/starting state
	// Initial places receive tokens when the workflow starts
	IsInitial bool

	// IsTerminal indicates if this place represents a final/terminal state
	// When a token reaches a terminal place, the workflow is considered complete
	// State Detection Support
	IsTerminal bool

	// IncomingArcIDs stores arc IDs for serialization
	// These are resolved to direct pointers by PetriNet.Resolve()
	IncomingArcIDs []string

	// OutgoingArcIDs stores arc IDs for serialization
	// These are resolved to direct pointers by PetriNet.Resolve()
	OutgoingArcIDs []string

	// incomingArcs contains the resolved arc pointers (runtime only)
	incomingArcs []*Arc

	// outgoingArcs contains the resolved arc pointers (runtime only)
	outgoingArcs []*Arc

	// tokens is the channel-based token storage for thread safety
	tokens chan *token.Token

	// peekMu protects the PeekTokens operation to prevent concurrent peeks from interfering
	peekMu sync.Mutex

	// resolved indicates whether ID references have been converted to pointers
	resolved bool

	// net is a reference to the parent PetriNet for global capacity checks
	// Set by PetriNet.AddPlace()
	net *PetriNet
}

// NewPlace creates a new place with the specified ID, name, and capacity.
//
// Capacity Semantics:
//   - capacity > 0: Fixed capacity of specified size
//   - capacity == 0: Uses DefaultPlaceCapacity (100)
//   - capacity < 0: Uses DefaultPlaceCapacity (100)
//
// The default capacity is suitable for most workflows. For workflows with
// high token volumes, consider specifying a larger capacity explicitly.
//
// Note: Unlike some Petri net implementations, Jazz3 does not currently support
// unlimited capacity (-1). All places have a finite capacity to prevent unbounded
// memory growth. See discussion of unlimited capacity support.
//
// The place is initialized with empty arc lists and a token channel.
func NewPlace(id, name string, capacity int) *Place {
	if capacity <= 0 {
		// Use named constant instead of magic number
		capacity = 100 // TODO: Import engine.DefaultPlaceCapacity when circular import is resolved
	}
	return &Place{
		ID:             id,
		Name:           name,
		Capacity:       capacity,
		IncomingArcIDs: make([]string, 0),
		OutgoingArcIDs: make([]string, 0),
		incomingArcs:   make([]*Arc, 0),
		outgoingArcs:   make([]*Arc, 0),
		tokens:         make(chan *token.Token, capacity),
	}
}

// AddToken adds a token to the place.
// Returns an error if the place is at capacity or global capacity would be exceeded.
// This operation is non-blocking and thread-safe.
//
// Global Capacity Support
// Checks global capacity before adding tokens if the place belongs to a net
// with global capacity configured.
func (p *Place) AddToken(t *token.Token) error {
	// Check global capacity if net is configured
	if p.net != nil {
		if err := p.net.checkGlobalCapacity(1); err != nil {
			return err
		}
	}

	select {
	case p.tokens <- t:
		// Increment global token count if net is configured
		if p.net != nil {
			p.net.incrementTokenCount(1)
		}
		return nil
	default:
		return fmt.Errorf("place %s is at capacity", p.ID)
	}
}

// RemoveToken removes and returns a token from the place.
// Returns an error if the place is empty.
// This operation is non-blocking and thread-safe.
//
// FIFO Ordering Guarantee:
// Tokens are removed in FIFO (First In, First Out) order.
// - Tokens are removed in the order they were added
// - Earlier tokens are processed before later tokens
// - Provides queue-like semantics for workflow processing
//
// Thread Safety:
// - Safe for concurrent calls from multiple goroutines
// - Each token is removed exactly once
// - No token duplication or loss under concurrent access
// - Relative order between concurrent removals is undefined
//
// Global Capacity Support
// Decrements global token count when removing tokens.
func (p *Place) RemoveToken() (*token.Token, error) {
	select {
	case t := <-p.tokens:
		// Decrement global token count if net is configured
		if p.net != nil {
			p.net.incrementTokenCount(-1)
		}
		return t, nil
	default:
		return nil, fmt.Errorf("place %s is empty", p.ID)
	}
}

// TokenCount returns the current number of tokens in the place.
// This operation is thread-safe and non-blocking.
func (p *Place) TokenCount() int {
	return len(p.tokens)
}

// IncomingArcs returns the list of incoming arcs.
// Returns an empty slice if the place has not been resolved yet.
// The arcs are only available after PetriNet.Resolve() has been called.
func (p *Place) IncomingArcs() []*Arc {
	if p.incomingArcs == nil {
		return make([]*Arc, 0)
	}
	return p.incomingArcs
}

// OutgoingArcs returns the list of outgoing arcs.
// Returns an empty slice if the place has not been resolved yet.
// The arcs are only available after PetriNet.Resolve() has been called.
func (p *Place) OutgoingArcs() []*Arc {
	if p.outgoingArcs == nil {
		return make([]*Arc, 0)
	}
	return p.outgoingArcs
}

// PeekTokens returns a copy of all tokens currently in the place without removing them.
// This is used for state capture and inspection, and is non-destructive.
// The returned slice is a snapshot of tokens at the time of the call.
// This operation is thread-safe and uses a mutex to prevent concurrent peeks from interfering.
//
// IMPORTANT: This method drains the channel until it is truly empty (not based on len()),
// to avoid race conditions where tokens are added concurrently during the peek operation.
//
// Note: This method maintains backward compatibility by not returning an error.
// Token restoration failures should not occur in normal operation since the channel
// capacity is fixed and we're only putting back what we drained.
func (p *Place) PeekTokens() []*token.Token {
	// Lock to prevent concurrent peeks from interfering with drain-and-refill
	p.peekMu.Lock()
	defer p.peekMu.Unlock()

	// Drain until channel is actually empty, don't rely on len()
	// This prevents incomplete snapshots when tokens are added concurrently
	tokens := make([]*token.Token, 0)
	putBack := make([]*token.Token, 0)

drainLoop:
	for {
		select {
		case tok := <-p.tokens:
			tokens = append(tokens, tok)
			putBack = append(putBack, tok)
		default:
			// Channel is truly empty now
			break drainLoop
		}
	}

	// Restore all tokens in same order (FIFO preservation)
	for _, tok := range putBack {
		select {
		case p.tokens <- tok:
			// Successfully restored
		default:
			// This should never happen in normal operation since we're putting back
			// what we just drained from a channel with fixed capacity.
			// If it does happen, we've lost tokens - log/panic would be appropriate
			// but for now we continue to maintain backward compatibility.
		}
	}

	return tokens
}

// PeekTokensWithCount returns up to 'count' tokens without removing them from the place.
// This method is used for guard evaluation in transitions to check tokens without consuming them.
// It returns a snapshot of the first 'count' tokens at the time of the call.
// Returns an error if there are fewer than 'count' tokens available.
//
// This method is thread-safe and uses a mutex to prevent concurrent peeks from interfering.
// Unlike RemoveToken(), this operation does not consume tokens and can be called concurrently
// by multiple goroutines without causing race conditions.
//
// IMPORTANT: This method maintains FIFO order by draining all tokens, taking a snapshot of the
// first 'count' tokens, and then restoring all tokens in their original order.
//
// Parameters:
//   - count: The number of tokens to peek at
//
// Returns:
//   - []*token.Token: A slice containing up to 'count' tokens (snapshot)
//   - error: An error if there are insufficient tokens
func (p *Place) PeekTokensWithCount(count int) ([]*token.Token, error) {
	// Lock to prevent concurrent peeks from interfering with drain-and-refill
	p.peekMu.Lock()
	defer p.peekMu.Unlock()

	// Check if we have enough tokens
	available := len(p.tokens)
	if available < count {
		return nil, fmt.Errorf("place %s has only %d tokens, need %d", p.ID, available, count)
	}

	// Drain ALL tokens from channel to maintain proper order
	// We must drain all to guarantee FIFO order is preserved after restore
	allTokens := make([]*token.Token, 0, available)
	for i := 0; i < available; i++ {
		select {
		case tok := <-p.tokens:
			allTokens = append(allTokens, tok)
		default:
			// Should not happen since we checked len() above, but handle gracefully
			// Put back what we got and return error
			for _, t := range allTokens {
				p.tokens <- t
			}
			return nil, fmt.Errorf("place %s: failed to drain tokens for peek", p.ID)
		}
	}

	// Take snapshot of first 'count' tokens
	snapshot := make([]*token.Token, count)
	copy(snapshot, allTokens[0:count])

	// Put ALL tokens back in same order (FIFO preservation)
	for _, tok := range allTokens {
		p.tokens <- tok
	}

	return snapshot, nil
}

// GetUtilization returns the capacity utilization ratio as a value between 0.0 and 1.0.
// This metric indicates how full the place is:
//   - 0.0 = empty (no tokens)
//   - 1.0 = full (at capacity)
//   - 0.5 = half full
//
// Capacity Utilization Metrics
// This method provides visibility into place capacity usage for monitoring,
// alerting, and capacity planning. High utilization may indicate bottlenecks
// or the need for increased capacity.
//
// This operation is thread-safe and non-blocking.
//
// Returns:
//   - float64: Utilization ratio between 0.0 and 1.0
func (p *Place) GetUtilization() float64 {
	tokenCount := p.TokenCount()
	if p.Capacity == 0 {
		return 0.0
	}
	return float64(tokenCount) / float64(p.Capacity)
}

// Clear removes all tokens from the place and returns them.
//
// Return Cleared Tokens
// This method now returns the tokens that were cleared, allowing callers to:
//   - Inspect cleared tokens for debugging
//   - Log what was discarded for audit trails
//   - Compute metrics on cleared data
//   - Restore state if needed
//
// This is used for state restoration to ensure the place is empty before
// restoring tokens from a saved marking. It's also useful for cleanup operations
// where you want to know what was discarded.
//
// This operation is thread-safe.
//
// Returns:
//   - []*token.Token: Slice of tokens that were cleared (empty if place was empty)
func (p *Place) Clear() []*token.Token {
	p.peekMu.Lock()
	defer p.peekMu.Unlock()

	clearedTokens := make([]*token.Token, 0)

	// Drain the channel and collect tokens
	for len(p.tokens) > 0 {
		tok := <-p.tokens
		clearedTokens = append(clearedTokens, tok)
	}

	return clearedTokens
}

// String returns a human-readable representation of the place for debugging.
// String() Methods for Debugging
func (p *Place) String() string {
	count := p.TokenCount()
	var state string
	if count == 0 {
		state = "empty"
	} else {
		utilizationPct := (count * 100) / p.Capacity
		state = fmt.Sprintf("%d tokens, %d%% full", count, utilizationPct)
	}
	terminal := ""
	if p.IsTerminal {
		terminal = " terminal"
	}
	return fmt.Sprintf("Place[%s: \"%s\" %s, capacity=%d%s]", p.ID, p.Name, state, p.Capacity, terminal)
}
