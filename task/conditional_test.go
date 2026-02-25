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

// TestConditionalTask_Execute_ThenBranch tests execution of then branch
func TestConditionalTask_Execute_ThenBranch(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test-token", Data: &token.IntToken{Value: 10}}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	thenExecuted := false
	elseExecuted := false

	conditional := &ConditionalTask{
		Name: "test-conditional",
		Condition: func(ctx *execCtx.ExecutionContext) bool {
			return ctx.Token.Data.(*token.IntToken).Value > 5
		},
		ThenTask: &InlineTask{
			Name: "then-task",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				thenExecuted = true
				return nil
			},
		},
		ElseTask: &InlineTask{
			Name: "else-task",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				elseExecuted = true
				return nil
			},
		},
	}

	// Act
	err := conditional.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if !thenExecuted {
		t.Error("Expected then branch to execute")
	}
	if elseExecuted {
		t.Error("Expected else branch not to execute")
	}
}

// TestConditionalTask_Execute_ElseBranch tests execution of else branch
func TestConditionalTask_Execute_ElseBranch(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test-token", Data: &token.IntToken{Value: 3}}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	thenExecuted := false
	elseExecuted := false

	conditional := &ConditionalTask{
		Name: "test-conditional",
		Condition: func(ctx *execCtx.ExecutionContext) bool {
			return ctx.Token.Data.(*token.IntToken).Value > 5
		},
		ThenTask: &InlineTask{
			Name: "then-task",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				thenExecuted = true
				return nil
			},
		},
		ElseTask: &InlineTask{
			Name: "else-task",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				elseExecuted = true
				return nil
			},
		},
	}

	// Act
	err := conditional.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if thenExecuted {
		t.Error("Expected then branch not to execute")
	}
	if !elseExecuted {
		t.Error("Expected else branch to execute")
	}
}

// TestConditionalTask_Execute_NoElseTask tests optional else task
func TestConditionalTask_Execute_NoElseTask(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test-token", Data: &token.IntToken{Value: 3}}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	thenExecuted := false

	conditional := &ConditionalTask{
		Name: "test-conditional",
		Condition: func(ctx *execCtx.ExecutionContext) bool {
			return ctx.Token.Data.(*token.IntToken).Value > 5
		},
		ThenTask: &InlineTask{
			Name: "then-task",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				thenExecuted = true
				return nil
			},
		},
		ElseTask: nil, // No else task
	}

	// Act
	err := conditional.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if thenExecuted {
		t.Error("Expected then branch not to execute")
	}
	// Should complete successfully without else task
}

// TestConditionalTask_Execute_ThenError tests error in then branch
func TestConditionalTask_Execute_ThenError(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test-token", Data: &token.IntToken{Value: 10}}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	expectedErr := errors.New("then task failed")

	conditional := &ConditionalTask{
		Name: "test-conditional",
		Condition: func(ctx *execCtx.ExecutionContext) bool {
			return true
		},
		ThenTask: &InlineTask{
			Name: "then-task",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				return expectedErr
			},
		},
	}

	// Act
	err := conditional.Execute(ctx)

	// Assert
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}

// TestConditionalTask_Execute_ElseError tests error in else branch
func TestConditionalTask_Execute_ElseError(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test-token"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	expectedErr := errors.New("else task failed")

	conditional := &ConditionalTask{
		Name: "test-conditional",
		Condition: func(ctx *execCtx.ExecutionContext) bool {
			return false
		},
		ThenTask: &InlineTask{
			Name: "then-task",
			Fn:   func(ctx *execCtx.ExecutionContext) error { return nil },
		},
		ElseTask: &InlineTask{
			Name: "else-task",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				return expectedErr
			},
		},
	}

	// Act
	err := conditional.Execute(ctx)

	// Assert
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}

// TestConditionalTask_Execute_ConditionPanic tests panic recovery in condition
func TestConditionalTask_Execute_ConditionPanic(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test-token"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	conditional := &ConditionalTask{
		Name: "test-conditional",
		Condition: func(ctx *execCtx.ExecutionContext) bool {
			panic("condition failed")
		},
		ThenTask: &InlineTask{
			Name: "then-task",
			Fn:   func(ctx *execCtx.ExecutionContext) error { return nil },
		},
	}

	// Act
	err := conditional.Execute(ctx)

	// Assert
	if err == nil {
		t.Fatal("Expected error from panic, got nil")
	}
	if !strings.Contains(err.Error(), "panicked") {
		t.Errorf("Expected panic error message, got: %v", err)
	}
}

