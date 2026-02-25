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
	"fmt"
	"sync"

	"github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/event"
)

// EventTrigger fires when an external event is received.
// Enables event-driven workflows triggered by webhooks, message queues, or other external systems.
//
// Example - Webhook trigger:
//
//	eventSys := event.NewInMemoryEventSystem()
//	t := NewTransition("on_push", "Process Git Push")
//	t.WithTrigger(&EventTrigger{
//	    EventSystem: eventSys,
//	    EventType:   "webhook.github.push",
//	    EventFilter: func(e event.Event) bool {
//	        return e.Source == "github" && e.Data.(*GitHubPush).Repo == "jazz3"
//	    },
//	})
//
// Example - Kafka message trigger:
//
//	t := NewTransition("on_order", "Process Order")
//	t.WithTrigger(&EventTrigger{
//	    EventSystem: eventSys,
//	    EventType:   "kafka.orders.created",
//	})
//
// Event Flow:
//  1. External system publishes event to EventSystem
//  2. EventSystem routes event to subscribed EventTriggers
//  3. EventTrigger.AwaitFiring() unblocks
//  4. Transition fires
//
// Thread Safety:
// EventTrigger is safe for concurrent calls but expects only one
// AwaitFiring() goroutine at a time (enforced by engine).
type EventTrigger struct {
	// EventSystem is the event system to subscribe to.
	// Required - must be set before using the trigger.
	EventSystem event.EventSystem

	// EventType identifies which events this trigger subscribes to.
	// Examples: "webhook.github.push", "kafka.orders.created"
	EventType string

	// EventFilter is an optional predicate to filter events.
	// If nil, all events of EventType trigger firing.
	// If set, only events where filter returns true trigger firing.
	EventFilter func(e event.Event) bool

	// Internal state
	subscriptionID string
	eventChan      chan event.Event
	ready          bool
	mu             sync.Mutex
}

// ShouldFire returns true if an event matching the filter has been received.
// This is a non-blocking check.
func (t *EventTrigger) ShouldFire(ctx *context.ExecutionContext) (bool, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.EventSystem == nil {
		return false, fmt.Errorf("EventSystem not set on EventTrigger")
	}

	// Check if an event is ready (non-blocking)
	return t.ready, nil
}

// AwaitFiring blocks until a matching event is received.
// Subscribes to the EventSystem and waits for a matching event.
func (t *EventTrigger) AwaitFiring(ctx *context.ExecutionContext) error {
	if t.EventSystem == nil {
		return fmt.Errorf("EventSystem not set on EventTrigger - required for event-driven workflows")
	}

	if t.EventType == "" {
		return fmt.Errorf("EventType not set on EventTrigger")
	}

	// Subscribe to events (idempotent - only subscribe once)
	t.mu.Lock()
	if t.eventChan == nil {
		t.eventChan = make(chan event.Event, 1)

		// Create handler that sends events to our channel
		handler := func(e event.Event) error {
			// Apply filter if set
			if t.EventFilter != nil && !t.EventFilter(e) {
				// Event didn't match filter, ignore
				return nil
			}

			// Send matching event to channel (non-blocking)
			select {
			case t.eventChan <- e:
			default:
				// Channel full, event already pending
			}

			return nil
		}

		// Subscribe to EventSystem
		subID, err := t.EventSystem.Subscribe(t.EventType, handler)
		if err != nil {
			t.mu.Unlock()
			return fmt.Errorf("failed to subscribe to event type %q: %w", t.EventType, err)
		}
		t.subscriptionID = subID
	}
	eventChan := t.eventChan
	t.mu.Unlock()

	// Wait for matching event
	select {
	case <-eventChan:
		// Matching event received
		t.mu.Lock()
		t.ready = true
		t.mu.Unlock()
		return nil
	case <-ctx.Context.Done():
		// Context cancelled
		return ctx.Context.Err()
	}
}

// Cancel unsubscribes from the EventSystem and cleans up resources.
func (t *EventTrigger) Cancel() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Unsubscribe from EventSystem
	if t.subscriptionID != "" && t.EventSystem != nil {
		if err := t.EventSystem.Unsubscribe(t.subscriptionID); err != nil {
			return fmt.Errorf("failed to unsubscribe: %w", err)
		}
		t.subscriptionID = ""
	}

	// Close and clear channel
	if t.eventChan != nil {
		close(t.eventChan)
		t.eventChan = nil
	}

	t.ready = false

	return nil
}
