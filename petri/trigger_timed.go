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

// TimedTrigger fires after a delay or at a specific time.
//
// Uses ExecutionContext.Clock for time operations, enabling:
//   - Virtual clock in tests (fast, deterministic)
//   - Real clock in production (actual wall-clock time)
//
// Exactly one of Delay or At must be specified:
//   - Delay: Fire after duration from when transition becomes enabled
//   - At: Fire at specific wall-clock time
//
// Example - Delay (send reminder 24 hours after order):
//
//	delay := 24 * time.Hour
//	t := NewTransition("reminder", "Send Reminder")
//	t.WithTrigger(&TimedTrigger{Delay: &delay})
//
// Example - Specific time (daily report at 9 AM):
//
//	reportTime := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
//	t := NewTransition("daily_report", "Generate Daily Report")
//	t.WithTrigger(&TimedTrigger{At: &reportTime})
//
// Thread Safety:
// TimedTrigger is safe for concurrent ShouldFire() calls but expects
// only one AwaitFiring() goroutine at a time (enforced by engine).
type TimedTrigger struct {
	// Delay fires after this duration from when transition becomes enabled.
	// Mutually exclusive with At - exactly one must be specified.
	Delay *time.Duration

	// At fires at this specific time.
	// Mutually exclusive with Delay - exactly one must be specified.
	At *time.Time

	// Internal state
	scheduledAt time.Time
	timer       <-chan time.Time
	mu          sync.Mutex
}

// ShouldFire returns true if the scheduled time has been reached.
// On first call, initializes the timer based on Delay or At.
func (t *TimedTrigger) ShouldFire(ctx *context.ExecutionContext) (bool, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Validate configuration on first call
	if t.Delay == nil && t.At == nil {
		return false, fmt.Errorf("TimedTrigger: neither Delay nor At specified")
	}
	if t.Delay != nil && t.At != nil {
		return false, fmt.Errorf("TimedTrigger: both Delay and At specified (must be mutually exclusive)")
	}

	// Check if timer already fired
	if t.timer != nil {
		select {
		case <-t.timer:
			return true, nil
		default:
			return false, nil
		}
	}

	// Initialize timer on first check
	var deadline time.Time
	if t.At != nil {
		deadline = *t.At
	} else { // t.Delay != nil
		deadline = ctx.Clock.Now().Add(*t.Delay)
	}

	t.scheduledAt = deadline
	remaining := deadline.Sub(ctx.Clock.Now())

	if remaining <= 0 {
		// Already past deadline - fire immediately
		return true, nil
	}

	// Create timer channel using ExecutionContext.Clock
	// This enables virtual clock in tests
	t.timer = ctx.Clock.After(remaining)
	return false, nil
}

// AwaitFiring blocks until the scheduled time is reached.
// Respects context cancellation for engine stop or workflow cancel.
func (t *TimedTrigger) AwaitFiring(ctx *context.ExecutionContext) error {
	t.mu.Lock()
	timer := t.timer
	t.mu.Unlock()

	if timer == nil {
		return fmt.Errorf("timer not initialized - call ShouldFire() first")
	}

	// Wait for timer or cancellation
	select {
	case <-timer:
		// Timer fired - transition is ready
		return nil
	case <-ctx.Context.Done():
		// Context cancelled (engine stopped or workflow cancelled)
		return ctx.Context.Err()
	}
}

// Cancel stops the timer and cleans up resources.
// Note: Go timers cannot be cancelled, but we can ignore the channel.
func (t *TimedTrigger) Cancel() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Clear timer reference (makes it garbage collectable)
	t.timer = nil
	return nil
}

// ScheduledAt returns the time when this trigger is scheduled to fire.
// Returns zero time if trigger hasn't been initialized yet.
func (t *TimedTrigger) ScheduledAt() time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.scheduledAt
}
