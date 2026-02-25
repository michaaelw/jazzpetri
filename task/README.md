# Task Package

The task package provides the Task interface and built-in task adapters for Jazz3 workflow engine.

## Installation

The task package can be used as a standalone Go module:

```bash
go get jazz3/core/task
```

**Dependencies:**
- `jazz3/core/context` - Execution context for observability and capabilities
- `jazz3/core/token` - Token system for data flow

## Standalone Usage

The task package can be used independently from Jazz3 for building workflow-like task composition:

```go
package main

import (
    "context"
    "fmt"
    "time"

    "jazz3/core/clock"
    execCtx "jazz3/core/context"
    "jazz3/core/task"
    "jazz3/core/token"
)

type OrderData struct {
    ID     string
    Status string
}

func (o *OrderData) Type() string { return "order" }

func main() {
    // Setup execution context
    clk := clock.NewRealTimeClock()
    tok := &token.Token{
        ID:   "order-123",
        Data: &OrderData{ID: "order-123", Status: "new"},
    }
    ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

    // Example 1: InlineTask - Simple function execution
    validateTask := &task.InlineTask{
        Name: "validate-order",
        Fn: func(ctx *execCtx.ExecutionContext) error {
            order := ctx.Token.Data.(*OrderData)
            fmt.Printf("Validating order: %s\n", order.ID)
            order.Status = "validated"
            return nil
        },
    }

    // Example 2: SequenceTask - Execute tasks in order
    workflow := &task.SequentialTask{
        Name: "order-workflow",
        Tasks: []task.Task{
            validateTask,
            &task.InlineTask{
                Name: "process-order",
                Fn: func(ctx *execCtx.ExecutionContext) error {
                    order := ctx.Token.Data.(*OrderData)
                    fmt.Printf("Processing order: %s\n", order.ID)
                    order.Status = "processed"
                    return nil
                },
            },
        },
    }

    // Example 3: ParallelTask - Execute tasks concurrently
    parallelChecks := &task.ParallelTask{
        Name: "parallel-checks",
        Tasks: []task.Task{
            &task.InlineTask{
                Name: "check-inventory",
                Fn: func(ctx *execCtx.ExecutionContext) error {
                    fmt.Println("Checking inventory...")
                    time.Sleep(100 * time.Millisecond)
                    return nil
                },
            },
            &task.InlineTask{
                Name: "check-credit",
                Fn: func(ctx *execCtx.ExecutionContext) error {
                    fmt.Println("Checking credit...")
                    time.Sleep(100 * time.Millisecond)
                    return nil
                },
            },
        },
    }

    // Execute the workflow
    if err := workflow.Execute(ctx); err != nil {
        fmt.Printf("Workflow failed: %v\n", err)
        return
    }

    // Execute parallel checks
    if err := parallelChecks.Execute(ctx); err != nil {
        fmt.Printf("Parallel checks failed: %v\n", err)
        return
    }

    fmt.Println("All tasks completed successfully")
}
```

**Key Features for Standalone Use:**
- **Composable Tasks**: Nest Sequential, Parallel, Conditional, Retry, and Recovery tasks
- **Type Safety**: Strong typing with compile-time checks
- **Testable**: Use VirtualClock for deterministic testing
- **Observable**: Built-in support for tracing, metrics, and logging
- **Thread Safe**: All tasks safe for concurrent execution

## Overview

Tasks are the units of work executed by transitions in the Petri net. The package implements the **Ports & Adapters pattern**:

- **Port**: `Task` interface - the contract all tasks implement
- **Adapters**: `InlineTask`, `HTTPTask`, `CommandTask` - concrete implementations

All tasks:
- Execute within an `ExecutionContext` for full observability
- Are thread-safe for concurrent execution
- Handle errors through `ExecutionContext.ErrorHandler`
- Support cancellation via `context.Context`
- Have immutable configuration after creation

## Architecture

