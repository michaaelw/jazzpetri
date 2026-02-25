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

// NoOp implementations provide zero-overhead observability when disabled.
// These implementations are used as defaults in ExecutionContext and have
// no runtime cost - the compiler optimizes away the empty method calls.

// NoOpTracer is a zero-overhead tracer implementation used when tracing is disabled.
// All methods are no-ops and should be optimized away by the compiler.
type NoOpTracer struct{}

// StartSpan returns a NoOpSpan. This is a no-op.
func (n *NoOpTracer) StartSpan(name string) Span {
	return &NoOpSpan{}
}

// NoOpSpan is a zero-overhead span implementation.
// All methods are no-ops and should be optimized away by the compiler.
type NoOpSpan struct{}

// End is a no-op.
func (n *NoOpSpan) End() {}

// SetAttribute is a no-op.
func (n *NoOpSpan) SetAttribute(key string, value interface{}) {}

// RecordError is a no-op.
func (n *NoOpSpan) RecordError(err error) {}

// NoOpMetrics is a zero-overhead metrics collector used when metrics are disabled.
// All methods are no-ops and should be optimized away by the compiler.
type NoOpMetrics struct{}

// Inc is a no-op.
func (n *NoOpMetrics) Inc(name string) {}

// Add is a no-op.
func (n *NoOpMetrics) Add(name string, value float64) {}

// Observe is a no-op.
func (n *NoOpMetrics) Observe(name string, value float64) {}

// Set is a no-op.
func (n *NoOpMetrics) Set(name string, value float64) {}

// NoOpLogger is a zero-overhead logger used when logging is disabled.
// All methods are no-ops and should be optimized away by the compiler.
type NoOpLogger struct{}

// Debug is a no-op.
func (n *NoOpLogger) Debug(msg string, fields map[string]interface{}) {}

// Info is a no-op.
func (n *NoOpLogger) Info(msg string, fields map[string]interface{}) {}

// Warn is a no-op.
func (n *NoOpLogger) Warn(msg string, fields map[string]interface{}) {}

// Error is a no-op.
func (n *NoOpLogger) Error(msg string, fields map[string]interface{}) {}
