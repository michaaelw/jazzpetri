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
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	execCtx "github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/clock"
	"github.com/jazzpetri/engine/token"
)

// TestParallelTask_Execute_Success tests successful parallel execution
func TestParallelTask_Execute_Success(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test-token"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	var count int32
	task1 := &InlineTask{
		Name: "task1",
		Fn: func(ctx *execCtx.ExecutionContext) error {
			atomic.AddInt32(&count, 1)
			return nil
		},
	}

	task2 := &InlineTask{
		Name: "task2",
		Fn: func(ctx *execCtx.ExecutionContext) error {
			atomic.AddInt32(&count, 1)
			return nil
		},
	}

	task3 := &InlineTask{
		Name: "task3",
		Fn: func(ctx *execCtx.ExecutionContext) error {
			atomic.AddInt32(&count, 1)
			return nil
		},
	}

	parallel := &ParallelTask{
		Name:  "test-parallel",
		Tasks: []Task{task1, task2, task3},
	}

	// Act
	err := parallel.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if atomic.LoadInt32(&count) != 3 {
		t.Errorf("Expected 3 tasks executed, got %d", count)
	}
}

// TestParallelTask_Execute_CollectsAllErrors tests that all errors are collected
func TestParallelTask_Execute_CollectsAllErrors(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test-token"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	err1 := errors.New("task1 failed")
	err2 := errors.New("task2 failed")

	task1 := &InlineTask{
		Name: "task1",
		Fn: func(ctx *execCtx.ExecutionContext) error {
			return err1
		},
	}

	task2 := &InlineTask{
		Name: "task2",
		Fn: func(ctx *execCtx.ExecutionContext) error {
			return err2
		},
	}

	task3 := &InlineTask{
		Name: "task3",
		Fn: func(ctx *execCtx.ExecutionContext) error {
			return nil
		},
	}

	parallel := &ParallelTask{
		Name:  "test-parallel",
		Tasks: []Task{task1, task2, task3},
	}

	// Act
	err := parallel.Execute(ctx)

	// Assert
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	parallelResult, ok := err.(*ParallelResult)
	if !ok {
		t.Fatalf("Expected *ParallelResult, got %T", err)
	}

	if parallelResult.FailureCount != 2 {
		t.Errorf("Expected 2 errors, got %d", parallelResult.FailureCount)
	}

	errMsg := parallelResult.Error()
	if !strings.Contains(errMsg, "task1 failed") {
		t.Errorf("Expected error to contain 'task1 failed', got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "task2 failed") {
		t.Errorf("Expected error to contain 'task2 failed', got: %s", errMsg)
	}
}

// TestParallelTask_Execute_TokenIsolation tests that tasks get isolated token copies
func TestParallelTask_Execute_TokenIsolation(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test-token", Data: &token.IntToken{Value: 0}}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	var tokenID1, tokenID2 string
	var mu sync.Mutex

	task1 := &InlineTask{
		Name: "task1",
		Fn: func(ctx *execCtx.ExecutionContext) error {
			mu.Lock()
			tokenID1 = ctx.Token.ID
			mu.Unlock()
			return nil
		},
	}

	task2 := &InlineTask{
		Name: "task2",
		Fn: func(ctx *execCtx.ExecutionContext) error {
			mu.Lock()
			tokenID2 = ctx.Token.ID
			mu.Unlock()
			return nil
		},
	}

	parallel := &ParallelTask{
		Name:  "test-parallel",
		Tasks: []Task{task1, task2},
	}

	// Act
	err := parallel.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Both tasks should have received a token copy with the same ID
	mu.Lock()
	defer mu.Unlock()
	if tokenID1 != tok.ID || tokenID2 != tok.ID {
		t.Errorf("Expected both tasks to receive token copies with ID %s, got %s and %s", tok.ID, tokenID1, tokenID2)
	}

	// Note: ParallelTask does shallow copy. Token.Data is shared between copies.
	// Tasks should treat token data as read-only or use immutable data structures.
}

// TestParallelTask_Execute_EmptyName tests validation for empty name
func TestParallelTask_Execute_EmptyName(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	parallel := &ParallelTask{
		Name:  "",
		Tasks: []Task{&InlineTask{Name: "task", Fn: func(ctx *execCtx.ExecutionContext) error { return nil }}},
	}

	// Act
	err := parallel.Execute(ctx)

	// Assert
	if err == nil {
		t.Fatal("Expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("Expected 'name is required' error, got: %v", err)
	}
}

// TestParallelTask_Execute_EmptyTasks tests validation for empty tasks list
func TestParallelTask_Execute_EmptyTasks(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	parallel := &ParallelTask{
		Name:  "test-parallel",
		Tasks: []Task{},
	}

	// Act
	err := parallel.Execute(ctx)

	// Assert
	if err == nil {
		t.Fatal("Expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "tasks list cannot be empty") {
		t.Errorf("Expected 'tasks list cannot be empty' error, got: %v", err)
	}
}

// TestParallelTask_Execute_Cancellation tests context cancellation
func TestParallelTask_Execute_Cancellation(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test-token"}

	goCtx, cancel := context.WithCancel(context.Background())
	ctx := execCtx.NewExecutionContext(goCtx, clk, tok)

	// Cancel immediately
	cancel()

	task1 := &InlineTask{
		Name: "task1",
		Fn: func(ctx *execCtx.ExecutionContext) error {
			// Check for cancellation
			select {
			case <-ctx.Context.Done():
				return ctx.Context.Err()
			default:
				return nil
			}
		},
	}

	parallel := &ParallelTask{
		Name:  "test-parallel",
		Tasks: []Task{task1},
	}

	// Act
	err := parallel.Execute(ctx)

	// Assert - Tasks should detect cancellation
	if err == nil {
		t.Error("Expected cancellation error, got nil")
	}
}

// TestParallelTask_Execute_Nested tests nested parallel tasks
func TestParallelTask_Execute_Nested(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test-token"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	var count int32

	// Inner parallel
	innerParallel := &ParallelTask{
		Name: "inner-parallel",
		Tasks: []Task{
			&InlineTask{
				Name: "inner-task1",
				Fn: func(ctx *execCtx.ExecutionContext) error {
					atomic.AddInt32(&count, 1)
					return nil
				},
			},
			&InlineTask{
				Name: "inner-task2",
				Fn: func(ctx *execCtx.ExecutionContext) error {
					atomic.AddInt32(&count, 1)
					return nil
				},
			},
		},
	}

	// Outer parallel
	outerParallel := &ParallelTask{
		Name: "outer-parallel",
		Tasks: []Task{
			&InlineTask{
				Name: "outer-task1",
				Fn: func(ctx *execCtx.ExecutionContext) error {
					atomic.AddInt32(&count, 1)
					return nil
				},
			},
			innerParallel,
			&InlineTask{
				Name: "outer-task2",
				Fn: func(ctx *execCtx.ExecutionContext) error {
					atomic.AddInt32(&count, 1)
					return nil
				},
			},
		},
	}

	// Act
	err := outerParallel.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if atomic.LoadInt32(&count) != 4 {
		t.Errorf("Expected 4 tasks executed, got %d", count)
	}
}

// TestParallelTask_Execute_Concurrent tests concurrent execution safety
func TestParallelTask_Execute_Concurrent(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())

	parallel := &ParallelTask{
		Name: "concurrent-parallel",
		Tasks: []Task{
			&InlineTask{
				Name: "task1",
				Fn:   func(ctx *execCtx.ExecutionContext) error { return nil },
			},
			&InlineTask{
				Name: "task2",
				Fn:   func(ctx *execCtx.ExecutionContext) error { return nil },
			},
		},
	}

	// Act - Execute parallel task concurrently
	var wg sync.WaitGroup
	errChan := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tok := &token.Token{ID: "token"}
			ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)
			err := parallel.Execute(ctx)
			if err != nil {
				errChan <- err
			}
		}()
	}

	wg.Wait()
	close(errChan)

	// Assert
	for err := range errChan {
		t.Errorf("Concurrent execution error: %v", err)
	}
}

// TestParallelTask_Execute_RaceDetection tests for race conditions
func TestParallelTask_Execute_RaceDetection(t *testing.T) {
	// This test is designed to trigger the race detector if there are issues
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test-token"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	// Create tasks that access shared state (intentionally not synchronized)
	// The ParallelTask itself should not have races in its own code
	parallel := &ParallelTask{
		Name: "race-test",
		Tasks: []Task{
			&InlineTask{Name: "task1", Fn: func(ctx *execCtx.ExecutionContext) error { return nil }},
			&InlineTask{Name: "task2", Fn: func(ctx *execCtx.ExecutionContext) error { return nil }},
			&InlineTask{Name: "task3", Fn: func(ctx *execCtx.ExecutionContext) error { return nil }},
		},
	}

	err := parallel.Execute(ctx)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}