```
┌─────────────────────────────────────────┐
│           Task Interface                │
│  Execute(ctx *ExecutionContext) error   │
└─────────────────────────────────────────┘
                    ▲
                    │
        ┌───────────┴───────────┐
        │                       │
   Built-in Tasks         Composite Tasks
        │                       │
  ┌─────┴─────┐           ┌────┴─────┐
  │  Inline   │           │Sequential│
  │   HTTP    │           │ Parallel │
  │  Command  │           │Conditional
  │   Wait    │           │  Retry   │
  │   Emit    │           │ Recovery │
  │   Human   │           └──────────┘
  │           │           (can nest any Task)
  │SubWorkflow│
  │Escalation │
  └───────────┘
```

## Task Types

### InlineTask

Executes Go functions directly within the workflow engine.

**Use Cases:**
- Data transformations
- Business logic execution
- Token data manipulation
- Synchronous operations

**Example:**
```go
task := &task.InlineTask{
    Name: "calculate-total",
    Fn: func(ctx *context.ExecutionContext) error {
        // Access token data
        order := ctx.Token.Data.(OrderData)

        // Perform calculation
        total := 0.0
        for _, item := range order.Items {
            total += item.Price * float64(item.Quantity)
        }
        order.Total = total

        // Log the result
        ctx.Logger.Info("Calculated order total", map[string]interface{}{
            "order_id": order.ID,
            "total": total,
        })

        return nil
    },
}
```

**Features:**
- Direct function execution (fastest)
- Full access to token data
- Panic recovery (panics converted to errors)
- Zero overhead when observability disabled

### HTTPTask

Executes HTTP requests to external services.

**Use Cases:**
- Calling external APIs
- Triggering webhooks
- Microservice communication
- Event notifications

**Example:**
```go
task := &task.HTTPTask{
    Name:   "notify-webhook",
    Method: "POST",
    URL:    "https://api.example.com/webhooks/order",
    Headers: map[string]string{
        "Content-Type": "application/json",
        "Authorization": "Bearer " + apiToken,
    },
    Body: []byte(`{"event":"order.created","order_id":"123"}`),
    Timeout: 30 * time.Second,
}
```

**Features:**
- Full HTTP method support (GET, POST, PUT, DELETE, etc.)
- Custom headers and request body
- Configurable timeout
- Context cancellation support
- Custom HTTP client support
- Automatic error handling for 4xx/5xx status codes

### CommandTask

Executes shell commands and external processes.

**Use Cases:**
- Running scripts (Python, Bash, etc.)
- Executing CLI tools
- Running data processing pipelines
- Integrating with legacy systems
- Performing system operations

**Example:**
```go
task := &task.CommandTask{
    Name:    "data-processing",
    Command: "python",
    Args:    []string{"process.py", "--input", "data.csv", "--output", "results.json"},
    Env: map[string]string{
        "API_KEY": "secret123",
        "REGION":  "us-west-2",
    },
    Dir: "/path/to/working/directory",
}
```

**Features:**
- Full subprocess control
- Custom environment variables
- Working directory configuration
- Stdout/stderr capture
- Exit code checking
- Context cancellation (kills process)

### WaitTask

Pauses workflow execution for a specified duration or until a deadline.

**Use Cases:**
- Adding delays between workflow steps
- Implementing rate limiting
- Waiting for external processes to complete
- SLA timeout boundaries
- Scheduled task execution

**Example - Fixed delay:**
```go
task := &task.WaitTask{
    Name:     "cool-down",
    Duration: 5 * time.Second,
}
```

**Example - Wait until specific time:**
```go
task := &task.WaitTask{
    Name:  "wait-until-9am",
    Until: time.Date(2024, 1, 15, 9, 0, 0, 0, time.Local),
}
```

**Example - Dynamic duration from token data:**
```go
task := &task.WaitTask{
    Name: "dynamic-wait",
    DurationFunc: func(ctx *context.ExecutionContext) time.Duration {
        priority := ctx.Token.Metadata["priority"]
        if priority == "high" {
            return 1 * time.Second
        }
        return 10 * time.Second
    },
}
```

**Features:**
- Fixed duration, specific deadline, or dynamic calculation
- Respects context cancellation during wait
- Uses VirtualClock for deterministic testing
- Zero-overhead when observability disabled

### EmitEventTask

Publishes events to external messaging systems (event buses, message queues).

