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
	"sync"
	"testing"
	"time"

	"github.com/jazzpetri/engine/clock"
)

// Test event log creation
func TestNewMemoryEventLog(t *testing.T) {
	log := NewMemoryEventLog()

	if log == nil {
		t.Fatal("expected event log, got nil")
	}
	if log.Count() != 0 {
		t.Errorf("expected empty log, got count %d", log.Count())
	}
}

// Test appending events
func TestMemoryEventLog_Append(t *testing.T) {
	t.Run("appends event successfully", func(t *testing.T) {
		log := NewMemoryEventLog()
		clk := clock.NewVirtualClock(time.Now())
		event := NewEvent(EventTransitionFired, "net1", clk)

		err := log.Append(event)
		if err != nil {
			t.Fatalf("failed to append event: %v", err)
		}

		if log.Count() != 1 {
			t.Errorf("expected count 1, got %d", log.Count())
		}
	})

	t.Run("rejects nil event", func(t *testing.T) {
		log := NewMemoryEventLog()

		err := log.Append(nil)
		if err == nil {
			t.Error("expected error for nil event")
		}
		if log.Count() != 0 {
			t.Errorf("expected count 0, got %d", log.Count())
		}
	})

	t.Run("rejects event without ID", func(t *testing.T) {
		log := NewMemoryEventLog()
		event := &Event{
			Type:   EventTransitionFired,
			NetID:  "net1",
			ID:     "", // Empty ID
		}

		err := log.Append(event)
		if err == nil {
			t.Error("expected error for event without ID")
		}
		if log.Count() != 0 {
			t.Errorf("expected count 0, got %d", log.Count())
		}
	})

	t.Run("rejects duplicate event ID", func(t *testing.T) {
		log := NewMemoryEventLog()
		clk := clock.NewVirtualClock(time.Now())
		event1 := NewEvent(EventTransitionFired, "net1", clk)
		event2 := &Event{
			ID:    event1.ID, // Same ID
			Type:  EventTransitionFired,
			NetID: "net1",
		}

		log.Append(event1)
		err := log.Append(event2)

		if err == nil {
			t.Error("expected error for duplicate event ID")
		}
		if log.Count() != 1 {
			t.Errorf("expected count 1, got %d", log.Count())
		}
	})

	t.Run("maintains insertion order", func(t *testing.T) {
		log := NewMemoryEventLog()
		clk := clock.NewVirtualClock(time.Now())

		events := make([]*Event, 5)
		for i := 0; i < 5; i++ {
			events[i] = NewEvent(EventTransitionFired, "net1", clk)
			log.Append(events[i])
		}

		all, _ := log.GetAll()
		for i, e := range all {
			if e.ID != events[i].ID {
				t.Errorf("expected event %d to be %s, got %s", i, events[i].ID, e.ID)
			}
		}
	})
}

// Test retrieving events by ID
func TestMemoryEventLog_Get(t *testing.T) {
	t.Run("retrieves existing event", func(t *testing.T) {
		log := NewMemoryEventLog()
		clk := clock.NewVirtualClock(time.Now())
		event := NewEvent(EventTransitionFired, "net1", clk)
		log.Append(event)

		retrieved, err := log.Get(event.ID)
		if err != nil {
			t.Fatalf("failed to get event: %v", err)
		}
		if retrieved.ID != event.ID {
			t.Errorf("expected event %s, got %s", event.ID, retrieved.ID)
		}
	})

	t.Run("returns error for non-existent event", func(t *testing.T) {
		log := NewMemoryEventLog()

		_, err := log.Get("nonexistent")
		if err == nil {
			t.Error("expected error for non-existent event")
		}
	})
}

