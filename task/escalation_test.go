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
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jazzpetri/engine/clock"
	execCtx "github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/token"
)

// waitForTimer waits for the virtual clock to have pending timers.
func waitForTimer(clk *clock.VirtualClock, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if clk.PendingTimers() > 0 {
			return true
		}
		time.Sleep(1 * time.Millisecond)
	}
	return false
}

// TestEscalationTask_TimeBasedMultiLevel tests time-based escalation through multiple levels.
func TestEscalationTask_TimeBasedMultiLevel(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "escalation-1"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	var escalations []EscalationLevel
	var mu sync.Mutex

	task := &EscalationTask{
		Name: "time-based-escalation",
		Levels: []EscalationConfig{
			{Level: EscalationLevel1, EscalateTo: []string{"team"}, After: 5 * time.Minute},
			{Level: EscalationLevel2, EscalateTo: []string{"manager"}, After: 10 * time.Minute},
			{Level: EscalationLevel3, EscalateTo: []string{"director"}, After: 15 * time.Minute},
		},
		CheckInterval: 1 * time.Minute,
		OnEscalate: func(ctx *execCtx.ExecutionContext, level EscalationLevel, recipients []string) error {
			mu.Lock()
			defer mu.Unlock()
			escalations = append(escalations, level)
			return nil
		},
	}

	// Act - run task in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- task.Execute(ctx)
	}()

	// Wait for timer to be registered before advancing
	if !waitForTimer(clk, 100*time.Millisecond) {
		t.Fatal("Timeout waiting for timer to be registered")
	}

	// Advance clock through all escalation levels
	clk.AdvanceBy(5 * time.Minute) // Level 1
	if !waitForTimer(clk, 100*time.Millisecond) {
		t.Fatal("Timeout waiting for timer after level 1")
	}
	clk.AdvanceBy(10 * time.Minute) // Level 2
	if !waitForTimer(clk, 100*time.Millisecond) {
		t.Fatal("Timeout waiting for timer after level 2")
	}
	clk.AdvanceBy(15 * time.Minute) // Level 3

	// Wait for completion
	err := <-errCh

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(escalations) != 3 {
		t.Errorf("Expected 3 escalations, got %d", len(escalations))
	}

	expectedLevels := []EscalationLevel{EscalationLevel1, EscalationLevel2, EscalationLevel3}
	for i, level := range expectedLevels {
		if i >= len(escalations) {
			t.Errorf("Missing escalation at index %d", i)
			continue
		}
		if escalations[i] != level {
			t.Errorf("Escalation %d: expected %s, got %s", i, level.String(), escalations[i].String())
		}
	}
}

// TestEscalationTask_ConditionBasedEscalation tests condition-based escalation trigger.
func TestEscalationTask_ConditionBasedEscalation(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "escalation-2"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	conditionMet := false

	task := &EscalationTask{
		Name: "condition-based",
		Levels: []EscalationConfig{
			{Level: EscalationLevel1, EscalateTo: []string{"team"}, After: 1 * time.Minute},
		},
		CheckInterval: 1 * time.Second,
		EscalateWhen: func(ctx *execCtx.ExecutionContext) (bool, error) {
			return conditionMet, nil
		},
		OnEscalate: func(ctx *execCtx.ExecutionContext, level EscalationLevel, recipients []string) error {
			return nil
		},
	}

	// Act 1 - condition not met, should complete immediately
	err := task.Execute(ctx)

	// Assert 1
	if err != nil {
		t.Fatalf("Expected no error when condition not met, got: %v", err)
	}

	// Act 2 - condition met, should escalate
	conditionMet = true
	escalated := false

	task.OnEscalate = func(ctx *execCtx.ExecutionContext, level EscalationLevel, recipients []string) error {
		escalated = true
		return nil
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- task.Execute(ctx)
	}()

	if !waitForTimer(clk, 100*time.Millisecond) {
		t.Fatal("Timeout waiting for timer to be registered")
	}
	clk.AdvanceBy(1 * time.Minute)
	err = <-errCh

	// Assert 2
	if err != nil {
		t.Fatalf("Expected no error when condition met, got: %v", err)
	}
	if !escalated {
		t.Error("Expected escalation to occur when condition met")
	}
}

