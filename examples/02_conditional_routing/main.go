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

// Package main demonstrates OR-split conditional routing via guards.
//
// This example models a loan application workflow where a credit score
// determines which branch the application follows:
//
//	                    ┌─ (auto_approve) [credit >= 700] ──► [approved]
//	[application] ──────┤
//	                    └─ (manual_review) [credit < 700] ──► [manual_review]
//
// Guards are predicate functions evaluated against the tokens in a
// transition's input places. Only the transition whose guard returns
// true will fire — creating a conditional OR-split.
//
// Run with:
//
//	go run ./examples/02_conditional_routing
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

// buildLoanNet constructs the loan application Petri net.
// It is called twice: once for the high-score run, once for the low-score run.
func buildLoanNet() (*petri.PetriNet, *petri.Place, *petri.Place, *petri.Place, error) {
	net := petri.NewPetriNet("loan-workflow", "Loan Application Routing")

	// ------------------------------------------------------------------
	// Places
	// ------------------------------------------------------------------
	pApplication := petri.NewPlace("application", "Application Received", 10)
	pApplication.IsInitial = true

	pApproved := petri.NewPlace("approved", "Auto-Approved", 10)
	pApproved.IsTerminal = true

	pManualReview := petri.NewPlace("review_queue", "Manual Review Queue", 10)
	pManualReview.IsTerminal = true

	// ------------------------------------------------------------------
	// Transitions with guards.
	// A guard receives the execution context and the slice of tokens
	// that would be consumed if the transition fires.
	// ------------------------------------------------------------------

	// Guard for auto-approval: credit score must be >= 700.
	tAutoApprove := petri.NewTransition("auto_approve", "Auto Approve").
		WithGuard(func(ctx *execContext.ExecutionContext, tokens []*token.Token) (bool, error) {
			if len(tokens) == 0 {
				return false, nil
			}
			data, ok := tokens[0].Data.(*petri.MapToken)
			if !ok {
				return false, nil
			}
			score, ok := data.Value["credit_score"].(float64)
			if !ok {
				return false, nil
			}
			return score >= 700, nil
		}).
		WithTask(&task.InlineTask{
			Name: "auto-approve",
			Fn: func(ctx *execContext.ExecutionContext) error {
				fmt.Println("  [auto_approve] Credit score meets threshold.")
				fmt.Println("  [auto_approve] Loan automatically approved.")
				return nil
			},
		})

	// Guard for manual review: credit score is below 700.
	tManualReview := petri.NewTransition("manual_review", "Send to Manual Review").
		WithGuard(func(ctx *execContext.ExecutionContext, tokens []*token.Token) (bool, error) {
			if len(tokens) == 0 {
				return false, nil
			}
			data, ok := tokens[0].Data.(*petri.MapToken)
			if !ok {
				return false, nil
			}
			score, ok := data.Value["credit_score"].(float64)
			if !ok {
				return false, nil
			}
			return score < 700, nil
		}).
		WithTask(&task.InlineTask{
			Name: "manual-review",
			Fn: func(ctx *execContext.ExecutionContext) error {
				fmt.Println("  [manual_review] Credit score below threshold.")
				fmt.Println("  [manual_review] Application queued for human review.")
				return nil
			},
		})

	// ------------------------------------------------------------------
	// Build and connect the net.
	// Both transitions share the same input place (application), so only
	// the one whose guard passes will consume the token.
	// ------------------------------------------------------------------
	net.AddPlace(pApplication)
	net.AddPlace(pApproved)
	net.AddPlace(pManualReview)
	net.AddTransition(tAutoApprove)
	net.AddTransition(tManualReview)

	// Both transitions read from the same input place — OR-split.
	net.AddArc(petri.NewArc("arc-app-approve", "application", "auto_approve", 1))
	net.AddArc(petri.NewArc("arc-approve-out", "auto_approve", "approved", 1))
	net.AddArc(petri.NewArc("arc-app-review", "application", "manual_review", 1))
	net.AddArc(petri.NewArc("arc-review-out", "manual_review", "review_queue", 1))

	if err := net.Resolve(); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("resolve failed: %w", err)
	}

	return net, pApplication, pApproved, pManualReview, nil
}

// runScenario executes the loan workflow for a given credit score.
func runScenario(applicant string, creditScore float64) {
	fmt.Printf("--- Applicant: %s  |  Credit Score: %.0f ---\n", applicant, creditScore)

	net, pApplication, pApproved, pManualReview, err := buildLoanNet()
	if err != nil {
		log.Fatalf("failed to build net: %v", err)
	}

	// Place the application token into the starting place.
	appToken := &token.Token{
		ID: "app-" + applicant,
		Data: &petri.MapToken{Value: map[string]interface{}{
			"applicant":    applicant,
			"credit_score": creditScore,
			"amount":       25000.0,
		}},
	}
	pApplication.AddToken(appToken)

	// Create context and engine.
	ctx := execContext.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)
	eng := engine.NewEngine(net, ctx, engine.Config{
		Mode:         engine.ModeSequential,
		PollInterval: 10 * time.Millisecond,
	})

	eng.Start()
	eng.Wait()
	eng.Stop()

	// Report which path was taken.
	switch {
	case pApproved.TokenCount() > 0:
		fmt.Printf("  Result: APPROVED (token in '%s')\n\n", pApproved.Name)
	case pManualReview.TokenCount() > 0:
		fmt.Printf("  Result: MANUAL REVIEW (token in '%s')\n\n", pManualReview.Name)

	default:
		fmt.Println("  Result: UNKNOWN — no terminal token found")
	}
}

func main() {
	fmt.Println("=== Example 02: Conditional Routing with Guards ===")
	fmt.Println()

	// Run the high credit-score scenario (should be auto-approved).
	runScenario("Alice", 750)

	// Run the low credit-score scenario (should go to manual review).
	runScenario("Bob", 620)

	// Print Petri net diagram
	net, _, _, _, err := buildLoanNet()
	if err != nil {
		log.Fatalf("failed to build diagram net: %v", err)
	}
	fmt.Println("--- Petri Net Diagram (Mermaid) ---")
	fmt.Println(net.ToMermaid())
}
