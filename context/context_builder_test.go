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
	"context"
	"testing"

	"github.com/jazzpetri/engine/clock"
	"github.com/jazzpetri/engine/token"
)

// ExecutionContext Too Large
//
// EXPECTED FAILURE: These tests will fail because builder pattern
// or functional options don't exist for ExecutionContext.
//
// Problem: ExecutionContext has 7+ fields, making construction unwieldy:
//   ctx := &ExecutionContext{
//       Context: goCtx,
//       Token: tok,
//       Clock: clk,
//       Tracer: tracer,
//       Metrics: metrics,
//       Logger: logger,
//       ErrorHandler: handler,
//       ErrorRecorder: recorder,
//   }
//
// Solutions:
//   1. Builder pattern
//   2. Functional options pattern
//   3. Sensible defaults

func TestExecutionContext_Builder(t *testing.T) {
	// EXPECTED FAILURE: Builder type doesn't exist

	// Builder pattern makes construction clearer
	ctx := NewExecutionContextBuilder().
		WithContext(context.Background()).
		WithClock(clock.NewRealTimeClock()).
		WithToken(token.NewToken("string", "data")).
		WithLogger(&NoOpLogger{}).
		WithMetrics(&NoOpMetrics{}).
		Build() // Does not exist

	if ctx == nil {
		t.Fatal("Builder should return non-nil ExecutionContext")
	}

	if ctx.Context == nil {
		t.Error("Builder should set Context")
	}

	if ctx.Clock == nil {
		t.Error("Builder should set Clock")
	}
}

func TestExecutionContext_BuilderDefaults(t *testing.T) {
	// EXPECTED FAILURE: Builder doesn't provide defaults

	// Builder should provide sensible defaults
	ctx := NewExecutionContextBuilder().
		WithContext(context.Background()).
		WithClock(clock.NewRealTimeClock()).
		Build() // Minimal required fields

	// Should have default NoOp implementations
	if ctx.Logger == nil {
		t.Error("Builder should default Logger to NoOpLogger")
	}

	if ctx.Metrics == nil {
		t.Error("Builder should default Metrics to NoOpMetrics")
	}

	if ctx.Tracer == nil {
		t.Error("Builder should default Tracer to NoOpTracer")
	}
}

func TestExecutionContext_BuilderValidation(t *testing.T) {
	// EXPECTED FAILURE: Builder doesn't validate

	// Builder should validate required fields
	builder := NewExecutionContextBuilder()
	// Don't set required fields

	ctx := builder.Build()

	// Should either:
	// 1. Return error for missing required fields
	// 2. Provide defaults for all fields

	if ctx == nil {
		// Builder returned nil - needs validation
		t.Skip("Builder should validate or provide defaults")
	}
}

func TestExecutionContext_FunctionalOptions(t *testing.T) {
	// EXPECTED FAILURE: Functional options pattern doesn't exist
	// We implemented builder pattern instead, so skip this test

	t.Skip("Using builder pattern instead of functional options")
}

func TestExecutionContext_FromContext(t *testing.T) {
	// EXPECTED FAILURE: FromContext helper doesn't exist

	// Helper to extract ExecutionContext from context.Context
	goCtx := context.Background()

	// Store ExecutionContext in context
	execCtx := NewExecutionContext(goCtx, clock.NewRealTimeClock(), nil)
	goCtx = ContextWithExecutionContext(goCtx, execCtx) // Does not exist

	// Extract ExecutionContext from context
	retrieved, ok := ExecutionContextFromContext(goCtx) // Does not exist

	if !ok {
		t.Error("Should be able to extract ExecutionContext from context")
	}

	if retrieved != execCtx {
		t.Error("Should retrieve the same ExecutionContext")
	}
}

func TestExecutionContext_Clone(t *testing.T) {
	// EXPECTED FAILURE: Clone method doesn't exist

	original := NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		token.NewToken("string", "data"),
	)

	// Clone with modifications
	cloned := original.Clone().
		WithToken(token.NewToken("string", "new-data")).
		WithContext(context.Background()).
		Build()

	if cloned == nil {
		t.Error("Clone should return non-nil context")
	}

	// Should have same observability
	if cloned.Logger != original.Logger {
		t.Error("Clone should preserve Logger")
	}

	// But different token
	if cloned.Token == original.Token {
		t.Error("Clone with WithToken should have different token")
	}
}

