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

package event

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestEvent_Fields(t *testing.T) {
	now := time.Now()
	event := Event{
		ID:        "evt-123",
		Type:      "test.event",
		Source:    "test-source",
		Timestamp: now,
		Data:      map[string]interface{}{"key": "value"},
		Metadata:  map[string]interface{}{"meta": "data"},
	}

	if event.ID != "evt-123" {
		t.Errorf("expected ID 'evt-123', got %q", event.ID)
	}
	if event.Type != "test.event" {
		t.Errorf("expected Type 'test.event', got %q", event.Type)
	}
	if event.Source != "test-source" {
		t.Errorf("expected Source 'test-source', got %q", event.Source)
	}
	if !event.Timestamp.Equal(now) {
		t.Errorf("timestamp mismatch")
	}
}

func TestInMemoryEventSystem_Subscribe(t *testing.T) {
	es := NewInMemoryEventSystem()
	defer es.Close()

	handler := func(e Event) error {
		return nil
	}

	// Subscribe successfully
	subID, err := es.Subscribe("test.event", handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if subID == "" {
		t.Error("expected non-empty subscription ID")
	}

	// Verify subscription count
	if es.SubscriptionCount() != 1 {
		t.Errorf("expected 1 subscription, got %d", es.SubscriptionCount())
	}
}

func TestInMemoryEventSystem_Subscribe_NilHandler(t *testing.T) {
	es := NewInMemoryEventSystem()
	defer es.Close()

	_, err := es.Subscribe("test.event", nil)
	if err == nil {
		t.Fatal("expected error for nil handler, got nil")
	}
}

func TestInMemoryEventSystem_Subscribe_EmptyEventType(t *testing.T) {
	es := NewInMemoryEventSystem()
	defer es.Close()

	handler := func(e Event) error { return nil }

	_, err := es.Subscribe("", handler)
	if err == nil {
		t.Fatal("expected error for empty event type, got nil")
	}
}

func TestInMemoryEventSystem_Unsubscribe(t *testing.T) {
	es := NewInMemoryEventSystem()
	defer es.Close()

	handler := func(e Event) error { return nil }

	subID, err := es.Subscribe("test.event", handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Unsubscribe successfully
	err = es.Unsubscribe(subID)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if es.SubscriptionCount() != 0 {
		t.Errorf("expected 0 subscriptions, got %d", es.SubscriptionCount())
	}
}

func TestInMemoryEventSystem_Unsubscribe_NotFound(t *testing.T) {
	es := NewInMemoryEventSystem()
	defer es.Close()

	err := es.Unsubscribe("non-existent")
	if err == nil {
		t.Error("expected error for non-existent subscription, got nil")
	}
}

func TestInMemoryEventSystem_Publish(t *testing.T) {
	es := NewInMemoryEventSystem()
	defer es.Close()

	// Channel to signal handler was called
	called := make(chan Event, 1)

	handler := func(e Event) error {
		called <- e
		return nil
	}

	_, err := es.Subscribe("test.event", handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Publish event
	event := Event{
		Type:   "test.event",
		Source: "test",
		Data:   "test-data",
	}

	err = es.Publish(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wait for handler to be called
	select {
	case received := <-called:
		if received.Type != "test.event" {
			t.Errorf("expected type 'test.event', got %q", received.Type)
		}
		if received.Source != "test" {
			t.Errorf("expected source 'test', got %q", received.Source)
		}
		// Verify ID and Timestamp were auto-set
		if received.ID == "" {
			t.Error("expected ID to be auto-set")
		}
		if received.Timestamp.IsZero() {
			t.Error("expected Timestamp to be auto-set")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for handler to be called")
	}
}

func TestInMemoryEventSystem_Publish_NoMatchingSubscriptions(t *testing.T) {
	es := NewInMemoryEventSystem()
	defer es.Close()

	handler := func(e Event) error { return nil }
	_, err := es.Subscribe("other.event", handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Publish event with different type
	event := Event{
		Type:   "test.event",
		Source: "test",
	}

	err = es.Publish(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Give time for async delivery (should not call handler)
	time.Sleep(100 * time.Millisecond)
}

func TestInMemoryEventSystem_Publish_WildcardSubscription(t *testing.T) {
	es := NewInMemoryEventSystem()
	defer es.Close()

	called := make(chan Event, 1)
	handler := func(e Event) error {
		called <- e
		return nil
	}

	// Subscribe to all events with wildcard
	_, err := es.Subscribe("*", handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Publish event
	event := Event{
		Type:   "any.event",
		Source: "test",
	}

	err = es.Publish(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wait for handler to be called
	select {
	case received := <-called:
		if received.Type != "any.event" {
			t.Errorf("expected type 'any.event', got %q", received.Type)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for wildcard handler")
	}
}

func TestInMemoryEventSystem_PublishSync(t *testing.T) {
	es := NewInMemoryEventSystem()
	defer es.Close()

	var callCount atomic.Int32
	handler := func(e Event) error {
		callCount.Add(1)
		return nil
	}

	_, err := es.Subscribe("test.event", handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Publish synchronously
	event := Event{
		Type:   "test.event",
		Source: "test",
	}

	ctx := context.Background()
	err = es.PublishSync(ctx, event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Handler should be called synchronously
	if callCount.Load() != 1 {
		t.Errorf("expected 1 call, got %d", callCount.Load())
	}
}

func TestInMemoryEventSystem_PublishSync_ContextCancellation(t *testing.T) {
	es := NewInMemoryEventSystem()
	defer es.Close()

	handler := func(e Event) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	}

	_, err := es.Subscribe("test.event", handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Create context with immediate cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	event := Event{
		Type:   "test.event",
		Source: "test",
	}

	err = es.PublishSync(ctx, event)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestInMemoryEventSystem_MultipleSubscribers(t *testing.T) {
	es := NewInMemoryEventSystem()
	defer es.Close()

	done := make(chan bool, 3)

	// Subscribe multiple handlers to same event type
	for i := 0; i < 3; i++ {
		handler := func(e Event) error {
			done <- true
			return nil
		}
		_, err := es.Subscribe("test.event", handler)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	// Publish event
	event := Event{
		Type:   "test.event",
		Source: "test",
	}

	err := es.Publish(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wait for all 3 handlers to be called
	for i := 0; i < 3; i++ {
		select {
		case <-done:
			// Handler called
		case <-time.After(1 * time.Second):
			t.Fatalf("timeout waiting for handler %d", i+1)
		}
	}
}

func TestInMemoryEventSystem_ConcurrentPublish(t *testing.T) {
	es := NewInMemoryEventSystem()
	defer es.Close()

	var received atomic.Int32
	handler := func(e Event) error {
		received.Add(1)
		return nil
	}

	_, err := es.Subscribe("test.event", handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Publish events concurrently
	numEvents := 100
	var wg sync.WaitGroup
	wg.Add(numEvents)

	for i := 0; i < numEvents; i++ {
		go func() {
			defer wg.Done()
			event := Event{
				Type:   "test.event",
				Source: "test",
			}
			es.Publish(event)
		}()
	}

	wg.Wait()

	// Wait for all events to be processed
	time.Sleep(500 * time.Millisecond)

	// Should receive all events
	if received.Load() != int32(numEvents) {
		t.Errorf("expected %d events, got %d", numEvents, received.Load())
	}
}

func TestInMemoryEventSystem_ConcurrentSubscribeUnsubscribe(t *testing.T) {
	es := NewInMemoryEventSystem()
	defer es.Close()

	handler := func(e Event) error {
		return nil
	}

	// Concurrent subscribe/unsubscribe operations
	var wg sync.WaitGroup
	numOps := 50

	// Subscribe
	var mu sync.Mutex
	subIDs := make([]string, numOps)
	wg.Add(numOps)
	for i := 0; i < numOps; i++ {
		go func(idx int) {
			defer wg.Done()
			subID, err := es.Subscribe("test.event", handler)
			if err != nil {
				t.Errorf("subscribe error: %v", err)
				return
			}
			mu.Lock()
			subIDs[idx] = subID
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	// Verify subscription count
	if es.SubscriptionCount() != numOps {
		t.Errorf("expected %d subscriptions, got %d", numOps, es.SubscriptionCount())
	}

	// Unsubscribe
	wg.Add(numOps)
	for i := 0; i < numOps; i++ {
		go func(idx int) {
			defer wg.Done()
			mu.Lock()
			subID := subIDs[idx]
			mu.Unlock()
			if subID != "" {
				es.Unsubscribe(subID)
			}
		}(i)
	}
	wg.Wait()

	// Verify all unsubscribed
	if es.SubscriptionCount() != 0 {
		t.Errorf("expected 0 subscriptions, got %d", es.SubscriptionCount())
	}
}

func TestInMemoryEventSystem_HandlerPanic(t *testing.T) {
	es := NewInMemoryEventSystem()
	defer es.Close()

	normalHandlerCalled := make(chan bool, 1)

	panicHandler := func(e Event) error {
		panic("handler panic")
	}

	normalHandler := func(e Event) error {
		normalHandlerCalled <- true
		return nil
	}

	_, err := es.Subscribe("test.event", panicHandler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = es.Subscribe("test.event", normalHandler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Publish event - should not crash despite panic in one handler
	event := Event{
		Type:   "test.event",
		Source: "test",
	}

	err = es.Publish(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wait for normal handler to be called (verifies panic didn't stop other handlers)
	select {
	case <-normalHandlerCalled:
		// Good - normal handler was called despite panic in other handler
	case <-time.After(1 * time.Second):
		t.Fatal("normal handler was not called - panic may have stopped event processing")
	}

	// System should still be running
	es.mu.RLock()
	running := es.running
	es.mu.RUnlock()

	if !running {
		t.Error("event system should still be running after handler panic")
	}
}

func TestInMemoryEventSystem_Close(t *testing.T) {
	es := NewInMemoryEventSystem()

	handler := func(e Event) error { return nil }
	_, err := es.Subscribe("test.event", handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Close the event system
	err = es.Close()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify system is not running
	if es.running {
		t.Error("event system should not be running after close")
	}

	// Publish after close should fail
	event := Event{
		Type:   "test.event",
		Source: "test",
	}

	err = es.Publish(event)
	if err == nil {
		t.Error("expected error publishing after close, got nil")
	}
}

func TestInMemoryEventSystem_DoubleClose(t *testing.T) {
	es := NewInMemoryEventSystem()

	err := es.Close()
	if err != nil {
		t.Errorf("unexpected error on first close: %v", err)
	}

	// Second close should return error
	err = es.Close()
	if err == nil {
		t.Error("expected error on second close, got nil")
	}
}

func TestInMemoryEventSystem_EventAutoFields(t *testing.T) {
	es := NewInMemoryEventSystem()
	defer es.Close()

	received := make(chan Event, 1)
	handler := func(e Event) error {
		received <- e
		return nil
	}

	_, err := es.Subscribe("test.event", handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Publish event without ID and Timestamp
	event := Event{
		Type:   "test.event",
		Source: "test",
	}

	err = es.Publish(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case e := <-received:
		// Verify ID was auto-generated
		if e.ID == "" {
			t.Error("expected ID to be auto-generated")
		}
		// Verify Timestamp was auto-set
		if e.Timestamp.IsZero() {
			t.Error("expected Timestamp to be auto-set")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestInMemoryEventSystem_EventWithMetadata(t *testing.T) {
	es := NewInMemoryEventSystem()
	defer es.Close()

	received := make(chan Event, 1)
	handler := func(e Event) error {
		received <- e
		return nil
	}

	_, err := es.Subscribe("test.event", handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Publish event with metadata
	event := Event{
		Type:   "test.event",
		Source: "test",
		Data:   "test-data",
		Metadata: map[string]interface{}{
			"priority": "high",
			"retries":  3,
		},
	}

	err = es.Publish(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case e := <-received:
		if e.Metadata["priority"] != "high" {
			t.Errorf("expected priority 'high', got %v", e.Metadata["priority"])
		}
		if e.Metadata["retries"] != 3 {
			t.Errorf("expected retries 3, got %v", e.Metadata["retries"])
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for event")
	}
}
