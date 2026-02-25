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
	"testing"
	"time"

	"github.com/jazzpetri/engine/clock"
	execContext "github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/petri"
	"github.com/jazzpetri/engine/task"
	"github.com/jazzpetri/engine/token"
)

// ============================================================================
// Engine Start/Stop Not Idempotent
// ============================================================================

// TestEngine_Start_Idempotent tests:
// Calling Start() twice returns error instead of being idempotent
//
// EXPECTED FAILURE: Currently returns error on second Start()
// Expected behavior: Start() should succeed if already running (no-op)
func TestEngine_Start_Idempotent(t *testing.T) {
	// Arrange
	net := createSimpleNet(t)
	ctx := execContext.NewExecutionContext(
		context.Background(),
		clock.NewVirtualClock(time.Now()),
		nil,
	)
	config := DefaultConfig()
	config.PollInterval = 10 * time.Millisecond
	engine := NewEngine(net, ctx, config)

	// Act: Start the engine
	err := engine.Start()
	if err != nil {
		t.Fatalf("First Start() failed: %v", err)
	}

	// Act: Start again (should be idempotent)
	err = engine.Start()

	// Assert
	// EXPECTED FAILURE: This currently returns "engine already running" error
	// After fix: Should return nil (idempotent operation)
	if err != nil {
		t.Errorf("EXPECTED FAILURE: Start() not idempotent, got error: %v", err)
		t.Logf("After fix: Second Start() should return nil (no-op)")
	}

	// Cleanup
	engine.Stop()
}

// TestEngine_Stop_Idempotent tests:
// Calling Stop() twice returns error instead of being idempotent
//
// EXPECTED FAILURE: Currently returns error on second Stop()
// Expected behavior: Stop() should succeed if already stopped (no-op)
func TestEngine_Stop_Idempotent(t *testing.T) {
	// Arrange
	net := createSimpleNet(t)
	ctx := execContext.NewExecutionContext(
		context.Background(),
		clock.NewVirtualClock(time.Now()),
		nil,
	)
	config := DefaultConfig()
	engine := NewEngine(net, ctx, config)

	// Start and stop the engine
	err := engine.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	time.Sleep(10 * time.Millisecond)

	err = engine.Stop()
	if err != nil {
		t.Fatalf("First Stop() failed: %v", err)
	}

	// Act: Stop again (should be idempotent)
	err = engine.Stop()

	// Assert
	// EXPECTED FAILURE: This currently returns "engine not running" error
	// After fix: Should return nil (idempotent operation)
	if err != nil {
		t.Errorf("EXPECTED FAILURE: Stop() not idempotent, got error: %v", err)
		t.Logf("After fix: Second Stop() should return nil (no-op)")
	}
}

// TestEngine_StartStopCycles tests:
// Multiple start/stop cycles should work without errors
//
// EXPECTED FAILURE: May fail if Start/Stop not idempotent
// Expected behavior: Should handle multiple cycles gracefully
func TestEngine_StartStopCycles(t *testing.T) {
	// Arrange
	net := createSimpleNet(t)
	ctx := execContext.NewExecutionContext(
		context.Background(),
		clock.NewVirtualClock(time.Now()),
		nil,
	)
	config := DefaultConfig()
	config.PollInterval = 10 * time.Millisecond
	engine := NewEngine(net, ctx, config)

	// Act: Multiple start/stop cycles
	for i := 0; i < 5; i++ {
		err := engine.Start()
		if err != nil {
			t.Errorf("EXPECTED FAILURE: Cycle %d: Start() failed: %v", i, err)
		}
		time.Sleep(10 * time.Millisecond)

		err = engine.Stop()
		if err != nil {
			t.Errorf("EXPECTED FAILURE: Cycle %d: Stop() failed: %v", i, err)
		}
	}

	t.Logf("After fix: All start/stop cycles should succeed")
}

// ============================================================================
// No Way to Pause/Resume Engine
// ============================================================================