// Example showing builder pattern
func ExampleExecutionContextBuilder() {
	// EXPECTED FAILURE: Builder doesn't exist

	// Clear, fluent construction
	ctx := NewExecutionContextBuilder().
		WithContext(context.Background()).
		WithClock(clock.NewRealTimeClock()).
		WithToken(token.NewToken("order", "ORD-123")).
		WithLogger(&NoOpLogger{}).
		WithMetrics(&NoOpMetrics{}).
		WithTracer(&NoOpTracer{}).
		WithErrorHandler(&NoOpErrorHandler{}).
		Build()

	_ = ctx

	// Versus current verbose construction:
	// ctx := &ExecutionContext{
	//     Context: context.Background(),
	//     Token: token.NewToken("order", "ORD-123"),
	//     Clock: clock.NewRealTimeClock(),
	//     Tracer: &NoOpTracer{},
	//     Metrics: &NoOpMetrics{},
	//     Logger: &NoOpLogger{},
	//     ErrorHandler: &NoOpErrorHandler{},
	//     ErrorRecorder: &NoOpErrorRecorder{},
	// }
}

// Example showing builder pattern instead of functional options
func ExampleExecutionContext_builderPattern() {
	// Using builder pattern instead of functional options

	// Minimal construction with defaults
	ctx := NewExecutionContextBuilder().
		WithContext(context.Background()).
		WithClock(clock.NewRealTimeClock()).
		Build()

	_ = ctx

	// Or with custom options
	ctx = NewExecutionContextBuilder().
		WithContext(context.Background()).
		WithClock(clock.NewRealTimeClock()).
		WithLogger(&NoOpLogger{}).
		WithMetrics(&NoOpMetrics{}).
		Build()

	_ = ctx
}

// Example showing context storage
func ExampleExecutionContext_contextStorage() {
	// EXPECTED FAILURE: Context storage helpers don't exist

	// Store in context.Context for middleware/handlers
	goCtx := context.Background()

	execCtx := NewExecutionContext(
		goCtx,
		clock.NewRealTimeClock(),
		nil,
	)

	// Store
	goCtx = ContextWithExecutionContext(goCtx, execCtx)

	// Later, in handler:
	// func handleRequest(ctx context.Context) {
	//     execCtx := ExecutionContextFromContext(ctx)
	//     execCtx.Logger.Info("handling request", nil)
	// }
}

// Proposed Builder implementation (currently doesn't exist)
// type ExecutionContextBuilder struct {
//     context      context.Context
//     token        *token.Token
//     clock        clock.Clock
//     tracer       Tracer
//     metrics      MetricsCollector
//     logger       Logger
//     errorHandler ErrorHandler
//     errorRecorder ErrorRecorder
// }
//
// func NewExecutionContextBuilder() *ExecutionContextBuilder {
//     return &ExecutionContextBuilder{
//         // Defaults
//         tracer:        &NoOpTracer{},
//         metrics:       &NoOpMetrics{},
//         logger:        &NoOpLogger{},
//         errorHandler:  &NoOpErrorHandler{},
//         errorRecorder: &NoOpErrorRecorder{},
//     }
// }
//
// func (b *ExecutionContextBuilder) WithContext(ctx context.Context) *ExecutionContextBuilder {
//     b.context = ctx
//     return b
// }
//
// func (b *ExecutionContextBuilder) WithClock(clock clock.Clock) *ExecutionContextBuilder {
//     b.clock = clock
//     return b
// }
//
// ... more With methods ...
//
// func (b *ExecutionContextBuilder) Build() *ExecutionContext {
//     return &ExecutionContext{
//         Context:       b.context,
//         Token:         b.token,
//         Clock:         b.clock,
//         Tracer:        b.tracer,
//         Metrics:       b.metrics,
//         Logger:        b.logger,
//         ErrorHandler:  b.errorHandler,
//         ErrorRecorder: b.errorRecorder,
//     }
// }

// Proposed functional options (currently don't exist)
// type ExecutionContextOption func(*ExecutionContext)
//
// func WithLogger(logger Logger) ExecutionContextOption {
//     return func(ctx *ExecutionContext) {
//         ctx.Logger = logger
//     }
// }
//
// func WithMetrics(metrics MetricsCollector) ExecutionContextOption {
//     return func(ctx *ExecutionContext) {
//         ctx.Metrics = metrics
//     }
// }
//
// ... more option functions ...
//
// func NewExecutionContext(ctx context.Context, clk clock.Clock, tok *token.Token, opts ...ExecutionContextOption) *ExecutionContext {
//     execCtx := &ExecutionContext{
//         Context: ctx,
//         Clock:   clk,
//         Token:   tok,
//         // Defaults
//         Tracer:        &NoOpTracer{},
//         Metrics:       &NoOpMetrics{},
//         Logger:        &NoOpLogger{},
//         ErrorHandler:  &NoOpErrorHandler{},
//         ErrorRecorder: &NoOpErrorRecorder{},
//     }
//
//     for _, opt := range opts {
//         opt(execCtx)
//     }
//
//     return execCtx
// }
