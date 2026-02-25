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

// Package main demonstrates formal verification of a Petri net workflow.
//
// JazzPetri can PROVE safety properties about your workflow BEFORE it runs.
// This is the key differentiator from traditional workflow engines: you get
// mathematical guarantees, not just test coverage.
//
// This example builds a producer-consumer pipeline with a bounded buffer and
// proves seven properties about it using the verification package:
//
//	[idle] ─(produce)─► [buffer] ─(consume)─► [done]
//
// Properties verified:
//  1. Boundedness:        no place accumulates unlimited tokens
//  2. Deadlock freedom:   no reachable non-terminal state blocks forever
//  3. Reachability:       the 'done' place is reachable (workflow can complete)
//  4. Mutual exclusion:   'idle' and 'done' are never both occupied simultaneously
//  5. Place invariant:    total tokens across idle+buffer+done never exceeds 1
//  6. Buffer invariant:   buffer never holds more than 1 token
//  7. Certificate:        a full proof certificate summarising all results
//
// Run with:
//
//	go run ./examples/04_formal_verification
package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/jazzpetri/engine/petri"
	"github.com/jazzpetri/engine/token"
	"github.com/jazzpetri/engine/verification"
)

// separator is a visual divider for output sections.
const separator = "────────────────────────────────────────────────────────────"

