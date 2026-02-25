# Clock Package

The `clock` package provides time abstractions for the Jazz3 workflow engine, enabling both production use with real wall-clock time and deterministic testing with virtual time.

## Installation

```bash
go get github.com/jazz3/jazz3/core/clock
```

## Standalone Usage

This package can be used independently of Jazz3 in any Go project that needs controllable time for testing:

```go
package main

import (
    "fmt"
    "time"

    "github.com/jazz3/jazz3/core/clock"
)

func main() {
    // Production: use real time
    realClock := clock.NewRealTimeClock()
    fmt.Printf("Current time: %v\n", realClock.Now())

    // Testing: use virtual time
    virtualClock := clock.NewVirtualClock(time.Now())
    timer := virtualClock.After(5 * time.Second)

    // Fast-forward time instantly
    virtualClock.AdvanceBy(10 * time.Second)

    <-timer // Fires immediately
    fmt.Println("Timer fired!")
}
```

## Overview

Jazz3 workflows often involve time-dependent operations such as delays, timeouts, and scheduling. The clock package abstracts these time operations behind a common `Clock` interface, allowing the same workflow code to:

- Run in production using real system time (`RealTimeClock`)
- Run in tests with controllable virtual time (`VirtualClock`)
- Support time-travel debugging and fast-forwarding through delays

This architecture enables:
- **Deterministic Testing**: Tests run at virtual time speed, not wall-clock speed
- **Fast Tests**: Hours of workflow time can execute in milliseconds
- **Time-Travel Debugging**: Step through workflows at any time granularity
- **Zero Production Overhead**: RealTimeClock delegates directly to Go's time package

## Architecture

### Clock Interface

The `Clock` interface defines three core operations:

```go
type Clock interface {
    Now() time.Time
    After(d time.Duration) <-chan time.Time
    Sleep(d time.Duration)
}
```

All implementations must be safe for concurrent use by multiple goroutines.

### RealTimeClock

`RealTimeClock` provides zero-overhead production time:

```go
clock := clock.NewRealTimeClock()
now := clock.Now()              // Delegates to time.Now()
timer := clock.After(5*time.Second)  // Delegates to time.After()
clock.Sleep(1*time.Second)      // Delegates to time.Sleep()
```

### VirtualClock

`VirtualClock` provides controllable time for testing:

```go
start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
clock := clock.NewVirtualClock(start)

// Time does not advance automatically
timer := clock.After(5 * time.Second)

// Manually advance time
clock.AdvanceBy(10 * time.Second)

// Timer fires immediately since we've passed the deadline
<-timer
```

## Usage Examples

### Production Workflow with Real Time

```go
package main

import (
    "fmt"
    "time"

    "github.com/jazz3/jazz3/core/clock"
)

func runWorkflow(clk clock.Clock) {
    fmt.Printf("Workflow started at %v\n", clk.Now())

    // Wait 5 seconds for external service
    <-clk.After(5 * time.Second)

    fmt.Printf("Service call completed at %v\n", clk.Now())
}

func main() {
    // Production: use real time
    clk := clock.NewRealTimeClock()
    runWorkflow(clk)
    // Output: waits actual 5 seconds
}
```

### Testing with Virtual Time

```go
package main

import (
    "testing"
    "time"

    "github.com/jazz3/jazz3/core/clock"
)

func TestWorkflow(t *testing.T) {
    start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
    clk := clock.NewVirtualClock(start)

    // Start workflow in goroutine
    done := make(chan bool)
    go func() {
        runWorkflow(clk)
        done <- true
    }()

    // Advance virtual time instantly
    clk.AdvanceBy(10 * time.Second)

    // Workflow completes immediately (no actual waiting)
    <-done

    if !clk.Now().Equal(start.Add(10 * time.Second)) {
        t.Error("Time did not advance correctly")
    }
}
```

### Concurrent Timers with Virtual Clock

