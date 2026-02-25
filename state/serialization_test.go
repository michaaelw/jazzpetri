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

package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jazzpetri/engine/clock"
	"github.com/jazzpetri/engine/token"
)

// TestSerializeEmptyMarking tests serializing an empty marking
func TestSerializeEmptyMarking(t *testing.T) {
	registry := token.NewMapRegistry()
	err := token.RegisterBuiltinTypes(registry)
	if err != nil {
		t.Fatalf("Failed to register builtin types: %v", err)
	}

	marking := &Marking{
		Timestamp: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		Places:    make(map[string]PlaceState),
	}

	data, err := marking.Serialize(registry)
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("Expected non-empty serialized data")
	}

	// Verify it's valid JSON
	var sm SerializableMarking
	if err := json.Unmarshal(data, &sm); err != nil {
		t.Errorf("Serialized data is not valid JSON: %v", err)
	}
}

// TestSerializeMarkingWithTokens tests serializing a marking with various token types
func TestSerializeMarkingWithTokens(t *testing.T) {
	registry := token.NewMapRegistry()
	err := token.RegisterBuiltinTypes(registry)
	if err != nil {
		t.Fatalf("Failed to register builtin types: %v", err)
	}

	marking := &Marking{
		Timestamp: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		Places: map[string]PlaceState{
			"place-1": {
				PlaceID:  "place-1",
				Capacity: 100,
				Tokens: []*token.Token{
					{
						ID:   "token-1",
						Data: &token.IntToken{Value: 42},
					},
					{
						ID:   "token-2",
						Data: &token.StringToken{Value: "hello"},
					},
				},
			},
			"place-2": {
				PlaceID:  "place-2",
				Capacity: 50,
				Tokens: []*token.Token{
					{
						ID: "token-3",
						Data: &token.MapToken{
							Value: map[string]interface{}{
								"name": "Alice",
								"age":  float64(30),
							},
						},
					},
				},
			},
		},
	}

	data, err := marking.Serialize(registry)
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	// Verify JSON structure
	var sm SerializableMarking
	if err := json.Unmarshal(data, &sm); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify places
	if len(sm.Places) != 2 {
		t.Errorf("Expected 2 places, got %d", len(sm.Places))
	}

	// Verify place-1
	place1, exists := sm.Places["place-1"]
	if !exists {
		t.Error("place-1 not found in serialized data")
	}
	if len(place1.Tokens) != 2 {
		t.Errorf("Expected 2 tokens in place-1, got %d", len(place1.Tokens))
	}
	if place1.Capacity != 100 {
		t.Errorf("Expected capacity 100, got %d", place1.Capacity)
	}

	// Verify token types are preserved
	if place1.Tokens[0].Type != "IntToken" {
		t.Errorf("Expected IntToken, got %s", place1.Tokens[0].Type)
	}
	if place1.Tokens[1].Type != "StringToken" {
		t.Errorf("Expected StringToken, got %s", place1.Tokens[1].Type)
	}
}

// TestDeserializeMarking tests deserializing a marking
func TestDeserializeMarking(t *testing.T) {
	registry := token.NewMapRegistry()
	err := token.RegisterBuiltinTypes(registry)
	if err != nil {
		t.Fatalf("Failed to register builtin types: %v", err)
	}

	jsonData := []byte(`{
		"timestamp": "2025-01-15T10:30:00Z",
		"places": {
			"place-1": {
				"place_id": "place-1",
				"capacity": 100,
				"tokens": [
					{
						"id": "token-1",
						"type": "IntToken",
						"data": {"Value": 42}
					}
				]
			}
		}
	}`)

	marking, err := DeserializeMarking(jsonData, registry)
	if err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	if len(marking.Places) != 1 {
		t.Errorf("Expected 1 place, got %d", len(marking.Places))
	}

	place, exists := marking.Places["place-1"]
	if !exists {
		t.Fatal("place-1 not found")
	}

	if len(place.Tokens) != 1 {
		t.Fatalf("Expected 1 token, got %d", len(place.Tokens))
	}

	intToken, ok := place.Tokens[0].Data.(*token.IntToken)
	if !ok {
		t.Fatalf("Expected IntToken, got %T", place.Tokens[0].Data)
	}

	if intToken.Value != 42 {
		t.Errorf("Expected value 42, got %d", intToken.Value)
	}
}

