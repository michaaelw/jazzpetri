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
	"sync"
	"time"

	"github.com/jazzpetri/engine/context"
)

// HumanInteraction is the interface for human-in-the-loop interactions.
// Implementations connect to notification systems, task queues, and approval UIs.
//
// This is the PORT in the Ports & Adapters pattern. Concrete implementations
// (Slack, Email, Web UI, Mobile Push, etc.) are ADAPTERS.
type HumanInteraction interface {
	// RequestApproval sends an approval request to human reviewers.
	// Returns the approval ID for tracking.
	RequestApproval(ctx *context.ExecutionContext, request *ApprovalRequest) (string, error)

	// WaitForDecision blocks until a human makes a decision or timeout occurs.
	// Returns the decision and any data provided by the human.
	WaitForDecision(ctx *context.ExecutionContext, approvalID string, timeout time.Duration) (*HumanDecision, error)

	// RequestDataEntry sends a data entry request to humans.
	// Returns the request ID for tracking.
	RequestDataEntry(ctx *context.ExecutionContext, request *DataEntryRequest) (string, error)

	// WaitForData blocks until a human provides data or timeout occurs.
	// Returns the data provided by the human.
	WaitForData(ctx *context.ExecutionContext, requestID string, timeout time.Duration) (*HumanDataResponse, error)
}

// ApprovalRequest contains information for an approval request.
type ApprovalRequest struct {
	// Title is a short summary of what needs approval.
	Title string

	// Description provides detailed context for the approver.
	Description string

	// RequiredApprovers specifies who can approve (user IDs, group IDs, roles).
	// Empty means anyone can approve.
	RequiredApprovers []string

	// MinApprovals is the minimum number of approvals needed.
	// Defaults to 1.
	MinApprovals int

	// Priority indicates urgency (low, medium, high, critical).
	Priority string

	// Metadata contains additional context for the approval UI.
	Metadata map[string]interface{}

	// DueDate is the deadline for the approval (optional).
	DueDate time.Time
}

// HumanDecision represents the outcome of a human approval.
type HumanDecision struct {
	// Approved indicates whether the request was approved.
	Approved bool

	// DecisionBy identifies who made the decision.
	DecisionBy string

	// DecisionAt is when the decision was made.
	DecisionAt time.Time

	// Comments contains any notes from the approver.
	Comments string

	// Metadata contains additional data from the approval.
	Metadata map[string]interface{}
}

// DataEntryRequest contains information for a data entry request.
type DataEntryRequest struct {
	// Title is a short summary of what data is needed.
	Title string

	// Description provides detailed context for the data entry.
	Description string

	// Fields defines the required data fields with their types and validation.
	Fields []DataField

	// AssignedTo specifies who should provide the data (optional).
	AssignedTo []string

	// Priority indicates urgency.
	Priority string

	// DueDate is the deadline for the data entry (optional).
	DueDate time.Time
}

// DataField defines a single data entry field.
type DataField struct {
	// Name is the field identifier.
	Name string

	// Label is the human-readable field label.
	Label string

	// Type is the field type (text, number, date, select, file, etc.).
	Type string

	// Required indicates if the field must be filled.
	Required bool

	// Default is the default value (optional).
	Default interface{}

	// Options contains valid choices for select fields.
	Options []string

	// Validation contains validation rules (regex, min, max, etc.).
	Validation map[string]interface{}
}

// HumanDataResponse contains data provided by a human.
type HumanDataResponse struct {
	// Data contains the field values provided.
	Data map[string]interface{}

	// ProvidedBy identifies who provided the data.
	ProvidedBy string

	// ProvidedAt is when the data was provided.
	ProvidedAt time.Time

	// Valid indicates if all validation passed.
	Valid bool

	// ValidationErrors contains any validation failures.
	ValidationErrors map[string]string
}

