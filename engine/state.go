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

// WorkflowState represents the current state of a workflow execution.
// State Detection
//
// This enum allows the engine to distinguish between different workflow states:
//   - StateRunning: Active transitions can fire immediately
//   - StateIdle: Waiting for external events (timers, human approvals, etc.)
//   - StateBlocked: Deadlock - no transitions enabled and no pending triggers
//   - StateComplete: Token(s) in terminal place(s) - workflow finished successfully
type WorkflowState int

const (
	// StateRunning indicates that transitions can fire immediately.
	// At least one transition is enabled and ready to execute.
	StateRunning WorkflowState = iota

	// StateIdle indicates the workflow is waiting for external events.
	// No transitions are currently enabled, but there are pending triggers
	// (timers, manual approvals, external events) that could enable transitions.
	StateIdle

	// StateBlocked indicates a deadlock condition.
	// No transitions are enabled and no triggers are pending.
	// The workflow cannot make progress without external intervention.
	StateBlocked

	// StateComplete indicates the workflow has finished successfully.
	// At least one token has reached a terminal place.
	StateComplete
)

// String returns a string representation of the workflow state.
func (s WorkflowState) String() string {
	switch s {
	case StateRunning:
		return "Running"
	case StateIdle:
		return "Idle"
	case StateBlocked:
		return "Blocked"
	case StateComplete:
		return "Complete"
	default:
		return "Unknown"
	}
}

// State returns the current workflow state based on the Petri net structure
// and enabled transitions.
//
// Detection Logic:
//  1. If any terminal place has tokens → COMPLETE
//  2. If any transition is enabled → RUNNING
//  3. If any transition has pending trigger → IDLE
//  4. Otherwise → BLOCKED (deadlock)
//
// State Detection
//
// This method is thread-safe and can be called while the engine is running.
// It provides a snapshot of the workflow state at the time of the call.
func (e *Engine) State() WorkflowState {
	// Check for tokens in terminal places (COMPLETE)
	for _, place := range e.net.Places {
		if place.IsTerminal && place.TokenCount() > 0 {
			return StateComplete
		}
	}

	// Check for enabled transitions (RUNNING)
	enabled := e.findEnabledTransitions()
	if len(enabled) > 0 {
		return StateRunning
	}

	// Check for pending triggers (IDLE)
	e.waitingMu.Lock()
	hasPendingTriggers := len(e.waitingTransitions) > 0
	e.waitingMu.Unlock()

	if hasPendingTriggers {
		return StateIdle
	}

	// No enabled transitions and no pending triggers (BLOCKED)
	return StateBlocked
}
