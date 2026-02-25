# State Package

[![Go Reference](https://pkg.go.dev/badge/jazz3/core/state.svg)](https://pkg.go.dev/jazz3/core/state)

The `state` package provides comprehensive state management and persistence capabilities for the Jazz3 workflow engine. It implements **markings** (state snapshots), **event logging**, and **state restoration** for Petri nets, enabling workflow persistence, recovery, debugging, and audit trails.

## Installation

```bash
go get jazz3/core/state
```

## Module Status

This package is a standalone Go module with the following dependencies:
- `jazz3/core/clock` - Time abstraction
- `jazz3/core/context` - Execution context
- `jazz3/core/petri` - Petri net model
- `jazz3/core/token` - Token system

## Overview

The state package provides three main capabilities:

### 1. Markings (Snapshots)
In Petri net terminology, a **marking** is the distribution of tokens across places at a specific point in time. A marking represents the complete state of a workflow, capturing:

- Which places contain tokens
- How many tokens are in each place
- The token data and metadata
- The timestamp when the snapshot was taken

Markings are:
- **Non-destructive**: Capturing a marking does not remove tokens from places
- **Thread-safe**: Multiple markings can be captured concurrently
- **Immutable**: Markings are read-only snapshots
- **Cloneable**: Deep copies can be created for safe modification
- **Serializable**: JSON serialization for persistence

### 2. Event Log
The event log records all workflow actions (token additions, transition firings, etc.) with timestamps and context. This provides:

- **Complete Audit Trail**: Every workflow action is recorded
- **Time-Travel Debugging**: Replay workflows from any point in time
- **Recovery**: Restore workflow state after crashes
- **Hybrid Architecture**: Periodic snapshots + event replay for optimal performance

### 3. State Restoration
Restore workflow state from snapshots and event logs:

- **Snapshot Restoration**: Restore marking to places
- **Event Replay**: Replay events from a checkpoint
- **Crash Recovery**: Resume workflows after failures
- **Testing**: Create deterministic test scenarios

## Key Concepts

### Marking

A `Marking` represents a point-in-time snapshot of workflow state:

```go
type Marking struct {
    Timestamp time.Time              // When snapshot was taken
    Places    map[string]PlaceState  // State of each place
}
```

### PlaceState

A `PlaceState` captures the state of a single place:

```go
type PlaceState struct {
    PlaceID  string         // Place identifier
    Tokens   []*token.Token // Tokens in this place
    Capacity int            // Place capacity
}
```

## Usage

### Basic Marking Capture

```go
// Create and resolve a Petri net
net := petri.NewPetriNet("workflow", "My Workflow")
// ... add places, transitions, arcs ...
net.Resolve()

// Add tokens to places
place.AddToken(token)

// Capture current state
clk := clock.NewRealTimeClock()
marking, err := state.NewMarking(net, clk)
if err != nil {
    log.Fatal(err)
}

// Inspect state
fmt.Printf("Total tokens: %d\n", marking.TokenCount())
fmt.Printf("Tokens in place 'p1': %d\n", marking.PlaceTokenCount("p1"))
```

### Engine Integration

The `Engine` provides a convenient method to capture workflow state:

```go
// Create engine
engine := engine.NewEngine(net, ctx, config)
engine.Start()

// Capture state at any time
marking, err := engine.GetMarking()
if err != nil {
    log.Fatal(err)
}

// Inspect current workflow state
for placeID, placeState := range marking.Places {
    if len(placeState.Tokens) > 0 {
        fmt.Printf("Place %s has %d tokens\n", placeID, len(placeState.Tokens))
    }
}
```

### Comparing States

```go
// Capture before operation
before, _ := state.NewMarking(net, clk)

// Perform workflow operation
transition.Fire(ctx)

// Capture after operation
after, _ := state.NewMarking(net, clk)

// Compare states
if before.Equal(after) {
    fmt.Println("No state change")
} else {
    fmt.Println("State changed")
    // Inspect differences...
}
```

### State Inspection for Debugging

```go
// Capture current state
marking, _ := engine.GetMarking()

// Find where tokens are stuck
for placeID, placeState := range marking.Places {
    if len(placeState.Tokens) > 0 {
        fmt.Printf("Found %d token(s) in place '%s'\n",
            len(placeState.Tokens), placeID)

        // Inspect individual tokens
        for _, tok := range placeState.Tokens {
            fmt.Printf("  Token ID: %s, Type: %s\n",
                tok.ID, tok.Data.Type())
        }
    }
}
```

### Cloning Markings

```go
// Create original marking
original, _ := state.NewMarking(net, clk)

// Create independent copy
clone := original.Clone()

// Modify clone without affecting original
cloneState := clone.Places["p1"]
cloneState.Capacity = 999
// original.Places["p1"].Capacity is unchanged
```

## API Reference

### Marking Creation

```go
// NewMarking creates a marking from the current state of a Petri net
func NewMarking(net *petri.PetriNet, clk clock.Clock) (*Marking, error)
```

**Requirements:**
- The Petri net must be resolved (`net.Resolve()` called)
- The clock is used for timestamp generation

**Returns:**
- A new `Marking` instance with complete workflow state
- An error if the net is not resolved

### Marking Methods

#### TokenCount

```go
// Returns total number of tokens across all places
func (m *Marking) TokenCount() int
```

#### PlaceTokenCount

```go
// Returns number of tokens in a specific place
// Returns 0 if place doesn't exist
func (m *Marking) PlaceTokenCount(placeID string) int
```

#### GetTokens

```go
// Returns tokens in a specific place
// Returns nil if place doesn't exist
func (m *Marking) GetTokens(placeID string) []*token.Token
```

#### IsEmpty

```go
// Returns true if no tokens in any place
func (m *Marking) IsEmpty() bool
```

#### Clone

```go
// Creates a deep copy of the marking
func (m *Marking) Clone() *Marking
```

#### Equal

```go
// Returns true if two markings are equivalent
// Compares places, capacities, and token IDs
func (m *Marking) Equal(other *Marking) bool
```

### Engine Integration

```go
// GetMarking captures the current workflow state
func (e *Engine) GetMarking() (*state.Marking, error)
```

## Implementation Details

### Non-Destructive Token Inspection

Marking capture uses `Place.PeekTokens()` to inspect tokens without removing them:

```go
// PeekTokens returns a copy of all tokens without removing them
func (p *Place) PeekTokens() []*token.Token
```

This is implemented as a drain-and-refill operation on the token channel, protected by a mutex to ensure thread safety during concurrent marking captures.

### Thread Safety

- **PeekTokens**: Uses a mutex to serialize concurrent peeks
- **NewMarking**: Safe to call concurrently from multiple goroutines
- **Marking instances**: Immutable after creation (read-only)
- **Clone**: Creates independent copies for safe modification

### Performance Considerations

- Marking capture is O(n) where n is the total number of tokens
- Each `PeekTokens()` operation drains and refills the channel
- Concurrent peeks are serialized per place
- Large markings (many places/tokens) may take time to capture

## Use Cases

### 1. Workflow State Inspection

Monitor workflow progress by capturing markings at key points:

```go
// Capture at workflow milestones
milestones := make(map[string]*state.Marking)

milestones["start"] = engine.GetMarking()
// ... workflow execution ...
milestones["validation"] = engine.GetMarking()
// ... workflow execution ...
milestones["completion"] = engine.GetMarking()

// Compare milestones
fmt.Printf("Tokens processed: %d -> %d\n",
    milestones["start"].TokenCount(),
    milestones["completion"].TokenCount())
```

### 2. Debugging Stuck Workflows

Identify where tokens are blocked:

```go
marking, _ := engine.GetMarking()

// Find places with tokens
for placeID, placeState := range marking.Places {
    if len(placeState.Tokens) > 0 {
        // Check if transitions are enabled
        // Inspect token data
        // Identify blocking conditions
    }
}
```

### 3. Testing and Validation

Assert workflow state in tests:

```go
// Arrange
net := createTestWorkflow()
// ... setup ...

// Act
transition.Fire(ctx)

// Assert
marking, _ := state.NewMarking(net, clk)
assert.Equal(t, 1, marking.PlaceTokenCount("output"))
assert.Equal(t, 0, marking.PlaceTokenCount("input"))
```

### 4. State Persistence (Future)

Markings will be serialized for workflow persistence:

```go
// Capture state
marking, _ := engine.GetMarking()

// Serialize (future milestone)
data, _ := json.Marshal(marking)

// Save to database/file
db.SaveWorkflowState(workflowID, data)
```

## Future Enhancements

The state package will be extended in future milestones:

### Milestone 1.4: Event Log
- Event recording for workflow actions
- Event replay for recovery
- Hybrid snapshots + event log architecture

### Milestone 1.5: Serialization
- JSON serialization of markings
- Type registry integration for token data
- Deserialization and state restoration

### Milestone 1.6: Recovery
- Restore workflow from marking
- Replay events from last snapshot
- Crash recovery and continuation

## Examples

See `marking_example_test.go` for complete runnable examples:

- Workflow state inspection
- Debugging workflow progress
- Comparing states before/after operations
- Engine integration patterns

## Testing

The package includes comprehensive tests:

- **Unit tests** (`marking_test.go`): Core marking functionality
- **Integration tests** (`core/engine/marking_integration_test.go`): Engine integration
- **Example tests** (`marking_example_test.go`): Usage patterns

Run tests:

```bash
# Unit tests
go test ./core/state

# Integration tests
go test ./core/engine -run ".*Marking.*"

# All tests with race detector
go test ./core/... -race

# Coverage
go test ./core/state -cover
```

## Design Decisions

### Decision 1: Non-Destructive Capture

**Choice**: Markings capture state without removing tokens

**Rationale**:
- Allows state inspection during execution
- Multiple captures don't affect workflow
- Debugging doesn't interfere with production

**Alternative**: Destructive capture (rejected - would stop workflow)

### Decision 2: Channel-Based Token Storage

**Choice**: Use drain-and-refill with mutex for PeekTokens

**Rationale**:
- Maintains existing channel-based Place implementation
- Provides FIFO ordering guarantees
- Thread-safe with minimal changes

**Alternative**: Switch to slice + mutex (rejected - larger refactor)

### Decision 3: Immutable Markings

**Choice**: Markings are read-only snapshots

**Rationale**:
- Prevents accidental state modification
- Safe to pass between goroutines
- Clone() available for intentional modification

**Alternative**: Mutable markings (rejected - error-prone)

### Decision 4: Timestamp Capture

**Choice**: Use Clock abstraction for timestamps

**Rationale**:
- Consistent with engine's time abstraction
- Enables virtual time in tests
- Supports deterministic testing

**Alternative**: time.Now() directly (rejected - not testable)

## Related Packages

- **core/petri**: Petri net model (places, transitions, arcs)
- **core/engine**: Workflow execution engine
- **core/token**: Token system and type registry
- **core/clock**: Time abstraction
- **core/context**: Execution context

## References

- [Petri Net Theory](https://en.wikipedia.org/wiki/Petri_net)
- [Marking (Petri nets)](https://en.wikipedia.org/wiki/Petri_net#Marking)
- [Workflow Patterns](http://www.workflowpatterns.com/)
