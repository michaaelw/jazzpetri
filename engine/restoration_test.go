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
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	jazzcontext "github.com/jazzpetri/engine/context"

	"github.com/jazzpetri/engine/clock"
	"github.com/jazzpetri/engine/petri"
	"github.com/jazzpetri/engine/state"
	"github.com/jazzpetri/engine/token"
)

// TestTokenData is a test token type
type TestTokenData struct {
	Value string
}

func (t *TestTokenData) Type() string {
	return "test"
}

func TestEngine_RestoreFromMarking(t *testing.T) {
	t.Run("restore while stopped", func(t *testing.T) {
		// Create engine
		net := createTestNet(t)
		net.Resolve()
		clk := clock.NewVirtualClock(time.Now())
		ctx := jazzcontext.NewExecutionContext(context.Background(), clk, nil)
		engine := NewEngine(net, ctx, DefaultConfig())

		// Add tokens
		p1 := net.Places["p1"]
		tok := &token.Token{ID: "tok1", Data: &TestTokenData{Value: "test"}}
		p1.AddToken(tok)

		// Capture marking
		marking, err := engine.GetMarking()
		if err != nil {
			t.Fatalf("failed to get marking: %v", err)
		}

		// Clear
		p1.Clear()

		// Restore
		err = engine.RestoreFromMarking(marking)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Verify restored
		if p1.TokenCount() != 1 {
			t.Errorf("expected 1 token after restore, got %d", p1.TokenCount())
		}
	})

	t.Run("restore while running fails", func(t *testing.T) {
		net := createTestNet(t)
		net.Resolve()
		clk := clock.NewVirtualClock(time.Now())
		ctx := jazzcontext.NewExecutionContext(context.Background(), clk, nil)
		engine := NewEngine(net, ctx, DefaultConfig())

		// Start engine
		engine.Start()
		defer engine.Stop()

		// Try to restore
		marking, _ := engine.GetMarking()
		err := engine.RestoreFromMarking(marking)

		if err == nil {
			t.Error("expected error when restoring while running")
		}
		// Error message now includes state 
		if !strings.Contains(err.Error(), "cannot restore while engine is running") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("restore with nil marking fails", func(t *testing.T) {
		net := createTestNet(t)
		net.Resolve()
		clk := clock.NewVirtualClock(time.Now())
		ctx := jazzcontext.NewExecutionContext(context.Background(), clk, nil)
		engine := NewEngine(net, ctx, DefaultConfig())

		err := engine.RestoreFromMarking(nil)
		if err == nil {
			t.Error("expected error for nil marking")
		}
	})
}

func TestEngine_RestoreFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	checkpointFile := filepath.Join(tmpDir, "checkpoint.json")

	registry := token.NewMapRegistry()
	registry.Register("test", reflect.TypeOf(TestTokenData{}))

	// Create engine
	net := createTestNet(t)
	net.Resolve()
	clk := clock.NewVirtualClock(time.Now())
	ctx := jazzcontext.NewExecutionContext(context.Background(), clk, nil)
	engine := NewEngine(net, ctx, DefaultConfig())

	// Add tokens
	p1 := net.Places["p1"]
	tok := &token.Token{ID: "tok1", Data: &TestTokenData{Value: "test"}}
	p1.AddToken(tok)

	// Save checkpoint
	err := engine.Checkpoint(checkpointFile, registry)
	if err != nil {
		t.Fatalf("failed to checkpoint: %v", err)
	}

	// Clear
	p1.Clear()

	// Restore from file
	err = engine.RestoreFromFile(checkpointFile, registry)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify restored
	if p1.TokenCount() != 1 {
		t.Errorf("expected 1 token after restore, got %d", p1.TokenCount())
	}
}

func TestEngine_Checkpoint(t *testing.T) {
	tmpDir := t.TempDir()
	checkpointFile := filepath.Join(tmpDir, "checkpoint.json")

	registry := token.NewMapRegistry()
	registry.Register("test", reflect.TypeOf(TestTokenData{}))

	net := createTestNet(t)
	net.Resolve()
	clk := clock.NewVirtualClock(time.Now())
	ctx := jazzcontext.NewExecutionContext(context.Background(), clk, nil)
	engine := NewEngine(net, ctx, DefaultConfig())

	// Add tokens
	p1 := net.Places["p1"]
	tok := &token.Token{ID: "tok1", Data: &TestTokenData{Value: "test"}}
	p1.AddToken(tok)

	// Checkpoint
	err := engine.Checkpoint(checkpointFile, registry)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify file created
	if _, err := os.Stat(checkpointFile); os.IsNotExist(err) {
		t.Error("checkpoint file was not created")
	}

	// Verify can load
	marking, err := state.LoadMarkingFromFile(checkpointFile, registry)
	if err != nil {
		t.Errorf("failed to load checkpoint: %v", err)
	}

	if marking.PlaceTokenCount("p1") != 1 {
		t.Errorf("expected 1 token in checkpoint, got %d", marking.PlaceTokenCount("p1"))
	}
}

func TestEngine_CheckpointWithEvents(t *testing.T) {
	tmpDir := t.TempDir()
	checkpointFile := filepath.Join(tmpDir, "checkpoint.json")

	registry := token.NewMapRegistry()
	registry.Register("test", reflect.TypeOf(TestTokenData{}))

	net := createTestNet(t)
	net.Resolve()
	clk := clock.NewVirtualClock(time.Now())

	// Create context with event log
	eventLog := state.NewMemoryEventLog()
	eventBuilder := state.NewDefaultEventBuilder(clk)
	ctx := jazzcontext.NewExecutionContext(context.Background(), clk, nil)
	ctx = ctx.WithEventLog(eventLog)
	ctx = ctx.WithEventBuilder(eventBuilder)

	engine := NewEngine(net, ctx, DefaultConfig())

	// Add tokens
	p1 := net.Places["p1"]
	tok := &token.Token{ID: "tok1", Data: &TestTokenData{Value: "test"}}
	p1.AddToken(tok)

	// Start and stop to generate events
	engine.Start()
	time.Sleep(50 * time.Millisecond)
	engine.Stop()

	// Checkpoint with events
	err := engine.CheckpointWithEvents(checkpointFile, registry)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify file created
	if _, err := os.Stat(checkpointFile); os.IsNotExist(err) {
		t.Error("checkpoint file was not created")
	}
}

func TestEngine_CheckpointWithEvents_NoEventLog(t *testing.T) {
	tmpDir := t.TempDir()
	checkpointFile := filepath.Join(tmpDir, "checkpoint.json")

	registry := token.NewMapRegistry()
	registry.Register("test", reflect.TypeOf(TestTokenData{}))

	net := createTestNet(t)
	net.Resolve()
	clk := clock.NewVirtualClock(time.Now())
	ctx := jazzcontext.NewExecutionContext(context.Background(), clk, nil)
	// No event log configured

	engine := NewEngine(net, ctx, DefaultConfig())

	// Checkpoint with events (should still work, just saves marking)
	err := engine.CheckpointWithEvents(checkpointFile, registry)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify file created
	if _, err := os.Stat(checkpointFile); os.IsNotExist(err) {
		t.Error("checkpoint file was not created")
	}
}

// Helper to create a test net
func createTestNet(t *testing.T) *petri.PetriNet {
	t.Helper()

	net := petri.NewPetriNet("test_net", "Test Net")
	p1 := petri.NewPlace("p1", "Place 1", 10)
	p2 := petri.NewPlace("p2", "Place 2", 10)

	net.AddPlace(p1)
	net.AddPlace(p2)

	return net
}
