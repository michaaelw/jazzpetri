# Context Package

The `context` package provides the ExecutionContext for Jazz3 workflow engine, serving as the central integration point that carries all capabilities through workflow execution.

## Installation

```bash
# Install as a standalone module
go get jazz3/core/context

# Or if using Go workspace
cd /path/to/jazz3
go work use ./core/context
```

## Overview

ExecutionContext combines:
- Token being processed
- Clock for time operations
- Observability tools (Tracer, Metrics, Logger)
- Error handling components (ErrorHandler, ErrorRecorder)
- Standard Go context for cancellation
- Context inheritance for hierarchical workflows

## Design Principles

- **Zero Overhead**: When observability is disabled (default), NoOp implementations have zero runtime cost (0 allocations, <1ns overhead)
- **Thread-Safe**: All implementations are safe for concurrent use
- **Immutable**: Context modifications return new contexts (copy-on-write)
- **Hierarchical**: Child contexts inherit parent's observability and error handling
- **Composable**: Easily customize specific aspects while keeping defaults for others

## Quick Start

### Standalone Usage

This package can be used independently of the Jazz3 engine for any Go application that needs context propagation with observability:

```go
package main

import (
    "context"
    "fmt"
    "time"

    execctx "jazz3/core/context"
    "jazz3/core/clock"
    "jazz3/core/token"
)

func main() {
    // Create execution context with virtual clock for testing
    virtualClock := clock.NewVirtualClock(time.Now())
    ctx := execctx.NewExecutionContext(
        context.Background(),
        virtualClock,
        nil,
    )

    // Use context in your application
    processOrder(ctx, "order-123")

    // Advance time for testing
    virtualClock.AdvanceBy(5 * time.Second)

    // Check elapsed time
    fmt.Printf("Processing took: %v\n", virtualClock.Now().Sub(virtualClock.Now()))
}

func processOrder(ctx *execctx.ExecutionContext, orderID string) {
    // Start tracing
    span := ctx.Tracer.StartSpan("process-order")
    defer span.End()

    // Log structured data
    ctx.Logger.Info("Processing order", map[string]interface{}{
        "order_id": orderID,
        "timestamp": ctx.Clock.Now(),
    })

    // Record metrics
    ctx.Metrics.Inc("orders_processed_total")

    // Your business logic here
    fmt.Printf("Processing order %s at %v\n", orderID, ctx.Clock.Now())
}
```

### Basic Usage with Jazz3

```go
package main

import (
    "context"
    "jazz3/core/clock"
    execctx "jazz3/core/context"
    "jazz3/core/token"
)

func main() {
    // Create a basic execution context with defaults (zero overhead)
    ctx := execctx.NewExecutionContext(
        context.Background(),
        clock.NewRealTimeClock(),
        nil, // no token initially
    )

    // Use the context in workflow execution
    ctx.Logger.Info("Workflow started", nil)

    span := ctx.Tracer.StartSpan("task-execution")
    defer span.End()

    // Process with token
    tok := &token.Token{
        ID: "tok-123",
        Data: &token.IntToken{Value: 42},
    }
    ctx = ctx.WithToken(tok)
}
```

### Enabling Observability

```go
// Create custom observability implementations
tracer := NewOpenTelemetryTracer()
metrics := NewPrometheusCollector()
logger := NewZapLogger()

// Configure context with observability
ctx := context.NewExecutionContext(
    context.Background(),
    clock.NewRealTimeClock(),
    nil,
)

ctx = ctx.WithTracer(tracer)
ctx = ctx.WithMetrics(metrics)
ctx = ctx.WithLogger(logger)

// Now observability is enabled
ctx.Logger.Info("Transition fired", map[string]interface{}{
    "transition": "t1",
    "duration_ms": 123,
})

ctx.Metrics.Inc("transitions_fired")
ctx.Metrics.Observe("transition_duration_seconds", 0.123)
```

### Error Handling

