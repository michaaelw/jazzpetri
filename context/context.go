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

// Package context provides the ExecutionContext for Jazz3 workflow engine.
//
// ExecutionContext is the central integration point that carries all capabilities
// through workflow execution. It combines:
//   - Token being processed
//   - Clock for time operations
//   - Observability tools (Tracer, Metrics, Logger)
//   - Error handling components (ErrorHandler, ErrorRecorder)
//   - Standard Go context for cancellation
//   - Context inheritance for hierarchical workflows
//
// Design Principles:
//   - Zero overhead when observability is disabled (NoOp implementations)
//   - Thread-safe for concurrent access
//   - Supports context inheritance for sub-workflows
//   - Integrates with Go's context.Context for cancellation
//
// Example usage:
//
//	ctx := context.NewExecutionContext(
//	    goCtx,
//	    clock.NewRealTimeClock(),
//	    myToken,
//	)
//	ctx.Logger.Info("Starting task", map[string]interface{}{"task": "example"})
//	span := ctx.Tracer.StartSpan("task-execution")
//	defer span.End()
package context

import (
	"context"

	"github.com/jazzpetri/engine/clock"
	"github.com/jazzpetri/engine/token"
)

// ExecutionContext carries all capabilities through workflow execution.
// It combines token processing, time management, observability, and error handling
// in a single context that can be passed through the entire execution pipeline.
//
// The context supports inheritance for hierarchical workflows, where child workflows
// inherit observability and error handling from their parent but maintain their own
// workflow-specific identity.
type ExecutionContext struct {
	// Context is the standard Go context for cancellation and deadlines.
	// This allows workflows to be cancelled gracefully using context cancellation.
	Context context.Context

	// Token is the token being processed by the current transition.
	// This is nil if the context is not associated with a specific token.
	Token *token.Token

	// Clock provides time operations for the workflow.
	// Use VirtualClock for testing or RealTimeClock for production.
	Clock clock.Clock

	// Observability components (zero-overhead when disabled)

	// Tracer handles distributed tracing (OpenTelemetry).
	// Defaults to NoOpTracer which has zero overhead.
	Tracer Tracer

	// Metrics handles metrics collection (Prometheus).
	// Defaults to NoOpMetrics which has zero overhead.
	Metrics MetricsCollector

	// Logger handles structured logging.
	// Defaults to NoOpLogger which has zero overhead.
	Logger Logger

	// Error handling components

	// ErrorHandler handles retry and compensation logic.
	// Defaults to NoOpErrorHandler which passes through errors.
	ErrorHandler ErrorHandler

	// ErrorRecorder records errors for observability.
	// Defaults to NoOpErrorRecorder which has zero overhead.
	ErrorRecorder ErrorRecorder

	// EventLog records workflow events for recovery and auditing.
	// This is nil by default and must be explicitly configured.
	EventLog EventLog

	// EventBuilder creates events for the event log.
	// This is nil by default and must be explicitly configured.
	EventBuilder EventBuilder

	// parent is the parent context for hierarchical workflows.
	// This is nil for root workflows.
	parent *ExecutionContext
}

// NewExecutionContext creates a new execution context with default NoOp implementations.
// This provides zero-overhead observability and error handling when not explicitly configured.
//
// Parameters:
//   - ctx: Standard Go context for cancellation and deadlines
//   - clk: Clock implementation for time operations
//   - tok: Token being processed (can be nil)
//
// Returns a new ExecutionContext with NoOp implementations for all observability
// and error handling components.
func NewExecutionContext(ctx context.Context, clk clock.Clock, tok *token.Token) *ExecutionContext {
	ec := &ExecutionContext{
		Context: ctx,
		Clock:   clk,
		Token:   tok,
		// Default to NoOp implementations (zero overhead)
		Tracer:        &NoOpTracer{},
		Metrics:       &NoOpMetrics{},
		Logger:        &NoOpLogger{},
		ErrorHandler:  &NoOpErrorHandler{},
		ErrorRecorder: &NoOpErrorRecorder{},
	}
	// Ensure observability is never nil
	ec.ensureObservability()
	return ec
}

// ensureObservability ensures all observability components are non-nil.
// This is called automatically after context creation or modification to prevent
// nil pointer dereferences. Any nil components are replaced with NoOp implementations
// that have zero overhead.
//
// This method can be called on any ExecutionContext to normalize it after direct
// field assignment (e.g., ctx.Logger = nil).
func (e *ExecutionContext) ensureObservability() {
	if e.Logger == nil {
		e.Logger = &NoOpLogger{}
	}
	if e.Metrics == nil {
		e.Metrics = &NoOpMetrics{}
	}
	if e.Tracer == nil {
		e.Tracer = &NoOpTracer{}
	}
	if e.ErrorHandler == nil {
		e.ErrorHandler = &NoOpErrorHandler{}
	}
	if e.ErrorRecorder == nil {
		e.ErrorRecorder = &NoOpErrorRecorder{}
	}
}

// WithParent creates a child context that inherits observability and error handling from parent.
// The child context maintains its own token and Go context but shares the parent's
// observability and error handling configuration.
//
// This is used for hierarchical workflows where subnets need to inherit the parent's
// configuration while maintaining their own execution state.
//
// Parameters:
//   - parent: The parent ExecutionContext to inherit from
//
// Returns a new ExecutionContext that inherits from the parent.
func (e *ExecutionContext) WithParent(parent *ExecutionContext) *ExecutionContext {
	child := *e
	child.parent = parent
	// Inherit observability and error handling from parent
	child.Tracer = parent.Tracer
	child.Metrics = parent.Metrics
	child.Logger = parent.Logger
	child.ErrorHandler = parent.ErrorHandler
	child.ErrorRecorder = parent.ErrorRecorder
	child.EventLog = parent.EventLog
	child.EventBuilder = parent.EventBuilder
	return &child
}