// HumanTask enables human-in-the-loop workflow patterns.
// This includes approval gates, manual review, data entry, and intervention points.
//
// HumanTask is ideal for:
//   - Approval workflows (expense approval, PR reviews, contract signing)
//   - Manual quality gates (human review of AI output)
//   - Data collection (user input, form submission)
//   - Exception handling (human intervention on failures)
//   - Compliance checkpoints (regulatory sign-offs)
//
// Example - Simple approval:
//
//	task := &HumanTask{
//	    Name:        "manager-approval",
//	    Interaction: slackInteraction,
//	    Mode:        HumanModeApproval,
//	    Timeout:     24 * time.Hour,
//	    Request: &ApprovalRequest{
//	        Title:       "Expense Report Approval",
//	        Description: "Review and approve expense report #1234",
//	        Priority:    "medium",
//	    },
//	}
//
// Example - Data entry with dynamic request:
//
//	task := &HumanTask{
//	    Name:        "customer-info",
//	    Interaction: webUIInteraction,
//	    Mode:        HumanModeDataEntry,
//	    Timeout:     1 * time.Hour,
//	    DataRequestFunc: func(ctx *context.ExecutionContext) (*DataEntryRequest, error) {
//	        order := ctx.Token.Data.(*OrderData)
//	        return &DataEntryRequest{
//	            Title:       "Shipping Address Required",
//	            Description: fmt.Sprintf("Please provide shipping address for order %s", order.ID),
//	            Fields: []DataField{
//	                {Name: "street", Label: "Street Address", Type: "text", Required: true},
//	                {Name: "city", Label: "City", Type: "text", Required: true},
//	                {Name: "zip", Label: "ZIP Code", Type: "text", Required: true},
//	            },
//	        }, nil
//	    },
//	    OnDataReceived: func(ctx *context.ExecutionContext, data *HumanDataResponse) error {
//	        // Store the address in token data
//	        order := ctx.Token.Data.(*OrderData)
//	        order.ShippingAddress = data.Data
//	        return nil
//	    },
//	}
//
// Thread Safety:
// HumanTask is safe for concurrent execution. The HumanInteraction implementation
// must also be thread-safe.
type HumanTask struct {
	// Name is the task identifier used in logs, traces, and metrics.
	Name string

	// Interaction is the human interaction backend.
	// This is the adapter that connects to the actual notification/UI system.
	Interaction HumanInteraction

	// Mode determines the type of human interaction.
	Mode HumanMode

	// Timeout is how long to wait for human response.
	// Zero means no timeout (wait indefinitely).
	Timeout time.Duration

	// Request is the static approval request.
	// Ignored if RequestFunc is set.
	Request *ApprovalRequest

	// RequestFunc generates the approval request dynamically.
	// Takes precedence over Request if both are set.
	RequestFunc func(ctx *context.ExecutionContext) (*ApprovalRequest, error)

	// DataRequest is the static data entry request.
	// Ignored if DataRequestFunc is set.
	DataRequest *DataEntryRequest

	// DataRequestFunc generates the data entry request dynamically.
	// Takes precedence over DataRequest if both are set.
	DataRequestFunc func(ctx *context.ExecutionContext) (*DataEntryRequest, error)

	// OnApproved is called when approval is granted (optional).
	OnApproved func(ctx *context.ExecutionContext, decision *HumanDecision) error

	// OnRejected is called when approval is denied (optional).
	OnRejected func(ctx *context.ExecutionContext, decision *HumanDecision) error

	// OnDataReceived is called when data is provided (optional).
	OnDataReceived func(ctx *context.ExecutionContext, data *HumanDataResponse) error

	// OnTimeout is called when timeout occurs (optional).
	// If nil, timeout returns an error.
	OnTimeout func(ctx *context.ExecutionContext) error
}

// HumanMode determines the type of human interaction.
type HumanMode int

const (
	// HumanModeApproval waits for human approval/rejection.
	HumanModeApproval HumanMode = iota

	// HumanModeDataEntry waits for human data input.
	HumanModeDataEntry

	// HumanModeReview requests human review without blocking on decision.
	HumanModeReview
)

