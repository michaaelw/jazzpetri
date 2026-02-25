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

package token_test

import (
	"fmt"

	"github.com/jazzpetri/engine/token"
)

// Example demonstrates basic token creation and type checking
func Example() {
	// Create tokens with different data types
	intToken := token.Token{
		ID:   "token-1",
		Data: &token.IntToken{Value: 42},
	}

	stringToken := token.Token{
		ID:   "token-2",
		Data: &token.StringToken{Value: "hello"},
	}

	mapToken := token.Token{
		ID:   "token-3",
		Data: &token.MapToken{Value: map[string]interface{}{"count": 10}},
	}

	fmt.Println(intToken.Data.Type())
	fmt.Println(stringToken.Data.Type())
	fmt.Println(mapToken.Data.Type())

	// Output:
	// IntToken
	// StringToken
	// MapToken
}

// ExampleTypeRegistry demonstrates token serialization and deserialization
func ExampleTypeRegistry() {
	// Create a registry and register built-in types
	registry := token.NewMapRegistry()
	token.RegisterBuiltinTypes(registry)

	// Create a token
	original := &token.IntToken{Value: 42}

	// Serialize
	serialized, err := registry.Serialize(original)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Serialized: %s\n", string(serialized))

	// Deserialize
	deserialized, err := registry.Deserialize("IntToken", serialized)
	if err != nil {
		panic(err)
	}

	// Verify
	intToken := deserialized.(*token.IntToken)
	fmt.Printf("Deserialized value: %d\n", intToken.Value)

	// Output:
	// Serialized: {"Value":42}
	// Deserialized value: 42
}

// ExampleMapToken demonstrates using MapToken for flexible data
func ExampleMapToken() {
	// Create a MapToken with mixed types
	data := &token.MapToken{
		Value: map[string]interface{}{
			"name":   "John Doe",
			"age":    30,
			"active": true,
		},
	}

	// Access values
	fmt.Printf("Type: %s\n", data.Type())
	fmt.Printf("Name: %s\n", data.Value["name"])
	fmt.Printf("Age: %d\n", data.Value["age"])
	fmt.Printf("Active: %t\n", data.Value["active"])

	// Output:
	// Type: MapToken
	// Name: John Doe
	// Age: 30
	// Active: true
}
