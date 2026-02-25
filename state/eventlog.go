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
	"sync"
	"time"
)

// EventLog is the interface for event storage and retrieval.
// Implementations must be safe for concurrent access by multiple goroutines.
//
// The event log provides an append-only audit trail of workflow events.
// Events are immutable once appended and can be queried by various criteria.
//
// EventLog supports:
//   - Appending new events (thread-safe)
//   - Retrieving events by ID, type, timestamp, or transition
//   - Counting total events
//   - Clearing all events (primarily for testing)
type EventLog interface {
	// Append adds a new event to the log.
	// Returns an error if the event is nil, has no ID, or has a duplicate ID.
	// This operation is thread-safe and can be called concurrently.
	Append(event *Event) error

	// Get retrieves an event by its unique ID.
	// Returns an error if the event is not found.
	Get(eventID string) (*Event, error)

	// GetAll retrieves all events in the order they were appended.
	// Returns a copy of the events slice to prevent external modification.
	// Returns an empty slice if the log is empty.
	GetAll() ([]*Event, error)

	// GetSince retrieves all events that occurred after the specified timestamp.
	// Events with timestamps exactly equal to the cutoff are not included.
	// Returns events in the order they were appended.
	GetSince(timestamp time.Time) ([]*Event, error)

	// GetByType retrieves all events of a specific type.
	// Returns events in the order they were appended.
	// Returns an empty slice if no events match.
	GetByType(eventType EventType) ([]*Event, error)

	// GetByTransition retrieves all events associated with a specific transition.
	// Returns events in the order they were appended.
	// Returns an empty slice if no events match.
	GetByTransition(transitionID string) ([]*Event, error)

	// Count returns the total number of events in the log.
	Count() int

	// Clear removes all events from the log.
	// This is primarily intended for testing and should be used with caution.
	Clear() error
}

// MemoryEventLog is an in-memory implementation of EventLog.
// It stores events in a slice and provides fast access via an index map.
//
// Circular Buffer Behavior:
// When MaxEvents is > 0, the log maintains a maximum number of events.
// Once the limit is reached, the oldest events are dropped (FIFO) to make room
// for new events. This prevents unbounded memory growth in long-running workflows.
// When MaxEvents is 0, the log grows unbounded (unlimited).
//
// Performance characteristics:
//   - Append: O(1) amortized
//   - Get by ID: O(1) via index
//   - GetAll: O(n) where n is total events
//   - GetSince, GetByType, GetByTransition: O(n) with filtering
//   - Count: O(1)
//   - Clear: O(1)
//
// Thread Safety:
// All operations are thread-safe using a RWMutex.
// Append operations take a write lock.
// Query operations take a read lock, allowing concurrent reads.
type MemoryEventLog struct {
	mu        sync.RWMutex
	events    []*Event
	index     map[string]*Event // Fast lookup by ID
	MaxEvents int               // Maximum number of events to keep (0 = unlimited)
}

// NewMemoryEventLog creates a new in-memory event log with configurable max events.
// The log is initialized empty and ready for use.
//
// Parameters:
//   - maxEvents: Maximum number of events to keep (0 = unlimited)
//     Common values:
//     - 0: Unlimited (use for debugging, short workflows)
//     - 1000: Small limit (testing, resource-constrained environments)
//     - 10000: Medium limit (typical production use)
//     - 100000: Large limit (intensive logging, audit trails)
//
// When the limit is reached, oldest events are dropped to maintain the limit.
func NewMemoryEventLog(maxEvents ...int) *MemoryEventLog {
	max := 0 // Default: unlimited
	if len(maxEvents) > 0 {
		max = maxEvents[0]
	}
	return &MemoryEventLog{
		events:    make([]*Event, 0),
		index:     make(map[string]*Event),
		MaxEvents: max,
	}
}

// Append adds a new event to the log.
// The event parameter should be a *Event.
// Returns an error if:
//   - event is nil
//   - event has no ID
//   - an event with the same ID already exists
//
// Circular Buffer Behavior:
// If MaxEvents > 0 and the log is at capacity, the oldest event is removed
// to make room for the new event. This implements FIFO (First In, First Out)
// eviction to prevent unbounded memory growth.
//
// Thread Safety: Takes a write lock for the duration of the append.
func (m *MemoryEventLog) Append(event interface{}) error {
	// Type assert to *Event
	e, ok := event.(*Event)
	if !ok {
		return fmt.Errorf("event must be of type *state.Event")
	}
	if e == nil {
		return fmt.Errorf("cannot append nil event")
	}
	if e.ID == "" {
		return fmt.Errorf("event must have an ID")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for duplicate
	if _, exists := m.index[e.ID]; exists {
		return fmt.Errorf("event %s already exists", e.ID)
	}

	// Implement circular buffer: drop oldest event if at capacity
	if m.MaxEvents > 0 && len(m.events) >= m.MaxEvents {
		// Remove oldest event (first in slice)
		oldestEvent := m.events[0]
		delete(m.index, oldestEvent.ID)
		m.events = m.events[1:] // Remove first element
	}

	m.events = append(m.events, e)
	m.index[e.ID] = e
	return nil
}

// Get retrieves an event by its unique ID.
// Returns an error if the event is not found.
//
// Thread Safety: Takes a read lock for the duration of the lookup.
func (m *MemoryEventLog) Get(eventID string) (*Event, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	event, exists := m.index[eventID]
	if !exists {
		return nil, fmt.Errorf("event %s not found", eventID)
	}
	return event, nil
}

// GetAll retrieves all events in the order they were appended.
// Returns a copy of the events slice to prevent external modification.
//
// Thread Safety: Takes a read lock for the duration of the copy.
func (m *MemoryEventLog) GetAll() ([]*Event, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make([]*Event, len(m.events))
	copy(result, m.events)
	return result, nil
}

// GetSince retrieves all events that occurred after the specified timestamp.
// Events with timestamps exactly equal to the cutoff are not included.
//
// Thread Safety: Takes a read lock for the duration of the filtering.
func (m *MemoryEventLog) GetSince(timestamp time.Time) ([]*Event, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Event, 0)
	for _, event := range m.events {
		if event.Timestamp.After(timestamp) {
			result = append(result, event)
		}
	}
	return result, nil
}

// GetByType retrieves all events of a specific type.
// Returns events in the order they were appended.
//
// Thread Safety: Takes a read lock for the duration of the filtering.
func (m *MemoryEventLog) GetByType(eventType EventType) ([]*Event, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Event, 0)
	for _, event := range m.events {
		if event.Type == eventType {
			result = append(result, event)
		}
	}
	return result, nil
}

// GetByTransition retrieves all events associated with a specific transition.
// Returns events in the order they were appended.
//
// Thread Safety: Takes a read lock for the duration of the filtering.
func (m *MemoryEventLog) GetByTransition(transitionID string) ([]*Event, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Event, 0)
	for _, event := range m.events {
		if event.TransitionID == transitionID {
			result = append(result, event)
		}
	}
	return result, nil
}

// Count returns the total number of events in the log.
//
// Thread Safety: Takes a read lock for the duration of the count.
func (m *MemoryEventLog) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.events)
}

// Clear removes all events from the log.
// This is primarily intended for testing and should be used with caution.
//
// Thread Safety: Takes a write lock for the duration of the clear.
func (m *MemoryEventLog) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.events = make([]*Event, 0)
	m.index = make(map[string]*Event)
	return nil
}
