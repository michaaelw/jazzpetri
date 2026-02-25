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

package state_test

import (
	"fmt"
	stdcontext "context"
	"time"

	"github.com/jazzpetri/engine/clock"
	"github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/petri"
	"github.com/jazzpetri/engine/state"
	"github.com/jazzpetri/engine/token"
)

// ExampleMemoryEventLog_basic demonstrates basic event log usage
func ExampleMemoryEventLog_basic() {
	// Create event log and clock
	eventLog := state.NewMemoryEventLog()
	clk := clock.NewVirtualClock(time.Now())

	// Create some events
	event1 := state.NewEvent(state.EventTransitionFired, "workflow1", clk)
	event1.TransitionID = "t1"
	event1.SetMetadata(map[string]interface{}{
		"transition_name": "Start Processing",
	})

	event2 := state.NewEvent(state.EventTransitionFired, "workflow1", clk)
	event2.TransitionID = "t2"
	event2.SetMetadata(map[string]interface{}{
		"transition_name": "Finish Processing",
	})

	// Append events
	eventLog.Append(event1)
	eventLog.Append(event2)

	// Query events
	allEvents, _ := eventLog.GetAll()
	fmt.Printf("Total events: %d\n", len(allEvents))

	// Query by type
	firedEvents, _ := eventLog.GetByType(state.EventTransitionFired)
	fmt.Printf("Fired events: %d\n", len(firedEvents))

	// Query by transition
	t1Events, _ := eventLog.GetByTransition("t1")
	fmt.Printf("Events for t1: %d\n", len(t1Events))

	// Output:
	// Total events: 2
	// Fired events: 2
	// Events for t1: 1
}

// ExampleMemoryEventLog_debugging demonstrates using event log for debugging
func ExampleMemoryEventLog_debugging() {
	// Set up workflow with event logging
	net := petri.NewPetriNet("debug-workflow", "Debug Workflow")

	start := petri.NewPlace("start", "Start", 1)
	end := petri.NewPlace("end", "End", 1)
	t1 := petri.NewTransition("process", "Process Data")

	net.AddPlace(start)
	net.AddPlace(end)
	net.AddTransition(t1)
	net.AddArc(petri.NewArc("a1", "start", "process", 1))
	net.AddArc(petri.NewArc("a2", "process", "end", 1))

	tok := &token.Token{
		ID:   "data-token",
		Data: &token.IntToken{Value: 42},
	}
	start.AddToken(tok)
	net.Resolve()

	// Create event logging
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

	// Execute transition
	t1.Fire(ctx)

	// Debug: What happened?
	events, _ := eventLog.GetAll()
	for _, e := range events {
		fmt.Printf("Event: %s for transition %s\n", e.Type, e.TransitionID)
	}

	// Output:
	// Event: transition.fired for transition process
}

// ExampleMemoryEventLog_auditTrail demonstrates using event log as an audit trail
func ExampleMemoryEventLog_auditTrail() {
	eventLog := state.NewMemoryEventLog()
	baseTime := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(baseTime)

	// Simulate workflow events throughout the day
	clk.AdvanceTo(baseTime.Add(0 * time.Hour))
	event1 := state.NewEvent(state.EventEngineStarted, "order-workflow", clk)
	eventLog.Append(event1)

	clk.AdvanceTo(baseTime.Add(1 * time.Hour))
	event2 := state.NewEvent(state.EventTransitionFired, "order-workflow", clk)
	event2.TransitionID = "validate-order"
	eventLog.Append(event2)

	clk.AdvanceTo(baseTime.Add(2 * time.Hour))
	event3 := state.NewEvent(state.EventTransitionFired, "order-workflow", clk)
	event3.TransitionID = "process-payment"
	eventLog.Append(event3)

	clk.AdvanceTo(baseTime.Add(3 * time.Hour))
	event4 := state.NewEvent(state.EventEngineStopped, "order-workflow", clk)
	eventLog.Append(event4)

	// Query events since 10:00
	cutoff := baseTime.Add(1*time.Hour + 30*time.Minute)
	recentEvents, _ := eventLog.GetSince(cutoff)

	fmt.Printf("Events since 10:30: %d\n", len(recentEvents))
	for _, e := range recentEvents {
		fmt.Printf("- %s at %02d:00\n", e.Type, e.Timestamp.Hour())
	}

	// Output:
	// Events since 10:30: 2
	// - transition.fired at 11:00
	// - engine.stopped at 12:00
}

// ExampleMemoryEventLog_errorInvestigation demonstrates using event log to investigate errors
func ExampleMemoryEventLog_errorInvestigation() {
	eventLog := state.NewMemoryEventLog()
	clk := clock.NewVirtualClock(time.Now())

	// Record some successful transitions
	for i := 1; i <= 3; i++ {
		event := state.NewEvent(state.EventTransitionFired, "workflow", clk)
		event.TransitionID = fmt.Sprintf("t%d", i)
		eventLog.Append(event)
	}

	// Record a failure
	failEvent := state.NewEvent(state.EventTransitionFailed, "workflow", clk)
	failEvent.TransitionID = "t4"
	failEvent.SetError("database connection timeout")
	failEvent.SetMetadata(map[string]interface{}{
		"transition_name": "Save to Database",
		"retry_count":     3,
	})
	eventLog.Append(failEvent)

	// Investigate: Find all failures
	failures, _ := eventLog.GetByType(state.EventTransitionFailed)
	fmt.Printf("Found %d failure(s)\n", len(failures))

	for _, f := range failures {
		fmt.Printf("Transition %s failed: %s\n", f.TransitionID, f.Error)
		fmt.Printf("Retries: %v\n", f.Metadata["retry_count"])
	}

	// Output:
	// Found 1 failure(s)
	// Transition t4 failed: database connection timeout
	// Retries: 3
}