// TestEngine_Pause tests:
// No Pause() method exists to pause execution
//
// EXPECTED FAILURE: Pause() method doesn't exist
// Expected behavior: New Pause() method that pauses transition firing
func TestEngine_Pause(t *testing.T) {
	// Arrange
	net := createNetWithTransitions(t)
	ctx := execContext.NewExecutionContext(
		context.Background(),
		clock.NewVirtualClock(time.Now()),
		nil,
	)

	// Add initial token to start workflow
	place, _ := net.GetPlace("p1")
	place.AddToken(&token.Token{
		ID:   "t1",
		Data: &petri.MapToken{Value: map[string]interface{}{"data": "test-data"}},
	})

	config := DefaultConfig()
	config.PollInterval = 10 * time.Millisecond
	engine := NewEngine(net, ctx, config)

	err := engine.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	time.Sleep(20 * time.Millisecond) // Let some transitions fire

	// Act & Assert
	// EXPECTED FAILURE: Pause() method doesn't exist - won't compile
	// After fix: Should compile and pause the engine
	/*
	err = engine.Pause()
	if err != nil {
		t.Errorf("EXPECTED FAILURE: Pause() failed: %v", err)
	}

	// Verify paused state
	if !engine.IsPaused() {
		t.Errorf("EXPECTED FAILURE: Engine not in paused state")
	}

	// Verify transitions don't fire while paused
	metricsBefore := ctx.Metrics.Get("engine_transitions_fired_total")
	time.Sleep(100 * time.Millisecond)
	metricsAfter := ctx.Metrics.Get("engine_transitions_fired_total")

	if metricsAfter > metricsBefore {
		t.Errorf("EXPECTED FAILURE: Transitions fired while paused")
	}
	*/

	t.Logf("EXPECTED FAILURE: Pause() method doesn't exist yet")
	t.Logf("After fix: Implement Pause() method")

	// Cleanup
	engine.Stop()
}

// TestEngine_Resume tests:
// No Resume() method exists to resume execution after pause
//
// EXPECTED FAILURE: Resume() method doesn't exist
// Expected behavior: New Resume() method that resumes transition firing
func TestEngine_Resume(t *testing.T) {
	// Arrange
	net := createNetWithTransitions(t)
	ctx := execContext.NewExecutionContext(
		context.Background(),
		clock.NewVirtualClock(time.Now()),
		nil,
	)
	config := DefaultConfig()
	config.PollInterval = 10 * time.Millisecond
	_ = NewEngine(net, ctx, config)

	// Act & Assert
	// EXPECTED FAILURE: Resume() method doesn't exist - won't compile
	// After fix: Should compile and resume the engine
	/*
	err := engine.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	err = engine.Pause()
	if err != nil {
		t.Fatalf("Pause() failed: %v", err)
	}

	err = engine.Resume()
	if err != nil {
		t.Errorf("EXPECTED FAILURE: Resume() failed: %v", err)
	}

	// Verify resumed state
	if engine.IsPaused() {
		t.Errorf("EXPECTED FAILURE: Engine still in paused state")
	}

	// Verify transitions fire after resume
	place, _ := net.GetPlace("p1")
	place.AddToken(token.NewToken("t1", "test-data"))

	metricsBefore := ctx.Metrics.Get("engine_transitions_fired_total")
	time.Sleep(100 * time.Millisecond)
	metricsAfter := ctx.Metrics.Get("engine_transitions_fired_total")

	if metricsAfter <= metricsBefore {
		t.Errorf("EXPECTED FAILURE: Transitions didn't fire after resume")
	}
	*/

	t.Logf("EXPECTED FAILURE: Resume() method doesn't exist yet")
	t.Logf("After fix: Implement Resume() method")
}

// TestEngine_PauseResumeCycles tests:
// Multiple pause/resume cycles should work
//
// EXPECTED FAILURE: Pause/Resume methods don't exist
// Expected behavior: Should handle multiple pause/resume cycles
func TestEngine_PauseResumeCycles(t *testing.T) {
	t.Logf("EXPECTED FAILURE: Pause() and Resume() methods don't exist yet")
	t.Logf("After fix: Test multiple pause/resume cycles")
}

// TestEngine_PauseWhileStopped tests:
// Pausing a stopped engine should return error
//
// EXPECTED FAILURE: Pause() method doesn't exist
// Expected behavior: Pause() should return error if engine is stopped
func TestEngine_PauseWhileStopped(t *testing.T) {
	// Arrange
	net := createSimpleNet(t)
	ctx := execContext.NewExecutionContext(
		context.Background(),
		clock.NewVirtualClock(time.Now()),
		nil,
	)
	config := DefaultConfig()
	_ = NewEngine(net, ctx, config)

	// Act & Assert
	// EXPECTED FAILURE: Pause() method doesn't exist
	/*
	err := engine.Pause()
	if err == nil {
		t.Errorf("EXPECTED FAILURE: Pause() on stopped engine should return error")
	}
	*/

	t.Logf("EXPECTED FAILURE: Pause() method doesn't exist yet")
	t.Logf("After fix: Pause() on stopped engine should return error")
}

// ============================================================================
// TryStep() - Already Fixed, Verify It Works
// ============================================================================

