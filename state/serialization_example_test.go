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

package state_test

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jazzpetri/engine/clock"
	"github.com/jazzpetri/engine/petri"
	"github.com/jazzpetri/engine/state"
	"github.com/jazzpetri/engine/token"
)

// ExampleMarking_Serialize demonstrates basic serialization of a marking
func ExampleMarking_Serialize() {
	// Setup registry and register builtin types
	registry := token.NewMapRegistry()
	token.RegisterBuiltinTypes(registry)

	// Create a simple marking
	marking := &state.Marking{
		Timestamp: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		Places: map[string]state.PlaceState{
			"inbox": {
				PlaceID:  "inbox",
				Capacity: 100,
				Tokens: []*token.Token{
					{
						ID:   "msg-1",
						Data: &token.StringToken{Value: "Hello, World!"},
					},
				},
			},
		},
	}

	// Serialize to JSON
	data, err := marking.Serialize(registry)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Serialized %d bytes\n", len(data))
	fmt.Println("JSON is human-readable and contains token type information")
	// Output:
	// Serialized 304 bytes
	// JSON is human-readable and contains token type information
}

// ExampleDeserializeMarking demonstrates deserializing a marking from JSON
func ExampleDeserializeMarking() {
	// Setup registry
	registry := token.NewMapRegistry()
	token.RegisterBuiltinTypes(registry)

	// JSON from a saved marking
	jsonData := []byte(`{
  "timestamp": "2025-01-15T10:30:00Z",
  "places": {
    "inbox": {
      "place_id": "inbox",
      "capacity": 100,
      "tokens": [
        {
          "id": "msg-1",
          "type": "StringToken",
          "data": {"Value": "Hello, World!"}
        }
      ]
    }
  }
}`)

	// Deserialize
	marking, err := state.DeserializeMarking(jsonData, registry)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Access the data
	tokens := marking.GetTokens("inbox")
	if len(tokens) > 0 {
		stringToken := tokens[0].Data.(*token.StringToken)
		fmt.Printf("Message: %s\n", stringToken.Value)
	}

	// Output:
	// Message: Hello, World!
}

// ExampleSaveMarkingToFile demonstrates saving a workflow checkpoint
func ExampleSaveMarkingToFile() {
	// Setup
	registry := token.NewMapRegistry()
	token.RegisterBuiltinTypes(registry)
	clk := clock.NewVirtualClock(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC))

	// Create a simple workflow
	net := petri.NewPetriNet("order-proc-1", "Order Processing")
	pending := petri.NewPlace("pending", "Pending Orders", 100)
	processing := petri.NewPlace("processing", "Processing Orders", 10)

	net.AddPlace(pending)
	net.AddPlace(processing)
	net.Resolve()

	// Add some tokens
	pending.AddToken(&token.Token{
		ID: "order-1",
		Data: &token.MapToken{
			Value: map[string]interface{}{
				"orderId": "ORD-001",
				"amount":  float64(99.99),
			},
		},
	})

	// Capture state
	marking, _ := state.NewMarking(net, clk)

	// Save to file
	tmpDir := os.TempDir()
	checkpointFile := filepath.Join(tmpDir, "checkpoint.json")
	err := state.SaveMarkingToFile(marking, checkpointFile, registry)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Verify file exists
	if _, err := os.Stat(checkpointFile); err == nil {
		fmt.Println("Checkpoint saved successfully")
		os.Remove(checkpointFile) // Cleanup
	}

	// Output:
	// Checkpoint saved successfully
}

// ExampleLoadMarkingFromFile demonstrates loading a workflow from a checkpoint
func ExampleLoadMarkingFromFile() {
	// Setup
	registry := token.NewMapRegistry()
	token.RegisterBuiltinTypes(registry)

	// Create and save a marking
	marking := &state.Marking{
		Timestamp: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		Places: map[string]state.PlaceState{
			"completed": {
				PlaceID:  "completed",
				Capacity: 100,
				Tokens: []*token.Token{
					{
						ID:   "task-1",
						Data: &token.IntToken{Value: 42},
					},
				},
			},
		},
	}

	tmpDir := os.TempDir()
	checkpointFile := filepath.Join(tmpDir, "test-checkpoint.json")
	state.SaveMarkingToFile(marking, checkpointFile, registry)

	// Later, load the checkpoint
	restored, err := state.LoadMarkingFromFile(checkpointFile, registry)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Loaded marking with %d tokens\n", restored.TokenCount())
	fmt.Printf("Token in 'completed' place: %d\n", restored.PlaceTokenCount("completed"))

	os.Remove(checkpointFile) // Cleanup

	// Output:
	// Loaded marking with 1 tokens
	// Token in 'completed' place: 1
}