**Use Cases:**
- Publishing domain events to event buses
- Triggering downstream workflows
- Notifying external systems of state changes
- Audit logging to event streams
- Integration with Kafka, RabbitMQ, etc.

**Example - Static payload:**
```go
task := &task.EmitEventTask{
    Name:    "order-created",
    Emitter: kafkaEmitter,
    Topic:   "orders.created",
    Payload: map[string]interface{}{
        "event_type": "ORDER_CREATED",
        "version":    "1.0",
    },
}
```

**Example - Dynamic payload from token:**
```go
task := &task.EmitEventTask{
    Name:    "order-completed",
    Emitter: kafkaEmitter,
    Topic:   "orders.completed",
    PayloadFunc: func(ctx *context.ExecutionContext) (interface{}, error) {
        order := ctx.Token.Data.(*OrderData)
        return map[string]interface{}{
            "order_id": order.ID,
            "total":    order.Total,
            "status":   order.Status,
        }, nil
    },
}
```

**Features:**
- Static or dynamic payload generation
- EventEmitter interface for different backends (Kafka, RabbitMQ, Redis, etc.)
- Built-in ChannelEventEmitter for testing
- Thread-safe for concurrent execution

### HumanTask

Enables human-in-the-loop workflow patterns including approvals, reviews, and data entry.

**Use Cases:**
- Approval workflows (expense approval, PR reviews, contract signing)
- Manual quality gates (human review of AI output)
- Data collection (user input, form submission)
- Exception handling (human intervention on failures)
- Compliance checkpoints (regulatory sign-offs)

**Example - Simple approval:**
```go
task := &task.HumanTask{
    Name:        "manager-approval",
    Interaction: slackInteraction,
    Mode:        task.HumanModeApproval,
    Timeout:     24 * time.Hour,
    Request: &task.ApprovalRequest{
        Title:       "Expense Report Approval",
        Description: "Review and approve expense report #1234",
        Priority:    "medium",
    },
    OnApproved: func(ctx *context.ExecutionContext, decision *task.HumanDecision) error {
        ctx.Logger.Info("Approved by", map[string]interface{}{"approver": decision.DecisionBy})
        return nil
    },
    OnRejected: func(ctx *context.ExecutionContext, decision *task.HumanDecision) error {
        return fmt.Errorf("rejected: %s", decision.Comments)
    },
}
```

**Example - Data entry:**
```go
task := &task.HumanTask{
    Name:        "shipping-address",
    Interaction: webUIInteraction,
    Mode:        task.HumanModeDataEntry,
    Timeout:     1 * time.Hour,
    DataRequest: &task.DataEntryRequest{
        Title:       "Shipping Address Required",
        Description: "Please provide shipping address",
        Fields: []task.DataField{
            {Name: "street", Label: "Street Address", Type: "text", Required: true},
            {Name: "city", Label: "City", Type: "text", Required: true},
            {Name: "zip", Label: "ZIP Code", Type: "text", Required: true},
        },
    },
    OnDataReceived: func(ctx *context.ExecutionContext, data *task.HumanDataResponse) error {
        ctx.Token.Metadata["shipping_address"] = data.Data
        return nil
    },
}
```

**Modes:**
- `HumanModeApproval` - Wait for approval/rejection
- `HumanModeDataEntry` - Wait for data input
- `HumanModeReview` - Fire-and-forget notification (non-blocking)

**Features:**
- HumanInteraction interface for different backends (Slack, Email, Web UI, etc.)
- Timeout handling with optional callback
- Dynamic request generation from token data
- Built-in ChannelHumanInteraction for testing

### SubWorkflowTask

Invokes nested workflows for composition and reuse.

**Use Cases:**
- Breaking complex workflows into reusable components
- Parallel execution of independent sub-processes
- Conditional workflow branches
- Workflow templates with parameterization
- Hierarchical process orchestration

**Example - Simple sub-workflow:**
```go
task := &task.SubWorkflowTask{
    Name:       "process-order",
    Invoker:    workflowInvoker,
    WorkflowID: "order-processing",
    InputFunc: func(ctx *context.ExecutionContext) (interface{}, error) {
        order := ctx.Token.Data.(*OrderData)
        return map[string]interface{}{
            "order_id": order.ID,
            "items":    order.Items,
        }, nil
    },
    OnSuccess: func(ctx *context.ExecutionContext, result *task.WorkflowResult) error {
        ctx.Token.Metadata["sub_result"] = result.Output
        return nil
    },
}
```

