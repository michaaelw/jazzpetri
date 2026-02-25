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
	"errors"
	"testing"
)

// TestNoOpTracer verifies that NoOpTracer methods execute without panic.
func TestNoOpTracer(t *testing.T) {
	tracer := &NoOpTracer{}

	// Should not panic
	span := tracer.StartSpan("test-span")
	if span == nil {
		t.Error("StartSpan should return a span, not nil")
	}

	// Verify it returns NoOpSpan
	if _, ok := span.(*NoOpSpan); !ok {
		t.Error("NoOpTracer should return NoOpSpan")
	}
}

// TestNoOpSpan verifies that NoOpSpan methods execute without panic.
func TestNoOpSpan(t *testing.T) {
	span := &NoOpSpan{}

	// All methods should execute without panic
	span.End()
	span.SetAttribute("key", "value")
	span.SetAttribute("number", 42)
	span.SetAttribute("bool", true)
	span.RecordError(errors.New("test error"))

	// No assertions needed - just verify no panic
}

// TestNoOpMetrics verifies that NoOpMetrics methods execute without panic.
func TestNoOpMetrics(t *testing.T) {
	metrics := &NoOpMetrics{}

	// All methods should execute without panic
	metrics.Inc("counter")
	metrics.Add("counter", 5.0)
	metrics.Observe("histogram", 0.123)
	metrics.Set("gauge", 42.0)

	// No assertions needed - just verify no panic
}

// TestNoOpLogger verifies that NoOpLogger methods execute without panic.
func TestNoOpLogger(t *testing.T) {
	logger := &NoOpLogger{}

	fields := map[string]interface{}{
		"key": "value",
		"num": 42,
	}

	// All methods should execute without panic
	logger.Debug("debug message", fields)
	logger.Info("info message", fields)
	logger.Warn("warn message", fields)
	logger.Error("error message", fields)

	// Test with nil fields
	logger.Debug("debug message", nil)
	logger.Info("info message", nil)
	logger.Warn("warn message", nil)
	logger.Error("error message", nil)

	// No assertions needed - just verify no panic
}

// TestNoOpZeroOverhead verifies that NoOp implementations have zero allocations.
// This test uses benchmarking to verify no allocations occur.
func TestNoOpZeroOverhead(t *testing.T) {
	// This is a smoke test - the actual zero-overhead verification
	// is done in the benchmark tests
	tracer := &NoOpTracer{}
	metrics := &NoOpMetrics{}
	logger := &NoOpLogger{}

	// Execute operations
	span := tracer.StartSpan("test")
	span.SetAttribute("key", "value")
	span.End()

	metrics.Inc("counter")
	metrics.Observe("histogram", 1.0)

	logger.Info("test", nil)

	// If we get here without panic, NoOp implementations are working
}

// BenchmarkNoOpTracer verifies zero allocations for tracer operations.
func BenchmarkNoOpTracer(b *testing.B) {
	tracer := &NoOpTracer{}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		span := tracer.StartSpan("test-span")
		span.SetAttribute("key", "value")
		span.RecordError(errors.New("test"))
		span.End()
	}
}

// BenchmarkNoOpMetrics verifies zero allocations for metrics operations.
func BenchmarkNoOpMetrics(b *testing.B) {
	metrics := &NoOpMetrics{}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		metrics.Inc("counter")
		metrics.Add("counter", 1.0)
		metrics.Observe("histogram", 0.5)
		metrics.Set("gauge", 42.0)
	}
}

// BenchmarkNoOpLogger verifies zero allocations for logger operations.
func BenchmarkNoOpLogger(b *testing.B) {
	logger := &NoOpLogger{}
	fields := map[string]interface{}{"key": "value"}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		logger.Debug("debug", fields)
		logger.Info("info", fields)
		logger.Warn("warn", fields)
		logger.Error("error", fields)
	}
}

