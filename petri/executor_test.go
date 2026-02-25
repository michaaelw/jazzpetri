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
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jazzpetri/engine/clock"
	execctx "github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/task"
	"github.com/jazzpetri/engine/token"
)

// Mock task for testing
type mockTask struct {
	name      string
	shouldErr bool
	executed  bool
	callCount int
	mu        sync.Mutex // Protects callCount and executed for concurrent access
}

func (m *mockTask) Execute(ctx *execctx.ExecutionContext) error {
	m.mu.Lock()
	m.executed = true
	m.callCount++
	m.mu.Unlock()

	if m.shouldErr {
		return errors.New("task execution failed")
	}
	return nil
}

func (m *mockTask) getCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// Mock token data
type mockTokenData struct {
	value string
}

func (m *mockTokenData) Type() string {
	return "mock"
}

// TestTransitionWithTask tests setting a task on a transition
func TestTransitionWithTask(t *testing.T) {
	trans := NewTransition("t1", "Test Transition")
	mockTask := &mockTask{name: "test-task"}

	result := trans.WithTask(mockTask)

	// Should return transition for chaining
	if result != trans {
		t.Error("WithTask should return the transition for chaining")
	}

	// Task should be set
	if trans.Task != mockTask {
		t.Error("Task was not set on transition")
	}
}

// TestFireUnresolvedTransition tests firing an unresolved transition
func TestFireUnresolvedTransition(t *testing.T) {
	trans := NewTransition("t1", "Test Transition")
	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	err := trans.Fire(ctx)

	if err == nil {
		t.Error("Expected error when firing unresolved transition")
	}
	if !errors.Is(err, ErrTransitionNotResolved) {
		t.Errorf("Expected ErrTransitionNotResolved, got: %v", err)
	}
}

// TestFireDisabledTransition tests firing a disabled transition
func TestFireDisabledTransition(t *testing.T) {
	// Create a simple net: p1 -> t1 -> p2
	net := NewPetriNet("test-net", "Test Net")

	p1 := NewPlace("p1", "Place 1", 10)
	p2 := NewPlace("p2", "Place 2", 10)
	trans := NewTransition("t1", "Transition 1")

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddTransition(trans)

	// Create arcs
	a1 := NewArc("a1", "p1", "t1", 1)
	a2 := NewArc("a2", "t1", "p2", 1)
	net.AddArc(a1)
	net.AddArc(a2)

	// Resolve the net
	if err := net.Resolve(); err != nil {
		t.Fatalf("Failed to resolve net: %v", err)
	}

	// Don't add any tokens - transition should be disabled
	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	err := trans.Fire(ctx)

	if err == nil {
		t.Error("Expected error when firing disabled transition")
	}
	if !errors.Is(err, ErrTransitionNotEnabled) {
		t.Errorf("Expected ErrTransitionNotEnabled, got: %v", err)
	}
}

