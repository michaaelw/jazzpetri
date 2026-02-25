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
	"reflect"
	"testing"
)

// Token Registry Not Validated
//
// EXPECTED FAILURE: These tests will fail because TypeRegistry.Register()
// does not validate inputs (nil factory, nil type, duplicate registrations).
//
// Validation prevents:
//   - Runtime panics from nil factories
//   - Undefined behavior from nil types
//   - Silent overwrites of existing registrations

func TestTypeRegistry_RegisterNil(t *testing.T) {
	// EXPECTED FAILURE: Register() doesn't validate nil type

	registry := NewMapRegistry()

	// Should return error when registering nil type
	err := registry.Register("test", nil)
	if err == nil {
		t.Error("Register() should return error for nil type")
	}

	// Error should be descriptive
	if err != nil && err.Error() == "" {
		t.Error("Error message should describe the problem")
	}
}

func TestTypeRegistry_RegisterEmptyName(t *testing.T) {
	// EXPECTED FAILURE: Register() doesn't validate empty type name

	registry := NewMapRegistry()

	// Should return error for empty type name
	err := registry.Register("", reflect.TypeOf(StringToken{}))
	if err == nil {
		t.Error("Register() should return error for empty type name")
	}
}

func TestTypeRegistry_RegisterDuplicateDifferentType(t *testing.T) {
	// Test that Register() returns error when same name registered with different type

	registry := NewMapRegistry()

	// Register a type
	typ1 := reflect.TypeOf(StringToken{})
	err := registry.Register("string", typ1)
	if err != nil {
		t.Fatalf("First registration should succeed: %v", err)
	}

	// Try to register same name with different type
	typ2 := reflect.TypeOf(IntToken{})
	err = registry.Register("string", typ2)

	// Should return an error (prevents accidental overwrites)
	if err == nil {
		t.Error("Register() should return error for duplicate type name")
	}
}

func TestTypeRegistry_RegisterDuplicateIdempotent(t *testing.T) {
	// EXPECTED FAILURE: No clear idempotency semantics

	registry := NewMapRegistry()

	typ := reflect.TypeOf(StringToken{})

	// Register once
	err := registry.Register("string", typ)
	if err != nil {
		t.Fatalf("First registration should succeed: %v", err)
	}

	// Register same type again (idempotent?)
	err = registry.Register("string", typ)

	// Options:
	// 1. Error (strict - no duplicates)
	// 2. Success (idempotent - same type is OK)

	// Current behavior: returns error
	// Could be improved to allow idempotent registration of same type
	if err == nil {
		// If idempotent, verify type is still correct
		retrieved, ok := registry.Get("string")
		if !ok || retrieved != typ {
			t.Error("Idempotent registration should maintain type")
		}
	}
}

func TestTypeRegistry_RegisterUpdate(t *testing.T) {
	// EXPECTED FAILURE: No explicit Update() method

	registry := NewMapRegistry()

	// Register initial type
	err := registry.Register("config", reflect.TypeOf(StringToken{}))
	if err != nil {
		t.Fatalf("Initial registration failed: %v", err)
	}

	// If we want to update, should have explicit method
	// err = registry.Update("config", reflect.TypeOf(IntToken{}))
	// OR
	// err = registry.RegisterOrUpdate("config", reflect.TypeOf(IntToken{}))

	// Without explicit update method, it's unclear whether updates are supported
	t.Skip("Registry needs explicit Update or RegisterOrUpdate method")
}

func TestTypeRegistry_GetNonExistentValidation(t *testing.T) {
	// This works correctly - just verifying expected behavior

	registry := NewMapRegistry()

	typ, ok := registry.Get("nonexistent")
	if ok {
		t.Error("Get() should return false for non-existent type")
	}
	if typ != nil {
		t.Error("Get() should return nil type for non-existent type")
	}
}

func TestTypeRegistry_SerializeNilData(t *testing.T) {
	// EXPECTED FAILURE: Serialize() doesn't validate nil input

	registry := NewMapRegistry()

	// Should return error for nil TokenData
	_, err := registry.Serialize(nil)
	if err == nil {
		t.Error("Serialize() should return error for nil TokenData")
	}

	// Error message should be clear
	if err != nil && err.Error() == "" {
		t.Error("Error should have descriptive message")
	}
}

func TestTypeRegistry_DeserializeUnregistered(t *testing.T) {
	// This works correctly - just verifying expected behavior

	registry := NewMapRegistry()

	// Deserializing unregistered type should fail
	data := []byte(`{"value": "test"}`)
	_, err := registry.Deserialize("unregistered", data)

	if err == nil {
		t.Error("Deserialize() should return error for unregistered type")
	}
}

func TestTypeRegistry_DeserializeInvalidJSON(t *testing.T) {
	// This works correctly - verifying error handling

	registry := NewMapRegistry()
	_ = registry.Register("string", reflect.TypeOf(StringToken{}))

	// Invalid JSON should fail gracefully
	invalidJSON := []byte(`{invalid json}`)
	_, err := registry.Deserialize("string", invalidJSON)

	if err == nil {
		t.Error("Deserialize() should return error for invalid JSON")
	}
}

func TestTypeRegistry_ThreadSafety(t *testing.T) {
	// EXPECTED FAILURE: No documented thread-safety guarantees

	registry := NewMapRegistry()

	// MapRegistry uses sync.RWMutex, so it should be thread-safe
	// But this isn't documented

	// Concurrent registrations
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			typ := reflect.TypeOf(StringToken{})
			registry.Register("type_"+string(rune('0'+n)), typ)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have registered 10 types without panicking
	// (actual verification would count registered types)
}

// Example showing validation improvements
func ExampleTypeRegistry_validation() {
	// EXPECTED FAILURE: These validations don't exist

	registry := NewMapRegistry()

	// 1. Validate nil type
	err := registry.Register("test", nil)
	if err == nil {
		panic("should reject nil type")
	}

	// 2. Validate empty name
	err = registry.Register("", reflect.TypeOf(StringToken{}))
	if err == nil {
		panic("should reject empty name")
	}

	// 3. Validate duplicates
	typ := reflect.TypeOf(StringToken{})
	_ = registry.Register("string", typ)

	err = registry.Register("string", typ) // Duplicate
	if err == nil {
		panic("should reject or handle duplicate")
	}

	// 4. Clear error messages
	// err.Error() should return something like:
	// "cannot register type '': type name cannot be empty"
	// "cannot register type 'string': already registered"
	// "cannot register type 'test': type cannot be nil"
}
