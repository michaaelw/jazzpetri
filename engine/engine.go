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

// Package engine provides the workflow execution engine for Jazz3.
//
// The engine manages the execution of Petri net workflows, supporting both
// sequential and concurrent execution modes. It polls for enabled transitions
// and fires them according to the configured execution strategy.
//
// # Execution Modes
//
// Sequential Mode: Fires one transition at a time in a single-threaded manner.
// This is simple, deterministic, and useful for debugging.
//
// Concurrent Mode: Fires multiple enabled transitions concurrently using a
// goroutine pool. This provides better performance for workflows with parallel
// branches while limiting resource consumption.
//
// # Usage Example
//
//	// Create a Petri net workflow
//	net := petri.NewPetriNet("workflow", "My Workflow")
//	// ... add places, transitions, arcs ...
//	net.Resolve()
//
//	// Create execution context
//	ctx := context.NewExecutionContext(
//	    context.Background(),
//	    clock.NewRealTimeClock(),
//	    nil,
//	)
//
//	// Create and start engine
//	engine := NewEngine(net, ctx, DefaultConfig())
//	engine.Start()
//
//	// Wait for completion
//	engine.Wait()
//	engine.Stop()
package engine

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/petri"
	"github.com/jazzpetri/engine/state"
	"github.com/jazzpetri/engine/token"
)

// ExecutionMode defines how the engine executes transitions.
type ExecutionMode int

const (
	// ModeSequential executes one transition at a time (single-threaded).
	ModeSequential ExecutionMode = iota

	// ModeConcurrent executes multiple transitions concurrently using a goroutine pool.
	ModeConcurrent
)

// EngineState represents the current state of the workflow engine.
// Engine State Inspection
type EngineState int

const (
	// EngineStopped indicates the engine is not running.
	EngineStopped EngineState = iota

	// EngineRunning indicates the engine is actively executing transitions.
	EngineRunning

	// EnginePaused indicates the engine is paused and not firing transitions.
	// Pause/Resume Support
	EnginePaused
)

// String returns a string representation of the engine state.
func (s EngineState) String() string {
	switch s {
	case EngineStopped:
		return "Stopped"
	case EngineRunning:
		return "Running"
	case EnginePaused:
		return "Paused"
	default:
		return "Unknown"
	}
}

// ExecutionStatistics tracks workflow execution metrics.
// Workflow Execution Statistics
type ExecutionStatistics struct {
	// TransitionsFired is the total number of transitions fired
	TransitionsFired int64

	// TokensCreated is the total number of tokens created during execution
	TokensCreated int64

	// TotalSteps is the number of execution steps (poll cycles)
	TotalSteps int64

	// ExecutionTime is the total time the engine has been running
	ExecutionTime time.Duration

	// StartTime is when the engine was last started
	StartTime time.Time

	// LastTransitionTime is when the last transition fired
	LastTransitionTime time.Time

	// mu protects concurrent access to statistics
	mu sync.RWMutex
}

// Config defines the configuration for the workflow engine.
type Config struct {
	// Mode determines sequential or concurrent execution.
	Mode ExecutionMode

	// MaxConcurrentTransitions limits concurrent goroutines in concurrent mode.
	// Ignored in sequential mode.
	MaxConcurrentTransitions int

	// PollInterval is how often to check for enabled transitions.
	PollInterval time.Duration

	// StepMode enables manual step-by-step execution.
	// When true, transitions only fire via Step() calls.
	StepMode bool

	// WaitTimeout is the maximum time to wait for workflow completion in Wait().
	// Zero means wait indefinitely (no timeout).
	// Default: 0 (no timeout)
	WaitTimeout time.Duration
}

// DefaultConfig returns a sensible default configuration.
// Uses the defined constants for consistent default values across the codebase.
func DefaultConfig() Config {
	return Config{
		Mode:                     ModeConcurrent,
		MaxConcurrentTransitions: DefaultMaxConcurrentTransitions,
		PollInterval:             DefaultPollInterval,
		StepMode:                 false,
	}
}

