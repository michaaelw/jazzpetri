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
	stdContext "context"
	"testing"
	"time"

	"github.com/jazzpetri/engine/clock"
	"github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/petri"
	"github.com/jazzpetri/engine/token"
)

// No Dry-Run Mode
//
// EXPECTED FAILURE: These tests will fail because DryRun() method
// does not exist on Engine.
//
// Dry-run mode enables:
//   - Workflow validation without execution
//   - Detecting structural issues (deadlocks, unreachable nodes)
//   - Testing workflow changes safely
//   - Documentation and analysis

func TestEngine_DryRun(t *testing.T) {
	// EXPECTED FAILURE: Engine.DryRun() method doesn't exist

	net := createTestWorkflow()
	ctx := createTestContextForDryrun()
	engine := NewEngine(net, ctx, DefaultConfig())

	// Should validate workflow without executing
	result := engine.DryRun() // Does not exist

	if result == nil {
		t.Fatal("DryRun should return validation result")
	}

	// Should report validation status
	if !result.Valid {
		t.Errorf("Valid workflow should pass dry-run: %v", result.Issues)
	}
}

func TestEngine_DryRun_DetectsUnreachable(t *testing.T) {
	// EXPECTED FAILURE: DryRun doesn't detect unreachable transitions

	net := createWorkflowWithUnreachable()
	ctx := createTestContextForDryrun()
	engine := NewEngine(net, ctx, DefaultConfig())

	result := engine.DryRun()

	// Should detect unreachable transitions
	if !result.HasIssues() {
		t.Error("DryRun should detect unreachable transitions")
	}

	// Should report specific issues
	foundUnreachable := false
	for _, issue := range result.Issues {
		if issue.Type == "unreachable_transition" {
			foundUnreachable = true
			break
		}
	}

	if !foundUnreachable {
		t.Error("DryRun should report unreachable transitions")
	}
}

func TestEngine_DryRun_DetectsDeadlock(t *testing.T) {
	// EXPECTED FAILURE: DryRun doesn't detect deadlocks

	net := createWorkflowWithDeadlock()
	ctx := createTestContextForDryrun()
	engine := NewEngine(net, ctx, DefaultConfig())

	result := engine.DryRun()

	// Should detect potential deadlocks
	if !result.HasIssues() {
		t.Error("DryRun should detect potential deadlocks")
	}

	foundDeadlock := false
	for _, issue := range result.Issues {
		if issue.Type == "potential_deadlock" {
			foundDeadlock = true
			break
		}
	}

	if !foundDeadlock {
		t.Error("DryRun should report potential deadlocks")
	}
}

func TestEngine_DryRun_DetectsUnboundedPlaces(t *testing.T) {
	// EXPECTED FAILURE: DryRun doesn't detect unbounded growth

	net := createWorkflowWithUnboundedPlace()
	ctx := createTestContextForDryrun()
	engine := NewEngine(net, ctx, DefaultConfig())

	result := engine.DryRun()

	// Should detect places that can grow unbounded
	foundUnbounded := false
	for _, issue := range result.Issues {
		if issue.Type == "unbounded_place" {
			foundUnbounded = true
			break
		}
	}

	if !foundUnbounded {
		t.Error("DryRun should detect unbounded places")
	}
}

func TestEngine_DryRun_Warnings(t *testing.T) {
	// EXPECTED FAILURE: DryRun doesn't provide warnings

	net := createWorkflowWithWarnings()
	ctx := createTestContextForDryrun()
	engine := NewEngine(net, ctx, DefaultConfig())

	result := engine.DryRun()

	// Should provide warnings for best practices
	if len(result.Warnings) == 0 {
		t.Error("DryRun should provide warnings for potential issues")
	}

	// Examples of warnings:
	// - Place capacity seems low
	// - Transition has no output arcs
	// - Complex guard function
}

func TestEngine_DryRun_NoSideEffects(t *testing.T) {
	// EXPECTED FAILURE: Need to verify no side effects

	net := createTestWorkflow()
	ctx := createTestContextForDryrun()
	engine := NewEngine(net, ctx, DefaultConfig())

	// Add initial marking
	p1, _ := net.GetPlace("p1")
	initialCount := p1.TokenCount()

	// Dry-run should not modify state
	_ = engine.DryRun()

	// Token count should be unchanged
	if p1.TokenCount() != initialCount {
		t.Error("DryRun should not modify workflow state")
	}

	// Engine should not be started
	if engine.IsRunning() {
		t.Error("DryRun should not start engine")
	}
}