// Execute performs the human interaction.
//
// Execution flow for Approval mode:
//  1. Validate task configuration
//  2. Start trace span
//  3. Generate approval request (from RequestFunc or static Request)
//  4. Send request via Interaction.RequestApproval
//  5. Wait for decision via Interaction.WaitForDecision
//  6. Handle decision (OnApproved/OnRejected callbacks)
//  7. Record metrics and complete
//
// Execution flow for DataEntry mode:
//  1. Validate task configuration
//  2. Start trace span
//  3. Generate data request (from DataRequestFunc or static DataRequest)
//  4. Send request via Interaction.RequestDataEntry
//  5. Wait for data via Interaction.WaitForData
//  6. Handle data (OnDataReceived callback)
//  7. Record metrics and complete
//
// Thread Safety:
// Execute is safe to call concurrently from multiple goroutines.
//
// Returns an error if:
//   - Task configuration is invalid (missing Name, Interaction)
//   - Request generation fails
//   - Interaction fails
//   - Timeout occurs (unless OnTimeout handles it)
//   - Approval is rejected (unless OnRejected handles it)
func (h *HumanTask) Execute(ctx *context.ExecutionContext) error {
	// Validate configuration
	if h.Name == "" {
		return fmt.Errorf("human task: name is required")
	}
	if h.Interaction == nil {
		return fmt.Errorf("human task: interaction is required")
	}

	switch h.Mode {
	case HumanModeApproval:
		return h.executeApproval(ctx)
	case HumanModeDataEntry:
		return h.executeDataEntry(ctx)
	case HumanModeReview:
		return h.executeReview(ctx)
	default:
		return fmt.Errorf("human task: unknown mode: %d", h.Mode)
	}
}

func (h *HumanTask) executeApproval(ctx *context.ExecutionContext) error {
	// Start trace span
	span := ctx.Tracer.StartSpan("task.human.approval." + h.Name)
	defer span.End()

	// Record metrics
	ctx.Metrics.Inc("task_executions_total")
	ctx.Metrics.Inc("task_human_executions_total")
	ctx.Metrics.Inc("task_human_approval_requests_total")

	// Log task start
	ctx.Logger.Debug("Executing human approval task", map[string]interface{}{
		"task_name": h.Name,
		"timeout":   h.Timeout.String(),
		"token_id":  tokenID(ctx),
	})

	// Generate approval request
	var request *ApprovalRequest
	var err error

	if h.RequestFunc != nil {
		request, err = h.RequestFunc(ctx)
		if err != nil {
			return h.handleError(ctx, span, fmt.Errorf("approval request generation failed: %w", err))
		}
	} else if h.Request != nil {
		request = h.Request
	} else {
		return h.handleError(ctx, span, fmt.Errorf("human task: approval request is required"))
	}

	span.SetAttribute("approval.title", request.Title)
	span.SetAttribute("approval.priority", request.Priority)

	// Send approval request
	approvalID, err := h.Interaction.RequestApproval(ctx, request)
	if err != nil {
		return h.handleError(ctx, span, fmt.Errorf("failed to request approval: %w", err))
	}

	span.SetAttribute("approval.id", approvalID)
	ctx.Logger.Info("Approval requested", map[string]interface{}{
		"task_name":   h.Name,
		"approval_id": approvalID,
		"title":       request.Title,
		"token_id":    tokenID(ctx),
	})

	// Wait for decision
	decision, err := h.Interaction.WaitForDecision(ctx, approvalID, h.Timeout)
	if err != nil {
		// Check if it's a timeout
		if h.OnTimeout != nil {
			ctx.Metrics.Inc("task_human_timeout_total")
			if timeoutErr := h.OnTimeout(ctx); timeoutErr != nil {
				return h.handleError(ctx, span, fmt.Errorf("timeout handler failed: %w", timeoutErr))
			}
			return nil
		}
		return h.handleError(ctx, span, fmt.Errorf("failed to get decision: %w", err))
	}

	// Handle decision
	span.SetAttribute("approval.approved", decision.Approved)
	span.SetAttribute("approval.decided_by", decision.DecisionBy)

	if decision.Approved {
		ctx.Metrics.Inc("task_human_approved_total")
		ctx.Logger.Info("Approval granted", map[string]interface{}{
			"task_name":   h.Name,
			"approval_id": approvalID,
			"decided_by":  decision.DecisionBy,
			"token_id":    tokenID(ctx),
		})

		if h.OnApproved != nil {
			if err := h.OnApproved(ctx, decision); err != nil {
				return h.handleError(ctx, span, fmt.Errorf("on_approved handler failed: %w", err))
			}
		}
	} else {
		ctx.Metrics.Inc("task_human_rejected_total")
		ctx.Logger.Info("Approval rejected", map[string]interface{}{
			"task_name":   h.Name,
			"approval_id": approvalID,
			"decided_by":  decision.DecisionBy,
			"comments":    decision.Comments,
			"token_id":    tokenID(ctx),
		})

		if h.OnRejected != nil {
			if err := h.OnRejected(ctx, decision); err != nil {
				return h.handleError(ctx, span, fmt.Errorf("on_rejected handler failed: %w", err))
			}
		} else {
			// Default behavior: rejection is an error
			return h.handleError(ctx, span, fmt.Errorf("approval rejected by %s: %s", decision.DecisionBy, decision.Comments))
		}
	}

	// Success
	ctx.Metrics.Inc("task_success_total")
	ctx.Metrics.Inc("task_human_success_total")

	return nil
}

