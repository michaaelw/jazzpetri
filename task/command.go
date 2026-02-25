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
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/jazzpetri/engine/context"
)

// CommandTask executes shell commands and external processes.
// This enables integration with scripts, CLI tools, and legacy systems.
//
// CommandTask is ideal for:
//   - Running scripts (Python, Bash, etc.)
//   - Executing CLI tools
//   - Running data processing pipelines
//   - Integrating with legacy systems
//   - Performing system operations
//
// Example:
//
//	task := &CommandTask{
//	    Name:    "data-processing",
//	    Command: "python",
//	    Args:    []string{"process.py", "--input", "data.csv", "--output", "results.json"},
//	    Env: map[string]string{
//	        "API_KEY": "secret123",
//	        "REGION":  "us-west-2",
//	    },
//	    Dir: "/path/to/working/directory",
//	}
//
// The task captures stdout and stderr, which are logged for debugging.
// Non-zero exit codes are treated as errors.
//
// Thread Safety:
// CommandTask is safe for concurrent execution. Each execution creates
// its own process with isolated stdin/stdout/stderr.
//
// Error Handling:
// CommandTask returns errors for:
//   - Command not found
//   - Non-zero exit codes
//   - Signal termination
//   - Context cancellation
type CommandTask struct {
	// Name is the task identifier used in logs, traces, and metrics.
	Name string

	// Command is the command to execute (e.g., "python", "bash", "curl").
	// This should be either an absolute path or a command in PATH.
	Command string

	// Args are the command arguments.
	// Example: []string{"script.py", "--verbose", "--output", "result.txt"}
	Args []string

	// Env are environment variables to set for the command.
	// These are added to (not replacing) the parent process environment.
	// Example: map[string]string{"API_KEY": "secret", "DEBUG": "true"}
	Env map[string]string

	// Dir is the working directory for the command.
	// If empty, the command runs in the current process's directory.
	Dir string
}

// Execute runs the command with observability and error handling.
//
// Execution flow:
//  1. Validate task configuration
//  2. Start trace span
//  3. Create command with context (for cancellation)
//  4. Set up environment variables and working directory
//  5. Capture stdout and stderr
//  6. Execute command
//  7. Check exit code
//  8. Log output and errors
//  9. Record metrics
//
// The task respects ctx.Context cancellation. If the context is cancelled,
// the process receives a kill signal.
//
// Thread Safety:
// Execute is safe to call concurrently from multiple goroutines.
// Each execution creates its own isolated process.
//
// Returns an error if:
//   - Task configuration is invalid (missing Name or Command)
//   - Command not found in PATH
//   - Command exits with non-zero status
//   - Command is terminated by signal
//   - Context is cancelled before command completes
func (c *CommandTask) Execute(ctx *context.ExecutionContext) error {
	// Validate configuration
	if c.Name == "" {
		return fmt.Errorf("command task: name is required")
	}
	if c.Command == "" {
		return fmt.Errorf("command task: command is required")
	}

	// Start trace span
	span := ctx.Tracer.StartSpan("task.command." + c.Name)
	defer span.End()

	span.SetAttribute("command.name", c.Command)
	span.SetAttribute("command.args", strings.Join(c.Args, " "))
	span.SetAttribute("task.name", c.Name)

	// Record metrics
	ctx.Metrics.Inc("task_executions_total")
	ctx.Metrics.Inc("task_command_executions_total")

	// Log task start
	ctx.Logger.Debug("Executing command task", map[string]interface{}{
		"task_name": c.Name,
		"command":   c.Command,
		"args":      c.Args,
		"token_id":  tokenID(ctx),
	})

	// Create command with context for cancellation support
	cmd := exec.CommandContext(ctx.Context, c.Command, c.Args...)

	// Set working directory if specified
	if c.Dir != "" {
		cmd.Dir = c.Dir
		span.SetAttribute("command.dir", c.Dir)
	}

	// Add environment variables
	if len(c.Env) > 0 {
		// Start with parent environment
		cmd.Env = append(cmd.Env, cmd.Environ()...)
		// Add custom environment variables
		for key, value := range c.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
		}
	}

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute command
	startTime := ctx.Clock.Now()
	err := cmd.Run()
	duration := ctx.Clock.Now().Sub(startTime).Seconds()

	// Record duration metric
	ctx.Metrics.Observe("task_command_duration_seconds", duration)

	// Capture output for logging
	stdoutStr := stdout.String()
	stderrStr := stderr.String()

	span.SetAttribute("command.stdout_size", len(stdoutStr))
	span.SetAttribute("command.stderr_size", len(stderrStr))

	// Handle error
	if err != nil {
		// Determine error type
		exitCode := -1
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}

		span.SetAttribute("command.exit_code", exitCode)

		return c.handleError(ctx, span, err, exitCode, stdoutStr, stderrStr)
	}

	// Success - exit code 0
	span.SetAttribute("command.exit_code", 0)
	ctx.Metrics.Inc("task_success_total")
	ctx.Metrics.Inc("task_command_success_total")

	ctx.Logger.Debug("Command task completed", map[string]interface{}{
		"task_name":   c.Name,
		"command":     c.Command,
		"exit_code":   0,
		"duration_ms": duration * 1000,
		"stdout_size": len(stdoutStr),
		"stderr_size": len(stderrStr),
		"token_id":    tokenID(ctx),
	})

	// Log stdout/stderr at debug level if present
	if len(stdoutStr) > 0 {
		ctx.Logger.Debug("Command stdout", map[string]interface{}{
			"task_name": c.Name,
			"stdout":    truncateString(stdoutStr, 1000),
			"token_id":  tokenID(ctx),
		})
	}
	if len(stderrStr) > 0 {
		ctx.Logger.Debug("Command stderr", map[string]interface{}{
			"task_name": c.Name,
			"stderr":    truncateString(stderrStr, 1000),
			"token_id":  tokenID(ctx),
		})
	}

	return nil
}

// handleError processes command errors with observability
func (c *CommandTask) handleError(
	ctx *context.ExecutionContext,
	span context.Span,
	err error,
	exitCode int,
	stdout, stderr string,
) error {
	span.RecordError(err)
	ctx.Metrics.Inc("task_errors_total")
	ctx.Metrics.Inc("task_command_errors_total")

	// Build detailed error message
	var errMsg strings.Builder
	errMsg.WriteString(fmt.Sprintf("command failed: %v", err))
	if exitCode >= 0 {
		errMsg.WriteString(fmt.Sprintf(" (exit code %d)", exitCode))
	}

	finalErr := fmt.Errorf("%s", errMsg.String())

	ctx.ErrorRecorder.RecordError(finalErr, map[string]interface{}{
		"task_name": c.Name,
		"command":   c.Command,
		"args":      c.Args,
		"exit_code": exitCode,
		"token_id":  tokenID(ctx),
	})

	ctx.Logger.Error("Command task failed", map[string]interface{}{
		"task_name": c.Name,
		"command":   c.Command,
		"args":      c.Args,
		"exit_code": exitCode,
		"error":     err.Error(),
		"stdout":    truncateString(stdout, 500),
		"stderr":    truncateString(stderr, 500),
		"token_id":  tokenID(ctx),
	})

	return ctx.ErrorHandler.Handle(ctx, finalErr)
}

// truncateString truncates a string to maxLen characters, adding ellipsis if truncated
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "... (truncated)"
}