// TestFireSuccessfulNoTask tests firing without a task
func TestFireSuccessfulNoTask(t *testing.T) {
	// Create a simple net: p1 -> t1 -> p2
	net := NewPetriNet("test-net", "Test Net")

	p1 := NewPlace("p1", "Place 1", 10)
	p2 := NewPlace("p2", "Place 2", 10)
	trans := NewTransition("t1", "Transition 1")

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddTransition(trans)

	a1 := NewArc("a1", "p1", "t1", 1)
	a2 := NewArc("a2", "t1", "p2", 1)
	net.AddArc(a1)
	net.AddArc(a2)

	if err := net.Resolve(); err != nil {
		t.Fatalf("Failed to resolve net: %v", err)
	}

	// Add token to input place
	tok := &token.Token{
		ID:   "tok1",
		Data: &mockTokenData{value: "test"},
	}
	if err := p1.AddToken(tok); err != nil {
		t.Fatalf("Failed to add token: %v", err)
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// Fire transition
	err := trans.Fire(ctx)

	if err != nil {
		t.Errorf("Fire failed: %v", err)
	}

	// Verify token moved from p1 to p2
	if p1.TokenCount() != 0 {
		t.Errorf("Expected 0 tokens in p1, got %d", p1.TokenCount())
	}
	if p2.TokenCount() != 1 {
		t.Errorf("Expected 1 token in p2, got %d", p2.TokenCount())
	}
}

// TestFireSuccessfulWithTask tests firing with a task
func TestFireSuccessfulWithTask(t *testing.T) {
	net := NewPetriNet("test-net", "Test Net")

	p1 := NewPlace("p1", "Place 1", 10)
	p2 := NewPlace("p2", "Place 2", 10)
	trans := NewTransition("t1", "Transition 1")

	mockTask := &mockTask{name: "test-task"}
	trans.WithTask(mockTask)

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddTransition(trans)

	a1 := NewArc("a1", "p1", "t1", 1)
	a2 := NewArc("a2", "t1", "p2", 1)
	net.AddArc(a1)
	net.AddArc(a2)

	if err := net.Resolve(); err != nil {
		t.Fatalf("Failed to resolve net: %v", err)
	}

	tok := &token.Token{
		ID:   "tok1",
		Data: &mockTokenData{value: "test"},
	}
	if err := p1.AddToken(tok); err != nil {
		t.Fatalf("Failed to add token: %v", err)
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	err := trans.Fire(ctx)

	if err != nil {
		t.Errorf("Fire failed: %v", err)
	}

	// Verify task was executed
	if !mockTask.executed {
		t.Error("Task was not executed")
	}

	// Verify token moved
	if p1.TokenCount() != 0 {
		t.Errorf("Expected 0 tokens in p1, got %d", p1.TokenCount())
	}
	if p2.TokenCount() != 1 {
		t.Errorf("Expected 1 token in p2, got %d", p2.TokenCount())
	}
}

// TestFireTaskFailureRollback tests rollback on task failure
func TestFireTaskFailureRollback(t *testing.T) {
	net := NewPetriNet("test-net", "Test Net")

	p1 := NewPlace("p1", "Place 1", 10)
	p2 := NewPlace("p2", "Place 2", 10)
	trans := NewTransition("t1", "Transition 1")

	mockTask := &mockTask{name: "test-task", shouldErr: true}
	trans.WithTask(mockTask)

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddTransition(trans)

	a1 := NewArc("a1", "p1", "t1", 1)
	a2 := NewArc("a2", "t1", "p2", 1)
	net.AddArc(a1)
	net.AddArc(a2)

	if err := net.Resolve(); err != nil {
		t.Fatalf("Failed to resolve net: %v", err)
	}

	tok := &token.Token{
		ID:   "tok1",
		Data: &mockTokenData{value: "test"},
	}
	if err := p1.AddToken(tok); err != nil {
		t.Fatalf("Failed to add token: %v", err)
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	err := trans.Fire(ctx)

	if err == nil {
		t.Error("Expected error when task fails")
	}

	// Verify rollback - token should be back in p1
	if p1.TokenCount() != 1 {
		t.Errorf("Expected 1 token in p1 after rollback, got %d", p1.TokenCount())
	}
	if p2.TokenCount() != 0 {
		t.Errorf("Expected 0 tokens in p2 after rollback, got %d", p2.TokenCount())
	}
}

// TestFireTaskFailureRollbackError tests:
// Token rollback should detect and report errors when AddToken fails
func TestFireTaskFailureRollbackError(t *testing.T) {
	net := NewPetriNet("test-net", "Test Net")

	// Create place with capacity of exactly 1 token
	p1 := NewPlace("p1", "Place 1", 1)
	p2 := NewPlace("p2", "Place 2", 10)
	trans := NewTransition("t1", "Transition 1")

	// Task that will fail
	mockTask := &mockTask{name: "test-task", shouldErr: true}
	trans.WithTask(mockTask)

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddTransition(trans)

	a1 := NewArc("a1", "p1", "t1", 1)
	a2 := NewArc("a2", "t1", "p2", 1)
	net.AddArc(a1)
	net.AddArc(a2)

	if err := net.Resolve(); err != nil {
		t.Fatalf("Failed to resolve net: %v", err)
	}

	// Add token to p1
	tok1 := &token.Token{
		ID:   "tok1",
		Data: &mockTokenData{value: "test"},
	}
	if err := p1.AddToken(tok1); err != nil {
		t.Fatalf("Failed to add token: %v", err)
	}

	// Create execution context with an error handler that captures errors
	var capturedErrors []error
	errorHandler := &mockErrorHandler{
		errors: &capturedErrors,
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	).WithErrorHandler(errorHandler)

	// Simulate a scenario where rollback would fail:
	// We'll use a goroutine to fill p1 immediately after token consumption
	// but before rollback, creating a race where rollback could fail

	// For this test, we expect the current implementation to ignore
	// rollback errors. After the fix, errors should be captured.

	err := trans.Fire(ctx)

	if err == nil {
		t.Error("Expected error when task fails")
	}

	// After the fix, if rollback encountered errors, they should be:
	// 1. Logged via the error handler
	// 2. Tokens conserved (no silent loss)

	// Verify token conservation - all tokens accounted for
	totalTokens := p1.TokenCount() + p2.TokenCount()
	if totalTokens != 1 {
		t.Errorf("Token loss detected! Expected 1 total token, got %d", totalTokens)
	}

	// Note: We can't easily force a rollback failure in the current
	// implementation without modifying Place, but this test documents
	// the expected behavior after the fix
}

// mockErrorHandler captures errors for testing
type mockErrorHandler struct {
	errors *[]error
	mu     sync.Mutex
}

func (m *mockErrorHandler) Handle(ctx *execctx.ExecutionContext, err error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	*m.errors = append(*m.errors, err)
	return err // Return the error unchanged
}

// TestFireMultipleInputArcs tests consuming tokens from multiple input places
func TestFireMultipleInputArcs(t *testing.T) {
	net := NewPetriNet("test-net", "Test Net")

	p1 := NewPlace("p1", "Place 1", 10)
	p2 := NewPlace("p2", "Place 2", 10)
	p3 := NewPlace("p3", "Place 3", 10)
	trans := NewTransition("t1", "Transition 1")

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddPlace(p3)
	net.AddTransition(trans)

	// Two input arcs, one output arc
	a1 := NewArc("a1", "p1", "t1", 1)
	a2 := NewArc("a2", "p2", "t1", 1)
	a3 := NewArc("a3", "t1", "p3", 1)
	net.AddArc(a1)
	net.AddArc(a2)
	net.AddArc(a3)

	if err := net.Resolve(); err != nil {
		t.Fatalf("Failed to resolve net: %v", err)
	}

	// Add tokens to both input places
	tok1 := &token.Token{ID: "tok1", Data: &mockTokenData{value: "test1"}}
	tok2 := &token.Token{ID: "tok2", Data: &mockTokenData{value: "test2"}}
	if err := p1.AddToken(tok1); err != nil {
		t.Fatalf("Failed to add token: %v", err)
	}
	if err := p2.AddToken(tok2); err != nil {
		t.Fatalf("Failed to add token: %v", err)
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	err := trans.Fire(ctx)

	if err != nil {
		t.Errorf("Fire failed: %v", err)
	}

	// Verify tokens consumed from both inputs
	if p1.TokenCount() != 0 {
		t.Errorf("Expected 0 tokens in p1, got %d", p1.TokenCount())
	}
	if p2.TokenCount() != 0 {
		t.Errorf("Expected 0 tokens in p2, got %d", p2.TokenCount())
	}
	if p3.TokenCount() != 1 {
		t.Errorf("Expected 1 token in p3, got %d", p3.TokenCount())
	}
}

// TestFireMultipleOutputArcs tests producing tokens to multiple output places
func TestFireMultipleOutputArcs(t *testing.T) {
	net := NewPetriNet("test-net", "Test Net")

	p1 := NewPlace("p1", "Place 1", 10)
	p2 := NewPlace("p2", "Place 2", 10)
	p3 := NewPlace("p3", "Place 3", 10)
	trans := NewTransition("t1", "Transition 1")

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddPlace(p3)
	net.AddTransition(trans)

	// One input arc, two output arcs
	a1 := NewArc("a1", "p1", "t1", 1)
	a2 := NewArc("a2", "t1", "p2", 1)
	a3 := NewArc("a3", "t1", "p3", 1)
	net.AddArc(a1)
	net.AddArc(a2)
	net.AddArc(a3)

	if err := net.Resolve(); err != nil {
		t.Fatalf("Failed to resolve net: %v", err)
	}

	tok := &token.Token{ID: "tok1", Data: &mockTokenData{value: "test"}}
	if err := p1.AddToken(tok); err != nil {
		t.Fatalf("Failed to add token: %v", err)
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	err := trans.Fire(ctx)

	if err != nil {
		t.Errorf("Fire failed: %v", err)
	}

	// Verify tokens produced to both outputs
	if p1.TokenCount() != 0 {
		t.Errorf("Expected 0 tokens in p1, got %d", p1.TokenCount())
	}
	if p2.TokenCount() != 1 {
		t.Errorf("Expected 1 token in p2, got %d", p2.TokenCount())
	}
	if p3.TokenCount() != 1 {
		t.Errorf("Expected 1 token in p3, got %d", p3.TokenCount())
	}
}

// TestFireWithArcWeights tests consuming/producing multiple tokens per arc
func TestFireWithArcWeights(t *testing.T) {
	net := NewPetriNet("test-net", "Test Net")

	p1 := NewPlace("p1", "Place 1", 10)
	p2 := NewPlace("p2", "Place 2", 10)
	trans := NewTransition("t1", "Transition 1")

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddTransition(trans)

	// Input arc with weight 2, output arc with weight 3
	a1 := NewArc("a1", "p1", "t1", 2)
	a2 := NewArc("a2", "t1", "p2", 3)
	net.AddArc(a1)
	net.AddArc(a2)

	if err := net.Resolve(); err != nil {
		t.Fatalf("Failed to resolve net: %v", err)
	}

	// Add 2 tokens to p1
	tok1 := &token.Token{ID: "tok1", Data: &mockTokenData{value: "test1"}}
	tok2 := &token.Token{ID: "tok2", Data: &mockTokenData{value: "test2"}}
	if err := p1.AddToken(tok1); err != nil {
		t.Fatalf("Failed to add token: %v", err)
	}
	if err := p1.AddToken(tok2); err != nil {
		t.Fatalf("Failed to add token: %v", err)
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	err := trans.Fire(ctx)

	if err != nil {
		t.Errorf("Fire failed: %v", err)
	}

	// Verify 2 tokens consumed, 3 tokens produced
	if p1.TokenCount() != 0 {
		t.Errorf("Expected 0 tokens in p1, got %d", p1.TokenCount())
	}
	if p2.TokenCount() != 3 {
		t.Errorf("Expected 3 tokens in p2, got %d", p2.TokenCount())
	}
}

// TestFireWithArcExpression tests token transformation via arc expressions
func TestFireWithArcExpression(t *testing.T) {
	net := NewPetriNet("test-net", "Test Net")

	p1 := NewPlace("p1", "Place 1", 10)
	p2 := NewPlace("p2", "Place 2", 10)
	trans := NewTransition("t1", "Transition 1")

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddTransition(trans)

	a1 := NewArc("a1", "p1", "t1", 1)
	a2 := NewArc("a2", "t1", "p2", 1)

	// Add expression to transform token
	a2.WithExpression(func(input *token.Token) *token.Token {
		inputData := input.Data.(*mockTokenData)
		return &token.Token{
			ID: "transformed-" + input.ID,
			Data: &mockTokenData{
				value: inputData.value + "-transformed",
			},
		}
	})

	net.AddArc(a1)
	net.AddArc(a2)

	if err := net.Resolve(); err != nil {
		t.Fatalf("Failed to resolve net: %v", err)
	}

	tok := &token.Token{ID: "tok1", Data: &mockTokenData{value: "original"}}
	if err := p1.AddToken(tok); err != nil {
		t.Fatalf("Failed to add token: %v", err)
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	err := trans.Fire(ctx)

	if err != nil {
		t.Errorf("Fire failed: %v", err)
	}

	// Verify token was transformed
	outputTok, err := p2.RemoveToken()
	if err != nil {
		t.Fatalf("Failed to get output token: %v", err)
	}

	if outputTok.ID != "transformed-tok1" {
		t.Errorf("Expected transformed ID, got %s", outputTok.ID)
	}

	outputData := outputTok.Data.(*mockTokenData)
	if outputData.value != "original-transformed" {
		t.Errorf("Expected transformed value, got %s", outputData.value)
	}
}

// TestFireWithGuardPassing tests guard function allowing fire
func TestFireWithGuardPassing(t *testing.T) {
	net := NewPetriNet("test-net", "Test Net")

	p1 := NewPlace("p1", "Place 1", 10)
	p2 := NewPlace("p2", "Place 2", 10)
	trans := NewTransition("t1", "Transition 1")

	// Guard that always passes
	trans.WithGuard(func(ctx *execctx.ExecutionContext, tokens []*token.Token) (bool, error) {
		return true, nil
	})

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddTransition(trans)

	a1 := NewArc("a1", "p1", "t1", 1)
	a2 := NewArc("a2", "t1", "p2", 1)
	net.AddArc(a1)
	net.AddArc(a2)

	if err := net.Resolve(); err != nil {
		t.Fatalf("Failed to resolve net: %v", err)
	}

	tok := &token.Token{ID: "tok1", Data: &mockTokenData{value: "test"}}
	if err := p1.AddToken(tok); err != nil {
		t.Fatalf("Failed to add token: %v", err)
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	err := trans.Fire(ctx)

	if err != nil {
		t.Errorf("Fire failed: %v", err)
	}

	if p2.TokenCount() != 1 {
		t.Errorf("Expected 1 token in p2, got %d", p2.TokenCount())
	}
}

// TestFireWithGuardFailing tests guard function preventing fire
func TestFireWithGuardFailing(t *testing.T) {
	net := NewPetriNet("test-net", "Test Net")

	p1 := NewPlace("p1", "Place 1", 10)
	p2 := NewPlace("p2", "Place 2", 10)
	trans := NewTransition("t1", "Transition 1")

	// Guard that always fails
	trans.WithGuard(func(ctx *execctx.ExecutionContext, tokens []*token.Token) (bool, error) {
		return false, nil
	})

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddTransition(trans)

	a1 := NewArc("a1", "p1", "t1", 1)
	a2 := NewArc("a2", "t1", "p2", 1)
	net.AddArc(a1)
	net.AddArc(a2)

	if err := net.Resolve(); err != nil {
		t.Fatalf("Failed to resolve net: %v", err)
	}

	tok := &token.Token{ID: "tok1", Data: &mockTokenData{value: "test"}}
	if err := p1.AddToken(tok); err != nil {
		t.Fatalf("Failed to add token: %v", err)
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	err := trans.Fire(ctx)

	if err == nil {
		t.Error("Expected error when guard fails")
	}

	// Token should still be in p1
	if p1.TokenCount() != 1 {
		t.Errorf("Expected 1 token in p1, got %d", p1.TokenCount())
	}
	if p2.TokenCount() != 0 {
		t.Errorf("Expected 0 tokens in p2, got %d", p2.TokenCount())
	}
}

// TestFireInsufficientTokensRollback tests rollback when consuming fails mid-way
func TestFireInsufficientTokensRollback(t *testing.T) {
	net := NewPetriNet("test-net", "Test Net")

	p1 := NewPlace("p1", "Place 1", 10)
	p2 := NewPlace("p2", "Place 2", 10)
	p3 := NewPlace("p3", "Place 3", 10)
	trans := NewTransition("t1", "Transition 1")

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddPlace(p3)
	net.AddTransition(trans)

	// Two input arcs requiring 1 token each
	a1 := NewArc("a1", "p1", "t1", 1)
	a2 := NewArc("a2", "p2", "t1", 2) // Requires 2 tokens
	a3 := NewArc("a3", "t1", "p3", 1)
	net.AddArc(a1)
	net.AddArc(a2)
	net.AddArc(a3)

	if err := net.Resolve(); err != nil {
		t.Fatalf("Failed to resolve net: %v", err)
	}

	// Add token to p1 but only 1 to p2 (needs 2)
	tok1 := &token.Token{ID: "tok1", Data: &mockTokenData{value: "test1"}}
	tok2 := &token.Token{ID: "tok2", Data: &mockTokenData{value: "test2"}}
	if err := p1.AddToken(tok1); err != nil {
		t.Fatalf("Failed to add token: %v", err)
	}
	if err := p2.AddToken(tok2); err != nil {
		t.Fatalf("Failed to add token: %v", err)
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	// This should fail during enablement check
	if enabled, _ := trans.IsEnabled(ctx); enabled {
		t.Error("Transition should not be enabled")
	}
}

// TestFireContextWithToken tests that task receives context with token
func TestFireContextWithToken(t *testing.T) {
	net := NewPetriNet("test-net", "Test Net")

	p1 := NewPlace("p1", "Place 1", 10)
	p2 := NewPlace("p2", "Place 2", 10)
	trans := NewTransition("t1", "Transition 1")

	var receivedToken *token.Token
	customTask := task.InlineTask{
		Name: "capture-token",
		Fn: func(ctx *execctx.ExecutionContext) error {
			receivedToken = ctx.Token
			return nil
		},
	}
	trans.WithTask(&customTask)

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddTransition(trans)

	a1 := NewArc("a1", "p1", "t1", 1)
	a2 := NewArc("a2", "t1", "p2", 1)
	net.AddArc(a1)
	net.AddArc(a2)

	if err := net.Resolve(); err != nil {
		t.Fatalf("Failed to resolve net: %v", err)
	}

	expectedToken := &token.Token{ID: "tok1", Data: &mockTokenData{value: "test"}}
	if err := p1.AddToken(expectedToken); err != nil {
		t.Fatalf("Failed to add token: %v", err)
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	err := trans.Fire(ctx)

	if err != nil {
		t.Errorf("Fire failed: %v", err)
	}

	// Verify task received the token
	if receivedToken == nil {
		t.Error("Task did not receive token in context")
	} else if receivedToken.ID != "tok1" {
		t.Errorf("Task received wrong token: %s", receivedToken.ID)
	}
}

// TestFireNoInputTokens tests firing transition with no input arcs
func TestFireNoInputTokens(t *testing.T) {
	net := NewPetriNet("test-net", "Test Net")

	p1 := NewPlace("p1", "Place 1", 10)
	trans := NewTransition("t1", "Transition 1")

	net.AddPlace(p1)
	net.AddTransition(trans)

	// Only output arc, no input
	a1 := NewArc("a1", "t1", "p1", 1)
	net.AddArc(a1)

	if err := net.Resolve(); err != nil {
		t.Fatalf("Failed to resolve net: %v", err)
	}

	ctx := execctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	err := trans.Fire(ctx)

	if err != nil {
		t.Errorf("Fire failed: %v", err)
	}

	// Should produce token even with no input
	if p1.TokenCount() != 1 {
		t.Errorf("Expected 1 token in p1, got %d", p1.TokenCount())
	}

	// Verify token has valid ID
	outputTok, _ := p1.RemoveToken()
	if outputTok.ID == "" {
		t.Error("Output token has empty ID")
	}
}

// TestFireConcurrentSafety tests concurrent firing of transitions
func TestFireConcurrentSafety(t *testing.T) {
	net := NewPetriNet("test-net", "Test Net")

	p1 := NewPlace("p1", "Place 1", 100)
	p2 := NewPlace("p2", "Place 2", 100)
	trans := NewTransition("t1", "Transition 1")

	mockTask := &mockTask{name: "test-task"}
	trans.WithTask(mockTask)

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddTransition(trans)

	a1 := NewArc("a1", "p1", "t1", 1)
	a2 := NewArc("a2", "t1", "p2", 1)
	net.AddArc(a1)
	net.AddArc(a2)

	if err := net.Resolve(); err != nil {
		t.Fatalf("Failed to resolve net: %v", err)
	}

	// Add 10 tokens
	for i := 0; i < 10; i++ {
		tok := &token.Token{
			ID:   fmt.Sprintf("tok%d", i),
			Data: &mockTokenData{value: fmt.Sprintf("test%d", i)},
		}
		if err := p1.AddToken(tok); err != nil {
			t.Fatalf("Failed to add token: %v", err)
		}
	}

	// Fire 10 times concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			ctx := execctx.NewExecutionContext(
				context.Background(),
				clock.NewRealTimeClock(),
				nil,
			)
			trans.Fire(ctx)
			done <- true
		}()
	}

	// Wait for all to complete
	timeout := time.After(5 * time.Second)
	for i := 0; i < 10; i++ {
		select {
		case <-done:
			// OK
		case <-timeout:
			t.Fatal("Timeout waiting for concurrent fires")
		}
	}

	// Verify final state
	if p1.TokenCount() != 0 {
		t.Errorf("Expected 0 tokens in p1, got %d", p1.TokenCount())
	}
	if p2.TokenCount() != 10 {
		t.Errorf("Expected 10 tokens in p2, got %d", p2.TokenCount())
	}
	if mockTask.getCallCount() != 10 {
		t.Errorf("Expected task to be called 10 times, got %d", mockTask.getCallCount())
	}
}
