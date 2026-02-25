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

// Package event provides event-driven architecture support for Jazz3.
//
// The event package implements pub/sub patterns to enable:
//   - Event-driven workflow execution
//   - Webhook integration
//   - External system notifications
//   - Asynchronous event processing
//
// Example usage:
//
//	// Create event system
//	eventSys := event.NewInMemoryEventSystem()
//
//	// Subscribe to events
//	handler := func(e event.Event) error {
//	    fmt.Printf("Received: %s\n", e.Type)
//	    return nil
//	}
//	subID, _ := eventSys.Subscribe("order.created", handler)
//
//	// Publish events
//	eventSys.Publish(event.Event{
//	    Type:   "order.created",
//	    Source: "order-service",
//	    Data:   orderData,
//	})
package event

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Event represents an event in the system.
// Events can be published by external systems (webhooks), timers,
// or internal Jazz3 components.
type Event struct {
	// ID is a unique identifier for this event
	ID string

	// Type is the event type (e.g., "order.created", "webhook.github.push")
	Type string

	// Source identifies where the event originated
	Source string

	// Timestamp when the event occurred
	Timestamp time.Time

	// Data contains the event payload
	Data interface{}

	// Metadata for additional event information
	Metadata map[string]interface{}
}

// EventHandler processes an event.
// Handlers should be idempotent as events may be delivered multiple times.
type EventHandler func(event Event) error

// EventSystem manages event subscription and publishing.
// This is the port in the Ports & Adapters pattern.
type EventSystem interface {
	// Subscribe registers a handler for events of the specified type.
	// Returns a subscription ID that can be used to unsubscribe.
	// Use "*" as the event type to subscribe to all events.
	Subscribe(eventType string, handler EventHandler) (string, error)

	// Unsubscribe removes a subscription by ID.
	Unsubscribe(subscriptionID string) error

	// Publish sends an event to all matching subscribers.
	// Events are delivered asynchronously.
	Publish(event Event) error

	// PublishSync sends an event synchronously to all matching subscribers.
	// Blocks until all handlers complete or context is cancelled.
	PublishSync(ctx context.Context, event Event) error

	// Close shuts down the event system and stops all handlers.
	Close() error
}

// subscription represents a single event subscription.
type subscription struct {
	id        string
	eventType string
	handler   EventHandler
}

// InMemoryEventSystem is an in-memory implementation of EventSystem.
// This is the simple adapter for MVP - suitable for single-instance deployments.
//
// For distributed systems, implement adapters for:
//   - Kafka (high-throughput, persistent)
//   - NATS (low-latency, cloud-native)
//   - Redis Pub/Sub (simple, fast)
//   - RabbitMQ (feature-rich, enterprise)
type InMemoryEventSystem struct {
	subscriptions map[string]*subscription
	eventChan     chan Event
	done          chan struct{}
	wg            sync.WaitGroup
	mu            sync.RWMutex
	running       bool
}

// NewInMemoryEventSystem creates a new in-memory event system.
// The event system starts processing events immediately.
func NewInMemoryEventSystem() *InMemoryEventSystem {
	es := &InMemoryEventSystem{
		subscriptions: make(map[string]*subscription),
		eventChan:     make(chan Event, 100), // Buffered channel for async delivery
		done:          make(chan struct{}),
		running:       true,
	}

	// Start event processing goroutine
	es.wg.Add(1)
	go es.processEvents()

	return es
}

// Subscribe registers a handler for events of the specified type.
func (es *InMemoryEventSystem) Subscribe(eventType string, handler EventHandler) (string, error) {
	if handler == nil {
		return "", fmt.Errorf("handler cannot be nil")
	}
	if eventType == "" {
		return "", fmt.Errorf("event type cannot be empty")
	}

	es.mu.Lock()
	defer es.mu.Unlock()

	// Generate subscription ID
	subID := fmt.Sprintf("sub-%d", time.Now().UnixNano())

	sub := &subscription{
		id:        subID,
		eventType: eventType,
		handler:   handler,
	}

	es.subscriptions[subID] = sub

	return subID, nil
}

// Unsubscribe removes a subscription by ID.
func (es *InMemoryEventSystem) Unsubscribe(subscriptionID string) error {
	es.mu.Lock()
	defer es.mu.Unlock()

	if _, exists := es.subscriptions[subscriptionID]; !exists {
		return fmt.Errorf("subscription %q not found", subscriptionID)
	}

	delete(es.subscriptions, subscriptionID)
	return nil
}