func (h *HumanTask) executeDataEntry(ctx *context.ExecutionContext) error {
	// Start trace span
	span := ctx.Tracer.StartSpan("task.human.data_entry." + h.Name)
	defer span.End()

	// Record metrics
	ctx.Metrics.Inc("task_executions_total")
	ctx.Metrics.Inc("task_human_executions_total")
	ctx.Metrics.Inc("task_human_data_entry_requests_total")

	// Log task start
	ctx.Logger.Debug("Executing human data entry task", map[string]interface{}{
		"task_name": h.Name,
		"timeout":   h.Timeout.String(),
		"token_id":  tokenID(ctx),
	})

	// Generate data request
	var request *DataEntryRequest
	var err error

	if h.DataRequestFunc != nil {
		request, err = h.DataRequestFunc(ctx)
		if err != nil {
			return h.handleError(ctx, span, fmt.Errorf("data request generation failed: %w", err))
		}
	} else if h.DataRequest != nil {
		request = h.DataRequest
	} else {
		return h.handleError(ctx, span, fmt.Errorf("human task: data entry request is required"))
	}

	span.SetAttribute("data_entry.title", request.Title)
	span.SetAttribute("data_entry.fields_count", len(request.Fields))

	// Send data entry request
	requestID, err := h.Interaction.RequestDataEntry(ctx, request)
	if err != nil {
		return h.handleError(ctx, span, fmt.Errorf("failed to request data entry: %w", err))
	}

	span.SetAttribute("data_entry.request_id", requestID)
	ctx.Logger.Info("Data entry requested", map[string]interface{}{
		"task_name":  h.Name,
		"request_id": requestID,
		"title":      request.Title,
		"token_id":   tokenID(ctx),
	})

	// Wait for data
	response, err := h.Interaction.WaitForData(ctx, requestID, h.Timeout)
	if err != nil {
		// Check if it's a timeout
		if h.OnTimeout != nil {
			ctx.Metrics.Inc("task_human_timeout_total")
			if timeoutErr := h.OnTimeout(ctx); timeoutErr != nil {
				return h.handleError(ctx, span, fmt.Errorf("timeout handler failed: %w", timeoutErr))
			}
			return nil
		}
		return h.handleError(ctx, span, fmt.Errorf("failed to get data: %w", err))
	}

	// Handle response
	span.SetAttribute("data_entry.provided_by", response.ProvidedBy)
	span.SetAttribute("data_entry.valid", response.Valid)

	if !response.Valid {
		ctx.Metrics.Inc("task_human_validation_failed_total")
		return h.handleError(ctx, span, fmt.Errorf("data validation failed: %v", response.ValidationErrors))
	}

	ctx.Logger.Info("Data received", map[string]interface{}{
		"task_name":   h.Name,
		"request_id":  requestID,
		"provided_by": response.ProvidedBy,
		"token_id":    tokenID(ctx),
	})

	if h.OnDataReceived != nil {
		if err := h.OnDataReceived(ctx, response); err != nil {
			return h.handleError(ctx, span, fmt.Errorf("on_data_received handler failed: %w", err))
		}
	}

	// Success
	ctx.Metrics.Inc("task_success_total")
	ctx.Metrics.Inc("task_human_success_total")

	return nil
}

