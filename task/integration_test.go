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
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	execCtx "github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/clock"
	"github.com/jazzpetri/engine/token"
)

// TestIntegration_MultipleTaskTypes tests using different task types together
func TestIntegration_MultipleTaskTypes(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{
		ID:   "workflow-token-1",
		Data: &testTokenData{Value: 100},
	}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	// Create tasks
	inlineTask := &InlineTask{
		Name: "calculate",
		Fn: func(ctx *execCtx.ExecutionContext) error {
			data := ctx.Token.Data.(*testTokenData)
			data.Value = data.Value * 2
			return nil
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	httpTask := &HTTPTask{
		Name:   "notify",
		Method: "POST",
		URL:    server.URL,
	}

	commandTask := &CommandTask{
		Name:    "log",
		Command: "echo",
		Args:    []string{"workflow", "complete"},
	}

	// Act - execute tasks in sequence
	err := inlineTask.Execute(ctx)
	if err != nil {
		t.Fatalf("InlineTask failed: %v", err)
	}

	err = httpTask.Execute(ctx)
	if err != nil {
		t.Fatalf("HTTPTask failed: %v", err)
	}

	err = commandTask.Execute(ctx)
	if err != nil {
		t.Fatalf("CommandTask failed: %v", err)
	}

	// Assert
	data := tok.Data.(*testTokenData)
	if data.Value != 200 {
		t.Errorf("Expected value 200, got %d", data.Value)
	}
}

// TestIntegration_ErrorHandling tests error handling across task types
func TestIntegration_ErrorHandling(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	// Custom error handler that counts errors
	errorCount := 0
	var mu sync.Mutex
	customHandler := &customErrorHandler{
		onHandle: func(ctx *execCtx.ExecutionContext, err error) error {
			mu.Lock()
			errorCount++
			mu.Unlock()
			return err // Pass through
		},
	}
	ctx = ctx.WithErrorHandler(customHandler)

	// Create failing tasks
	inlineTask := &InlineTask{
		Name: "failing-inline",
		Fn: func(ctx *execCtx.ExecutionContext) error {
			return errors.New("inline error")
		},
	}

	httpTask := &HTTPTask{
		Name:   "failing-http",
		Method: "GET",
		URL:    "http://localhost:9999", // Non-existent server
	}

	// Act
	_ = inlineTask.Execute(ctx)
	_ = httpTask.Execute(ctx)

	// Assert
	mu.Lock()
	count := errorCount
	mu.Unlock()

	if count != 2 {
		t.Errorf("Expected 2 errors handled, got %d", count)
	}
}

// TestIntegration_ObservabilityPipeline tests observability across task execution
func TestIntegration_ObservabilityPipeline(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	// Set up observability
	tracer := &testTracer{}
	metrics := &testMetrics{}
	logger := &testLogger{}

	ctx = ctx.WithTracer(tracer)
	ctx = ctx.WithMetrics(metrics)
	ctx = ctx.WithLogger(logger)

	// Create tasks
	task1 := &InlineTask{
		Name: "step1",
		Fn:   func(ctx *execCtx.ExecutionContext) error { return nil },
	}

	task2 := &InlineTask{
		Name: "step2",
		Fn:   func(ctx *execCtx.ExecutionContext) error { return nil },
	}

	task3 := &InlineTask{
		Name: "step3",
		Fn:   func(ctx *execCtx.ExecutionContext) error { return nil },
	}

	// Act
	task1.Execute(ctx)
	task2.Execute(ctx)
	task3.Execute(ctx)

	// Assert
	if tracer.spanCount != 3 {
		t.Errorf("Expected 3 spans, got %d", tracer.spanCount)
	}
	if metrics.counters["task_executions_total"] != 3 {
		t.Errorf("Expected 3 task executions")
	}
	if metrics.counters["task_success_total"] != 3 {
		t.Errorf("Expected 3 successful tasks")
	}
}

// TestIntegration_ContextCancellation tests cancellation across task types
func TestIntegration_ContextCancellation(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	goCtx, cancel := context.WithCancel(context.Background())
	ctx := execCtx.NewExecutionContext(goCtx, clk, nil)

	// Create a long-running HTTP task
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	task := &HTTPTask{
		Name:   "long-task",
		Method: "GET",
		URL:    server.URL,
	}

	// Cancel after short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	// Act
	err := task.Execute(ctx)

	// Assert
	if err == nil {
		t.Error("Expected cancellation error")
	}
}

// TestIntegration_TokenDataModification tests token data flow through tasks
func TestIntegration_TokenDataModification(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{
		ID:   "data-flow-token",
		Data: &testTokenData{Value: 0},
	}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	// Create pipeline of tasks that modify token data
	tasks := []Task{
		&InlineTask{
			Name: "add-10",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				data := ctx.Token.Data.(*testTokenData)
				data.Value += 10
				return nil
			},
		},
		&InlineTask{
			Name: "multiply-2",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				data := ctx.Token.Data.(*testTokenData)
				data.Value *= 2
				return nil
			},
		},
		&InlineTask{
			Name: "add-5",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				data := ctx.Token.Data.(*testTokenData)
				data.Value += 5
				return nil
			},
		},
	}

	// Act - execute pipeline
	for _, task := range tasks {
		err := task.Execute(ctx)
		if err != nil {
			t.Fatalf("Task failed: %v", err)
		}
	}

	// Assert - (0 + 10) * 2 + 5 = 25
	data := tok.Data.(*testTokenData)
	if data.Value != 25 {
		t.Errorf("Expected value 25, got %d", data.Value)
	}
}

