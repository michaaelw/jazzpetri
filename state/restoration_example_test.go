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

package state

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	jazzcontext "github.com/jazzpetri/engine/context"

	"github.com/jazzpetri/engine/clock"
	"github.com/jazzpetri/engine/petri"
	"github.com/jazzpetri/engine/token"
)

// ExampleRestoreMarkingToNet demonstrates basic checkpoint and restore
func ExampleRestoreMarkingToNet() {
	// Create a simple workflow
	net := petri.NewPetriNet("workflow", "Example Workflow")

	p1 := petri.NewPlace("p1", "Place 1", 10)
	p2 := petri.NewPlace("p2", "Place 2", 10)

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.Resolve()

	// Add some tokens
	tok1 := &token.Token{ID: "tok1", Data: &token.StringToken{Value: "hello"}}
	tok2 := &token.Token{ID: "tok2", Data: &token.StringToken{Value: "world"}}
	p1.AddToken(tok1)
	p1.AddToken(tok2)

	// Capture marking
	clk := clock.NewRealTimeClock()
	marking, _ := NewMarking(net, clk)

	fmt.Printf("Tokens in p1 before clear: %d\n", p1.TokenCount())

	// Clear the net
	p1.Clear()
	p2.Clear()

	fmt.Printf("Tokens in p1 after clear: %d\n", p1.TokenCount())

	// Restore from marking
	RestoreMarkingToNet(marking, net)

	fmt.Printf("Tokens in p1 after restore: %d\n", p1.TokenCount())

	// Output:
	// Tokens in p1 before clear: 2
	// Tokens in p1 after clear: 0
	// Tokens in p1 after restore: 2
}

// Example_checkpointRestore demonstrates checkpoint to file and restore
func Example_checkpointRestore() {
	// Setup
	tmpDir, _ := os.MkdirTemp("", "petri-example")
	defer os.RemoveAll(tmpDir)
	checkpointFile := filepath.Join(tmpDir, "checkpoint.json")

	registry := token.NewMapRegistry()
	registry.Register("StringToken", reflect.TypeOf(token.StringToken{}))

	// Create workflow
	net := petri.NewPetriNet("workflow", "Checkpoint Example")
	p1 := petri.NewPlace("p1", "Place 1", 10)
	net.AddPlace(p1)
	net.Resolve()

	// Add tokens
	p1.AddToken(&token.Token{ID: "tok1", Data: &token.StringToken{Value: "checkpoint"}})

	// Save checkpoint
	clk := clock.NewRealTimeClock()
	marking, _ := NewMarking(net, clk)
	SaveMarkingToFile(marking, checkpointFile, registry)

	fmt.Printf("Checkpoint saved to: %s\n", filepath.Base(checkpointFile))
	fmt.Printf("Tokens before clear: %d\n", p1.TokenCount())

	// Clear
	p1.Clear()
	fmt.Printf("Tokens after clear: %d\n", p1.TokenCount())

	// Restore
	restoredMarking, _ := LoadMarkingFromFile(checkpointFile, registry)
	RestoreMarkingToNet(restoredMarking, net)

	fmt.Printf("Tokens after restore: %d\n", p1.TokenCount())

	// Output:
	// Checkpoint saved to: checkpoint.json
	// Tokens before clear: 1
	// Tokens after clear: 0
	// Tokens after restore: 1
}

// Example_periodicCheckpoint demonstrates periodic checkpoint pattern
func Example_periodicCheckpoint() {
	tmpDir, _ := os.MkdirTemp("", "petri-example")
	defer os.RemoveAll(tmpDir)

	registry := token.NewMapRegistry()
	registry.Register("StringToken", reflect.TypeOf(token.StringToken{}))

	net := petri.NewPetriNet("workflow", "Periodic Checkpoint")
	p1 := petri.NewPlace("p1", "Place 1", 10)
	net.AddPlace(p1)
	net.Resolve()

	clk := clock.NewRealTimeClock()

	// Simulate workflow with periodic checkpoints
	checkpoints := []string{}

	for i := 1; i <= 3; i++ {
		// Add a token
		tok := &token.Token{
			ID:   fmt.Sprintf("tok%d", i),
			Data: &token.StringToken{Value: fmt.Sprintf("step%d", i)},
		}
		p1.AddToken(tok)

		// Create checkpoint
		checkpointFile := filepath.Join(tmpDir, fmt.Sprintf("checkpoint-%d.json", i))
		marking, _ := NewMarking(net, clk)
		SaveMarkingToFile(marking, checkpointFile, registry)

		checkpoints = append(checkpoints, filepath.Base(checkpointFile))
		fmt.Printf("Step %d: Checkpoint created, %d tokens\n", i, p1.TokenCount())
	}

	fmt.Printf("\nCheckpoints created: %v\n", checkpoints)

	// Output:
	// Step 1: Checkpoint created, 1 tokens
	// Step 2: Checkpoint created, 2 tokens
	// Step 3: Checkpoint created, 3 tokens
	//
	// Checkpoints created: [checkpoint-1.json checkpoint-2.json checkpoint-3.json]
}

