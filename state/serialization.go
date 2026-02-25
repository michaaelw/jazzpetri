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
	"time"

	"github.com/jazzpetri/engine/token"
)

// SerializableMarking is the JSON-serializable representation of a Marking.
// It converts the complex token.Token structure into a format that can be
// properly serialized to JSON while preserving type information.
type SerializableMarking struct {
	Timestamp time.Time                   `json:"timestamp"`
	Places    map[string]SerializablePlace `json:"places"`
}

// SerializablePlace is the JSON-serializable representation of a PlaceState.
// It maintains place metadata and converts tokens to a serializable format.
type SerializablePlace struct {
	PlaceID  string              `json:"place_id"`
	Tokens   []SerializableToken `json:"tokens"`
	Capacity int                 `json:"capacity"`
}

// SerializableToken is the JSON-serializable representation of a Token.
// It preserves the token ID, type identifier, and serialized data payload.
type SerializableToken struct {
	ID   string          `json:"id"`
	Type string          `json:"type"` // Token type identifier
	Data json.RawMessage `json:"data"` // Serialized token data
}

// Serialize converts a Marking to JSON using the token registry.
// The registry is used to serialize individual token data while preserving type information.
//
// The serialized format is human-readable JSON with proper indentation.
// Token type information is preserved in the "type" field of each token.
//
// Parameters:
//   - registry: TypeRegistry for serializing token data
//
// Returns:
//   - Serialized JSON bytes with indentation
//   - Error if marking is nil, registry is nil, or serialization fails
func (m *Marking) Serialize(registry token.TypeRegistry) ([]byte, error) {
	if m == nil {
		return nil, fmt.Errorf("cannot serialize nil marking")
	}
	if registry == nil {
		return nil, fmt.Errorf("registry cannot be nil")
	}

	// Convert to serializable representation
	sm := SerializableMarking{
		Timestamp: m.Timestamp,
		Places:    make(map[string]SerializablePlace),
	}

	for placeID, placeState := range m.Places {
		sp := SerializablePlace{
			PlaceID:  placeState.PlaceID,
			Capacity: placeState.Capacity,
			Tokens:   make([]SerializableToken, 0, len(placeState.Tokens)),
		}

		// Serialize each token
		for _, tok := range placeState.Tokens {
			// Serialize token data using registry
			data, err := registry.Serialize(tok.Data)
			if err != nil {
				return nil, fmt.Errorf("failed to serialize token %s: %w", tok.ID, err)
			}

			st := SerializableToken{
				ID:   tok.ID,
				Type: tok.Data.Type(),
				Data: json.RawMessage(data),
			}
			sp.Tokens = append(sp.Tokens, st)
		}

		sm.Places[placeID] = sp
	}

	// Marshal to JSON with indentation for human readability
	return json.MarshalIndent(sm, "", "  ")
}

// DeserializeMarking converts JSON back to a Marking using the token registry.
// The registry is used to deserialize individual token data based on type information.
//
// This function reconstructs the complete Marking structure including all places,
// tokens, and their data payloads.
//
// Parameters:
//   - data: JSON bytes to deserialize
//   - registry: TypeRegistry for deserializing token data
//
// Returns:
//   - Reconstructed Marking
//   - Error if data is empty, registry is nil, JSON is invalid, or token types are unknown
func DeserializeMarking(data []byte, registry token.TypeRegistry) (*Marking, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("cannot deserialize empty data")
	}
	if registry == nil {
		return nil, fmt.Errorf("registry cannot be nil")
	}

	// Unmarshal from JSON
	var sm SerializableMarking
	if err := json.Unmarshal(data, &sm); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Convert to Marking
	marking := &Marking{
		Timestamp: sm.Timestamp,
		Places:    make(map[string]PlaceState),
	}

	for placeID, sp := range sm.Places {
		ps := PlaceState{
			PlaceID:  sp.PlaceID,
			Capacity: sp.Capacity,
			Tokens:   make([]*token.Token, 0, len(sp.Tokens)),
		}

		// Deserialize each token
		for _, st := range sp.Tokens {
			// Deserialize token data using registry
			tokenData, err := registry.Deserialize(st.Type, []byte(st.Data))
			if err != nil {
				return nil, fmt.Errorf("failed to deserialize token %s: %w", st.ID, err)
			}

			tok := &token.Token{
				ID:   st.ID,
				Data: tokenData,
			}
			ps.Tokens = append(ps.Tokens, tok)
		}

		marking.Places[placeID] = ps
	}

	return marking, nil
}

