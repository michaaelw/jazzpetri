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

// Package clock provides time abstractions for testing and production use in Jazz3.
//
// The clock package implements a hybrid time management system that supports both
// real wall-clock time (for production) and virtual time (for testing and debugging).
// This design enables deterministic testing of time-dependent workflows while maintaining
// zero overhead in production deployments.
//
// Key Features:
//   - Clock interface abstracts all time operations
//   - RealTimeClock uses actual system time for production
//   - VirtualClock provides manual time control for testing
//   - Channel-based After() method integrates with Go's concurrency patterns
//   - Thread-safe implementations for concurrent workflow execution
//
// Example usage in production:
//
//	clock := clock.NewRealTimeClock()
//	now := clock.Now()
//	timer := clock.After(5 * time.Second)
//	<-timer // waits 5 seconds
//
// Example usage in tests:
//
//	clock := clock.NewVirtualClock(startTime)
//	timer := clock.After(5 * time.Second)
//	clock.AdvanceTo(startTime.Add(10 * time.Second)) // instantly advance
//	<-timer // receives immediately
package clock

import "time"

// Clock abstracts time operations for testing and production.
// Implementations must be safe for concurrent use by multiple goroutines.
//
// The Clock interface provides three core operations:
//   - Now() returns the current time
//   - After() returns a channel that fires after a duration
//   - Sleep() pauses execution for a duration
//
// In production, use RealTimeClock which delegates to Go's time package.
// In tests, use VirtualClock which allows manual time advancement.
type Clock interface {
	// Now returns the current time according to this clock.
	// For RealTimeClock, this is time.Now().
	// For VirtualClock, this is the manually advanced time.
	Now() time.Time

	// After returns a channel that receives the current time after duration d.
	// This mimics time.After() but is controllable in virtual time.
	// The channel receives exactly once and is then closed.
	//
	// For RealTimeClock, delegates to time.After().
	// For VirtualClock, the channel fires when time is advanced past the deadline.
	After(d time.Duration) <-chan time.Time

	// Sleep pauses the current goroutine for at least duration d.
	// For RealTimeClock, delegates to time.Sleep().
	// For VirtualClock, blocks until time is advanced past the deadline.
	Sleep(d time.Duration)
}
