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

package petri

import (
	"testing"
)

// TestMetadata tests the SetMetadata and GetMetadata methods.
// Workflow metadata support
func TestMetadata(t *testing.T) {
	net := NewPetriNet("net1", "Test Net")

	// Test setting and getting string metadata
	net.SetMetadata("author", "John Doe")
	value, ok := net.GetMetadata("author")
	if !ok {
		t.Error("Expected metadata key 'author' to exist")
	}
	if value != "John Doe" {
		t.Errorf("Expected author='John Doe', got %v", value)
	}

	// Test setting and getting numeric metadata
	net.SetMetadata("version", 1)
	value, ok = net.GetMetadata("version")
	if !ok {
		t.Error("Expected metadata key 'version' to exist")
	}
	if value != 1 {
		t.Errorf("Expected version=1, got %v", value)
	}

	// Test setting and getting complex metadata
	metadata := map[string]string{
		"department": "Engineering",
		"team":       "Platform",
	}
	net.SetMetadata("organization", metadata)
	value, ok = net.GetMetadata("organization")
	if !ok {
		t.Error("Expected metadata key 'organization' to exist")
	}
	orgMap, ok := value.(map[string]string)
	if !ok {
		t.Error("Expected organization metadata to be map[string]string")
	}
	if orgMap["department"] != "Engineering" {
		t.Errorf("Expected department='Engineering', got %s", orgMap["department"])
	}

	// Test getting non-existent key
	_, ok = net.GetMetadata("nonexistent")
	if ok {
		t.Error("Expected nonexistent key to return false")
	}

	// Test overwriting existing key
	net.SetMetadata("author", "Jane Smith")
	value, ok = net.GetMetadata("author")
	if !ok {
		t.Error("Expected metadata key 'author' to exist")
	}
	if value != "Jane Smith" {
		t.Errorf("Expected author='Jane Smith', got %v", value)
	}
}

// TestMetadataInitialization tests that Metadata is properly initialized.
// Workflow metadata support
func TestMetadataInitialization(t *testing.T) {
	net := NewPetriNet("net1", "Test Net")

	if net.Metadata == nil {
		t.Error("Expected Metadata map to be initialized")
	}

	// Should be able to set metadata on new net
	net.SetMetadata("test", "value")
	value, ok := net.GetMetadata("test")
	if !ok {
		t.Error("Expected to be able to set metadata on new net")
	}
	if value != "value" {
		t.Errorf("Expected test='value', got %v", value)
	}
}

// TestMetadataWithNilMap tests that SetMetadata/GetMetadata handle nil Metadata map.
// Workflow metadata support
func TestMetadataWithNilMap(t *testing.T) {
	net := &PetriNet{
		ID:   "net1",
		Name: "Test Net",
		// Metadata is nil
	}

	// SetMetadata should initialize the map
	net.SetMetadata("key", "value")
	if net.Metadata == nil {
		t.Error("Expected SetMetadata to initialize Metadata map")
	}

	value, ok := net.GetMetadata("key")
	if !ok {
		t.Error("Expected key to exist after SetMetadata")
	}
	if value != "value" {
		t.Errorf("Expected key='value', got %v", value)
	}

	// GetMetadata on nil map should return false
	net2 := &PetriNet{
		ID:   "net2",
		Name: "Test Net 2",
		// Metadata is nil
	}
	_, ok = net2.GetMetadata("key")
	if ok {
		t.Error("Expected GetMetadata to return false for nil Metadata map")
	}
}
