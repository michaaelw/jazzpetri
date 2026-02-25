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

// Package main demonstrates timed workflow execution with VirtualClock.
//
// This example models an SLA escalation workflow for a support ticket:
//
//	[ticket_open] ──(escalate, delay=1h)──► [escalated] ──(critical, delay=24h)──► [critical]
//
// In production, the delays are real wall-clock waits (1 hour, 24 hours).
// In this example we use VirtualClock so the entire workflow completes
// instantly — the clock is advanced programmatically, not waited on.
//
// The VirtualClock pattern is essential for:
//   - Fast unit and integration tests for time-dependent workflows
//   - Deterministic replay of time-sensitive workflow scenarios
//   - Development and demos without waiting for real delays
//
// Run with:
//
//	go run ./examples/05_timed_triggers
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jazzpetri/engine/clock"
	execContext "github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/engine"
	"github.com/jazzpetri/engine/petri"
	"github.com/jazzpetri/engine/task"
	"github.com/jazzpetri/engine/token"
)

func main() {
	fmt.Println("=== Example 05: Timed Triggers with VirtualClock ===")
	fmt.Println()
	fmt.Println("Workflow: support ticket SLA escalation")
	fmt.Println("  Stage 1: Open ──[1 hour]──► Escalated")
	fmt.Println("  Stage 2: Escalated ──[24 hours]──► Critical")
	fmt.Println()
	fmt.Println("Using VirtualClock: real time does NOT pass — clock is advanced manually.")
	fmt.Println()

	// ------------------------------------------------------------------
	// Create a VirtualClock starting at a fixed point in time.
	// All time operations (After, Sleep, Now) use this virtual time.
	// ------------------------------------------------------------------
	startTime := time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)
	vClock := clock.NewVirtualClock(startTime)

	fmt.Printf("Virtual clock initialised at: %s\n\n", vClock.Now().Format(time.RFC3339))

	// ------------------------------------------------------------------
	// Build the escalation workflow net.
	//
	// Stage 1: ticket_open ──(escalate, 1h delay)──► escalated
	// Stage 2: escalated   ──(critical, 24h delay)──► critical_breach
	// ------------------------------------------------------------------
	net := petri.NewPetriNet("sla-escalation", "SLA Escalation Workflow")

	// Places — each with a distinct ID (no naming collisions with transitions).
	pOpen := petri.NewPlace("ticket_open", "Ticket Open", 10)
	pOpen.IsInitial = true

	pEscalated := petri.NewPlace("escalated", "Ticket Escalated", 10)

	pCritical := petri.NewPlace("critical_breach", "Critical — SLA Breach", 10)
	pCritical.IsTerminal = true

	// ------------------------------------------------------------------
	// Timed transitions using TimedTrigger.
	//
	// The Delay field sets how long to wait after the transition becomes
	// enabled before firing. The engine uses the clock from the
	// ExecutionContext, so VirtualClock controls the wait.
	// ------------------------------------------------------------------
	escalationDelay := 1 * time.Hour
	tEscalate := petri.NewTransition("escalate", "Escalate Ticket").
		WithTrigger(&petri.TimedTrigger{Delay: &escalationDelay}).
		WithTask(&task.InlineTask{
			Name: "escalate",
			Fn: func(ctx *execContext.ExecutionContext) error {
				fmt.Printf("  [%s] ESCALATED: Ticket unresolved — notifying on-call engineer.\n",
					ctx.Clock.Now().Format("15:04 UTC"))
				return nil
			},
		})

	criticalDelay := 24 * time.Hour
	tCritical := petri.NewTransition("critical", "Mark Critical").
		WithTrigger(&petri.TimedTrigger{Delay: &criticalDelay}).
		WithTask(&task.InlineTask{
			Name: "critical",
			Fn: func(ctx *execContext.ExecutionContext) error {
				fmt.Printf("  [%s] CRITICAL: 25-hour breach — opening P0 incident, notifying VP Engineering.\n",
					ctx.Clock.Now().Format("15:04 UTC"))
				return nil
			},
		})

	// Assemble.
	net.AddPlace(pOpen)
	net.AddPlace(pEscalated)
	net.AddPlace(pCritical)
	net.AddTransition(tEscalate)
	net.AddTransition(tCritical)

	// Arcs: ticket_open ──(escalate)──► escalated ──(critical)──► critical_breach
	net.AddArc(petri.NewArc("arc-open-escalate", "ticket_open", "escalate", 1))
	net.AddArc(petri.NewArc("arc-escalate-esc", "escalate", "escalated", 1))
	net.AddArc(petri.NewArc("arc-esc-critical", "escalated", "critical", 1))
	net.AddArc(petri.NewArc("arc-critical-breach", "critical", "critical_breach", 1))

	if err := net.Resolve(); err != nil {
		log.Fatalf("failed to resolve net: %v", err)
	}

	// Add the initial ticket token.
	ticketToken := &token.Token{
		ID: "ticket-JIRA-4242",
		Data: &petri.MapToken{Value: map[string]interface{}{
			"ticketID": "JIRA-4242",
			"reporter": "alice@example.com",
			"priority": "high",
		}},
	}
	pOpen.AddToken(ticketToken)
	fmt.Printf("Ticket '%v' opened at %s.\n\n",
		ticketToken.Data.(*petri.MapToken).Value["ticketID"],
		vClock.Now().Format(time.RFC3339))

	// ------------------------------------------------------------------
	// Create the execution context with our VirtualClock.
	// ------------------------------------------------------------------
	ctx := execContext.NewExecutionContext(
		context.Background(),
		vClock,
		nil,
	)

	cfg := engine.Config{
		Mode:         engine.ModeSequential,
		PollInterval: 5 * time.Millisecond,
	}
	eng := engine.NewEngine(net, ctx, cfg)
	eng.Start()

	// ------------------------------------------------------------------
	// Drive the workflow by advancing the virtual clock.
	//
	// After each AdvanceBy call the engine's trigger-waiter goroutine
	// unblocks (because the clock.After channel fires), fires the
	// transition, and moves the token to the next place.
	//
	// We Sleep(real time) between advances to give the engine goroutines
	// time to react before we advance again.
	// ------------------------------------------------------------------

	// Let the engine register its first timer (escalate, T+1h).
	time.Sleep(20 * time.Millisecond)

	fmt.Printf("--- Advancing virtual clock by 1 hour (T+1h) ---\n")
	vClock.AdvanceBy(1 * time.Hour)
	fmt.Printf("    Clock now: %s\n\n", vClock.Now().Format(time.RFC3339))

	// Wait for the escalation transition to fire and the engine to register
	// the second timer (critical, T+24h from escalation).
	time.Sleep(50 * time.Millisecond)

	fmt.Printf("--- Advancing virtual clock by 24 hours (T+25h) ---\n")
	vClock.AdvanceBy(24 * time.Hour)
	fmt.Printf("    Clock now: %s\n\n", vClock.Now().Format(time.RFC3339))

	// Give the critical transition time to fire.
	time.Sleep(50 * time.Millisecond)

	eng.Stop()

	// ------------------------------------------------------------------
	// Show final state.
	// ------------------------------------------------------------------
	fmt.Printf("Final token counts:\n")
	fmt.Printf("  %-22s %d token(s)\n", pOpen.Name+":", pOpen.TokenCount())
	fmt.Printf("  %-22s %d token(s)\n", pEscalated.Name+":", pEscalated.TokenCount())
	fmt.Printf("  %-22s %d token(s)\n", pCritical.Name+":", pCritical.TokenCount())
	fmt.Println()
	fmt.Println("SLA escalation workflow complete.")
	fmt.Println("(Total real elapsed time: < 1 second — VirtualClock made it instant.)")
}
