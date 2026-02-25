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

package context

// ErrorHandler handles retry and compensation logic for workflow errors.
// Implementations should be thread-safe for concurrent use.
//
// The ErrorHandler is invoked when a task fails, allowing for sophisticated
// error handling strategies including retry with backoff, compensation transactions,
// and error propagation to parent workflows.
//
// Use NoOpErrorHandler when custom error handling is not needed - it passes
// through errors unchanged.
type ErrorHandler interface {
	// Handle processes an error that occurred during workflow execution.
	// The handler can:
	//   - Return nil to suppress the error (after handling it)
	//   - Return a modified error
	//   - Return the original error unchanged
	//
	// The ExecutionContext provides access to the workflow state, token,
	// and observability tools for implementing sophisticated error handling.
	//
	// Example implementations:
	//   - Retry with exponential backoff
	//   - Execute compensation transactions
	//   - Transform errors for propagation
	//   - Record error metrics and logs
	Handle(ctx *ExecutionContext, err error) error
}

// ErrorRecorder records errors for observability and debugging.
// Implementations should be thread-safe for concurrent use.
//
// The ErrorRecorder captures error information for metrics, logging, and
// distributed tracing. This is separate from ErrorHandler to allow recording
// errors even when they are handled and suppressed.
//
// Use NoOpErrorRecorder when error recording is not needed for zero overhead.
type ErrorRecorder interface {
	// RecordError records an error with optional metadata.
	// This should be called whenever an error occurs, even if it's handled.
	//
	// Metadata can include:
	//   - "workflow_id": workflow identifier
	//   - "transition_id": transition that failed
	//   - "token_id": token being processed
	//   - "error_type": classification of error
	//   - "severity": error severity level
	//
	// Example:
	//   recorder.RecordError(err, map[string]interface{}{
	//       "workflow_id": "wf-123",
	//       "transition_id": "t1",
	//       "error_type": "network_timeout",
	//   })
	RecordError(err error, metadata map[string]interface{})

	// RecordRetry records a retry attempt with the attempt number and next retry time.
	// This is used to track retry patterns and backoff behavior.
	//
	// Parameters:
	//   - attempt: the retry attempt number (1-based)
	//   - nextRetry: time until next retry (duration) or time of next retry (time.Time)
	//
	// Example:
	//   recorder.RecordRetry(3, 8*time.Second)
	RecordRetry(attempt int, nextRetry interface{})

	// RecordCompensation records a compensation action being executed.
	// Compensation actions undo the effects of failed operations.
	//
	// Example:
	//   recorder.RecordCompensation("refund-payment")
	RecordCompensation(action string)
}

// NoOpErrorHandler is a zero-overhead error handler that passes through errors unchanged.
// This is used as the default when custom error handling is not needed.
type NoOpErrorHandler struct{}

// Handle returns the error unchanged. This is a no-op that passes through errors.
func (n *NoOpErrorHandler) Handle(ctx *ExecutionContext, err error) error {
	return err
}

// NoOpErrorRecorder is a zero-overhead error recorder used when error recording is disabled.
// All methods are no-ops and should be optimized away by the compiler.
type NoOpErrorRecorder struct{}

// RecordError is a no-op.
func (n *NoOpErrorRecorder) RecordError(err error, metadata map[string]interface{}) {}

// RecordRetry is a no-op.
func (n *NoOpErrorRecorder) RecordRetry(attempt int, nextRetry interface{}) {}

// RecordCompensation is a no-op.
func (n *NoOpErrorRecorder) RecordCompensation(action string) {}
