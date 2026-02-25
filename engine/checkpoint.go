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
	"fmt"

	"github.com/jazzpetri/engine/state"
	"github.com/jazzpetri/engine/token"
)

// Checkpoint captures the current workflow state to a file.
// This creates a snapshot of the current marking (token distribution across places)
// and saves it to the specified file in JSON format.
//
// The checkpoint can later be restored using RestoreFromFile to resume
// workflow execution from the saved state.
//
// This is useful for:
//   - Periodic state backups
//   - Testing and debugging (save known good states)
//   - Crash recovery (resume from last checkpoint)
//
// Parameters:
//   - filepath: Path where the checkpoint file should be created
//   - registry: Token type registry for serializing token data
//
// Returns an error if:
//   - marking capture fails
//   - file writing fails
func (e *Engine) Checkpoint(filepath string, registry token.TypeRegistry) error {
	// Get current marking
	marking, err := e.GetMarking()
	if err != nil {
		return fmt.Errorf("failed to capture marking: %w", err)
	}

	// Save to file
	if err := state.SaveMarkingToFile(marking, filepath, registry); err != nil {
		return fmt.Errorf("failed to save checkpoint: %w", err)
	}

	e.ctx.Logger.Info("checkpoint created", map[string]interface{}{
		"filepath": filepath,
	})

	return nil
}

// CheckpointWithEvents saves both marking and event log to a file.
// This creates a complete checkpoint that includes:
//   - Current marking (token distribution)
//   - Event log (history of all events)
//
// This provides more complete state for debugging and auditing purposes.
// The event log can be used to understand how the workflow reached its
// current state.
//
// Note: This requires the engine to have an event log configured via
// ExecutionContext.WithEventLog(). If no event log is configured,
// this method will behave the same as Checkpoint().
//
// Parameters:
//   - filepath: Path where the checkpoint file should be created
//   - registry: Token type registry for serializing token data
//
// Returns an error if:
//   - marking capture fails
//   - event log retrieval fails
//   - file writing fails
func (e *Engine) CheckpointWithEvents(filepath string, registry token.TypeRegistry) error {
	marking, err := e.GetMarking()
	if err != nil {
		return fmt.Errorf("failed to capture marking: %w", err)
	}

	// If no event log configured, just save marking
	if e.ctx.EventLog == nil {
		return state.SaveMarkingToFile(marking, filepath, registry)
	}

	// Type assert to MemoryEventLog to access GetAll
	eventLog, ok := e.ctx.EventLog.(*state.MemoryEventLog)
	if !ok {
		// Fallback to saving just marking if event log is not MemoryEventLog
		return state.SaveMarkingToFile(marking, filepath, registry)
	}

	return state.SaveWorkflowState(marking, eventLog, filepath, registry)
}