// TestEngine_TryStep_VerifyFixed tests:
// TryStep() should distinguish between idle and error states
//
// THIS SHOULD PASS: TryStep() 
// Verifies the fix works correctly
func TestEngine_TryStep_VerifyFixed(t *testing.T) {
	// Arrange
	net := createSimpleNet(t)
	ctx := execContext.NewExecutionContext(
		context.Background(),
		clock.NewVirtualClock(time.Now()),
		nil,
	)
	config := Config{
		Mode:     ModeSequential,
		StepMode: true,
	}
	engine := NewEngine(net, ctx, config)

	// Test 1: No transitions enabled (idle workflow)
	fired, err := engine.TryStep()

	// Assert: Should return (false, nil) - not an error, just idle
	if err != nil {
		t.Errorf("TryStep() returned error for idle workflow: %v", err)
	}
	if fired {
		t.Errorf("TryStep() returned fired=true for idle workflow")
	}

	// Test 2: Add token to enable transition
	place, _ := net.GetPlace("p1")
	place.AddToken(&token.Token{
		ID:   "t1",
		Data: &petri.MapToken{Value: map[string]interface{}{"data": "test-data"}},
	})

	fired, err = engine.TryStep()

	// Assert: Should return (true, nil) - transition fired successfully
	if err != nil {
		t.Errorf("TryStep() returned error when transition enabled: %v", err)
	}
	if !fired {
		t.Errorf("TryStep() returned fired=false when transition was enabled")
	}

	t.Logf("PASS: TryStep() works correctly")
}

// ============================================================================
// No Way to Inspect Engine State
// ============================================================================

// TestEngine_GetState tests:
// No GetState() method to query engine state
//
// EXPECTED FAILURE: GetState() method doesn't exist
// Expected behavior: New GetState() method returning enum (Running/Stopped/Paused)
func TestEngine_GetState(t *testing.T) {
	// Arrange
	net := createSimpleNet(t)
	ctx := execContext.NewExecutionContext(
		context.Background(),
		clock.NewVirtualClock(time.Now()),
		nil,
	)
	config := DefaultConfig()
	_ = NewEngine(net, ctx, config)

	// Act & Assert
	// EXPECTED FAILURE: GetState() method doesn't exist - won't compile
	/*
	// Test stopped state
	state := engine.GetState()
	if state != EngineStopped {
		t.Errorf("EXPECTED FAILURE: Initial state should be Stopped, got %v", state)
	}

	// Test running state
	engine.Start()
	state = engine.GetState()
	if state != EngineRunning {
		t.Errorf("EXPECTED FAILURE: State should be Running, got %v", state)
	}

	// Test paused state (if Pause is implemented)
	engine.Pause()
	state = engine.GetState()
	if state != EnginePaused {
		t.Errorf("EXPECTED FAILURE: State should be Paused, got %v", state)
	}

	engine.Stop()
	*/

	t.Logf("EXPECTED FAILURE: GetState() method doesn't exist yet")
	t.Logf("After fix: Implement GetState() returning EngineState enum")
}

// TestEngine_IsRunning_AlreadyExists tests:
// IsRunning() already exists and should work
//
// THIS SHOULD PASS: IsRunning() already implemented
func TestEngine_IsRunning_AlreadyExists(t *testing.T) {
	// Arrange
	net := createSimpleNet(t)
	ctx := execContext.NewExecutionContext(
		context.Background(),
		clock.NewVirtualClock(time.Now()),
		nil,
	)
	config := DefaultConfig()
	engine := NewEngine(net, ctx, config)

	// Assert: Should not be running initially
	if engine.IsRunning() {
		t.Errorf("Engine should not be running initially")
	}

	// Start engine
	err := engine.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Assert: Should be running
	if !engine.IsRunning() {
		t.Errorf("Engine should be running after Start()")
	}

	// Stop engine
	time.Sleep(10 * time.Millisecond)
	err = engine.Stop()
	if err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}

	// Assert: Should not be running
	if engine.IsRunning() {
		t.Errorf("Engine should not be running after Stop()")
	}

	t.Logf("PASS: IsRunning() works correctly")
}

// TestEngine_IsPaused tests:
// No IsPaused() method to check if engine is paused
//
// EXPECTED FAILURE: IsPaused() method doesn't exist
// Expected behavior: New IsPaused() method
func TestEngine_IsPaused(t *testing.T) {
	// EXPECTED FAILURE: IsPaused() method doesn't exist - won't compile
	/*
	net := createSimpleNet(t)
	ctx := execContext.NewExecutionContext(
		context.Background(),
		clock.NewVirtualClock(time.Now()),
		nil,
	)
	config := DefaultConfig()
	engine := NewEngine(net, ctx, config)

	// Should not be paused initially
	if engine.IsPaused() {
		t.Errorf("Engine should not be paused initially")
	}

	engine.Start()
	engine.Pause()

	// Should be paused
	if !engine.IsPaused() {
		t.Errorf("Engine should be paused after Pause()")
	}

	engine.Resume()

	// Should not be paused
	if engine.IsPaused() {
		t.Errorf("Engine should not be paused after Resume()")
	}

	engine.Stop()
	*/

	t.Logf("EXPECTED FAILURE: IsPaused() method doesn't exist yet")
	t.Logf("After fix: Implement IsPaused() method")
}