// Publish sends an event to all matching subscribers asynchronously.
func (es *InMemoryEventSystem) Publish(event Event) error {
	es.mu.RLock()
	running := es.running
	es.mu.RUnlock()

	if !running {
		return fmt.Errorf("event system is not running")
	}

	// Set timestamp if not set
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Set ID if not set
	if event.ID == "" {
		event.ID = fmt.Sprintf("evt-%d", time.Now().UnixNano())
	}

	// Send to event channel (non-blocking if channel is full)
	select {
	case es.eventChan <- event:
		return nil
	default:
		return fmt.Errorf("event channel full, event dropped")
	}
}

// PublishSync sends an event synchronously to all matching subscribers.
func (es *InMemoryEventSystem) PublishSync(ctx context.Context, event Event) error {
	es.mu.RLock()
	running := es.running
	es.mu.RUnlock()

	if !running {
		return fmt.Errorf("event system is not running")
	}

	// Set timestamp if not set
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Set ID if not set
	if event.ID == "" {
		event.ID = fmt.Sprintf("evt-%d", time.Now().UnixNano())
	}

	// Find matching subscriptions
	es.mu.RLock()
	subs := es.findMatchingSubscriptions(event.Type)
	es.mu.RUnlock()

	// Call handlers synchronously
	var errs []error
	for _, sub := range subs {
		if err := sub.handler(event); err != nil {
			errs = append(errs, fmt.Errorf("handler %s failed: %w", sub.id, err))
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("handler errors: %v", errs)
	}

	return nil
}

// Close shuts down the event system.
func (es *InMemoryEventSystem) Close() error {
	es.mu.Lock()
	if !es.running {
		es.mu.Unlock()
		return fmt.Errorf("event system already closed")
	}
	es.running = false
	es.mu.Unlock()

	close(es.done)
	es.wg.Wait()

	return nil
}

// processEvents processes events from the event channel.
func (es *InMemoryEventSystem) processEvents() {
	defer es.wg.Done()

	for {
		select {
		case event := <-es.eventChan:
			es.deliverEvent(event)
		case <-es.done:
			// Drain remaining events
			for {
				select {
				case event := <-es.eventChan:
					es.deliverEvent(event)
				default:
					return
				}
			}
		}
	}
}

// deliverEvent delivers an event to all matching subscribers.
func (es *InMemoryEventSystem) deliverEvent(event Event) {
	es.mu.RLock()
	subs := es.findMatchingSubscriptions(event.Type)
	es.mu.RUnlock()

	// Call handlers concurrently
	var wg sync.WaitGroup
	for _, sub := range subs {
		wg.Add(1)
		go func(s *subscription) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					// Handler panicked - log but continue
					fmt.Printf("Handler %s panicked: %v\n", s.id, r)
				}
			}()

			// Call handler with timeout
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			done := make(chan error, 1)
			go func() {
				defer func() {
					if r := recover(); r != nil {
						// Handler panicked in execution
						done <- fmt.Errorf("handler panicked: %v", r)
					}
				}()
				done <- s.handler(event)
			}()

			select {
			case err := <-done:
				if err != nil {
					// Handler returned error - log but continue
					fmt.Printf("Handler %s failed: %v\n", s.id, err)
				}
			case <-ctx.Done():
				// Handler timed out
				fmt.Printf("Handler %s timed out\n", s.id)
			}
		}(sub)
	}

	wg.Wait()
}

// findMatchingSubscriptions returns all subscriptions matching the event type.
// Must be called with read lock held.
func (es *InMemoryEventSystem) findMatchingSubscriptions(eventType string) []*subscription {
	var matching []*subscription

	for _, sub := range es.subscriptions {
		// Match wildcard or exact type
		if sub.eventType == "*" || sub.eventType == eventType {
			matching = append(matching, sub)
		}
	}

	return matching
}

// SubscriptionCount returns the number of active subscriptions.
// Useful for testing and monitoring.
func (es *InMemoryEventSystem) SubscriptionCount() int {
	es.mu.RLock()
	defer es.mu.RUnlock()
	return len(es.subscriptions)
}
