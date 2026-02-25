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

package clock

import (
	"sync"
	"time"
)

// VirtualClock provides a Clock implementation with manual time control for testing.
// Unlike RealTimeClock, VirtualClock does not use actual wall-clock time. Instead,
// time only advances when explicitly commanded via AdvanceTo or AdvanceBy methods.
//
// This enables deterministic testing of time-dependent workflows, fast-forwarding
// through delays, and time-travel debugging capabilities.
//
// VirtualClock is safe for concurrent use by multiple goroutines. All operations
// are protected by an internal mutex.
//
// Example:
//
//	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
//	clock := clock.NewVirtualClock(start)
//
//	// Create a timer for 5 seconds in the future
//	timer := clock.After(5 * time.Second)
//
//	// Time hasn't advanced yet, so timer hasn't fired
//	select {
//	case <-timer:
//	    t.Fatal("timer fired before time advanced")
//	default:
//	    // Expected
//	}
//
//	// Advance time by 10 seconds
//	clock.AdvanceBy(10 * time.Second)
//
//	// Now the timer fires immediately
//	<-timer
type VirtualClock struct {
	mu      sync.RWMutex
	current time.Time
	timers  []*virtualTimer
	sleeps  []*virtualSleep
}

// virtualTimer represents a pending timer created by After().
type virtualTimer struct {
	deadline time.Time
	ch       chan time.Time
	fired    bool
}

// virtualSleep represents a goroutine blocked in Sleep().
type virtualSleep struct {
	deadline time.Time
	ch       chan struct{}
	fired    bool
}

// NewVirtualClock creates a new virtual clock starting at the specified time.
// The clock's time will only advance when AdvanceTo() or AdvanceBy() is called.
func NewVirtualClock(start time.Time) *VirtualClock {
	return &VirtualClock{
		current: start,
		timers:  make([]*virtualTimer, 0),
		sleeps:  make([]*virtualSleep, 0),
	}
}

// Now returns the current virtual time.
// Time only advances via AdvanceTo() or AdvanceBy() calls.
func (v *VirtualClock) Now() time.Time {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.current
}

// After returns a channel that receives the current time after duration d.
// The channel only fires when the virtual time is advanced past the deadline.
//
// If time is already past the deadline when After() is called, the channel
// fires immediately.
func (v *VirtualClock) After(d time.Duration) <-chan time.Time {
	v.mu.Lock()
	defer v.mu.Unlock()

	deadline := v.current.Add(d)
	ch := make(chan time.Time, 1)

	timer := &virtualTimer{
		deadline: deadline,
		ch:       ch,
		fired:    false,
	}

	// If deadline is already in the past, fire immediately
	if !deadline.After(v.current) {
		timer.ch <- v.current
		close(timer.ch)
		timer.fired = true
	} else {
		// Add to pending timers
		v.timers = append(v.timers, timer)
	}

	return ch
}

// Sleep blocks the current goroutine until the virtual time is advanced
// past the sleep deadline (current time + duration d).
//
// If duration is <= 0, Sleep returns immediately.
func (v *VirtualClock) Sleep(d time.Duration) {
	if d <= 0 {
		return
	}

	v.mu.Lock()
	deadline := v.current.Add(d)
	ch := make(chan struct{})

	sleep := &virtualSleep{
		deadline: deadline,
		ch:       ch,
		fired:    false,
	}

	// If deadline is already in the past, return immediately
	if !deadline.After(v.current) {
		v.mu.Unlock()
		return
	}

	// Add to pending sleeps
	v.sleeps = append(v.sleeps, sleep)
	v.mu.Unlock()

	// Block until the sleep is fired
	<-ch
}

// AdvanceTo advances the virtual clock to the specified time and fires all
// timers and sleeps whose deadlines have been reached.
//
// If the target time is before the current time, the clock does not move backward.
// In this case, AdvanceTo is a no-op.
//
// All timers and sleeps with deadlines <= targetTime will fire in deadline order.
func (v *VirtualClock) AdvanceTo(targetTime time.Time) {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Don't move time backward
	if !targetTime.After(v.current) {
		return
	}

	v.current = targetTime
	v.fireTimersAndSleeps()
}

// AdvanceBy advances the virtual clock by the specified duration and fires all
// timers and sleeps whose deadlines have been reached.
//
// This is equivalent to calling AdvanceTo(clock.Now().Add(d)).
//
// If duration is <= 0, AdvanceBy is a no-op.
func (v *VirtualClock) AdvanceBy(d time.Duration) {
	if d <= 0 {
		return
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	v.current = v.current.Add(d)
	v.fireTimersAndSleeps()
}

// fireTimersAndSleeps fires all timers and sleeps whose deadlines have been reached.
// Must be called with mutex locked.
func (v *VirtualClock) fireTimersAndSleeps() {
	// Fire all timers that are ready
	remainingTimers := make([]*virtualTimer, 0, len(v.timers))
	for _, timer := range v.timers {
		if !timer.fired && !timer.deadline.After(v.current) {
			// Fire the timer
			timer.ch <- v.current
			close(timer.ch)
			timer.fired = true
		} else if !timer.fired {
			// Keep unfired timers
			remainingTimers = append(remainingTimers, timer)
		}
	}
	v.timers = remainingTimers

	// Fire all sleeps that are ready
	remainingSleeps := make([]*virtualSleep, 0, len(v.sleeps))
	for _, sleep := range v.sleeps {
		if !sleep.fired && !sleep.deadline.After(v.current) {
			// Fire the sleep
			close(sleep.ch)
			sleep.fired = true
		} else if !sleep.fired {
			// Keep unfired sleeps
			remainingSleeps = append(remainingSleeps, sleep)
		}
	}
	v.sleeps = remainingSleeps
}

// PendingTimers returns the number of timers waiting to fire.
// This is useful for testing to verify that timers have been properly cleaned up.
func (v *VirtualClock) PendingTimers() int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return len(v.timers)
}

// PendingSleeps returns the number of goroutines blocked in Sleep().
// This is useful for testing to verify that sleeps have been properly cleaned up.
func (v *VirtualClock) PendingSleeps() int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return len(v.sleeps)
}
