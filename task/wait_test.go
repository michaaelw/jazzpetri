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
	stdContext "context"
	"testing"
	"time"

	"github.com/jazzpetri/engine/clock"
	"github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/token"
)

func TestWaitTask_FixedDuration(t *testing.T) {
	// Create virtual clock
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(start)

	// Create execution context
	tok := &token.Token{ID: "test-token", Metadata: make(map[string]interface{})}
	ctx := context.NewExecutionContext(stdContext.Background(), clk, tok)

	// Create wait task
	task := &WaitTask{
		Name:     "test-wait",
		Duration: 5 * time.Second,
	}

	// Execute in goroutine (will block on Sleep)
	done := make(chan error)
	go func() {
		done <- task.Execute(ctx)
	}()

	// Advance time
	time.Sleep(10 * time.Millisecond) // Give goroutine time to start
	clk.AdvanceBy(5 * time.Second)

	// Wait for completion
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("test timed out")
	}
}

func TestWaitTask_UntilTime(t *testing.T) {
	// Create virtual clock
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(start)

	// Create execution context
	tok := &token.Token{ID: "test-token", Metadata: make(map[string]interface{})}
	ctx := context.NewExecutionContext(stdContext.Background(), clk, tok)

	// Wait until specific time
	until := start.Add(10 * time.Second)
	task := &WaitTask{
		Name:  "wait-until",
		Until: until,
	}

	// Execute in goroutine
	done := make(chan error)
	go func() {
		done <- task.Execute(ctx)
	}()

	// Advance time past the target
	time.Sleep(10 * time.Millisecond)
	clk.AdvanceBy(15 * time.Second)

	// Wait for completion
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("test timed out")
	}
}

func TestWaitTask_UntilTimeInPast(t *testing.T) {
	// Create virtual clock at a later time
	start := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(start)

	// Create execution context
	tok := &token.Token{ID: "test-token", Metadata: make(map[string]interface{})}
	ctx := context.NewExecutionContext(stdContext.Background(), clk, tok)

	// Wait until a time in the past
	until := start.Add(-1 * time.Hour) // 1 hour ago
	task := &WaitTask{
		Name:  "wait-past",
		Until: until,
	}

	// Execute - should complete immediately
	err := task.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWaitTask_DynamicDuration(t *testing.T) {
	// Create virtual clock
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(start)

	// Create execution context with token data
	tok := &token.Token{
		ID:       "test-token",
		Metadata: make(map[string]interface{}),
	}
	tok.Metadata["priority"] = "high"
	ctx := context.NewExecutionContext(stdContext.Background(), clk, tok)

	// Create wait task with dynamic duration
	task := &WaitTask{
		Name: "dynamic-wait",
		DurationFunc: func(ctx *context.ExecutionContext) time.Duration {
			priority := ctx.Token.Metadata["priority"]
			if priority == "high" {
				return 1 * time.Second
			}
			return 10 * time.Second
		},
	}

	// Execute in goroutine
	done := make(chan error)
	go func() {
		done <- task.Execute(ctx)
	}()

	// Advance time (should only need 1 second for high priority)
	time.Sleep(10 * time.Millisecond)
	clk.AdvanceBy(2 * time.Second)

	// Wait for completion
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("test timed out")
	}
}

func TestWaitTask_ContextCancellation(t *testing.T) {
	// Create virtual clock
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(start)

	// Create cancellable context
	goCtx, cancel := stdContext.WithCancel(stdContext.Background())
	tok := &token.Token{ID: "test-token", Metadata: make(map[string]interface{})}
	ctx := context.NewExecutionContext(goCtx, clk, tok)

	// Create wait task with long duration
	task := &WaitTask{
		Name:     "long-wait",
		Duration: 1 * time.Hour,
	}

	// Execute in goroutine
	done := make(chan error)
	go func() {
		done <- task.Execute(ctx)
	}()

	// Cancel the context
	time.Sleep(10 * time.Millisecond)
	cancel()

	// Should return with context error
	select {
	case err := <-done:
		if err != stdContext.Canceled {
			t.Fatalf("expected context.Canceled, got: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("test timed out")
	}
}

func TestWaitTask_ValidationErrors(t *testing.T) {
	// Create virtual clock
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(start)

	tok := &token.Token{ID: "test-token", Metadata: make(map[string]interface{})}
	ctx := context.NewExecutionContext(stdContext.Background(), clk, tok)

	// Test missing name
	task := &WaitTask{
		Duration: 5 * time.Second,
	}
	err := task.Execute(ctx)
	if err == nil || err.Error() != "wait task: name is required" {
		t.Fatalf("expected name required error, got: %v", err)
	}

	// Test missing duration
	task = &WaitTask{
		Name: "test",
	}
	err = task.Execute(ctx)
	if err == nil || err.Error() != "wait task: duration, until, or duration_func is required" {
		t.Fatalf("expected duration required error, got: %v", err)
	}
}

func TestWaitTask_ZeroDuration(t *testing.T) {
	// Create virtual clock
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(start)

	tok := &token.Token{ID: "test-token", Metadata: make(map[string]interface{})}
	ctx := context.NewExecutionContext(stdContext.Background(), clk, tok)

	// Create wait task with zero duration via DurationFunc
	task := &WaitTask{
		Name: "zero-wait",
		DurationFunc: func(ctx *context.ExecutionContext) time.Duration {
			return 0
		},
	}

	// Execute - should complete immediately
	err := task.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
