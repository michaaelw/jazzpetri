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

package task

import (
	stdContext "context"
	"strings"
	"testing"
	"time"

	"github.com/jazzpetri/engine/clock"
	"github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/token"
)

func TestEmitEventTask_StaticPayload(t *testing.T) {
	// Create virtual clock
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(start)

	// Create execution context
	tok := &token.Token{ID: "test-token", Metadata: make(map[string]interface{})}
	ctx := context.NewExecutionContext(stdContext.Background(), clk, tok)

	// Create channel emitter
	emitter := NewChannelEventEmitter()
	eventCh := emitter.Subscribe("orders.created", 10)

	// Create emit event task
	payload := map[string]interface{}{
		"order_id": "12345",
		"amount":   100.50,
	}
	task := &EmitEventTask{
		Name:    "emit-order",
		Emitter: emitter,
		Topic:   "orders.created",
		Payload: payload,
	}

	// Execute
	err := task.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check event was received
	select {
	case received := <-eventCh:
		receivedMap := received.(map[string]interface{})
		if receivedMap["order_id"] != "12345" {
			t.Errorf("expected order_id 12345, got %v", receivedMap["order_id"])
		}
	default:
		t.Fatal("expected event on channel")
	}
}

func TestEmitEventTask_DynamicPayload(t *testing.T) {
	// Create virtual clock
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(start)

	// Create execution context with token data
	tok := &token.Token{
		ID:       "test-token",
		Metadata: make(map[string]interface{}),
	}
	tok.Metadata["order_id"] = "order-123"
	tok.Metadata["status"] = "completed"
	ctx := context.NewExecutionContext(stdContext.Background(), clk, tok)

	// Create channel emitter
	emitter := NewChannelEventEmitter()
	eventCh := emitter.Subscribe("orders.completed", 10)

	// Create emit event task with dynamic payload
	task := &EmitEventTask{
		Name:    "emit-order-completed",
		Emitter: emitter,
		Topic:   "orders.completed",
		PayloadFunc: func(ctx *context.ExecutionContext) (interface{}, error) {
			return map[string]interface{}{
				"order_id": ctx.Token.Metadata["order_id"],
				"status":   ctx.Token.Metadata["status"],
				"timestamp": ctx.Clock.Now().Format(time.RFC3339),
			}, nil
		},
	}

	// Execute
	err := task.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check event was received
	select {
	case received := <-eventCh:
		receivedMap := received.(map[string]interface{})
		if receivedMap["order_id"] != "order-123" {
			t.Errorf("expected order_id order-123, got %v", receivedMap["order_id"])
		}
		if receivedMap["status"] != "completed" {
			t.Errorf("expected status completed, got %v", receivedMap["status"])
		}
	default:
		t.Fatal("expected event on channel")
	}
}

func TestEmitEventTask_NoSubscribers(t *testing.T) {
	// Create virtual clock
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(start)

	// Create execution context
	tok := &token.Token{ID: "test-token", Metadata: make(map[string]interface{})}
	ctx := context.NewExecutionContext(stdContext.Background(), clk, tok)

	// Create channel emitter (no subscribers)
	emitter := NewChannelEventEmitter()

	// Create emit event task
	task := &EmitEventTask{
		Name:    "emit-no-sub",
		Emitter: emitter,
		Topic:   "unsubscribed.topic",
		Payload: "test",
	}

	// Execute - should succeed even with no subscribers
	err := task.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEmitEventTask_ChannelFull(t *testing.T) {
	// Create virtual clock
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(start)

	// Create execution context
	tok := &token.Token{ID: "test-token", Metadata: make(map[string]interface{})}
	ctx := context.NewExecutionContext(stdContext.Background(), clk, tok)

	// Create channel emitter with small buffer
	emitter := NewChannelEventEmitter()
	_ = emitter.Subscribe("small.channel", 1) // Buffer of 1

	// Fill the channel
	task := &EmitEventTask{
		Name:    "emit-1",
		Emitter: emitter,
		Topic:   "small.channel",
		Payload: "first",
	}
	err := task.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error on first emit: %v", err)
	}

	// Second emit should fail (channel full)
	task = &EmitEventTask{
		Name:    "emit-2",
		Emitter: emitter,
		Topic:   "small.channel",
		Payload: "second",
	}
	err = task.Execute(ctx)
	if err == nil {
		t.Fatal("expected error for full channel")
	}
}

