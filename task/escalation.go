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

// EscalationLevel represents the escalation severity level.
type EscalationLevel int

const (
	// EscalationLevelNone indicates no escalation is active.
	EscalationLevelNone EscalationLevel = iota

	// EscalationLevel1 is the first escalation level (e.g., direct manager).
	EscalationLevel1

	// EscalationLevel2 is the second escalation level (e.g., senior manager).
	EscalationLevel2

	// EscalationLevel3 is the third escalation level (e.g., executive).
	EscalationLevel3
)

// String returns the string representation of the escalation level.
func (e EscalationLevel) String() string {
	switch e {
	case EscalationLevelNone:
		return "none"
	case EscalationLevel1:
		return "level1"
	case EscalationLevel2:
		return "level2"
	case EscalationLevel3:
		return "level3"
	default:
		return fmt.Sprintf("level%d", int(e))
	}
}

// EscalationConfig defines a single escalation level.
type EscalationConfig struct {
	// Level is the escalation severity level.
	Level EscalationLevel

	// EscalateTo is the list of recipients at this level (user IDs, emails, roles).
	EscalateTo []string

	// After is the time to wait before escalating to the next level.
	// If this is the last level, escalation stops after this duration.
	After time.Duration
}

// EscalationResolver checks if an issue has been resolved.
// Implementations connect to notification systems, task queues, databases, etc.
//
// This is the PORT in the Ports & Adapters pattern. Concrete implementations
// (Database check, API polling, Channel-based, etc.) are ADAPTERS.
type EscalationResolver interface {
	// IsResolved checks if the issue has been resolved.
	IsResolved(ctx *context.ExecutionContext) (bool, error)

	// WaitForResolution returns a channel that signals when the issue is resolved.
	// The channel receives true when resolved, false on timeout.
	WaitForResolution(ctx *context.ExecutionContext) <-chan bool
}

// EscalationTask handles escalation when SLAs are breached or conditions are met.
// This enables multi-level escalation chains with time-based and condition-based triggers.
//
// EscalationTask is ideal for:
//   - SLA breach escalations (support tickets, incident response)
//   - Approval timeout escalations (when approvers don't respond)
//   - Multi-level notification chains (L1 → L2 → L3 managers)
//   - Conditional escalations (based on data thresholds)
//   - Automated incident management
//
// Features:
//   - Multi-level escalation chains with configurable durations
//   - Time-based and condition-based escalation triggers
//   - Resolution mechanism to stop escalation early
//   - Customizable notification handlers at each level
//   - Full observability with metrics and tracing
//
// Example - Time-based SLA escalation:
//
//	task := &EscalationTask{
//	    Name: "support-ticket-escalation",
//	    Levels: []EscalationConfig{
//	        {Level: EscalationLevel1, EscalateTo: []string{"support-team"}, After: 30 * time.Minute},
//	        {Level: EscalationLevel2, EscalateTo: []string{"senior-support"}, After: 1 * time.Hour},
//	        {Level: EscalationLevel3, EscalateTo: []string{"manager"}, After: 2 * time.Hour},
//	    },
//	    CheckInterval: 1 * time.Minute,
//	    OnEscalate: func(ctx *context.ExecutionContext, level EscalationLevel, recipients []string) error {
//	        // Send notifications
//	        return notifyTeam(recipients, "Ticket escalated to " + level.String())
//	    },
//	}
//
// Example - Condition-based escalation with resolution:
//
//	task := &EscalationTask{
//	    Name: "order-delay-escalation",
//	    Levels: []EscalationConfig{
//	        {Level: EscalationLevel1, EscalateTo: []string{"ops-team"}, After: 5 * time.Minute},
//	        {Level: EscalationLevel2, EscalateTo: []string{"ops-manager"}, After: 15 * time.Minute},
//	    },
//	    CheckInterval: 1 * time.Minute,
//	    EscalateWhen: func(ctx *context.ExecutionContext) (bool, error) {
//	        order := ctx.Token.Data.(*OrderData)
//	        delay := ctx.Clock.Now().Sub(order.CreatedAt)
//	        return delay > order.SLA, nil
//	    },
//	    Resolver: dbResolver,
//	    OnResolved: func(ctx *context.ExecutionContext, level EscalationLevel) error {
//	        ctx.Logger.Info("Issue resolved before full escalation", map[string]interface{}{
//	            "stopped_at_level": level.String(),
//	        })
//	        return nil
//	    },
//	}
//
// Thread Safety:
// EscalationTask is safe for concurrent execution. Each execution manages its own
// escalation state and goroutines.
type EscalationTask struct {
	// Name is the task identifier used in logs, traces, and metrics.
	Name string

	// Levels defines the escalation chain.
	// Escalation progresses through levels in order.
	Levels []EscalationConfig

	// CheckInterval is how often to check the escalation condition.
	// Defaults to 1 minute if not specified.
	CheckInterval time.Duration

	// EscalateWhen is an optional condition function for condition-based escalation.
	// If nil, escalation proceeds purely on time-based schedule.
	// If returns true, escalation starts immediately.
	EscalateWhen func(ctx *context.ExecutionContext) (bool, error)

	// OnEscalate is called when escalation reaches a new level.
	// Use this to send notifications, create incidents, etc.
	OnEscalate func(ctx *context.ExecutionContext, level EscalationLevel, recipients []string) error

	// OnResolved is called when the issue is resolved before full escalation.
	// Optional - if nil, resolution just stops escalation.
	OnResolved func(ctx *context.ExecutionContext, level EscalationLevel) error

	// Resolver checks for resolution and allows early termination.
	// If nil, escalation runs to completion through all levels.
	Resolver EscalationResolver
}