```go
func TestMultipleTimers(t *testing.T) {
    start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
    clk := clock.NewVirtualClock(start)

    // Create multiple timers with different deadlines
    timer1 := clk.After(1 * time.Second)
    timer2 := clk.After(2 * time.Second)
    timer3 := clk.After(3 * time.Second)

    // Advance by 1.5 seconds - only timer1 fires
    clk.AdvanceBy(1500 * time.Millisecond)

    select {
    case <-timer1:
        // Expected: timer1 fired
    case <-time.After(100 * time.Millisecond):
        t.Fatal("timer1 should have fired")
    }

    // timer2 and timer3 not fired yet
    select {
    case <-timer2:
        t.Fatal("timer2 fired too early")
    default:
        // Expected
    }

    // Advance to fire remaining timers
    clk.AdvanceBy(2 * time.Second)

    <-timer2  // Fires immediately
    <-timer3  // Fires immediately
}
```

### Time-Travel Debugging Example

```go
func DebugWorkflowAt(clk *clock.VirtualClock, targetTime time.Time) {
    // Fast-forward to specific point in workflow
    clk.AdvanceTo(targetTime)

    // Inspect workflow state at this exact moment
    fmt.Printf("Workflow state at %v\n", clk.Now())
    fmt.Printf("Pending timers: %d\n", clk.PendingTimers())
    fmt.Printf("Blocked sleeps: %d\n", clk.PendingSleeps())
}
```

### Sleep vs After

Both `Sleep()` and `After()` pause execution, but they work differently:

```go
// Sleep blocks the current goroutine
func processWithSleep(clk clock.Clock) {
    clk.Sleep(5 * time.Second)
    // This line executes after 5 seconds
}

// After returns a channel for select statements
func processWithAfter(clk clock.Clock) {
    timer := clk.After(5 * time.Second)

    select {
    case <-timer:
        // This executes after 5 seconds
    case <-cancelChan:
        // Can cancel before timer fires
    }
}
```

Use `After()` when you need to select on multiple channels. Use `Sleep()` for simple delays.

## Virtual Clock Behavior

### Time Advancement

Time in `VirtualClock` only advances explicitly:

```go
clk := clock.NewVirtualClock(start)

// These do NOT advance time:
time.Sleep(1 * time.Second)  // Real sleep, virtual time unchanged
<-time.After(1 * time.Second)  // Real timer, virtual time unchanged

// These DO advance time:
clk.AdvanceBy(1 * time.Second)  // Advance by duration
clk.AdvanceTo(futureTime)       // Advance to specific time
```

### Timer Firing Order

When time advances, timers fire in deadline order:

```go
clk := clock.NewVirtualClock(start)

t3 := clk.After(3 * time.Second)
t1 := clk.After(1 * time.Second)
t2 := clk.After(2 * time.Second)

clk.AdvanceBy(5 * time.Second)

// Timers fire in deadline order: t1, t2, t3
<-t1  // deadline: start + 1s
<-t2  // deadline: start + 2s
<-t3  // deadline: start + 3s
```

### Time Never Goes Backward

Attempting to move time backward is a no-op:

```go
clk := clock.NewVirtualClock(start)
clk.AdvanceBy(10 * time.Second)

// Attempting to go backward does nothing
clk.AdvanceTo(start)  // No-op
clk.AdvanceBy(-5 * time.Second)  // No-op

// Time remains at start + 10s
```

### Pending Operations

VirtualClock tracks pending operations:

```go
clk := clock.NewVirtualClock(start)

// Create 3 timers
t1 := clk.After(1 * time.Second)
t2 := clk.After(2 * time.Second)
t3 := clk.After(3 * time.Second)

fmt.Println(clk.PendingTimers())  // 3

clk.AdvanceBy(1500 * time.Millisecond)

fmt.Println(clk.PendingTimers())  // 2 (t1 fired)
```

This is useful for:
- Debugging: "Why hasn't this timer fired?"
- Resource leaks: "Did we forget to fire all timers?"
- Test assertions: "All timers should be cleaned up"

## Thread Safety

Both `RealTimeClock` and `VirtualClock` are safe for concurrent use:

```go
clk := clock.NewVirtualClock(start)

// These can all happen concurrently
go func() { _ = clk.Now() }()
go func() { <-clk.After(1 * time.Second) }()
go func() { clk.Sleep(2 * time.Second) }()
go func() { clk.AdvanceBy(5 * time.Second) }()
```

`VirtualClock` uses internal synchronization (RWMutex) to ensure correctness.

## Integration with Jazz3

In Jazz3, the `Clock` is carried by `ExecutionContext`:

