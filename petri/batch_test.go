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
	"testing"

	"github.com/jazzpetri/engine/clock"
	execctx "github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/task"
	"github.com/jazzpetri/engine/token"
)

// simpleTokenData is a simple test implementation of TokenData
type simpleTokenData struct {
	value int
}

func (s *simpleTokenData) Type() string {
	return "simple"
}

// TestTransition_BatchFire tests:
// Fire() processes one set of tokens at a time, inefficient for bulk operations
//
// EXPECTED FAILURE: BatchFire() method does not exist
//
// Use case: Processing multiple independent items in a batch
// Example: Sending 100 email notifications - more efficient to batch
func TestTransition_BatchFire(t *testing.T) {
	// Arrange
	net := NewPetriNet("test-net", "Test Network")
	input := NewPlace("input", "Input Queue", 1000)
	output := NewPlace("output", "Output Queue", 1000)
	trans := NewTransition("process", "Process Items")

	net.AddPlace(input)
	net.AddPlace(output)
	net.AddTransition(trans)
	net.AddArc(NewArc("a1", "input", "process", 1))
	net.AddArc(NewArc("a2", "process", "output", 1))
	net.Resolve()

	// Add 100 tokens to process
	for i := 0; i < 100; i++ {
		tok := &token.Token{
			ID:   fmt.Sprintf("token-%d", i),
			Data: &simpleTokenData{value: i},
		}
		input.AddToken(tok)
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Act: Try to process in batch
	// EXPECTED FAILURE: BatchFire method does not exist
	// Uncomment when implementing:
	// batchSize := 10
	// batches, err := trans.BatchFire(ctx, batchSize)

	// Current approach: Fire repeatedly (inefficient)
	firingCount := 0
	for input.TokenCount() > 0 {
		if err := trans.Fire(ctx); err != nil {
			t.Fatalf("Fire failed at iteration %d: %v", firingCount, err)
		}
		firingCount++
	}

	// Assert
	// EXPECTED FAILURE: Had to fire 100 times (one per token)
	expectedFirings := 100
	if firingCount != expectedFirings {
		t.Errorf("Fired %d times, expected %d", firingCount, expectedFirings)
	}

	// With batch processing, could fire 10 times (10 tokens per batch)
	expectedBatchFirings := 10
	if firingCount > expectedBatchFirings {
		t.Logf("EXPECTED FAILURE: Fired %d times, with batching could be %d times",
			firingCount, expectedBatchFirings)
	}

	// Verify all tokens processed
	if output.TokenCount() != 100 {
		t.Errorf("Output has %d tokens, want 100", output.TokenCount())
	}

	t.Log("EXPECTED FAILURE: BatchFire() does not exist")
	t.Log("After fix: Add method: func (t *Transition) BatchFire(ctx, batchSize int) (int, error)")
	t.Log("Benefits:")
	t.Log("  - Fewer function calls overhead")
	t.Log("  - Batch task execution (e.g., bulk DB inserts)")
	t.Log("  - Better performance for high-throughput workflows")
}

// TestTransition_BatchFire_PartialFailure tests:
// Some batches succeed, some fail - partial processing
//
// EXPECTED FAILURE: BatchFire() method does not exist
func TestTransition_BatchFire_PartialFailure(t *testing.T) {
	// Arrange
	net := NewPetriNet("test-net", "Test Network")
	input := NewPlace("input", "Input Queue", 1000)
	output := NewPlace("output", "Output Queue", 1000)
	failed := NewPlace("failed", "Failed Queue", 1000)
	trans := NewTransition("process", "Process Items")

	// Task that fails for even-numbered tokens
	processedTokens := 0
	failedTokens := 0

	trans.WithTask(&task.InlineTask{
		Name: "batch-processor",
		Fn: func(ctx *execctx.ExecutionContext) error {
			if ctx.Token == nil {
				return fmt.Errorf("no token")
			}

			data, ok := ctx.Token.Data.(*simpleTokenData)
			if !ok {
				return fmt.Errorf("invalid token data")
			}

			// Fail on multiples of 10
			if data.value%10 == 0 {
				failedTokens++
				return fmt.Errorf("processing failed for token %d", data.value)
			}

			processedTokens++
			return nil
		},
	})

	net.AddPlace(input)
	net.AddPlace(output)
	net.AddPlace(failed)
	net.AddTransition(trans)
	net.AddArc(NewArc("a1", "input", "process", 1))
	net.AddArc(NewArc("a2", "process", "output", 1))
	net.Resolve()

	// Add 50 tokens
	for i := 0; i < 50; i++ {
		tok := &token.Token{
			ID:   fmt.Sprintf("token-%d", i),
			Data: &simpleTokenData{value: i},
		}
		input.AddToken(tok)
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Act: Process in batches (if BatchFire existed)
	// EXPECTED FAILURE: BatchFire does not exist
	/*
	batchSize := 10
	var batchResults []BatchFireResult

	for input.TokenCount() > 0 {
		result, err := trans.BatchFire(ctx, batchSize)
		batchResults = append(batchResults, result)

		if err != nil {
			// Some items in batch failed - move failed tokens to failed place
			for _, failedToken := range result.FailedTokens {
				failed.AddToken(failedToken)
			}
		}
	}

	// Assert
	totalProcessed := 0
	totalFailed := 0
	for _, result := range batchResults {
		totalProcessed += result.SuccessCount
		totalFailed += result.FailureCount
	}

	if totalProcessed+totalFailed != 50 {
		t.Errorf("Processed %d + Failed %d != 50", totalProcessed, totalFailed)
	}

	if output.TokenCount() != totalProcessed {
		t.Errorf("Output has %d tokens, want %d", output.TokenCount(), totalProcessed)
	}

	if failed.TokenCount() != totalFailed {
		t.Errorf("Failed has %d tokens, want %d", failed.TokenCount(), totalFailed)
	}
	*/

	// Current approach: Must fire one at a time with error handling
	// PROBLEM: When Fire() fails, tokens are rolled back to input,
	// causing infinite retry loop. We need to track failures and break.
	maxAttempts := 100 // Prevent infinite loop
	attempts := 0
	successCount := 0
	failureCount := 0

	for input.TokenCount() > 0 && attempts < maxAttempts {
		attempts++
		err := trans.Fire(ctx)
		if err != nil {
			// Manual error handling - without BatchFire, we can't easily
			// distinguish between retriable and permanent failures
			failureCount++
			t.Logf("Firing failed: %v", err)

			// Without batch processing, we must manually remove failed tokens
			// to prevent infinite retry. This demonstrates the limitation.
			if input.TokenCount() > 0 {
				// Manually drain one token to break the retry loop
				_, _ = input.RemoveToken()
			}
		} else {
			successCount++
		}
	}

	// Log the results of manual error handling
	t.Logf("Processed: %d attempts, %d successes, %d failures", attempts, successCount, failureCount)

	t.Log("EXPECTED FAILURE: BatchFire() with partial failure handling does not exist")
	t.Log("After fix: BatchFire should return:")
	t.Log("  type BatchFireResult struct {")
	t.Log("      SuccessCount   int")
	t.Log("      FailureCount   int")
	t.Log("      SuccessTokens  []*token.Token")
	t.Log("      FailedTokens   []*token.Token")
	t.Log("      Errors         []error")
	t.Log("  }")
}

// TestTransition_BatchFire_Atomicity tests:
// Batch should be atomic per batch (not globally atomic)
//
// EXPECTED FAILURE: BatchFire() does not exist
func TestTransition_BatchFire_Atomicity(t *testing.T) {
	// Arrange
	net := NewPetriNet("test-net", "Test Network")
	input := NewPlace("input", "Input Queue", 100)
	output := NewPlace("output", "Output Queue", 100)
	trans := NewTransition("process", "Process Batch")

	batchesProcessed := 0
	trans.WithTask(&task.InlineTask{
		Name: "batch-counter",
		Fn: func(ctx *execctx.ExecutionContext) error {
			batchesProcessed++
			return nil
		},
	})

	net.AddPlace(input)
	net.AddPlace(output)
	net.AddTransition(trans)
	net.AddArc(NewArc("a1", "input", "process", 1))
	net.AddArc(NewArc("a2", "process", "output", 1))
	net.Resolve()

	// Add 30 tokens
	for i := 0; i < 30; i++ {
		input.AddToken(&token.Token{ID: fmt.Sprintf("token-%d", i), Data: &simpleTokenData{value: i}})
	}

	_ = execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Act: Process in batches of 10
	// EXPECTED FAILURE: BatchFire does not exist
	/*
	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)
	batchSize := 10
	batches := 0

	for input.TokenCount() > 0 {
		result, err := trans.BatchFire(ctx, batchSize)
		if err != nil {
			t.Fatalf("BatchFire failed: %v", err)
		}
		batches++

		// Atomicity: All tokens in batch should succeed or fail together
		// Partial success within a batch is NOT allowed
		if result.SuccessCount != batchSize && input.TokenCount() > 0 {
			t.Errorf("Batch %d: partial success not allowed in atomic batch", batches)
		}
	}

	// Should have processed 3 batches (30 tokens / 10 per batch)
	expectedBatches := 3
	if batches != expectedBatches {
		t.Errorf("Processed %d batches, want %d", batches, expectedBatches)
	}
	*/

	t.Log("EXPECTED FAILURE: BatchFire() atomicity semantics not defined")
	t.Log("After fix: Define atomicity guarantee:")
	t.Log("  - Each batch is atomic (all succeed or all fail)")
	t.Log("  - If batch fails, all tokens rolled back to input")
	t.Log("  - Batches are independent (batch 2 can succeed even if batch 1 fails)")
}

// TestTransition_BatchFire_VariableBatchSize tests:
// Handle batches smaller than batchSize (last batch)
//
// EXPECTED FAILURE: BatchFire() does not exist
func TestTransition_BatchFire_VariableBatchSize(t *testing.T) {
	// Arrange
	net := NewPetriNet("test-net", "Test Network")
	input := NewPlace("input", "Input Queue", 100)
	output := NewPlace("output", "Output Queue", 100)
	trans := NewTransition("process", "Process Batch")

	net.AddPlace(input)
	net.AddPlace(output)
	net.AddTransition(trans)
	net.AddArc(NewArc("a1", "input", "process", 1))
	net.AddArc(NewArc("a2", "process", "output", 1))
	net.Resolve()

	// Add 25 tokens (not evenly divisible by batch size)
	for i := 0; i < 25; i++ {
		input.AddToken(&token.Token{ID: fmt.Sprintf("token-%d", i), Data: &simpleTokenData{value: i}})
	}

	_ = execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Act: Process in batches of 10
	// EXPECTED FAILURE: BatchFire does not exist
	/*
	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)
	batchSize := 10
	batchSizes := []int{}

	for input.TokenCount() > 0 {
		result, err := trans.BatchFire(ctx, batchSize)
		if err != nil {
			t.Fatalf("BatchFire failed: %v", err)
		}

		actualBatchSize := result.SuccessCount
		batchSizes = append(batchSizes, actualBatchSize)
	}

	// Should have 3 batches: [10, 10, 5]
	expectedBatchSizes := []int{10, 10, 5}
	if len(batchSizes) != len(expectedBatchSizes) {
		t.Errorf("Got %d batches, want %d", len(batchSizes), len(expectedBatchSizes))
	}

	for i, size := range batchSizes {
		if size != expectedBatchSizes[i] {
			t.Errorf("Batch %d: size %d, want %d", i, size, expectedBatchSizes[i])
		}
	}
	*/

	t.Log("EXPECTED FAILURE: BatchFire() does not handle variable batch sizes")
	t.Log("After fix: Last batch should process remaining tokens (< batchSize)")
}

// TestTransition_BatchFire_MultipleInputs tests:
// Batch processing with multiple input arcs (e.g., weight > 1)
//
// EXPECTED FAILURE: BatchFire() does not exist and behavior undefined
func TestTransition_BatchFire_MultipleInputs(t *testing.T) {
	// Arrange
	net := NewPetriNet("test-net", "Test Network")
	inputA := NewPlace("inputA", "Input A", 100)
	inputB := NewPlace("inputB", "Input B", 100)
	output := NewPlace("output", "Output", 100)
	trans := NewTransition("process", "Join A and B")

	net.AddPlace(inputA)
	net.AddPlace(inputB)
	net.AddPlace(output)
	net.AddTransition(trans)

	// Transition requires 1 token from A and 2 from B
	net.AddArc(NewArc("a1", "inputA", "process", 1))
	net.AddArc(NewArc("a2", "inputB", "process", 2))
	net.AddArc(NewArc("a3", "process", "output", 1))
	net.Resolve()

	// Add tokens
	for i := 0; i < 10; i++ {
		inputA.AddToken(&token.Token{ID: fmt.Sprintf("tokenA-%d", i), Data: &simpleTokenData{value: i}})
	}
	for i := 0; i < 20; i++ {
		inputB.AddToken(&token.Token{ID: fmt.Sprintf("tokenB-%d", i), Data: &simpleTokenData{value: i}})
	}

	_ = execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Act: Try batch fire
	// EXPECTED FAILURE: BatchFire does not exist
	// Question: What does batchSize mean with multiple inputs?
	// - batchSize transitions fired?
	// - batchSize tokens consumed from primary input?
	// - batchSize sets of tokens (1A + 2B)?

	/*
	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)
	batchSize := 5 // Fire 5 times (consuming 5A + 10B tokens)

	result, err := trans.BatchFire(ctx, batchSize)
	if err != nil {
		t.Fatalf("BatchFire failed: %v", err)
	}

	// Should consume 5 from A, 10 from B
	if inputA.TokenCount() != 5 {
		t.Errorf("InputA: %d tokens remain, want 5", inputA.TokenCount())
	}

	if inputB.TokenCount() != 10 {
		t.Errorf("InputB: %d tokens remain, want 10", inputB.TokenCount())
	}

	if output.TokenCount() != 5 {
		t.Errorf("Output: %d tokens, want 5", output.TokenCount())
	}
	*/

	t.Log("EXPECTED FAILURE: BatchFire() with multiple inputs not defined")
	t.Log("After fix: Define semantics for transitions with weight > 1:")
	t.Log("  - batchSize = number of transition firings")
	t.Log("  - Each firing consumes arc.Weight tokens per input arc")
	t.Log("  - Batch fails if any input lacks sufficient tokens")
}

// TestTransition_BatchFire_Performance tests:
// Verify batch processing is actually more efficient than individual fires
//
// EXPECTED FAILURE: BatchFire() does not exist
func TestTransition_BatchFire_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// This test would compare:
	// 1. Time to Fire() 1000 times
	// 2. Time to BatchFire() 10 times (batch size 100)
	//
	// Expected: BatchFire should be significantly faster

	t.Log("EXPECTED FAILURE: BatchFire() does not exist")
	t.Log("After fix: Add benchmark comparing Fire vs BatchFire")
	t.Log("Expected improvement: 2-10x faster depending on task overhead")
}

// TestTransition_FireWithBatchSize tests:
// Alternative API: Fire() with optional batch size parameter
//
// EXPECTED FAILURE: Fire() does not accept batch size parameter
func TestTransition_FireWithBatchSize(t *testing.T) {
	// Alternative design: extend Fire() instead of new method
	// func (t *Transition) Fire(ctx, options ...FireOption) error
	// WithBatchSize(10) would be a FireOption

	t.Log("EXPECTED FAILURE: Fire() does not support batch size option")
	t.Log("After fix: Consider API design options:")
	t.Log("  Option 1: New method BatchFire(ctx, batchSize)")
	t.Log("  Option 2: Fire(ctx, WithBatchSize(n))")
	t.Log("  Option 3: Config on transition: t.SetBatchSize(10)")
	t.Skip("API design decision needed")
}

// TestTransition_BatchFire_WithGuard tests:
// Batch processing with guards - guard evaluated per batch or per token?
//
// EXPECTED FAILURE: BatchFire() does not exist
func TestTransition_BatchFire_WithGuard(t *testing.T) {
	// Arrange
	net := NewPetriNet("test-net", "Test Network")
	input := NewPlace("input", "Input Queue", 100)
	output := NewPlace("output", "Output Queue", 100)
	trans := NewTransition("process", "Process with Guard")

	// Guard that checks token value
	trans.WithGuard(func(ectx *execctx.ExecutionContext, tokens []*token.Token) (bool, error) {
		if len(tokens) == 0 {
			return false, nil
		}
		// Only allow tokens with even data
		data, ok := tokens[0].Data.(*simpleTokenData)
		return ok && data.value%2 == 0, nil
	})

	net.AddPlace(input)
	net.AddPlace(output)
	net.AddTransition(trans)
	net.AddArc(NewArc("a1", "input", "process", 1))
	net.AddArc(NewArc("a2", "process", "output", 1))
	net.Resolve()

	// Add mixed tokens (even and odd)
	for i := 0; i < 20; i++ {
		input.AddToken(&token.Token{
			ID:   fmt.Sprintf("token-%d", i),
			Data: &simpleTokenData{value: i},
		})
	}

	_ = execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Act: Batch fire with guard
	// EXPECTED FAILURE: Unclear how guard works with batches
	// Question: Is guard evaluated:
	// - Once per batch (on first token)?
	// - Per token in batch?
	// - On all tokens in batch collectively?

	t.Log("EXPECTED FAILURE: BatchFire() guard semantics undefined")
	t.Log("After fix: Define guard behavior for batches:")
	t.Log("  Option 1: Guard evaluated per token (filter batch)")
	t.Log("  Option 2: Guard evaluated on batch collectively")
	t.Log("  Option 3: Batching disabled when guard present")
	t.Skip("Guard semantics for batches not defined")
}