// Parent returns the parent context for hierarchical workflows.
// Returns nil if this is a root context with no parent.
func (e *ExecutionContext) Parent() *ExecutionContext {
	return e.parent
}

// GetLogger returns the logger, ensuring it's never nil.
// If the logger was set to nil, this returns a NoOpLogger.
func (e *ExecutionContext) GetLogger() Logger {
	if e.Logger == nil {
		e.Logger = &NoOpLogger{}
	}
	return e.Logger
}

// GetMetrics returns the metrics collector, ensuring it's never nil.
// If the metrics was set to nil, this returns a NoOpMetrics.
func (e *ExecutionContext) GetMetrics() MetricsCollector {
	if e.Metrics == nil {
		e.Metrics = &NoOpMetrics{}
	}
	return e.Metrics
}

// GetTracer returns the tracer, ensuring it's never nil.
// If the tracer was set to nil, this returns a NoOpTracer.
func (e *ExecutionContext) GetTracer() Tracer {
	if e.Tracer == nil {
		e.Tracer = &NoOpTracer{}
	}
	return e.Tracer
}

// GetErrorHandler returns the error handler, ensuring it's never nil.
// If the error handler was set to nil, this returns a NoOpErrorHandler.
func (e *ExecutionContext) GetErrorHandler() ErrorHandler {
	if e.ErrorHandler == nil {
		e.ErrorHandler = &NoOpErrorHandler{}
	}
	return e.ErrorHandler
}

// GetErrorRecorder returns the error recorder, ensuring it's never nil.
// If the error recorder was set to nil, this returns a NoOpErrorRecorder.
func (e *ExecutionContext) GetErrorRecorder() ErrorRecorder {
	if e.ErrorRecorder == nil {
		e.ErrorRecorder = &NoOpErrorRecorder{}
	}
	return e.ErrorRecorder
}

// WithTracer returns a new context with the specified tracer.
// This allows enabling distributed tracing for specific workflows.
func (e *ExecutionContext) WithTracer(tracer Tracer) *ExecutionContext {
	newCtx := *e
	newCtx.Tracer = tracer
	newCtx.ensureObservability()
	return &newCtx
}

// WithMetrics returns a new context with the specified metrics collector.
// This allows enabling metrics collection for specific workflows.
func (e *ExecutionContext) WithMetrics(metrics MetricsCollector) *ExecutionContext {
	newCtx := *e
	newCtx.Metrics = metrics
	newCtx.ensureObservability()
	return &newCtx
}

// WithLogger returns a new context with the specified logger.
// This allows enabling structured logging for specific workflows.
func (e *ExecutionContext) WithLogger(logger Logger) *ExecutionContext {
	newCtx := *e
	newCtx.Logger = logger
	newCtx.ensureObservability()
	return &newCtx
}

// WithErrorHandler returns a new context with the specified error handler.
// This allows customizing error handling behavior for specific workflows.
func (e *ExecutionContext) WithErrorHandler(handler ErrorHandler) *ExecutionContext {
	newCtx := *e
	newCtx.ErrorHandler = handler
	newCtx.ensureObservability()
	return &newCtx
}

// WithErrorRecorder returns a new context with the specified error recorder.
// This allows enabling error recording for specific workflows.
func (e *ExecutionContext) WithErrorRecorder(recorder ErrorRecorder) *ExecutionContext {
	newCtx := *e
	newCtx.ErrorRecorder = recorder
	newCtx.ensureObservability()
	return &newCtx
}

// WithToken returns a new context with the specified token.
// This is used when processing different tokens through transitions.
func (e *ExecutionContext) WithToken(tok *token.Token) *ExecutionContext {
	newCtx := *e
	newCtx.Token = tok
	return &newCtx
}

// WithContext returns a new context with the specified Go context.
// This is used when creating child contexts with different cancellation behavior.
func (e *ExecutionContext) WithContext(ctx context.Context) *ExecutionContext {
	newCtx := *e
	newCtx.Context = ctx
	return &newCtx
}

// WithEventLog returns a new context with the specified event log.
// This allows enabling event logging for specific workflows.
func (e *ExecutionContext) WithEventLog(log EventLog) *ExecutionContext {
	newCtx := *e
	newCtx.EventLog = log
	return &newCtx
}

// WithEventBuilder returns a new context with the specified event builder.
// This allows enabling event creation for specific workflows.
func (e *ExecutionContext) WithEventBuilder(builder EventBuilder) *ExecutionContext {
	newCtx := *e
	newCtx.EventBuilder = builder
	return &newCtx
}

// Clone creates a copy of the ExecutionContext that can be modified.
// This returns a builder that allows chaining modifications.
// ExecutionContext Builder Pattern
func (e *ExecutionContext) Clone() *ExecutionContextBuilder {
	return &ExecutionContextBuilder{
		ctx:           e.Context,
		clock:         e.Clock,
		token:         e.Token,
		logger:        e.Logger,
		metrics:       e.Metrics,
		tracer:        e.Tracer,
		errorHandler:  e.ErrorHandler,
		errorRecorder: e.ErrorRecorder,
		eventLog:      e.EventLog,
		eventBuilder:  e.EventBuilder,
	}
}