// ============================================================================
// No Workflow Execution Statistics
// ============================================================================

// TestEngine_GetStatistics tests:
// No GetStatistics() method to retrieve execution stats
//
// EXPECTED FAILURE: GetStatistics() method doesn't exist
// Expected behavior: New GetStatistics() returning struct with stats
func TestEngine_GetStatistics(t *testing.T) {
	// Arrange
	net := createNetWithTransitions(t)
	ctx := execContext.NewExecutionContext(
		context.Background(),
		clock.NewVirtualClock(time.Now()),
		nil,
	)

	// Add tokens to start workflow
	place, _ := net.GetPlace("p1")
	place.AddToken(&token.Token{
		ID:   "t1",
		Data: &petri.MapToken{Value: map[string]interface{}{"data": "data1"}},
	})
	place.AddToken(&token.Token{
		ID:   "t2",
		Data: &petri.MapToken{Value: map[string]interface{}{"data": "data2"}},
	})

	config := DefaultConfig()
	config.PollInterval = 10 * time.Millisecond
	engine := NewEngine(net, ctx, config)

	// Act: Run engine for a bit
	engine.Start()
	time.Sleep(100 * time.Millisecond)
	engine.Stop()

	// Assert
	// EXPECTED FAILURE: GetStatistics() method doesn't exist - won't compile
	/*
	stats := engine.GetStatistics()

	// Verify statistics are tracked
	if stats.TransitionsFired == 0 {
		t.Errorf("EXPECTED FAILURE: TransitionsFired should be > 0")
	}

	if stats.TokensCreated == 0 {
		t.Errorf("EXPECTED FAILURE: TokensCreated should be > 0")
	}

	if stats.ExecutionTime == 0 {
		t.Errorf("EXPECTED FAILURE: ExecutionTime should be > 0")
	}

	if stats.TotalSteps == 0 {
		t.Errorf("EXPECTED FAILURE: TotalSteps should be > 0")
	}

	t.Logf("Statistics: Transitions=%d, Tokens=%d, Time=%v, Steps=%d",
		stats.TransitionsFired, stats.TokensCreated, stats.ExecutionTime, stats.TotalSteps)
	*/

	t.Logf("EXPECTED FAILURE: GetStatistics() method doesn't exist yet")
	t.Logf("After fix: Implement GetStatistics() returning ExecutionStatistics struct")
	t.Logf("Should track: TransitionsFired, TokensCreated, ExecutionTime, TotalSteps")
}

// ============================================================================
// Helper Functions
// ============================================================================

// createSimpleNet creates a basic Petri net for testing
func createSimpleNet(t *testing.T) *petri.PetriNet {
	t.Helper()

	net := petri.NewPetriNet("test-net", "Test Net")

	p1 := petri.NewPlace("p1", "Place 1", 10)
	p2 := petri.NewPlace("p2", "Place 2", 10)

	net.AddPlace(p1)
	net.AddPlace(p2)

	trans := petri.NewTransition("t1", "Transition 1")
	net.AddTransition(trans)

	arc1 := petri.NewArc("a1", "p1", "t1", 1)
	arc2 := petri.NewArc("a2", "t1", "p2", 1)

	net.AddArc(arc1)
	net.AddArc(arc2)

	err := net.Resolve()
	if err != nil {
		t.Fatalf("Failed to resolve net: %v", err)
	}

	return net
}

// createNetWithTransitions creates a net with tasks for testing
func createNetWithTransitions(t *testing.T) *petri.PetriNet {
	t.Helper()

	net := petri.NewPetriNet("test-net", "Test Net")

	p1 := petri.NewPlace("p1", "Place 1", 10)
	p2 := petri.NewPlace("p2", "Place 2", 10)
	p3 := petri.NewPlace("p3", "Place 3", 10)

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddPlace(p3)

	task1 := &task.InlineTask{
		Name: "task1",
		Fn: func(ctx *execContext.ExecutionContext) error {
			time.Sleep(5 * time.Millisecond)
			return nil
		},
	}

	trans := petri.NewTransition("t1", "Transition 1").WithTask(task1)
	net.AddTransition(trans)

	arc1 := petri.NewArc("a1", "p1", "t1", 1)
	arc2 := petri.NewArc("a2", "t1", "p2", 1)

	net.AddArc(arc1)
	net.AddArc(arc2)

	err := net.Resolve()
	if err != nil {
		t.Fatalf("Failed to resolve net: %v", err)
	}

	return net
}