// TestEscalationTask_ResolutionBeforeEscalation tests resolution before first escalation level.
func TestEscalationTask_ResolutionBeforeEscalation(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "escalation-3"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	resolver := NewChannelEscalationResolver()
	escalated := false
	resolved := false

	task := &EscalationTask{
		Name: "resolution-before",
		Levels: []EscalationConfig{
			{Level: EscalationLevel1, EscalateTo: []string{"team"}, After: 5 * time.Minute},
		},
		CheckInterval: 1 * time.Minute,
		Resolver:      resolver,
		OnEscalate: func(ctx *execCtx.ExecutionContext, level EscalationLevel, recipients []string) error {
			escalated = true
			return nil
		},
		OnResolved: func(ctx *execCtx.ExecutionContext, level EscalationLevel) error {
			resolved = true
			if level != EscalationLevelNone {
				t.Errorf("Expected resolution at none level, got %s", level.String())
			}
			return nil
		},
	}

	// Act - run task and resolve before escalation
	errCh := make(chan error, 1)
	go func() {
		errCh <- task.Execute(ctx)
	}()

	// Wait for timer, advance clock partway, then resolve
	if !waitForTimer(clk, 100*time.Millisecond) {
		t.Fatal("Timeout waiting for timer to be registered")
	}
	clk.AdvanceBy(2 * time.Minute)
	time.Sleep(10 * time.Millisecond)
	resolver.Resolve()

	err := <-errCh

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if escalated {
		t.Error("Expected no escalation when resolved early")
	}
	if !resolved {
		t.Error("Expected OnResolved to be called")
	}
}

// TestEscalationTask_ResolutionDuringEscalation tests resolution during escalation chain.
func TestEscalationTask_ResolutionDuringEscalation(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "escalation-4"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	resolver := NewChannelEscalationResolver()
	var escalations []EscalationLevel
	var mu sync.Mutex
	resolvedAtLevel := EscalationLevelNone

	task := &EscalationTask{
		Name: "resolution-during",
		Levels: []EscalationConfig{
			{Level: EscalationLevel1, EscalateTo: []string{"team"}, After: 5 * time.Minute},
			{Level: EscalationLevel2, EscalateTo: []string{"manager"}, After: 10 * time.Minute},
			{Level: EscalationLevel3, EscalateTo: []string{"director"}, After: 15 * time.Minute},
		},
		CheckInterval: 1 * time.Minute,
		Resolver:      resolver,
		OnEscalate: func(ctx *execCtx.ExecutionContext, level EscalationLevel, recipients []string) error {
			mu.Lock()
			defer mu.Unlock()
			escalations = append(escalations, level)
			return nil
		},
		OnResolved: func(ctx *execCtx.ExecutionContext, level EscalationLevel) error {
			resolvedAtLevel = level
			return nil
		},
	}

	// Act - run task, escalate to level 1, then resolve
	errCh := make(chan error, 1)
	go func() {
		errCh <- task.Execute(ctx)
	}()

	// Wait for timer, then advance to trigger level 1
	if !waitForTimer(clk, 100*time.Millisecond) {
		t.Fatal("Timeout waiting for timer to be registered")
	}
	clk.AdvanceBy(5 * time.Minute)

	// Wait for next timer, advance partway to level 2, then resolve
	if !waitForTimer(clk, 100*time.Millisecond) {
		t.Fatal("Timeout waiting for timer after level 1")
	}
	clk.AdvanceBy(3 * time.Minute)
	time.Sleep(10 * time.Millisecond)
	resolver.Resolve()

	err := <-errCh

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(escalations) != 1 {
		t.Errorf("Expected 1 escalation, got %d", len(escalations))
	}
	if len(escalations) > 0 && escalations[0] != EscalationLevel1 {
		t.Errorf("Expected escalation to level 1, got %s", escalations[0].String())
	}
	if resolvedAtLevel != EscalationLevel1 {
		t.Errorf("Expected resolution at level 1, got %s", resolvedAtLevel.String())
	}
}

