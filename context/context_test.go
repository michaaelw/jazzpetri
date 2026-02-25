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
	"context"
	"testing"
	"time"

	"github.com/jazzpetri/engine/clock"
	"github.com/jazzpetri/engine/token"
)

// TestNewExecutionContext verifies that NewExecutionContext creates a context
// with the correct default values.
func TestNewExecutionContext(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewRealTimeClock()
	tok := &token.Token{
		ID: "test-token",
		Data: &token.IntToken{
			Value: 42,
		},
	}

	execCtx := NewExecutionContext(ctx, clk, tok)

	// Verify basic fields
	if execCtx.Context != ctx {
		t.Error("Context not set correctly")
	}
	if execCtx.Clock != clk {
		t.Error("Clock not set correctly")
	}
	if execCtx.Token != tok {
		t.Error("Token not set correctly")
	}

	// Verify NoOp implementations are set
	if execCtx.Tracer == nil {
		t.Error("Tracer should not be nil")
	}
	if _, ok := execCtx.Tracer.(*NoOpTracer); !ok {
		t.Error("Tracer should be NoOpTracer by default")
	}

	if execCtx.Metrics == nil {
		t.Error("Metrics should not be nil")
	}
	if _, ok := execCtx.Metrics.(*NoOpMetrics); !ok {
		t.Error("Metrics should be NoOpMetrics by default")
	}

	if execCtx.Logger == nil {
		t.Error("Logger should not be nil")
	}
	if _, ok := execCtx.Logger.(*NoOpLogger); !ok {
		t.Error("Logger should be NoOpLogger by default")
	}

	if execCtx.ErrorHandler == nil {
		t.Error("ErrorHandler should not be nil")
	}
	if _, ok := execCtx.ErrorHandler.(*NoOpErrorHandler); !ok {
		t.Error("ErrorHandler should be NoOpErrorHandler by default")
	}

	if execCtx.ErrorRecorder == nil {
		t.Error("ErrorRecorder should not be nil")
	}
	if _, ok := execCtx.ErrorRecorder.(*NoOpErrorRecorder); !ok {
		t.Error("ErrorRecorder should be NoOpErrorRecorder by default")
	}

	// Verify no parent
	if execCtx.Parent() != nil {
		t.Error("New context should have no parent")
	}
}

// TestNewExecutionContext_NilToken verifies that NewExecutionContext accepts nil token.
func TestNewExecutionContext_NilToken(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewRealTimeClock()

	execCtx := NewExecutionContext(ctx, clk, nil)

	if execCtx.Token != nil {
		t.Error("Token should be nil when passed as nil")
	}
}

// TestWithParent verifies that WithParent creates a child context with proper inheritance.
func TestWithParent(t *testing.T) {
	// Create parent context
	parentCtx := context.Background()
	parentClk := clock.NewRealTimeClock()
	parentToken := &token.Token{ID: "parent-token"}

	parent := NewExecutionContext(parentCtx, parentClk, parentToken)

	// Create custom implementations for parent
	parentTracer := &mockTracer{}
	parentMetrics := &mockMetrics{}
	parentLogger := &mockLogger{}
	parentHandler := &mockErrorHandler{}
	parentRecorder := &mockErrorRecorder{}

	parent = parent.WithTracer(parentTracer)
	parent = parent.WithMetrics(parentMetrics)
	parent = parent.WithLogger(parentLogger)
	parent = parent.WithErrorHandler(parentHandler)
	parent = parent.WithErrorRecorder(parentRecorder)

	// Create child context with different token
	childCtx := context.Background()
	childClk := clock.NewVirtualClock(time.Now())
	childToken := &token.Token{ID: "child-token"}

	child := NewExecutionContext(childCtx, childClk, childToken)
	child = child.WithParent(parent)

	// Verify child inherits observability from parent
	if child.Tracer != parentTracer {
		t.Error("Child should inherit Tracer from parent")
	}
	if child.Metrics != parentMetrics {
		t.Error("Child should inherit Metrics from parent")
	}
	if child.Logger != parentLogger {
		t.Error("Child should inherit Logger from parent")
	}
	if child.ErrorHandler != parentHandler {
		t.Error("Child should inherit ErrorHandler from parent")
	}
	if child.ErrorRecorder != parentRecorder {
		t.Error("Child should inherit ErrorRecorder from parent")
	}

	// Verify child maintains its own identity
	if child.Context != childCtx {
		t.Error("Child should maintain its own Context")
	}
	if child.Clock != childClk {
		t.Error("Child should maintain its own Clock")
	}
	if child.Token != childToken {
		t.Error("Child should maintain its own Token")
	}

	// Verify parent relationship
	if child.Parent() != parent {
		t.Error("Child.Parent() should return parent")
	}
}

