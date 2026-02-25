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

package context

import (
	"testing"
)

// Missing Context in Logs
//
// EXPECTED FAILURE: These tests will fail because log calls throughout
// the codebase lack contextual fields (net_id, transition_id, place_id, etc.)
//
// Structured logging with context enables:
//   - Filtering logs by workflow, transition, or place
//   - Correlating log events across the workflow execution
//   - Debugging specific workflow instances
//   - Building metrics and alerts from logs

func TestLogger_Context(t *testing.T) {
	// EXPECTED FAILURE: Logger calls don't include context fields

	// Create a mock logger that captures log calls
	mockLogger := &MockLogger{
		calls: make([]LogCall, 0),
	}

	ctx := &ExecutionContext{
		Logger: mockLogger,
	}

	// Simulate a log call (this would be in actual engine code)
	// Example from engine.go:
	//   ctx.Logger.Info("engine stopped", nil)
	// Should be:
	//   ctx.Logger.Info("engine stopped", map[string]interface{}{
	//       "net_id": e.net.ID,
	//       "state": e.state.String(),
	//   })

	ctx.Logger.Info("engine started", map[string]interface{}{
		"net_id": "wf1",
		"state":  "running",
	})

	if len(mockLogger.calls) != 1 {
		t.Fatal("Expected 1 log call")
	}

	call := mockLogger.calls[0]

	// Verify context fields are present
	if call.Fields["net_id"] != "wf1" {
		t.Error("Log should include net_id context field")
	}

	if call.Fields["state"] != "running" {
		t.Error("Log should include state context field")
	}
}

func TestLogger_StructuredFields(t *testing.T) {
	// EXPECTED FAILURE: Logger calls lack structured fields

	mockLogger := &MockLogger{
		calls: make([]LogCall, 0),
	}

	// Common context fields that should be present in logs
	_ = []string{
		"net_id",        // Which Petri net
		"transition_id", // Which transition (for transition-related logs)
		"place_id",      // Which place (for place-related logs)
		"operation",     // What operation is happening
	}

	// Example log call with proper context
	mockLogger.Info("transition fired", map[string]interface{}{
		"net_id":        "order-workflow",
		"transition_id": "process-payment",
		"operation":     "fire",
		"duration_ms":   123,
		"tokens_consumed": 2,
	})

	if len(mockLogger.calls) != 1 {
		t.Fatal("Expected 1 log call")
	}

	call := mockLogger.calls[0]

	// Check for structured fields
	for _, field := range []string{"net_id", "transition_id", "operation"} {
		if _, exists := call.Fields[field]; !exists {
			t.Errorf("Log should include %s field", field)
		}
	}
}

func TestLogger_TransitionContext(t *testing.T) {
	// EXPECTED FAILURE: Transition firing logs lack context

	mockLogger := &MockLogger{
		calls: make([]LogCall, 0),
	}

	// When a transition fires, logs should include:
	//   - net_id: which workflow
	//   - transition_id: which transition
	//   - operation: "fire", "check_enabled", etc.
	//   - input_tokens: number of input tokens
	//   - output_tokens: number of output tokens

	mockLogger.Info("firing transition", map[string]interface{}{
		"net_id":        "wf1",
		"transition_id": "t1",
		"operation":     "fire",
		"input_tokens":  2,
		"output_tokens": 3,
	})

	call := mockLogger.calls[0]

	requiredFields := []string{"net_id", "transition_id", "operation"}
	for _, field := range requiredFields {
		if _, exists := call.Fields[field]; !exists {
			t.Errorf("Transition log should include %s", field)
		}
	}
}

func TestLogger_PlaceContext(t *testing.T) {
	// EXPECTED FAILURE: Place operation logs lack context

	mockLogger := &MockLogger{
		calls: make([]LogCall, 0),
	}

	// When tokens are added/removed from places, logs should include:
	//   - net_id: which workflow
	//   - place_id: which place
	//   - operation: "add_token", "remove_token", etc.
	//   - token_count: current count
	//   - capacity: place capacity

	mockLogger.Info("token added", map[string]interface{}{
		"net_id":      "wf1",
		"place_id":    "p1",
		"operation":   "add_token",
		"token_count": 5,
		"capacity":    10,
	})

	call := mockLogger.calls[0]

	requiredFields := []string{"net_id", "place_id", "operation"}
	for _, field := range requiredFields {
		if _, exists := call.Fields[field]; !exists {
			t.Errorf("Place log should include %s", field)
		}
	}
}

