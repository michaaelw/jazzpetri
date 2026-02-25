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
	"reflect"
	"testing"
)

// TestTypeRegistry_RegisterAndGet tests type registration and retrieval
func TestTypeRegistry_RegisterAndGet(t *testing.T) {
	registry := NewMapRegistry()

	// Register IntToken
	intType := reflect.TypeOf(IntToken{})
	err := registry.Register("IntToken", intType)
	if err != nil {
		t.Fatalf("Failed to register IntToken: %v", err)
	}

	// Retrieve IntToken
	retrievedType, ok := registry.Get("IntToken")
	if !ok {
		t.Fatal("Failed to retrieve registered IntToken")
	}

	if retrievedType != intType {
		t.Errorf("Retrieved type %v, want %v", retrievedType, intType)
	}
}

// TestTypeRegistry_RegisterDuplicate tests duplicate registration error
func TestTypeRegistry_RegisterDuplicate(t *testing.T) {
	registry := NewMapRegistry()

	intType := reflect.TypeOf(IntToken{})

	// First registration should succeed
	err := registry.Register("IntToken", intType)
	if err != nil {
		t.Fatalf("First registration failed: %v", err)
	}

	// Duplicate registration should fail
	err = registry.Register("IntToken", intType)
	if err == nil {
		t.Error("Expected error for duplicate registration, got nil")
	}
}

// TestTypeRegistry_GetNonExistent tests retrieving non-existent type
func TestTypeRegistry_GetNonExistent(t *testing.T) {
	registry := NewMapRegistry()

	_, ok := registry.Get("NonExistentType")
	if ok {
		t.Error("Expected false for non-existent type, got true")
	}
}

// TestTypeRegistry_Serialize tests JSON serialization of token data
func TestTypeRegistry_Serialize(t *testing.T) {
	registry := NewMapRegistry()
	registry.Register("IntToken", reflect.TypeOf(IntToken{}))

	tests := []struct {
		name    string
		data    TokenData
		wantErr bool
	}{
		{
			name:    "Serialize IntToken",
			data:    &IntToken{Value: 42},
			wantErr: false,
		},
		{
			name:    "Serialize StringToken",
			data:    &StringToken{Value: "test"},
			wantErr: false,
		},
		{
			name:    "Serialize MapToken",
			data:    &MapToken{Value: map[string]interface{}{"key": "value"}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := registry.Serialize(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Serialize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(data) == 0 {
				t.Error("Serialize() returned empty data")
			}

			// Verify it's valid JSON
			if !tt.wantErr && !json.Valid(data) {
				t.Error("Serialize() did not return valid JSON")
			}
		})
	}
}

// TestTypeRegistry_Deserialize tests JSON deserialization to token data
func TestTypeRegistry_Deserialize(t *testing.T) {
	registry := NewMapRegistry()
	registry.Register("IntToken", reflect.TypeOf(IntToken{}))
	registry.Register("StringToken", reflect.TypeOf(StringToken{}))
	registry.Register("MapToken", reflect.TypeOf(MapToken{}))

	tests := []struct {
		name     string
		typeName string
		jsonData string
		want     TokenData
		wantErr  bool
	}{
		{
			name:     "Deserialize IntToken",
			typeName: "IntToken",
			jsonData: `{"Value": 42}`,
			want:     &IntToken{Value: 42},
			wantErr:  false,
		},
		{
			name:     "Deserialize StringToken",
			typeName: "StringToken",
			jsonData: `{"Value": "hello"}`,
			want:     &StringToken{Value: "hello"},
			wantErr:  false,
		},
		{
			name:     "Deserialize MapToken",
			typeName: "MapToken",
			jsonData: `{"Value": {"count": 10}}`,
			want:     &MapToken{Value: map[string]interface{}{"count": float64(10)}}, // JSON numbers are float64
			wantErr:  false,
		},
		{
			name:     "Deserialize unregistered type",
			typeName: "UnknownToken",
			jsonData: `{"Value": 1}`,
			want:     nil,
			wantErr:  true,
		},
		{
			name:     "Deserialize invalid JSON",
			typeName: "IntToken",
			jsonData: `{invalid json}`,
			want:     nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := registry.Deserialize(tt.typeName, []byte(tt.jsonData))
			if (err != nil) != tt.wantErr {
				t.Errorf("Deserialize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Deserialize() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestTypeRegistry_SerializeDeserializeRoundTrip tests round-trip serialization
func TestTypeRegistry_SerializeDeserializeRoundTrip(t *testing.T) {
	registry := NewMapRegistry()
	registry.Register("IntToken", reflect.TypeOf(IntToken{}))
	registry.Register("StringToken", reflect.TypeOf(StringToken{}))
	registry.Register("MapToken", reflect.TypeOf(MapToken{}))

	tests := []struct {
		name string
		data TokenData
	}{
		{
			name: "IntToken round-trip",
			data: &IntToken{Value: 12345},
		},
		{
			name: "StringToken round-trip",
			data: &StringToken{Value: "round-trip-test"},
		},
		{
			name: "MapToken round-trip",
			data: &MapToken{Value: map[string]interface{}{
				"string": "value",
				"number": float64(42),
				"bool":   true,
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Serialize
			serialized, err := registry.Serialize(tt.data)
			if err != nil {
				t.Fatalf("Serialize() error = %v", err)
			}

			// Deserialize
			deserialized, err := registry.Deserialize(tt.data.Type(), serialized)
			if err != nil {
				t.Fatalf("Deserialize() error = %v", err)
			}

			// Compare
			if !reflect.DeepEqual(deserialized, tt.data) {
				t.Errorf("Round-trip failed: got %v, want %v", deserialized, tt.data)
			}
		})
	}
}

// TestTypeRegistry_ConcurrentAccess tests thread-safety
func TestTypeRegistry_ConcurrentAccess(t *testing.T) {
	registry := NewMapRegistry()

	// Register in parallel
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(index int) {
			registry.Register("IntToken", reflect.TypeOf(IntToken{}))
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify registration succeeded
	_, ok := registry.Get("IntToken")
	if !ok {
		t.Error("Expected IntToken to be registered")
	}
}