// Execute performs the escalation workflow.
//
// Execution flow:
//  1. Validate task configuration
//  2. Start trace span
//  3. Check initial escalation condition (if EscalateWhen set)
//  4. For each escalation level:
//     a. Wait for level duration (checking for resolution periodically)
//     b. If resolved, call OnResolved and exit
//     c. If not resolved, escalate to level and call OnEscalate
//  5. Complete after final level
//
// Thread Safety:
// Execute is safe to call concurrently from multiple goroutines.
//
// Returns an error if:
//   - Task configuration is invalid (missing Name, no levels)
//   - Escalation condition check fails
//   - OnEscalate callback fails
//   - Context is cancelled
func (e *EscalationTask) Execute(ctx *context.ExecutionContext) error {
	// Validate configuration
	if e.Name == "" {
		return fmt.Errorf("escalation task: name is required")
	}
	if len(e.Levels) == 0 {
		return fmt.Errorf("escalation task: at least one escalation level is required")
	}

	// Set default check interval
	checkInterval := e.CheckInterval
	if checkInterval <= 0 {
		checkInterval = 1 * time.Minute
	}

	// Start trace span
	span := ctx.Tracer.StartSpan("task.escalation." + e.Name)
	defer span.End()

	span.SetAttribute("escalation.levels", len(e.Levels))
	span.SetAttribute("escalation.check_interval", checkInterval.String())

	// Record metrics
	ctx.Metrics.Inc("task_executions_total")
	ctx.Metrics.Inc("task_escalation_executions_total")
	ctx.Metrics.Inc("escalation_started_total")

	// Log task start
	ctx.Logger.Debug("Executing escalation task", map[string]interface{}{
		"task_name":      e.Name,
		"levels":         len(e.Levels),
		"check_interval": checkInterval.String(),
		"token_id":       tokenID(ctx),
	})

	// Check if escalation should start immediately
	if e.EscalateWhen != nil {
		shouldEscalate, err := e.checkCondition(ctx)
		if err != nil {
			return e.handleError(ctx, span, fmt.Errorf("escalation condition check failed: %w", err))
		}
		if !shouldEscalate {
			ctx.Logger.Debug("Escalation condition not met, task completed", map[string]interface{}{
				"task_name": e.Name,
				"token_id":  tokenID(ctx),
			})
			ctx.Metrics.Inc("task_success_total")
			ctx.Metrics.Inc("task_escalation_success_total")
			return nil
		}
	}

	// Record start time for duration tracking
	startTime := ctx.Clock.Now()

	// Execute escalation chain
	currentLevel := EscalationLevelNone
	for i, level := range e.Levels {
		// Wait for the specified duration while checking for resolution
		resolved, err := e.waitWithResolutionCheck(ctx, level.After, checkInterval)
		if err != nil {
			return e.handleError(ctx, span, fmt.Errorf("escalation wait failed: %w", err))
		}

		if resolved {
			// Issue was resolved before reaching this level
			ctx.Logger.Info("Escalation resolved before reaching level", map[string]interface{}{
				"task_name":      e.Name,
				"stopped_before": level.Level.String(),
				"token_id":       tokenID(ctx),
			})

			ctx.Metrics.Inc("escalation_resolved_total")

			duration := ctx.Clock.Now().Sub(startTime)
			ctx.Metrics.Observe("escalation_duration_seconds", duration.Seconds())

			if e.OnResolved != nil {
				if err := e.OnResolved(ctx, currentLevel); err != nil {
					return e.handleError(ctx, span, fmt.Errorf("on_resolved handler failed: %w", err))
				}
			}

			ctx.Metrics.Inc("task_success_total")
			ctx.Metrics.Inc("task_escalation_success_total")
			return nil
		}

		// Escalate to this level
		currentLevel = level.Level

		span.SetAttribute(fmt.Sprintf("escalation.reached_level_%d", i+1), level.Level.String())
		ctx.Metrics.Inc("escalation_level_reached")

		ctx.Logger.Warn("Escalating to level", map[string]interface{}{
			"task_name":  e.Name,
			"level":      level.Level.String(),
			"recipients": level.EscalateTo,
			"token_id":   tokenID(ctx),
		})

		// Call escalation handler
		if e.OnEscalate != nil {
			if err := e.OnEscalate(ctx, level.Level, level.EscalateTo); err != nil {
				return e.handleError(ctx, span, fmt.Errorf("on_escalate handler failed at level %s: %w",
					level.Level.String(), err))
			}
		}
	}

	// Escalation completed through all levels
	duration := ctx.Clock.Now().Sub(startTime)
	ctx.Metrics.Observe("escalation_duration_seconds", duration.Seconds())

	ctx.Logger.Warn("Escalation completed through all levels", map[string]interface{}{
		"task_name":   e.Name,
		"final_level": currentLevel.String(),
		"duration":    duration.String(),
		"token_id":    tokenID(ctx),
	})

	ctx.Metrics.Inc("task_success_total")
	ctx.Metrics.Inc("task_escalation_success_total")

	return nil
}