```go
type ExecutionContext struct {
    WorkflowID string
    Clock      clock.Clock
    // ... other fields
}
```

Workflow components (transitions, tasks) use the context's clock:

```go
func (t *TimedTransition) Fire(ctx *ExecutionContext) error {
    // Wait for configured delay
    <-ctx.Clock.After(t.delay)

    // Execute task
    return t.task.Execute(ctx)
}
```

This ensures:
- Production workflows use real time
- Test workflows use virtual time
- Same code works in both environments

## Best Practices

### 1. Always Use Clock Interface

❌ **Bad**: Direct time.Now() usage
```go
func process() {
    start := time.Now()  // Not testable
    // ...
}
```

✅ **Good**: Use Clock interface
```go
func process(clk clock.Clock) {
    start := clk.Now()  // Testable
    // ...
}
```

### 2. Use After() for Select Statements

❌ **Bad**: Sleep blocks and cannot be cancelled
```go
func process(clk clock.Clock, cancel <-chan bool) {
    clk.Sleep(5 * time.Second)
    // Cannot detect cancellation during sleep
}
```

✅ **Good**: After() allows cancellation
```go
func process(clk clock.Clock, cancel <-chan bool) {
    select {
    case <-clk.After(5 * time.Second):
        // Normal completion
    case <-cancel:
        // Early cancellation
    }
}
```

### 3. Advance Time in Small Increments for Debugging

❌ **Bad**: Large time jumps miss intermediate states
```go
clk.AdvanceBy(1 * time.Hour)  // What happened in between?
```

✅ **Good**: Small increments for debugging
```go
for i := 0; i < 60; i++ {
    clk.AdvanceBy(1 * time.Minute)
    fmt.Printf("State at T+%dm: %v\n", i+1, getState())
}
```

### 4. Clean Up Timers in Tests

Always verify timers are cleaned up:

```go
func TestWorkflow(t *testing.T) {
    clk := clock.NewVirtualClock(start)

    runWorkflow(clk)

    // Ensure no leaked timers
    if clk.PendingTimers() != 0 {
        t.Errorf("Leaked %d timers", clk.PendingTimers())
    }
}
```

## Performance

### RealTimeClock

- **Zero overhead**: Direct delegation to Go's time package
- **No allocations**: Stateless implementation
- **No locks**: Completely lock-free

### VirtualClock

- **Read locks**: `Now()` uses RLock (multiple concurrent readers)
- **Write locks**: `After()`, `Sleep()`, `AdvanceBy()` use Lock (exclusive)
- **Memory**: O(n) where n is number of pending timers/sleeps
- **Fire timers**: O(n) where n is number of timers (could be optimized to heap-based)

For most use cases, VirtualClock performance is more than adequate for testing. If you have thousands of concurrent timers, consider using a production-optimized scheduler.

## Limitations

### VirtualClock Limitations

1. **Not suitable for production**: Virtual time is for testing only
2. **No real-time guarantees**: Timers don't fire until time is explicitly advanced
3. **Memory grows with timers**: Pending timers stored in memory
4. **No timer cancellation**: Once created, timers wait until deadline or test end

### RealTimeClock Limitations

1. **Not controllable**: Time advances at real-time rate
2. **Tests are slow**: Must wait actual duration
3. **Non-deterministic**: System load affects timing

## Testing

The clock package itself has 97% test coverage. Tests include:

- Basic functionality (Now, After, Sleep)
- Time advancement (AdvanceTo, AdvanceBy)
- Multiple concurrent timers
- Multiple concurrent sleeps
- Thread safety (race detector)
- Deterministic behavior
- Edge cases (zero/negative durations, backward time)

Run tests with:

```bash
go test ./core/clock/...
```

Run with race detector:

```bash
go test -race ./core/clock/...
```

Check coverage:

```bash
go test -cover ./core/clock/...
```

## Further Reading

- [Go time package documentation](https://pkg.go.dev/time)
- [Jazz3 Architecture: Time Management](../../docs/architecture/DISCOVERY-SUMMARY.md#decision-4-time-management-architecture)
- [Testing Time-Dependent Code](https://go.dev/wiki/TestingTips#testing-time-dependent-code)

## License

Part of the Jazz3 project. See LICENSE file in repository root.