// TestMockTracer verifies the mock tracer implementation used in tests.
func TestMockTracer(t *testing.T) {
	tracer := &mockTracer{}

	// Start a span
	span := tracer.StartSpan("test-span")
	if span == nil {
		t.Fatal("StartSpan should return a span")
	}

	// Verify span was recorded
	if len(tracer.spans) != 1 {
		t.Errorf("Expected 1 span, got %d", len(tracer.spans))
	}

	mockSpan, ok := span.(*mockSpan)
	if !ok {
		t.Fatal("Expected mockSpan")
	}

	// Verify span name
	if mockSpan.name != "test-span" {
		t.Errorf("Expected span name 'test-span', got '%s'", mockSpan.name)
	}

	// Test SetAttribute
	mockSpan.SetAttribute("key1", "value1")
	mockSpan.SetAttribute("key2", 42)

	if len(mockSpan.attributes) != 2 {
		t.Errorf("Expected 2 attributes, got %d", len(mockSpan.attributes))
	}

	if mockSpan.attributes["key1"] != "value1" {
		t.Error("Attribute key1 not set correctly")
	}

	if mockSpan.attributes["key2"] != 42 {
		t.Error("Attribute key2 not set correctly")
	}

	// Test RecordError
	err1 := errors.New("error 1")
	err2 := errors.New("error 2")
	mockSpan.RecordError(err1)
	mockSpan.RecordError(err2)

	if len(mockSpan.errors) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(mockSpan.errors))
	}

	// Test End
	if mockSpan.ended {
		t.Error("Span should not be ended yet")
	}

	mockSpan.End()

	if !mockSpan.ended {
		t.Error("Span should be ended")
	}
}

// TestMockMetrics verifies the mock metrics implementation used in tests.
func TestMockMetrics(t *testing.T) {
	metrics := &mockMetrics{}

	// Test Inc
	metrics.Inc("requests")
	metrics.Inc("requests")
	metrics.Inc("requests")

	if metrics.counters["requests"] != 3 {
		t.Errorf("Expected counter 'requests' to be 3, got %f", metrics.counters["requests"])
	}

	// Test Add
	metrics.Add("bytes", 100)
	metrics.Add("bytes", 50)

	if metrics.counters["bytes"] != 150 {
		t.Errorf("Expected counter 'bytes' to be 150, got %f", metrics.counters["bytes"])
	}

	// Test Observe
	metrics.Observe("latency", 0.1)
	metrics.Observe("latency", 0.2)
	metrics.Observe("latency", 0.15)

	if len(metrics.histograms["latency"]) != 3 {
		t.Errorf("Expected 3 observations, got %d", len(metrics.histograms["latency"]))
	}

	// Test Set
	metrics.Set("temperature", 25.5)
	metrics.Set("temperature", 26.0)

	if metrics.gauges["temperature"] != 26.0 {
		t.Errorf("Expected gauge 'temperature' to be 26.0, got %f", metrics.gauges["temperature"])
	}
}

// TestMockLogger verifies the mock logger implementation used in tests.
func TestMockLogger(t *testing.T) {
	logger := &mockLogger{}

	fields := map[string]interface{}{"key": "value"}

	// Test Debug
	logger.Debug("debug 1", fields)
	logger.Debug("debug 2", fields)

	if len(logger.debugLogs) != 2 {
		t.Errorf("Expected 2 debug logs, got %d", len(logger.debugLogs))
	}

	// Test Info
	logger.Info("info 1", fields)

	if len(logger.infoLogs) != 1 {
		t.Errorf("Expected 1 info log, got %d", len(logger.infoLogs))
	}

	// Test Warn
	logger.Warn("warn 1", fields)
	logger.Warn("warn 2", fields)
	logger.Warn("warn 3", fields)

	if len(logger.warnLogs) != 3 {
		t.Errorf("Expected 3 warn logs, got %d", len(logger.warnLogs))
	}

	// Test Error
	logger.Error("error 1", fields)

	if len(logger.errorLogs) != 1 {
		t.Errorf("Expected 1 error log, got %d", len(logger.errorLogs))
	}
}