// Example_failureRecovery demonstrates failure recovery pattern
func Example_failureRecovery() {
	tmpDir, _ := os.MkdirTemp("", "petri-example")
	defer os.RemoveAll(tmpDir)
	checkpointFile := filepath.Join(tmpDir, "last-checkpoint.json")

	registry := token.NewMapRegistry()
	registry.Register("StringToken", reflect.TypeOf(token.StringToken{}))

	// Create workflow
	net := petri.NewPetriNet("workflow", "Failure Recovery")
	start := petri.NewPlace("start", "Start", 10)
	end := petri.NewPlace("end", "End", 10)
	net.AddPlace(start)
	net.AddPlace(end)

	t1 := petri.NewTransition("t1", "Process")
	net.AddTransition(t1)

	inArc := petri.NewArc("a1", start.ID, t1.ID, 1)
	outArc := petri.NewArc("a2", t1.ID, end.ID, 1)
	net.AddArc(inArc)
	net.AddArc(outArc)
	net.Resolve()

	// Initial state
	start.AddToken(&token.Token{ID: "tok1", Data: &token.StringToken{Value: "work"}})

	// Checkpoint before processing
	clk := clock.NewRealTimeClock()
	marking, _ := NewMarking(net, clk)
	SaveMarkingToFile(marking, checkpointFile, registry)

	fmt.Printf("Initial state: start=%d, end=%d\n", start.TokenCount(), end.TokenCount())
	fmt.Println("Checkpoint saved before processing")

	// Process (transition fires)
	ctx := jazzcontext.NewExecutionContext(context.Background(), clk, nil)
	t1.Fire(ctx)

	fmt.Printf("After processing: start=%d, end=%d\n", start.TokenCount(), end.TokenCount())

	// Simulate failure - restore to checkpoint
	fmt.Println("\nSimulating failure - restoring from checkpoint...")
	restoredMarking, _ := LoadMarkingFromFile(checkpointFile, registry)
	RestoreMarkingToNet(restoredMarking, net)

	fmt.Printf("After restore: start=%d, end=%d\n", start.TokenCount(), end.TokenCount())

	// Output:
	// Initial state: start=1, end=0
	// Checkpoint saved before processing
	// After processing: start=0, end=1
	//
	// Simulating failure - restoring from checkpoint...
	// After restore: start=1, end=0
}

// Example_stateMigration demonstrates migrating state between workflow versions
func Example_stateMigration() {
	tmpDir, _ := os.MkdirTemp("", "petri-example")
	defer os.RemoveAll(tmpDir)
	checkpointFile := filepath.Join(tmpDir, "state.json")

	registry := token.NewMapRegistry()
	registry.Register("StringToken", reflect.TypeOf(token.StringToken{}))

	// Version 1 of workflow
	v1Net := petri.NewPetriNet("workflow-v1", "Version 1")
	v1Place := petri.NewPlace("p1", "Place 1", 10)
	v1Net.AddPlace(v1Place)
	v1Net.Resolve()

	// Add data
	v1Place.AddToken(&token.Token{ID: "tok1", Data: &token.StringToken{Value: "migrate-me"}})

	// Save state
	clk := clock.NewRealTimeClock()
	marking, _ := NewMarking(v1Net, clk)
	SaveMarkingToFile(marking, checkpointFile, registry)

	fmt.Println("Version 1 state saved")
	fmt.Printf("V1 tokens: %d\n", v1Place.TokenCount())

	// Version 2 of workflow (same place IDs for compatibility)
	v2Net := petri.NewPetriNet("workflow-v2", "Version 2")
	v2Place := petri.NewPlace("p1", "Place 1 Enhanced", 20) // Increased capacity
	v2Net.AddPlace(v2Place)
	v2Net.Resolve()

	// Migrate state
	restoredMarking, _ := LoadMarkingFromFile(checkpointFile, registry)
	RestoreMarkingToNet(restoredMarking, v2Net)

	fmt.Println("\nState migrated to Version 2")
	fmt.Printf("V2 tokens: %d\n", v2Place.TokenCount())
	fmt.Printf("V2 capacity: %d\n", v2Place.Capacity)

	// Output:
	// Version 1 state saved
	// V1 tokens: 1
	//
	// State migrated to Version 2
	// V2 tokens: 1
	// V2 capacity: 20
}