func TestEngine_DryRun_StructuralAnalysis(t *testing.T) {
	// EXPECTED FAILURE: DryRun doesn't provide structural analysis

	net := createTestWorkflow()
	ctx := createTestContextForDryrun()
	engine := NewEngine(net, ctx, DefaultConfig())

	result := engine.DryRun()

	// Should include structural analysis
	if result.Analysis == nil {
		t.Error("DryRun should include structural analysis")
	}

	// Analysis should include metrics
	if result.Analysis["place_count"] == nil || result.Analysis["place_count"].(int) == 0 {
		t.Error("Analysis should include place count")
	}

	if result.Analysis["transition_count"] == nil || result.Analysis["transition_count"].(int) == 0 {
		t.Error("Analysis should include transition count")
	}

	if result.Analysis["arc_count"] == nil || result.Analysis["arc_count"].(int) == 0 {
		t.Error("Analysis should include arc count")
	}
}

func TestEngine_DryRun_ReachabilityAnalysis(t *testing.T) {
	// EXPECTED FAILURE: Advanced reachability analysis doesn't exist

	net := createTestWorkflow()
	ctx := createTestContextForDryrun()
	engine := NewEngine(net, ctx, DefaultConfig())

	result := engine.DryRun()

	// Advanced: Reachability analysis
	// Which transitions can fire from initial marking?
	// Which places can be reached?
	// Are there isolated subgraphs?

	if result.Reachability == nil {
		t.Skip("Advanced reachability analysis not implemented")
	}
}

// Helper functions to create test workflows
func createTestWorkflow() *petri.PetriNet {
	// Simple valid workflow: p1 -> t1 -> p2
	net := petri.NewPetriNet("test-workflow", "Test Workflow")

	p1 := petri.NewPlace("p1", "Place 1", 10)
	p2 := petri.NewPlace("p2", "Place 2", 10)

	t1 := petri.NewTransition("t1", "Transition 1")

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddTransition(t1)

	net.AddArc(petri.NewArc("a1", "p1", "t1", 1))
	net.AddArc(petri.NewArc("a2", "t1", "p2", 1))

	net.Resolve()
	return net
}

func createWorkflowWithUnreachable() *petri.PetriNet {
	// Workflow with unreachable transition (no input arcs)
	net := petri.NewPetriNet("unreachable-workflow", "Unreachable Workflow")

	p1 := petri.NewPlace("p1", "Place 1", 10)
	p2 := petri.NewPlace("p2", "Place 2", 10)

	t1 := petri.NewTransition("t1", "Transition 1")
	t2 := petri.NewTransition("t2", "Unreachable Transition")

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddTransition(t1)
	net.AddTransition(t2)

	net.AddArc(petri.NewArc("a1", "p1", "t1", 1))
	net.AddArc(petri.NewArc("a2", "t1", "p2", 1))
	// t2 has no input arcs - unreachable!

	net.Resolve()
	return net
}

func createWorkflowWithDeadlock() *petri.PetriNet {
	// Workflow that can deadlock: place with no incoming arcs but has outgoing arcs
	net := petri.NewPetriNet("deadlock-workflow", "Deadlock Workflow")

	p1 := petri.NewPlace("p1", "Place 1", 10)
	p2 := petri.NewPlace("p2", "Place 2", 10)

	t1 := petri.NewTransition("t1", "Transition 1")

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddTransition(t1)

	// p1 has initial tokens but no incoming arcs - will deadlock when consumed
	p1.AddToken(&token.Token{ID: "tok1", Data: &token.IntToken{Value: 1}})

	net.AddArc(petri.NewArc("a1", "p1", "t1", 1))
	net.AddArc(petri.NewArc("a2", "t1", "p2", 1))

	net.Resolve()
	return net
}

func createWorkflowWithUnboundedPlace() *petri.PetriNet {
	// Workflow where a place can grow unbounded
	// p2 receives more tokens than it consumes - will grow unbounded
	net := petri.NewPetriNet("unbounded-workflow", "Unbounded Workflow")

	p1 := petri.NewPlace("p1", "Place 1", 10)
	p2 := petri.NewPlace("p2", "Place 2", 100) // Large capacity but will still grow
	p3 := petri.NewPlace("p3", "Place 3", 10)

	t1 := petri.NewTransition("t1", "Transition 1")
	t2 := petri.NewTransition("t2", "Transition 2")

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddPlace(p3)
	net.AddTransition(t1)
	net.AddTransition(t2)

	// p2 gets 5 tokens in but only 1 token out - unbounded growth!
	net.AddArc(petri.NewArc("a1", "p1", "t1", 1))
	net.AddArc(petri.NewArc("a2", "t1", "p2", 5)) // Weight 5 in
	net.AddArc(petri.NewArc("a3", "p2", "t2", 1)) // Weight 1 out - unbounded!
	net.AddArc(petri.NewArc("a4", "t2", "p3", 1))

	net.Resolve()
	return net
}

