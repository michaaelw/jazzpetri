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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/task"
	"github.com/jazzpetri/engine/token"
)

// --- TransitionType.String() tests ---

func TestTransitionType_String_Auto(t *testing.T) {
	if got := AutoTransition.String(); got != "auto" {
		t.Errorf("AutoTransition.String() = %q, want %q", got, "auto")
	}
}

func TestTransitionType_String_Agent(t *testing.T) {
	if got := AgentTransition.String(); got != "agent" {
		t.Errorf("AgentTransition.String() = %q, want %q", got, "agent")
	}
}

func TestTransitionType_String_Tool(t *testing.T) {
	if got := ToolTransition.String(); got != "tool" {
		t.Errorf("ToolTransition.String() = %q, want %q", got, "tool")
	}
}

func TestTransitionType_String_Human(t *testing.T) {
	if got := HumanTransition.String(); got != "human" {
		t.Errorf("HumanTransition.String() = %q, want %q", got, "human")
	}
}

func TestTransitionType_String_Gate(t *testing.T) {
	if got := GateTransition.String(); got != "gate" {
		t.Errorf("GateTransition.String() = %q, want %q", got, "gate")
	}
}

func TestTransitionType_String_Unknown(t *testing.T) {
	unknown := TransitionType(99)
	if got := unknown.String(); got != "unknown" {
		t.Errorf("unknown TransitionType.String() = %q, want %q", got, "unknown")
	}
}

func TestTransitionType_String_AllValues(t *testing.T) {
	tests := []struct {
		tt   TransitionType
		want string
	}{
		{AutoTransition, "auto"},
		{AgentTransition, "agent"},
		{ToolTransition, "tool"},
		{HumanTransition, "human"},
		{GateTransition, "gate"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.tt.String(); got != tc.want {
				t.Errorf("TransitionType(%d).String() = %q, want %q", int(tc.tt), got, tc.want)
			}
		})
	}
}

// --- WithType method tests ---

func TestTransition_WithType_ReturnsTransition(t *testing.T) {
	trans := NewTransition("t1", "Test")
	result := trans.WithType(AgentTransition)
	if result != trans {
		t.Error("WithType should return the same transition for chaining")
	}
}

func TestTransition_WithType_SetsType(t *testing.T) {
	trans := NewTransition("t1", "Test")
	trans.WithType(ToolTransition)
	if trans.Type != ToolTransition {
		t.Errorf("WithType should set Type to ToolTransition, got %v", trans.Type)
	}
}

func TestTransition_WithType_DefaultIsAuto(t *testing.T) {
	trans := NewTransition("t1", "Test")
	if trans.Type != AutoTransition {
		t.Errorf("Default Type should be AutoTransition (0), got %v", trans.Type)
	}
}

func TestTransition_WithType_Chaining(t *testing.T) {
	guard := func(ctx *context.ExecutionContext, tokens []*token.Token) (bool, error) {
		return true, nil
	}
	trans := NewTransition("t1", "Test").
		WithType(GateTransition).
		WithGuard(guard)

	if trans.Type != GateTransition {
		t.Errorf("Expected GateTransition after chaining, got %v", trans.Type)
	}
	if trans.Guard == nil {
		t.Error("Expected guard to be set after chaining")
	}
}

func TestTransition_WithType_OverrideType(t *testing.T) {
	trans := NewTransition("t1", "Test").
		WithType(AgentTransition).
		WithType(ToolTransition)

	if trans.Type != ToolTransition {
		t.Errorf("Expected ToolTransition after override, got %v", trans.Type)
	}
}

// --- String() output includes type when non-auto ---

func TestTransition_String_AutoType_NotIncluded(t *testing.T) {
	trans := NewTransition("t1", "My Transition")
	str := trans.String()

	// AutoTransition should NOT appear in the string
	if strings.Contains(str, "type=") {
		t.Errorf("AutoTransition should not appear as type= in String(), got: %s", str)
	}
	if strings.Contains(str, "auto") {
		t.Errorf("AutoTransition should not appear as 'auto' in String(), got: %s", str)
	}
}

