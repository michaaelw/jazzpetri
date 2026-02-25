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

// TestRecoveryTask_Execute_PrimarySuccess tests successful primary execution
func TestRecoveryTask_Execute_PrimarySuccess(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test-token"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	primaryExecuted := false
	compensationExecuted := false

	recovery := &RecoveryTask{
		Name: "test-recovery",
		Task: &InlineTask{
			Name: "primary",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				primaryExecuted = true
				return nil
			},
		},
		CompensationTask: &InlineTask{
			Name: "compensation",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				compensationExecuted = true
				return nil
			},
		},
	}

	// Act
	err := recovery.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if !primaryExecuted {
		t.Error("Expected primary task to execute")
	}
	if compensationExecuted {
		t.Error("Expected compensation not to execute")
	}
}

// TestRecoveryTask_Execute_CompensationSuccess tests successful compensation
func TestRecoveryTask_Execute_CompensationSuccess(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test-token"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	primaryExecuted := false
	compensationExecuted := false

	recovery := &RecoveryTask{
		Name: "test-recovery",
		Task: &InlineTask{
			Name: "primary",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				primaryExecuted = true
				return errors.New("primary failed")
			},
		},
		CompensationTask: &InlineTask{
			Name: "compensation",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				compensationExecuted = true
				return nil // Compensation succeeds
			},
		},
	}

	// Act
	err := recovery.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error after successful compensation, got: %v", err)
	}
	if !primaryExecuted {
		t.Error("Expected primary task to execute")
	}
	if !compensationExecuted {
		t.Error("Expected compensation to execute")
	}
}

// TestRecoveryTask_Execute_CompensationFailure tests failed compensation
func TestRecoveryTask_Execute_CompensationFailure(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test-token"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	primaryErr := errors.New("primary failed")
	compensationErr := errors.New("compensation failed")

	recovery := &RecoveryTask{
		Name: "test-recovery",
		Task: &InlineTask{
			Name: "primary",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				return primaryErr
			},
		},
		CompensationTask: &InlineTask{
			Name: "compensation",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				return compensationErr
			},
		},
	}

	// Act
	err := recovery.Execute(ctx)

	// Assert
	if err == nil {
		t.Fatal("Expected compensation error, got nil")
	}
	if err != compensationErr {
		t.Errorf("Expected compensation error %v, got %v", compensationErr, err)
	}
	// Note: Original error is logged but not returned
}