// TestConditionalTask_Execute_EmptyName tests validation for empty name
func TestConditionalTask_Execute_EmptyName(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	conditional := &ConditionalTask{
		Name:      "",
		Condition: func(ctx *execCtx.ExecutionContext) bool { return true },
		ThenTask:  &InlineTask{Name: "task", Fn: func(ctx *execCtx.ExecutionContext) error { return nil }},
	}

	// Act
	err := conditional.Execute(ctx)

	// Assert
	if err == nil {
		t.Fatal("Expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("Expected 'name is required' error, got: %v", err)
	}
}

// TestConditionalTask_Execute_NilCondition tests validation for nil condition
func TestConditionalTask_Execute_NilCondition(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	conditional := &ConditionalTask{
		Name:      "test",
		Condition: nil,
		ThenTask:  &InlineTask{Name: "task", Fn: func(ctx *execCtx.ExecutionContext) error { return nil }},
	}

	// Act
	err := conditional.Execute(ctx)

	// Assert
	if err == nil {
		t.Fatal("Expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "condition function is required") {
		t.Errorf("Expected 'condition function is required' error, got: %v", err)
	}
}

// TestConditionalTask_Execute_NilThenTask tests validation for nil then task
func TestConditionalTask_Execute_NilThenTask(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	conditional := &ConditionalTask{
		Name:      "test",
		Condition: func(ctx *execCtx.ExecutionContext) bool { return true },
		ThenTask:  nil,
	}

	// Act
	err := conditional.Execute(ctx)

	// Assert
	if err == nil {
		t.Fatal("Expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "then task is required") {
		t.Errorf("Expected 'then task is required' error, got: %v", err)
	}
}

// TestConditionalTask_Execute_Nested tests nested conditionals
func TestConditionalTask_Execute_Nested(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test-token", Data: &token.IntToken{Value: 7}}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	var result string
	var mu sync.Mutex

	innerConditional := &ConditionalTask{
		Name: "inner-conditional",
		Condition: func(ctx *execCtx.ExecutionContext) bool {
			return ctx.Token.Data.(*token.IntToken).Value > 5
		},
		ThenTask: &InlineTask{
			Name: "inner-then",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				mu.Lock()
				result = "high"
				mu.Unlock()
				return nil
			},
		},
		ElseTask: &InlineTask{
			Name: "inner-else",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				mu.Lock()
				result = "medium"
				mu.Unlock()
				return nil
			},
		},
	}

	outerConditional := &ConditionalTask{
		Name: "outer-conditional",
		Condition: func(ctx *execCtx.ExecutionContext) bool {
			return ctx.Token.Data.(*token.IntToken).Value > 0
		},
		ThenTask: innerConditional,
		ElseTask: &InlineTask{
			Name: "outer-else",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				mu.Lock()
				result = "low"
				mu.Unlock()
				return nil
			},
		},
	}

	// Act
	err := outerConditional.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if result != "high" {
		t.Errorf("Expected result 'high', got '%s'", result)
	}
}

// TestConditionalTask_Execute_Concurrent tests concurrent execution safety
func TestConditionalTask_Execute_Concurrent(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())

	conditional := &ConditionalTask{
		Name: "concurrent-conditional",
		Condition: func(ctx *execCtx.ExecutionContext) bool {
			return ctx.Token.Data.(*token.IntToken).Value > 5
		},
		ThenTask: &InlineTask{
			Name: "then-task",
			Fn:   func(ctx *execCtx.ExecutionContext) error { return nil },
		},
		ElseTask: &InlineTask{
			Name: "else-task",
			Fn:   func(ctx *execCtx.ExecutionContext) error { return nil },
		},
	}

	// Act - Execute conditionally concurrently
	var wg sync.WaitGroup
	errChan := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			tok := &token.Token{ID: "token", Data: &token.IntToken{Value: val}}
			ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)
			err := conditional.Execute(ctx)
			if err != nil {
				errChan <- err
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