// SaveMarkingToFile serializes a marking to a JSON file.
// The file is created with 0644 permissions (readable by all, writable by owner).
//
// Parameters:
//   - marking: The marking to save
//   - filepath: Path where the file should be created
//   - registry: TypeRegistry for serializing token data
//
// Returns:
//   - Error if serialization fails or file cannot be written
func SaveMarkingToFile(marking *Marking, filepath string, registry token.TypeRegistry) error {
	data, err := marking.Serialize(registry)
	if err != nil {
		return fmt.Errorf("serialization failed: %w", err)
	}

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// LoadMarkingFromFile deserializes a marking from a JSON file.
//
// Parameters:
//   - filepath: Path to the JSON file
//   - registry: TypeRegistry for deserializing token data
//
// Returns:
//   - Reconstructed Marking
//   - Error if file cannot be read or deserialization fails
func LoadMarkingFromFile(filepath string, registry token.TypeRegistry) (*Marking, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return DeserializeMarking(data, registry)
}

// SerializeEventLog converts an event log to JSON.
// Events are serialized in the order they appear in the log.
//
// Parameters:
//   - log: The MemoryEventLog to serialize
//
// Returns:
//   - Serialized JSON bytes with indentation
//   - Error if GetAll fails or marshaling fails
func SerializeEventLog(log *MemoryEventLog) ([]byte, error) {
	events, err := log.GetAll()
	if err != nil {
		return nil, fmt.Errorf("failed to get events: %w", err)
	}

	return json.MarshalIndent(events, "", "  ")
}

// DeserializeEventLog converts JSON back to events.
// This returns a slice of events that can be used to populate a new EventLog.
//
// Parameters:
//   - data: JSON bytes to deserialize
//
// Returns:
//   - Slice of Event pointers
//   - Error if JSON is invalid
func DeserializeEventLog(data []byte) ([]*Event, error) {
	var events []*Event
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, fmt.Errorf("failed to unmarshal events: %w", err)
	}
	return events, nil
}

// SaveEventLogToFile saves event log to a JSON file.
// The file is created with 0644 permissions (readable by all, writable by owner).
//
// Parameters:
//   - log: The MemoryEventLog to save
//   - filepath: Path where the file should be created
//
// Returns:
//   - Error if serialization fails or file cannot be written
func SaveEventLogToFile(log *MemoryEventLog, filepath string) error {
	data, err := SerializeEventLog(log)
	if err != nil {
		return fmt.Errorf("serialization failed: %w", err)
	}

	return os.WriteFile(filepath, data, 0644)
}

// LoadEventLogFromFile loads events from a JSON file into a new EventLog.
// This creates a new MemoryEventLog and populates it with the events from the file.
//
// Parameters:
//   - filepath: Path to the JSON file
//
// Returns:
//   - New EventLog populated with events from file
//   - Error if file cannot be read, JSON is invalid, or event appending fails
func LoadEventLogFromFile(filepath string) (*MemoryEventLog, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	events, err := DeserializeEventLog(data)
	if err != nil {
		return nil, err
	}

	// Create new log and populate
	log := NewMemoryEventLog()
	for _, event := range events {
		if err := log.Append(event); err != nil {
			return nil, fmt.Errorf("failed to append event %s: %w", event.ID, err)
		}
	}

	return log, nil
}

// WorkflowState represents complete workflow state.
// This combines both the current marking (snapshot) and the event log (history).
// Together they provide a complete picture of workflow state that can be
// used for checkpointing, recovery, and debugging.
type WorkflowState struct {
	Marking *SerializableMarking `json:"marking"`
	Events  []*Event             `json:"events"`
}

// SaveWorkflowState saves both marking and event log to a single JSON file.
// This provides a complete checkpoint of workflow state.
//
// Parameters:
//   - marking: The current marking to save
//   - eventLog: The event log to save
//   - filepath: Path where the file should be created
//   - registry: TypeRegistry for serializing token data
//
// Returns:
//   - Error if serialization fails or file cannot be written
func SaveWorkflowState(marking *Marking, eventLog *MemoryEventLog, filepath string, registry token.TypeRegistry) error {
	// Serialize marking
	markingData, err := marking.Serialize(registry)
	if err != nil {
		return fmt.Errorf("failed to serialize marking: %w", err)
	}

	// Convert to SerializableMarking
	var sm SerializableMarking
	if err := json.Unmarshal(markingData, &sm); err != nil {
		return fmt.Errorf("failed to unmarshal marking: %w", err)
	}

	// Get events
	events, err := eventLog.GetAll()
	if err != nil {
		return fmt.Errorf("failed to get events: %w", err)
	}

	state := WorkflowState{
		Marking: &sm,
		Events:  events,
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal workflow state: %w", err)
	}

	return os.WriteFile(filepath, data, 0644)
}
