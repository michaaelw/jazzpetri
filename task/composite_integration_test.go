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
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	execCtx "github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/clock"
	"github.com/jazzpetri/engine/token"
)

// TestComposite_SequentialWithParallel tests sequence containing parallel tasks
func TestComposite_SequentialWithParallel(t *testing.T) {
	// Arrange - Fan-out/fan-in pattern
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test-token", Data: &token.IntToken{Value: 0}}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	var count int32

	// Sequential: setup -> parallel processing -> finalize
	workflow := &SequentialTask{
		Name: "fanout-fanin",
		Tasks: []Task{
			&InlineTask{
				Name: "setup",
				Fn: func(ctx *execCtx.ExecutionContext) error {
					ctx.Token.Data.(*token.IntToken).Value = 10
					return nil
				},
			},
			&ParallelTask{
				Name: "parallel-processing",
				Tasks: []Task{
					&InlineTask{Name: "worker1", Fn: func(ctx *execCtx.ExecutionContext) error {
						atomic.AddInt32(&count, 1)
						return nil
					}},
					&InlineTask{Name: "worker2", Fn: func(ctx *execCtx.ExecutionContext) error {
						atomic.AddInt32(&count, 1)
						return nil
					}},
					&InlineTask{Name: "worker3", Fn: func(ctx *execCtx.ExecutionContext) error {
						atomic.AddInt32(&count, 1)
						return nil
					}},
				},
			},
			&InlineTask{
				Name: "finalize",
				Fn: func(ctx *execCtx.ExecutionContext) error {
					ctx.Token.Data.(*token.IntToken).Value += 100
					return nil
				},
			},
		},
	}

	// Act
	err := workflow.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if atomic.LoadInt32(&count) != 3 {
		t.Errorf("Expected 3 parallel tasks executed, got %d", count)
	}
	if tok.Data.(*token.IntToken).Value != 110 {
		t.Errorf("Expected token value 110, got %d", tok.Data.(*token.IntToken).Value)
	}
}

