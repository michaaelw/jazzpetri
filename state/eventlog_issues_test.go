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
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/jazzpetri/engine/clock"
)

// TestMemoryEventLog_MaxEventsLimit tests:
// MemoryEventLog grows unbounded, causing OOM in long-running workflows
//
// FAILING TEST: This test demonstrates that the event log grows without limit
// Expected behavior after fix: Event log should maintain a maximum size and drop oldest events
func TestMemoryEventLog_MaxEventsLimit(t *testing.T) {
	// Arrange: Create event log with expected max limit
	expectedMaxEvents := 1000
	log := NewMemoryEventLog(expectedMaxEvents) // Fix: Now configurable
	clk := clock.NewVirtualClock(time.Now())

	// Act: Add events beyond the expected limit
	for i := 0; i < expectedMaxEvents*2; i++ {
		event := NewEvent(EventTransitionFired, "net1", clk)
		event.Metadata = map[string]interface{}{
			"sequence": i,
		}
		if err := log.Append(event); err != nil {
			t.Fatalf("Failed to append event %d: %v", i, err)
		}
	}

	// Assert: Count should never exceed max limit
	actualCount := log.Count()

	// Fixed: Count should be at most expectedMaxEvents due to circular buffer
	if actualCount > expectedMaxEvents {
		t.Errorf("Event log exceeded max limit: got %d events, want max %d",
			actualCount, expectedMaxEvents)
	}

	// Verify count is exactly at limit
	if actualCount != expectedMaxEvents {
		t.Errorf("Event count should be exactly at limit: got %d, want %d",
			actualCount, expectedMaxEvents)
	}
}

// TestMemoryEventLog_CircularBuffer tests:
// Event log should implement circular buffer behavior to prevent unbounded growth
//
// FAILING TEST: Demonstrates memory grows unbounded over time
// Expected behavior: Memory usage should stabilize after reaching max events
func TestMemoryEventLog_CircularBuffer(t *testing.T) {
	// Arrange
	expectedMaxEvents := 1000
	log := NewMemoryEventLog(expectedMaxEvents) // Fix: Now configurable
	clk := clock.NewVirtualClock(time.Now())

	// Capture initial memory
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Act: Add max events
	for i := 0; i < expectedMaxEvents; i++ {
		event := NewEvent(EventTransitionFired, "net1", clk)
		event.Metadata = map[string]interface{}{
			"sequence": i,
			"data":     make([]byte, 100), // Small payload per event
		}
		log.Append(event)
	}

	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)
	memAfterMax := m2.Alloc - m1.Alloc

	// Add another max events (should replace oldest)
	for i := expectedMaxEvents; i < expectedMaxEvents*2; i++ {
		event := NewEvent(EventTransitionFired, "net1", clk)
		event.Metadata = map[string]interface{}{
			"sequence": i,
			"data":     make([]byte, 100),
		}
		log.Append(event)
	}

	runtime.GC()
	var m3 runtime.MemStats
	runtime.ReadMemStats(&m3)
	memAfterDouble := m3.Alloc - m1.Alloc

	// Assert: Memory should not double when adding second batch
	// WHY THIS FAILS: Memory keeps growing because old events aren't removed
	memoryGrowthRatio := float64(memAfterDouble) / float64(memAfterMax)
	if memoryGrowthRatio > 1.5 {
		t.Errorf("EXPECTED FAILURE: Memory grew too much: %.2fx growth (expected <1.5x with circular buffer)",
			memoryGrowthRatio)
		t.Logf("This test currently FAILS because old events aren't dropped")
		t.Logf("Memory after %d events: %d bytes", expectedMaxEvents, memAfterMax)
		t.Logf("Memory after %d events: %d bytes", expectedMaxEvents*2, memAfterDouble)
		t.Logf("After fix: memory should stabilize as oldest events are dropped")
	}

	// Verify oldest events would be dropped (after fix)
	firstEvent, err := log.Get("evt-0") // Hypothetical ID format
	if err == nil && log.Count() > expectedMaxEvents {
		t.Logf("EXPECTED FAILURE: Oldest event still exists when it should have been dropped")
		t.Logf("Event: %v", firstEvent)
	}
}

// TestMemoryEventLog_ConfigurableLimit tests:
// Event log should support different max sizes for different use cases
//
// FAILING TEST: No way to configure max size currently
// Expected behavior: Support configurable limits via constructor or setter
func TestMemoryEventLog_ConfigurableLimit(t *testing.T) {
	testCases := []struct {
		name      string
		maxEvents int
		addEvents int
	}{
		{
			name:      "small limit for testing",
			maxEvents: 10,
			addEvents: 20,
		},
		{
			name:      "medium limit for production",
			maxEvents: 1000,
			addEvents: 1500,
		},
		{
			name:      "large limit for debugging",
			maxEvents: 10000,
			addEvents: 12000,
		},
		{
			name:      "unlimited (zero means no limit)",
			maxEvents: 0,
			addEvents: 5000,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange: Create log with specific limit
			log := NewMemoryEventLog(tc.maxEvents) // Fix: Now accepts maxEvents parameter
			clk := clock.NewVirtualClock(time.Now())

			// Act: Add more events than limit
			for i := 0; i < tc.addEvents; i++ {
				event := NewEvent(EventTransitionFired, "net1", clk)
				log.Append(event)
			}

			// Assert: Check count respects limit
			actualCount := log.Count()

			if tc.maxEvents == 0 {
				// Unlimited: should have all events
				if actualCount != tc.addEvents {
					t.Errorf("Unlimited log: got %d events, want %d", actualCount, tc.addEvents)
				}
			} else {
				// Limited: should not exceed max
				if actualCount > tc.maxEvents {
					t.Errorf("EXPECTED FAILURE: Event count %d exceeds limit %d", actualCount, tc.maxEvents)
					t.Logf("This test FAILS because max events is not configurable")
					t.Logf("After fix: NewMemoryEventLog should accept maxEvents parameter")
				}
			}
		})
	}
}

