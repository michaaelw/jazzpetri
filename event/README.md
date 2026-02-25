# Jazz3 Event System

[![Go Reference](https://pkg.go.dev/badge/github.com/jazz3/jazz3/core/event.svg)](https://pkg.go.dev/github.com/jazz3/jazz3/core/event)

The `event` package provides a production-grade event-driven architecture implementation for Jazz3 workflows. It enables pub/sub patterns for event-driven workflow execution, webhook integration, external system notifications, and asynchronous event processing.

## Features

- **Simple Pub/Sub API**: Subscribe to events by type, publish events asynchronously or synchronously
- **Wildcard Subscriptions**: Subscribe to all events using `"*"` as the event type
- **Concurrent-Safe**: Thread-safe subscription management and event delivery
- **Panic Recovery**: Handlers that panic don't crash the system or affect other handlers
- **Timeout Protection**: Handler execution is limited to 30 seconds by default
- **Auto-Generated Fields**: Event IDs and timestamps are automatically set if not provided
- **Zero Dependencies**: Built using only Go standard library

## Installation

```bash
go get github.com/jazz3/jazz3/core/event
```

## Standalone Usage

The event package can be used independently of Jazz3 for any Go application that needs pub/sub functionality:

```go
package main

import (
    "fmt"
    "github.com/jazz3/jazz3/core/event"
)

func main() {
    // Create event system
    eventSys := event.NewInMemoryEventSystem()
    defer eventSys.Close()

    // Subscribe to specific event type
    handler := func(e event.Event) error {
        fmt.Printf("Received event: %s from %s\n", e.Type, e.Source)
        fmt.Printf("Data: %v\n", e.Data)
        return nil
    }

    subID, err := eventSys.Subscribe("order.created", handler)
    if err != nil {
        panic(err)
    }
    defer eventSys.Unsubscribe(subID)

    // Publish events asynchronously
    err = eventSys.Publish(event.Event{
        Type:   "order.created",
        Source: "order-service",
        Data: map[string]interface{}{
            "orderID": "12345",
            "amount":  99.99,
        },
    })
    if err != nil {
        panic(err)
    }

    // Wait for async processing
    time.Sleep(100 * time.Millisecond)
}
```

## Architecture

The event package follows the **Ports & Adapters (Hexagonal Architecture)** pattern:

- **Port**: `EventSystem` interface defines the core pub/sub contract
- **Adapter**: `InMemoryEventSystem` is a simple in-memory implementation for MVP/single-instance deployments

### Future Adapters

For distributed systems, additional adapters can be implemented:

- **Kafka**: High-throughput, persistent event streaming
- **NATS**: Low-latency, cloud-native messaging
- **Redis Pub/Sub**: Simple, fast in-memory messaging
- **RabbitMQ**: Feature-rich, enterprise messaging

All adapters implement the same `EventSystem` interface, allowing zero-code changes when switching implementations.

## API Reference

### Event

The `Event` struct represents an event in the system:

```go
type Event struct {
    ID        string                 // Auto-generated if empty
    Type      string                 // Event type (e.g., "order.created")
    Source    string                 // Where the event originated
    Timestamp time.Time              // Auto-set if zero
    Data      interface{}            // Event payload
    Metadata  map[string]interface{} // Additional metadata
}
```

### EventSystem Interface

```go
type EventSystem interface {
    // Subscribe to events of a specific type
    // Use "*" to subscribe to all events
    Subscribe(eventType string, handler EventHandler) (string, error)

    // Unsubscribe removes a subscription by ID
    Unsubscribe(subscriptionID string) error

    // Publish sends an event asynchronously to all matching subscribers
    Publish(event Event) error

    // PublishSync sends an event synchronously, blocking until all handlers complete
    PublishSync(ctx context.Context, event Event) error

    // Close shuts down the event system
    Close() error
}
```

### EventHandler

Event handlers are simple functions that process events:

```go
type EventHandler func(event Event) error
```

**Important**: Handlers should be idempotent, as events may be delivered multiple times in distributed scenarios.

## Usage Examples

### Wildcard Subscription

Subscribe to all events regardless of type:

```go
handler := func(e event.Event) error {
    log.Printf("Received any event: %s", e.Type)
    return nil
}

subID, _ := eventSys.Subscribe("*", handler)
```

### Synchronous Publishing

Wait for all handlers to complete before continuing:

```go
ctx := context.Background()
err := eventSys.PublishSync(ctx, event.Event{
    Type:   "payment.processed",
    Source: "payment-service",
    Data:   paymentData,
})
if err != nil {
    log.Printf("Handler errors: %v", err)
}
```

### Multiple Subscribers

Multiple handlers can subscribe to the same event type:

```go
// Logger handler
logHandler := func(e event.Event) error {
    log.Printf("Event logged: %s", e.Type)
    return nil
}
eventSys.Subscribe("order.created", logHandler)

// Metrics handler
metricsHandler := func(e event.Event) error {
    metrics.Inc("orders_created")
    return nil
}
eventSys.Subscribe("order.created", metricsHandler)

// Notification handler
notifyHandler := func(e event.Event) error {
    sendEmail(e.Data)
    return nil
}
eventSys.Subscribe("order.created", notifyHandler)
```

All three handlers will be called concurrently when "order.created" is published.

### Event Metadata

Use metadata for cross-cutting concerns:

```go
eventSys.Publish(event.Event{
    Type:   "user.registered",
    Source: "auth-service",
    Data:   userData,
    Metadata: map[string]interface{}{
        "priority":      "high",
        "correlationID": requestID,
        "retry_count":   0,
    },
})
```

### Error Handling

Handlers that return errors are logged but don't stop other handlers:

```go
handler := func(e event.Event) error {
    if err := processEvent(e); err != nil {
        // Error will be logged, other handlers still execute
        return fmt.Errorf("failed to process: %w", err)
    }
    return nil
}
```

### Graceful Shutdown

Always close the event system when done:

```go
eventSys := event.NewInMemoryEventSystem()
defer eventSys.Close() // Processes remaining events before shutdown
```

## Concurrency & Thread Safety

The event system is fully concurrent-safe:

- **Subscribe/Unsubscribe**: Can be called concurrently from multiple goroutines
- **Publish**: Can be called concurrently, events are queued
- **Handler Execution**: Handlers run concurrently in separate goroutines
- **Panic Recovery**: Handler panics are recovered and don't affect other handlers
- **Timeout Protection**: Handlers that run too long (>30s) are logged and timed out

## Testing

Run tests with coverage:

```bash
cd core/event
go test -cover ./...
```

Run with race detector:

```bash
go test -race ./...
```

## Performance Characteristics

**InMemoryEventSystem**:
- Event publishing: O(1) - events are queued in a buffered channel
- Event delivery: O(n) where n = number of matching subscribers
- Subscribe/Unsubscribe: O(1) with mutex-protected map
- Memory: O(s) where s = number of subscriptions

**Throughput**: Suitable for thousands of events per second in single-instance deployments.

**Buffer Size**: Default 100 events. If the buffer fills, `Publish()` returns an error.

## Integration with Jazz3

When used within Jazz3 workflows:

- **Event Triggers**: Transitions can wait for specific events before firing
- **Webhook Integration**: External webhooks publish events that trigger workflow execution
- **State Changes**: Workflow state changes can publish events for external systems
- **Error Events**: Errors and compensations can publish events for monitoring

## Design Philosophy

1. **Simple by Default**: In-memory implementation for MVP and single-instance use cases
2. **Extensible by Design**: Port/adapter pattern allows sophisticated implementations later
3. **Fail-Safe**: Handler panics and errors don't crash the system
4. **Observable**: All events flow through a single system for easy monitoring
5. **Idiomatic Go**: Uses channels, goroutines, and context for natural concurrency

## License

MIT License - Part of the Jazz3 workflow orchestration engine.

## Contributing

This package is part of the Jazz3 project. See the main repository for contribution guidelines.
