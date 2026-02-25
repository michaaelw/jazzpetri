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

package petri

import (
	"github.com/jazzpetri/engine/task"
)

// NewAgentTransition creates a transition for AI agent execution.
// The agent's task performs autonomous processing.
//
// Example:
//
//	agentTask := &task.InlineTask{Name: "agent", Fn: myAgentHandler}
//	t := NewAgentTransition("analyze", "Analyze Document", agentTask)
func NewAgentTransition(id, name string, agentTask task.Task) *Transition {
	return NewTransition(id, name).
		WithType(AgentTransition).
		WithTask(agentTask)
}

// NewToolTransition creates a transition for deterministic tool execution.
// The tool task performs a fixed computation or external call without AI reasoning.
//
// Example:
//
//	toolTask := &task.InlineTask{Name: "validate", Fn: myValidator}
//	t := NewToolTransition("validate", "Validate Input", toolTask)
func NewToolTransition(id, name string, toolTask task.Task) *Transition {
	return NewTransition(id, name).
		WithType(ToolTransition).
		WithTask(toolTask)
}

// NewHumanTransition creates a transition requiring human approval.
// Uses a ManualTrigger for the approval flow.
//
// Example:
//
//	timeout := 48 * time.Hour
//	trigger := &ManualTrigger{
//	    ApprovalID: "expense-123",
//	    Approvers:  []string{"alice@example.com"},
//	    Timeout:    &timeout,
//	}
//	t := NewHumanTransition("approve", "Approve Expense", trigger)
func NewHumanTransition(id, name string, trigger *ManualTrigger) *Transition {
	return NewTransition(id, name).
		WithType(HumanTransition).
		WithTrigger(trigger)
}

// NewGateTransition creates a pure control-flow transition with guard-only logic.
// Gates route tokens based on conditions without executing tasks.
//
// Example:
//
//	t := NewGateTransition("high-priority", "High Priority Route",
//	    func(ctx *context.ExecutionContext, tokens []*token.Token) (bool, error) {
//	        order := tokens[0].Data.(*Order)
//	        return order.Priority > 8, nil
//	    })
func NewGateTransition(id, name string, guard GuardFunc) *Transition {
	return NewTransition(id, name).
		WithType(GateTransition).
		WithGuard(guard)
}
