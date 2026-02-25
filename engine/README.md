# Engine Package

[![Go Reference](https://pkg.go.dev/badge/jazz3/core/engine.svg)](https://pkg.go.dev/jazz3/core/engine)

The `engine` package provides the complete workflow orchestration engine for Jazz3. It manages the execution of Petri net workflows, supporting both sequential and concurrent execution modes with configurable parallelism, comprehensive observability, and production-grade state management.

## Installation

```bash
go get jazz3/core/engine
```

## Module Status

This package is a standalone Go module with the following dependencies:
- `jazz3/core/context` - Execution context with observability
- `jazz3/core/petri` - Petri net model (places, transitions, arcs)
- `jazz3/core/state` - State management (markings, event log, restoration)
- `jazz3/core/token` - Token system and data carriers

## Overview

The engine is the top-level orchestration component that brings together:
- **Petri Nets** - Workflow structure (places, transitions, arcs)
- **Execution Context** - Observability, error handling, clock
- **Task Execution** - Business logic execution
- **Token Management** - Data flow through the workflow

## Features

- **Multiple Execution Modes**
  - Sequential: Single-threaded, deterministic execution
  - Concurrent: Parallel execution with goroutine pool

- **Lifecycle Management**
  - Start/Stop control
  - Graceful shutdown
  - Multiple start/stop cycles supported

- **Step-by-Step Execution**
  - Manual stepping for debugging
  - Fine-grained control over workflow progression

- **Thread Safety**
  - Concurrent transition firing
  - Safe for multi-threaded environments
  - Race detector passing

- **Resource Management**
  - Configurable goroutine pool
  - Bounded parallelism
  - No unbounded resource growth

- **Observability**
  - Integrated metrics collection
  - Distributed tracing support
  - Structured logging

## Architecture

### Execution Model

The engine follows a **polling-based** execution model:

1. **Poll** for enabled transitions at regular intervals
2. **Fire** enabled transitions according to execution mode
3. **Repeat** until workflow completes or stopped

This design provides:
- Simple and reliable implementation
- Predictable resource usage
- Easy to reason about behavior

### Sequential Mode

```
Poll -> Find Enabled -> Fire ONE -> Poll -> ...
```

- Fires one transition per poll cycle
- Deterministic execution order
- Single-threaded
- Best for: Debugging, simple workflows, deterministic requirements

### Concurrent Mode

```
Poll -> Find Enabled -> Fire ALL (with pool limit) -> Wait -> Poll -> ...
```

- Fires multiple transitions concurrently
- Goroutine pool limits parallelism
- Non-deterministic execution order
- Best for: Performance, parallel workflows, high throughput

## Usage

### Basic Example

```go
import (
    "context"
    "jazz3/core/clock"
    execContext "jazz3/core/context"
    "jazz3/core/engine"
    "jazz3/core/petri"
    "jazz3/core/task"
    "jazz3/core/token"
)

// Create a Petri net workflow
net := petri.NewPetriNet("my-workflow", "My Workflow")

// Add places
pStart := petri.NewPlace("start", "Start", 10)
pEnd := petri.NewPlace("end", "End", 10)

// Add transition with task
t := petri.NewTransition("process", "Process").WithTask(&task.InlineTask{
    Name: "process",
    Fn: func(ctx *execContext.ExecutionContext) error {
        // Your business logic here
        return nil
    },
})

// Build the net
net.AddPlace(pStart)
net.AddPlace(pEnd)
net.AddTransition(t)
net.AddArc(petri.NewArc("arc1", "start", "process", 1))
net.AddArc(petri.NewArc("arc2", "process", "end", 1))

// Resolve references (required before execution)
net.Resolve()

// Add initial token
pStart.AddToken(&token.Token{
    ID:   "tok1",
    Data: &petri.MapToken{Value: map[string]interface{}{}},
})

// Create execution context
ctx := execContext.NewExecutionContext(
    context.Background(),
    clock.NewRealTimeClock(),
    nil,
)

// Create and run engine
engine := engine.NewEngine(net, ctx, engine.DefaultConfig())
engine.Start()
engine.Wait()  // Wait for completion
engine.Stop()
```

### Configuration

```go
config := engine.Config{
    // Execution mode
    Mode: engine.ModeConcurrent,  // or ModeSequential

    // Max concurrent transitions (ignored in sequential mode)
    MaxConcurrentTransitions: 10,

    // How often to poll for enabled transitions
    PollInterval: 100 * time.Millisecond,

    // Enable step-by-step mode
    StepMode: false,
}

engine := engine.NewEngine(net, ctx, config)
```

### Sequential Execution

```go
config := engine.Config{
    Mode:         engine.ModeSequential,
    PollInterval: 50 * time.Millisecond,
}

engine := engine.NewEngine(net, ctx, config)
engine.Start()
engine.Wait()
engine.Stop()
```

### Concurrent Execution

```go
config := engine.Config{
    Mode:                     engine.ModeConcurrent,
    MaxConcurrentTransitions: 20,  // Pool size
    PollInterval:             100 * time.Millisecond,
}

engine := engine.NewEngine(net, ctx, config)
engine.Start()
engine.Wait()
engine.Stop()
```

### Step-by-Step Execution

```go
config := engine.Config{
    Mode:     engine.ModeSequential,
    StepMode: true,
}

engine := engine.NewEngine(net, ctx, config)

// Execute one transition at a time
for i := 0; i < 3; i++ {
    if err := engine.Step(); err != nil {
        // No more enabled transitions
        break
    }
}
```

### Lifecycle Management

```go
engine := engine.NewEngine(net, ctx, config)

// Start execution
if err := engine.Start(); err != nil {
    // Already running
}

// Check if running
if engine.IsRunning() {
    // Engine is active
}

// Stop execution
if err := engine.Stop(); err != nil {
    // Not running
}

// Multiple cycles supported
engine.Start()
engine.Stop()
engine.Start()
engine.Stop()
```

### Waiting for Completion

```go
engine := engine.NewEngine(net, ctx, config)
engine.Start()

// Block until workflow completes (no enabled transitions)
if err := engine.Wait(); err != nil {
    // Timeout (30 seconds default)
}

engine.Stop()
```

## Execution Patterns

### Parallel Processing

```go
// Create workflow with parallel branches
net := petri.NewPetriNet("parallel", "Parallel Workflow")

// Split: p1 -> tSplit -> (p2a and p2b)
// Process: p2a -> tA -> p3a, p2b -> tB -> p3b
// Join: p3a + p3b -> tJoin -> p4

// Use concurrent mode for parallel execution
config := engine.Config{
    Mode:                     engine.ModeConcurrent,
    MaxConcurrentTransitions: 10,
}
```

### Conditional Routing

```go
// Create transitions with guards
tApprove := petri.NewTransition("approve", "Approve").
    WithGuard(func(tokens []*token.Token) bool {
        // Check condition
        return isApproved(tokens)
    })

tReject := petri.NewTransition("reject", "Reject").
    WithGuard(func(tokens []*token.Token) bool {
        return !isApproved(tokens)
    })

// Engine will fire the transition whose guard passes
```

### Error Handling

```go
// Transitions that fail will rollback tokens
t := petri.NewTransition("process", "Process").WithTask(&task.InlineTask{
    Name: "risky-task",
    Fn: func(ctx *execContext.ExecutionContext) error {
        if err := riskyOperation(); err != nil {
            // Token will be returned to input place
            return err
        }
        return nil
    },
})

// Engine will continue executing other transitions
// Failed transition will be retried on next poll cycle
```

## Performance

### Benchmarks

Tests on simple workflows show:

- **Sequential Mode**: ~40ms for 2x20ms tasks (sequential execution)
- **Concurrent Mode**: ~20ms for 2x20ms tasks (parallel execution)

Performance characteristics:
- Sequential: Predictable, deterministic
- Concurrent: 2-3x faster for parallel workflows
- Pool size: Tune based on workload and system resources

### Tuning

```go
// High throughput
config := engine.Config{
    Mode:                     engine.ModeConcurrent,
    MaxConcurrentTransitions: 50,   // Large pool
    PollInterval:             50 * time.Millisecond,  // Fast polling
}

// Low latency, deterministic
config := engine.Config{
    Mode:         engine.ModeSequential,
    PollInterval: 10 * time.Millisecond,  // Very fast polling
}

// Balanced
config := engine.DefaultConfig()  // 10 workers, 100ms poll
```

## Testing

### Unit Tests

```bash
go test ./core/engine -run "^Test" -v
```

Coverage: **93.3%**

Tests include:
- Engine creation and configuration
- Start/Stop lifecycle
- Multiple start/stop cycles
- Sequential execution
- Concurrent execution
- Step-by-step mode
- Error handling
- Wait for completion

### Integration Tests

```bash
go test ./core/engine -run "TestComplete|TestParallel|TestConditional" -v
```

Tests include:
- Complete workflow execution
- Parallel branch workflows
- Conditional routing with guards
- Error recovery workflows
- Performance comparison

### Race Detection

```bash
go test -race ./core/engine
```

Status: **PASS** - No race conditions detected

### Example Tests

```bash
go test ./core/engine -run "^Example"
```

Examples demonstrate:
- Sequential workflow
- Parallel workflow
- Conditional routing
- Step-by-step debugging

## Design Decisions

### Polling vs Events

**Decision**: Polling-based execution

**Rationale**:
- Simple and reliable implementation
- Predictable resource usage
- Easy to reason about behavior
- Future: Can add event-driven optimization

### Goroutine Pool

**Decision**: Bounded pool for concurrent mode

**Rationale**:
- Prevents unbounded goroutine creation
- Predictable memory usage
- Configurable for different workloads
- Safe for long-running workflows

### Thread Safety

**Decision**: Place channels provide token storage safety

**Rationale**:
- Go channels are thread-safe by design
- No additional locking needed for token operations
- Engine uses mutex only for lifecycle state
- Multiple transitions can fire concurrently

### Completion Detection

**Decision**: Workflow completes when no transitions are enabled

**Rationale**:
- Simple and unambiguous condition
- No special "end" markers needed
- Works for all workflow patterns
- Wait() method provides synchronous completion

## Integration with Jazz3

### Observability

```go
// Engine integrates with ExecutionContext observability
ctx := execContext.NewExecutionContext(
    context.Background(),
    clock.NewRealTimeClock(),
    nil,
).
    WithTracer(myTracer).
    WithMetrics(myMetrics).
    WithLogger(myLogger)

engine := engine.NewEngine(net, ctx, config)
```

Metrics collected:
- `engine_idle_total` - Poll cycles with no enabled transitions
- `engine_transitions_fired_total` - Successful transition firings
- `engine_fire_errors_total` - Failed transition firings
- `engine_steps_total` - Manual step executions

### Error Handling

```go
// Engine uses ExecutionContext error handler
ctx := execContext.NewExecutionContext(
    context.Background(),
    clock.NewRealTimeClock(),
    nil,
).
    WithErrorHandler(myErrorHandler)

engine := engine.NewEngine(net, ctx, config)
```

Error handling:
- Failed transitions trigger ErrorHandler
- Tokens are rolled back to input places
- Engine continues executing other transitions
- Retry happens on next poll cycle

## Limitations

1. **Polling Overhead**: Minimum poll interval determines responsiveness
2. **No Priority**: All enabled transitions have equal priority
3. **No Deadlock Detection**: Engine will idle if workflow is structurally blocked
4. **No Distributed Execution**: Single-node execution only (future: distributed)

## Future Enhancements

1. **Event-Driven Enablement**: Optimize poll cycles with event notifications
2. **Priority Scheduling**: Support transition priorities
3. **Deadlock Detection**: Warn when workflow is blocked
4. **Distributed Execution**: Multi-node workflow execution
5. **Adaptive Polling**: Adjust poll interval based on activity

## See Also

- [core/petri](../petri) - Petri net model
- [core/task](../task) - Task execution
- [core/context](../context) - Execution context
- [core/token](../token) - Token system
- [Examples](engine_example_test.go) - Usage examples
- [Tests](engine_test.go) - Unit tests