// ExampleSaveWorkflowState demonstrates saving complete workflow state
func ExampleSaveWorkflowState() {
	// Setup
	registry := token.NewMapRegistry()
	token.RegisterBuiltinTypes(registry)
	clk := clock.NewVirtualClock(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC))

	// Create a workflow
	net := petri.NewPetriNet("workflow-1", "Workflow 1")
	start := petri.NewPlace("start", "Start", 1)
	net.AddPlace(start)
	net.Resolve()

	start.AddToken(&token.Token{
		ID:   "token-1",
		Data: &token.IntToken{Value: 1},
	})

	// Capture marking
	marking, _ := state.NewMarking(net, clk)

	// Create event log
	eventLog := state.NewMemoryEventLog()
	event := state.NewEvent(state.EventEngineStarted, "workflow-1", clk)
	eventLog.Append(event)

	// Save complete state
	tmpDir := os.TempDir()
	stateFile := filepath.Join(tmpDir, "workflow-state.json")
	err := state.SaveWorkflowState(marking, eventLog, stateFile, registry)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Verify
	if _, err := os.Stat(stateFile); err == nil {
		fmt.Println("Complete workflow state saved")
		fmt.Println("File contains both marking and event log")
		os.Remove(stateFile) // Cleanup
	}

	// Output:
	// Complete workflow state saved
	// File contains both marking and event log
}

// Example_debuggingWithJSON demonstrates using JSON for debugging
func Example_debuggingWithJSON() {
	// Setup
	registry := token.NewMapRegistry()
	token.RegisterBuiltinTypes(registry)
	clk := clock.NewVirtualClock(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC))

	// Create workflow
	net := petri.NewPetriNet("debug-wf", "Debug Workflow")
	place1 := petri.NewPlace("place-1", "Place 1", 10)
	place2 := petri.NewPlace("place-2", "Place 2", 10)
	net.AddPlace(place1)
	net.AddPlace(place2)
	net.Resolve()

	// Add tokens with different types
	place1.AddToken(&token.Token{ID: "t1", Data: &token.IntToken{Value: 42}})
	place2.AddToken(&token.Token{ID: "t2", Data: &token.StringToken{Value: "debug"}})

	// Capture and serialize
	marking, _ := state.NewMarking(net, clk)
	jsonData, _ := marking.Serialize(registry)

	// The JSON is human-readable for debugging
	fmt.Println("JSON format is human-readable")
	fmt.Printf("Contains %d places\n", len(marking.Places))
	fmt.Printf("Total tokens: %d\n", marking.TokenCount())
	fmt.Printf("JSON size: %d bytes\n", len(jsonData))

	// Output:
	// JSON format is human-readable
	// Contains 2 places
	// Total tokens: 2
	// JSON size: 525 bytes
}

// Example_roundTripVerification demonstrates verifying data integrity
func Example_roundTripVerification() {
	// Setup
	registry := token.NewMapRegistry()
	token.RegisterBuiltinTypes(registry)

	// Create original marking
	original := &state.Marking{
		Timestamp: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		Places: map[string]state.PlaceState{
			"test": {
				PlaceID:  "test",
				Capacity: 100,
				Tokens: []*token.Token{
					{ID: "t1", Data: &token.IntToken{Value: 123}},
				},
			},
		},
	}

	// Serialize
	data, _ := original.Serialize(registry)

	// Deserialize
	restored, _ := state.DeserializeMarking(data, registry)

	// Verify
	if original.Equal(restored) {
		fmt.Println("Round-trip successful")
		fmt.Println("Data integrity verified")
	}

	// Verify token data
	origTokens := original.GetTokens("test")
	restTokens := restored.GetTokens("test")
	origInt := origTokens[0].Data.(*token.IntToken)
	restInt := restTokens[0].Data.(*token.IntToken)

	if origInt.Value == restInt.Value {
		fmt.Println("Token data preserved")
	}

	// Output:
	// Round-trip successful
	// Data integrity verified
	// Token data preserved
}
