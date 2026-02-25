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
	"fmt"
	"sync"
	"time"

	"github.com/jazzpetri/engine/context"
)

// ManualTrigger waits for explicit external approval before firing.
// Enables human-in-the-loop workflows where transitions require manual intervention.
//
// Example - Expense approval:
//
//	t := NewTransition("approve_expense", "Approve Expense")
//	timeout := 48 * time.Hour
//	t.WithTrigger(&ManualTrigger{
//	    ApprovalID: "expense-REQ-12345",
//	    Approvers:  []string{"alice@example.com", "bob@example.com"},
//	    RequireAll: false, // Any one approver can approve
//	    Timeout:    &timeout,
//	})
//
// Approval Flow:
//  1. Transition becomes enabled (tokens + guard pass)
//  2. ManualTrigger.AwaitFiring() blocks until approval
//  3. External system calls Approve() method (via REST API, UI, etc.)
//  4. Transition fires
//
// Timeout:
// If Timeout is specified and no approval received within duration,
// AwaitFiring() returns error and transition does not fire.
//
// Multiple Approvers:
//   - RequireAll=false (default): First approver triggers firing
//   - RequireAll=true: All approvers must approve before firing
//
// Thread Safety:
// ManualTrigger is safe for concurrent Approve() calls and one AwaitFiring() goroutine.
//
// Future Enhancement (Milestone 1.6):
// Integration with ApprovalManager for centralized approval tracking,
// REST APIs, and approval UI.
type ManualTrigger struct {
	// ApprovalID is the unique identifier for this approval request.
	// Used to reference this approval in external systems (UI, API).
	ApprovalID string

	// Approvers is the list of user IDs who can approve this transition.
	// Format depends on your auth system (email, user ID, etc.).
	Approvers []string

	// RequireAll determines whether all approvers must approve or just one.
	//   - false (default): Any one approver can trigger firing
	//   - true: All approvers must approve before firing
	RequireAll bool

	// Timeout is the optional maximum time to wait for approval.
	// If nil, waits indefinitely (until approval or context cancellation).
	// If set, AwaitFiring() returns error after duration with no approval.
	Timeout *time.Duration

	// Internal state
	approvalChan chan struct{}     // Closed when approved
	approved     bool               // Whether approval received
	approvedBy   map[string]bool   // Track who has approved (for RequireAll)
	mu           sync.RWMutex
}

// ShouldFire returns true if approval has been received.
func (t *ManualTrigger) ShouldFire(ctx *context.ExecutionContext) (bool, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.approved, nil
}

// AwaitFiring blocks until approval is received, timeout occurs, or context is cancelled.
func (t *ManualTrigger) AwaitFiring(ctx *context.ExecutionContext) error {
	// Initialize approval channel on first call
	t.mu.Lock()
	if t.approvalChan == nil {
		t.approvalChan = make(chan struct{})
		t.approvedBy = make(map[string]bool)
	}
	approvalChan := t.approvalChan
	t.mu.Unlock()

	// TODO (Milestone 1.6): Register with ApprovalManager
	// if ctx.ApprovalManager != nil {
	//     ctx.ApprovalManager.RegisterApproval(t.ApprovalID, t)
	// }

	// Wait for approval, timeout, or cancellation
	if t.Timeout != nil {
		timer := ctx.Clock.After(*t.Timeout)
		select {
		case <-approvalChan:
			return nil
		case <-timer:
			return fmt.Errorf("approval timeout after %v for approval ID: %s", *t.Timeout, t.ApprovalID)
		case <-ctx.Context.Done():
			return ctx.Context.Err()
		}
	} else {
		select {
		case <-approvalChan:
			return nil
		case <-ctx.Context.Done():
			return ctx.Context.Err()
		}
	}
}

// Approve grants approval for this transition to fire.
// Called by external systems (REST API, UI, workflow orchestrator).
//
// Parameters:
//   - userID: The ID of the approver
//
// Returns error if:
//   - userID is not in Approvers list (unauthorized)
//   - Already approved (if RequireAll=false)
//
// For RequireAll=true, tracks which users have approved and only fires
// when all required approvers have called Approve().
func (t *ManualTrigger) Approve(userID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Check if user is authorized approver
	authorized := false
	for _, approver := range t.Approvers {
		if approver == userID {
			authorized = true
			break
		}
	}
	if !authorized {
		return fmt.Errorf("user %s not authorized to approve %s", userID, t.ApprovalID)
	}

	// If already approved, reject duplicate
	if t.approved && !t.RequireAll {
		return fmt.Errorf("approval %s already granted", t.ApprovalID)
	}

	// Track this approval
	if t.approvedBy == nil {
		t.approvedBy = make(map[string]bool)
	}
	t.approvedBy[userID] = true

	// Check if all required approvals received
	if t.RequireAll {
		// Check if all approvers have approved
		allApproved := true
		for _, approver := range t.Approvers {
			if !t.approvedBy[approver] {
				allApproved = false
				break
			}
		}
		if !allApproved {
			// Still waiting for more approvals
			return nil
		}
	}

	// Mark as approved and signal waiting goroutine
	t.approved = true
	if t.approvalChan != nil {
		close(t.approvalChan)
	}

	return nil
}

// Reject denies the approval and prevents the transition from firing.
// Future Enhancement: Could route to different output or trigger compensation.
func (t *ManualTrigger) Reject(userID string, reason string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Check authorization
	authorized := false
	for _, approver := range t.Approvers {
		if approver == userID {
			authorized = true
			break
		}
	}
	if !authorized {
		return fmt.Errorf("user %s not authorized to reject %s", userID, t.ApprovalID)
	}

	// Mark as rejected (currently treated as timeout/never approved)
	// Future: Could track rejection separately and route to rejection handler
	return fmt.Errorf("approval %s rejected by %s: %s", t.ApprovalID, userID, reason)
}

// Cancel stops waiting for approval and cleans up resources.
func (t *ManualTrigger) Cancel() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.approvalChan != nil {
		// Close channel to unblock AwaitFiring() with context cancellation
		close(t.approvalChan)
	}
	return nil
}

// IsApproved returns whether approval has been granted.
func (t *ManualTrigger) IsApproved() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.approved
}

// ApprovedBy returns the list of users who have approved so far.
// Useful for RequireAll=true to track progress.
func (t *ManualTrigger) ApprovedBy() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	approved := make([]string, 0, len(t.approvedBy))
	for userID := range t.approvedBy {
		approved = append(approved, userID)
	}
	return approved
}