func TestTransition_String_AgentType_Included(t *testing.T) {
	trans := NewTransition("t1", "My Agent").WithType(AgentTransition)
	str := trans.String()

	if !strings.Contains(str, "type=agent") {
		t.Errorf("AgentTransition should appear as 'type=agent' in String(), got: %s", str)
	}
}

func TestTransition_String_ToolType_Included(t *testing.T) {
	trans := NewTransition("t1", "My Tool").WithType(ToolTransition)
	str := trans.String()

	if !strings.Contains(str, "type=tool") {
		t.Errorf("ToolTransition should appear as 'type=tool' in String(), got: %s", str)
	}
}

func TestTransition_String_HumanType_Included(t *testing.T) {
	trans := NewTransition("t1", "Approval").WithType(HumanTransition)
	str := trans.String()

	if !strings.Contains(str, "type=human") {
		t.Errorf("HumanTransition should appear as 'type=human' in String(), got: %s", str)
	}
}

func TestTransition_String_GateType_Included(t *testing.T) {
	trans := NewTransition("t1", "Route").WithType(GateTransition)
	str := trans.String()

	if !strings.Contains(str, "type=gate") {
		t.Errorf("GateTransition should appear as 'type=gate' in String(), got: %s", str)
	}
}

func TestTransition_String_TypeAndGuard_Combined(t *testing.T) {
	guard := func(ctx *context.ExecutionContext, tokens []*token.Token) (bool, error) {
		return true, nil
	}
	trans := NewTransition("t1", "Gate Route").
		WithType(GateTransition).
		WithGuard(guard)
	str := trans.String()

	if !strings.Contains(str, "type=gate") {
		t.Errorf("String() should include 'type=gate', got: %s", str)
	}
	if !strings.Contains(str, "guarded") {
		t.Errorf("String() should include 'guarded', got: %s", str)
	}
}

// --- Factory function tests ---

func TestNewAgentTransition_SetsType(t *testing.T) {
	agentTask := &task.InlineTask{
		Name: "test-agent",
		Fn:   func(ctx *context.ExecutionContext) error { return nil },
	}
	trans := NewAgentTransition("a1", "My Agent", agentTask)

	if trans.ID != "a1" {
		t.Errorf("Expected ID 'a1', got %q", trans.ID)
	}
	if trans.Name != "My Agent" {
		t.Errorf("Expected Name 'My Agent', got %q", trans.Name)
	}
	if trans.Type != AgentTransition {
		t.Errorf("Expected AgentTransition type, got %v", trans.Type)
	}
	if trans.Task == nil {
		t.Error("Expected Task to be set")
	}
}

func TestNewAgentTransition_NoGuardByDefault(t *testing.T) {
	agentTask := &task.InlineTask{
		Name: "test-agent",
		Fn:   func(ctx *context.ExecutionContext) error { return nil },
	}
	trans := NewAgentTransition("a1", "My Agent", agentTask)

	if trans.Guard != nil {
		t.Error("NewAgentTransition should not set a Guard by default")
	}
}

func TestNewToolTransition_SetsType(t *testing.T) {
	toolTask := &task.InlineTask{
		Name: "test-tool",
		Fn:   func(ctx *context.ExecutionContext) error { return nil },
	}
	trans := NewToolTransition("tool1", "My Tool", toolTask)

	if trans.ID != "tool1" {
		t.Errorf("Expected ID 'tool1', got %q", trans.ID)
	}
	if trans.Name != "My Tool" {
		t.Errorf("Expected Name 'My Tool', got %q", trans.Name)
	}
	if trans.Type != ToolTransition {
		t.Errorf("Expected ToolTransition type, got %v", trans.Type)
	}
	if trans.Task == nil {
		t.Error("Expected Task to be set")
	}
}

func TestNewToolTransition_NoGuardByDefault(t *testing.T) {
	toolTask := &task.InlineTask{
		Name: "test-tool",
		Fn:   func(ctx *context.ExecutionContext) error { return nil },
	}
	trans := NewToolTransition("tool1", "My Tool", toolTask)

	if trans.Guard != nil {
		t.Error("NewToolTransition should not set a Guard by default")
	}
}

