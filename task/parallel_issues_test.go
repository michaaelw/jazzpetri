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

package task

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jazzpetri/engine/clock"
	execCtx "github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/token"
)

// mutableTokenData is a token data type with mutable state
// Used to test data race conditions with shallow copying
type mutableTokenData struct {
	mu      sync.Mutex
	counter int
	data    map[string]interface{}
}

func (m *mutableTokenData) Type() string {
	return "mutable"
}

// Clone implements token.Cloneable for deep copying
// This allows ParallelTask to create independent copies
func (m *mutableTokenData) Clone() token.TokenData {
	m.mu.Lock()
	defer m.mu.Unlock()

	return &mutableTokenData{
		counter: m.counter,
		data:    deepCloneMap(m.data),
	}
}

// deepCloneMap recursively clones a map and all nested maps
func deepCloneMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}

	cloned := make(map[string]interface{})
	for k, v := range m {
		// Recursively clone nested maps
		if nestedMap, ok := v.(map[string]interface{}); ok {
			cloned[k] = deepCloneMap(nestedMap)
		} else {
			// For other types, copy the value
			// Note: This doesn't deep-clone slices or other complex types
			// For production use, consider using a library like copystructure
			cloned[k] = v
		}
	}
	return cloned
}

// TestParallelTask_TokenDataIsolation tests:
// ParallelTask uses shallow copy, causing data races when tasks modify token data
//
// FAILING TEST: Demonstrates data race when tasks modify shared token data
// Expected behavior: Each task should get a deep copy with isolated data
//
// Run with: go test -race -run TestParallelTask_TokenDataIsolation
func TestParallelTask_TokenDataIsolation(t *testing.T) {
	// Arrange: Create token with mutable data
	clk := clock.NewVirtualClock(time.Now())
	sharedData := &mutableTokenData{
		counter: 0,
		data:    make(map[string]interface{}),
	}
	sharedData.data["value"] = 0

	tok := &token.Token{
		ID:   "test-token",
		Data: sharedData,
	}

	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	// Create tasks that modify token data
	task1 := &InlineTask{
		Name: "modifier-1",
		Fn: func(ctx *execCtx.ExecutionContext) error {
			// Get token data and modify it
			if data, ok := ctx.Token.Data.(*mutableTokenData); ok {
				data.mu.Lock()
				data.counter++
				data.data["task1"] = "modified"
				data.mu.Unlock()
			}
			time.Sleep(10 * time.Millisecond) // Increase race window
			return nil
		},
	}

	task2 := &InlineTask{
		Name: "modifier-2",
		Fn: func(ctx *execCtx.ExecutionContext) error {
			if data, ok := ctx.Token.Data.(*mutableTokenData); ok {
				data.mu.Lock()
				data.counter++
				data.data["task2"] = "modified"
				data.mu.Unlock()
			}
			time.Sleep(10 * time.Millisecond)
			return nil
		},
	}

	task3 := &InlineTask{
		Name: "modifier-3",
		Fn: func(ctx *execCtx.ExecutionContext) error {
			if data, ok := ctx.Token.Data.(*mutableTokenData); ok {
				data.mu.Lock()
				data.counter++
				data.data["task3"] = "modified"
				data.mu.Unlock()
			}
			time.Sleep(10 * time.Millisecond)
			return nil
		},
	}

	parallel := &ParallelTask{
		Name:  "test-parallel",
		Tasks: []Task{task1, task2, task3},
	}

	// Act: Execute parallel tasks
	err := parallel.Execute(ctx)

	// Assert: Original token data should be unchanged
	if err != nil {
		t.Fatalf("Parallel execution failed: %v", err)
	}

	// WHY THIS FAILS: Shallow copy means all tasks share the same Data pointer
	// All modifications affect the original token
	sharedData.mu.Lock()
	originalCounter := sharedData.counter
	originalData := sharedData.data
	sharedData.mu.Unlock()

	if originalCounter > 0 {
		t.Errorf("EXPECTED FAILURE: Original token was modified (counter=%d, want 0)",
			originalCounter)
		t.Logf("This test FAILS because shallow copy shares Data pointer")
		t.Logf("Original data: %v", originalData)
		t.Logf("After fix: implement deep copy so tasks can't affect original token")
	}

	// Run with -race flag to detect concurrent access to shared data
	t.Logf("Run with 'go test -race' to detect data race in shallow copy")
}