func main() {
	fmt.Println("=== Example 04: Formal Verification of a Producer-Consumer Workflow ===")
	fmt.Println()
	fmt.Println("Building producer-consumer net with bounded buffer...")
	fmt.Println()

	// ------------------------------------------------------------------
	// Build the producer-consumer Petri net.
	//
	// Structure:
	//   [idle] ──(produce)──► [buffer] ──(consume)──► [done]
	//
	// Semantics:
	//   - One token starts in 'idle' representing an idle producer.
	//   - 'produce' fires: removes from idle, places in buffer.
	//   - 'consume' fires: removes from buffer, places in done.
	//   - Terminal state: token in 'done'.
	// ------------------------------------------------------------------
	net := petri.NewPetriNet("producer-consumer", "Producer-Consumer with Bounded Buffer")

	// Places
	pIdle := petri.NewPlace("idle", "Producer Idle", 1) // capacity 1 = bounded
	pIdle.IsInitial = true

	pBuffer := petri.NewPlace("buffer", "Shared Buffer", 1) // bounded buffer
	pBuffer.IsTerminal = false

	pDone := petri.NewPlace("done", "Consumption Complete", 10)
	pDone.IsTerminal = true

	// Transitions
	tProduce := petri.NewTransition("produce", "Produce Item")
	tConsume := petri.NewTransition("consume", "Consume Item")

	// Assemble
	net.AddPlace(pIdle)
	net.AddPlace(pBuffer)
	net.AddPlace(pDone)
	net.AddTransition(tProduce)
	net.AddTransition(tConsume)

	// Arcs: idle ──(produce)──► buffer ──(consume)──► done
	net.AddArc(petri.NewArc("arc-idle-produce", "idle", "produce", 1))
	net.AddArc(petri.NewArc("arc-produce-buffer", "produce", "buffer", 1))
	net.AddArc(petri.NewArc("arc-buffer-consume", "buffer", "consume", 1))
	net.AddArc(petri.NewArc("arc-consume-done", "consume", "done", 1))

	if err := net.Resolve(); err != nil {
		log.Fatalf("failed to resolve net: %v", err)
	}

	// Seed the initial token: one idle producer.
	pIdle.AddToken(&token.Token{
		ID:   "producer-1",
		Data: &petri.MapToken{Value: map[string]interface{}{"item": "widget"}},
	})

	fmt.Println("Net structure:")
	fmt.Println("  [idle] ──(produce)──► [buffer] ──(consume)──► [done]")
	fmt.Println()
	fmt.Printf("  Initial marking: idle=1, buffer=0, done=0\n")
	fmt.Println()

	// ------------------------------------------------------------------
	// Create a verifier and build the complete state space.
	//
	// State space exploration uses breadth-first search to enumerate every
	// reachable marking reachable from the initial marking.
	// maxStates=1000 prevents runaway exploration on unbounded nets.
	// ------------------------------------------------------------------
	v := verification.NewVerifier(net, 1000)

	fmt.Println(separator)
	fmt.Println("Building state space (BFS reachability graph)...")
	ss, err := v.BuildStateSpace()
	if err != nil {
		// A non-nil error means the state space limit was hit.
		// We can still check properties on the partial state space.
		fmt.Printf("  WARNING: %v\n", err)
	}
	fmt.Printf("  States discovered: %d\n", len(ss.States))
	totalEdges := 0
	for _, edges := range ss.Edges {
		totalEdges += len(edges)
	}
	fmt.Printf("  Transitions (edges) in graph: %d\n", totalEdges)
	fmt.Println()

	// Print all discovered states for educational visibility.
	fmt.Println("Reachable states (marking = token counts per place):")
	for _, s := range ss.States {
		fmt.Printf("  State %d: idle=%d  buffer=%d  done=%d\n",
			s.ID, s.Marking["idle"], s.Marking["buffer"], s.Marking["done"])
	}
	fmt.Println()

	// ------------------------------------------------------------------
	// Run individual property checks.
	// ------------------------------------------------------------------

	// 1. Boundedness
	fmt.Println(separator)
	fmt.Println("CHECK 1: Boundedness")
	printResult(v.CheckBoundedness(ss))

	// 2. Deadlock freedom
	fmt.Println(separator)
	fmt.Println("CHECK 2: Deadlock Freedom")
	printResult(v.CheckDeadlockFreedom(ss))

	// 3. Reachability — can we reach a state where 'done' has 1 token?
	fmt.Println(separator)
	fmt.Println("CHECK 3: Reachability of completion state (done=1)")
	target := verification.Marking{"done": 1}
	result := v.CheckReachability(ss, target)
	printResult(result)
	if result.Satisfied && len(result.Witness) > 0 {
		fmt.Printf("         Witness path: %s\n", strings.Join(result.Witness, " → "))
	}

	// 4. Mutual exclusion — 'idle' and 'done' should never both have tokens.
	fmt.Println(separator)
	fmt.Println("CHECK 4: Mutual Exclusion between 'idle' and 'done'")
	printResult(v.CheckMutualExclusion(ss, "idle", "done"))

	// 5. Place invariant — total tokens in the system is always exactly 1.
	//    Coefficients: idle*1 + buffer*1 + done*1 <= 1
	fmt.Println(separator)
	fmt.Println("CHECK 5: Token Conservation Invariant (idle + buffer + done <= 1)")
	coefficients := map[string]int{"idle": 1, "buffer": 1, "done": 1}
	printResult(v.CheckPlaceInvariant(ss, coefficients, 1))

	// 6. Buffer capacity invariant — buffer never exceeds its capacity of 1.
	fmt.Println(separator)
	fmt.Println("CHECK 6: Buffer Capacity Invariant (buffer <= 1)")
	bufferCoeff := map[string]int{"buffer": 1}
	printResult(v.CheckPlaceInvariant(ss, bufferCoeff, 1))

	// ------------------------------------------------------------------
	// 7. Generate a full proof certificate.
	//    GenerateCertificate builds the state space internally and runs
	//    boundedness + deadlock freedom, returning a structured summary.
	// ------------------------------------------------------------------
	fmt.Println(separator)
	fmt.Println("CHECK 7: Full Proof Certificate")
	fmt.Println()

	// Re-seed for a fresh certificate (GenerateCertificate snapshots current tokens).
	pIdle.AddToken(&token.Token{
		ID:   "producer-cert",
		Data: &petri.MapToken{Value: map[string]interface{}{"item": "gadget"}},
	})
	// Drain any tokens left from prior exploration to get a clean initial marking.
	for pBuffer.TokenCount() > 0 {
		pBuffer.RemoveToken()
	}
	for pDone.TokenCount() > 0 {
		pDone.RemoveToken()
	}
	// pIdle already drained during state-space run; add fresh token.

	v2 := verification.NewVerifier(net, 1000)
	cert, certErr := v2.GenerateCertificate()
	if certErr != nil {
		fmt.Printf("  Certificate WARNING: %v\n", certErr)
	}

	fmt.Printf("  Workflow ID:   %s\n", cert.WorkflowID)
	fmt.Printf("  States:        %d\n", cert.StateCount)
	fmt.Printf("  Edges:         %d\n", cert.EdgeCount)
	fmt.Printf("  Bounded:       %v\n", cert.Bounded)
	fmt.Printf("  Deadlock-free: %v\n", cert.DeadlockFree)
	fmt.Printf("  Max tokens:    %d (in any single place, any reachable state)\n", cert.MaxTokens)
	fmt.Println()

	fmt.Println("  Properties in certificate:")
	for _, p := range cert.Properties {
		status := "PASS"
		if !p.Satisfied {
			status = "FAIL"
		}
		fmt.Printf("    [%s] %-20s — %s\n", status, p.Property, p.Message)
	}
	fmt.Println()

	allOK := cert.AllSatisfied()
	fmt.Println(separator)
	if allOK {
		fmt.Println("RESULT: All properties satisfied. Workflow is formally verified.")
	} else {
		fmt.Println("RESULT: One or more properties FAILED. Review the certificate above.")
	}
	fmt.Println()
	fmt.Println("This proof was produced by exhaustive state-space exploration.")
	fmt.Println("It holds for every possible execution — not just the paths you tested.")

	// Print Petri net diagram
	fmt.Println("\n--- Petri Net Diagram (Mermaid) ---")
	fmt.Println(net.ToMermaid())
}

// printResult pretty-prints a single VerificationResult.
func printResult(r verification.VerificationResult) {
	status := "PASS"
	if !r.Satisfied {
		status = "FAIL"
	}
	fmt.Printf("  [%s] %s\n", status, r.Message)
	fmt.Printf("       States checked: %d\n", r.StatesChecked)
	if len(r.Witness) > 0 {
		fmt.Printf("       Witness: %s\n", strings.Join(r.Witness, " → "))
	}
}