func TestEmitEventTask_ContextCancellation(t *testing.T) {
	// Create virtual clock
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(start)

	// Create cancelled context
	goCtx, cancel := stdContext.WithCancel(stdContext.Background())
	cancel() // Cancel immediately
	tok := &token.Token{ID: "test-token", Metadata: make(map[string]interface{})}
	ctx := context.NewExecutionContext(goCtx, clk, tok)

	// Create channel emitter with full channel
	emitter := NewChannelEventEmitter()
	_ = emitter.Subscribe("test.topic", 0) // Zero buffer

	// Create emit event task
	task := &EmitEventTask{
		Name:    "emit-cancelled",
		Emitter: emitter,
		Topic:   "test.topic",
		Payload: "test",
	}

	// Execute - should fail with context error (wrapped in emission error)
	err := task.Execute(ctx)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	// The error is wrapped, so check that it contains the context error
	if !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("expected error containing 'context canceled', got: %v", err)
	}
}

func TestEmitEventTask_ValidationErrors(t *testing.T) {
	// Create virtual clock
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(start)

	tok := &token.Token{ID: "test-token", Metadata: make(map[string]interface{})}
	ctx := context.NewExecutionContext(stdContext.Background(), clk, tok)

	emitter := NewChannelEventEmitter()

	// Test missing name
	task := &EmitEventTask{
		Emitter: emitter,
		Topic:   "test.topic",
		Payload: "test",
	}
	err := task.Execute(ctx)
	if err == nil || err.Error() != "emit event task: name is required" {
		t.Fatalf("expected name required error, got: %v", err)
	}

	// Test missing emitter
	task = &EmitEventTask{
		Name:    "test",
		Topic:   "test.topic",
		Payload: "test",
	}
	err = task.Execute(ctx)
	if err == nil || err.Error() != "emit event task: emitter is required" {
		t.Fatalf("expected emitter required error, got: %v", err)
	}

	// Test missing topic
	task = &EmitEventTask{
		Name:    "test",
		Emitter: emitter,
		Payload: "test",
	}
	err = task.Execute(ctx)
	if err == nil || err.Error() != "emit event task: topic is required" {
		t.Fatalf("expected topic required error, got: %v", err)
	}
}

func TestEmitEventTask_PayloadFuncError(t *testing.T) {
	// Create virtual clock
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := clock.NewVirtualClock(start)

	tok := &token.Token{ID: "test-token", Metadata: make(map[string]interface{})}
	ctx := context.NewExecutionContext(stdContext.Background(), clk, tok)

	emitter := NewChannelEventEmitter()
	emitter.Subscribe("test.topic", 10)

	// Create task with failing payload function
	task := &EmitEventTask{
		Name:    "emit-fail",
		Emitter: emitter,
		Topic:   "test.topic",
		PayloadFunc: func(ctx *context.ExecutionContext) (interface{}, error) {
			return nil, stdContext.DeadlineExceeded
		},
	}

	// Execute - should fail
	err := task.Execute(ctx)
	if err == nil {
		t.Fatal("expected error from payload function")
	}
}

func TestSanitizeMetricName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"orders.created", "orders_created"},
		{"orders-completed", "orders_completed"},
		{"simple", "simple"},
		{"with spaces", "with_spaces"},
		{"UPPERCASE", "UPPERCASE"},
		{"mixed_123", "mixed_123"},
		{"special!@#$%chars", "special_____chars"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeMetricName(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeMetricName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
