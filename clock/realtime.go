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

import "time"

// RealTimeClock provides a production Clock implementation using actual wall-clock time.
// All operations delegate directly to Go's time package with zero overhead.
//
// RealTimeClock is safe for concurrent use by multiple goroutines as it maintains no state.
//
// Example:
//
//	clock := clock.NewRealTimeClock()
//	start := clock.Now()
//	clock.Sleep(1 * time.Second)
//	elapsed := clock.Now().Sub(start) // approximately 1 second
type RealTimeClock struct{}

// NewRealTimeClock creates a new real-time clock for production use.
// The returned clock uses the system's wall-clock time.
func NewRealTimeClock() *RealTimeClock {
	return &RealTimeClock{}
}

// Now returns the current wall-clock time by delegating to time.Now().
func (r *RealTimeClock) Now() time.Time {
	return time.Now()
}

// After returns a channel that receives the current time after duration d.
// This delegates directly to time.After() and behaves identically.
func (r *RealTimeClock) After(d time.Duration) <-chan time.Time {
	return time.After(d)
}

// Sleep pauses the current goroutine for at least duration d.
// This delegates directly to time.Sleep() and behaves identically.
func (r *RealTimeClock) Sleep(d time.Duration) {
	time.Sleep(d)
}