// TestComposite_ConditionalWithRetry tests conditional branching with retry logic
func TestComposite_ConditionalWithRetry(t *testing.T) {
	// Arrange
	clk := clock.NewRealTimeClock()
	tok := &token.Token{ID: "test-token", Data: &token.IntToken{Value: 10}}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	var attempts int32

	workflow := &ConditionalTask{
		Name: "high-priority-with-retry",
		Condition: func(ctx *execCtx.ExecutionContext) bool {
			return ctx.Token.Data.(*token.IntToken).Value > 5
		},
		ThenTask: &RetryTask{
			Name: "retry-high-priority",
			Task: &InlineTask{
				Name: "high-priority-task",
				Fn: func(ctx *execCtx.ExecutionContext) error {
					count := atomic.AddInt32(&attempts, 1)
					if count < 3 {
						return errors.New("temporary failure")
					}
					return nil
				},
			},
			MaxAttempts: 5,
			Backoff:     1 * time.Millisecond,
		},
		ElseTask: &InlineTask{
			Name: "low-priority-task",
			Fn:   func(ctx *execCtx.ExecutionContext) error { return nil },
		},
	}

	// Act
	err := workflow.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

// TestComposite_SagaPattern tests distributed saga with recovery
func TestComposite_SagaPattern(t *testing.T) {
	// Arrange - Simulate distributed transaction with compensations
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "order-123"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	operations := []string{}
	var mu sync.Mutex

	saga := &SequentialTask{
		Name: "order-saga",
		Tasks: []Task{
			// Step 1: Reserve inventory
			&RecoveryTask{
				Name: "reserve-inventory",
				Task: &InlineTask{
					Name: "reserve",
					Fn: func(ctx *execCtx.ExecutionContext) error {
						mu.Lock()
						operations = append(operations, "reserve-inventory")
						mu.Unlock()
						return nil
					},
				},
				CompensationTask: &InlineTask{
					Name: "release",
					Fn: func(ctx *execCtx.ExecutionContext) error {
						mu.Lock()
						operations = append(operations, "release-inventory")
						mu.Unlock()
						return nil
					},
				},
			},
			// Step 2: Charge payment
			&RecoveryTask{
				Name: "charge-payment",
				Task: &InlineTask{
					Name: "charge",
					Fn: func(ctx *execCtx.ExecutionContext) error {
						mu.Lock()
						operations = append(operations, "charge-payment")
						mu.Unlock()
						return nil
					},
				},
				CompensationTask: &InlineTask{
					Name: "refund",
					Fn: func(ctx *execCtx.ExecutionContext) error {
						mu.Lock()
						operations = append(operations, "refund-payment")
						mu.Unlock()
						return nil
					},
				},
			},
			// Step 3: Notify customer
			&InlineTask{
				Name: "notify",
				Fn: func(ctx *execCtx.ExecutionContext) error {
					mu.Lock()
					operations = append(operations, "notify-customer")
					mu.Unlock()
					return nil
				},
			},
		},
	}

	// Act
	err := saga.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	expected := []string{"reserve-inventory", "charge-payment", "notify-customer"}
	if len(operations) != len(expected) {
		t.Errorf("Expected %d operations, got %d: %v", len(expected), len(operations), operations)
	}
	for i, op := range expected {
		if operations[i] != op {
			t.Errorf("Expected operation %d to be %s, got %s", i, op, operations[i])
		}
	}
}

// TestComposite_ParallelWithNested tests deeply nested parallel and sequential tasks
func TestComposite_ParallelWithNested(t *testing.T) {
	// Arrange - Complex nested structure
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test-token"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	var count int32

	workflow := &ParallelTask{
		Name: "parallel-with-sequences",
		Tasks: []Task{
			&SequentialTask{
				Name: "sequence-1",
				Tasks: []Task{
					&InlineTask{Name: "s1-t1", Fn: func(ctx *execCtx.ExecutionContext) error {
						atomic.AddInt32(&count, 1)
						return nil
					}},
					&InlineTask{Name: "s1-t2", Fn: func(ctx *execCtx.ExecutionContext) error {
						atomic.AddInt32(&count, 1)
						return nil
					}},
				},
			},
			&SequentialTask{
				Name: "sequence-2",
				Tasks: []Task{
					&InlineTask{Name: "s2-t1", Fn: func(ctx *execCtx.ExecutionContext) error {
						atomic.AddInt32(&count, 1)
						return nil
					}},
					&InlineTask{Name: "s2-t2", Fn: func(ctx *execCtx.ExecutionContext) error {
						atomic.AddInt32(&count, 1)
						return nil
					}},
				},
			},
			&InlineTask{Name: "direct", Fn: func(ctx *execCtx.ExecutionContext) error {
				atomic.AddInt32(&count, 1)
				return nil
			}},
		},
	}

	// Act
	err := workflow.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if atomic.LoadInt32(&count) != 5 {
		t.Errorf("Expected 5 tasks executed, got %d", count)
	}
}

// TestComposite_ComplexWorkflow tests a realistic complex workflow
func TestComposite_ComplexWorkflow(t *testing.T) {
	// Arrange - Realistic order processing workflow
	clk := clock.NewRealTimeClock()
	tok := &token.Token{ID: "order-456", Data: &token.MapToken{
		Value: map[string]interface{}{
			"priority":  "high",
			"total":     150.00,
			"inventory": true,
		},
	}}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	executed := []string{}
	var mu sync.Mutex

	workflow := &SequentialTask{
		Name: "order-processing",
		Tasks: []Task{
			// Validate order
			&InlineTask{
				Name: "validate",
				Fn: func(ctx *execCtx.ExecutionContext) error {
					mu.Lock()
					executed = append(executed, "validate")
					mu.Unlock()
					return nil
				},
			},
			// Conditional routing based on priority
			&ConditionalTask{
				Name: "route-by-priority",
				Condition: func(ctx *execCtx.ExecutionContext) bool {
					m := ctx.Token.Data.(*token.MapToken)
					return m.Value["priority"] == "high"
				},
				ThenTask: &SequentialTask{
					Name: "high-priority-flow",
					Tasks: []Task{
						// Parallel checks for high priority
						&ParallelTask{
							Name: "parallel-checks",
							Tasks: []Task{
								&InlineTask{Name: "check-inventory", Fn: func(ctx *execCtx.ExecutionContext) error {
									mu.Lock()
									executed = append(executed, "check-inventory")
									mu.Unlock()
									return nil
								}},
								&InlineTask{Name: "check-credit", Fn: func(ctx *execCtx.ExecutionContext) error {
									mu.Lock()
									executed = append(executed, "check-credit")
									mu.Unlock()
									return nil
								}},
							},
						},
						// Retry payment with recovery
						&RecoveryTask{
							Name: "payment-with-recovery",
							Task: &RetryTask{
								Name: "charge",
								Task: &InlineTask{
									Name: "charge-card",
									Fn: func(ctx *execCtx.ExecutionContext) error {
										mu.Lock()
										executed = append(executed, "charge-card")
										mu.Unlock()
										return nil
									},
								},
								MaxAttempts: 3,
								Backoff:     1 * time.Millisecond,
							},
							CompensationTask: &InlineTask{
								Name: "notify-failure",
								Fn: func(ctx *execCtx.ExecutionContext) error {
									mu.Lock()
									executed = append(executed, "notify-failure")
									mu.Unlock()
									return nil
								},
							},
						},
					},
				},
				ElseTask: &InlineTask{
					Name: "standard-processing",
					Fn: func(ctx *execCtx.ExecutionContext) error {
						mu.Lock()
						executed = append(executed, "standard-processing")
						mu.Unlock()
						return nil
					},
				},
			},
			// Final notification
			&InlineTask{
				Name: "notify",
				Fn: func(ctx *execCtx.ExecutionContext) error {
					mu.Lock()
					executed = append(executed, "notify")
					mu.Unlock()
					return nil
				},
			},
		},
	}

	// Act
	err := workflow.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// Verify high-priority flow was executed
	if len(executed) < 5 {
		t.Errorf("Expected at least 5 operations, got %d: %v", len(executed), executed)
	}

	// Verify validate and notify are always present
	if executed[0] != "validate" {
		t.Errorf("Expected first operation to be 'validate', got %s", executed[0])
	}
	if executed[len(executed)-1] != "notify" {
		t.Errorf("Expected last operation to be 'notify', got %s", executed[len(executed)-1])
	}
}

// TestComposite_ErrorPropagation tests error propagation through nested composites
func TestComposite_ErrorPropagation(t *testing.T) {
	// Arrange
	clk := clock.NewVirtualClock(time.Now())
	tok := &token.Token{ID: "test-token"}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	expectedErr := errors.New("deep error")

	workflow := &SequentialTask{
		Name: "outer",
		Tasks: []Task{
			&InlineTask{Name: "task1", Fn: func(ctx *execCtx.ExecutionContext) error { return nil }},
			&ParallelTask{
				Name: "parallel",
				Tasks: []Task{
					&SequentialTask{
						Name: "inner-seq",
						Tasks: []Task{
							&InlineTask{Name: "deep-task", Fn: func(ctx *execCtx.ExecutionContext) error {
								return expectedErr
							}},
						},
					},
				},
			},
			&InlineTask{Name: "task3", Fn: func(ctx *execCtx.ExecutionContext) error { return nil }},
		},
	}

	// Act
	err := workflow.Execute(ctx)

	// Assert
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// Error should be wrapped in MultiError by ParallelTask
	parallelResult, ok := err.(*ParallelResult)
	if !ok {
		t.Fatalf("Expected *ParallelResult, got %T", err)
	}

	if parallelResult.FailureCount != 1 {
		t.Errorf("Expected 1 error in ParallelResult, got %d", parallelResult.FailureCount)
	}

	// Check that the failure is the expected error
	foundExpectedError := false
	for _, r := range parallelResult.Results {
		if !r.Success && r.Error == expectedErr {
			foundExpectedError = true
			break
		}
	}
	if !foundExpectedError {
		t.Errorf("Expected error %v not found in results", expectedErr)
	}
}

// TestComposite_AllPatternsTogether tests all composite patterns in one workflow
func TestComposite_AllPatternsTogether(t *testing.T) {
	// Arrange - Kitchen sink workflow
	clk := clock.NewRealTimeClock()
	tok := &token.Token{ID: "test-token", Data: &token.IntToken{Value: 7}}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	var count int32

	workflow := &SequentialTask{
		Name: "kitchen-sink",
		Tasks: []Task{
			// Parallel tasks
			&ParallelTask{
				Name: "parallel-phase",
				Tasks: []Task{
					&InlineTask{Name: "p1", Fn: func(ctx *execCtx.ExecutionContext) error {
						atomic.AddInt32(&count, 1)
						return nil
					}},
					&InlineTask{Name: "p2", Fn: func(ctx *execCtx.ExecutionContext) error {
						atomic.AddInt32(&count, 1)
						return nil
					}},
				},
			},
			// Conditional with retry
			&ConditionalTask{
				Name: "conditional-phase",
				Condition: func(ctx *execCtx.ExecutionContext) bool {
					return atomic.LoadInt32(&count) > 0
				},
				ThenTask: &RetryTask{
					Name: "retry-phase",
					Task: &InlineTask{
						Name: "flaky-task",
						Fn: func(ctx *execCtx.ExecutionContext) error {
							if atomic.AddInt32(&count, 1) < 5 {
								return errors.New("not yet")
							}
							return nil
						},
					},
					MaxAttempts: 5,
					Backoff:     1 * time.Millisecond,
				},
			},
			// Recovery with nested sequence
			&RecoveryTask{
				Name: "recovery-phase",
				Task: &SequentialTask{
					Name: "nested-seq",
					Tasks: []Task{
						&InlineTask{Name: "s1", Fn: func(ctx *execCtx.ExecutionContext) error {
							atomic.AddInt32(&count, 1)
							return nil
						}},
						&InlineTask{Name: "s2", Fn: func(ctx *execCtx.ExecutionContext) error {
							atomic.AddInt32(&count, 1)
							return nil
						}},
					},
				},
				CompensationTask: &InlineTask{
					Name: "compensation",
					Fn:   func(ctx *execCtx.ExecutionContext) error { return nil },
				},
			},
		},
	}

	// Act
	err := workflow.Execute(ctx)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	finalCount := atomic.LoadInt32(&count)
	if finalCount < 7 {
		t.Errorf("Expected at least 7 operations, got %d", finalCount)
	}
}
