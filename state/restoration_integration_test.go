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
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	jazzcontext "github.com/jazzpetri/engine/context"

	"github.com/jazzpetri/engine/clock"
	"github.com/jazzpetri/engine/petri"
	"github.com/jazzpetri/engine/token"
)

func TestRestoration_CompleteCheckpointRestoreCycle(t *testing.T) {
	// Create test directory for checkpoints
	tmpDir := t.TempDir()
	checkpointFile := filepath.Join(tmpDir, "checkpoint.json")

	// Setup token registry
	registry := token.NewMapRegistry()
	err := registry.Register("test", reflect.TypeOf(TestTokenData{}))
	if err != nil {
		t.Fatalf("failed to register token type: %v", err)
	}

	// Create a workflow net
	net := createWorkflowNet(t)
	net.Resolve()

	// Add initial tokens
	p1 := net.Places["start"]
	tok1 := &token.Token{ID: "tok1", Data: &TestTokenData{Value: "data1"}}
	tok2 := &token.Token{ID: "tok2", Data: &TestTokenData{Value: "data2"}}
	p1.AddToken(tok1)
	p1.AddToken(tok2)

	p2 := net.Places["processing"]
	tok3 := &token.Token{ID: "tok3", Data: &TestTokenData{Value: "data3"}}
	p2.AddToken(tok3)

	// Capture marking
	clk := clock.NewVirtualClock(time.Now())
	marking, err := NewMarking(net, clk)
	if err != nil {
		t.Fatalf("failed to capture marking: %v", err)
	}

	// Save to file
	err = SaveMarkingToFile(marking, checkpointFile, registry)
	if err != nil {
		t.Fatalf("failed to save checkpoint: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(checkpointFile); os.IsNotExist(err) {
		t.Fatal("checkpoint file was not created")
	}

	// Clear the net
	for _, place := range net.Places {
		place.Clear()
	}

	// Verify net is cleared
	if p1.TokenCount() != 0 || p2.TokenCount() != 0 {
		t.Fatal("net should be cleared")
	}

	// Restore from file
	restoredMarking, err := LoadMarkingFromFile(checkpointFile, registry)
	if err != nil {
		t.Fatalf("failed to load marking: %v", err)
	}

	err = RestoreMarkingToNet(restoredMarking, net)
	if err != nil {
		t.Fatalf("failed to restore marking: %v", err)
	}

	// Verify state matches original
	if p1.TokenCount() != 2 {
		t.Errorf("expected 2 tokens in start place, got %d", p1.TokenCount())
	}
	if p2.TokenCount() != 1 {
		t.Errorf("expected 1 token in processing place, got %d", p2.TokenCount())
	}

	// Verify token IDs preserved
	removed1, _ := p1.RemoveToken()
	removed2, _ := p1.RemoveToken()
	removed3, _ := p2.RemoveToken()

	if removed1.ID != "tok1" || removed2.ID != "tok2" || removed3.ID != "tok3" {
		t.Error("token IDs not preserved after restore")
	}

	// Verify token data preserved
	data1, ok := removed1.Data.(*TestTokenData)
	if !ok || data1.Value != "data1" {
		t.Error("token data not preserved")
	}
}

func TestRestoration_MultipleCheckpointRestoreCycles(t *testing.T) {
	tmpDir := t.TempDir()
	registry := token.NewMapRegistry()
	registry.Register("test", reflect.TypeOf(TestTokenData{}))

	net := createWorkflowNet(t)
	net.Resolve()
	clk := clock.NewVirtualClock(time.Now())

	// Cycle 1: Start with 1 token
	p1 := net.Places["start"]
	tok1 := &token.Token{ID: "tok1", Data: &TestTokenData{Value: "v1"}}
	p1.AddToken(tok1)

	checkpoint1 := filepath.Join(tmpDir, "checkpoint1.json")
	marking1, _ := NewMarking(net, clk)
	SaveMarkingToFile(marking1, checkpoint1, registry)

	// Cycle 2: Add more tokens
	tok2 := &token.Token{ID: "tok2", Data: &TestTokenData{Value: "v2"}}
	p1.AddToken(tok2)

	checkpoint2 := filepath.Join(tmpDir, "checkpoint2.json")
	marking2, _ := NewMarking(net, clk)
	SaveMarkingToFile(marking2, checkpoint2, registry)

	// Restore to checkpoint 1
	restoredMarking1, _ := LoadMarkingFromFile(checkpoint1, registry)
	RestoreMarkingToNet(restoredMarking1, net)

	if p1.TokenCount() != 1 {
		t.Errorf("expected 1 token after restoring checkpoint1, got %d", p1.TokenCount())
	}

	// Restore to checkpoint 2
	restoredMarking2, _ := LoadMarkingFromFile(checkpoint2, registry)
	RestoreMarkingToNet(restoredMarking2, net)

	if p1.TokenCount() != 2 {
		t.Errorf("expected 2 tokens after restoring checkpoint2, got %d", p1.TokenCount())
	}
}

func TestRestoration_ContinueExecutionAfterRestore(t *testing.T) {
	tmpDir := t.TempDir()
	checkpointFile := filepath.Join(tmpDir, "checkpoint.json")

	registry := token.NewMapRegistry()
	registry.Register("test", reflect.TypeOf(TestTokenData{}))

	// Create workflow with transition
	net := createWorkflowWithTransition(t)
	net.Resolve()

	clk := clock.NewVirtualClock(time.Now())

	// Add token to start place
	start := net.Places["start"]
	tok := &token.Token{ID: "tok1", Data: &TestTokenData{Value: "data1"}}
	start.AddToken(tok)

	// Checkpoint before transition
	marking, _ := NewMarking(net, clk)
	SaveMarkingToFile(marking, checkpointFile, registry)

	// Clear and restore
	for _, place := range net.Places {
		place.Clear()
	}
	restoredMarking, _ := LoadMarkingFromFile(checkpointFile, registry)
	RestoreMarkingToNet(restoredMarking, net)

	// Verify token is back in start
	if start.TokenCount() != 1 {
		t.Fatalf("expected 1 token in start after restore, got %d", start.TokenCount())
	}

	// Now fire the transition
	ctx := jazzcontext.NewExecutionContext(context.Background(), clk, nil)
	transition := net.Transitions["t1"]

	enabled, err := transition.IsEnabled(ctx)
	if err != nil {
		t.Fatalf("error checking if enabled: %v", err)
	}
	if !enabled {
		t.Fatal("transition should be enabled after restore")
	}

	err = transition.Fire(ctx)
	if err != nil {
		t.Fatalf("failed to fire transition: %v", err)
	}

	// Verify token moved to end place
	end := net.Places["end"]
	if end.TokenCount() != 1 {
		t.Errorf("expected 1 token in end place after firing, got %d", end.TokenCount())
	}
	if start.TokenCount() != 0 {
		t.Errorf("expected 0 tokens in start place after firing, got %d", start.TokenCount())
	}
}

func TestRestoration_DifferentTokenTypes(t *testing.T) {
	tmpDir := t.TempDir()
	checkpointFile := filepath.Join(tmpDir, "checkpoint.json")

	// Create registry with multiple token types
	registry := token.NewMapRegistry()
	registry.Register("test", reflect.TypeOf(TestTokenData{}))
	registry.Register("StringToken", reflect.TypeOf(token.StringToken{}))

	net := createWorkflowNet(t)
	net.Resolve()
	clk := clock.NewVirtualClock(time.Now())

	// Add different token types
	p1 := net.Places["start"]
	tok1 := &token.Token{ID: "tok1", Data: &TestTokenData{Value: "test"}}
	tok2 := &token.Token{ID: "tok2", Data: &token.StringToken{Value: "hello"}}
	p1.AddToken(tok1)
	p1.AddToken(tok2)

	// Checkpoint and restore
	marking, _ := NewMarking(net, clk)
	SaveMarkingToFile(marking, checkpointFile, registry)

	p1.Clear()

	restoredMarking, err := LoadMarkingFromFile(checkpointFile, registry)
	if err != nil {
		t.Fatalf("failed to load marking: %v", err)
	}

	err = RestoreMarkingToNet(restoredMarking, net)
	if err != nil {
		t.Fatalf("failed to restore: %v", err)
	}

	// Verify both token types restored
	if p1.TokenCount() != 2 {
		t.Errorf("expected 2 tokens, got %d", p1.TokenCount())
	}

	removed1, _ := p1.RemoveToken()
	removed2, _ := p1.RemoveToken()

	// Verify types
	if _, ok := removed1.Data.(*TestTokenData); !ok {
		t.Error("first token should be TestTokenData")
	}
	if _, ok := removed2.Data.(*token.StringToken); !ok {
		t.Error("second token should be StringToken")
	}
}

// Helper to create a workflow net for testing
func createWorkflowNet(t *testing.T) *petri.PetriNet {
	t.Helper()

	net := petri.NewPetriNet("workflow", "Test Workflow")

	start := petri.NewPlace("start", "Start Place", 10)
	processing := petri.NewPlace("processing", "Processing Place", 10)
	end := petri.NewPlace("end", "End Place", 10)

	net.AddPlace(start)
	net.AddPlace(processing)
	net.AddPlace(end)

	return net
}

// Helper to create a workflow with a transition
func createWorkflowWithTransition(t *testing.T) *petri.PetriNet {
	t.Helper()

	net := petri.NewPetriNet("workflow", "Test Workflow")

	start := petri.NewPlace("start", "Start", 10)
	end := petri.NewPlace("end", "End", 10)

	net.AddPlace(start)
	net.AddPlace(end)

	// Create transition
	t1 := petri.NewTransition("t1", "Transition 1")
	net.AddTransition(t1)

	// Create arcs
	inArc := petri.NewArc("a1", start.ID, t1.ID, 1)
	outArc := petri.NewArc("a2", t1.ID, end.ID, 1)
	net.AddArc(inArc)
	net.AddArc(outArc)

	return net
}