// TestRoundTripSerialization tests serialize -> deserialize maintains data
func TestRoundTripSerialization(t *testing.T) {
	registry := token.NewMapRegistry()
	err := token.RegisterBuiltinTypes(registry)
	if err != nil {
		t.Fatalf("Failed to register builtin types: %v", err)
	}

	original := &Marking{
		Timestamp: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		Places: map[string]PlaceState{
			"place-1": {
				PlaceID:  "place-1",
				Capacity: 100,
				Tokens: []*token.Token{
					{ID: "token-1", Data: &token.IntToken{Value: 42}},
					{ID: "token-2", Data: &token.StringToken{Value: "test"}},
				},
			},
			"place-2": {
				PlaceID:  "place-2",
				Capacity: 50,
				Tokens: []*token.Token{
					{
						ID: "token-3",
						Data: &token.MapToken{
							Value: map[string]interface{}{
								"key": "value",
							},
						},
					},
				},
			},
		},
	}

	// Serialize
	data, err := original.Serialize(registry)
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	// Deserialize
	restored, err := DeserializeMarking(data, registry)
	if err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	// Verify structure
	if !original.Equal(restored) {
		t.Error("Round-trip serialization changed the marking structure")
	}

	// Verify timestamps
	if !original.Timestamp.Equal(restored.Timestamp) {
		t.Errorf("Timestamp mismatch: %v != %v", original.Timestamp, restored.Timestamp)
	}

	// Verify token data
	place1 := restored.Places["place-1"]
	intToken := place1.Tokens[0].Data.(*token.IntToken)
	if intToken.Value != 42 {
		t.Errorf("IntToken value mismatch: expected 42, got %d", intToken.Value)
	}

	stringToken := place1.Tokens[1].Data.(*token.StringToken)
	if stringToken.Value != "test" {
		t.Errorf("StringToken value mismatch: expected 'test', got '%s'", stringToken.Value)
	}
}

// TestSerializeNilMarking tests error handling for nil marking
func TestSerializeNilMarking(t *testing.T) {
	registry := token.NewMapRegistry()
	err := token.RegisterBuiltinTypes(registry)
	if err != nil {
		t.Fatalf("Failed to register builtin types: %v", err)
	}

	var marking *Marking
	_, err = marking.Serialize(registry)
	if err == nil {
		t.Error("Expected error for nil marking")
	}
}

// TestSerializeNilRegistry tests error handling for nil registry
func TestSerializeNilRegistry(t *testing.T) {
	marking := &Marking{
		Timestamp: time.Now(),
		Places:    make(map[string]PlaceState),
	}

	_, err := marking.Serialize(nil)
	if err == nil {
		t.Error("Expected error for nil registry")
	}
}

// TestDeserializeEmptyData tests error handling for empty data
func TestDeserializeEmptyData(t *testing.T) {
	registry := token.NewMapRegistry()
	err := token.RegisterBuiltinTypes(registry)
	if err != nil {
		t.Fatalf("Failed to register builtin types: %v", err)
	}

	_, err = DeserializeMarking([]byte{}, registry)
	if err == nil {
		t.Error("Expected error for empty data")
	}
}