// Test retrieving all events
func TestMemoryEventLog_GetAll(t *testing.T) {
	t.Run("returns empty slice for empty log", func(t *testing.T) {
		log := NewMemoryEventLog()

		events, err := log.GetAll()
		if err != nil {
			t.Fatalf("failed to get all events: %v", err)
		}
		if len(events) != 0 {
			t.Errorf("expected 0 events, got %d", len(events))
		}
	})

	t.Run("returns all appended events", func(t *testing.T) {
		log := NewMemoryEventLog()
		clk := clock.NewVirtualClock(time.Now())

		for i := 0; i < 3; i++ {
			event := NewEvent(EventTransitionFired, "net1", clk)
			log.Append(event)
		}

		events, err := log.GetAll()
		if err != nil {
			t.Fatalf("failed to get all events: %v", err)
		}
		if len(events) != 3 {
			t.Errorf("expected 3 events, got %d", len(events))
		}
	})

	t.Run("returns copy to prevent external modification", func(t *testing.T) {
		log := NewMemoryEventLog()
		clk := clock.NewVirtualClock(time.Now())
		event := NewEvent(EventTransitionFired, "net1", clk)
		log.Append(event)

		events1, _ := log.GetAll()
		events2, _ := log.GetAll()

		// Modify the slice (not the events themselves)
		events1[0] = nil

		// events2 should still be valid
		if events2[0] == nil {
			t.Error("GetAll should return independent copies")
		}
	})
}

// Test filtering by timestamp
func TestMemoryEventLog_GetSince(t *testing.T) {
	t.Run("filters events after timestamp", func(t *testing.T) {
		log := NewMemoryEventLog()
		baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
		clk := clock.NewVirtualClock(baseTime)

		// Add events at different times
		event1 := NewEvent(EventTransitionFired, "net1", clk)
		log.Append(event1)

		clk.AdvanceTo(baseTime.Add(1 * time.Hour))
		event2 := NewEvent(EventTransitionFired, "net1", clk)
		log.Append(event2)

		clk.AdvanceTo(baseTime.Add(2 * time.Hour))
		event3 := NewEvent(EventTransitionFired, "net1", clk)
		log.Append(event3)

		// Get events since 30 minutes
		cutoff := baseTime.Add(30 * time.Minute)
		events, err := log.GetSince(cutoff)
		if err != nil {
			t.Fatalf("failed to get events since: %v", err)
		}

		// Should get event2 and event3
		if len(events) != 2 {
			t.Errorf("expected 2 events, got %d", len(events))
		}
	})

	t.Run("returns empty for future timestamp", func(t *testing.T) {
		log := NewMemoryEventLog()
		clk := clock.NewVirtualClock(time.Now())
		event := NewEvent(EventTransitionFired, "net1", clk)
		log.Append(event)

		future := time.Now().Add(1 * time.Hour)
		events, err := log.GetSince(future)
		if err != nil {
			t.Fatalf("failed to get events since: %v", err)
		}
		if len(events) != 0 {
			t.Errorf("expected 0 events, got %d", len(events))
		}
	})
}

// Test filtering by event type
func TestMemoryEventLog_GetByType(t *testing.T) {
	t.Run("filters events by type", func(t *testing.T) {
		log := NewMemoryEventLog()
		clk := clock.NewVirtualClock(time.Now())

		// Add different event types
		event1 := NewEvent(EventTransitionFired, "net1", clk)
		log.Append(event1)

		event2 := NewEvent(EventEngineStarted, "net1", clk)
		log.Append(event2)

		event3 := NewEvent(EventTransitionFired, "net1", clk)
		log.Append(event3)

		// Get only transition fired events
		events, err := log.GetByType(EventTransitionFired)
		if err != nil {
			t.Fatalf("failed to get events by type: %v", err)
		}

		if len(events) != 2 {
			t.Errorf("expected 2 events, got %d", len(events))
		}
		for _, e := range events {
			if e.Type != EventTransitionFired {
				t.Errorf("expected type %s, got %s", EventTransitionFired, e.Type)
			}
		}
	})

	t.Run("returns empty for non-existent type", func(t *testing.T) {
		log := NewMemoryEventLog()
		clk := clock.NewVirtualClock(time.Now())
		event := NewEvent(EventTransitionFired, "net1", clk)
		log.Append(event)

		events, err := log.GetByType(EventEngineStarted)
		if err != nil {
			t.Fatalf("failed to get events by type: %v", err)
		}
		if len(events) != 0 {
			t.Errorf("expected 0 events, got %d", len(events))
		}
	})
}