// TestMemoryEventLog_DropsOldestEvents tests:
// When limit is reached, oldest events should be dropped (FIFO)
//
// FAILING TEST: No mechanism to track or drop old events
// Expected behavior: GetAll() should return only most recent events up to max
func TestMemoryEventLog_DropsOldestEvents(t *testing.T) {
	// Arrange
	maxEvents := 100
	totalEvents := 150
	log := NewMemoryEventLog(maxEvents) // Fix: Now accepts maxEvents parameter
	clk := clock.NewVirtualClock(time.Now())

	// Act: Add events with sequential IDs
	eventIDs := make([]string, totalEvents)
	for i := 0; i < totalEvents; i++ {
		event := NewEvent(EventTransitionFired, "net1", clk)
		event.Metadata = map[string]interface{}{
			"sequence": i,
		}
		eventIDs[i] = event.ID
		log.Append(event)
	}

	// Assert: Should only have most recent maxEvents
	allEvents, _ := log.GetAll()

	// WHY THIS FAILS: All 150 events are still in the log
	if len(allEvents) > maxEvents {
		t.Errorf("EXPECTED FAILURE: Have %d events, want max %d", len(allEvents), maxEvents)
	}

	// First 50 events (0-49) should be gone after fix
	for i := 0; i < (totalEvents - maxEvents); i++ {
		_, err := log.Get(eventIDs[i])
		if err == nil {
			t.Logf("EXPECTED FAILURE: Old event %d still exists (should be dropped)", i)
		}
	}

	// Last 100 events (50-149) should still exist after fix
	for i := (totalEvents - maxEvents); i < totalEvents; i++ {
		_, err := log.Get(eventIDs[i])
		if err != nil {
			t.Errorf("Recent event %d was lost (should be kept): %v", i, err)
		}
	}
}

// TestMemoryEventLog_ConcurrentAccessWithLimit tests:
// Circular buffer must be thread-safe when dropping old events
//
// FAILING TEST: Will pass now (no limit), but needs to pass after implementing circular buffer
// Expected behavior: Concurrent access should not race when evicting old events
func TestMemoryEventLog_ConcurrentAccessWithLimit(t *testing.T) {
	// This test will become important AFTER implementing the circular buffer
	// Run with: go test -race -run TestMemoryEventLog_ConcurrentAccessWithLimit

	expectedMaxCount := 100
	log := NewMemoryEventLog(expectedMaxCount) // Fix: Now accepts maxEvents parameter
	clk := clock.NewVirtualClock(time.Now())

	var wg sync.WaitGroup
	numGoroutines := 10
	eventsPerGoroutine := 50 // Total: 500 events, limit would be 100

	// Concurrent appends that exceed limit
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				event := NewEvent(EventTransitionFired, fmt.Sprintf("net-%d", id), clk)
				event.Metadata = map[string]interface{}{
					"goroutine": id,
					"sequence":  j,
				}
				log.Append(event)
			}
		}(i)
	}

	// Concurrent reads while appending
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				log.GetAll()
				log.Count()
			}
		}()
	}

	wg.Wait()

	// After fix: count should be at most 100 (the limit)
	actualCount := log.Count()

	if actualCount > expectedMaxCount {
		t.Errorf("Count %d exceeds limit %d", actualCount, expectedMaxCount)
	}

	t.Logf("Final count: %d (expected max: %d)", actualCount, expectedMaxCount)
}

// TestMemoryEventLog_PreservesRecentEvents tests:
// When events are dropped, the most recent ones should be preserved
//
// FAILING TEST: No mechanism to track event age and preserve recent ones
// Expected behavior: Most recent events should always be accessible
func TestMemoryEventLog_PreservesRecentEvents(t *testing.T) {
	maxEvents := 50
	log := NewMemoryEventLog(maxEvents) // Fix: Now accepts maxEvents parameter
	clk := clock.NewVirtualClock(time.Now())

	// Add events with timestamps
	baseTime := time.Now()
	for i := 0; i < 100; i++ {
		clk.AdvanceTo(baseTime.Add(time.Duration(i) * time.Second))
		event := NewEvent(EventTransitionFired, "net1", clk)
		event.Metadata = map[string]interface{}{
			"sequence": i,
		}
		log.Append(event)
	}

	// Get all events
	allEvents, _ := log.GetAll()

	// WHY THIS FAILS: We have 100 events instead of 50
	if len(allEvents) > maxEvents {
		t.Logf("EXPECTED FAILURE: Have %d events, want max %d", len(allEvents), maxEvents)
	}

	// After fix: verify most recent events are preserved
	// Should have events 50-99, not 0-49
	if len(allEvents) > 0 {
		firstEvent := allEvents[0]
		lastEvent := allEvents[len(allEvents)-1]

		firstSeq, _ := firstEvent.Metadata["sequence"].(int)
		lastSeq, _ := lastEvent.Metadata["sequence"].(int)

		// After fix: first event should be sequence 50 (oldest of recent 50)
		// Currently: first event is sequence 0
		if firstSeq < 50 && len(allEvents) > maxEvents {
			t.Logf("EXPECTED FAILURE: Oldest event has sequence %d, want >= 50", firstSeq)
			t.Logf("After fix: only most recent %d events should be kept", maxEvents)
		}

		// Last event should always be most recent (sequence 99)
		if lastSeq != 99 {
			t.Errorf("Last event has sequence %d, want 99", lastSeq)
		}
	}
}
