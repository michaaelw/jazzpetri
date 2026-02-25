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

package token

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
)

// TypeRegistry manages token type registration and serialization.
// It provides methods to register custom token types and serialize/deserialize
// token data using JSON encoding.
type TypeRegistry interface {
	// Register associates a type name with a reflect.Type.
	// Returns an error if the type name is already registered.
	Register(typeName string, typ reflect.Type) error

	// Get retrieves the reflect.Type for a registered type name.
	// Returns false if the type name is not registered.
	Get(typeName string) (reflect.Type, bool)

	// Serialize converts TokenData to JSON bytes.
	// The type information is not included in the serialized data;
	// it must be tracked separately (e.g., in a Token wrapper).
	Serialize(data TokenData) ([]byte, error)

	// Deserialize converts JSON bytes back to TokenData.
	// The typeName must be registered in the registry.
	// Returns an error if the type is not registered or deserialization fails.
	Deserialize(typeName string, data []byte) (TokenData, error)
}

// MapRegistry is a thread-safe implementation of TypeRegistry
// using a map for type storage.
type MapRegistry struct {
	mu    sync.RWMutex
	types map[string]reflect.Type
}

// NewMapRegistry creates a new MapRegistry instance.
func NewMapRegistry() *MapRegistry {
	return &MapRegistry{
		types: make(map[string]reflect.Type),
	}
}

// Register associates a type name with a reflect.Type.
// Added validation for nil type and empty type name.
// Returns an error if:
//   - typeName is empty
//   - typ is nil
//   - type name is already registered
func (r *MapRegistry) Register(typeName string, typ reflect.Type) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Validate type name is not empty
	if typeName == "" {
		return fmt.Errorf("cannot register type '': type name cannot be empty")
	}

	// Validate type is not nil
	if typ == nil {
		return fmt.Errorf("cannot register type '%s': type cannot be nil", typeName)
	}

	// Check for duplicate registration
	if _, exists := r.types[typeName]; exists {
		return fmt.Errorf("cannot register type '%s': already registered", typeName)
	}

	r.types[typeName] = typ
	return nil
}

// Get retrieves the reflect.Type for a registered type name.
// Returns false if the type name is not registered.
func (r *MapRegistry) Get(typeName string) (reflect.Type, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	typ, ok := r.types[typeName]
	return typ, ok
}

// Serialize converts TokenData to JSON bytes.
// Uses json.Marshal for encoding, which works with exported struct fields.
func (r *MapRegistry) Serialize(data TokenData) ([]byte, error) {
	if data == nil {
		return nil, fmt.Errorf("cannot serialize nil TokenData")
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize %s: %w", data.Type(), err)
	}

	return bytes, nil
}

// Deserialize converts JSON bytes back to TokenData.
// Creates a new instance of the registered type and unmarshals the JSON into it.
// Returns an error if the type is not registered or deserialization fails.
func (r *MapRegistry) Deserialize(typeName string, data []byte) (TokenData, error) {
	typ, ok := r.Get(typeName)
	if !ok {
		return nil, fmt.Errorf("type %s is not registered", typeName)
	}

	// Create a pointer to a new instance of the type
	instance := reflect.New(typ).Interface()

	// Unmarshal JSON into the instance
	if err := json.Unmarshal(data, instance); err != nil {
		return nil, fmt.Errorf("failed to deserialize %s: %w", typeName, err)
	}

	// Type assert to TokenData
	tokenData, ok := instance.(TokenData)
	if !ok {
		return nil, fmt.Errorf("type %s does not implement TokenData", typeName)
	}

	return tokenData, nil
}