// Engine manages workflow execution for a Petri net.
// It coordinates transition firing according to the configured execution mode.
//
// The engine is thread-safe and can be started/stopped multiple times.
//
// Pause/Resume Support
// Engine State Inspection
type Engine struct {
	// net is the Petri net being executed
	net *petri.PetriNet

	// ctx is the execution context with observability
	ctx *context.ExecutionContext

	// config holds the engine configuration
	config Config

	// Execution state
	state   EngineState // State tracking
	paused  bool        // Pause flag
	mu      sync.RWMutex
	stopCh  chan struct{}
	doneCh  chan struct{}
	pauseCh chan struct{} // Pause signal channel

	// Statistics tracking 
	stats ExecutionStatistics

	// Trigger support: track transitions waiting for triggers
	waitingTransitions map[string]bool
	waitingMu          sync.Mutex

	// Track currently executing transitions (for Wait() to check)
	executingCount int32 // Use atomic operations for thread-safe access
}

// NewEngine creates a new workflow engine for the given Petri net.
//
// Parameters:
//   - net: The Petri net to execute (must be resolved)
//   - ctx: Execution context with observability and error handling
//   - config: Engine configuration
//
// Configuration Validation:
// MaxConcurrentTransitions is validated and defaults to 10 if invalid (<= 0).
// This prevents panics when creating the semaphore channel in concurrent mode.
//
// The engine is created in a stopped state. Call Start() to begin execution.
func NewEngine(net *petri.PetriNet, ctx *context.ExecutionContext, config Config) *Engine {
	// Validate MaxConcurrentTransitions
	// Zero or negative values would cause panic when creating semaphore channel
	// Default to a safe minimum value
	if config.MaxConcurrentTransitions <= 0 {
		config.MaxConcurrentTransitions = 10 // Safe default
		ctx.Logger.Warn("Invalid MaxConcurrentTransitions, using default", map[string]interface{}{
			"net_id":    net.ID,
			"default":   10,
			"operation": "engine_init",
		})
	}

	return &Engine{
		net:                net,
		ctx:                ctx,
		config:             config,
		state:              EngineStopped, // Initialize state
		waitingTransitions: make(map[string]bool),
	}
}

// Start begins workflow execution in a background goroutine.
//
// Idempotency :
// Calling Start() multiple times is safe and has no effect if the engine is
// already running. This allows callers to safely call Start() without checking
// IsRunning() first, simplifying usage patterns.
//
// The engine will continuously poll for enabled transitions and fire them
// according to the configured execution mode until Stop() is called or the
// workflow completes (no enabled transitions).
//
// Returns:
//   - nil if engine starts successfully or is already running (idempotent)
func (e *Engine) Start() error {
	e.mu.Lock()
	if e.state == EngineRunning {
		e.mu.Unlock()
		// Already running - idempotent, not an error
		return nil
	}
	e.state = EngineRunning
	e.paused = false
	e.stopCh = make(chan struct{})
	e.doneCh = make(chan struct{})
	e.pauseCh = make(chan struct{})

	// Initialize statistics
	e.stats.StartTime = e.ctx.Clock.Now()
	e.mu.Unlock()

	// Record engine started event
	if e.ctx.EventLog != nil && e.ctx.EventBuilder != nil {
		event := e.ctx.EventBuilder.NewEvent(context.EventEngineStarted, e.net.ID)
		e.ctx.EventLog.Append(event)
	}

	go e.run()
	return nil
}

