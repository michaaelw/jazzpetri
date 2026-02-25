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

// TestIntToken_Type tests IntToken type identifier
func TestIntToken_Type(t *testing.T) {
	token := &IntToken{Value: 42}
	if got := token.Type(); got != "IntToken" {
		t.Errorf("IntToken.Type() = %v, want IntToken", got)
	}
}

// TestIntToken_Values tests IntToken with different values
func TestIntToken_Values(t *testing.T) {
	tests := []struct {
		name  string
		value int
	}{
		{"Positive value", 42},
		{"Zero value", 0},
		{"Negative value", -100},
		{"Large value", 999999999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &IntToken{Value: tt.value}
			if token.Value != tt.value {
				t.Errorf("IntToken.Value = %v, want %v", token.Value, tt.value)
			}
		})
	}
}

// TestStringToken_Type tests StringToken type identifier
func TestStringToken_Type(t *testing.T) {
	token := &StringToken{Value: "test"}
	if got := token.Type(); got != "StringToken" {
		t.Errorf("StringToken.Type() = %v, want StringToken", got)
	}
}

// TestStringToken_Values tests StringToken with different values
func TestStringToken_Values(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"Simple string", "hello"},
		{"Empty string", ""},
		{"Unicode string", "你好世界"},
		{"String with special chars", "Hello\nWorld\t!"},
		{"Long string", string(make([]byte, 10000))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &StringToken{Value: tt.value}
			if token.Value != tt.value {
				t.Errorf("StringToken.Value length = %v, want %v", len(token.Value), len(tt.value))
			}
		})
	}
}

// TestMapToken_Type tests MapToken type identifier
func TestMapToken_Type(t *testing.T) {
	token := &MapToken{Value: map[string]interface{}{}}
	if got := token.Type(); got != "MapToken" {
		t.Errorf("MapToken.Type() = %v, want MapToken", got)
	}
}

// TestMapToken_Values tests MapToken with different values
func TestMapToken_Values(t *testing.T) {
	tests := []struct {
		name  string
		value map[string]interface{}
	}{
		{
			name:  "Empty map",
			value: map[string]interface{}{},
		},
		{
			name: "Simple map",
			value: map[string]interface{}{
				"key": "value",
			},
		},
		{
			name: "Mixed types map",
			value: map[string]interface{}{
				"string":  "value",
				"int":     42,
				"float":   3.14,
				"bool":    true,
				"nil":     nil,
			},
		},
		{
			name: "Nested map",
			value: map[string]interface{}{
				"outer": map[string]interface{}{
					"inner": "value",
				},
			},
		},
		{
			name: "Map with array",
			value: map[string]interface{}{
				"array": []interface{}{1, 2, 3},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &MapToken{Value: tt.value}
			if !reflect.DeepEqual(token.Value, tt.value) {
				t.Errorf("MapToken.Value = %v, want %v", token.Value, tt.value)
			}
		})
	}
}

// TestMapToken_NilValue tests MapToken with nil value
func TestMapToken_NilValue(t *testing.T) {
	token := &MapToken{Value: nil}
	if token.Value != nil {
		t.Errorf("Expected nil value, got %v", token.Value)
	}
}

// TestRegisterBuiltinTypes tests registration of all built-in types
func TestRegisterBuiltinTypes(t *testing.T) {
	registry := NewMapRegistry()

	err := RegisterBuiltinTypes(registry)
	if err != nil {
		t.Fatalf("RegisterBuiltinTypes() error = %v", err)
	}

	// Verify all built-in types are registered
	expectedTypes := []string{"IntToken", "StringToken", "MapToken"}
	for _, typeName := range expectedTypes {
		t.Run("Check "+typeName, func(t *testing.T) {
			_, ok := registry.Get(typeName)
			if !ok {
				t.Errorf("Built-in type %s not registered", typeName)
			}
		})
	}
}

// TestRegisterBuiltinTypes_Idempotent tests that registering twice doesn't cause issues
func TestRegisterBuiltinTypes_Idempotent(t *testing.T) {
	registry := NewMapRegistry()

	// First registration
	err := RegisterBuiltinTypes(registry)
	if err != nil {
		t.Fatalf("First RegisterBuiltinTypes() error = %v", err)
	}

	// Second registration should fail (duplicate)
	err = RegisterBuiltinTypes(registry)
	if err == nil {
		t.Error("Expected error on duplicate registration, got nil")
	}
}

// TestBuiltinTypes_SerializationCompatibility tests that built-in types work with registry
func TestBuiltinTypes_SerializationCompatibility(t *testing.T) {
	registry := NewMapRegistry()
	RegisterBuiltinTypes(registry)

	tests := []struct {
		name string
		data TokenData
	}{
		{
			name: "IntToken serialization",
			data: &IntToken{Value: 42},
		},
		{
			name: "StringToken serialization",
			data: &StringToken{Value: "test"},
		},
		{
			name: "MapToken serialization",
			data: &MapToken{Value: map[string]interface{}{"key": "value"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test serialization
			serialized, err := registry.Serialize(tt.data)
			if err != nil {
				t.Errorf("Serialize() error = %v", err)
				return
			}

			// Test deserialization
			deserialized, err := registry.Deserialize(tt.data.Type(), serialized)
			if err != nil {
				t.Errorf("Deserialize() error = %v", err)
				return
			}

			// Verify round-trip
			if !reflect.DeepEqual(deserialized, tt.data) {
				t.Errorf("Round-trip failed: got %v, want %v", deserialized, tt.data)
			}
		})
	}
}
