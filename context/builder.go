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

import (
	stdcontext "context"
	"github.com/jazzpetri/engine/clock"
	"github.com/jazzpetri/engine/token"
)

// ExecutionContextBuilder provides a fluent API for building ExecutionContext.
// ExecutionContext Builder Pattern
type ExecutionContextBuilder struct {
	ctx           stdcontext.Context
	clock         clock.Clock
	token         *token.Token
	logger        Logger
	metrics       MetricsCollector
	tracer        Tracer
	errorHandler  ErrorHandler
	errorRecorder ErrorRecorder
	eventLog      EventLog
	eventBuilder  EventBuilder
}

// NewExecutionContextBuilder creates a new builder with default values.
// ExecutionContext Builder Pattern
func NewExecutionContextBuilder() *ExecutionContextBuilder {
	return &ExecutionContextBuilder{
		ctx:     stdcontext.Background(),
		clock:   clock.NewRealTimeClock(),
		logger:  &NoOpLogger{},
		metrics: &NoOpMetrics{},
		tracer:  &NoOpTracer{},
	}
}

// WithLogger sets the logger.
func (b *ExecutionContextBuilder) WithLogger(logger Logger) *ExecutionContextBuilder {
	b.logger = logger
	return b
}

// WithMetrics sets the metrics collector.
func (b *ExecutionContextBuilder) WithMetrics(metrics MetricsCollector) *ExecutionContextBuilder {
	b.metrics = metrics
	return b
}

// WithTracer sets the tracer.
func (b *ExecutionContextBuilder) WithTracer(tracer Tracer) *ExecutionContextBuilder {
	b.tracer = tracer
	return b
}

// WithContext sets the standard context.
func (b *ExecutionContextBuilder) WithContext(ctx stdcontext.Context) *ExecutionContextBuilder {
	b.ctx = ctx
	return b
}

// WithClock sets the clock.
func (b *ExecutionContextBuilder) WithClock(clock clock.Clock) *ExecutionContextBuilder {
	b.clock = clock
	return b
}

// WithToken sets the token.
func (b *ExecutionContextBuilder) WithToken(tok *token.Token) *ExecutionContextBuilder {
	b.token = tok
	return b
}

// WithErrorHandler sets the error handler.
func (b *ExecutionContextBuilder) WithErrorHandler(handler ErrorHandler) *ExecutionContextBuilder {
	b.errorHandler = handler
	return b
}

// WithErrorRecorder sets the error recorder.
func (b *ExecutionContextBuilder) WithErrorRecorder(recorder ErrorRecorder) *ExecutionContextBuilder {
	b.errorRecorder = recorder
	return b
}

// WithEventLog sets the event log.
func (b *ExecutionContextBuilder) WithEventLog(log EventLog) *ExecutionContextBuilder {
	b.eventLog = log
	return b
}

// WithEventBuilder sets the event builder.
func (b *ExecutionContextBuilder) WithEventBuilder(builder EventBuilder) *ExecutionContextBuilder {
	b.eventBuilder = builder
	return b
}

// Build creates the ExecutionContext.
func (b *ExecutionContextBuilder) Build() *ExecutionContext {
	ec := NewExecutionContext(b.ctx, b.clock, b.token)

	// Override with custom implementations if provided
	if b.logger != nil {
		ec.Logger = b.logger
	}
	if b.metrics != nil {
		ec.Metrics = b.metrics
	}
	if b.tracer != nil {
		ec.Tracer = b.tracer
	}
	if b.errorHandler != nil {
		ec.ErrorHandler = b.errorHandler
	}
	if b.errorRecorder != nil {
		ec.ErrorRecorder = b.errorRecorder
	}
	if b.eventLog != nil {
		ec.EventLog = b.eventLog
	}
	if b.eventBuilder != nil {
		ec.EventBuilder = b.eventBuilder
	}

	return ec
}

// ContextWithExecutionContext embeds ExecutionContext in a standard context.
// Context embedding (stub)
func ContextWithExecutionContext(ctx stdcontext.Context, execCtx *ExecutionContext) stdcontext.Context {
	return stdcontext.WithValue(ctx, executionContextKey, execCtx)
}

// ExecutionContextFromContext retrieves ExecutionContext from a standard context.
// Context extraction (stub)
func ExecutionContextFromContext(ctx stdcontext.Context) (*ExecutionContext, bool) {
	execCtx, ok := ctx.Value(executionContextKey).(*ExecutionContext)
	return execCtx, ok
}

type contextKey int

const executionContextKey contextKey = 0