func TestNewHumanTransition_SetsType(t *testing.T) {
	timeout := 48 * time.Hour
	trigger := &ManualTrigger{
		ApprovalID: "approval-001",
		Approvers:  []string{"alice@example.com"},
		Timeout:    &timeout,
	}
	trans := NewHumanTransition("approve", "Approve Request", trigger)

	if trans.ID != "approve" {
		t.Errorf("Expected ID 'approve', got %q", trans.ID)
	}
	if trans.Name != "Approve Request" {
		t.Errorf("Expected Name 'Approve Request', got %q", trans.Name)
	}
	if trans.Type != HumanTransition {
		t.Errorf("Expected HumanTransition type, got %v", trans.Type)
	}
	if trans.Trigger == nil {
		t.Error("Expected Trigger to be set")
	}
}

func TestNewHumanTransition_NoTaskByDefault(t *testing.T) {
	trigger := &ManualTrigger{
		ApprovalID: "approval-001",
		Approvers:  []string{"alice@example.com"},
	}
	trans := NewHumanTransition("approve", "Approve Request", trigger)

	if trans.Task != nil {
		t.Error("NewHumanTransition should not set a Task by default")
	}
}

func TestNewGateTransition_SetsType(t *testing.T) {
	guard := func(ctx *context.ExecutionContext, tokens []*token.Token) (bool, error) {
		return len(tokens) > 0, nil
	}
	trans := NewGateTransition("route", "Route Decision", guard)

	if trans.ID != "route" {
		t.Errorf("Expected ID 'route', got %q", trans.ID)
	}
	if trans.Name != "Route Decision" {
		t.Errorf("Expected Name 'Route Decision', got %q", trans.Name)
	}
	if trans.Type != GateTransition {
		t.Errorf("Expected GateTransition type, got %v", trans.Type)
	}
	if trans.Guard == nil {
		t.Error("Expected Guard to be set")
	}
}

func TestNewGateTransition_NoTaskByDefault(t *testing.T) {
	guard := func(ctx *context.ExecutionContext, tokens []*token.Token) (bool, error) {
		return true, nil
	}
	trans := NewGateTransition("gate1", "Gate", guard)

	if trans.Task != nil {
		t.Error("NewGateTransition should not set a Task by default")
	}
}

func TestNewGateTransition_NoTriggerByDefault(t *testing.T) {
	guard := func(ctx *context.ExecutionContext, tokens []*token.Token) (bool, error) {
		return true, nil
	}
	trans := NewGateTransition("gate1", "Gate", guard)

	if trans.Trigger != nil {
		t.Error("NewGateTransition should not set a Trigger by default")
	}
}

// --- Backward compatibility tests ---

func TestTransitionType_BackwardCompatibility_ZeroValue(t *testing.T) {
	// AutoTransition must be the zero value (iota = 0) for backward compatibility.
	// Existing code that creates Transition structs without setting Type will
	// get AutoTransition by default.
	var tt TransitionType
	if tt != AutoTransition {
		t.Errorf("Zero value of TransitionType should be AutoTransition, got %v", tt)
	}
	if int(AutoTransition) != 0 {
		t.Errorf("AutoTransition must be iota=0 for backward compatibility, got %d", int(AutoTransition))
	}
}

func TestTransition_BackwardCompatibility_DefaultType(t *testing.T) {
	// Existing transitions created without WithType() must default to AutoTransition.
	trans := NewTransition("t1", "Legacy Transition")
	if trans.Type != AutoTransition {
		t.Errorf("Legacy transition should have AutoTransition type, got %v", trans.Type)
	}
}

// --- Example showing factory usage ---

func ExampleNewAgentTransition() {
	agentTask := &task.InlineTask{
		Name: "analyze",
		Fn:   func(ctx *context.ExecutionContext) error { return nil },
	}
	trans := NewAgentTransition("analyze-doc", "Analyze Document", agentTask)

	fmt.Printf("ID: %s, Type: %s\n", trans.ID, trans.Type)
	// Output: ID: analyze-doc, Type: agent
}

func ExampleNewGateTransition() {
	trans := NewGateTransition("high-priority", "High Priority Route",
		func(ctx *context.ExecutionContext, tokens []*token.Token) (bool, error) {
			return true, nil
		})

	fmt.Printf("ID: %s, Type: %s\n", trans.ID, trans.Type)
	// Output: ID: high-priority, Type: gate
}