// TestWithTracer verifies that WithTracer creates a new context with custom tracer.
func TestWithTracer(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewRealTimeClock()
	execCtx := NewExecutionContext(ctx, clk, nil)

	customTracer := &mockTracer{}
	newCtx := execCtx.WithTracer(customTracer)

	// Verify new context has custom tracer
	if newCtx.Tracer != customTracer {
		t.Error("WithTracer should set custom tracer")
	}

	// Verify original context unchanged
	if _, ok := execCtx.Tracer.(*NoOpTracer); !ok {
		t.Error("Original context should still have NoOpTracer")
	}

	// Verify it's a new context
	if newCtx == execCtx {
		t.Error("WithTracer should create a new context")
	}
}

// TestWithMetrics verifies that WithMetrics creates a new context with custom metrics.
func TestWithMetrics(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewRealTimeClock()
	execCtx := NewExecutionContext(ctx, clk, nil)

	customMetrics := &mockMetrics{}
	newCtx := execCtx.WithMetrics(customMetrics)

	if newCtx.Metrics != customMetrics {
		t.Error("WithMetrics should set custom metrics")
	}

	if _, ok := execCtx.Metrics.(*NoOpMetrics); !ok {
		t.Error("Original context should still have NoOpMetrics")
	}
}

// TestWithLogger verifies that WithLogger creates a new context with custom logger.
func TestWithLogger(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewRealTimeClock()
	execCtx := NewExecutionContext(ctx, clk, nil)

	customLogger := &mockLogger{}
	newCtx := execCtx.WithLogger(customLogger)

	if newCtx.Logger != customLogger {
		t.Error("WithLogger should set custom logger")
	}

	if _, ok := execCtx.Logger.(*NoOpLogger); !ok {
		t.Error("Original context should still have NoOpLogger")
	}
}

// TestWithErrorHandler verifies that WithErrorHandler creates a new context with custom handler.
func TestWithErrorHandler(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewRealTimeClock()
	execCtx := NewExecutionContext(ctx, clk, nil)

	customHandler := &mockErrorHandler{}
	newCtx := execCtx.WithErrorHandler(customHandler)

	if newCtx.ErrorHandler != customHandler {
		t.Error("WithErrorHandler should set custom handler")
	}

	if _, ok := execCtx.ErrorHandler.(*NoOpErrorHandler); !ok {
		t.Error("Original context should still have NoOpErrorHandler")
	}
}

// TestWithErrorRecorder verifies that WithErrorRecorder creates a new context with custom recorder.
func TestWithErrorRecorder(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewRealTimeClock()
	execCtx := NewExecutionContext(ctx, clk, nil)

	customRecorder := &mockErrorRecorder{}
	newCtx := execCtx.WithErrorRecorder(customRecorder)

	if newCtx.ErrorRecorder != customRecorder {
		t.Error("WithErrorRecorder should set custom recorder")
	}

	if _, ok := execCtx.ErrorRecorder.(*NoOpErrorRecorder); !ok {
		t.Error("Original context should still have NoOpErrorRecorder")
	}
}

// TestWithToken verifies that WithToken creates a new context with different token.
func TestWithToken(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewRealTimeClock()
	token1 := &token.Token{ID: "token-1"}
	execCtx := NewExecutionContext(ctx, clk, token1)

	token2 := &token.Token{ID: "token-2"}
	newCtx := execCtx.WithToken(token2)

	if newCtx.Token != token2 {
		t.Error("WithToken should set new token")
	}

	if execCtx.Token != token1 {
		t.Error("Original context should still have original token")
	}
}

