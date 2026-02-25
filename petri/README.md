# Petri Package

**The first production-grade Colored Petri Net library for Go.**

The `petri` package provides a robust, formally-verified Petri net implementation with:
- **Colored tokens**: Strongly-typed data flow through the workflow
- **AND-join synchronization**: True parallel workflow coordination with formal guarantees
- **Guards**: Conditional transition firing based on token state
- **Thread-safe execution**: Built on Go channels for safe concurrent operation
- **Hybrid graph model**: Serializable structure with efficient runtime execution

This package can be used standalone or as part of the Jazz3 workflow engine.

## Installation

```bash
# As a standalone module
go get jazz3/core/petri

# Dependencies (automatically installed)
go get jazz3/core/token
go get jazz3/core/context
go get jazz3/core/task
go get jazz3/core/event
go get jazz3/core/clock
```

## Quick Start

```go
package main

import (
    "fmt"
    "jazz3/core/petri"
    "jazz3/core/token"
    execContext "jazz3/core/context"
)

func main() {
    // Create a simple workflow: input -> process -> output
    net := petri.NewPetriNet("workflow", "Simple Workflow")

    // Define places
    input := petri.NewPlace("input", "Input Queue", 10)
    output := petri.NewPlace("output", "Output Queue", 10)

    // Define transition with guard
    process := petri.NewTransition("process", "Process Work")
    process.WithGuard(func(tokens []*token.Token) bool {
        // Only process if we have tokens
        return len(tokens) > 0
    })

    // Connect with arcs
    inputArc := petri.NewArc("arc1", "input", "process", 1)
    outputArc := petri.NewArc("arc2", "process", "output", 1)

    // Build the net
    net.AddPlace(input)
    net.AddPlace(output)
    net.AddTransition(process)
    net.AddArc(inputArc)
    net.AddArc(outputArc)

    // Validate and resolve
    if err := net.Validate(); err != nil {
        panic(err)
    }
    if err := net.Resolve(); err != nil {
        panic(err)
    }

    // Add a token
    tok := &token.Token{
        ID:   "work-1",
        Data: token.NewStringToken("hello"),
    }
    input.AddToken(tok)

    // Fire the transition
    ctx := execContext.NewExecutionContext()
    if process.IsEnabled() {
        err := process.Fire(ctx)
        if err != nil {
            panic(err)
        }
    }

    fmt.Printf("Tokens in output: %d\n", output.TokenCount())
}
```

## Why Petri Nets?

Petri nets provide **formal guarantees** about workflow behavior that are impossible with ad-hoc state machines:

- **Deadlock detection**: Mathematically prove workflows won't get stuck
- **Liveness verification**: Ensure all paths can complete
- **Reachability analysis**: Determine what states are possible
- **AND-join correctness**: Guarantee parallel branches synchronize properly

## Overview

A Petri net is a mathematical modeling language for describing distributed systems. In Jazz3, Petri nets model workflow execution with:

- **Places**: Hold tokens (data)
- **Transitions**: Consume and produce tokens (process steps)
- **Arcs**: Connect places to transitions with weights and expressions
- **Tokens**: Carry data through the workflow

## Architecture

### Hybrid Graph Structure

The package uses a hybrid approach for graph representation:

- **ID References** (strings): Stored for serialization and deserialization
- **Direct Pointers**: Created by `Resolve()` for efficient runtime execution
- **Lazy Resolution**: `PetriNet.Resolve()` converts IDs to pointers after loading

This design provides:
- Serializable graph structure (JSON, database, etc.)
- Fast execution without ID lookups
- Clear separation of concerns

### Thread Safety

- Places use Go channels for thread-safe token storage
- FIFO ordering guaranteed for token operations
- Non-blocking operations with capacity management
- Race detector verified

## Core Types

### Place

Holds tokens in the Petri net. Places have a capacity and maintain FIFO ordering.

```go
place := petri.NewPlace("p1", "Input Queue", 100)

// Add tokens (non-blocking if at capacity)
token := &token.Token{
    ID:   "tok1",
    Data: myData,
}
err := place.AddToken(token)

// Remove tokens (non-blocking if empty)
tok, err := place.RemoveToken()

// Check token count
count := place.TokenCount()
```

### Transition

Consumes tokens from input places and produces tokens to output places. Transitions can have guards for conditional firing.