// TestEscalationTask_NoResolver tests escalation without resolver (runs to completion).
func TestEscalationTask_NoResolver(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "escalation-5"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	var escalations []EscalationLevel
	var mu sync.Mutex

	task := &EscalationTask{
		Name: "no-resolver",
		Levels: []EscalationConfig{
			{Level: EscalationLevel1, EscalateTo: []string{"team"}, After: 2 * time.Minute},
			{Level: EscalationLevel2, EscalateTo: []string{"manager"}, After: 3 * time.Minute},
		},
		CheckInterval: 1 * time.Minute,
		Resolver:      nil, // No resolver
		OnEscalate: func(ctx *execCtx.ExecutionContext, level EscalationLevel, recipients []string) error {
			mu.Lock()
			defer mu.Unlock()
			escalations = append(escalations, level)
			return nil
		},
	}

	// Act
	errCh := make(chan error, 1)
	go func() {
		errCh <- task.Execute(ctx)
	}()

	if !waitForTimer(clk, 100*time.Millisecond) {
		t.Fatal("Timeout waiting for timer to be registered")
	}
	clk.AdvanceBy(2 * time.Minute)
	if !waitForTimer(clk, 100*time.Millisecond) {
		t.Fatal("Timeout waiting for timer after level 1")
	}
	clk.AdvanceBy(3 * time.Minute)

	err := <-errCh

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(escalations) != 2 {
		t.Errorf("Expected 2 escalations, got %d", len(escalations))
	}
}

// TestEscalationTask_ContextCancellation tests cancellation during escalation.
func TestEscalationTask_ContextCancellation(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "escalation-6"}
	goCtx, cancel := context.WithCancel(context.Background())
	ctx := execCtx.NewExecutionContext(goCtx, clk, tok)

	task := &EscalationTask{
		Name: "context-cancel",
		Levels: []EscalationConfig{
			{Level: EscalationLevel1, EscalateTo: []string{"team"}, After: 10 * time.Minute},
		},
		CheckInterval: 1 * time.Minute,
		OnEscalate: func(ctx *execCtx.ExecutionContext, level EscalationLevel, recipients []string) error {
			return nil
		},
	}

	// Act
	errCh := make(chan error, 1)
	go func() {
		errCh <- task.Execute(ctx)
	}()

	// Wait for timer, then cancel context partway through
	if !waitForTimer(clk, 100*time.Millisecond) {
		t.Fatal("Timeout waiting for timer to be registered")
	}
	clk.AdvanceBy(2 * time.Minute)
	time.Sleep(10 * time.Millisecond)
	cancel()

	err := <-errCh

	// Assert
	if err == nil {
		t.Fatal("Expected error on context cancellation")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}
}

// TestEscalationTask_ValidationErrors tests validation of task configuration.
func TestEscalationTask_ValidationErrors(t *testing.T) {
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "escalation-7"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	tests := []struct {
		name        string
		task        *EscalationTask
		expectedErr string
	}{
		{
			name: "missing name",
			task: &EscalationTask{
				Levels: []EscalationConfig{
					{Level: EscalationLevel1, EscalateTo: []string{"team"}, After: 1 * time.Minute},
				},
			},
			expectedErr: "name is required",
		},
		{
			name: "no levels",
			task: &EscalationTask{
				Name:   "test",
				Levels: []EscalationConfig{},
			},
			expectedErr: "at least one escalation level is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.task.Execute(ctx)
			if err == nil {
				t.Fatal("Expected validation error, got nil")
			}
			if tt.expectedErr != "" && !strings.Contains(err.Error(), tt.expectedErr) {
				t.Errorf("Expected error containing '%s', got: %v", tt.expectedErr, err)
			}
		})
	}
}

// TestEscalationTask_OnEscalateError tests error handling in OnEscalate callback.
func TestEscalationTask_OnEscalateError(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "escalation-8"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	expectedErr := errors.New("notification failed")

	task := &EscalationTask{
		Name: "escalate-error",
		Levels: []EscalationConfig{
			{Level: EscalationLevel1, EscalateTo: []string{"team"}, After: 1 * time.Minute},
		},
		CheckInterval: 1 * time.Second,
		OnEscalate: func(ctx *execCtx.ExecutionContext, level EscalationLevel, recipients []string) error {
			return expectedErr
		},
	}

	// Act
	errCh := make(chan error, 1)
	go func() {
		errCh <- task.Execute(ctx)
	}()

	if !waitForTimer(clk, 100*time.Millisecond) {
		t.Fatal("Timeout waiting for timer to be registered")
	}
	clk.AdvanceBy(1 * time.Minute)
	err := <-errCh

	// Assert
	if err == nil {
		t.Fatal("Expected error from OnEscalate")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("Expected error to wrap %v, got: %v", expectedErr, err)
	}
}

// TestEscalationTask_ConditionPanic tests panic recovery in condition function.
func TestEscalationTask_ConditionPanic(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "escalation-9"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	task := &EscalationTask{
		Name: "condition-panic",
		Levels: []EscalationConfig{
			{Level: EscalationLevel1, EscalateTo: []string{"team"}, After: 1 * time.Minute},
		},
		EscalateWhen: func(ctx *execCtx.ExecutionContext) (bool, error) {
			panic("condition function panicked!")
		},
	}

	// Act
	err := task.Execute(ctx)

	// Assert
	if err == nil {
		t.Fatal("Expected error from panicking condition")
	}
	if !strings.Contains(err.Error(), "panicked") {
		t.Errorf("Expected panic error, got: %v", err)
	}
}

