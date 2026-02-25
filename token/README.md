# Token Package

The `token` package provides the core token system for the Jazz3 workflow engine. Tokens are the fundamental data carriers in a Petri net, flowing through places and being transformed by transitions.

## Installation

This package can be used standalone or as part of the Jazz3 workflow engine.

```bash
go get github.com/jazz3/jazz3/core/token
```

## Overview

This package implements **Decision 2** from the Jazz3 architectural discovery: **Strongly-Typed Tokens with Interface + Type Registry**. This design provides:

- **Type Safety**: All tokens implement the `TokenData` interface
- **Flexibility**: Custom token types can be registered at runtime
- **Serialization**: JSON-based serialization for state persistence
- **Thread Safety**: Registry operations are thread-safe

## Core Components

### TokenData Interface

All token types must implement this interface:

```go
type TokenData interface {
    Type() string  // Returns type identifier for serialization
}
```

### Token Struct

The `Token` struct wraps token data with metadata:

```go
type Token struct {
    ID   string      // Unique identifier
    Data TokenData   // Actual payload
}
```

### TypeRegistry

The `TypeRegistry` interface manages token type registration and serialization:

```go
type TypeRegistry interface {
    Register(typeName string, typ reflect.Type) error
    Get(typeName string) (reflect.Type, bool)
    Serialize(data TokenData) ([]byte, error)
    Deserialize(typeName string, data []byte) (TokenData, error)
}
```

**Implementation**: `MapRegistry` provides a thread-safe, map-based implementation.

## Built-in Token Types

### IntToken

Holds integer values:

```go
token := &IntToken{Value: 42}
```

### StringToken

Holds string values:

```go
token := &StringToken{Value: "hello world"}
```

### MapToken

Holds flexible key-value data (map[string]interface{}):

```go
token := &MapToken{
    Value: map[string]interface{}{
        "name": "John",
        "age": 30,
        "active": true,
    },
}
```

## Standalone Usage

This package has no dependencies outside the Go standard library and can be used independently of Jazz3.

```go
package main

import (
    "fmt"
    "github.com/jazz3/jazz3/core/token"
)

func main() {
    // Create a registry
    registry := token.NewMapRegistry()
    token.RegisterBuiltinTypes(registry)

    // Create and serialize a token
    data := &token.MapToken{
        Value: map[string]interface{}{
            "user": "alice",
            "score": 100,
        },
    }

    serialized, _ := registry.Serialize(data)
    fmt.Printf("Serialized: %s\n", string(serialized))

    // Deserialize
    recovered, _ := registry.Deserialize("MapToken", serialized)
    mapData := recovered.(*token.MapToken)
    fmt.Printf("User: %s, Score: %d\n", mapData.Value["user"], mapData.Value["score"])
}
```

## Usage Examples

### Basic Token Creation

```go
import "github.com/jazz3/jazz3/core/token"

// Create a token with integer data
intToken := token.Token{
    ID: "token-1",
    Data: &token.IntToken{Value: 42},
}

// Access token data
fmt.Println(intToken.Data.Type())  // "IntToken"
```

### Type Registry and Serialization

```go
// Create registry and register built-in types
registry := token.NewMapRegistry()
token.RegisterBuiltinTypes(registry)

// Create and serialize token data
original := &token.IntToken{Value: 42}
serialized, err := registry.Serialize(original)

// Deserialize back to token data
deserialized, err := registry.Deserialize("IntToken", serialized)
intToken := deserialized.(*token.IntToken)
fmt.Println(intToken.Value)  // 42
```

### Custom Token Types

You can define custom token types by implementing the `TokenData` interface:

```go
type CustomToken struct {
    Name  string
    Count int
}

func (c *CustomToken) Type() string {
    return "CustomToken"
}

// Register the custom type
registry.Register("CustomToken", reflect.TypeOf(CustomToken{}))

// Now you can serialize/deserialize custom tokens
custom := &CustomToken{Name: "test", Count: 10}
data, _ := registry.Serialize(custom)
recovered, _ := registry.Deserialize("CustomToken", data)
```

## Design Decisions

### Why Pointer-Based Tokens?

Token types (IntToken, StringToken, etc.) are typically used as pointers (`*IntToken`) because:

1. **Interface Implementation**: Methods are defined on pointer receivers
2. **Mutability**: Allows modification of token data during workflow execution
3. **Efficiency**: Avoids copying large data structures

### Why JSON Serialization?

JSON was chosen for serialization because:

1. **Human Readable**: Easy to debug and inspect
2. **Language Agnostic**: Can be consumed by non-Go systems
3. **Standard Library**: No external dependencies
4. **Flexible**: Supports nested structures and mixed types

Trade-offs:
- **Performance**: Binary formats (protobuf, msgpack) would be faster
- **Size**: JSON is more verbose than binary formats
- **Type Safety**: JSON numbers are float64, requiring type assertions

For production use cases requiring higher performance, the `TypeRegistry` interface can be implemented with alternative serialization formats.

### Thread Safety

The `MapRegistry` uses `sync.RWMutex` to ensure thread-safe operations:

- **Register**: Write lock (exclusive)
- **Get**: Read lock (shared)
- **Serialize/Deserialize**: Read lock for registry lookups

This allows concurrent reads while preventing race conditions during registration.

## Testing

The package includes comprehensive tests:

- **Unit Tests**: Test individual components in isolation
- **Integration Tests**: Test serialization round-trips
- **Concurrency Tests**: Verify thread-safety
- **Example Tests**: Demonstrate usage patterns

Run tests:

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run with race detector
go test -race ./...

# Run examples
go test -run Example ./...
```

Current coverage: **85.6%**

## Performance Considerations

### Registry Lookups

Type registry lookups are O(1) map operations, making serialization/deserialization efficient even with many registered types.

### Memory Usage

- **Token**: ~24 bytes (ID string + pointer)
- **IntToken**: ~8 bytes
- **StringToken**: ~16 bytes + string data
- **MapToken**: ~8 bytes + map overhead (~48 bytes) + data

### Serialization Performance

JSON serialization performance (approximate):

- **IntToken**: ~100 ns/op
- **StringToken**: ~150 ns/op (depends on length)
- **MapToken**: ~500 ns/op (depends on complexity)

For high-throughput workflows, consider:

1. Reusing token instances (object pooling)
2. Batch serialization operations
3. Using binary formats for large tokens

## Future Enhancements

Potential improvements not in the current scope:

1. **Binary Serialization**: Add support for protobuf/msgpack
2. **Schema Validation**: Validate token structure against schemas
3. **Type Coercion**: Automatic conversion between compatible types
4. **Compression**: Compress large token data automatically
5. **Versioning**: Handle schema evolution for long-running workflows

## Integration with Jazz3

The token system integrates with other Jazz3 components:

- **Places**: Store tokens in channels
- **Transitions**: Consume and produce tokens
- **Arcs**: Transform tokens using arc functions
- **State Persistence**: Serialize tokens for snapshots and event logs
- **Hierarchy**: Pass tokens across subnet boundaries

See the main Jazz3 documentation for complete workflow examples.