// Stop gracefully stops the engine.
//
// Idempotency :
// Calling Stop() multiple times is safe and has no effect if the engine is
// already stopped. This allows callers to safely call Stop() in cleanup code
// without checking IsRunning() first, simplifying error handling and shutdown logic.
//
// This method blocks until the execution loop has fully stopped (if it was running).
//
// Returns:
//   - nil if engine stops successfully or is already stopped (idempotent)
func (e *Engine) Stop() error {
	e.mu.Lock()
	if e.state == EngineStopped {
		e.mu.Unlock()
		// Already stopped - idempotent, not an error
		return nil
	}

	// Capture channels before unlocking
	stopCh := e.stopCh
	doneCh := e.doneCh
	e.mu.Unlock()

	// Clean up waiting transitions before stopping
	e.waitingMu.Lock()
	for transID := range e.waitingTransitions {
		// Find transition and cancel trigger
		for _, t := range e.net.Transitions {
			if t.ID == transID && t.Trigger != nil {
				t.Trigger.Cancel()
			}
		}
	}
	e.waitingTransitions = make(map[string]bool) // Clear
	e.waitingMu.Unlock()

	close(stopCh)

	// Wait for run loop to finish
	<-doneCh

	e.mu.Lock()
	e.state = EngineStopped
	e.paused = false

	// Update execution time
	if !e.stats.StartTime.IsZero() {
		e.stats.ExecutionTime += e.ctx.Clock.Now().Sub(e.stats.StartTime)
	}
	e.mu.Unlock()

	// Record engine stopped event
	if e.ctx.EventLog != nil && e.ctx.EventBuilder != nil {
		event := e.ctx.EventBuilder.NewEvent(context.EventEngineStopped, e.net.ID)
		e.ctx.EventLog.Append(event)
	}

	return nil
}

// Net returns the Petri net being executed by this engine.
func (e *Engine) Net() *petri.PetriNet {
	return e.net
}

// ExecutionContext returns the execution context used by this engine.
func (e *Engine) ExecutionContext() *context.ExecutionContext {
	return e.ctx
}

// IsRunning returns true if the engine is currently running (not stopped or paused).
func (e *Engine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state == EngineRunning
}

// GetState returns the current state of the engine.
// Engine State Inspection
//
// Returns:
//   - EngineStopped: Engine is not running
//   - EngineRunning: Engine is actively executing transitions
//   - EnginePaused: Engine is paused and not firing transitions
func (e *Engine) GetState() EngineState {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state
}

// IsPaused returns true if the engine is currently paused.
// Pause/Resume Support
func (e *Engine) IsPaused() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.paused
}

// Pause pauses the engine execution.
// Transitions will not fire while the engine is paused.
// Pause/Resume Support
//
// Returns an error if the engine is not running.
func (e *Engine) Pause() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.state != EngineRunning {
		return fmt.Errorf("cannot pause: engine is not running (state: %s)", e.state)
	}

	if e.paused {
		// Already paused - idempotent
		return nil
	}

	e.paused = true
	e.state = EnginePaused

	e.ctx.Logger.Info("engine paused", map[string]interface{}{
		"net_id":    e.net.ID,
		"state":     e.state.String(),
		"operation": "pause",
	})
	return nil
}

// Resume resumes the engine execution after a pause.
// Pause/Resume Support
//
// Returns an error if the engine is not paused.
func (e *Engine) Resume() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.state != EnginePaused {
		return fmt.Errorf("cannot resume: engine is not paused (state: %s)", e.state)
	}

	e.paused = false
	e.state = EngineRunning

	// Signal resume to the run loop
	close(e.pauseCh)
	e.pauseCh = make(chan struct{})

	e.ctx.Logger.Info("engine resumed", map[string]interface{}{
		"net_id":    e.net.ID,
		"state":     e.state.String(),
		"operation": "resume",
	})
	return nil
}

// GetStatistics returns the current execution statistics.
// Workflow Execution Statistics
//
// Returns a snapshot of the statistics to prevent concurrent modification.
func (e *Engine) GetStatistics() *ExecutionStatistics {
	e.mu.RLock()
	defer e.mu.RUnlock()

	e.stats.mu.RLock()
	defer e.stats.mu.RUnlock()

	// Return a copy (copy fields individually to avoid copying the mutex)
	statsCopy := ExecutionStatistics{
		TransitionsFired:   e.stats.TransitionsFired,
		TokensCreated:      e.stats.TokensCreated,
		TotalSteps:         e.stats.TotalSteps,
		ExecutionTime:      e.stats.ExecutionTime,
		StartTime:          e.stats.StartTime,
		LastTransitionTime: e.stats.LastTransitionTime,
	}

	// Update execution time if currently running
	if e.state == EngineRunning || e.state == EnginePaused {
		if !e.stats.StartTime.IsZero() {
			statsCopy.ExecutionTime = e.stats.ExecutionTime + e.ctx.Clock.Now().Sub(e.stats.StartTime)
		}
	}

	return &statsCopy
}

