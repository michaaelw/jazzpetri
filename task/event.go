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
	"fmt"

	"github.com/jazzpetri/engine/context"
)

// EventEmitter is the interface for publishing events to external systems.
// Implementations can target message queues, event buses, webhooks, etc.
//
// This is the PORT in the Ports & Adapters pattern. Concrete implementations
// (Kafka, RabbitMQ, Redis Pub/Sub, etc.) are ADAPTERS.
type EventEmitter interface {
	// Emit publishes an event to the specified topic/channel.
	// The payload can be any serializable data.
	// Returns an error if the event could not be published.
	Emit(ctx *context.ExecutionContext, topic string, payload interface{}) error
}

// EventPayloadFunc generates the event payload dynamically from the execution context.
// This allows event data to be derived from the current token state.
type EventPayloadFunc func(ctx *context.ExecutionContext) (interface{}, error)

// EmitEventTask publishes events to external messaging systems.
// This enables event-driven architectures, pub/sub patterns, and system integration.
//
// EmitEventTask is ideal for:
//   - Publishing domain events to event buses
//   - Triggering downstream workflows
//   - Notifying external systems of state changes
//   - Audit logging to event streams
//   - Integration with message queues (Kafka, RabbitMQ, etc.)
//
// Example - Static payload:
//
//	task := &EmitEventTask{
//	    Name:    "order-created",
//	    Emitter: kafkaEmitter,
//	    Topic:   "orders.created",
//	    Payload: map[string]interface{}{
//	        "event_type": "ORDER_CREATED",
//	        "version":    "1.0",
//	    },
//	}
//
// Example - Dynamic payload from token:
//
//	task := &EmitEventTask{
//	    Name:    "order-completed",
//	    Emitter: kafkaEmitter,
//	    Topic:   "orders.completed",
//	    PayloadFunc: func(ctx *context.ExecutionContext) (interface{}, error) {
//	        order := ctx.Token.Data.(*OrderData)
//	        return map[string]interface{}{
//	            "order_id": order.ID,
//	            "total":    order.Total,
//	            "status":   order.Status,
//	        }, nil
//	    },
//	}
//
// Thread Safety:
// EmitEventTask is safe for concurrent execution. The EventEmitter implementation
// must also be thread-safe.
type EmitEventTask struct {
	// Name is the task identifier used in logs, traces, and metrics.
	Name string

	// Emitter is the event publishing backend.
	// This is the adapter that connects to the actual messaging system.
	Emitter EventEmitter

	// Topic is the destination channel/topic/queue for the event.
	// The interpretation depends on the Emitter implementation.
	Topic string

	// Payload is the static event data to publish.
	// Ignored if PayloadFunc is set.
	Payload interface{}

	// PayloadFunc generates the event payload dynamically.
	// This allows event data to be derived from token state at runtime.
	// Takes precedence over Payload if both are set.
	PayloadFunc EventPayloadFunc
}

// Execute publishes the event to the configured emitter.
//
// Execution flow:
//  1. Validate task configuration
//  2. Start trace span
//  3. Generate payload (from PayloadFunc or static Payload)
//  4. Publish event via Emitter
//  5. Record metrics and complete
//
// Thread Safety:
// Execute is safe to call concurrently from multiple goroutines.
//
// Returns an error if:
//   - Task configuration is invalid (missing Name, Emitter, Topic)
//   - PayloadFunc returns an error
//   - Emitter.Emit fails
func (e *EmitEventTask) Execute(ctx *context.ExecutionContext) error {
	// Validate configuration
	if e.Name == "" {
		return fmt.Errorf("emit event task: name is required")
	}
	if e.Emitter == nil {
		return fmt.Errorf("emit event task: emitter is required")
	}
	if e.Topic == "" {
		return fmt.Errorf("emit event task: topic is required")
	}

	// Start trace span
	span := ctx.Tracer.StartSpan("task.emit." + e.Name)
	defer span.End()

	span.SetAttribute("event.topic", e.Topic)

	// Record metrics
	ctx.Metrics.Inc("task_executions_total")
	ctx.Metrics.Inc("task_emit_executions_total")

	// Log task start
	ctx.Logger.Debug("Executing emit event task", map[string]interface{}{
		"task_name": e.Name,
		"topic":     e.Topic,
		"token_id":  tokenID(ctx),
	})

	// Generate payload
	var payload interface{}
	var err error

	if e.PayloadFunc != nil {
		payload, err = e.PayloadFunc(ctx)
		if err != nil {
			return e.handleError(ctx, span, fmt.Errorf("payload generation failed: %w", err))
		}
	} else {
		payload = e.Payload
	}

	// Emit event
	if err := e.Emitter.Emit(ctx, e.Topic, payload); err != nil {
		return e.handleError(ctx, span, fmt.Errorf("event emission failed: %w", err))
	}

	// Success
	ctx.Metrics.Inc("task_success_total")
	ctx.Metrics.Inc("task_emit_success_total")
	ctx.Metrics.Inc(fmt.Sprintf("task_emit_topic_%s_total", sanitizeMetricName(e.Topic)))

	ctx.Logger.Debug("Emit event task completed", map[string]interface{}{
		"task_name": e.Name,
		"topic":     e.Topic,
		"token_id":  tokenID(ctx),
	})

	return nil
}

// handleError processes errors with observability and error handling
func (e *EmitEventTask) handleError(ctx *context.ExecutionContext, span context.Span, err error) error {
	span.RecordError(err)
	ctx.Metrics.Inc("task_errors_total")
	ctx.Metrics.Inc("task_emit_errors_total")
	ctx.ErrorRecorder.RecordError(err, map[string]interface{}{
		"task_name": e.Name,
		"topic":     e.Topic,
		"token_id":  tokenID(ctx),
	})
	ctx.Logger.Error("Emit event task failed", map[string]interface{}{
		"task_name": e.Name,
		"topic":     e.Topic,
		"error":     err.Error(),
		"token_id":  tokenID(ctx),
	})
	return ctx.ErrorHandler.Handle(ctx, err)
}

// sanitizeMetricName converts a topic name to a valid metric label
func sanitizeMetricName(name string) string {
	result := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			result = append(result, c)
		} else {
			result = append(result, '_')
		}
	}
	return string(result)
}

// ChannelEventEmitter is a simple in-memory EventEmitter using Go channels.
// Useful for testing and simple in-process event distribution.
type ChannelEventEmitter struct {
	// Channels maps topic names to their event channels
	Channels map[string]chan interface{}
}

// NewChannelEventEmitter creates a new channel-based event emitter
func NewChannelEventEmitter() *ChannelEventEmitter {
	return &ChannelEventEmitter{
		Channels: make(map[string]chan interface{}),
	}
}

// Subscribe creates a channel for receiving events on a topic
func (c *ChannelEventEmitter) Subscribe(topic string, bufferSize int) <-chan interface{} {
	ch := make(chan interface{}, bufferSize)
	c.Channels[topic] = ch
	return ch
}

// Emit publishes an event to the topic's channel
func (c *ChannelEventEmitter) Emit(ctx *context.ExecutionContext, topic string, payload interface{}) error {
	ch, exists := c.Channels[topic]
	if !exists {
		// No subscribers, that's OK - event is discarded
		return nil
	}

	select {
	case ch <- payload:
		return nil
	case <-ctx.Context.Done():
		return ctx.Context.Err()
	default:
		return fmt.Errorf("event channel full for topic: %s", topic)
	}
}