```go
// Create custom error handler with retry logic
errorHandler := &RetryErrorHandler{
    MaxAttempts: 3,
    Backoff:     ExponentialBackoff(time.Second),
}

errorRecorder := &MetricsErrorRecorder{
    metrics: prometheusCollector,
}

ctx = ctx.WithErrorHandler(errorHandler)
ctx = ctx.WithErrorRecorder(errorRecorder)

// Handle errors during execution
if err := someTask.Execute(ctx); err != nil {
    // Record the error
    ctx.ErrorRecorder.RecordError(err, map[string]interface{}{
        "task": "process-payment",
        "attempt": 1,
    })

    // Handle with retry logic
    err = ctx.ErrorHandler.Handle(ctx, err)
}
```

### Hierarchical Workflows

```go
// Parent workflow context
parent := context.NewExecutionContext(
    context.Background(),
    clock.NewRealTimeClock(),
    nil,
)

// Configure parent with observability
parent = parent.WithTracer(tracer)
parent = parent.WithMetrics(metrics)
parent = parent.WithLogger(logger)

// Create child workflow that inherits observability
child := context.NewExecutionContext(
    context.Background(),
    clock.NewVirtualClock(time.Now()), // child can have different clock
    childToken,
)

child = child.WithParent(parent)

// Child automatically inherits:
// - Tracer from parent
// - Metrics from parent
// - Logger from parent
// - ErrorHandler from parent
// - ErrorRecorder from parent

// But maintains its own:
// - Go context (for cancellation)
// - Clock (can be different for testing)
// - Token (processing different data)
```

## API Reference

### Core Types

#### ExecutionContext

The main context type that carries all capabilities.

```go
type ExecutionContext struct {
    Context       context.Context    // Go context for cancellation
    Token         *token.Token       // Token being processed
    Clock         clock.Clock        // Time operations
    Tracer        Tracer            // Distributed tracing
    Metrics       MetricsCollector  // Metrics collection
    Logger        Logger            // Structured logging
    ErrorHandler  ErrorHandler      // Error retry/compensation
    ErrorRecorder ErrorRecorder     // Error recording
}
```

#### Constructor

```go
func NewExecutionContext(
    ctx context.Context,
    clk clock.Clock,
    tok *token.Token,
) *ExecutionContext
```

Creates a new ExecutionContext with NoOp implementations (zero overhead).

### Context Modification Methods

All modification methods return a **new** context (immutable pattern):

```go
// Set custom observability components
func (e *ExecutionContext) WithTracer(tracer Tracer) *ExecutionContext
func (e *ExecutionContext) WithMetrics(metrics MetricsCollector) *ExecutionContext
func (e *ExecutionContext) WithLogger(logger Logger) *ExecutionContext

// Set custom error handling
func (e *ExecutionContext) WithErrorHandler(handler ErrorHandler) *ExecutionContext
func (e *ExecutionContext) WithErrorRecorder(recorder ErrorRecorder) *ExecutionContext

// Set execution state
func (e *ExecutionContext) WithToken(tok *token.Token) *ExecutionContext
func (e *ExecutionContext) WithContext(ctx context.Context) *ExecutionContext

// Create hierarchical relationships
func (e *ExecutionContext) WithParent(parent *ExecutionContext) *ExecutionContext
func (e *ExecutionContext) Parent() *ExecutionContext
```

## Observability Interfaces

### Tracer

Distributed tracing interface (e.g., OpenTelemetry):

```go
type Tracer interface {
    StartSpan(name string) Span
}

type Span interface {
    End()
    SetAttribute(key string, value interface{})
    RecordError(err error)
}
```

**Example Implementation:**

```go
type OpenTelemetryTracer struct {
    provider trace.TracerProvider
}

func (t *OpenTelemetryTracer) StartSpan(name string) Span {
    ctx, span := t.provider.Tracer("jazz3").Start(
        context.Background(),
        name,
    )
    return &OTelSpan{ctx: ctx, span: span}
}
```

### MetricsCollector

Metrics collection interface (e.g., Prometheus):

