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
	"testing"
	"time"

	execCtx "github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/clock"
	"github.com/jazzpetri/engine/token"
)

// TestSequentialTask_Execute_Success tests successful execution of all tasks
func TestSequentialTask_Execute_Success(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test-token", Data: &token.IntToken{Value: 0}}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	executionOrder := []int{}
	var mu sync.Mutex

	task1 := &InlineTask{
		Name: "task1",
		Fn: func(ctx *execCtx.ExecutionContext) error {
			mu.Lock()
			executionOrder = append(executionOrder, 1)
			ctx.Token.Data.(*token.IntToken).Value += 1
			mu.Unlock()
			return nil
		},
	}

	task2 := &InlineTask{
		Name: "task2",
		Fn: func(ctx *execCtx.ExecutionContext) error {
			mu.Lock()
			executionOrder = append(executionOrder, 2)
			ctx.Token.Data.(*token.IntToken).Value += 10
			mu.Unlock()
			return nil
		},
	}

	task3 := &InlineTask{
		Name: "task3",
		Fn: func(ctx *execCtx.ExecutionContext) error {
			mu.Lock()
			executionOrder = append(executionOrder, 3)
			ctx.Token.Data.(*token.IntToken).Value += 100
			mu.Unlock()
			return nil
		},
	}

	seq := &SequentialTask{
		Name:  "test-sequence",
		Tasks: []Task{task1, task2, task3},
	}

	// Act
	err := seq.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify execution order
	if len(executionOrder) != 3 {
		t.Fatalf("Expected 3 tasks executed, got %d", len(executionOrder))
	}
	if executionOrder[0] != 1 || executionOrder[1] != 2 || executionOrder[2] != 3 {
		t.Errorf("Expected order [1,2,3], got %v", executionOrder)
	}

	// Verify token was modified by all tasks (1 + 10 + 100 = 111)
	if tok.Data.(*token.IntToken).Value != 111 {
		t.Errorf("Expected token data to be 111, got %d", tok.Data.(*token.IntToken).Value)
	}
}

// TestSequentialTask_Execute_StopsOnError tests that execution stops at first error
func TestSequentialTask_Execute_StopsOnError(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test-token"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	executionCount := 0
	var mu sync.Mutex

	task1 := &InlineTask{
		Name: "task1",
		Fn: func(ctx *execCtx.ExecutionContext) error {
			mu.Lock()
			executionCount++
			mu.Unlock()
			return nil
		},
	}

	expectedErr := errors.New("task2 failed")
	task2 := &InlineTask{
		Name: "task2",
		Fn: func(ctx *execCtx.ExecutionContext) error {
			mu.Lock()
			executionCount++
			mu.Unlock()
			return expectedErr
		},
	}

	task3 := &InlineTask{
		Name: "task3",
		Fn: func(ctx *execCtx.ExecutionContext) error {
			mu.Lock()
			executionCount++
			mu.Unlock()
			return nil
		},
	}

	seq := &SequentialTask{
		Name:  "test-sequence",
		Tasks: []Task{task1, task2, task3},
	}

	// Act
	err := seq.Execute(ctx)

	// Assert
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}

	// Only first two tasks should have executed
	mu.Lock()
	defer mu.Unlock()
	if executionCount != 2 {
		t.Errorf("Expected 2 tasks executed, got %d", executionCount)
	}
}