// TestIntegration_ConcurrentTaskExecution tests concurrent execution
func TestIntegration_ConcurrentTaskExecution(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())

	var counter int
	var mu sync.Mutex

	task := &InlineTask{
		Name: "concurrent",
		Fn: func(ctx *execCtx.ExecutionContext) error {
			mu.Lock()
			counter++
			mu.Unlock()
			return nil
		},
	}

	// Act - execute same task concurrently with different contexts
	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			tok := &token.Token{ID: string(rune('0' + id))}
			ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)
			err := task.Execute(ctx)
			if err != nil {
				t.Errorf("Task %d failed: %v", id, err)
			}
		}(i)
	}

	wg.Wait()

	// Assert
	if counter != numGoroutines {
		t.Errorf("Expected %d executions, got %d", numGoroutines, counter)
	}
}

// TestIntegration_ErrorRecovery tests error recording and recovery
func TestIntegration_ErrorRecovery(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	recorder := &testErrorRecorder{}
	ctx = ctx.WithErrorRecorder(recorder)

	// Create tasks with different error types
	tasks := []struct {
		name string
		task Task
	}{
		{
			name: "panic",
			task: &InlineTask{
				Name: "panicking",
				Fn: func(ctx *execCtx.ExecutionContext) error {
					panic("panic error")
				},
			},
		},
		{
			name: "error",
			task: &InlineTask{
				Name: "returning-error",
				Fn: func(ctx *execCtx.ExecutionContext) error {
					return errors.New("normal error")
				},
			},
		},
	}

	// Act
	for _, tt := range tasks {
		_ = tt.task.Execute(ctx)
	}

	// Assert
	// Panic task records error twice: once in panic recovery, once in error handling
	// Normal error task records once
	if recorder.errorCount != 3 {
		t.Errorf("Expected 3 errors recorded (2 for panic, 1 for error), got %d", recorder.errorCount)
	}
}

// TestIntegration_TaskInterface tests that all tasks implement Task
func TestIntegration_TaskInterface(t *testing.T) {
	// This test verifies interface compliance at compile time
	var _ Task = (*InlineTask)(nil)
	var _ Task = (*HTTPTask)(nil)
	var _ Task = (*CommandTask)(nil)
}

// TestIntegration_ImmutableConfiguration tests that task configuration is immutable
func TestIntegration_ImmutableConfiguration(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())

	originalName := "original-name"
	task := &InlineTask{
		Name: originalName,
		Fn:   func(ctx *execCtx.ExecutionContext) error { return nil },
	}

	// Act - execute task multiple times
	for i := 0; i < 5; i++ {
		ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)
		task.Execute(ctx)
	}

	// Assert - configuration unchanged
	if task.Name != originalName {
		t.Errorf("Task name was modified: expected %s, got %s", originalName, task.Name)
	}
}

// customErrorHandler is a test error handler that allows custom behavior
type customErrorHandler struct {
	onHandle func(ctx *execCtx.ExecutionContext, err error) error
}

func (h *customErrorHandler) Handle(ctx *execCtx.ExecutionContext, err error) error {
	if h.onHandle != nil {
		return h.onHandle(ctx, err)
	}
	return err
}
