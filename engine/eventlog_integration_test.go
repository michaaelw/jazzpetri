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

package engine

import (
	stdcontext "context"
	"errors"
	"testing"
	"time"

	"github.com/jazzpetri/engine/clock"
	"github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/petri"
	"github.com/jazzpetri/engine/state"
	"github.com/jazzpetri/engine/task"
	"github.com/jazzpetri/engine/token"
)

// TestEventLogging_WorkflowExecution tests event recording during a complete workflow
func TestEventLogging_WorkflowExecution(t *testing.T) {
	// Create a simple workflow: Start -> Process -> End
	net := petri.NewPetriNet("workflow1", "Test Workflow")

	// Places
	start := petri.NewPlace("start", "Start", 1)
	process := petri.NewPlace("process", "Process", 1)
	end := petri.NewPlace("end", "End", 1)

	// Transitions
	t1 := petri.NewTransition("t1", "Start Processing")
	t2 := petri.NewTransition("t2", "Finish Processing")

	// Arcs
	arc1 := petri.NewArc("a1", "start", "t1", 1)
	arc2 := petri.NewArc("a2", "t1", "process", 1)
	arc3 := petri.NewArc("a3", "process", "t2", 1)
	arc4 := petri.NewArc("a4", "t2", "end", 1)

	// Add to net
	net.AddPlace(start)
	net.AddPlace(process)
	net.AddPlace(end)
	net.AddTransition(t1)
	net.AddTransition(t2)
	net.AddArc(arc1)
	net.AddArc(arc2)
	net.AddArc(arc3)
	net.AddArc(arc4)

	// Add initial token
	tok := &token.Token{
		ID:   "token1",
		Data: &token.IntToken{Value: 42},
	}
	start.AddToken(tok)

	// Resolve net
	if err := net.Resolve(); err != nil {
		t.Fatalf("failed to resolve net: %v", err)
	}

	// Create event log and event builder
	eventLog := state.NewMemoryEventLog()
	clk := clock.NewVirtualClock(time.Now())
	eventBuilder := state.NewDefaultEventBuilder(clk)

	// Create execution context with event logging
	ctx := context.NewExecutionContext(
		stdcontext.Background(),
		clk,
		nil,
	)
	ctx = ctx.WithEventLog(eventLog)
	ctx = ctx.WithEventBuilder(eventBuilder)

	// Create and run engine
	eng := NewEngine(net, ctx, Config{
		Mode:         ModeSequential,
		PollInterval: 10 * time.Millisecond,
	})

	// Start engine
	eng.Start()

	// Wait for workflow to complete
	time.Sleep(100 * time.Millisecond)
	eng.Stop()

	// Verify events were recorded
	events, err := eventLog.GetAll()
	if err != nil {
		t.Fatalf("failed to get events: %v", err)
	}

	// Should have: EngineStarted, TransitionFired (t1), TransitionFired (t2), EngineStopped
	if len(events) < 4 {
		t.Errorf("expected at least 4 events, got %d", len(events))
	}

	// Verify engine started event
	startEvent := events[0]
	if startEvent.Type != state.EventEngineStarted {
		t.Errorf("expected first event to be EngineStarted, got %s", startEvent.Type)
	}
	if startEvent.NetID != "workflow1" {
		t.Errorf("expected netID workflow1, got %s", startEvent.NetID)
	}

	// Verify transition fired events
	firedEvents, _ := eventLog.GetByType(state.EventTransitionFired)
	if len(firedEvents) != 2 {
		t.Errorf("expected 2 transition fired events, got %d", len(firedEvents))
	}

	// Verify t1 fired
	t1Events, _ := eventLog.GetByTransition("t1")
	if len(t1Events) == 0 {
		t.Error("expected events for transition t1")
	}

	// Verify t2 fired
	t2Events, _ := eventLog.GetByTransition("t2")
	if len(t2Events) == 0 {
		t.Error("expected events for transition t2")
	}

	// Verify engine stopped event
	stopEvent := events[len(events)-1]
	if stopEvent.Type != state.EventEngineStopped {
		t.Errorf("expected last event to be EngineStopped, got %s", stopEvent.Type)
	}
}