// TestParallelTask_DeepCopyTokenData tests:
// Verify token data is deeply copied, not just the Token struct
//
// FAILING TEST: Demonstrates shallow copy problem
// Expected behavior: Modifications in tasks don't affect original
func TestParallelTask_DeepCopyTokenData(t *testing.T) {
	// Arrange: Token with map-based data
	clk := clock.NewVirtualClock(time.Now())

	originalMap := map[string]interface{}{
		"counter": 0,
		"name":    "original",
		"nested": map[string]interface{}{
			"value": "original-nested",
		},
	}

	tok := &token.Token{
		ID: "test-token",
		Data: &mutableTokenData{
			data: originalMap,
		},
	}

	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	// Task that modifies token data
	modifier := &InlineTask{
		Name: "data-modifier",
		Fn: func(ctx *execCtx.ExecutionContext) error {
			if data, ok := ctx.Token.Data.(*mutableTokenData); ok {
				data.mu.Lock()
				// Modify top-level map
				data.data["counter"] = 100
				data.data["name"] = "modified"

				// Modify nested map
				if nested, ok := data.data["nested"].(map[string]interface{}); ok {
					nested["value"] = "modified-nested"
					nested["new-key"] = "added"
				}

				// Add new key
				data.data["new-field"] = "added"
				data.mu.Unlock()
			}
			return nil
		},
	}

	parallel := &ParallelTask{
		Name:  "test-parallel",
		Tasks: []Task{modifier},
	}

	// Act: Execute parallel task
	parallel.Execute(ctx)

	// Assert: Original data should be unchanged
	if data, ok := tok.Data.(*mutableTokenData); ok {
		data.mu.Lock()
		defer data.mu.Unlock()

		// WHY THIS FAILS: Shallow copy means modifications affect original
		if counter, ok := data.data["counter"].(int); ok && counter != 0 {
			t.Errorf("EXPECTED FAILURE: Original counter modified to %d (want 0)", counter)
		}

		if name, ok := data.data["name"].(string); ok && name != "original" {
			t.Errorf("EXPECTED FAILURE: Original name modified to %s (want 'original')", name)
		}

		if _, exists := data.data["new-field"]; exists {
			t.Errorf("EXPECTED FAILURE: New field added to original data")
		}

		if nested, ok := data.data["nested"].(map[string]interface{}); ok {
			if nestedVal := nested["value"]; nestedVal != "original-nested" {
				t.Errorf("EXPECTED FAILURE: Nested value modified to %v", nestedVal)
			}
			if _, exists := nested["new-key"]; exists {
				t.Errorf("EXPECTED FAILURE: New key added to nested map in original")
			}
		}

		t.Logf("After fix: all original values should be unchanged")
		t.Logf("Current implementation does shallow copy: &token.Token{ID: ctx.Token.ID, Data: ctx.Token.Data}")
		t.Logf("Fix required: deep copy the Data field, not just copy the pointer")
	}
}

// TestParallelTask_RaceDetection tests:
// Detect data races when multiple tasks access shared token data concurrently
//
// FAILING TEST: Will trigger race detector with shallow copy
// Expected behavior: No races with proper deep copy
//
// IMPORTANT: Run with -race flag: go test -race -run TestParallelTask_RaceDetection
func TestParallelTask_RaceDetection(t *testing.T) {
	// Arrange: Token with mutable data
	clk := clock.NewVirtualClock(time.Now())

	sharedData := &mutableTokenData{
		counter: 0,
		data:    make(map[string]interface{}),
	}

	tok := &token.Token{
		ID:   "test-token",
		Data: sharedData,
	}

	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	// Create multiple tasks that read AND write concurrently
	numTasks := 10
	tasks := make([]Task, numTasks)

	for i := 0; i < numTasks; i++ {
		taskID := i
		tasks[i] = &InlineTask{
			Name: "concurrent-accessor",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				if data, ok := ctx.Token.Data.(*mutableTokenData); ok {
					// Concurrent read/write WITHOUT lock to expose race
					// WHY THIS CAUSES RACE: All tasks share same Data pointer
					oldCounter := data.counter          // READ
					data.counter = oldCounter + 1       // WRITE
					data.data[string(rune(taskID))] = taskID // WRITE to map

					// Sleep to increase race window
					time.Sleep(1 * time.Millisecond)

					// Another read
					_ = data.counter // READ
				}
				return nil
			},
		}
	}

	parallel := &ParallelTask{
		Name:  "race-test",
		Tasks: tasks,
	}

	// Act: Execute (will trigger race detector)
	parallel.Execute(ctx)

	// Assert: Race detector will fail this test if shallow copy is used
	t.Logf("EXPECTED FAILURE with -race: Data race detected in shallow copy")
	t.Logf("Run: go test -race -run TestParallelTask_RaceDetection")
	t.Logf("After fix: race detector should pass with deep copy")
}

