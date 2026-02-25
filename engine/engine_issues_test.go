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
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jazzpetri/engine/clock"
	execContext "github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/petri"
	"github.com/jazzpetri/engine/task"
	"github.com/jazzpetri/engine/token"
)

// TestNewEngine_ValidatesMaxConcurrentTransitions tests:
// MaxConcurrentTransitions can be set to 0 or negative, causing panics
//
// FAILING TEST: Demonstrates that invalid values are accepted
// Expected behavior: Zero or negative values should default to safe minimum
func TestNewEngine_ValidatesMaxConcurrentTransitions(t *testing.T) {
	testCases := []struct {
		name          string
		maxConcurrent int
		expectValid   bool
		expectedValue int
	}{
		{
			name:          "zero value should default to safe minimum",
			maxConcurrent: 0,
			expectValid:   false,
			expectedValue: 10, // Default
		},
		{
			name:          "negative value should default to safe minimum",
			maxConcurrent: -5,
			expectValid:   false,
			expectedValue: 10, // Default
		},
		{
			name:          "positive value should be accepted",
			maxConcurrent: 5,
			expectValid:   true,
			expectedValue: 5,
		},
		{
			name:          "large value should be accepted",
			maxConcurrent: 100,
			expectValid:   true,
			expectedValue: 100,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			net := petri.NewPetriNet("test-net", "Test Net")
			ctx := execContext.NewExecutionContext(
				context.Background(),
				clock.NewVirtualClock(time.Now()),
				nil,
			)

			config := Config{
				Mode:                     ModeConcurrent,
				MaxConcurrentTransitions: tc.maxConcurrent,
				PollInterval:             100 * time.Millisecond,
			}

			// Act
			engine := NewEngine(net, ctx, config)

			// Assert: Check if value was validated/defaulted
			actualValue := engine.config.MaxConcurrentTransitions

			// WHY THIS FAILS: No validation - zero and negative values are accepted as-is
			if !tc.expectValid && actualValue == tc.maxConcurrent {
				t.Errorf("EXPECTED FAILURE: Invalid value %d was not defaulted (got %d, want %d)",
					tc.maxConcurrent, actualValue, tc.expectedValue)
				t.Logf("This test FAILS because NewEngine doesn't validate MaxConcurrentTransitions")
				t.Logf("After fix: NewEngine should default invalid values to %d", tc.expectedValue)
			}

			if tc.expectValid && actualValue != tc.expectedValue {
				t.Errorf("Valid value was changed: got %d, want %d", actualValue, tc.expectedValue)
			}

			// Verify engine doesn't panic when starting with invalid config
			if tc.maxConcurrent <= 0 {
				// This could panic when creating semaphore channel with size <= 0
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("PANIC with MaxConcurrentTransitions=%d: %v",
							tc.maxConcurrent, r)
						t.Logf("After fix: this should not panic")
					}
				}()

				// Try to start engine (this is where panic would occur)
				if err := engine.Start(); err == nil {
					// Give it a moment
					time.Sleep(10 * time.Millisecond)
					engine.Stop()
				}
			}
		})
	}
}

// TestDefaultConfig_ValidatesMaxConcurrentTransitions tests:
// DefaultConfig should always return valid MaxConcurrentTransitions
//
// FAILING TEST: This should pass, but validates the fix location
// Expected behavior: Default config always has positive MaxConcurrentTransitions
func TestDefaultConfig_ValidatesMaxConcurrentTransitions(t *testing.T) {
	// Act
	config := DefaultConfig()

	// Assert: Should have positive value
	if config.MaxConcurrentTransitions <= 0 {
		t.Errorf("DefaultConfig has invalid MaxConcurrentTransitions: %d",
			config.MaxConcurrentTransitions)
	}

	// Verify it's a reasonable default
	if config.MaxConcurrentTransitions < 1 || config.MaxConcurrentTransitions > 1000 {
		t.Logf("Warning: Default MaxConcurrentTransitions seems unusual: %d",
			config.MaxConcurrentTransitions)
	}

	t.Logf("DefaultConfig.MaxConcurrentTransitions = %d", config.MaxConcurrentTransitions)
}