```go
trans := petri.NewTransition("t1", "Process")

// Add guard for conditional firing
trans.WithGuard(func(tokens []*token.Token) bool {
    // Check if tokens meet condition
    return condition
})

// Check if transition is enabled
if trans.IsEnabled() {
    // Transition can fire
}
```

### Arc

Connects places to transitions or transitions to places. Arcs have weights and optional expressions for token transformation.

```go
// Simple arc with weight
arc := petri.NewArc("a1", "p1", "t1", 2)

// Arc with expression for token transformation
arc.WithExpression(func(input *token.Token) *token.Token {
    // Transform token
    return transformedToken
})
```

### PetriNet

Container for the complete Petri net graph with validation and resolution.

```go
net := petri.NewPetriNet("net1", "Order Processing")

// Add components
net.AddPlace(place)
net.AddTransition(transition)
net.AddArc(arc)

// Validate structure
if err := net.Validate(); err != nil {
    log.Fatal(err)
}

// Resolve ID references to pointers
if err := net.Resolve(); err != nil {
    log.Fatal(err)
}
```

## Workflow Routing Patterns

Jazz3's Petri net model naturally supports all fundamental workflow routing patterns. Understanding these patterns is essential for designing effective workflows.

### Quick Reference

| Pattern | Implementation | Use Case |
|---------|----------------|----------|
| **AND-split** | Multiple output arcs from transition | Create parallel flows |
| **AND-join** | Multiple input arcs to transition | Synchronize parallel flows |
| **OR-split** | Guards on transitions | Exclusive choice routing |
| **OR-join** | Multiple transitions to same place | Merge alternative paths |

### Example: Parallel Processing

```go
// AND-split: Create parallel branches
split := petri.NewTransition("split", "Split Work")
net.AddArc(petri.NewArc("a1", "input", "split", 1))
net.AddArc(petri.NewArc("a2", "split", "worker_1", 1))  // Branch 1
net.AddArc(petri.NewArc("a3", "split", "worker_2", 1))  // Branch 2

// AND-join: Synchronize results
join := petri.NewTransition("join", "Join Results")
net.AddArc(petri.NewArc("a4", "worker_1", "join", 1))  // Wait for 1
net.AddArc(petri.NewArc("a5", "worker_2", "join", 1))  // Wait for 2
net.AddArc(petri.NewArc("a6", "join", "output", 1))
```

### Example: Conditional Routing

```go
// OR-split: Route based on condition
highPriority := petri.NewTransition("high", "High Priority Path")
highPriority.WithGuard(func(tokens []*token.Token) bool {
    return checkPriority(tokens) > 5
})

lowPriority := petri.NewTransition("low", "Low Priority Path")
lowPriority.WithGuard(func(tokens []*token.Token) bool {
    return checkPriority(tokens) <= 5
})

// Both read from same place (only one will fire)
net.AddArc(petri.NewArc("a1", "decision", "high", 1))
net.AddArc(petri.NewArc("a2", "decision", "low", 1))
```

**For comprehensive coverage of routing patterns**, see:
- [ROUTING.md](ROUTING.md) - Detailed routing patterns guide with best practices
- [Example 07](../../examples/07-routing-patterns/) - Complete working example


## Complete Example

This example shows building a simple workflow: Input -> Process -> Output