```go
type MetricsCollector interface {
    Inc(name string)                    // Increment counter
    Add(name string, value float64)     // Add to counter
    Observe(name string, value float64) // Record histogram value
    Set(name string, value float64)     // Set gauge value
}
```

**Example Implementation:**

```go
type PrometheusCollector struct {
    counters   map[string]prometheus.Counter
    histograms map[string]prometheus.Histogram
    gauges     map[string]prometheus.Gauge
}
```

### Logger

Structured logging interface (e.g., Zap, Zerolog):

```go
type Logger interface {
    Debug(msg string, fields map[string]interface{})
    Info(msg string, fields map[string]interface{})
    Warn(msg string, fields map[string]interface{})
    Error(msg string, fields map[string]interface{})
}
```

**Example Implementation:**

```go
type ZapLogger struct {
    logger *zap.Logger
}

func (l *ZapLogger) Info(msg string, fields map[string]interface{}) {
    zapFields := make([]zap.Field, 0, len(fields))
    for k, v := range fields {
        zapFields = append(zapFields, zap.Any(k, v))
    }
    l.logger.Info(msg, zapFields...)
}
```

## Error Handling Interfaces

### ErrorHandler

Handles retry and compensation logic:

```go
type ErrorHandler interface {
    Handle(ctx *ExecutionContext, err error) error
}
```

**Example Implementation:**

```go
type RetryErrorHandler struct {
    MaxAttempts int
    Backoff     BackoffStrategy
}

func (h *RetryErrorHandler) Handle(ctx *ExecutionContext, err error) error {
    for attempt := 1; attempt <= h.MaxAttempts; attempt++ {
        ctx.ErrorRecorder.RecordRetry(attempt, h.Backoff.Next(attempt))

        time.Sleep(h.Backoff.Next(attempt))

        // Retry the operation (implementation specific)
        if retryErr := retryOperation(ctx); retryErr == nil {
            return nil
        }
    }
    return err
}
```

### ErrorRecorder

Records errors for observability:

```go
type ErrorRecorder interface {
    RecordError(err error, metadata map[string]interface{})
    RecordRetry(attempt int, nextRetry interface{})
    RecordCompensation(action string)
}
```

**Example Implementation:**

```go
type MetricsErrorRecorder struct {
    metrics MetricsCollector
}

func (r *MetricsErrorRecorder) RecordError(err error, metadata map[string]interface{}) {
    r.metrics.Inc("errors_total")

    if errorType, ok := metadata["error_type"].(string); ok {
        r.metrics.Inc("errors_by_type_" + errorType)
    }
}
```

## NoOp Implementations

All NoOp implementations have **zero runtime overhead**:

```go
type NoOpTracer struct{}        // 0 allocations, <1ns
type NoOpSpan struct{}          // 0 allocations, <1ns
type NoOpMetrics struct{}       // 0 allocations, <1ns
type NoOpLogger struct{}        // 0 allocations, <1ns
type NoOpErrorHandler struct{}  // 0 allocations, <1ns (passes through errors)
type NoOpErrorRecorder struct{} // 0 allocations, <1ns
```

These are the default implementations when you create a context with `NewExecutionContext()`. The Go compiler optimizes away these empty method calls, resulting in zero runtime cost.

## Performance Characteristics

Benchmark results (Apple M1 Max):

```
BenchmarkNoOpTracer-10           1000000000    0.31 ns/op    0 B/op    0 allocs/op
BenchmarkNoOpMetrics-10          1000000000    0.32 ns/op    0 B/op    0 allocs/op
BenchmarkNoOpLogger-10           1000000000    0.32 ns/op    0 B/op    0 allocs/op
BenchmarkNoOpErrorHandler-10     1000000000    0.31 ns/op    0 B/op    0 allocs/op
BenchmarkNoOpErrorRecorder-10    1000000000    0.32 ns/op    0 B/op    0 allocs/op
```

**Key Takeaway**: When observability is disabled (default), there is zero overhead.

## Testing

The package includes comprehensive test coverage (100%):

```bash
# Run tests
go test ./core/context/...

# Run tests with coverage
go test -cover ./core/context/...

# Run benchmarks
go test -bench=. -benchmem ./core/context/...
```