// Test filtering by transition
func TestMemoryEventLog_GetByTransition(t *testing.T) {
	t.Run("filters events by transition ID", func(t *testing.T) {
		log := NewMemoryEventLog()
		clk := clock.NewVirtualClock(time.Now())

		// Add events for different transitions
		event1 := NewEvent(EventTransitionFired, "net1", clk)
		event1.TransitionID = "t1"
		log.Append(event1)

		event2 := NewEvent(EventTransitionFired, "net1", clk)
		event2.TransitionID = "t2"
		log.Append(event2)

		event3 := NewEvent(EventTransitionFired, "net1", clk)
		event3.TransitionID = "t1"
		log.Append(event3)

		// Get events for t1
		events, err := log.GetByTransition("t1")
		if err != nil {
			t.Fatalf("failed to get events by transition: %v", err)
		}

		if len(events) != 2 {
			t.Errorf("expected 2 events, got %d", len(events))
		}
		for _, e := range events {
			if e.TransitionID != "t1" {
				t.Errorf("expected transition t1, got %s", e.TransitionID)
			}
		}
	})

	t.Run("returns empty for non-existent transition", func(t *testing.T) {
		log := NewMemoryEventLog()
		clk := clock.NewVirtualClock(time.Now())
		event := NewEvent(EventTransitionFired, "net1", clk)
		event.TransitionID = "t1"
		log.Append(event)

		events, err := log.GetByTransition("t2")
		if err != nil {
			t.Fatalf("failed to get events by transition: %v", err)
		}
		if len(events) != 0 {
			t.Errorf("expected 0 events, got %d", len(events))
		}
	})
}

// Test count
func TestMemoryEventLog_Count(t *testing.T) {
	log := NewMemoryEventLog()
	clk := clock.NewVirtualClock(time.Now())

	if log.Count() != 0 {
		t.Errorf("expected count 0, got %d", log.Count())
	}

	for i := 1; i <= 5; i++ {
		event := NewEvent(EventTransitionFired, "net1", clk)
		log.Append(event)

		if log.Count() != i {
			t.Errorf("expected count %d, got %d", i, log.Count())
		}
	}
}

// Test clear
func TestMemoryEventLog_Clear(t *testing.T) {
	log := NewMemoryEventLog()
	clk := clock.NewVirtualClock(time.Now())

	// Add some events
	for i := 0; i < 3; i++ {
		event := NewEvent(EventTransitionFired, "net1", clk)
		log.Append(event)
	}

	if log.Count() != 3 {
		t.Errorf("expected count 3, got %d", log.Count())
	}

	// Clear the log
	err := log.Clear()
	if err != nil {
		t.Fatalf("failed to clear log: %v", err)
	}

	if log.Count() != 0 {
		t.Errorf("expected count 0 after clear, got %d", log.Count())
	}

	// Verify Get returns error for previous events
	events, _ := log.GetAll()
	if len(events) != 0 {
		t.Errorf("expected empty log after clear, got %d events", len(events))
	}
}

// Test concurrent access
func TestMemoryEventLog_ConcurrentAccess(t *testing.T) {
	log := NewMemoryEventLog()
	clk := clock.NewVirtualClock(time.Now())

	var wg sync.WaitGroup
	numGoroutines := 10
	eventsPerGoroutine := 10

	// Concurrent appends
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				event := NewEvent(EventTransitionFired, "net1", clk)
				log.Append(event)
			}
		}()
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				log.GetAll()
				log.GetByType(EventTransitionFired)
			}
		}()
	}

	wg.Wait()

	// Verify all events were added
	expectedCount := numGoroutines * eventsPerGoroutine
	if log.Count() != expectedCount {
		t.Errorf("expected count %d, got %d", expectedCount, log.Count())
	}
}

// Benchmark tests
func BenchmarkMemoryEventLog_Append(b *testing.B) {
	log := NewMemoryEventLog()
	clk := clock.NewVirtualClock(time.Now())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		event := NewEvent(EventTransitionFired, "net1", clk)
		log.Append(event)
	}
}

func BenchmarkMemoryEventLog_Get(b *testing.B) {
	log := NewMemoryEventLog()
	clk := clock.NewVirtualClock(time.Now())

	// Pre-populate with events
	eventIDs := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		event := NewEvent(EventTransitionFired, "net1", clk)
		eventIDs[i] = event.ID
		log.Append(event)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.Get(eventIDs[i%1000])
	}
}

func BenchmarkMemoryEventLog_GetByType(b *testing.B) {
	log := NewMemoryEventLog()
	clk := clock.NewVirtualClock(time.Now())

	// Pre-populate with events
	for i := 0; i < 1000; i++ {
		event := NewEvent(EventTransitionFired, "net1", clk)
		log.Append(event)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.GetByType(EventTransitionFired)
	}
}