```go
package main

import (
    "fmt"
    "jazz3/core/petri"
    "jazz3/core/token"
)

// WorkItem represents a work item token
type WorkItem struct {
    ID     string
    Status string
}

func (w *WorkItem) Type() string {
    return "work_item"
}

func main() {
    // Create Petri net
    net := petri.NewPetriNet("workflow1", "Simple Workflow")

    // Create places
    input := petri.NewPlace("p1", "Input", 10)
    output := petri.NewPlace("p2", "Output", 10)

    // Create transition with guard
    process := petri.NewTransition("t1", "Process")
    process.WithGuard(func(tokens []*token.Token) bool {
        // Only process tokens with "pending" status
        for _, tok := range tokens {
            if item, ok := tok.Data.(*WorkItem); ok {
                if item.Status != "pending" {
                    return false
                }
            }
        }
        return true
    })

    // Create arcs with expressions
    inputArc := petri.NewArc("a1", "p1", "t1", 1)

    outputArc := petri.NewArc("a2", "t1", "p2", 1)
    outputArc.WithExpression(func(tok *token.Token) *token.Token {
        // Transform token: mark as processed
        if item, ok := tok.Data.(*WorkItem); ok {
            return &token.Token{
                ID: tok.ID,
                Data: &WorkItem{
                    ID:     item.ID,
                    Status: "processed",
                },
            }
        }
        return tok
    })

    // Set up ID references for serialization
    input.OutgoingArcIDs = []string{"a1"}
    process.InputArcIDs = []string{"a1"}
    process.OutputArcIDs = []string{"a2"}
    output.IncomingArcIDs = []string{"a2"}

    // Add to net
    net.AddPlace(input)
    net.AddPlace(output)
    net.AddTransition(process)
    net.AddArc(inputArc)
    net.AddArc(outputArc)

    // Validate
    if err := net.Validate(); err != nil {
        panic(err)
    }

    // Resolve (convert IDs to pointers)
    if err := net.Resolve(); err != nil {
        panic(err)
    }

    // Add work item to input
    workItem := &token.Token{
        ID: "item1",
        Data: &WorkItem{
            ID:     "WI-001",
            Status: "pending",
        },
    }
    input.AddToken(workItem)

    // Check if transition is enabled
    if process.IsEnabled() {
        fmt.Println("Transition is enabled and ready to fire")
    }

    fmt.Printf("Input tokens: %d\n", input.TokenCount())
    fmt.Printf("Output tokens: %d\n", output.TokenCount())
}
```

## Guards and Expressions

### Guard Functions

Guards enable conditional transition firing based on input tokens:

```go
// Only fire if all tokens have matching IDs
trans.WithGuard(func(tokens []*token.Token) bool {
    if len(tokens) < 2 {
        return false
    }
    firstID := tokens[0].ID
    for _, tok := range tokens[1:] {
        if tok.ID != firstID {
            return false
        }
    }
    return true
})
```

### Arc Expressions

Arc expressions transform tokens as they traverse arcs:

```go
// Increment a counter
arc.WithExpression(func(tok *token.Token) *token.Token {
    if data, ok := tok.Data.(*CounterData); ok {
        return &token.Token{
            ID: tok.ID,
            Data: &CounterData{Value: data.Value + 1},
        }
    }
    return tok
})

// Filter and transform
arc.WithExpression(func(tok *token.Token) *token.Token {
    if data, ok := tok.Data.(*MessageData); ok {
        // Transform message
        return &token.Token{
            ID:   tok.ID + "_transformed",
            Data: &MessageData{Content: strings.ToUpper(data.Content)},
        }
    }
    return tok
})
```

## Serialization Pattern

The hybrid structure supports serialization:

```go
// Save net (uses ID references)
data, err := json.Marshal(net)

// Load net
var loadedNet petri.PetriNet
json.Unmarshal(data, &loadedNet)

// Resolve ID references to pointers before execution
loadedNet.Resolve()
```

## Testing

The package includes comprehensive tests:

```bash
# Run tests
go test ./core/petri

# Run with coverage
go test -cover ./core/petri

# Run with race detector
go test -race ./core/petri
```

Coverage: 87.8% of statements

## Design Decisions

1. **Channel-based Places**: Go channels provide thread-safe FIFO token storage
2. **Hybrid Graph**: ID references for serialization, pointers for execution
3. **Lazy Resolution**: Explicit `Resolve()` step after deserialization
4. **Guard Token Restoration**: Guards temporarily remove tokens for evaluation, then restore them
5. **Capacity Limits**: Places have configurable capacity (default 100)
6. **Non-blocking Operations**: AddToken/RemoveToken return errors instead of blocking

## Integration

The petri package integrates with:

- **core/token**: Token system for data flow
- **core/context** (future): Execution context during firing
- **core/executor** (future): Transition execution engine

## Future Extensions

- Colored Petri nets (typed tokens)
- Timed transitions
- Hierarchical nets (subnets)
- Inhibitor arcs
- Reset arcs
- Priority transitions

## References

- [Petri Nets - Wikipedia](https://en.wikipedia.org/wiki/Petri_net)
- Jazz3 Discovery Document: `/docs/architecture/DISCOVERY-SUMMARY.md`
