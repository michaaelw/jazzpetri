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
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	execCtx "github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/clock"
	"github.com/jazzpetri/engine/token"
)

// TestCommandTask_Execute_Success tests successful command execution
func TestCommandTask_Execute_Success(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test-token-1"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	// Use echo command which is available on all platforms
	task := &CommandTask{
		Name:    "test-echo",
		Command: "echo",
		Args:    []string{"hello", "world"},
	}

	// Act
	err := task.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

// TestCommandTask_Execute_WithArgs tests command with multiple arguments
func TestCommandTask_Execute_WithArgs(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	// Use printf which is available on Unix-like systems
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	task := &CommandTask{
		Name:    "test-printf",
		Command: "printf",
		Args:    []string{"%s %d", "value", "42"},
	}

	// Act
	err := task.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

// TestCommandTask_Execute_NonZeroExit tests non-zero exit code
func TestCommandTask_Execute_NonZeroExit(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	var task *CommandTask
	if runtime.GOOS == "windows" {
		// Windows: use cmd /c exit 1
		task = &CommandTask{
			Name:    "exit-nonzero",
			Command: "cmd",
			Args:    []string{"/c", "exit", "1"},
		}
	} else {
		// Unix: use sh -c "exit 1"
		task = &CommandTask{
			Name:    "exit-nonzero",
			Command: "sh",
			Args:    []string{"-c", "exit 1"},
		}
	}

	// Act
	err := task.Execute(ctx)

	// Assert
	if err == nil {
		t.Error("Expected error for non-zero exit code")
	}
	if !strings.Contains(err.Error(), "command failed") {
		t.Errorf("Expected command failed error, got: %v", err)
	}
}

// TestCommandTask_Execute_CommandNotFound tests missing command
func TestCommandTask_Execute_CommandNotFound(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	task := &CommandTask{
		Name:    "not-found",
		Command: "this-command-definitely-does-not-exist-12345",
		Args:    []string{},
	}

	// Act
	err := task.Execute(ctx)

	// Assert
	if err == nil {
		t.Error("Expected error for command not found")
	}
}

// TestCommandTask_Execute_WithEnv tests environment variables
func TestCommandTask_Execute_WithEnv(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	var task *CommandTask
	if runtime.GOOS == "windows" {
		// Windows: use cmd /c echo %TEST_VAR%
		task = &CommandTask{
			Name:    "test-env",
			Command: "cmd",
			Args:    []string{"/c", "echo", "%TEST_VAR%"},
			Env: map[string]string{
				"TEST_VAR": "test_value_123",
			},
		}
	} else {
		// Unix: use sh -c "echo $TEST_VAR"
		task = &CommandTask{
			Name:    "test-env",
			Command: "sh",
			Args:    []string{"-c", "echo $TEST_VAR"},
			Env: map[string]string{
				"TEST_VAR": "test_value_123",
			},
		}
	}

	// Act
	err := task.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	// Note: We can't easily verify the output was correct without capturing it,
	// but we can verify the command executed successfully with the env var set
}

// TestCommandTask_Execute_ContextCancellation tests context cancellation
func TestCommandTask_Execute_ContextCancellation(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	goCtx, cancel := context.WithCancel(context.Background())
	ctx := execCtx.NewExecutionContext(goCtx, clk, nil)

	var task *CommandTask
	if runtime.GOOS == "windows" {
		// Windows: timeout command
		task = &CommandTask{
			Name:    "long-running",
			Command: "timeout",
			Args:    []string{"10"}, // 10 seconds
		}
	} else {
		// Unix: sleep command
		task = &CommandTask{
			Name:    "long-running",
			Command: "sleep",
			Args:    []string{"10"}, // 10 seconds
		}
	}

	// Cancel context after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	// Act
	err := task.Execute(ctx)

	// Assert
	if err == nil {
		t.Error("Expected error from context cancellation")
	}
}

// TestCommandTask_Execute_NoName tests validation
func TestCommandTask_Execute_NoName(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	task := &CommandTask{
		Name:    "", // Empty name
		Command: "echo",
		Args:    []string{"test"},
	}

	// Act
	err := task.Execute(ctx)

	// Assert
	if err == nil {
		t.Error("Expected validation error")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("Expected name validation error, got: %v", err)
	}
}

// TestCommandTask_Execute_NoCommand tests validation
func TestCommandTask_Execute_NoCommand(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	task := &CommandTask{
		Name:    "test",
		Command: "", // Empty command
		Args:    []string{"test"},
	}

	// Act
	err := task.Execute(ctx)

	// Assert
	if err == nil {
		t.Error("Expected validation error")
	}
	if !strings.Contains(err.Error(), "command is required") {
		t.Errorf("Expected command validation error, got: %v", err)
	}
}

// TestCommandTask_Execute_Concurrent tests thread safety
func TestCommandTask_Execute_Concurrent(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())

	task := &CommandTask{
		Name:    "concurrent-echo",
		Command: "echo",
		Args:    []string{"concurrent"},
	}

	// Act - execute concurrently
	const numGoroutines = 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	errCount := 0
	var mu sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)
			err := task.Execute(ctx)
			if err != nil {
				mu.Lock()
				errCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// Assert
	if errCount > 0 {
		t.Errorf("Expected no errors, got %d errors", errCount)
	}
}

// TestCommandTask_Execute_WithCustomObservability tests observability
func TestCommandTask_Execute_WithCustomObservability(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	tracer := &testTracer{}
	metrics := &testMetrics{}
	logger := &testLogger{}

	ctx = ctx.WithTracer(tracer)
	ctx = ctx.WithMetrics(metrics)
	ctx = ctx.WithLogger(logger)

	task := &CommandTask{
		Name:    "observable-command",
		Command: "echo",
		Args:    []string{"test"},
	}

	// Act
	err := task.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify observability
	if tracer.spanCount != 1 {
		t.Errorf("Expected 1 span, got %d", tracer.spanCount)
	}
	if metrics.counters["task_command_executions_total"] != 1 {
		t.Errorf("Expected task_command_executions_total=1")
	}
	if metrics.counters["task_command_success_total"] != 1 {
		t.Errorf("Expected task_command_success_total=1")
	}
	if len(metrics.histograms["task_command_duration_seconds"]) != 1 {
		t.Error("Expected duration observation")
	}
}

// TestCommandTask_Execute_WithCustomObservability_Error tests error observability
func TestCommandTask_Execute_WithCustomObservability_Error(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	tracer := &testTracer{}
	metrics := &testMetrics{}
	logger := &testLogger{}
	recorder := &testErrorRecorder{}

	ctx = ctx.WithTracer(tracer)
	ctx = ctx.WithMetrics(metrics)
	ctx = ctx.WithLogger(logger)
	ctx = ctx.WithErrorRecorder(recorder)

	var task *CommandTask
	if runtime.GOOS == "windows" {
		task = &CommandTask{
			Name:    "failing-command",
			Command: "cmd",
			Args:    []string{"/c", "exit", "1"},
		}
	} else {
		task = &CommandTask{
			Name:    "failing-command",
			Command: "sh",
			Args:    []string{"-c", "exit 1"},
		}
	}

	// Act
	err := task.Execute(ctx)

	// Assert
	if err == nil {
		t.Error("Expected error")
	}

	// Verify error metrics
	if metrics.counters["task_errors_total"] != 1 {
		t.Errorf("Expected task_errors_total=1")
	}
	if metrics.counters["task_command_errors_total"] != 1 {
		t.Errorf("Expected task_command_errors_total=1")
	}
	if logger.errorCount != 1 {
		t.Errorf("Expected 1 error log, got %d", logger.errorCount)
	}
	if recorder.errorCount != 1 {
		t.Errorf("Expected 1 recorded error, got %d", recorder.errorCount)
	}
}

// TestCommandTask_TruncateString tests string truncation
func TestCommandTask_TruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "No truncation needed",
			input:    "short",
			maxLen:   10,
			expected: "short",
		},
		{
			name:     "Exact length",
			input:    "exactly10!",
			maxLen:   10,
			expected: "exactly10!",
		},
		{
			name:     "Truncation needed",
			input:    "this is a very long string that needs truncation",
			maxLen:   20,
			expected: "this is a very long ... (truncated)",
		},
		{
			name:     "Empty string",
			input:    "",
			maxLen:   10,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}
