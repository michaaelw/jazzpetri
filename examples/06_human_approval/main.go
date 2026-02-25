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

// Package main demonstrates human-in-the-loop approval via ManualTrigger.
//
// This example models an expense report approval workflow:
//
//	[submitted] ──(approve, ManualTrigger)──► [approved]
//
// The 'approve' transition will NOT fire automatically. It blocks until an
// authorised approver calls trigger.Approve(userID). This models the pattern
// where a transition waits for a human decision — common in:
//   - Expense approvals
//   - Code review gates
//   - Compliance sign-offs
//   - Deployment authorisation
//
// In this example the approval is simulated by a goroutine that calls
// Approve() after a short delay — in production this would be driven by
// a REST API call, a web UI button, or a Slack slash command.
//
// Run with:
//
//	go run ./examples/06_human_approval
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
	fmt.Println("=== Example 06: Human Approval with ManualTrigger ===")
	fmt.Println()
	fmt.Println("Workflow: expense report approval")
	fmt.Println("  [submitted] ──(approve, ManualTrigger)──► [approved]")
	fmt.Println()

	// ------------------------------------------------------------------
	// Build the approval workflow net.
	// ------------------------------------------------------------------
	net := petri.NewPetriNet("expense-approval", "Expense Report Approval")

	// Places
	pSubmitted := petri.NewPlace("submitted", "Expense Submitted", 10)
	pSubmitted.IsInitial = true

	pApproved := petri.NewPlace("approved", "Expense Approved", 10)
	pApproved.IsTerminal = true

	// ------------------------------------------------------------------
	// ManualTrigger — the heart of this example.
	//
	// The trigger will block until one of the listed Approvers calls
	// trigger.Approve(userID). RequireAll=false means any single approver
	// is sufficient (e.g., either the manager or the finance lead).
	//
	// Timeout is optional: set it to enforce an SLA on the approval.
	// If no approval is received within the timeout, the transition does
	// not fire and the engine logs an error.
	// ------------------------------------------------------------------
	approvalTimeout := 5 * time.Second // generous timeout for the demo
	approvalTrigger := &petri.ManualTrigger{
		ApprovalID: "expense-REQ-2026-042",
		Approvers:  []string{"alice@example.com", "finance-lead@example.com"},
		RequireAll: false,    // any ONE of the listed approvers is sufficient
		Timeout:    &approvalTimeout,
	}

	tApprove := petri.NewTransition("approve", "Approve Expense Report").
		WithTrigger(approvalTrigger).
		WithTask(&task.InlineTask{
			Name: "record-approval",
			Fn: func(ctx *execContext.ExecutionContext) error {
				fmt.Println("  [approve] Recording approval in the finance system...")
				fmt.Println("  [approve] Sending reimbursement to employee bank account...")
				fmt.Println("  [approve] Approval complete.")
				return nil
			},
		})

	// Assemble the net.
	net.AddPlace(pSubmitted)
	net.AddPlace(pApproved)
	net.AddTransition(tApprove)

	net.AddArc(petri.NewArc("arc-sub-approve", "submitted", "approve", 1))
	net.AddArc(petri.NewArc("arc-approve-done", "approve", "approved", 1))

	if err := net.Resolve(); err != nil {
		log.Fatalf("failed to resolve net: %v", err)
	}

	// ------------------------------------------------------------------
	// Place the expense report token into the workflow.
	// ------------------------------------------------------------------
	expenseToken := &token.Token{
		ID: "expense-REQ-2026-042",
		Data: &petri.MapToken{Value: map[string]interface{}{
			"employee":    "Bob Smith",
			"amount":      847.50,
			"currency":    "USD",
			"description": "AWS Summit conference travel",
			"submitted":   time.Now().Format(time.RFC3339),
		}},
	}
	pSubmitted.AddToken(expenseToken)

	// Print expense details.
	data := expenseToken.Data.(*petri.MapToken).Value
	fmt.Printf("Expense report submitted:\n")
	fmt.Printf("  ID:          %s\n", expenseToken.ID)
	fmt.Printf("  Employee:    %v\n", data["employee"])
	fmt.Printf("  Amount:      $%.2f %v\n", data["amount"], data["currency"])
	fmt.Printf("  Description: %v\n", data["description"])
	fmt.Printf("  Approvers:   %v\n", approvalTrigger.Approvers)
	fmt.Println()

	// ------------------------------------------------------------------
	// Create execution context and start the engine.
	// ------------------------------------------------------------------
	ctx := execContext.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	eng := engine.NewEngine(net, ctx, engine.Config{
		Mode:         engine.ModeSequential,
		PollInterval: 10 * time.Millisecond,
	})

	fmt.Println("Engine started. Waiting for human approval...")
	fmt.Printf("  Approval ID:  %s\n", approvalTrigger.ApprovalID)
	fmt.Printf("  Timeout:      %v\n\n", approvalTimeout)

	eng.Start()

	// Let the engine spin up and register the AwaitFiring goroutine.
	// The ManualTrigger's AwaitFiring blocks on approvalChan — it is
	// running in a background goroutine spawned by the engine.
	time.Sleep(50 * time.Millisecond)

	// ------------------------------------------------------------------
	// Simulate a human approver acting via an external system.
	//
	// In production this goroutine would not exist — instead your REST
	// handler (or Slack bot, or approval UI) would receive a webhook and
	// call approvalTrigger.Approve(userID).
	// ------------------------------------------------------------------
	approver := "alice@example.com"
	fmt.Printf(">>> [External System] Manager '%s' is reviewing the expense report...\n", approver)
	time.Sleep(300 * time.Millisecond)
	fmt.Printf(">>> [External System] '%s' clicks APPROVE.\n", approver)

	if err := approvalTrigger.Approve(approver); err != nil {
		// This would happen if the user is not in the Approvers list.
		log.Fatalf("approval failed: %v", err)
	}
	fmt.Printf(">>> [External System] Approval signal delivered to workflow engine.\n\n")

	// Give the engine time to pick up the approval and fire the transition.
	time.Sleep(100 * time.Millisecond)
	eng.Stop()

	// ------------------------------------------------------------------
	// Verify the final state.
	// ------------------------------------------------------------------
	fmt.Printf("Final state:\n")
	fmt.Printf("  submitted: %d token(s)\n", pSubmitted.TokenCount())
	fmt.Printf("  approved:  %d token(s)\n", pApproved.TokenCount())
	fmt.Println()

	if pApproved.TokenCount() > 0 {
		fmt.Println("Expense report approved and reimbursement processed.")
	} else {
		fmt.Println("Expense report NOT approved (check engine logs for details).")
	}

	// ------------------------------------------------------------------
	// Show the approval audit trail.
	// ------------------------------------------------------------------
	fmt.Println()
	fmt.Println("Audit trail:")
	fmt.Printf("  Approval ID:   %s\n", approvalTrigger.ApprovalID)
	fmt.Printf("  Approved:      %v\n", approvalTrigger.IsApproved())
	approvedBy := approvalTrigger.ApprovedBy()
	if len(approvedBy) > 0 {
		fmt.Printf("  Approved by:   %v\n", approvedBy)
	}

	// Print Petri net diagram
	fmt.Println("\n--- Petri Net Diagram (Mermaid) ---")
	fmt.Println(net.ToMermaid())
}
