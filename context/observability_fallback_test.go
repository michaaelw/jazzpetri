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
)

// No Graceful Degradation for Observability
//
// EXPECTED FAILURE: Tests will fail because the engine doesn't
// gracefully handle nil observability components.
//
// If logger/metrics/tracer are nil, the engine should:
//   - Use no-op implementations automatically
//   - Not panic or crash
//   - Continue operating normally
//   - Log a warning (if logger is available)

func TestEngine_NilLogger(t *testing.T) {
	// Tests that ExecutionContext auto-initializes nil logger through getter

	ctx := NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Explicitly set logger to nil
	ctx.Logger = nil

	// Should not panic when engine tries to log
	// Engine should auto-substitute NoOpLogger via getter
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Engine panicked with nil logger: %v", r)
		}
	}()

	// Access via getter to trigger auto-initialization
	logger := ctx.GetLogger()

	// Verify the getter returned a non-nil logger
	if logger == nil {
		t.Error("GetLogger() should return non-nil logger")
	}

	// Verify the field was fixed
	if ctx.Logger == nil {
		t.Error("GetLogger() should auto-initialize nil Logger field to NoOpLogger")
	}
}

func TestEngine_NilMetrics(t *testing.T) {
	// Tests that ExecutionContext auto-initializes nil metrics through getter

	ctx := NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Explicitly set metrics to nil
	ctx.Metrics = nil

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Engine panicked with nil metrics: %v", r)
		}
	}()

	// Access via getter to trigger auto-initialization
	metrics := ctx.GetMetrics()

	// Verify the getter returned a non-nil metrics
	if metrics == nil {
		t.Error("GetMetrics() should return non-nil metrics")
	}

	// Verify the field was fixed
	if ctx.Metrics == nil {
		t.Error("GetMetrics() should auto-initialize nil Metrics field to NoOpMetrics")
	}
}

func TestEngine_NilTracer(t *testing.T) {
	// Tests that ExecutionContext auto-initializes nil tracer through getter

	ctx := NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	ctx.Tracer = nil

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Engine panicked with nil tracer: %v", r)
		}
	}()

	// Access via getter to trigger auto-initialization
	tracer := ctx.GetTracer()

	// Verify the getter returned a non-nil tracer
	if tracer == nil {
		t.Error("GetTracer() should return non-nil tracer")
	}

	// Verify the field was fixed
	if ctx.Tracer == nil {
		t.Error("GetTracer() should auto-initialize nil Tracer field to NoOpTracer")
	}
}

func TestEngine_NilErrorHandler(t *testing.T) {
	// Tests that ExecutionContext auto-initializes nil error handler through getter

	ctx := NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	ctx.ErrorHandler = nil

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Engine panicked with nil error handler: %v", r)
		}
	}()

	// Access via getter to trigger auto-initialization
	handler := ctx.GetErrorHandler()

	// Verify the getter returned a non-nil error handler
	if handler == nil {
		t.Error("GetErrorHandler() should return non-nil error handler")
	}

	// Verify the field was fixed
	if ctx.ErrorHandler == nil {
		t.Error("GetErrorHandler() should auto-initialize nil ErrorHandler field to NoOpErrorHandler")
	}
}

func TestEngine_NilErrorRecorder(t *testing.T) {
	// Tests that ExecutionContext auto-initializes nil error recorder through getter

	ctx := NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	ctx.ErrorRecorder = nil

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Engine panicked with nil error recorder: %v", r)
		}
	}()

	// Access via getter to trigger auto-initialization
	recorder := ctx.GetErrorRecorder()

	// Verify the getter returned a non-nil error recorder
	if recorder == nil {
		t.Error("GetErrorRecorder() should return non-nil error recorder")
	}

	// Verify the field was fixed
	if ctx.ErrorRecorder == nil {
		t.Error("GetErrorRecorder() should auto-initialize nil ErrorRecorder field to NoOpErrorRecorder")
	}
}

func TestExecutionContext_AutoInitializeObservability(t *testing.T) {
	// EXPECTED FAILURE: NewExecutionContext doesn't auto-initialize

	// When creating context, all observability should default to NoOp
	ctx := NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// All observability components should be non-nil
	if ctx.Logger == nil {
		t.Error("Logger should be initialized to NoOpLogger")
	}

	if ctx.Metrics == nil {
		t.Error("Metrics should be initialized to NoOpMetrics")
	}

	if ctx.Tracer == nil {
		t.Error("Tracer should be initialized to NoOpTracer")
	}

	if ctx.ErrorHandler == nil {
		t.Error("ErrorHandler should be initialized to NoOpErrorHandler")
	}

	if ctx.ErrorRecorder == nil {
		t.Error("ErrorRecorder should be initialized to NoOpErrorRecorder")
	}
}