// TestDeserializeInvalidJSON tests error handling for invalid JSON
func TestDeserializeInvalidJSON(t *testing.T) {
	registry := token.NewMapRegistry()
	err := token.RegisterBuiltinTypes(registry)
	if err != nil {
		t.Fatalf("Failed to register builtin types: %v", err)
	}

	_, err = DeserializeMarking([]byte("not json"), registry)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

// TestDeserializeUnknownTokenType tests error handling for unknown token types
func TestDeserializeUnknownTokenType(t *testing.T) {
	registry := token.NewMapRegistry()
	// Don't register builtin types

	jsonData := []byte(`{
		"timestamp": "2025-01-15T10:30:00Z",
		"places": {
			"place-1": {
				"place_id": "place-1",
				"capacity": 100,
				"tokens": [
					{
						"id": "token-1",
						"type": "UnknownToken",
						"data": {}
					}
				]
			}
		}
	}`)

	_, err := DeserializeMarking(jsonData, registry)
	if err == nil {
		t.Error("Expected error for unknown token type")
	}
}

// TestSaveMarkingToFile tests saving a marking to a file
func TestSaveMarkingToFile(t *testing.T) {
	registry := token.NewMapRegistry()
	err := token.RegisterBuiltinTypes(registry)
	if err != nil {
		t.Fatalf("Failed to register builtin types: %v", err)
	}

	marking := &Marking{
		Timestamp: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		Places: map[string]PlaceState{
			"place-1": {
				PlaceID:  "place-1",
				Capacity: 100,
				Tokens: []*token.Token{
					{ID: "token-1", Data: &token.IntToken{Value: 42}},
				},
			},
		},
	}

	// Create temp file
	tmpDir := t.TempDir()
	filepath := filepath.Join(tmpDir, "marking.json")

	err = SaveMarkingToFile(marking, filepath, registry)
	if err != nil {
		t.Fatalf("SaveMarkingToFile failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		t.Error("File was not created")
	}

	// Verify file contents
	data, err := os.ReadFile(filepath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	var sm SerializableMarking
	if err := json.Unmarshal(data, &sm); err != nil {
		t.Errorf("File does not contain valid JSON: %v", err)
	}
}

// TestLoadMarkingFromFile tests loading a marking from a file
func TestLoadMarkingFromFile(t *testing.T) {
	registry := token.NewMapRegistry()
	err := token.RegisterBuiltinTypes(registry)
	if err != nil {
		t.Fatalf("Failed to register builtin types: %v", err)
	}

	jsonData := []byte(`{
		"timestamp": "2025-01-15T10:30:00Z",
		"places": {
			"place-1": {
				"place_id": "place-1",
				"capacity": 100,
				"tokens": [
					{
						"id": "token-1",
						"type": "IntToken",
						"data": {"Value": 99}
					}
				]
			}
		}
	}`)

	// Create temp file
	tmpDir := t.TempDir()
	filepath := filepath.Join(tmpDir, "marking.json")
	err = os.WriteFile(filepath, jsonData, 0644)
	if err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	// Load from file
	marking, err := LoadMarkingFromFile(filepath, registry)
	if err != nil {
		t.Fatalf("LoadMarkingFromFile failed: %v", err)
	}

	// Verify data
	if len(marking.Places) != 1 {
		t.Errorf("Expected 1 place, got %d", len(marking.Places))
	}

	place := marking.Places["place-1"]
	intToken := place.Tokens[0].Data.(*token.IntToken)
	if intToken.Value != 99 {
		t.Errorf("Expected value 99, got %d", intToken.Value)
	}
}

// TestSaveAndLoadRoundTrip tests save -> load maintains data
func TestSaveAndLoadRoundTrip(t *testing.T) {
	registry := token.NewMapRegistry()
	err := token.RegisterBuiltinTypes(registry)
	if err != nil {
		t.Fatalf("Failed to register builtin types: %v", err)
	}

	original := &Marking{
		Timestamp: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		Places: map[string]PlaceState{
			"place-1": {
				PlaceID:  "place-1",
				Capacity: 100,
				Tokens: []*token.Token{
					{ID: "token-1", Data: &token.IntToken{Value: 42}},
					{ID: "token-2", Data: &token.StringToken{Value: "test"}},
				},
			},
		},
	}

	// Save to file
	tmpDir := t.TempDir()
	filepath := filepath.Join(tmpDir, "marking.json")
	err = SaveMarkingToFile(original, filepath, registry)
	if err != nil {
		t.Fatalf("SaveMarkingToFile failed: %v", err)
	}

	// Load from file
	restored, err := LoadMarkingFromFile(filepath, registry)
	if err != nil {
		t.Fatalf("LoadMarkingFromFile failed: %v", err)
	}

	// Verify equality
	if !original.Equal(restored) {
		t.Error("Save/Load round-trip changed the marking")
	}
}

// TestSerializeEventLog tests serializing event logs
func TestSerializeEventLog(t *testing.T) {
	clk := clock.NewVirtualClock(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC))
	log := NewMemoryEventLog()

	// Add events
	event1 := NewEvent(EventTransitionFired, "net-1", clk)
	event1.TransitionID = "t1"
	log.Append(event1)

	event2 := NewEvent(EventTokenProduced, "net-1", clk)
	event2.PlaceID = "p1"
	event2.TokenID = "token-1"
	log.Append(event2)

	// Serialize
	data, err := SerializeEventLog(log)
	if err != nil {
		t.Fatalf("SerializeEventLog failed: %v", err)
	}

	// Verify JSON
	var events []*Event
	if err := json.Unmarshal(data, &events); err != nil {
		t.Errorf("Serialized data is not valid JSON: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("Expected 2 events, got %d", len(events))
	}
}

// TestDeserializeEventLog tests deserializing event logs
func TestDeserializeEventLog(t *testing.T) {
	jsonData := []byte(`[
		{
			"ID": "event-1",
			"Type": "transition.fired",
			"Timestamp": "2025-01-15T10:00:00Z",
			"NetID": "net-1",
			"TransitionID": "t1",
			"PlaceID": "",
			"TokenID": "",
			"Error": "",
			"Metadata": {}
		}
	]`)

	events, err := DeserializeEventLog(jsonData)
	if err != nil {
		t.Fatalf("DeserializeEventLog failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	if events[0].ID != "event-1" {
		t.Errorf("Expected ID 'event-1', got '%s'", events[0].ID)
	}

	if events[0].TransitionID != "t1" {
		t.Errorf("Expected TransitionID 't1', got '%s'", events[0].TransitionID)
	}
}

// TestSaveEventLogToFile tests saving event log to file
func TestSaveEventLogToFile(t *testing.T) {
	clk := clock.NewVirtualClock(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC))
	log := NewMemoryEventLog()

	event := NewEvent(EventTransitionFired, "net-1", clk)
	log.Append(event)

	tmpDir := t.TempDir()
	filepath := filepath.Join(tmpDir, "events.json")

	err := SaveEventLogToFile(log, filepath)
	if err != nil {
		t.Fatalf("SaveEventLogToFile failed: %v", err)
	}

	// Verify file exists and contains valid JSON
	data, err := os.ReadFile(filepath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	var events []*Event
	if err := json.Unmarshal(data, &events); err != nil {
		t.Errorf("File does not contain valid JSON: %v", err)
	}
}

// TestLoadEventLogFromFile tests loading event log from file
func TestLoadEventLogFromFile(t *testing.T) {
	jsonData := []byte(`[
		{
			"ID": "event-1",
			"Type": "transition.fired",
			"Timestamp": "2025-01-15T10:00:00Z",
			"NetID": "net-1",
			"TransitionID": "t1",
			"PlaceID": "",
			"TokenID": "",
			"Error": "",
			"Metadata": {}
		}
	]`)

	tmpDir := t.TempDir()
	filepath := filepath.Join(tmpDir, "events.json")
	err := os.WriteFile(filepath, jsonData, 0644)
	if err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	log, err := LoadEventLogFromFile(filepath)
	if err != nil {
		t.Fatalf("LoadEventLogFromFile failed: %v", err)
	}

	if log.Count() != 1 {
		t.Errorf("Expected 1 event in log, got %d", log.Count())
	}

	event, err := log.Get("event-1")
	if err != nil {
		t.Fatalf("Failed to get event: %v", err)
	}

	if event.TransitionID != "t1" {
		t.Errorf("Expected TransitionID 't1', got '%s'", event.TransitionID)
	}
}

// TestSaveWorkflowState tests saving complete workflow state
func TestSaveWorkflowState(t *testing.T) {
	registry := token.NewMapRegistry()
	err := token.RegisterBuiltinTypes(registry)
	if err != nil {
		t.Fatalf("Failed to register builtin types: %v", err)
	}

	clk := clock.NewVirtualClock(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC))

	// Create marking
	marking := &Marking{
		Timestamp: clk.Now(),
		Places: map[string]PlaceState{
			"place-1": {
				PlaceID:  "place-1",
				Capacity: 100,
				Tokens: []*token.Token{
					{ID: "token-1", Data: &token.IntToken{Value: 42}},
				},
			},
		},
	}

	// Create event log
	eventLog := NewMemoryEventLog()
	event := NewEvent(EventTransitionFired, "net-1", clk)
	event.TransitionID = "t1"
	eventLog.Append(event)

	// Save workflow state
	tmpDir := t.TempDir()
	filepath := filepath.Join(tmpDir, "workflow.json")

	err = SaveWorkflowState(marking, eventLog, filepath, registry)
	if err != nil {
		t.Fatalf("SaveWorkflowState failed: %v", err)
	}

	// Verify file exists and contains valid JSON
	data, err := os.ReadFile(filepath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	var state WorkflowState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Errorf("File does not contain valid JSON: %v", err)
	}

	// Verify both marking and events are present
	if len(state.Events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(state.Events))
	}
}

// TestLargeMarkingSerialization tests performance with large markings
func TestLargeMarkingSerialization(t *testing.T) {
	registry := token.NewMapRegistry()
	err := token.RegisterBuiltinTypes(registry)
	if err != nil {
		t.Fatalf("Failed to register builtin types: %v", err)
	}

	// Create a large marking
	marking := &Marking{
		Timestamp: time.Now(),
		Places:    make(map[string]PlaceState),
	}

	// Add 100 places with 10 tokens each
	for i := 0; i < 100; i++ {
		placeID := fmt.Sprintf("place-%d", i)
		tokens := make([]*token.Token, 10)
		for j := 0; j < 10; j++ {
			tokens[j] = &token.Token{
				ID:   fmt.Sprintf("token-%d-%d", i, j),
				Data: &token.IntToken{Value: i*10 + j},
			}
		}
		marking.Places[placeID] = PlaceState{
			PlaceID:  placeID,
			Capacity: 100,
			Tokens:   tokens,
		}
	}

	// Serialize
	start := time.Now()
	data, err := marking.Serialize(registry)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	t.Logf("Serialized 1000 tokens in %v, size: %d bytes", elapsed, len(data))

	// Deserialize
	start = time.Now()
	restored, err := DeserializeMarking(data, registry)
	elapsed = time.Since(start)

	if err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	t.Logf("Deserialized 1000 tokens in %v", elapsed)

	// Verify
	if restored.TokenCount() != 1000 {
		t.Errorf("Expected 1000 tokens, got %d", restored.TokenCount())
	}
}