// Step executes one enabled transition and returns.
// This is used for step-by-step debugging and testing.
//
// Returns an error if:
//   - Engine is running in continuous mode (not step mode)
//   - No transitions are enabled
//   - Transition firing fails
func (e *Engine) Step() error {
	e.mu.RLock()
	if e.state == EngineRunning && !e.config.StepMode {
		e.mu.RUnlock()
		return fmt.Errorf("engine running in continuous mode")
	}
	e.mu.RUnlock()

	// Find and fire one enabled transition
	enabled := e.findEnabledTransitions()
	if len(enabled) == 0 {
		return fmt.Errorf("no enabled transitions")
	}

	// Fire first enabled transition
	t := enabled[0]
	if err := t.Fire(e.ctx); err != nil {
		return fmt.Errorf("step failed: %w", err)
	}

	e.ctx.Metrics.Inc("engine_steps_total")
	return nil
}

// TryStep attempts to execute one enabled transition.
// This method provides better semantics than Step() for distinguishing
// between "no work to do" (idle) and "firing failed" (error).
//
// Improved Step Semantics
// TryStep() returns (fired bool, err error) to distinguish:
//   - (true, nil): A transition was fired successfully
//   - (false, nil): No transitions enabled (workflow idle/complete)
//   - (false, err): Firing failed (actual error)
//
// Returns:
//   - fired: true if a transition was fired, false if no transitions were enabled
//   - err: error if firing failed or engine is in invalid state
//
// This method respects the same constraints as Step():
//   - Cannot be called when engine is running in continuous mode
func (e *Engine) TryStep() (bool, error) {
	e.mu.RLock()
	if e.state == EngineRunning && !e.config.StepMode {
		e.mu.RUnlock()
		return false, fmt.Errorf("engine running in continuous mode")
	}
	e.mu.RUnlock()

	// Find enabled transitions
	enabled := e.findEnabledTransitions()
	if len(enabled) == 0 {
		// No work to do - not an error condition
		return false, nil
	}

	// Fire first enabled transition
	t := enabled[0]
	if err := t.Fire(e.ctx); err != nil {
		// Firing failed - return error
		return false, fmt.Errorf("step failed: %w", err)
	}

	e.ctx.Metrics.Inc("engine_steps_total")
	return true, nil
}

// Wait blocks until the workflow completes (no enabled transitions remain).
// Uses the timeout configured in Config.WaitTimeout (0 = wait indefinitely).
//
// This is useful for synchronous workflow execution where you want to wait
// for the entire workflow to finish before proceeding.
func (e *Engine) Wait() error {
	return e.WaitWithTimeout(e.config.WaitTimeout)
}

// WaitWithTimeout blocks until the workflow completes or the timeout expires.
// A timeout of 0 means wait indefinitely (no timeout).
//
// This provides explicit control over wait timeout independent of engine configuration.
//
// A workflow is considered complete when:
//  1. No transitions are currently enabled (no more work can start)
//  2. No transitions are currently executing (all work has finished)
//
// This ensures we don't return prematurely while transitions are still running.
//
// Returns an error if the wait times out.
func (e *Engine) WaitWithTimeout(timeout time.Duration) error {
	ticker := time.NewTicker(e.config.PollInterval)
	defer ticker.Stop()

	var timeoutCh <-chan time.Time
	if timeout > 0 {
		timeoutCh = time.After(timeout)
	}

	for {
		enabled := e.findEnabledTransitions()
		executing := atomic.LoadInt32(&e.executingCount)

		// Workflow is complete when no transitions are enabled AND none are executing
		if len(enabled) == 0 && executing == 0 {
			return nil
		}

		select {
		case <-ticker.C:
			continue
		case <-timeoutCh:
			// When timeout is 0, timeoutCh is nil and this case will never match
			return fmt.Errorf("timeout waiting for completion")
		}
	}
}