// checkCondition checks the escalation condition function with panic recovery.
func (e *EscalationTask) checkCondition(ctx *context.ExecutionContext) (result bool, err error) {
	// Recover from panics in the condition function
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("escalation condition panicked: %v", r)
		}
	}()

	return e.EscalateWhen(ctx)
}

// waitWithResolutionCheck waits for the specified duration while periodically checking for resolution.
// Returns true if resolved, false if duration elapsed without resolution.
func (e *EscalationTask) waitWithResolutionCheck(ctx *context.ExecutionContext, duration, checkInterval time.Duration) (bool, error) {
	if duration <= 0 {
		// No wait needed
		return e.checkResolution(ctx)
	}

	deadline := ctx.Clock.Now().Add(duration)

	// If we have a resolver, use it
	if e.Resolver != nil {
		return e.waitWithResolver(ctx, deadline, checkInterval)
	}

	// No resolver - just wait for the duration
	select {
	case <-ctx.Clock.After(duration):
		return false, nil
	case <-ctx.Context.Done():
		return false, ctx.Context.Err()
	}
}

// waitWithResolver waits with periodic resolution checks.
func (e *EscalationTask) waitWithResolver(ctx *context.ExecutionContext, deadline time.Time, checkInterval time.Duration) (bool, error) {
	// Create a channel for resolution checks
	resolutionCh := e.Resolver.WaitForResolution(ctx)

	// Create timer for next check
	nextCheck := ctx.Clock.After(checkInterval)

	for {
		select {
		case resolved := <-resolutionCh:
			return resolved, nil

		case <-nextCheck:
			// Periodic check
			resolved, err := e.checkResolution(ctx)
			if err != nil {
				ctx.Logger.Warn("Resolution check failed", map[string]interface{}{
					"task_name": e.Name,
					"error":     err.Error(),
					"token_id":  tokenID(ctx),
				})
				// Continue waiting despite check failure
				// Reset timer for next check
				nextCheck = ctx.Clock.After(checkInterval)
				continue
			}
			if resolved {
				return true, nil
			}

			// Check if we've reached the deadline
			if ctx.Clock.Now().After(deadline) {
				return false, nil
			}

			// Reset timer for next check
			nextCheck = ctx.Clock.After(checkInterval)

		case <-ctx.Context.Done():
			return false, ctx.Context.Err()
		}
	}
}

// checkResolution checks if the issue has been resolved.
func (e *EscalationTask) checkResolution(ctx *context.ExecutionContext) (bool, error) {
	if e.Resolver == nil {
		return false, nil
	}

	return e.Resolver.IsResolved(ctx)
}

// handleError processes errors with observability.
func (e *EscalationTask) handleError(ctx *context.ExecutionContext, span context.Span, err error) error {
	span.RecordError(err)
	ctx.Metrics.Inc("task_errors_total")
	ctx.Metrics.Inc("task_escalation_errors_total")
	ctx.ErrorRecorder.RecordError(err, map[string]interface{}{
		"task_name": e.Name,
		"token_id":  tokenID(ctx),
	})
	ctx.Logger.Error("Escalation task failed", map[string]interface{}{
		"task_name": e.Name,
		"error":     err.Error(),
		"token_id":  tokenID(ctx),
	})
	return ctx.ErrorHandler.Handle(ctx, err)
}

// ChannelEscalationResolver is a simple channel-based resolver for testing.
type ChannelEscalationResolver struct {
	// resolvedCh signals when the issue is resolved
	resolvedCh chan bool

	// mu protects the resolved flag
	mu       sync.RWMutex
	resolved bool
}

// NewChannelEscalationResolver creates a new channel-based resolver for testing.
func NewChannelEscalationResolver() *ChannelEscalationResolver {
	return &ChannelEscalationResolver{
		resolvedCh: make(chan bool, 1),
	}
}

// IsResolved returns the current resolution state.
func (c *ChannelEscalationResolver) IsResolved(ctx *context.ExecutionContext) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.resolved, nil
}

// WaitForResolution returns a channel that signals resolution.
func (c *ChannelEscalationResolver) WaitForResolution(ctx *context.ExecutionContext) <-chan bool {
	return c.resolvedCh
}

// Resolve marks the issue as resolved (for testing).
func (c *ChannelEscalationResolver) Resolve() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.resolved {
		c.resolved = true
		select {
		case c.resolvedCh <- true:
		default:
			// Channel already has a value
		}
	}
}