// TestWithContext verifies that WithContext creates a new context with different Go context.
func TestWithContext(t *testing.T) {
	ctx1 := context.Background()
	ctx2, cancel := context.WithCancel(context.Background())
	defer cancel()

	clk := clock.NewRealTimeClock()
	execCtx := NewExecutionContext(ctx1, clk, nil)

	newCtx := execCtx.WithContext(ctx2)

	if newCtx.Context != ctx2 {
		t.Error("WithContext should set new context")
	}

	if execCtx.Context != ctx1 {
		t.Error("Original context should still have original Go context")
	}
}

// TestHierarchicalInheritance verifies multi-level context inheritance.
func TestHierarchicalInheritance(t *testing.T) {
	// Create root context
	root := NewExecutionContext(context.Background(), clock.NewRealTimeClock(), nil)
	rootTracer := &mockTracer{}
	root = root.WithTracer(rootTracer)

	// Create level-1 child
	child1 := NewExecutionContext(context.Background(), clock.NewRealTimeClock(), nil)
	child1 = child1.WithParent(root)

	// Create level-2 grandchild
	child2 := NewExecutionContext(context.Background(), clock.NewRealTimeClock(), nil)
	child2 = child2.WithParent(child1)

	// Verify inheritance through multiple levels
	if child2.Tracer != rootTracer {
		t.Error("Grandchild should inherit tracer from root through parent")
	}

	// Verify parent chain
	if child2.Parent() != child1 {
		t.Error("child2.Parent() should be child1")
	}
	if child1.Parent() != root {
		t.Error("child1.Parent() should be root")
	}
	if root.Parent() != nil {
		t.Error("root.Parent() should be nil")
	}
}

// Mock implementations for testing

type mockTracer struct {
	spans []*mockSpan
}

func (m *mockTracer) StartSpan(name string) Span {
	span := &mockSpan{name: name}
	m.spans = append(m.spans, span)
	return span
}

type mockSpan struct {
	name       string
	attributes map[string]interface{}
	errors     []error
	ended      bool
}

func (m *mockSpan) End() {
	m.ended = true
}

func (m *mockSpan) SetAttribute(key string, value interface{}) {
	if m.attributes == nil {
		m.attributes = make(map[string]interface{})
	}
	m.attributes[key] = value
}

func (m *mockSpan) RecordError(err error) {
	m.errors = append(m.errors, err)
}

type mockMetrics struct {
	counters   map[string]float64
	histograms map[string][]float64
	gauges     map[string]float64
}

func (m *mockMetrics) Inc(name string) {
	if m.counters == nil {
		m.counters = make(map[string]float64)
	}
	m.counters[name]++
}

func (m *mockMetrics) Add(name string, value float64) {
	if m.counters == nil {
		m.counters = make(map[string]float64)
	}
	m.counters[name] += value
}

func (m *mockMetrics) Observe(name string, value float64) {
	if m.histograms == nil {
		m.histograms = make(map[string][]float64)
	}
	m.histograms[name] = append(m.histograms[name], value)
}

func (m *mockMetrics) Set(name string, value float64) {
	if m.gauges == nil {
		m.gauges = make(map[string]float64)
	}
	m.gauges[name] = value
}

type mockLogger struct {
	debugLogs []string
	infoLogs  []string
	warnLogs  []string
	errorLogs []string
}

func (m *mockLogger) Debug(msg string, fields map[string]interface{}) {
	m.debugLogs = append(m.debugLogs, msg)
}

func (m *mockLogger) Info(msg string, fields map[string]interface{}) {
	m.infoLogs = append(m.infoLogs, msg)
}

func (m *mockLogger) Warn(msg string, fields map[string]interface{}) {
	m.warnLogs = append(m.warnLogs, msg)
}

func (m *mockLogger) Error(msg string, fields map[string]interface{}) {
	m.errorLogs = append(m.errorLogs, msg)
}

type mockErrorHandler struct {
	handledErrors []error
}

func (m *mockErrorHandler) Handle(ctx *ExecutionContext, err error) error {
	m.handledErrors = append(m.handledErrors, err)
	return err
}

type mockErrorRecorder struct {
	errors        []error
	retries       []int
	compensations []string
}

func (m *mockErrorRecorder) RecordError(err error, metadata map[string]interface{}) {
	m.errors = append(m.errors, err)
}

func (m *mockErrorRecorder) RecordRetry(attempt int, nextRetry interface{}) {
	m.retries = append(m.retries, attempt)
}

func (m *mockErrorRecorder) RecordCompensation(action string) {
	m.compensations = append(m.compensations, action)
}
