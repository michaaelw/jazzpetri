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
	stdContext "context"
	"testing"
	"time"

	"github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/event"
)

func TestEventTrigger_ShouldFire_EventSystemNotSet(t *testing.T) {
	trigger := &EventTrigger{}

	ctx := context.NewExecutionContext(stdContext.Background(), nil, nil)

	shouldFire, err := trigger.ShouldFire(ctx)
	if err == nil {
		t.Fatal("expected error for EventSystem not set, got nil")
	}
	if shouldFire {
		t.Error("expected shouldFire to be false")
	}
}

func TestEventTrigger_ShouldFire_NotReady(t *testing.T) {
	eventSys := event.NewInMemoryEventSystem()
	defer eventSys.Close()

	trigger := &EventTrigger{
		EventSystem: eventSys,
		EventType:   "test.event",
	}

	ctx := context.NewExecutionContext(stdContext.Background(), nil, nil)

	shouldFire, err := trigger.ShouldFire(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if shouldFire {
		t.Error("expected shouldFire to be false before event received")
	}
}

func TestEventTrigger_AwaitFiring_EventSystemNotSet(t *testing.T) {
	trigger := &EventTrigger{}

	ctx := context.NewExecutionContext(stdContext.Background(), nil, nil)

	err := trigger.AwaitFiring(ctx)
	if err == nil {
		t.Fatal("expected error for EventSystem not set, got nil")
	}
}

func TestEventTrigger_AwaitFiring_EventTypeNotSet(t *testing.T) {
	eventSys := event.NewInMemoryEventSystem()
	defer eventSys.Close()

	trigger := &EventTrigger{
		EventSystem: eventSys,
	}

	ctx := context.NewExecutionContext(stdContext.Background(), nil, nil)

	err := trigger.AwaitFiring(ctx)
	if err == nil {
		t.Fatal("expected error for EventType not set, got nil")
	}
}

func TestEventTrigger_AwaitFiring_Success(t *testing.T) {
	eventSys := event.NewInMemoryEventSystem()
	defer eventSys.Close()

	trigger := &EventTrigger{
		EventSystem: eventSys,
		EventType:   "test.event",
	}

	ctx := context.NewExecutionContext(stdContext.Background(), nil, nil)

	// Start waiting in goroutine
	done := make(chan error, 1)
	go func() {
		done <- trigger.AwaitFiring(ctx)
	}()

	// Give time for subscription to be set up
	time.Sleep(50 * time.Millisecond)

	// Publish matching event
	eventSys.Publish(event.Event{
		Type:   "test.event",
		Source: "test",
	})

	// Wait for AwaitFiring to complete
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for AwaitFiring")
	}

	// Verify trigger is ready
	shouldFire, err := trigger.ShouldFire(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !shouldFire {
		t.Error("expected shouldFire to be true after event received")
	}
}

func TestEventTrigger_AwaitFiring_WithFilter_Match(t *testing.T) {
	eventSys := event.NewInMemoryEventSystem()
	defer eventSys.Close()

	trigger := &EventTrigger{
		EventSystem: eventSys,
		EventType:   "webhook.github.push",
		EventFilter: func(e event.Event) bool {
			// Only fire for jazz3 repository
			data, ok := e.Data.(map[string]interface{})
			if !ok {
				return false
			}
			repo, ok := data["repo"].(string)
			return ok && repo == "jazz3"
		},
	}

	ctx := context.NewExecutionContext(stdContext.Background(), nil, nil)

	// Start waiting
	done := make(chan error, 1)
	go func() {
		done <- trigger.AwaitFiring(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Publish matching event (jazz3 repo)
	eventSys.Publish(event.Event{
		Type:   "webhook.github.push",
		Source: "github",
		Data: map[string]interface{}{
			"repo": "jazz3",
		},
	})

	// Should complete
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for AwaitFiring")
	}
}

func TestEventTrigger_AwaitFiring_WithFilter_NoMatch(t *testing.T) {
	eventSys := event.NewInMemoryEventSystem()
	defer eventSys.Close()

	trigger := &EventTrigger{
		EventSystem: eventSys,
		EventType:   "webhook.github.push",
		EventFilter: func(e event.Event) bool {
			data, ok := e.Data.(map[string]interface{})
			if !ok {
				return false
			}
			repo, ok := data["repo"].(string)
			return ok && repo == "jazz3"
		},
	}

	ctx := context.NewExecutionContext(stdContext.Background(), nil, nil)

	// Start waiting
	done := make(chan error, 1)
	go func() {
		done <- trigger.AwaitFiring(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Publish non-matching event (different repo)
	eventSys.Publish(event.Event{
		Type:   "webhook.github.push",
		Source: "github",
		Data: map[string]interface{}{
			"repo": "other-repo",
		},
	})

	// Give time for non-matching event to be processed
	time.Sleep(100 * time.Millisecond)

	// Should still be waiting (not completed)
	select {
	case <-done:
		t.Fatal("AwaitFiring should not have completed for non-matching event")
	default:
		// Expected - still waiting
	}

	// Publish matching event
	eventSys.Publish(event.Event{
		Type:   "webhook.github.push",
		Source: "github",
		Data: map[string]interface{}{
			"repo": "jazz3",
		},
	})

	// Now should complete
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for matching event")
	}
}

func TestEventTrigger_AwaitFiring_ContextCancellation(t *testing.T) {
	eventSys := event.NewInMemoryEventSystem()
	defer eventSys.Close()

	trigger := &EventTrigger{
		EventSystem: eventSys,
		EventType:   "test.event",
	}

	// Create context with cancellation
	baseCtx, cancel := stdContext.WithCancel(stdContext.Background())
	ctx := context.NewExecutionContext(baseCtx, nil, nil)

	// Start waiting
	done := make(chan error, 1)
	go func() {
		done <- trigger.AwaitFiring(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Cancel context
	cancel()

	// Should complete with context cancellation error
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected context cancellation error, got nil")
		}
		if err != stdContext.Canceled {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for context cancellation")
	}
}

func TestEventTrigger_Cancel(t *testing.T) {
	eventSys := event.NewInMemoryEventSystem()
	defer eventSys.Close()

	trigger := &EventTrigger{
		EventSystem: eventSys,
		EventType:   "test.event",
	}

	ctx := context.NewExecutionContext(stdContext.Background(), nil, nil)

	// Start waiting
	go trigger.AwaitFiring(ctx)

	// Give time for subscription
	time.Sleep(50 * time.Millisecond)

	// Verify subscription exists
	if eventSys.SubscriptionCount() != 1 {
		t.Errorf("expected 1 subscription, got %d", eventSys.SubscriptionCount())
	}

	// Cancel trigger
	err := trigger.Cancel()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify subscription removed
	if eventSys.SubscriptionCount() != 0 {
		t.Errorf("expected 0 subscriptions after cancel, got %d", eventSys.SubscriptionCount())
	}

	// Verify ready flag cleared
	shouldFire, err := trigger.ShouldFire(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if shouldFire {
		t.Error("expected shouldFire to be false after cancel")
	}
}

func TestEventTrigger_Cancel_NoSubscription(t *testing.T) {
	eventSys := event.NewInMemoryEventSystem()
	defer eventSys.Close()

	trigger := &EventTrigger{
		EventSystem: eventSys,
		EventType:   "test.event",
	}

	// Cancel without ever subscribing - should not error
	err := trigger.Cancel()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestEventTrigger_MultipleEvents(t *testing.T) {
	eventSys := event.NewInMemoryEventSystem()
	defer eventSys.Close()

	trigger := &EventTrigger{
		EventSystem: eventSys,
		EventType:   "test.event",
	}

	ctx := context.NewExecutionContext(stdContext.Background(), nil, nil)

	// Start waiting
	done := make(chan error, 1)
	go func() {
		done <- trigger.AwaitFiring(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Publish multiple events - only first should unblock
	for i := 0; i < 3; i++ {
		eventSys.Publish(event.Event{
			Type:   "test.event",
			Source: "test",
		})
		time.Sleep(10 * time.Millisecond)
	}

	// Should complete after first event
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for AwaitFiring")
	}
}

func TestEventTrigger_ConcurrentAwaitFiring(t *testing.T) {
	eventSys := event.NewInMemoryEventSystem()
	defer eventSys.Close()

	trigger := &EventTrigger{
		EventSystem: eventSys,
		EventType:   "test.event",
	}

	ctx := context.NewExecutionContext(stdContext.Background(), nil, nil)

	// Start multiple concurrent waits (engine should only have one, but test thread safety)
	// Note: Only the first waiter will get the first event due to channel size 1
	numWaiters := 3
	done := make(chan error, numWaiters)

	for i := 0; i < numWaiters; i++ {
		go func() {
			done <- trigger.AwaitFiring(ctx)
		}()
	}

	time.Sleep(100 * time.Millisecond)

	// Publish multiple events - one for each waiter
	for i := 0; i < numWaiters; i++ {
		eventSys.Publish(event.Event{
			Type:   "test.event",
			Source: "test",
		})
		time.Sleep(20 * time.Millisecond) // Give time for event to be consumed
	}

	// All waiters should eventually complete
	for i := 0; i < numWaiters; i++ {
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("waiter %d got error: %v", i, err)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for waiter %d", i)
		}
	}
}

func TestEventTrigger_IdempotentSubscription(t *testing.T) {
	eventSys := event.NewInMemoryEventSystem()
	defer eventSys.Close()

	trigger := &EventTrigger{
		EventSystem: eventSys,
		EventType:   "test.event",
	}

	ctx := context.NewExecutionContext(stdContext.Background(), nil, nil)

	// Start first wait
	done1 := make(chan error, 1)
	go func() {
		done1 <- trigger.AwaitFiring(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Verify only one subscription
	if eventSys.SubscriptionCount() != 1 {
		t.Errorf("expected 1 subscription, got %d", eventSys.SubscriptionCount())
	}

	// Publish event
	eventSys.Publish(event.Event{
		Type:   "test.event",
		Source: "test",
	})

	// Wait for completion
	<-done1

	// Start second wait - should reuse subscription or create new one
	done2 := make(chan error, 1)
	go func() {
		done2 <- trigger.AwaitFiring(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Publish another event
	eventSys.Publish(event.Event{
		Type:   "test.event",
		Source: "test",
	})

	// Wait for completion
	select {
	case err := <-done2:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for second AwaitFiring")
	}
}