// TestSequentialTask_Execute_EmptyName tests validation for empty name
func TestSequentialTask_Execute_EmptyName(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	seq := &SequentialTask{
		Name:  "",
		Tasks: []Task{&InlineTask{Name: "task", Fn: func(ctx *execCtx.ExecutionContext) error { return nil }}},
	}

	// Act
	err := seq.Execute(ctx)

	// Assert
	if err == nil {
		t.Fatal("Expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("Expected 'name is required' error, got: %v", err)
	}
}

// TestSequentialTask_Execute_EmptyTasks tests validation for empty tasks list
func TestSequentialTask_Execute_EmptyTasks(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	seq := &SequentialTask{
		Name:  "test-sequence",
		Tasks: []Task{},
	}

	// Act
	err := seq.Execute(ctx)

	// Assert
	if err == nil {
		t.Fatal("Expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "tasks list cannot be empty") {
		t.Errorf("Expected 'tasks list cannot be empty' error, got: %v", err)
	}
}

// TestSequentialTask_Execute_Cancellation tests context cancellation
func TestSequentialTask_Execute_Cancellation(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test-token"}

	// Create cancellable context
	goCtx, cancel := context.WithCancel(context.Background())
	ctx := execCtx.NewExecutionContext(goCtx, clk, tok)

	executionCount := 0
	var mu sync.Mutex

	task1 := &InlineTask{
		Name: "task1",
		Fn: func(ctx *execCtx.ExecutionContext) error {
			mu.Lock()
			executionCount++
			mu.Unlock()
			// Cancel after first task
			cancel()
			return nil
		},
	}

	task2 := &InlineTask{
		Name: "task2",
		Fn: func(ctx *execCtx.ExecutionContext) error {
			mu.Lock()
			executionCount++
			mu.Unlock()
			return nil
		},
	}

	seq := &SequentialTask{
		Name:  "test-sequence",
		Tasks: []Task{task1, task2},
	}

	// Act
	err := seq.Execute(ctx)

	// Assert
	if err == nil {
		t.Fatal("Expected cancellation error, got nil")
	}
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got: %v", err)
	}

	// Only first task should have executed
	mu.Lock()
	defer mu.Unlock()
	if executionCount != 1 {
		t.Errorf("Expected 1 task executed, got %d", executionCount)
	}
}

// TestSequentialTask_Execute_Nested tests nested sequential tasks
func TestSequentialTask_Execute_Nested(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test-token", Data: &token.IntToken{Value: 0}}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	// Inner sequence
	innerSeq := &SequentialTask{
		Name: "inner-sequence",
		Tasks: []Task{
			&InlineTask{
				Name: "inner-task1",
				Fn: func(ctx *execCtx.ExecutionContext) error {
					ctx.Token.Data.(*token.IntToken).Value += 1
					return nil
				},
			},
			&InlineTask{
				Name: "inner-task2",
				Fn: func(ctx *execCtx.ExecutionContext) error {
					ctx.Token.Data.(*token.IntToken).Value += 10
					return nil
				},
			},
		},
	}

	// Outer sequence
	outerSeq := &SequentialTask{
		Name: "outer-sequence",
		Tasks: []Task{
			&InlineTask{
				Name: "outer-task1",
				Fn: func(ctx *execCtx.ExecutionContext) error {
					ctx.Token.Data.(*token.IntToken).Value += 100
					return nil
				},
			},
			innerSeq,
			&InlineTask{
				Name: "outer-task2",
				Fn: func(ctx *execCtx.ExecutionContext) error {
					ctx.Token.Data.(*token.IntToken).Value += 1000
					return nil
				},
			},
		},
	}

	// Act
	err := outerSeq.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify token was modified by all tasks (100 + 1 + 10 + 1000 = 1111)
	if tok.Data.(*token.IntToken).Value != 1111 {
		t.Errorf("Expected token data to be 1111, got %d", tok.Data.(*token.IntToken).Value)
	}
}

// TestSequentialTask_Execute_Concurrent tests concurrent execution safety
func TestSequentialTask_Execute_Concurrent(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())

	seq := &SequentialTask{
		Name: "concurrent-sequence",
		Tasks: []Task{
			&InlineTask{
				Name: "task1",
				Fn: func(ctx *execCtx.ExecutionContext) error {
					ctx.Token.Data.(*token.IntToken).Value += 1
					return nil
				},
			},
			&InlineTask{
				Name: "task2",
				Fn: func(ctx *execCtx.ExecutionContext) error {
					ctx.Token.Data.(*token.IntToken).Value *= 2
					return nil
				},
			},
		},
	}

	// Act - Execute sequence concurrently with different tokens
	var wg sync.WaitGroup
	errChan := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			tok := &token.Token{ID: "token", Data: &token.IntToken{Value: val}}
			ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)
			err := seq.Execute(ctx)
			if err != nil {
				errChan <- err
			}
			// Verify calculation: (val + 1) * 2
			expected := (val + 1) * 2
			if tok.Data.(*token.IntToken).Value != expected {
				errChan <- errors.New("token calculation incorrect")
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	// Assert
	for err := range errChan {
		t.Errorf("Concurrent execution error: %v", err)
	}
}