// run is the main execution loop that runs in a background goroutine.
// It continuously polls for enabled transitions and fires them according
// to the configured execution mode.
//
// Respects pause state
// Updates execution statistics
func (e *Engine) run() {
	defer close(e.doneCh)
	defer func() {
		// Mark engine as stopped when exiting
		e.mu.Lock()
		e.state = EngineStopped
		e.mu.Unlock()
	}()

	span := e.ctx.Tracer.StartSpan("engine.run")
	defer span.End()

	ticker := time.NewTicker(e.config.PollInterval)
	defer ticker.Stop()

	for {
		// Check pause state
		e.mu.RLock()
		isPaused := e.paused
		pauseCh := e.pauseCh
		e.mu.RUnlock()

		if isPaused {
			// Wait for resume signal
			select {
			case <-e.stopCh:
				e.ctx.Logger.Info("engine stopped while paused", map[string]interface{}{
					"net_id":    e.net.ID,
					"state":     "paused",
					"operation": "stop",
				})
				return
			case <-e.ctx.Context.Done():
				e.ctx.Logger.Info("engine cancelled while paused", map[string]interface{}{
					"net_id":    e.net.ID,
					"state":     "paused",
					"error":     e.ctx.Context.Err().Error(),
					"operation": "cancel",
				})
				return
			case <-pauseCh:
				// Resume signal received, continue execution
				continue
			}
		}

		select {
		case <-e.stopCh:
			e.ctx.Logger.Info("engine stopped", map[string]interface{}{
				"net_id":    e.net.ID,
				"state":     "stopping",
				"operation": "stop",
			})
			return

		// Respect context cancellation
		case <-e.ctx.Context.Done():
			e.ctx.Logger.Info("engine stopped due to context cancellation", map[string]interface{}{
				"net_id":    e.net.ID,
				"error":     e.ctx.Context.Err().Error(),
				"operation": "cancel",
			})
			return

		case <-ticker.C:
			// In step mode, don't auto-fire — only fire via Step()/Fire() calls
			if e.config.StepMode {
				continue
			}

			// Increment step counter
			e.stats.mu.Lock()
			e.stats.TotalSteps++
			e.stats.mu.Unlock()

			// Find enabled transitions
			enabled := e.findEnabledTransitions()

			if len(enabled) == 0 {
				// No enabled transitions - workflow complete or blocked
				e.ctx.Metrics.Inc("engine_idle_total")
				continue
			}

			// Fire enabled transitions based on mode
			switch e.config.Mode {
			case ModeSequential:
				e.fireSequential(enabled)
			case ModeConcurrent:
				e.fireConcurrent(enabled)
			}
		}
	}
}

// findEnabledTransitions returns all transitions that are currently enabled.
// A transition is enabled if it has sufficient input tokens and its guard
// passes (if a guard is set).
//
// Guards now receive ExecutionContext to access capabilities (Clock, Tracer, Metrics, Logger).
// Guard errors are logged but transitions with errors are treated as not enabled.
//
// Trigger Integration:
// After checking structural enablement (tokens + guard), this method also checks
// the trigger (WHEN to fire). If the trigger is not ready, a goroutine is spawned
// to await the trigger, and the transition is excluded from the returned list.
func (e *Engine) findEnabledTransitions() []*petri.Transition {
	var enabled []*petri.Transition

	for _, t := range e.net.Transitions {
		// 1. Check structural + guard enablement (IF it CAN fire)
		isEnabled, err := t.IsEnabled(e.ctx)
		if err != nil {
			// Guard evaluation error - log and treat as not enabled
			e.ctx.Metrics.Inc("guard_evaluation_error_total")
			if e.ctx.Logger != nil {
				e.ctx.Logger.Error("guard evaluation error", map[string]interface{}{
					"transition_id": t.ID,
					"error":         err.Error(),
				})
			}
			continue
		}
		if !isEnabled {
			continue
		}

		// 2. Check trigger (WHEN to fire)
		if t.Trigger != nil {
			shouldFire, err := t.Trigger.ShouldFire(e.ctx)
			if err != nil {
				e.ctx.Metrics.Inc("trigger_evaluation_error_total")
				if e.ctx.Logger != nil {
					e.ctx.Logger.Error("trigger evaluation error", map[string]interface{}{
						"transition_id": t.ID,
						"error":         err.Error(),
					})
				}
				continue
			}
			if !shouldFire {
				// Not ready to fire yet - spawn waiter goroutine
				e.spawnTriggerWaiter(t)
				continue
			}
		}
		// else: nil trigger = AutomaticTrigger (fire immediately)

		enabled = append(enabled, t)
	}

	return enabled
}