**Example - Async (fire-and-forget):**
```go
task := &task.SubWorkflowTask{
    Name:       "send-notifications",
    Invoker:    workflowInvoker,
    WorkflowID: "notification-batch",
    Async:      true,  // Don't wait for completion
    Input: map[string]interface{}{
        "template": "order-confirmation",
    },
}
```

**Example - With timeout and failure handling:**
```go
task := &task.SubWorkflowTask{
    Name:            "verify-identity",
    Invoker:         workflowInvoker,
    WorkflowID:      "identity-verification",
    Timeout:         5 * time.Minute,
    PropagateCancel: true,  // Cancel sub-workflow if parent is cancelled
    OnFailure: func(ctx *context.ExecutionContext, result *task.WorkflowResult) error {
        // Escalate to manual review
        ctx.Token.Metadata["needs_manual_review"] = true
        return nil
    },
}
```

**Features:**
- WorkflowInvoker interface decouples from engine package
- Sync (wait for completion) or async (fire-and-forget) modes
- Dynamic workflow ID and input generation
- Timeout handling with optional callback
- Cancellation propagation to sub-workflows
- Built-in MockWorkflowInvoker for testing

### EscalationTask

Handles escalation when SLAs are breached or conditions are met.

**Use Cases:**
- SLA breach escalations (support tickets, incident response)
- Approval timeout escalations (when approvers don't respond)
- Multi-level notification chains (L1 → L2 → L3 managers)
- Conditional escalations (based on data thresholds)
- Automated incident management

**Example - Time-based SLA escalation:**
```go
task := &task.EscalationTask{
    Name: "support-ticket-escalation",
    Levels: []task.EscalationConfig{
        {Level: task.EscalationLevel1, EscalateTo: []string{"support-team"}, After: 30 * time.Minute},
        {Level: task.EscalationLevel2, EscalateTo: []string{"senior-support"}, After: 1 * time.Hour},
        {Level: task.EscalationLevel3, EscalateTo: []string{"manager"}, After: 2 * time.Hour},
    },
    CheckInterval: 1 * time.Minute,
    OnEscalate: func(ctx *context.ExecutionContext, level task.EscalationLevel, recipients []string) error {
        // Send notifications to escalation recipients
        return notifyTeam(recipients, "Ticket escalated to " + level.String())
    },
}
```

**Example - Condition-based escalation with resolution:**
```go
task := &task.EscalationTask{
    Name: "order-delay-escalation",
    Levels: []task.EscalationConfig{
        {Level: task.EscalationLevel1, EscalateTo: []string{"ops-team"}, After: 5 * time.Minute},
        {Level: task.EscalationLevel2, EscalateTo: []string{"ops-manager"}, After: 15 * time.Minute},
    },
    CheckInterval: 1 * time.Minute,
    EscalateWhen: func(ctx *context.ExecutionContext) (bool, error) {
        order := ctx.Token.Data.(*OrderData)
        delay := ctx.Clock.Now().Sub(order.CreatedAt)
        return delay > order.SLA, nil
    },
    Resolver: dbResolver,
    OnResolved: func(ctx *context.ExecutionContext, level task.EscalationLevel) error {
        ctx.Logger.Info("Issue resolved before full escalation", map[string]interface{}{
            "stopped_at_level": level.String(),
        })
        return nil
    },
}
```

**Features:**
- Multi-level escalation chains with configurable durations
- Time-based and condition-based escalation triggers
- Resolution mechanism to stop escalation early
- Customizable notification handlers at each level
- Full observability with metrics and tracing
- Built-in ChannelEscalationResolver for testing

## ExecutionContext Integration

All tasks execute within an `ExecutionContext` which provides:

### Token Access
```go
// Access token data
data := ctx.Token.Data.(MyDataType)

// Modify token data
data.Status = "processed"
```

### Observability

**Tracing:**
```go
span := ctx.Tracer.StartSpan("custom-operation")
defer span.End()
span.SetAttribute("key", "value")
```

**Metrics:**
```go
ctx.Metrics.Inc("items_processed")
ctx.Metrics.Observe("processing_duration_seconds", duration)
```

**Logging:**
```go
ctx.Logger.Info("Processing item", map[string]interface{}{
    "item_id": item.ID,
    "status": "in_progress",
})
```

### Error Handling

```go
// Errors are automatically processed by ErrorHandler
if err := processItem(item); err != nil {
    return err // ErrorHandler will handle retry/compensation
}
```

### Cancellation

```go
select {
case <-ctx.Context.Done():
    return ctx.Context.Err()
case result := <-processingChan:
    // Handle result
}
```

## Usage Examples

### Simple Workflow Pipeline

```go
package main

import (
    "context"
    "fmt"

    "jazz3/core/clock"
    execCtx "jazz3/core/context"
    "jazz3/core/task"
    "jazz3/core/token"
)

type OrderData struct {
    ID     string
    Total  float64
    Status string
}

func (o *OrderData) Type() string { return "order" }

func main() {
    // Create execution context
    clk := clock.NewRealTimeClock()
    tok := &token.Token{
        ID: "order-123",
        Data: &OrderData{
            ID:    "order-123",
            Total: 0,
            Status: "new",
        },
    }
    ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

    // Define workflow tasks
    tasks := []task.Task{
        // Step 1: Calculate total
        &task.InlineTask{
            Name: "calculate-total",
            Fn: func(ctx *execCtx.ExecutionContext) error {
                order := ctx.Token.Data.(*OrderData)
                order.Total = 99.99 // Simplified calculation
                order.Status = "calculated"
                return nil
            },
        },

        // Step 2: Notify external service
        &task.HTTPTask{
            Name:   "notify-service",
            Method: "POST",
            URL:    "https://api.example.com/orders",
            Headers: map[string]string{
                "Content-Type": "application/json",
            },
            Body: []byte(fmt.Sprintf(`{"order_id":"%s","total":%.2f}`,
                tok.Data.(*OrderData).ID,
                tok.Data.(*OrderData).Total)),
        },

        // Step 3: Generate report
        &task.CommandTask{
            Name:    "generate-report",
            Command: "python",
            Args:    []string{"generate_report.py", "--order", "order-123"},
        },
    }

    // Execute workflow
    for i, t := range tasks {
        if err := t.Execute(ctx); err != nil {
            fmt.Printf("Task %d failed: %v\n", i, err)
            return
        }
    }

    fmt.Println("Workflow completed successfully")
}
```

### Concurrent Task Execution

```go
func processConcurrently(items []Item) error {
    clk := clock.NewRealTimeClock()

    task := &task.InlineTask{
        Name: "process-item",
        Fn: func(ctx *execCtx.ExecutionContext) error {
            item := ctx.Token.Data.(*ItemData)
            return processItem(item)
        },
    }

    var wg sync.WaitGroup
    errChan := make(chan error, len(items))

    for _, item := range items {
        wg.Add(1)
        go func(item Item) {
            defer wg.Done()

            tok := &token.Token{
                ID: item.ID,
                Data: &ItemData{Item: item},
            }
            ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

            if err := task.Execute(ctx); err != nil {
                errChan <- err
            }
        }(item)
    }

    wg.Wait()
    close(errChan)

    // Check for errors
    for err := range errChan {
        return err
    }

    return nil
}
```

### Custom Error Handling

```go
type RetryErrorHandler struct {
    MaxRetries int
}

func (h *RetryErrorHandler) Handle(ctx *execCtx.ExecutionContext, err error) error {
    // Implement retry logic
    for attempt := 1; attempt <= h.MaxRetries; attempt++ {
        ctx.Logger.Warn("Retrying task", map[string]interface{}{
            "attempt": attempt,
            "max_retries": h.MaxRetries,
        })

        ctx.ErrorRecorder.RecordRetry(attempt, time.Second * time.Duration(attempt))
        ctx.Clock.Sleep(time.Second * time.Duration(attempt))

        // Retry would happen here in actual implementation
    }

    return err // Return original error after retries exhausted
}

// Use custom error handler
ctx = ctx.WithErrorHandler(&RetryErrorHandler{MaxRetries: 3})
```

## Testing

### Unit Testing Tasks

```go
func TestMyTask(t *testing.T) {
    // Arrange
    clk := clock.NewVirtualClock(time.Now())
    tok := &token.Token{ID: "test-1"}
    ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

    task := &task.InlineTask{
        Name: "test-task",
        Fn: func(ctx *execCtx.ExecutionContext) error {
            // Task logic
            return nil
        },
    }

    // Act
    err := task.Execute(ctx)

    // Assert
    if err != nil {
        t.Errorf("Expected no error, got: %v", err)
    }
}
```

### Testing with Mock Observability

```go
type MockMetrics struct {
    counters map[string]float64
}

func (m *MockMetrics) Inc(name string) {
    m.counters[name]++
}

// ... implement other MetricsCollector methods

func TestTaskWithMetrics(t *testing.T) {
    metrics := &MockMetrics{counters: make(map[string]float64)}
    ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)
    ctx = ctx.WithMetrics(metrics)

    task.Execute(ctx)

    if metrics.counters["task_executions_total"] != 1 {
        t.Error("Expected task execution metric")
    }
}
```

## Performance

### Benchmarks

Task execution overhead is minimal:

- **InlineTask**: ~500ns per execution (just function call overhead)
- **HTTPTask**: Dominated by network latency
- **CommandTask**: Dominated by process startup time

### Thread Safety

All task types are safe for concurrent execution:
- Tasks can be executed by multiple goroutines simultaneously
- Each execution receives its own ExecutionContext
- Task configuration is immutable after creation

### Zero-Cost Observability

When using NoOp implementations (default), observability has zero overhead:
- Compiler optimizes away NoOp method calls
- No allocations for disabled observability

## Error Handling

### Error Flow

1. Task returns error
2. Error recorded by `ErrorRecorder`
3. Error processed by `ErrorHandler` (retry/compensation)
4. Error logged and traced
5. Error propagated to caller

### Common Error Patterns

**Network Errors (HTTPTask):**
- Connection refused
- Timeout
- DNS resolution failure
- HTTP error status codes (4xx, 5xx)

**Command Errors (CommandTask):**
- Command not found
- Non-zero exit code
- Signal termination
- Permission denied

**Runtime Errors (InlineTask):**
- Panics (recovered and converted to errors)
- Returned errors
- Type assertion failures

## Best Practices

1. **Use the right task type:**
   - InlineTask for in-process logic
   - HTTPTask for external services
   - CommandTask for scripts and CLI tools

2. **Keep tasks focused:**
   - Single responsibility per task
   - Compose complex workflows from simple tasks

3. **Handle errors explicitly:**
   - Return meaningful error messages
   - Use ErrorRecorder for observability
   - Implement retry logic in ErrorHandler

4. **Use observability:**
   - Log important events
   - Record metrics for monitoring
   - Create spans for distributed tracing

5. **Test thoroughly:**
   - Unit test each task
   - Test error scenarios
   - Test with VirtualClock for determinism

6. **Design for concurrency:**
   - Ensure tasks are thread-safe
   - Use proper synchronization for shared state
   - Leverage ExecutionContext for isolation

## Metrics Reference

All tasks emit these metrics:
- `task_executions_total` - Total task executions
- `task_success_total` - Successful executions
- `task_errors_total` - Failed executions

Type-specific metrics:
- `task_inline_*` - InlineTask metrics
- `task_http_*` - HTTPTask metrics
- `task_http_duration_seconds` - HTTP request duration
- `task_command_*` - CommandTask metrics
- `task_command_duration_seconds` - Command execution duration
- `task_wait_*` - WaitTask metrics
- `task_wait_duration_seconds` - Actual wait duration
- `task_emit_*` - EmitEventTask metrics
- `task_emit_topic_*_total` - Per-topic event counts
- `task_human_*` - HumanTask metrics
- `task_human_approved_total` / `task_human_rejected_total` - Approval outcomes
- `task_human_timeout_total` - Human task timeouts
- `task_subworkflow_*` - SubWorkflowTask metrics
- `task_subworkflow_completed_total` / `task_subworkflow_failed_total` - Sub-workflow outcomes
- `task_escalation_*` - EscalationTask metrics
- `escalation_started_total` - Escalation processes started
- `escalation_level_reached` - Counter for each level reached
- `escalation_resolved_total` - Escalations resolved before completion
- `escalation_duration_seconds` - Total escalation duration (histogram)

## Test Coverage

The task package has **88.5% test coverage** with comprehensive tests for:
- All task types
- Error scenarios
- Concurrent execution
- Observability integration
- Context cancellation
- Integration scenarios

All tests pass race detector verification.

## Composite Task Patterns

The task package includes powerful composite patterns that allow tasks to be combined and nested to create complex workflows. All composite tasks implement the `Task` interface and can be composed with unlimited nesting.

### SequentialTask

Executes tasks in order, stopping on the first error. Token flows through the chain - each task can read and modify it.

**Use Cases:**
- Multi-step workflows where order matters
- Data transformation pipelines
- Workflows with dependencies between steps

**Example:**
```go
seq := &task.SequentialTask{
    Name: "order-processing",
    Tasks: []task.Task{
        &task.InlineTask{Name: "validate", Fn: validateOrder},
        &task.HTTPTask{Name: "payment", URL: "https://payment.example.com/charge"},
        &task.InlineTask{Name: "fulfill", Fn: fulfillOrder},
    },
}
```

**Features:**
- Sequential execution with deterministic order
- Token modifications visible to subsequent tasks
- Early termination on error
- Full observability for each step

### ParallelTask

Executes tasks concurrently using goroutines. Each task receives a copy of the token for isolation.

**Use Cases:**
- Independent operations that can run concurrently
- I/O-bound tasks (HTTP requests, database queries)
- Fan-out patterns

**Example:**
```go
parallel := &task.ParallelTask{
    Name: "fetch-data",
    Tasks: []task.Task{
        &task.HTTPTask{Name: "fetch-users", URL: "https://api.example.com/users"},
        &task.HTTPTask{Name: "fetch-orders", URL: "https://api.example.com/orders"},
        &task.HTTPTask{Name: "fetch-products", URL: "https://api.example.com/products"},
    },
}
```

**Features:**
- Concurrent execution for performance
- Token isolation (each task gets a copy)
- All tasks run to completion
- Errors collected into MultiError

### ConditionalTask

Executes one of two tasks based on a condition function. ElseTask is optional.

**Use Cases:**
- Branching workflows based on data
- Dynamic routing
- Optional task execution

**Example:**
```go
conditional := &task.ConditionalTask{
    Name: "route-order",
    Condition: func(ctx *context.ExecutionContext) bool {
        order := ctx.Token.Data.(*OrderData)
        return order.Priority == "high"
    },
    ThenTask: &task.HTTPTask{Name: "express-shipping", URL: "..."},
    ElseTask: &task.HTTPTask{Name: "standard-shipping", URL: "..."},
}
```

**Features:**
- Dynamic branching based on runtime conditions
- Optional else branch
- Panic recovery in condition function
- Full condition result tracing

### RetryTask

Wraps a task with retry logic and exponential backoff.

**Use Cases:**
- HTTP requests that may fail transiently
- Database operations with temporary connection issues
- External service calls that need resilience

**Example - Fixed backoff:**
```go
retry := &task.RetryTask{
    Name: "fetch-with-retry",
    Task: &task.HTTPTask{URL: "https://api.example.com/data"},
    MaxAttempts: 3,
    Backoff: 100 * time.Millisecond,
}
```

**Example - Exponential backoff:**
```go
retry := &task.RetryTask{
    Name: "fetch-with-retry",
    Task: &task.HTTPTask{URL: "https://api.example.com/data"},
    MaxAttempts: 5,
    BackoffFunc: func(attempt int) time.Duration {
        return time.Duration(1<<uint(attempt)) * 100 * time.Millisecond
    },
}
```

**Features:**
- Configurable max attempts
- Fixed or exponential backoff
- Respects context cancellation
- Logs each retry attempt
- Clock-based backoff (testable with VirtualClock)

### RecoveryTask

Wraps a task with error recovery and compensation logic (Saga pattern).

**Use Cases:**
- Distributed transactions with rollback
- Compensating transactions
- Error recovery with cleanup actions

**Example:**
```go
recovery := &task.RecoveryTask{
    Name: "payment-with-rollback",
    Task: &task.HTTPTask{
        URL: "https://payment.example.com/charge",
        Method: "POST",
    },
    CompensationTask: &task.HTTPTask{
        URL: "https://payment.example.com/refund",
        Method: "POST",
    },
}
```

**Saga Pattern Example:**
```go
saga := &task.SequentialTask{
    Name: "order-saga",
    Tasks: []task.Task{
        &task.RecoveryTask{
            Name: "reserve-inventory",
            Task: &task.InlineTask{Name: "reserve", Fn: reserveInventory},
            CompensationTask: &task.InlineTask{Name: "release", Fn: releaseInventory},
        },
        &task.RecoveryTask{
            Name: "charge-payment",
            Task: &task.InlineTask{Name: "charge", Fn: chargePayment},
            CompensationTask: &task.InlineTask{Name: "refund", Fn: refundPayment},
        },
    },
}
```

**Features:**
- Automatic compensation on failure
- Successful compensation = successful recovery
- Saga pattern support for distributed transactions
- Full tracing of primary and compensation tasks

### Complex Workflow Example

Composites can be nested to create sophisticated workflows:

```go
workflow := &task.SequentialTask{
    Name: "order-processing",
    Tasks: []task.Task{
        // Validate order
        &task.InlineTask{Name: "validate", Fn: validateOrder},

        // Conditional routing based on priority
        &task.ConditionalTask{
            Name: "route-by-priority",
            Condition: func(ctx *context.ExecutionContext) bool {
                return ctx.Token.Data.(*OrderData).Priority == "high"
            },
            ThenTask: &task.SequentialTask{
                Name: "high-priority-flow",
                Tasks: []task.Task{
                    // Parallel checks
                    &task.ParallelTask{
                        Name: "parallel-checks",
                        Tasks: []task.Task{
                            &task.HTTPTask{Name: "check-inventory", URL: "..."},
                            &task.HTTPTask{Name: "check-credit", URL: "..."},
                        },
                    },
                    // Payment with retry and recovery
                    &task.RecoveryTask{
                        Name: "payment-with-recovery",
                        Task: &task.RetryTask{
                            Name: "charge",
                            Task: &task.HTTPTask{URL: "https://payment.com/charge"},
                            MaxAttempts: 3,
                            Backoff: 100 * time.Millisecond,
                        },
                        CompensationTask: &task.InlineTask{Name: "notify-failure", Fn: notifyFailure},
                    },
                },
            },
            ElseTask: &task.InlineTask{Name: "standard-processing", Fn: standardProcessing},
        },

        // Final notification
        &task.HTTPTask{Name: "notify", URL: "https://webhook.example.com/notify"},
    },
}
```

### Composite Metrics

All composite tasks emit additional metrics:
- `sequence_tasks_total` - Sequential task executions
- `sequence_success_total` / `sequence_errors_total` - Success/failure counts
- `parallel_tasks_total` - Parallel task executions
- `conditional_tasks_total` - Conditional task executions
- `condition_true_total` / `condition_false_total` - Branch taken counts
- `retry_tasks_total` / `retry_attempts_total` - Retry metrics
- `retry_success_total` / `retry_failure_total` - Retry outcomes
- `recovery_tasks_total` / `compensation_executed_total` - Recovery metrics

## Future Enhancements

Potential additions:
- **gRPC Task**: For gRPC service calls
- **Database Task**: For SQL operations
- **Plugin System**: For custom task adapters
- **Response Storage**: Store HTTP/Command output in tokens
- **Loop/Iterate Task**: Execute tasks over collections
- **Map/Reduce Task**: Process collections in parallel with aggregation

## See Also

- [ExecutionContext](../context/README.md) - Execution context documentation
- [Token System](../token/README.md) - Token and data flow
- [Clock Abstraction](../clock/README.md) - Time management
- [Petri Net Model](../petri/README.md) - Workflow model
