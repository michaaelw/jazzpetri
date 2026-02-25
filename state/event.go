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
	"fmt"
	"sync/atomic"
	"time"

	"github.com/jazzpetri/engine/clock"
	"github.com/jazzpetri/engine/context"
)

// eventIDCounter is an atomic counter for generating unique event IDs
var eventIDCounter uint64

// Re-export EventType and constants from context package for convenience
type EventType = context.EventType

const (
	EventTransitionFired   = context.EventTransitionFired
	EventTransitionEnabled = context.EventTransitionEnabled
	EventTransitionFailed  = context.EventTransitionFailed
	EventEngineStarted     = context.EventEngineStarted
	EventEngineStopped     = context.EventEngineStopped
	EventTokenProduced     = context.EventTokenProduced
	EventTokenConsumed     = context.EventTokenConsumed
	EventErrorOccurred     = context.EventErrorOccurred
)

// Event represents a single workflow event in the event log.
// Events are immutable records of state transitions and workflow activities.
// They provide an append-only audit trail for recovery, replay, and debugging.
type Event struct {
	// ID is a unique identifier for this event
	ID string

	// Type identifies the category of event
	Type EventType

	// Timestamp records when the event occurred
	Timestamp time.Time

	// NetID identifies the Petri net that generated this event
	NetID string

	// TransitionID identifies the transition for transition events
	// Empty for non-transition events
	TransitionID string

	// PlaceID identifies the place for token events
	// Empty for non-token events
	PlaceID string

	// TokenID identifies the token for token events
	// Empty for non-token events
	TokenID string

	// Error contains error message for error events
	// Empty for non-error events
	Error string

	// Metadata contains additional context-specific information
	// This allows events to carry arbitrary data without schema changes
	Metadata map[string]interface{}
}

// NewEvent creates a new event with an auto-generated unique ID.
// The ID is generated using an atomic counter to ensure uniqueness.
// The timestamp is set using the provided clock.
//
// Parameters:
//   - eventType: The type of event being created
//   - netID: The ID of the Petri net generating the event
//   - clk: Clock for timestamp generation
//
// Returns a new Event with initialized metadata map.
func NewEvent(eventType EventType, netID string, clk clock.Clock) *Event {
	id := atomic.AddUint64(&eventIDCounter, 1)

	return &Event{
		ID:        fmt.Sprintf("event-%d", id),
		Type:      eventType,
		Timestamp: clk.Now(),
		NetID:     netID,
		Metadata:  make(map[string]interface{}),
	}
}

// DefaultEventBuilder is the standard implementation of EventBuilder.
// It creates Event instances using NewEvent().
type DefaultEventBuilder struct {
	clock clock.Clock
}

// NewDefaultEventBuilder creates a new DefaultEventBuilder with the given clock.
func NewDefaultEventBuilder(clk clock.Clock) *DefaultEventBuilder {
	return &DefaultEventBuilder{clock: clk}
}

// NewEvent creates a new event with the given type and net ID.
// This implements the context.EventBuilder interface.
func (b *DefaultEventBuilder) NewEvent(eventType context.EventType, netID string) interface{} {
	return NewEvent(eventType, netID, b.clock)
}

// Setter methods for Event to allow field updates without importing state package

// SetTransitionID sets the transition ID field.
func (e *Event) SetTransitionID(id string) {
	e.TransitionID = id
}

// SetPlaceID sets the place ID field.
func (e *Event) SetPlaceID(id string) {
	e.PlaceID = id
}

// SetTokenID sets the token ID field.
func (e *Event) SetTokenID(id string) {
	e.TokenID = id
}

// SetError sets the error message field.
func (e *Event) SetError(err string) {
	e.Error = err
}

// SetMetadata sets or merges metadata fields.
func (e *Event) SetMetadata(m map[string]interface{}) {
	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{})
	}
	for k, v := range m {
		e.Metadata[k] = v
	}
}
