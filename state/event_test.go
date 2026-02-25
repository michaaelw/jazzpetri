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
	"testing"
	"time"

	"github.com/jazzpetri/engine/clock"
)

func TestNewEvent(t *testing.T) {
	t.Run("creates event with auto-generated ID", func(t *testing.T) {
		clk := clock.NewVirtualClock(time.Now())
		event := NewEvent(EventTransitionFired, "net1", clk)

		if event == nil {
			t.Fatal("expected event, got nil")
		}
		if event.ID == "" {
			t.Error("expected event ID to be generated")
		}
		if event.Type != EventTransitionFired {
			t.Errorf("expected type %s, got %s", EventTransitionFired, event.Type)
		}
		if event.NetID != "net1" {
			t.Errorf("expected netID net1, got %s", event.NetID)
		}
		if event.Timestamp.IsZero() {
			t.Error("expected timestamp to be set")
		}
	})

	t.Run("creates events with unique IDs", func(t *testing.T) {
		clk := clock.NewVirtualClock(time.Now())
		event1 := NewEvent(EventTransitionFired, "net1", clk)
		event2 := NewEvent(EventTransitionFired, "net1", clk)

		if event1.ID == event2.ID {
			t.Error("expected unique event IDs")
		}
	})

	t.Run("uses clock for timestamp", func(t *testing.T) {
		baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
		clk := clock.NewVirtualClock(baseTime)
		event := NewEvent(EventTransitionFired, "net1", clk)

		if !event.Timestamp.Equal(baseTime) {
			t.Errorf("expected timestamp %v, got %v", baseTime, event.Timestamp)
		}
	})
}

func TestEventTypes(t *testing.T) {
	// Verify all event types are defined correctly
	types := []EventType{
		EventTransitionFired,
		EventTransitionEnabled,
		EventTransitionFailed,
		EventEngineStarted,
		EventEngineStopped,
		EventTokenProduced,
		EventTokenConsumed,
		EventErrorOccurred,
	}

	for _, typ := range types {
		if string(typ) == "" {
			t.Errorf("event type %v has empty string value", typ)
		}
	}
}

func TestEventMetadata(t *testing.T) {
	t.Run("metadata can be set and retrieved", func(t *testing.T) {
		clk := clock.NewVirtualClock(time.Now())
		event := NewEvent(EventTransitionFired, "net1", clk)
		event.Metadata = map[string]interface{}{
			"key1": "value1",
			"key2": 42,
		}

		if event.Metadata["key1"] != "value1" {
			t.Error("metadata key1 not set correctly")
		}
		if event.Metadata["key2"] != 42 {
			t.Error("metadata key2 not set correctly")
		}
	})

	t.Run("metadata is initialized as empty map", func(t *testing.T) {
		clk := clock.NewVirtualClock(time.Now())
		event := NewEvent(EventTransitionFired, "net1", clk)

		if event.Metadata == nil {
			t.Error("expected metadata to be initialized")
		}
		if len(event.Metadata) != 0 {
			t.Error("expected metadata to be empty")
		}
	})
}

func TestEventFields(t *testing.T) {
	t.Run("transition event fields", func(t *testing.T) {
		clk := clock.NewVirtualClock(time.Now())
		event := NewEvent(EventTransitionFired, "net1", clk)
		event.TransitionID = "t1"

		if event.TransitionID != "t1" {
			t.Error("transition ID not set correctly")
		}
	})

	t.Run("token event fields", func(t *testing.T) {
		clk := clock.NewVirtualClock(time.Now())
		event := NewEvent(EventTokenProduced, "net1", clk)
		event.PlaceID = "p1"
		event.TokenID = "tok1"

		if event.PlaceID != "p1" {
			t.Error("place ID not set correctly")
		}
		if event.TokenID != "tok1" {
			t.Error("token ID not set correctly")
		}
	})

	t.Run("error event fields", func(t *testing.T) {
		clk := clock.NewVirtualClock(time.Now())
		event := NewEvent(EventErrorOccurred, "net1", clk)
		event.Error = "test error"

		if event.Error != "test error" {
			t.Error("error field not set correctly")
		}
	})
}
