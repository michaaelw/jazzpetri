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

// TestRetryTask_Execute_SuccessFirstAttempt tests success on first attempt
func TestRetryTask_Execute_SuccessFirstAttempt(t *testing.T) {
	// Arrange
	clk := clock.NewRealTimeClock()
	tok := &token.Token{ID: "test-token"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	var attempts int32

	retry := &RetryTask{
		Name: "test-retry",
		Task: &InlineTask{
			Name: "task",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				atomic.AddInt32(&attempts, 1)
				return nil
			},
		},
		MaxAttempts: 3,
		Backoff:     100 * time.Millisecond,
	}

	// Act
	err := retry.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("Expected 1 attempt, got %d", attempts)
	}
}

// TestRetryTask_Execute_SuccessAfterRetries tests success after some retries
func TestRetryTask_Execute_SuccessAfterRetries(t *testing.T) {
	// Arrange
	clk := clock.NewRealTimeClock() // Use real clock for timing tests
	tok := &token.Token{ID: "test-token"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	var attempts int32

	retry := &RetryTask{
		Name: "test-retry",
		Task: &InlineTask{
			Name: "task",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				count := atomic.AddInt32(&attempts, 1)
				if count < 3 {
					return errors.New("temporary failure")
				}
				return nil
			},
		},
		MaxAttempts: 5,
		Backoff:     1 * time.Millisecond, // Very short for testing
	}

	// Act
	err := retry.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

// TestRetryTask_Execute_FailureAllAttempts tests failure after all attempts
func TestRetryTask_Execute_FailureAllAttempts(t *testing.T) {
	// Arrange
	clk := clock.NewRealTimeClock() // Use real clock for timing tests
	tok := &token.Token{ID: "test-token"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	var attempts int32
	expectedErr := errors.New("persistent failure")

	retry := &RetryTask{
		Name: "test-retry",
		Task: &InlineTask{
			Name: "task",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				atomic.AddInt32(&attempts, 1)
				return expectedErr
			},
		},
		MaxAttempts: 3,
		Backoff:     1 * time.Millisecond, // Very short for testing
	}

	// Act
	err := retry.Execute(ctx)

	// Assert
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

// TestRetryTask_Execute_ExponentialBackoff tests custom backoff function
func TestRetryTask_Execute_ExponentialBackoff(t *testing.T) {
	// Arrange
	clk := clock.NewRealTimeClock()
	tok := &token.Token{ID: "test-token"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	var attempts int32
	backoffCalls := []int{}
	var mu sync.Mutex

	retry := &RetryTask{
		Name: "test-retry",
		Task: &InlineTask{
			Name: "task",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				count := atomic.AddInt32(&attempts, 1)
				if count < 4 {
					return errors.New("failure")
				}
				return nil
			},
		},
		MaxAttempts: 5,
		BackoffFunc: func(attempt int) time.Duration {
			mu.Lock()
			backoffCalls = append(backoffCalls, attempt)
			mu.Unlock()
			// Exponential: 10ms, 20ms, 40ms...
			return time.Duration(1<<uint(attempt)) * 1 * time.Millisecond
		},
	}

	// Act
	err := retry.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if atomic.LoadInt32(&attempts) != 4 {
		t.Errorf("Expected 4 attempts, got %d", attempts)
	}

	mu.Lock()
	defer mu.Unlock()
	// Should have called backoff 3 times (after attempts 1, 2, 3)
	if len(backoffCalls) != 3 {
		t.Errorf("Expected 3 backoff calls, got %d", len(backoffCalls))
	}
}

// TestRetryTask_Execute_Cancellation tests context cancellation
func TestRetryTask_Execute_Cancellation(t *testing.T) {
	// Arrange
	clk := clock.NewRealTimeClock()
	tok := &token.Token{ID: "test-token"}

	goCtx, cancel := context.WithCancel(context.Background())
	ctx := execCtx.NewExecutionContext(goCtx, clk, tok)

	var attempts int32

	retry := &RetryTask{
		Name: "test-retry",
		Task: &InlineTask{
			Name: "task",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				count := atomic.AddInt32(&attempts, 1)
				if count == 2 {
					// Cancel after second attempt
					cancel()
				}
				return errors.New("failure")
			},
		},
		MaxAttempts: 5,
		Backoff:     1 * time.Millisecond,
	}

	// Act
	err := retry.Execute(ctx)

	// Assert
	if err == nil {
		t.Fatal("Expected cancellation error, got nil")
	}
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got: %v", err)
	}
	// Should have attempted twice before cancellation check
	if atomic.LoadInt32(&attempts) != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

// TestRetryTask_Execute_CancellationDuringBackoff tests cancellation during backoff
func TestRetryTask_Execute_CancellationDuringBackoff(t *testing.T) {
	// Arrange
	clk := clock.NewRealTimeClock()
	tok := &token.Token{ID: "test-token"}

	goCtx, cancel := context.WithCancel(context.Background())
	ctx := execCtx.NewExecutionContext(goCtx, clk, tok)

	var attempts int32

	retry := &RetryTask{
		Name: "test-retry",
		Task: &InlineTask{
			Name: "task",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				atomic.AddInt32(&attempts, 1)
				return errors.New("failure")
			},
		},
		MaxAttempts: 5,
		Backoff:     1 * time.Second, // Long backoff
	}

	// Start execution in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- retry.Execute(ctx)
	}()

	// Give it time to fail first attempt and start backoff
	time.Sleep(50 * time.Millisecond)

	// Cancel during backoff
	cancel()

	// Wait for result
	err := <-errChan

	// Assert
	if err == nil {
		t.Fatal("Expected cancellation error, got nil")
	}
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got: %v", err)
	}
	// Should have attempted once before cancellation during backoff
	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("Expected 1 attempt, got %d", attempts)
	}
}

