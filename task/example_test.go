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

package task_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/jazzpetri/engine/clock"
	execCtx "github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/task"
	"github.com/jazzpetri/engine/token"
)

// OrderData represents an order in the workflow
type OrderData struct {
	ID     string
	Total  float64
	Status string
}

func (o *OrderData) Type() string { return "order" }

// Example_inlineTask demonstrates using InlineTask for data processing
func Example_inlineTask() {
	// Create execution context
	clk := clock.NewRealTimeClock()
	tok := &token.Token{
		ID:   "order-123",
		Data: &OrderData{ID: "order-123", Total: 0, Status: "new"},
	}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	// Create inline task
	calculateTask := &task.InlineTask{
		Name: "calculate-total",
		Fn: func(ctx *execCtx.ExecutionContext) error {
			order := ctx.Token.Data.(*OrderData)
			order.Total = 99.99
			order.Status = "calculated"
			fmt.Printf("Calculated total: $%.2f\n", order.Total)
			return nil
		},
	}

	// Execute task
	if err := calculateTask.Execute(ctx); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("Task completed successfully")
	// Output:
	// Calculated total: $99.99
	// Task completed successfully
}

// Example_httpTask demonstrates using HTTPTask for external service calls
func Example_httpTask() {
	// Create mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Received webhook notification")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"received"}`))
	}))
	defer server.Close()

	// Create execution context
	clk := clock.NewRealTimeClock()
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	// Create HTTP task
	webhookTask := &task.HTTPTask{
		Name:   "notify-webhook",
		Method: "POST",
		URL:    server.URL,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: []byte(`{"event":"order.created","order_id":"123"}`),
	}

	// Execute task
	if err := webhookTask.Execute(ctx); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("Webhook sent successfully")
	// Output:
	// Received webhook notification
	// Webhook sent successfully
}

// Example_commandTask demonstrates using CommandTask for running scripts
func Example_commandTask() {
	// Create execution context
	clk := clock.NewRealTimeClock()
	ctx := execCtx.NewExecutionContext(context.Background(), clk, nil)

	// Create command task
	echoTask := &task.CommandTask{
		Name:    "echo-message",
		Command: "echo",
		Args:    []string{"Hello from Jazz3 workflow"},
	}

	// Execute task
	if err := echoTask.Execute(ctx); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("Command executed successfully")
	// Output:
	// Command executed successfully
}

// Example_workflowPipeline demonstrates chaining multiple tasks
func Example_workflowPipeline() {
	// Create execution context
	clk := clock.NewRealTimeClock()
	tok := &token.Token{
		ID:   "workflow-1",
		Data: &OrderData{ID: "order-456", Total: 0, Status: "new"},
	}
	ctx := execCtx.NewExecutionContext(context.Background(), clk, tok)

	// Define workflow steps
	tasks := []task.Task{
		&task.InlineTask{
			Name: "validate",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				order := ctx.Token.Data.(*OrderData)
				order.Status = "validated"
				fmt.Println("Step 1: Order validated")
				return nil
			},
		},
		&task.InlineTask{
			Name: "calculate",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				order := ctx.Token.Data.(*OrderData)
				order.Total = 149.99
				order.Status = "calculated"
				fmt.Printf("Step 2: Total calculated: $%.2f\n", order.Total)
				return nil
			},
		},
		&task.InlineTask{
			Name: "complete",
			Fn: func(ctx *execCtx.ExecutionContext) error {
				order := ctx.Token.Data.(*OrderData)
				order.Status = "completed"
				fmt.Println("Step 3: Order completed")
				return nil
			},
		},
	}

	// Execute workflow
	for _, t := range tasks {
		if err := t.Execute(ctx); err != nil {
			fmt.Printf("Workflow failed: %v\n", err)
			return
		}
	}

	order := tok.Data.(*OrderData)
	fmt.Printf("Workflow complete - Status: %s, Total: $%.2f\n", order.Status, order.Total)
	// Output:
	// Step 1: Order validated
	// Step 2: Total calculated: $149.99
	// Step 3: Order completed
	// Workflow complete - Status: completed, Total: $149.99
}