// TestEscalationTask_EscalationLevelString tests String() method on EscalationLevel.
func TestEscalationTask_EscalationLevelString(t *testing.T) {
	tests := []struct {
		level    EscalationLevel
		expected string
	}{
		{EscalationLevelNone, "none"},
		{EscalationLevel1, "level1"},
		{EscalationLevel2, "level2"},
		{EscalationLevel3, "level3"},
		{EscalationLevel(99), "level99"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.level.String()
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestEscalationTask_ZeroDurationLevel tests level with zero duration (immediate escalation).
func TestEscalationTask_ZeroDurationLevel(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "escalation-10"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	escalated := false

	task := &EscalationTask{
		Name: "zero-duration",
		Levels: []EscalationConfig{
			{Level: EscalationLevel1, EscalateTo: []string{"team"}, After: 0}, // Immediate
		},
		OnEscalate: func(ctx *execCtx.ExecutionContext, level EscalationLevel, recipients []string) error {
			escalated = true
			return nil
		},
	}

	// Act
	err := task.Execute(ctx)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if !escalated {
		t.Error("Expected immediate escalation with zero duration")
	}
}

// TestEscalationTask_DefaultCheckInterval tests that check interval defaults to 1 minute.
func TestEscalationTask_DefaultCheckInterval(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "escalation-11"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	resolver := NewChannelEscalationResolver()

	task := &EscalationTask{
		Name: "default-interval",
		Levels: []EscalationConfig{
			{Level: EscalationLevel1, EscalateTo: []string{"team"}, After: 5 * time.Minute},
		},
		// CheckInterval not set - should default to 1 minute
		Resolver: resolver,
		OnEscalate: func(ctx *execCtx.ExecutionContext, level EscalationLevel, recipients []string) error {
			return nil
		},
	}

	// Act - run task
	errCh := make(chan error, 1)
	go func() {
		errCh <- task.Execute(ctx)
	}()

	// Wait for timer, then advance and resolve
	if !waitForTimer(clk, 100*time.Millisecond) {
		t.Fatal("Timeout waiting for timer to be registered")
	}
	clk.AdvanceBy(2 * time.Minute)
	time.Sleep(10 * time.Millisecond)
	resolver.Resolve()

	err := <-errCh

	// Assert
	if err != nil {
		t.Fatalf("Expected no error with default check interval, got: %v", err)
	}
}

// TestChannelEscalationResolver_MultipleResolves tests that multiple Resolve() calls are idempotent.
func TestChannelEscalationResolver_MultipleResolves(t *testing.T) {
	// Arrange
	resolver := NewChannelEscalationResolver()
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	// Act - resolve multiple times
	resolver.Resolve()
	resolver.Resolve()
	resolver.Resolve()

	// Assert
	resolved, err := resolver.IsResolved(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if !resolved {
		t.Error("Expected resolver to be resolved")
	}

	// Check channel doesn't block
	select {
	case <-resolver.WaitForResolution(ctx):
		// Expected - channel should have value
	case <-time.After(100 * time.Millisecond):
		t.Error("WaitForResolution blocked unexpectedly")
	}
}

// TestEscalationTask_ConcurrentExecution tests concurrent task execution.
func TestEscalationTask_ConcurrentExecution(t *testing.T) {
	// Arrange - use zero duration for immediate escalation (no timing issues)
	clk := clock.NewVirtualClock(time.Now())

	task := &EscalationTask{
		Name: "concurrent",
		Levels: []EscalationConfig{
			{Level: EscalationLevel1, EscalateTo: []string{"team"}, After: 0}, // Immediate escalation
		},
		OnEscalate: func(ctx *execCtx.ExecutionContext, level EscalationLevel, recipients []string) error {
			return nil
		},
	}

	// Act - run task concurrently
	var wg sync.WaitGroup
	errCh := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			tok := &token.Token{ID: fmt.Sprintf("escalation-%d", id)}
			ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

			if err := task.Execute(ctx); err != nil {
				errCh <- err
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	// Assert
	for err := range errCh {
		t.Errorf("Concurrent execution error: %v", err)
	}
}