// spawnTriggerWaiter spawns a goroutine to wait for a trigger to become ready.
// Only spawns one waiter per transition (tracked in waitingTransitions map).
// When trigger is ready, re-checks IsEnabled() and fires if still enabled.
//
// This method implements the asynchronous trigger-based firing pattern:
//  1. Check if already waiting for this transition (idempotent)
//  2. Mark transition as waiting
//  3. Spawn goroutine to block on AwaitFiring()
//  4. When ready, re-check IsEnabled() (tokens may have been consumed)
//  5. Fire the transition if still enabled
//
// The waiter goroutine respects context cancellation and engine shutdown.
func (e *Engine) spawnTriggerWaiter(t *petri.Transition) {
	// Check if already waiting for this transition
	e.waitingMu.Lock()
	if e.waitingTransitions[t.ID] {
		e.waitingMu.Unlock()
		return // Already waiting
	}
	e.waitingTransitions[t.ID] = true
	e.waitingMu.Unlock()

	e.ctx.Metrics.Inc("trigger_await_started_total")

	go func() {
		defer func() {
			e.waitingMu.Lock()
			delete(e.waitingTransitions, t.ID)
			e.waitingMu.Unlock()
		}()

		// Block until trigger ready
		if err := t.Trigger.AwaitFiring(e.ctx); err != nil {
			if e.ctx.Logger != nil {
				e.ctx.Logger.Error("trigger await failed", map[string]interface{}{
					"transition_id": t.ID,
					"error":         err.Error(),
				})
			}
			e.ctx.Metrics.Inc("trigger_await_failed_total")
			return
		}

		e.ctx.Metrics.Inc("trigger_await_completed_total")

		// Trigger is ready - re-check IsEnabled (tokens may have been consumed)
		isEnabled, err := t.IsEnabled(e.ctx)
		if err != nil {
			if e.ctx.Logger != nil {
				e.ctx.Logger.Error("IsEnabled check failed after trigger ready", map[string]interface{}{
					"transition_id": t.ID,
					"error":         err.Error(),
				})
			}
			return
		}
		if !isEnabled {
			// No longer enabled (tokens consumed by another transition)
			e.ctx.Metrics.Inc("trigger_fired_but_not_enabled_total")
			return
		}

		// Fire the transition
		if err := t.FireUnchecked(e.ctx); err != nil {
			e.ctx.ErrorHandler.Handle(e.ctx, err)
			e.ctx.Metrics.Inc("engine_fire_errors_total")
		} else {
			e.ctx.Metrics.Inc("engine_transitions_fired_total")
			e.stats.mu.Lock()
			e.stats.TransitionsFired++
			e.stats.LastTransitionTime = e.ctx.Clock.Now()
			e.stats.mu.Unlock()
		}
	}()
}

// fireSequential fires transitions one at a time in sequential order.
// Only one transition is fired per poll cycle, even if multiple are enabled.
//
// Uses FireUnchecked() to avoid redundant IsEnabled() check
// since findEnabledTransitions() has already verified enablement.
//
// Updates execution statistics
func (e *Engine) fireSequential(transitions []*petri.Transition) {
	// Fire one transition at a time
	for _, t := range transitions {
		// Increment executing count before firing
		atomic.AddInt32(&e.executingCount, 1)

		// Use FireUnchecked() since we already checked IsEnabled() in findEnabledTransitions()
		// This avoids wasteful double guard evaluation 
		err := t.FireUnchecked(e.ctx)

		// Decrement executing count after firing completes
		atomic.AddInt32(&e.executingCount, -1)

		if err != nil {
			e.ctx.ErrorHandler.Handle(e.ctx, err)
			e.ctx.Metrics.Inc("engine_fire_errors_total")
		} else {
			e.ctx.Metrics.Inc("engine_transitions_fired_total")

			// Update statistics
			e.stats.mu.Lock()
			e.stats.TransitionsFired++
			e.stats.LastTransitionTime = e.ctx.Clock.Now()
			e.stats.mu.Unlock()
		}

		// Only fire one per cycle in sequential mode
		break
	}
}