func (h *HumanTask) executeReview(ctx *context.ExecutionContext) error {
	// Start trace span
	span := ctx.Tracer.StartSpan("task.human.review." + h.Name)
	defer span.End()

	// Record metrics
	ctx.Metrics.Inc("task_executions_total")
	ctx.Metrics.Inc("task_human_executions_total")
	ctx.Metrics.Inc("task_human_review_requests_total")

	// Log task start
	ctx.Logger.Debug("Executing human review task", map[string]interface{}{
		"task_name": h.Name,
		"token_id":  tokenID(ctx),
	})

	// Generate approval request (reuse approval request for review)
	var request *ApprovalRequest
	var err error

	if h.RequestFunc != nil {
		request, err = h.RequestFunc(ctx)
		if err != nil {
			return h.handleError(ctx, span, fmt.Errorf("review request generation failed: %w", err))
		}
	} else if h.Request != nil {
		request = h.Request
	} else {
		return h.handleError(ctx, span, fmt.Errorf("human task: review request is required"))
	}

	span.SetAttribute("review.title", request.Title)

	// Send review request (fire and forget - don't wait for decision)
	approvalID, err := h.Interaction.RequestApproval(ctx, request)
	if err != nil {
		return h.handleError(ctx, span, fmt.Errorf("failed to request review: %w", err))
	}

	ctx.Logger.Info("Review requested (non-blocking)", map[string]interface{}{
		"task_name":   h.Name,
		"approval_id": approvalID,
		"title":       request.Title,
		"token_id":    tokenID(ctx),
	})

	// Success - don't wait for response
	ctx.Metrics.Inc("task_success_total")
	ctx.Metrics.Inc("task_human_success_total")

	return nil
}

// handleError processes errors with observability
func (h *HumanTask) handleError(ctx *context.ExecutionContext, span context.Span, err error) error {
	span.RecordError(err)
	ctx.Metrics.Inc("task_errors_total")
	ctx.Metrics.Inc("task_human_errors_total")
	ctx.ErrorRecorder.RecordError(err, map[string]interface{}{
		"task_name": h.Name,
		"mode":      h.Mode,
		"token_id":  tokenID(ctx),
	})
	ctx.Logger.Error("Human task failed", map[string]interface{}{
		"task_name": h.Name,
		"mode":      h.Mode,
		"error":     err.Error(),
		"token_id":  tokenID(ctx),
	})
	return ctx.ErrorHandler.Handle(ctx, err)
}

// ChannelHumanInteraction is a simple in-memory HumanInteraction for testing.
// It uses Go channels to simulate human responses.
// Thread-safe for concurrent use.
type ChannelHumanInteraction struct {
	// mu protects all fields
	mu sync.RWMutex

	// ApprovalResponses maps approval IDs to their decision channels
	ApprovalResponses map[string]chan *HumanDecision

	// DataResponses maps request IDs to their data channels
	DataResponses map[string]chan *HumanDataResponse

	// nextID is used to generate unique IDs
	nextID int
}