// TestEventLogging_TransitionFailure tests event recording when a transition fails
func TestEventLogging_TransitionFailure(t *testing.T) {
	net := petri.NewPetriNet("workflow2", "Failing Workflow")

	start := petri.NewPlace("start", "Start", 1)
	end := petri.NewPlace("end", "End", 1)

	// Transition with a task that fails
	t1 := petri.NewTransition("t1", "Failing Task")
	t1.Task = &task.InlineTask{
		Name: "fail-task",
		Fn: func(ctx *context.ExecutionContext) error {
			return errors.New("intentional failure")
		},
	}

	arc1 := petri.NewArc("a1", "start", "t1", 1)
	arc2 := petri.NewArc("a2", "t1", "end", 1)

	net.AddPlace(start)
	net.AddPlace(end)
	net.AddTransition(t1)
	net.AddArc(arc1)
	net.AddArc(arc2)

	tok := &token.Token{
		ID:   "token1",
		Data: &token.IntToken{Value: 42},
	}
	start.AddToken(tok)

	if err := net.Resolve(); err != nil {
		t.Fatalf("failed to resolve net: %v", err)
	}

	// Create event log
	eventLog := state.NewMemoryEventLog()
	clk := clock.NewVirtualClock(time.Now())
	eventBuilder := state.NewDefaultEventBuilder(clk)

	ctx := context.NewExecutionContext(
		stdcontext.Background(),
		clk,
		nil,
	)
	ctx = ctx.WithEventLog(eventLog)
	ctx = ctx.WithEventBuilder(eventBuilder)

	// Fire the transition (will fail)
	err := t1.Fire(ctx)
	if err == nil {
		t.Error("expected transition to fail")
	}

	// Verify failure event was recorded
	failedEvents, _ := eventLog.GetByType(state.EventTransitionFailed)
	if len(failedEvents) != 1 {
		t.Errorf("expected 1 failed event, got %d", len(failedEvents))
	}

	// Verify failure details
	failEvent := failedEvents[0]
	if failEvent.TransitionID != "t1" {
		t.Errorf("expected transition t1, got %s", failEvent.TransitionID)
	}
	if failEvent.Error == "" {
		t.Error("expected error message in event")
	}
	if failEvent.Error != "intentional failure" {
		t.Errorf("expected error 'intentional failure', got '%s'", failEvent.Error)
	}
}

// TestEventLogging_EventOrdering tests that events maintain temporal ordering
func TestEventLogging_EventOrdering(t *testing.T) {
	net := petri.NewPetriNet("workflow3", "Sequential Workflow")

	// Chain of places: p1 -> t1 -> p2 -> t2 -> p3
	p1 := petri.NewPlace("p1", "Place 1", 1)
	p2 := petri.NewPlace("p2", "Place 2", 1)
	p3 := petri.NewPlace("p3", "Place 3", 1)

	t1 := petri.NewTransition("t1", "Transition 1")
	t2 := petri.NewTransition("t2", "Transition 2")

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddPlace(p3)
	net.AddTransition(t1)
	net.AddTransition(t2)

	net.AddArc(petri.NewArc("a1", "p1", "t1", 1))
	net.AddArc(petri.NewArc("a2", "t1", "p2", 1))
	net.AddArc(petri.NewArc("a3", "p2", "t2", 1))
	net.AddArc(petri.NewArc("a4", "t2", "p3", 1))

	tok := &token.Token{
		ID:   "token1",
		Data: &token.IntToken{Value: 1},
	}
	p1.AddToken(tok)

	net.Resolve()

	// Create event log with virtual clock
	eventLog := state.NewMemoryEventLog()
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(baseTime)
	eventBuilder := state.NewDefaultEventBuilder(clk)

	ctx := context.NewExecutionContext(
		stdcontext.Background(),
		clk,
		nil,
	)
	ctx = ctx.WithEventLog(eventLog)
	ctx = ctx.WithEventBuilder(eventBuilder)

	// Fire t1
	clk.AdvanceTo(baseTime.Add(1 * time.Second))
	t1.Fire(ctx)

	// Fire t2
	clk.AdvanceTo(baseTime.Add(2 * time.Second))
	t2.Fire(ctx)

	// Get all events
	events, _ := eventLog.GetAll()

	// Verify ordering
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}

	// Events should be in temporal order
	for i := 1; i < len(events); i++ {
		if events[i].Timestamp.Before(events[i-1].Timestamp) {
			t.Errorf("events not in temporal order: event %d timestamp %v < event %d timestamp %v",
				i, events[i].Timestamp, i-1, events[i-1].Timestamp)
		}
	}

	// Verify t1 fired before t2
	t1Events, _ := eventLog.GetByTransition("t1")
	t2Events, _ := eventLog.GetByTransition("t2")

	if len(t1Events) == 0 || len(t2Events) == 0 {
		t.Fatal("expected events for both transitions")
	}

	if t2Events[0].Timestamp.Before(t1Events[0].Timestamp) {
		t.Error("t2 should fire after t1")
	}
}

// TestEventLogging_QuerySince tests filtering events by timestamp
func TestEventLogging_QuerySince(t *testing.T) {
	eventLog := state.NewMemoryEventLog()
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(baseTime)

	// Add events at different times
	for i := 0; i < 5; i++ {
		clk.AdvanceTo(baseTime.Add(time.Duration(i) * time.Hour))
		event := state.NewEvent(state.EventTransitionFired, "net1", clk)
		event.TransitionID = "t1"
		eventLog.Append(event)
	}

	// Query events since 2 hours
	cutoff := baseTime.Add(2 * time.Hour)
	recentEvents, err := eventLog.GetSince(cutoff)
	if err != nil {
		t.Fatalf("failed to query events: %v", err)
	}

	// Should get events at 3 and 4 hours (2 events)
	if len(recentEvents) != 2 {
		t.Errorf("expected 2 recent events, got %d", len(recentEvents))
	}

	// Verify all returned events are after cutoff
	for _, e := range recentEvents {
		if !e.Timestamp.After(cutoff) {
			t.Errorf("event timestamp %v is not after cutoff %v", e.Timestamp, cutoff)
		}
	}
}