// TestRecoveryTask_Execute_EmptyName tests validation for empty name
func TestRecoveryTask_Execute_EmptyName(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	recovery := &RecoveryTask{
		Name:             "",
		Task:             &InlineTask{Name: "task", Fn: func(ctx *execCtx.ExecutionContext) error { return nil }},
		CompensationTask: &InlineTask{Name: "comp", Fn: func(ctx *execCtx.ExecutionContext) error { return nil }},
	}

	// Act
	err := recovery.Execute(ctx)

	// Assert
	if err == nil {
		t.Fatal("Expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("Expected 'name is required' error, got: %v", err)
	}
}

// TestRecoveryTask_Execute_NilTask tests validation for nil task
func TestRecoveryTask_Execute_NilTask(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	recovery := &RecoveryTask{
		Name:             "test",
		Task:             nil,
		CompensationTask: &InlineTask{Name: "comp", Fn: func(ctx *execCtx.ExecutionContext) error { return nil }},
	}

	// Act
	err := recovery.Execute(ctx)

	// Assert
	if err == nil {
		t.Fatal("Expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "task is required") {
		t.Errorf("Expected 'task is required' error, got: %v", err)
	}
}

// TestRecoveryTask_Execute_NilCompensation tests validation for nil compensation
func TestRecoveryTask_Execute_NilCompensation(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	recovery := &RecoveryTask{
		Name:             "test",
		Task:             &InlineTask{Name: "task", Fn: func(ctx *execCtx.ExecutionContext) error { return nil }},
		CompensationTask: nil,
	}

	// Act
	err := recovery.Execute(ctx)

	// Assert
	if err == nil {
		t.Fatal("Expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "compensation task is required") {
		t.Errorf("Expected 'compensation task is required' error, got: %v", err)
	}
}

// TestRecoveryTask_Execute_SagaPattern tests saga pattern with multiple compensations
func TestRecoveryTask_Execute_SagaPattern(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test-token", Data: &token.IntToken{Value: 0}}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	// Track operations
	operations := []string{}
	var mu sync.Mutex

	// Step 1: Reserve inventory (succeeds)
	step1 := &RecoveryTask{
		Name: "reserve-inventory",
		Task: &InlineTask{
			Name: "reserve",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				mu.Lock()
				operations = append(operations, "reserve")
				mu.Unlock()
				return nil
			},
		},
		CompensationTask: &InlineTask{
			Name: "release",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				mu.Lock()
				operations = append(operations, "release")
				mu.Unlock()
				return nil
			},
		},
	}

	// Step 2: Charge payment (succeeds)
	step2 := &RecoveryTask{
		Name: "charge-payment",
		Task: &InlineTask{
			Name: "charge",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				mu.Lock()
				operations = append(operations, "charge")
				mu.Unlock()
				return nil
			},
		},
		CompensationTask: &InlineTask{
			Name: "refund",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				mu.Lock()
				operations = append(operations, "refund")
				mu.Unlock()
				return nil
			},
		},
	}

	saga := &SequentialTask{
		Name:  "order-saga",
		Tasks: []Task{step1, step2},
	}

	// Act
	err := saga.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// Should have reserved and charged (no compensation)
	if len(operations) != 2 {
		t.Errorf("Expected 2 operations, got %d: %v", len(operations), operations)
	}
	if operations[0] != "reserve" || operations[1] != "charge" {
		t.Errorf("Expected [reserve, charge], got %v", operations)
	}
}

// TestRecoveryTask_Execute_Nested tests nested recovery tasks
func TestRecoveryTask_Execute_Nested(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test-token"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	operations := []string{}
	var mu sync.Mutex

	innerRecovery := &RecoveryTask{
		Name: "inner-recovery",
		Task: &InlineTask{
			Name: "inner-task",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				mu.Lock()
				operations = append(operations, "inner-task")
				mu.Unlock()
				return errors.New("inner failed")
			},
		},
		CompensationTask: &InlineTask{
			Name: "inner-compensation",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				mu.Lock()
				operations = append(operations, "inner-compensation")
				mu.Unlock()
				return nil
			},
		},
	}

	outerRecovery := &RecoveryTask{
		Name: "outer-recovery",
		Task: innerRecovery,
		CompensationTask: &InlineTask{
			Name: "outer-compensation",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				mu.Lock()
				operations = append(operations, "outer-compensation")
				mu.Unlock()
				return nil
			},
		},
	}

	// Act
	err := outerRecovery.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error after compensations, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// Inner task fails, inner compensation succeeds, so outer task succeeds
	if len(operations) != 2 {
		t.Errorf("Expected 2 operations, got %d: %v", len(operations), operations)
	}
	if operations[0] != "inner-task" || operations[1] != "inner-compensation" {
		t.Errorf("Expected [inner-task, inner-compensation], got %v", operations)
	}
}

// TestRecoveryTask_Execute_Concurrent tests concurrent execution safety
func TestRecoveryTask_Execute_Concurrent(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())

	recovery := &RecoveryTask{
		Name: "concurrent-recovery",
		Task: &InlineTask{
			Name: "task",
			Fn:   func(ctx *execCtx.ExecutionContext) error { return nil },
		},
		CompensationTask: &InlineTask{
			Name: "compensation",
			Fn:   func(ctx *execCtx.ExecutionContext) error { return nil },
		},
	}

	// Act - Execute recovery task concurrently
	var wg sync.WaitGroup
	errChan := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tok := &token.Token{ID: "token"}
			ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)
			err := recovery.Execute(ctx)
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