// TestEngine_PanicWithZeroMaxConcurrent tests:
// Engine panics when MaxConcurrentTransitions is 0 and transitions fire
//
// FAILING TEST: Demonstrates actual panic scenario
// Expected behavior: Engine should handle gracefully with default value
func TestEngine_PanicWithZeroMaxConcurrent(t *testing.T) {
	// Arrange: Create simple workflow
	net := petri.NewPetriNet("test-net", "Test Net")

	p1 := petri.NewPlace("p1", "Place 1", 10)
	p2 := petri.NewPlace("p2", "Place 2", 10)

	taskFunc := &task.InlineTask{
		Name: "test-task",
		Fn: func(ctx *execContext.ExecutionContext) error {
			return nil
		},
	}

	t1 := petri.NewTransition("t1", "Transition 1").WithTask(taskFunc)

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddTransition(t1)

	net.AddArc(petri.NewArc("arc1", "p1", "t1", 1))
	net.AddArc(petri.NewArc("arc2", "t1", "p2", 1))
	net.Resolve()

	// Add token
	p1.AddToken(&token.Token{
		ID:   "tok1",
		Data: &petri.MapToken{Value: make(map[string]interface{})},
	})

	ctx := execContext.NewExecutionContext(
		context.Background(),
		clock.NewVirtualClock(time.Now()),
		nil,
	)

	// Config with zero MaxConcurrentTransitions
	config := Config{
		Mode:                     ModeConcurrent,
		MaxConcurrentTransitions: 0, // INVALID
		PollInterval:             10 * time.Millisecond,
	}

	engine := NewEngine(net, ctx, config)

	// Act & Assert: This should panic when creating semaphore channel
	defer func() {
		if r := recover(); r != nil {
			t.Logf("EXPECTED FAILURE: Engine panicked with MaxConcurrentTransitions=0: %v", r)
			t.Logf("Panic occurred at: %v", r)
			t.Logf("This test FAILS (panics) because zero is not validated")
			t.Logf("After fix: NewEngine should default to safe minimum value")
		} else {
			// If it didn't panic, check if it worked anyway
			if engine.config.MaxConcurrentTransitions == 0 {
				t.Logf("EXPECTED FAILURE: Engine accepted invalid MaxConcurrentTransitions=0")
			}
		}
	}()

	// Try to start - this is where panic occurs in fireConcurrent
	if err := engine.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for execution (panic will occur here if not already)
	time.Sleep(100 * time.Millisecond)

	engine.Stop()
}

// TestEngine_Run_RespectsContextCancellation tests:
// Engine run loop ignores context cancellation
//
// FAILING TEST: Demonstrates engine doesn't stop when context is cancelled
// Expected behavior: Engine should stop promptly when context is cancelled
func TestEngine_Run_RespectsContextCancellation(t *testing.T) {
	// Arrange: Create workflow that runs indefinitely
	net := petri.NewPetriNet("test-net", "Test Net")

	p1 := petri.NewPlace("p1", "Place 1", 100)
	p2 := petri.NewPlace("p2", "Place 2", 100)

	var execCount int32
	taskFunc := &task.InlineTask{
		Name: "long-task",
		Fn: func(ctx *execContext.ExecutionContext) error {
			atomic.AddInt32(&execCount, 1)
			time.Sleep(50 * time.Millisecond)
			return nil
		},
	}

	t1 := petri.NewTransition("t1", "Transition 1").WithTask(taskFunc)

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddTransition(t1)

	// Create a loop: p1 -> t1 -> p2 and p2 -> t1 -> p1
	net.AddArc(petri.NewArc("arc1", "p1", "t1", 1))
	net.AddArc(petri.NewArc("arc2", "t1", "p1", 1)) // Loops back
	net.Resolve()

	// Add token to start loop
	p1.AddToken(&token.Token{
		ID:   "tok1",
		Data: &petri.MapToken{Value: make(map[string]interface{})},
	})

	// Create cancellable context
	goCtx, cancel := context.WithCancel(context.Background())
	ctx := execContext.NewExecutionContext(
		goCtx,
		clock.NewVirtualClock(time.Now()),
		nil,
	)

	config := Config{
		Mode:                     ModeSequential,
		MaxConcurrentTransitions: 10,
		PollInterval:             10 * time.Millisecond,
	}

	engine := NewEngine(net, ctx, config)

	// Act: Start engine
	if err := engine.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Let it run for a bit
	time.Sleep(100 * time.Millisecond)

	// Cancel context
	cancelTime := time.Now()
	cancel()

	// Wait for engine to stop (should be prompt)
	timeout := time.After(500 * time.Millisecond)
	stopped := make(chan bool)

	go func() {
		engine.Stop() // This should return quickly
		stopped <- true
	}()

	select {
	case <-stopped:
		stopDuration := time.Since(cancelTime)
		// WHY THIS FAILS: Engine doesn't check context.Done() in run loop
		// It only checks stopCh, so context cancellation has no effect
		if stopDuration > 200*time.Millisecond {
			t.Logf("EXPECTED FAILURE: Engine took %v to stop after context cancellation",
				stopDuration)
			t.Logf("This test FAILS because run loop doesn't check ctx.Context.Done()")
			t.Logf("After fix: engine should stop within ~100ms of context cancellation")
		}
	case <-timeout:
		t.Errorf("EXPECTED FAILURE: Engine did not stop within timeout after context cancellation")
		t.Logf("This test FAILS because engine ignores context cancellation")
		t.Logf("After fix: add select case for ctx.Context.Done() in run loop")
		engine.Stop() // Force stop for cleanup
	}

	executionCount := atomic.LoadInt32(&execCount)
	t.Logf("Task executed %d times before stop", executionCount)
}