// TestParallelTask_NestedDataModification tests:
// Test deeply nested data structures with shallow copy
//
// FAILING TEST: Demonstrates problem with nested structures
// Expected behavior: Deep copy should handle nested structures
func TestParallelTask_NestedDataModification(t *testing.T) {
	// Arrange: Complex nested data structure
	clk := clock.NewVirtualClock(time.Now())

	nestedData := &mutableTokenData{
		data: map[string]interface{}{
			"level1": map[string]interface{}{
				"level2": map[string]interface{}{
					"level3": map[string]interface{}{
						"value": "deep-original",
					},
				},
			},
		},
	}

	tok := &token.Token{
		ID:   "test-token",
		Data: nestedData,
	}

	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	// Task that modifies deeply nested value
	modifier := &InlineTask{
		Name: "deep-modifier",
		Fn: func(ctx *execCtx.ExecutionContext) error {
			if data, ok := ctx.Token.Data.(*mutableTokenData); ok {
				data.mu.Lock()
				defer data.mu.Unlock()

				// Navigate deep and modify
				if l1, ok := data.data["level1"].(map[string]interface{}); ok {
					if l2, ok := l1["level2"].(map[string]interface{}); ok {
						if l3, ok := l2["level3"].(map[string]interface{}); ok {
							l3["value"] = "deep-modified"
							l3["new-deep-key"] = "added-deep"
						}
					}
				}
			}
			return nil
		},
	}

	parallel := &ParallelTask{
		Name:  "test-parallel",
		Tasks: []Task{modifier},
	}

	// Act
	parallel.Execute(ctx)

	// Assert: Original nested data should be unchanged
	nestedData.mu.Lock()
	defer nestedData.mu.Unlock()

	if l1, ok := nestedData.data["level1"].(map[string]interface{}); ok {
		if l2, ok := l1["level2"].(map[string]interface{}); ok {
			if l3, ok := l2["level3"].(map[string]interface{}); ok {
				if val := l3["value"]; val != "deep-original" {
					t.Errorf("EXPECTED FAILURE: Deep nested value modified to %v", val)
					t.Logf("This FAILS because even nested maps share pointers with shallow copy")
				}

				if _, exists := l3["new-deep-key"]; exists {
					t.Errorf("EXPECTED FAILURE: New key added to deeply nested map")
					t.Logf("After fix: deep copy should clone entire nested structure")
				}
			}
		}
	}
}

// TestParallelTask_TokenImmutabilityContract tests:
// Document expected immutability contract
//
// This test documents the expected behavior after fix
func TestParallelTask_TokenImmutabilityContract(t *testing.T) {
	t.Log("TOKEN IMMUTABILITY CONTRACT:")
	t.Log("")
	t.Log("Current behavior (BROKEN):")
	t.Log("  - ParallelTask uses shallow copy: &token.Token{ID: tok.ID, Data: tok.Data}")
	t.Log("  - Token.Data pointer is shared across all parallel tasks")
	t.Log("  - Tasks can modify each other's data (DATA RACE)")
	t.Log("  - Tasks can modify the original token (SIDE EFFECTS)")
	t.Log("")
	t.Log("Expected behavior after fix:")
	t.Log("  - Each parallel task receives a DEEP COPY of the token")
	t.Log("  - Token.Data is cloned (not just pointer copied)")
	t.Log("  - Tasks operate on isolated data (NO RACES)")
	t.Log("  - Original token remains unchanged (NO SIDE EFFECTS)")
	t.Log("")
	t.Log("Implementation options:")
	t.Log("  1. Implement Token.DeepCopy() method")
	t.Log("  2. Require token.Data types to implement Cloneable interface")
	t.Log("  3. Use serialization/deserialization for deep copy")
	t.Log("  4. Document that token.Data must be immutable (behavioral contract)")
	t.Log("")
	t.Log("Recommendation: Option 2 - Cloneable interface")
	t.Log("  type Cloneable interface {")
	t.Log("      Clone() interface{}")
	t.Log("  }")
	t.Log("")
	t.Log("  Then in ParallelTask.Execute():")
	t.Log("  if cloneable, ok := ctx.Token.Data.(Cloneable); ok {")
	t.Log("      tokenCopy.Data = cloneable.Clone()")
	t.Log("  }")
}

// BenchmarkParallelTask_ShallowVsDeepCopy benchmarks impact of deep copy
//
// This benchmark will be useful after implementing deep copy to measure overhead
func BenchmarkParallelTask_ShallowVsDeepCopy(b *testing.B) {
	// Simple data for shallow copy (current)
	simpleData := &mutableTokenData{
		data: map[string]interface{}{
			"value": 42,
		},
	}

	// Complex data for deep copy testing
	complexData := &mutableTokenData{
		data: map[string]interface{}{
			"l1": map[string]interface{}{
				"l2": map[string]interface{}{
					"values": []int{1, 2, 3, 4, 5},
				},
			},
		},
	}

	b.Run("shallow-copy-simple", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			tok := &token.Token{ID: "tok", Data: simpleData}
			// Current implementation
			_ = &token.Token{
				ID:   tok.ID,
				Data: tok.Data, // Shallow copy
			}
		}
	})

	b.Run("shallow-copy-complex", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			tok := &token.Token{ID: "tok", Data: complexData}
			_ = &token.Token{
				ID:   tok.ID,
				Data: tok.Data, // Shallow copy
			}
		}
	})

	// After fix: add deep copy benchmarks
	b.Log("After implementing deep copy, add benchmarks for comparison")
}