// TestRetryTask_Execute_EmptyName tests validation for empty name
func TestRetryTask_Execute_EmptyName(t *testing.T) {
	// Arrange
	clk := clock.NewRealTimeClock()
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	retry := &RetryTask{
		Name:        "",
		Task:        &InlineTask{Name: "task", Fn: func(ctx *execCtx.ExecutionContext) error { return nil }},
		MaxAttempts: 3,
	}

	// Act
	err := retry.Execute(ctx)

	// Assert
	if err == nil {
		t.Fatal("Expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("Expected 'name is required' error, got: %v", err)
	}
}

// TestRetryTask_Execute_NilTask tests validation for nil task
func TestRetryTask_Execute_NilTask(t *testing.T) {
	// Arrange
	clk := clock.NewRealTimeClock()
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	retry := &RetryTask{
		Name:        "test",
		Task:        nil,
		MaxAttempts: 3,
	}

	// Act
	err := retry.Execute(ctx)

	// Assert
	if err == nil {
		t.Fatal("Expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "task is required") {
		t.Errorf("Expected 'task is required' error, got: %v", err)
	}
}

// TestRetryTask_Execute_InvalidMaxAttempts tests validation for invalid max attempts
func TestRetryTask_Execute_InvalidMaxAttempts(t *testing.T) {
	// Arrange
	clk := clock.NewRealTimeClock()
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	retry := &RetryTask{
		Name:        "test",
		Task:        &InlineTask{Name: "task", Fn: func(ctx *execCtx.ExecutionContext) error { return nil }},
		MaxAttempts: 0,
	}

	// Act
	err := retry.Execute(ctx)

	// Assert
	if err == nil {
		t.Fatal("Expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "max attempts must be at least 1") {
		t.Errorf("Expected 'max attempts must be at least 1' error, got: %v", err)
	}
}

// TestRetryTask_Execute_Nested tests nested retry tasks
func TestRetryTask_Execute_Nested(t *testing.T) {
	// Arrange
	clk := clock.NewRealTimeClock()
	tok := &token.Token{ID: "test-token"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	var attempts int32

	innerRetry := &RetryTask{
		Name: "inner-retry",
		Task: &InlineTask{
			Name: "task",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				count := atomic.AddInt32(&attempts, 1)
				if count < 2 {
					return errors.New("failure")
				}
				return nil
			},
		},
		MaxAttempts: 3,
		Backoff:     1 * time.Millisecond,
	}

	outerRetry := &RetryTask{
		Name:        "outer-retry",
		Task:        innerRetry,
		MaxAttempts: 2,
		Backoff:     1 * time.Millisecond,
	}

	// Act
	err := outerRetry.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	// Inner retry should succeed on 2nd attempt
	if atomic.LoadInt32(&attempts) != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

// TestRetryTask_Execute_Concurrent tests concurrent execution safety
func TestRetryTask_Execute_Concurrent(t *testing.T) {
	// Arrange
	clk := clock.NewRealTimeClock()

	retry := &RetryTask{
		Name: "concurrent-retry",
		Task: &InlineTask{
			Name: "task",
			Fn:   func(ctx *execCtx.ExecutionContext) error { return nil },
		},
		MaxAttempts: 3,
		Backoff:     1 * time.Millisecond,
	}

	// Act - Execute retry task concurrently
	var wg sync.WaitGroup
	errChan := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tok := &token.Token{ID: "token"}
			ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)
			err := retry.Execute(ctx)
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
