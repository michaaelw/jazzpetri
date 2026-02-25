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
	"github.com/jazzpetri/engine/token"
)

// batchTokenData is a simple test implementation of TokenData
type batchTokenData struct {
	value int
}

func (b *batchTokenData) Type() string {
	return "batch-test"
}

// TestBatchFire_Verification tests:
// Verify BatchFire method works correctly
func TestBatchFire_Verification(t *testing.T) {
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

	// Add 100 tokens
	for i := 0; i < 100; i++ {
		tok := &token.Token{
			ID:   fmt.Sprintf("token-%d", i),
			Data: &batchTokenData{value: i},
		}
		input.AddToken(tok)
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Act: Process in batches of 10
	totalFired := 0
	for input.TokenCount() > 0 {
		fired, err := trans.BatchFire(ctx, 10)
		if err != nil {
			t.Fatalf("BatchFire failed: %v", err)
		}
		totalFired += fired
		if fired == 0 {
			break
		}
	}

	// Assert
	if totalFired != 100 {
		t.Errorf("Fired %d times, expected 100", totalFired)
	}

	if output.TokenCount() != 100 {
		t.Errorf("Output has %d tokens, want 100", output.TokenCount())
	}

	if input.TokenCount() != 0 {
		t.Errorf("Input still has %d tokens, want 0", input.TokenCount())
	}

	t.Logf("SUCCESS: BatchFire processed 100 tokens correctly")
}

// TestBatchFire_VariableBatchSize_Verification tests:
// Verify BatchFire handles non-divisible batch sizes
func TestBatchFire_VariableBatchSize_Verification(t *testing.T) {
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
		input.AddToken(&token.Token{
			ID:   fmt.Sprintf("token-%d", i),
			Data: &batchTokenData{value: i},
		})
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Act: Process in batches of 10
	batchSize := 10
	batchSizes := []int{}

	for input.TokenCount() > 0 {
		fired, err := trans.BatchFire(ctx, batchSize)
		if err != nil {
			t.Fatalf("BatchFire failed: %v", err)
		}
		if fired == 0 {
			break
		}
		batchSizes = append(batchSizes, fired)
	}

	// Assert: Should have 3 batches: [10, 10, 5]
	expectedBatchSizes := []int{10, 10, 5}
	if len(batchSizes) != len(expectedBatchSizes) {
		t.Errorf("Got %d batches, want %d", len(batchSizes), len(expectedBatchSizes))
	}

	for i, size := range batchSizes {
		if i < len(expectedBatchSizes) && size != expectedBatchSizes[i] {
			t.Errorf("Batch %d: size %d, want %d", i, size, expectedBatchSizes[i])
		}
	}

	if output.TokenCount() != 25 {
		t.Errorf("Output has %d tokens, want 25", output.TokenCount())
	}

	t.Logf("SUCCESS: BatchFire handled variable batch sizes: %v", batchSizes)
}

// TestBatchFire_InvalidBatchSize tests:
// Verify BatchFire rejects invalid batch sizes
func TestBatchFire_InvalidBatchSize(t *testing.T) {
	// Arrange
	net := NewPetriNet("test-net", "Test Network")
	input := NewPlace("input", "Input Queue", 100)
	output := NewPlace("output", "Output Queue", 100)
	trans := NewTransition("process", "Process")

	net.AddPlace(input)
	net.AddPlace(output)
	net.AddTransition(trans)
	net.AddArc(NewArc("a1", "input", "process", 1))
	net.AddArc(NewArc("a2", "process", "output", 1))
	net.Resolve()

	input.AddToken(&token.Token{ID: "t1", Data: &batchTokenData{value: 1}})

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Act & Assert: Test invalid batch sizes
	tests := []struct {
		name      string
		batchSize int
		wantErr   bool
	}{
		{"zero batch size", 0, true},
		{"negative batch size", -1, true},
		{"positive batch size", 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := trans.BatchFire(ctx, tt.batchSize)
			if (err != nil) != tt.wantErr {
				t.Errorf("BatchFire(%d) error = %v, wantErr %v", tt.batchSize, err, tt.wantErr)
			}
		})
	}

	t.Log("SUCCESS: BatchFire validates batch size correctly")
}