### Testing with Mock Implementations

The package provides mock implementations for testing:

```go
// Mock tracer that records spans
tracer := &mockTracer{}
ctx := context.NewExecutionContext(context.Background(), clock, nil)
ctx = ctx.WithTracer(tracer)

// Execute code that creates spans
span := ctx.Tracer.StartSpan("test-span")
span.SetAttribute("key", "value")
span.End()

// Verify spans were created
if len(tracer.spans) != 1 {
    t.Error("Expected 1 span")
}
```

## Integration with Jazz3 Components

### Token Package

```go
import "jazz3/core/token"

tok := &token.Token{
    ID: "tok-123",
    Data: &token.IntToken{Value: 42},
}

ctx = ctx.WithToken(tok)
```

### Clock Package

```go
import "jazz3/core/clock"

// Production: real time
ctx := context.NewExecutionContext(
    context.Background(),
    clock.NewRealTimeClock(),
    nil,
)

// Testing: virtual time
virtualClock := clock.NewVirtualClock(time.Now())
ctx := context.NewExecutionContext(
    context.Background(),
    virtualClock,
    nil,
)

// Advance time in tests
virtualClock.AdvanceBy(5 * time.Second)
```

## Best Practices

### 1. Use NoOp by Default

Start with the default NoOp implementations and only enable observability when needed:

```go
// Development/testing: zero overhead
ctx := context.NewExecutionContext(ctx, clock, token)

// Production: enable observability
if productionMode {
    ctx = ctx.WithTracer(otelTracer)
    ctx = ctx.WithMetrics(promCollector)
    ctx = ctx.WithLogger(zapLogger)
}
```

### 2. Create Context Once, Reuse

Create the context configuration once and reuse it:

```go
func NewProductionContext(tok *token.Token) *context.ExecutionContext {
    ctx := context.NewExecutionContext(
        context.Background(),
        clock.NewRealTimeClock(),
        tok,
    )

    return ctx.
        WithTracer(globalTracer).
        WithMetrics(globalMetrics).
        WithLogger(globalLogger).
        WithErrorHandler(globalErrorHandler)
}
```

### 3. Inherit for Hierarchical Workflows

Use `WithParent()` for subnets:

```go
// Parent workflow
parent := NewProductionContext(parentToken)

// Child workflow inherits observability
child := context.NewExecutionContext(
    context.Background(),
    clock,
    childToken,
).WithParent(parent)

// Child automatically has parent's tracer, metrics, logger, etc.
```

### 4. Use Structured Fields in Logs

Always use structured fields for better log analysis:

```go
// Good
ctx.Logger.Info("Transition fired", map[string]interface{}{
    "workflow_id": "wf-123",
    "transition_id": "t1",
    "duration_ms": 45,
})

// Avoid
ctx.Logger.Info("Transition t1 fired in 45ms", nil)
```

### 5. Record Metrics at Key Points

Instrument at important boundaries:

```go
// Before operation
start := ctx.Clock.Now()
span := ctx.Tracer.StartSpan("transition-fire")
defer span.End()

// Do work
err := transition.Execute(ctx)

// After operation
duration := ctx.Clock.Now().Sub(start).Seconds()
ctx.Metrics.Observe("transition_duration_seconds", duration)

if err != nil {
    ctx.Metrics.Inc("transition_errors_total")
}
```

## Future Enhancements

Potential future additions:

- **Profiler Interface**: CPU and memory profiling integration
- **Variables Map**: Shared state across workflow execution
- **Context Cancellation Helpers**: Wrapper methods for timeout and deadline management
- **Span Context Injection**: Automatic span context propagation to external services
- **Batch Metrics**: Efficient batch metric recording

## Contributing

When adding new observability or error handling features:

1. Define the interface in the appropriate file
2. Implement NoOp version (must be zero allocation)
3. Add benchmark to verify zero overhead
4. Provide mock implementation for testing
5. Update this README with usage examples

## License

This package is part of the Jazz3 workflow engine.