// NewChannelHumanInteraction creates a new channel-based human interaction for testing.
func NewChannelHumanInteraction() *ChannelHumanInteraction {
	return &ChannelHumanInteraction{
		ApprovalResponses: make(map[string]chan *HumanDecision),
		DataResponses:     make(map[string]chan *HumanDataResponse),
	}
}

// RequestApproval creates a channel for receiving the decision.
func (c *ChannelHumanInteraction) RequestApproval(ctx *context.ExecutionContext, request *ApprovalRequest) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.nextID++
	id := fmt.Sprintf("approval-%d", c.nextID)
	c.ApprovalResponses[id] = make(chan *HumanDecision, 1)
	return id, nil
}

// WaitForDecision waits for a decision on the approval channel.
func (c *ChannelHumanInteraction) WaitForDecision(ctx *context.ExecutionContext, approvalID string, timeout time.Duration) (*HumanDecision, error) {
	c.mu.RLock()
	ch, exists := c.ApprovalResponses[approvalID]
	c.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unknown approval ID: %s", approvalID)
	}

	if timeout == 0 {
		// No timeout - wait indefinitely
		select {
		case decision := <-ch:
			return decision, nil
		case <-ctx.Context.Done():
			return nil, ctx.Context.Err()
		}
	}

	// With timeout
	timeoutCh := ctx.Clock.After(timeout)

	select {
	case decision := <-ch:
		return decision, nil
	case <-timeoutCh:
		return nil, fmt.Errorf("approval timeout after %s", timeout)
	case <-ctx.Context.Done():
		return nil, ctx.Context.Err()
	}
}

// RequestDataEntry creates a channel for receiving the data.
func (c *ChannelHumanInteraction) RequestDataEntry(ctx *context.ExecutionContext, request *DataEntryRequest) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.nextID++
	id := fmt.Sprintf("data-entry-%d", c.nextID)
	c.DataResponses[id] = make(chan *HumanDataResponse, 1)
	return id, nil
}

// WaitForData waits for data on the data entry channel.
func (c *ChannelHumanInteraction) WaitForData(ctx *context.ExecutionContext, requestID string, timeout time.Duration) (*HumanDataResponse, error) {
	c.mu.RLock()
	ch, exists := c.DataResponses[requestID]
	c.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unknown request ID: %s", requestID)
	}

	if timeout == 0 {
		// No timeout - wait indefinitely
		select {
		case data := <-ch:
			return data, nil
		case <-ctx.Context.Done():
			return nil, ctx.Context.Err()
		}
	}

	// With timeout
	timeoutCh := ctx.Clock.After(timeout)

	select {
	case data := <-ch:
		return data, nil
	case <-timeoutCh:
		return nil, fmt.Errorf("data entry timeout after %s", timeout)
	case <-ctx.Context.Done():
		return nil, ctx.Context.Err()
	}
}

// SimulateApproval simulates a human approving/rejecting a request (for testing).
func (c *ChannelHumanInteraction) SimulateApproval(approvalID string, approved bool, decidedBy, comments string) error {
	c.mu.RLock()
	ch, exists := c.ApprovalResponses[approvalID]
	c.mu.RUnlock()

	if !exists {
		return fmt.Errorf("unknown approval ID: %s", approvalID)
	}

	ch <- &HumanDecision{
		Approved:   approved,
		DecisionBy: decidedBy,
		DecisionAt: time.Now(),
		Comments:   comments,
	}
	return nil
}

// SimulateDataEntry simulates a human providing data (for testing).
func (c *ChannelHumanInteraction) SimulateDataEntry(requestID string, data map[string]interface{}, providedBy string) error {
	c.mu.RLock()
	ch, exists := c.DataResponses[requestID]
	c.mu.RUnlock()

	if !exists {
		return fmt.Errorf("unknown request ID: %s", requestID)
	}

	ch <- &HumanDataResponse{
		Data:       data,
		ProvidedBy: providedBy,
		ProvidedAt: time.Now(),
		Valid:      true,
	}
	return nil
}
