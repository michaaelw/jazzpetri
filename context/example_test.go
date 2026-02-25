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

package context_test

import (
	"context"
	"fmt"
	"time"

	"github.com/jazzpetri/engine/clock"
	ctx "github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/token"
)

// ExampleNewExecutionContext demonstrates basic ExecutionContext creation.
func ExampleNewExecutionContext() {
	// Create a basic execution context with defaults (zero overhead)
	execCtx := ctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// The context has NoOp implementations by default (zero overhead)
	fmt.Printf("Tracer type: %T\n", execCtx.Tracer)
	fmt.Printf("Metrics type: %T\n", execCtx.Metrics)
	fmt.Printf("Logger type: %T\n", execCtx.Logger)

	// Output:
	// Tracer type: *context.NoOpTracer
	// Metrics type: *context.NoOpMetrics
	// Logger type: *context.NoOpLogger
}

// ExampleExecutionContext_WithToken demonstrates token association.
func ExampleExecutionContext_WithToken() {
	execCtx := ctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Create a token
	tok := &token.Token{
		ID: "tok-123",
		Data: &token.IntToken{Value: 42},
	}

	// Associate token with context
	execCtx = execCtx.WithToken(tok)

	fmt.Printf("Token ID: %s\n", execCtx.Token.ID)
	fmt.Printf("Token Value: %d\n", execCtx.Token.Data.(*token.IntToken).Value)

	// Output:
	// Token ID: tok-123
	// Token Value: 42
}

// ExampleExecutionContext_WithParent demonstrates hierarchical context inheritance.
func ExampleExecutionContext_WithParent() {
	// Create parent context with custom logger
	parent := ctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// In real code, this would be a real logger implementation
	// parent = parent.WithLogger(zapLogger)

	// Create child context that inherits from parent
	child := ctx.NewExecutionContext(
		context.Background(),
		clock.NewVirtualClock(time.Now()),
		nil,
	)
	child = child.WithParent(parent)

	// Child inherits logger from parent
	fmt.Printf("Child has parent: %t\n", child.Parent() != nil)
	fmt.Printf("Child logger type: %T\n", child.Logger)

	// Output:
	// Child has parent: true
	// Child logger type: *context.NoOpLogger
}

// ExampleNoOpTracer demonstrates zero-overhead tracing when disabled.
func ExampleNoOpTracer() {
	execCtx := ctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Tracing operations have zero overhead when using NoOpTracer
	span := execCtx.Tracer.StartSpan("operation")
	span.SetAttribute("key", "value")
	span.End()

	// No output - operations are no-ops
	fmt.Println("Tracing operations completed with zero overhead")

	// Output:
	// Tracing operations completed with zero overhead
}

// ExampleNoOpMetrics demonstrates zero-overhead metrics when disabled.
func ExampleNoOpMetrics() {
	execCtx := ctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Metrics operations have zero overhead when using NoOpMetrics
	execCtx.Metrics.Inc("counter")
	execCtx.Metrics.Observe("histogram", 0.123)
	execCtx.Metrics.Set("gauge", 42.0)

	// No output - operations are no-ops
	fmt.Println("Metrics operations completed with zero overhead")

	// Output:
	// Metrics operations completed with zero overhead
}

// ExampleNoOpErrorHandler demonstrates error pass-through.
func ExampleNoOpErrorHandler() {
	execCtx := ctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Create an error
	err := fmt.Errorf("example error")

	// NoOpErrorHandler passes through errors unchanged
	result := execCtx.ErrorHandler.Handle(execCtx, err)

	fmt.Printf("Error passed through: %v\n", result)

	// Output:
	// Error passed through: example error
}

// ExampleExecutionContext_multiLevel demonstrates multi-level hierarchical inheritance.
func ExampleExecutionContext_multiLevel() {
	// Root context
	root := ctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Level 1 child
	child1 := ctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)
	child1 = child1.WithParent(root)

	// Level 2 grandchild
	child2 := ctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)
	child2 = child2.WithParent(child1)

	// Verify parent chain
	fmt.Printf("child2 -> child1: %t\n", child2.Parent() == child1)
	fmt.Printf("child1 -> root: %t\n", child1.Parent() == root)
	fmt.Printf("root -> nil: %t\n", root.Parent() == nil)

	// Output:
	// child2 -> child1: true
	// child1 -> root: true
	// root -> nil: true
}

// ExampleExecutionContext_contextCancellation demonstrates Go context integration.
func ExampleExecutionContext_contextCancellation() {
	// Create cancellable context
	goCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	execCtx := ctx.NewExecutionContext(
		goCtx,
		clock.NewRealTimeClock(),
		nil,
	)

	// Check if context is still valid
	select {
	case <-execCtx.Context.Done():
		fmt.Println("Context cancelled")
	default:
		fmt.Println("Context active")
	}

	// Output:
	// Context active
}

// ExampleExecutionContext_immutability demonstrates immutable context pattern.
func ExampleExecutionContext_immutability() {
	// Create original context
	original := ctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Modify context (creates new instance)
	tok := &token.Token{ID: "tok-123"}
	modified := original.WithToken(tok)

	// Original is unchanged
	fmt.Printf("Original token: %v\n", original.Token)
	fmt.Printf("Modified token: %s\n", modified.Token.ID)
	fmt.Printf("Different instances: %t\n", original != modified)

	// Output:
	// Original token: <nil>
	// Modified token: tok-123
	// Different instances: true
}