func createWorkflowWithWarnings() *petri.PetriNet {
	// Workflow with potential issues that are warnings, not errors
	net := petri.NewPetriNet("warning-workflow", "Warning Workflow")

	p1 := petri.NewPlace("p1", "Place 1", 10)
	p2 := petri.NewPlace("p2", "Place 2", 10)
	p3 := petri.NewPlace("p3", "Isolated Place", 10) // No arcs - warning

	t1 := petri.NewTransition("t1", "Transition 1")

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddPlace(p3)
	net.AddTransition(t1)

	net.AddArc(petri.NewArc("a1", "p1", "t1", 1))
	net.AddArc(petri.NewArc("a2", "t1", "p2", 1))
	// p3 has no arcs - will trigger warning

	net.Resolve()
	return net
}

// Example showing dry-run usage
func ExampleEngine_DryRun() {
	// EXPECTED FAILURE: DryRun method doesn't exist

	net := createTestWorkflow()
	ctx := createTestContextForDryrun()
	engine := NewEngine(net, ctx, DefaultConfig())

	// Validate workflow before running
	result := engine.DryRun()

	if result.Valid {
		// Safe to run
		_ = engine.Start()
	} else {
		// Print issues
		for _, issue := range result.Issues {
			// fmt.Printf("ERROR: %s - %s\n", issue.Type, issue.Message)
			_ = issue
		}
	}

	// Check warnings
	for _, warning := range result.Warnings {
		// fmt.Printf("WARNING: %s\n", warning.Message)
		_ = warning
	}
}

// Proposed DryRunResult structure (currently doesn't exist)
// type DryRunResult struct {
//     Valid        bool                // Overall validation status
//     Issues       []ValidationIssue   // Errors that prevent execution
//     Warnings     []ValidationWarning // Warnings about potential problems
//     Analysis     *StructuralAnalysis // Structural metrics
//     Reachability *ReachabilityInfo   // Reachability analysis (optional)
// }
//
// type ValidationIssue struct {
//     Type        string   // "unreachable_transition", "potential_deadlock", etc.
//     Severity    string   // "error", "warning"
//     NodeID      string   // Which place/transition
//     Message     string   // Human-readable description
//     Suggestion  string   // How to fix
// }
//
// type ValidationWarning struct {
//     Type       string
//     NodeID     string
//     Message    string
//     Suggestion string
// }
//
// type StructuralAnalysis struct {
//     PlaceCount      int
//     TransitionCount int
//     ArcCount        int
//     MaxFanIn        int // Max input arcs to any transition
//     MaxFanOut       int // Max output arcs from any transition
//     HasCycles       bool
//     IsolatedNodes   []string
// }
//
// type ReachabilityInfo struct {
//     ReachablePlaces      []string
//     ReachableTransitions []string
//     UnreachablePlaces    []string
//     UnreachableTransitions []string
// }

// Example showing detailed dry-run output
func ExampleEngine_DryRun_detailed() {
	// EXPECTED FAILURE: DryRun and detailed output don't exist

	net := createTestWorkflow()
	ctx := createTestContextForDryrun()
	engine := NewEngine(net, ctx, DefaultConfig())

	result := engine.DryRun()

	// Validation summary
	// fmt.Printf("Valid: %v\n", result.Valid)
	// fmt.Printf("Issues: %d\n", len(result.Issues))
	// fmt.Printf("Warnings: %d\n", len(result.Warnings))

	// Structural analysis
	// fmt.Printf("\nStructural Analysis:\n")
	// fmt.Printf("  Places: %d\n", result.Analysis.PlaceCount)
	// fmt.Printf("  Transitions: %d\n", result.Analysis.TransitionCount)
	// fmt.Printf("  Arcs: %d\n", result.Analysis.ArcCount)
	// fmt.Printf("  Has Cycles: %v\n", result.Analysis.HasCycles)

	// Issues
	// for _, issue := range result.Issues {
	//     fmt.Printf("\nERROR [%s]: %s\n", issue.Type, issue.Message)
	//     fmt.Printf("  Node: %s\n", issue.NodeID)
	//     fmt.Printf("  Fix: %s\n", issue.Suggestion)
	// }

	_ = result
}

func createTestContextForDryrun() *context.ExecutionContext {
	return context.NewExecutionContext(
		stdContext.Background(),
		clock.NewVirtualClock(time.Now()),
		nil,
	)
}