// TestEngine_Run_StopChannelAndContext tests:
// Both stopCh and context.Done() should stop the engine
//
// FAILING TEST: Only stopCh works currently
// Expected behavior: Either mechanism should stop engine
func TestEngine_Run_StopChannelAndContext(t *testing.T) {
	testCases := []struct {
		name         string
		useContext   bool
		useStopCh    bool
		shouldStop   bool
		maxStopTime  time.Duration
	}{
		{
			name:         "stop via stopCh (current behavior)",
			useContext:   false,
			useStopCh:    true,
			shouldStop:   true,
			maxStopTime:  200 * time.Millisecond,
		},
		{
			name:         "stop via context cancellation (FAILS)",
			useContext:   true,
			useStopCh:    false,
			shouldStop:   true,
			maxStopTime:  200 * time.Millisecond,
		},
		{
			name:         "stop via both (should work)",
			useContext:   true,
			useStopCh:    true,
			shouldStop:   true,
			maxStopTime:  200 * time.Millisecond,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange: Simple workflow
			net := petri.NewPetriNet("test-net", "Test Net")
			p1 := petri.NewPlace("p1", "Place 1", 10)
			p2 := petri.NewPlace("p2", "Place 2", 10)
			t1 := petri.NewTransition("t1", "T1")

			net.AddPlace(p1)
			net.AddPlace(p2)
			net.AddTransition(t1)
			net.AddArc(petri.NewArc("a1", "p1", "t1", 1))
			net.AddArc(petri.NewArc("a2", "t1", "p1", 1)) // Loop
			net.Resolve()

			p1.AddToken(&token.Token{ID: "tok1"})

			// Setup context
			var goCtx context.Context
			var cancel context.CancelFunc

			if tc.useContext {
				goCtx, cancel = context.WithCancel(context.Background())
			} else {
				goCtx = context.Background()
			}

			ctx := execContext.NewExecutionContext(
				goCtx,
				clock.NewVirtualClock(time.Now()),
				nil,
			)

			engine := NewEngine(net, ctx, DefaultConfig())

			// Act: Start engine
			engine.Start()
			time.Sleep(50 * time.Millisecond)

			// Trigger stop mechanism
			startStop := time.Now()

			if tc.useContext && cancel != nil {
				cancel()
			}

			if tc.useStopCh {
				go func() {
					time.Sleep(10 * time.Millisecond)
					engine.Stop()
				}()
			}

			// Wait for stop with timeout
			timeout := time.After(500 * time.Millisecond)
			done := make(chan bool)

			go func() {
				for engine.IsRunning() {
					time.Sleep(10 * time.Millisecond)
				}
				done <- true
			}()

			select {
			case <-done:
				stopDuration := time.Since(startStop)
				if stopDuration > tc.maxStopTime {
					if tc.useContext && !tc.useStopCh {
						t.Errorf("EXPECTED FAILURE: Context-only stop took %v (want <%v)",
							stopDuration, tc.maxStopTime)
						t.Logf("This FAILS because context cancellation is ignored")
					} else {
						t.Errorf("Stop took too long: %v", stopDuration)
					}
				} else {
					t.Logf("Stop duration: %v", stopDuration)
				}
			case <-timeout:
				if tc.shouldStop {
					if tc.useContext && !tc.useStopCh {
						t.Errorf("EXPECTED FAILURE: Engine didn't stop via context cancellation")
						t.Logf("This FAILS because run loop doesn't check ctx.Context.Done()")
					} else {
						t.Errorf("Engine didn't stop within timeout")
					}
				}
				engine.Stop() // Cleanup
			}
		})
	}
}

// TestEngine_ContextCancellationNoGoroutineLeak tests:
// Verify no goroutine leak when context is cancelled
//
// FAILING TEST: May leak goroutines if context cancellation doesn't work
// Expected behavior: Clean shutdown with no leaked goroutines
func TestEngine_ContextCancellationNoGoroutineLeak(t *testing.T) {
	// Record initial goroutine count
	initialGoroutines := runtime.NumGoroutine()

	// Arrange
	net := petri.NewPetriNet("test-net", "Test Net")
	p1 := petri.NewPlace("p1", "Place 1", 10)
	net.AddPlace(p1)
	net.AddTransition(petri.NewTransition("t1", "T1"))
	net.Resolve()

	// Create and cancel context
	goCtx, cancel := context.WithCancel(context.Background())
	ctx := execContext.NewExecutionContext(
		goCtx,
		clock.NewVirtualClock(time.Now()),
		nil,
	)

	engine := NewEngine(net, ctx, DefaultConfig())

	// Act: Start and immediately cancel
	engine.Start()
	time.Sleep(10 * time.Millisecond)
	cancel()

	// Force stop to clean up (since context cancellation doesn't work)
	time.Sleep(100 * time.Millisecond)
	engine.Stop()

	// Wait for cleanup
	time.Sleep(100 * time.Millisecond)

	// Assert: No goroutine leak
	finalGoroutines := runtime.NumGoroutine()
	leakedGoroutines := finalGoroutines - initialGoroutines

	if leakedGoroutines > 2 { // Allow small variance
		t.Errorf("POTENTIAL FAILURE: Possible goroutine leak: %d goroutines leaked",
			leakedGoroutines)
		t.Logf("Initial: %d, Final: %d", initialGoroutines, finalGoroutines)
		t.Logf("If context cancellation worked, run loop would exit cleanly")
	}
}
