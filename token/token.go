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

// Package token provides the core token system for Jazz3 workflow engine.
// Tokens carry data through the Petri net and can be serialized for persistence.
package token

import (
	"fmt"
	"sync/atomic"
	"time"
)

// TokenData is the interface that all token types must implement.
// It provides methods for type identification which is essential for
// serialization and type registry operations.
type TokenData interface {
	// Type returns the type identifier string for this token type.
	// This identifier is used for serialization and type registry lookups.
	Type() string
}

// Cloneable is an optional interface that token data types can implement
// to support deep copying for parallel task execution.
//
// Shallow token copy in ParallelTask
// When token data is mutable and tasks run in parallel, shallow copying
// causes data races. Types implementing Cloneable will be deep-copied
// before being passed to parallel tasks.
//
// If a token type doesn't implement Cloneable, parallel tasks will receive
// the same Data reference (shallow copy). This is acceptable if:
// - The data is immutable (read-only)
// - Tasks don't modify the token data
// - The data type has its own internal synchronization
//
// Types with mutable state SHOULD implement this interface.
type Cloneable interface {
	// Clone creates a deep copy of the token data.
	// The returned value must be independent of the original.
	// Modifications to the clone should not affect the original.
	Clone() TokenData
}

// Token wraps token data with metadata.
// A Token consists of an identifier and the actual data payload.
// The ID is used for tracking tokens through the workflow execution.
type Token struct {
	// ID is the unique identifier for this token instance.
	ID string

	// Data is the actual payload carried by the token.
	// It must implement the TokenData interface.
	Data TokenData

	// Metadata holds arbitrary key-value pairs for task-specific data.
	// This is used by tasks to store intermediate results, configuration,
	// and other data that doesn't belong in the structured Data payload.
	Metadata map[string]interface{}
}

// Global atomic counter for token ID generation
// Token ID collisions with time.UnixNano()
// Using atomic counter ensures uniqueness even under high throughput
var tokenIDCounter uint64

// GenerateTokenID generates a unique token ID.
//
// The ID format is: "token-{timestamp}-{counter}"
// - timestamp: Unix milliseconds for approximate ordering and human readability
// - counter: Atomic counter incremented on each call for uniqueness
//
// This hybrid approach provides:
// - Uniqueness: Atomic counter guarantees no collisions
// - Ordering: Timestamp prefix allows approximate chronological sorting
// - Performance: No locks needed (atomic operations only)
// - Human-readable: Timestamp makes IDs debuggable
//
// Previous implementation used time.Now().UnixNano() which
// could collide in tight loops or under concurrent load. This implementation
// uses an atomic counter which is guaranteed to be unique.
func GenerateTokenID() string {
	counter := atomic.AddUint64(&tokenIDCounter, 1)
	timestamp := time.Now().UnixMilli() // Milliseconds for readability
	return fmt.Sprintf("token-%d-%d", timestamp, counter)
}

// NewToken creates a new token with the specified type and value.
// This is a convenience function for creating tokens.
//
// The typeName parameter is kept for backward compatibility with tests,
// but the actual data type is determined by the value's Go type.
//
// Added to support test requirements
func NewToken(typeName string, value interface{}) *Token {
	return &Token{
		ID:       GenerateTokenID(),
		Data:     newTokenData(value),
		Metadata: make(map[string]interface{}),
	}
}

// newTokenData creates appropriate TokenData based on the value's type
func newTokenData(value interface{}) TokenData {
	switch v := value.(type) {
	case string:
		return &StringToken{Value: v}
	case int:
		return &IntToken{Value: v}
	case map[string]interface{}:
		return &MapToken{Value: v}
	case TokenData:
		return v
	default:
		// Fallback: convert to string
		return &StringToken{Value: fmt.Sprintf("%v", value)}
	}
}