// fireConcurrent fires all enabled transitions concurrently using a worker pool.
// The pool size is limited by MaxConcurrentTransitions to prevent unbounded
// goroutine creation.
//
// Uses FireUnchecked() to avoid redundant IsEnabled() check
// since findEnabledTransitions() has already verified enablement.
//
// Updates execution statistics
func (e *Engine) fireConcurrent(transitions []*petri.Transition) {
	// Fire transitions concurrently with worker pool
	sem := make(chan struct{}, e.config.MaxConcurrentTransitions)
	var wg sync.WaitGroup

	for _, t := range transitions {
		wg.Add(1)
		sem <- struct{}{} // Acquire slot

		go func(transition *petri.Transition) {
			defer wg.Done()
			defer func() { <-sem }() // Release slot

			// Increment executing count before firing
			atomic.AddInt32(&e.executingCount, 1)

			// Use FireUnchecked() since we already checked IsEnabled() in findEnabledTransitions()
			// This avoids wasteful double guard evaluation 
			err := transition.FireUnchecked(e.ctx)

			// Decrement executing count after firing completes
			atomic.AddInt32(&e.executingCount, -1)

			if err != nil {
				e.ctx.ErrorHandler.Handle(e.ctx, err)
				e.ctx.Metrics.Inc("engine_fire_errors_total")
			} else {
				e.ctx.Metrics.Inc("engine_transitions_fired_total")

				// Update statistics (thread-safe)
				e.stats.mu.Lock()
				e.stats.TransitionsFired++
				e.stats.LastTransitionTime = e.ctx.Clock.Now()
				e.stats.mu.Unlock()
			}
		}(t)
	}

	wg.Wait()
}

// GetMarking captures the current workflow state as a marking (snapshot).
// This is a non-destructive operation that does not affect workflow execution.
//
// Returns an error if the Petri net is not resolved.
func (e *Engine) GetMarking() (*state.Marking, error) {
	return state.NewMarking(e.net, e.ctx.Clock)
}

// RestoreFromMarking restores the engine's workflow state from a marking.
// This replaces the current state of all places with the state from the marking.
//
// The engine must be stopped before calling this method. Attempting to restore
// while the engine is running will return an error.
//
// This operation clears all existing tokens from all places before restoring
// the tokens from the marking. If restoration fails partway through, the net
// may be in a partially cleared state.
//
// Parameters:
//   - marking: The marking to restore from
//
// Returns an error if:
//   - the engine is running
//   - marking restoration fails (see state.RestoreMarkingToNet for details)
func (e *Engine) RestoreFromMarking(marking *state.Marking) error {
	e.mu.Lock()
	if e.state != EngineStopped {
		e.mu.Unlock()
		return fmt.Errorf("cannot restore while engine is running (state: %s)", e.state)
	}
	e.mu.Unlock()

	if err := state.RestoreMarkingToNet(marking, e.net); err != nil {
		return fmt.Errorf("restoration failed: %w", err)
	}

	e.ctx.Logger.Info("engine state restored", map[string]interface{}{
		"net_id":      e.net.ID,
		"timestamp":   marking.Timestamp,
		"places":      len(marking.Places),
		"operation":   "restore",
		"engine_state": e.state.String(),
	})

	return nil
}

// RestoreFromFile restores workflow state from a JSON file.
// This is a convenience method that loads a marking from a file and then
// calls RestoreFromMarking.
//
// The engine must be stopped before calling this method.
//
// Parameters:
//   - filepath: Path to the JSON file containing the saved marking
//   - registry: Token type registry for deserializing token data
//
// Returns an error if:
//   - the file cannot be loaded
//   - deserialization fails
//   - restoration fails
func (e *Engine) RestoreFromFile(filepath string, registry token.TypeRegistry) error {
	marking, err := state.LoadMarkingFromFile(filepath, registry)
	if err != nil {
		return fmt.Errorf("failed to load marking: %w", err)
	}

	return e.RestoreFromMarking(marking)
}