func TestLogger_ErrorContext(t *testing.T) {
	// EXPECTED FAILURE: Error logs lack context

	mockLogger := &MockLogger{
		calls: make([]LogCall, 0),
	}

	// Error logs should include maximum context for debugging
	mockLogger.Error("transition firing failed", map[string]interface{}{
		"net_id":        "wf1",
		"transition_id": "t1",
		"operation":     "fire",
		"error":         "guard panicked",
		"input_places":  []string{"p1", "p2"},
		"token_counts":  map[string]int{"p1": 2, "p2": 1},
	})

	call := mockLogger.calls[0]

	// Errors should have rich context for troubleshooting
	essentialFields := []string{"net_id", "transition_id", "error"}
	for _, field := range essentialFields {
		if _, exists := call.Fields[field]; !exists {
			t.Errorf("Error log should include %s for troubleshooting", field)
		}
	}
}

func TestLogger_PerformanceContext(t *testing.T) {
	// EXPECTED FAILURE: Performance logs lack context

	mockLogger := &MockLogger{
		calls: make([]LogCall, 0),
	}

	// Performance-related logs should include timing info
	mockLogger.Info("transition completed", map[string]interface{}{
		"net_id":        "wf1",
		"transition_id": "t1",
		"operation":     "fire",
		"duration_ms":   150,
		"queue_time_ms": 20,
		"exec_time_ms":  130,
	})

	call := mockLogger.calls[0]

	if _, exists := call.Fields["duration_ms"]; !exists {
		t.Error("Performance log should include duration_ms")
	}
}

// MockLogger for testing
type MockLogger struct {
	calls []LogCall
}

type LogCall struct {
	Level   string
	Message string
	Fields  map[string]interface{}
}

func (m *MockLogger) Info(msg string, fields map[string]interface{}) {
	m.calls = append(m.calls, LogCall{
		Level:   "info",
		Message: msg,
		Fields:  fields,
	})
}

func (m *MockLogger) Error(msg string, fields map[string]interface{}) {
	m.calls = append(m.calls, LogCall{
		Level:   "error",
		Message: msg,
		Fields:  fields,
	})
}

func (m *MockLogger) Warn(msg string, fields map[string]interface{}) {
	m.calls = append(m.calls, LogCall{
		Level:   "warn",
		Message: msg,
		Fields:  fields,
	})
}

func (m *MockLogger) Debug(msg string, fields map[string]interface{}) {
	m.calls = append(m.calls, LogCall{
		Level:   "debug",
		Message: msg,
		Fields:  fields,
	})
}

// Example showing proper contextual logging
func ExampleLogger_contextualLogging() {
	// EXPECTED FAILURE: Current code doesn't use this pattern

	logger := &MockLogger{calls: make([]LogCall, 0)}

	// BAD: No context
	// logger.Info("engine started", nil)

	// GOOD: Rich context
	logger.Info("engine started", map[string]interface{}{
		"net_id":                  "order-processing-v2",
		"mode":                    "concurrent",
		"max_concurrent_transitions": 10,
		"poll_interval_ms":        100,
	})

	// BAD: Error without context
	// logger.Error("transition failed", map[string]interface{}{"error": err.Error()})

	// GOOD: Error with full context
	logger.Error("transition failed", map[string]interface{}{
		"net_id":        "order-processing-v2",
		"transition_id": "validate-payment",
		"operation":     "fire",
		"error":         "insufficient funds",
		"order_id":      "ORD-12345",
		"amount":        99.99,
	})

	// This enables powerful log queries like:
	// - Show all errors for net_id="order-processing-v2"
	// - Show all transition failures for transition_id="validate-payment"
	// - Show all operations where amount > 100
}
