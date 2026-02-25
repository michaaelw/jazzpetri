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

// Tracer handles distributed tracing using OpenTelemetry or similar systems.
// Implementations should be thread-safe for concurrent use.
//
// The Tracer interface provides the ability to create trace spans that represent
// units of work in a distributed system. Spans can be nested to show parent-child
// relationships and can carry attributes for additional context.
//
// Use NoOpTracer when tracing is disabled for zero overhead.
type Tracer interface {
	// StartSpan creates a new trace span with the given name.
	// The span should be ended by calling End() when the operation completes.
	//
	// Example:
	//   span := tracer.StartSpan("transition-fire")
	//   defer span.End()
	StartSpan(name string) Span
}

// Span represents a single trace span in distributed tracing.
// Spans track operations and can carry attributes for additional context.
// Implementations must be safe for concurrent use.
type Span interface {
	// End marks the span as complete.
	// This should be called when the operation finishes, typically via defer.
	End()

	// SetAttribute adds a key-value attribute to the span.
	// Attributes provide additional context about the operation.
	//
	// Common attribute types:
	//   - string: textual information
	//   - int/int64: numeric values
	//   - bool: flags
	//   - float64: measurements
	SetAttribute(key string, value interface{})

	// RecordError records an error that occurred during the span.
	// This is separate from ending the span and can be called multiple times.
	RecordError(err error)
}

// MetricsCollector handles metrics collection using Prometheus or similar systems.
// Implementations should be thread-safe for concurrent use.
//
// The MetricsCollector provides methods for common metric types:
//   - Counters: monotonically increasing values (e.g., requests processed)
//   - Histograms: distributions of values (e.g., latencies)
//   - Gauges: values that can go up and down (e.g., queue size)
//
// Use NoOpMetrics when metrics are disabled for zero overhead.
type MetricsCollector interface {
	// Inc increments a counter metric by 1.
	// Counters are used for monotonically increasing values.
	//
	// Example:
	//   metrics.Inc("transitions_fired")
	Inc(name string)

	// Add adds a value to a counter or gauge metric.
	// For counters, the value should be positive.
	// For gauges, the value can be positive or negative.
	//
	// Example:
	//   metrics.Add("tokens_produced", 5)
	Add(name string, value float64)

	// Observe records a value in a histogram metric.
	// Histograms are used for tracking distributions (e.g., latencies).
	//
	// Example:
	//   metrics.Observe("transition_duration_seconds", 0.123)
	Observe(name string, value float64)

	// Set sets a gauge metric to a specific value.
	// Gauges represent values that can go up and down.
	//
	// Example:
	//   metrics.Set("active_workflows", 42)
	Set(name string, value float64)
}

// Logger handles structured logging with contextual fields.
// Implementations should be thread-safe for concurrent use.
//
// The Logger interface provides leveled logging methods that accept
// structured fields for rich context. Use map[string]interface{} for fields.
//
// Use NoOpLogger when logging is disabled for zero overhead.
type Logger interface {
	// Debug logs a debug-level message with optional fields.
	// Debug logs are typically used for detailed troubleshooting information.
	//
	// Example:
	//   logger.Debug("Processing token", map[string]interface{}{
	//       "token_id": tok.ID,
	//       "transition": "t1",
	//   })
	Debug(msg string, fields map[string]interface{})

	// Info logs an info-level message with optional fields.
	// Info logs are used for general informational messages.
	//
	// Example:
	//   logger.Info("Workflow started", map[string]interface{}{
	//       "workflow_id": "wf-123",
	//   })
	Info(msg string, fields map[string]interface{})

	// Warn logs a warning-level message with optional fields.
	// Warnings indicate potentially problematic situations.
	//
	// Example:
	//   logger.Warn("High token count", map[string]interface{}{
	//       "place": "queue",
	//       "count": 1000,
	//   })
	Warn(msg string, fields map[string]interface{})

	// Error logs an error-level message with optional fields.
	// Errors indicate failure conditions that should be investigated.
	//
	// Example:
	//   logger.Error("Transition failed", map[string]interface{}{
	//       "transition": "t1",
	//       "error": err.Error(),
	//   })
	Error(msg string, fields map[string]interface{})
}

// EventType identifies the type of workflow event.
// These constants are used when creating events for the event log.
type EventType string

const (
	// EventTransitionFired is recorded when a transition successfully fires
	EventTransitionFired EventType = "transition.fired"

	// EventTransitionEnabled is recorded when a transition becomes enabled
	EventTransitionEnabled EventType = "transition.enabled"

	// EventTransitionFailed is recorded when a transition fails to fire
	EventTransitionFailed EventType = "transition.failed"

	// EventEngineStarted is recorded when the workflow engine starts
	EventEngineStarted EventType = "engine.started"

	// EventEngineStopped is recorded when the workflow engine stops
	EventEngineStopped EventType = "engine.stopped"

	// EventTokenProduced is recorded when a token is produced to a place
	EventTokenProduced EventType = "token.produced"

	// EventTokenConsumed is recorded when a token is consumed from a place
	EventTokenConsumed EventType = "token.consumed"

	// EventErrorOccurred is recorded when an error occurs during execution
	EventErrorOccurred EventType = "error.occurred"
)

// EventLog is the interface for event storage and retrieval.
// This is a forward declaration to avoid circular dependencies with pkg/state.
// The actual Event type and implementations are in pkg/state.
//
// EventLog provides an append-only audit trail of workflow events.
// It is used for recovery, replay, debugging, and auditing.
//
// Note: EventLog is optional and defaults to nil for zero overhead.
// Use ExecutionContext.WithEventLog() to enable event logging.
type EventLog interface {
	// Append adds a new event to the log.
	// The event parameter will be of type *state.Event.
	// Returns an error if the event cannot be appended.
	Append(event interface{}) error
}

// EventBuilder creates events for the event log.
// This helper avoids circular dependencies by allowing event creation
// without importing pkg/state.
type EventBuilder interface {
	// NewEvent creates a new event with the given type and net ID.
	// Returns an event that can be passed to EventLog.Append().
	NewEvent(eventType EventType, netID string) interface{}
}
