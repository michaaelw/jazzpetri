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
)

// IntToken holds an integer value.
// This is a built-in token type for carrying simple integer data.
type IntToken struct {
	Value int
}

// Type returns the type identifier for IntToken.
func (i *IntToken) Type() string {
	return "IntToken"
}

// StringToken holds a string value.
// This is a built-in token type for carrying text data.
type StringToken struct {
	Value string
}

// Type returns the type identifier for StringToken.
func (s *StringToken) Type() string {
	return "StringToken"
}

// MapToken holds flexible key-value data.
// This is a built-in token type for carrying structured data with string keys
// and values of any type. It's useful for passing complex data structures
// through the workflow without defining custom types.
type MapToken struct {
	Value map[string]interface{}
}

// Type returns the type identifier for MapToken.
func (m *MapToken) Type() string {
	return "MapToken"
}

// RegisterBuiltinTypes registers all built-in token types with the given registry.
// This function should be called during initialization to make built-in types
// available for serialization and deserialization.
//
// Returns an error if any type is already registered.
func RegisterBuiltinTypes(registry TypeRegistry) error {
	types := map[string]reflect.Type{
		"IntToken":    reflect.TypeOf(IntToken{}),
		"StringToken": reflect.TypeOf(StringToken{}),
		"MapToken":    reflect.TypeOf(MapToken{}),
	}

	for name, typ := range types {
		if err := registry.Register(name, typ); err != nil {
			return err
		}
	}

	return nil
}