// HealthStatus represents the current health status of the engine.
// Health Check
type HealthStatus struct {
	// State indicates the overall health: "healthy", "unhealthy", "stuck", "stopped"
	State string `json:"state"`

	// EngineState indicates the engine's execution state: "running", "paused", "stopped"
	EngineState string `json:"engine_state"`

	// Uptime is how long the engine has been running
	Uptime time.Duration `json:"uptime_ms"`

	// LastActivity is when the last transition was fired
	LastActivity time.Time `json:"last_activity"`

	// TransitionsFired is the total number of transitions fired
	TransitionsFired int64 `json:"transitions_fired"`

	// EnabledTransitions is the number of currently enabled transitions
	EnabledTransitions int `json:"enabled_transitions"`

	// Metrics contains key operational metrics
	Metrics map[string]interface{} `json:"metrics,omitempty"`

	// Config contains configuration information
	Config map[string]interface{} `json:"config,omitempty"`
}

// HealthCheck returns the current health status of the engine.
// Health Check
func (e *Engine) HealthCheck() *HealthStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Convert EngineState to string
	var engineStateStr string
	switch e.state {
	case EngineRunning:
		engineStateStr = "running"
	case EnginePaused:
		engineStateStr = "paused"
	case EngineStopped:
		engineStateStr = "stopped"
	default:
		engineStateStr = "unknown"
	}

	status := &HealthStatus{
		EngineState:        engineStateStr,
		TransitionsFired:   e.stats.TransitionsFired,
		EnabledTransitions: 0,
		Metrics:            make(map[string]interface{}),
		Config:             make(map[string]interface{}),
	}

	// Calculate uptime
	if !e.stats.StartTime.IsZero() {
		if e.state == EngineRunning {
			status.Uptime = time.Since(e.stats.StartTime)
		} else {
			// Use ExecutionTime which already tracks total execution time
			status.Uptime = e.stats.ExecutionTime
		}
	}

	// Set last activity
	status.LastActivity = e.stats.LastTransitionTime

	// Count enabled transitions
	if e.net != nil {
		enabled := e.findEnabledTransitions()
		status.EnabledTransitions = len(enabled)
	}

	// Determine overall health state
	switch e.state {
	case EngineRunning:
		status.State = "healthy"
	case EnginePaused:
		status.State = "healthy" // Paused is still healthy
	case EngineStopped:
		status.State = "stopped"
	default:
		status.State = "unknown"
	}

	// Populate metrics
	status.Metrics["transitions_fired_total"] = e.stats.TransitionsFired
	status.Metrics["enabled_transitions"] = status.EnabledTransitions
	status.Metrics["total_steps"] = e.stats.TotalSteps
	status.Metrics["execution_time_ms"] = status.Uptime.Milliseconds()
	status.Metrics["tokens_created"] = e.stats.TokensCreated

	// Populate config - convert ExecutionMode to string
	var modeStr string
	switch e.config.Mode {
	case ModeSequential:
		modeStr = "sequential"
	case ModeConcurrent:
		modeStr = "concurrent"
	default:
		modeStr = "unknown"
	}

	status.Config["mode"] = modeStr
	status.Config["max_concurrent_transitions"] = e.config.MaxConcurrentTransitions
	status.Config["poll_interval_ms"] = e.config.PollInterval.Milliseconds()
	status.Config["step_mode"] = e.config.StepMode

	return status
}

// DryRunResult contains the results of a dry-run validation.
// Dry-Run Mode
type DryRunResult struct {
	Valid        bool
	Issues       []DryRunIssue
	Warnings     []DryRunIssue
	Analysis     map[string]interface{}
	Reachability map[string]bool
}

// DryRunIssue represents a problem found during dry-run validation.
type DryRunIssue struct {
	Type        string
	Severity    string
	Component   string
	ComponentID string
	Message     string
}

// HasIssues returns true if there are any validation issues.
func (r *DryRunResult) HasIssues() bool {
	return len(r.Issues) > 0
}

// DryRun is implemented in dryrun.go