func TestExecutionContext_LazyInitialization(t *testing.T) {
	// EXPECTED FAILURE: Getters don't provide lazy initialization

	_ = &ExecutionContext{
		Context: context.Background(),
		Clock:   clock.NewRealTimeClock(),
	}

	// Don't set observability components

	// Accessing them should return NoOp implementations, not nil
	// Option 1: Getter methods with lazy init
	// logger := ctx.GetLogger() // Returns NoOpLogger if nil

	// Option 2: Direct field access with defensive checks in engine
	// The engine should check for nil before using

	// Current implementation: direct field access without checks
	// This leads to panics if fields are nil
}

func TestExecutionContext_With_Methods_PreserveDefaults(t *testing.T) {
	// EXPECTED FAILURE: With* methods might not preserve NoOp defaults

	ctx := NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Create new context with custom logger
	customLogger := &NoOpLogger{}
	newCtx := ctx.WithLogger(customLogger)

	// Other observability should still be non-nil NoOp implementations
	if newCtx.Metrics == nil {
		t.Error("WithLogger should preserve other NoOp defaults")
	}

	if newCtx.Tracer == nil {
		t.Error("WithLogger should preserve other NoOp defaults")
	}
}

func TestEngine_RobustObservability(t *testing.T) {
	// EXPECTED FAILURE: Engine doesn't validate observability before using

	// Engine should validate observability components on creation
	// and substitute NoOp implementations for nil values

	// Example pattern in NewEngine:
	// if ctx.Logger == nil {
	//     ctx.Logger = &NoOpLogger{}
	//     // Optionally warn via stderr
	// }
	// if ctx.Metrics == nil {
	//     ctx.Metrics = &NoOpMetrics{}
	// }
	// etc.

	t.Skip("Engine should validate and fix observability on creation")
}

// Example showing defensive observability usage
func ExampleExecutionContext_defensiveUsage() {
	// EXPECTED FAILURE: This pattern isn't used in the codebase

	ctx := &ExecutionContext{
		Context: context.Background(),
		Clock:   clock.NewRealTimeClock(),
		// Observability not set - could be nil
	}

	// DEFENSIVE: Check before using
	if ctx.Logger != nil {
		ctx.Logger.Info("starting", nil)
	}

	// BETTER: Initialize in NewExecutionContext
	// NewExecutionContext should return:
	// &ExecutionContext{
	//     Context: ctx,
	//     Clock: clk,
	//     Logger: &NoOpLogger{},    // Never nil
	//     Metrics: &NoOpMetrics{},  // Never nil
	//     Tracer: &NoOpTracer{},    // Never nil
	//     ...
	// }

	// BEST: Getter with lazy initialization
	// func (e *ExecutionContext) GetLogger() Logger {
	//     if e.Logger == nil {
	//         return &NoOpLogger{}
	//     }
	//     return e.Logger
	// }
}

// Example showing engine validation
func Example_engineValidateObservability() {
	// EXPECTED FAILURE: Engine doesn't validate on creation

	// In NewEngine, should validate and fix:
	// func NewEngine(net *PetriNet, ctx *ExecutionContext, config Config) *Engine {
	//     // Ensure observability is never nil
	//     if ctx.Logger == nil {
	//         ctx.Logger = &NoOpLogger{}
	//         fmt.Fprintln(os.Stderr, "Warning: Logger was nil, using NoOpLogger")
	//     }
	//     if ctx.Metrics == nil {
	//         ctx.Metrics = &NoOpMetrics{}
	//     }
	//     if ctx.Tracer == nil {
	//         ctx.Tracer = &NoOpTracer{}
	//     }
	//     if ctx.ErrorHandler == nil {
	//         ctx.ErrorHandler = &NoOpErrorHandler{}
	//     }
	//     if ctx.ErrorRecorder == nil {
	//         ctx.ErrorRecorder = &NoOpErrorRecorder{}
	//     }
	//
	//     // Now safe to use ctx throughout engine
	//     return &Engine{...}
	// }
}
