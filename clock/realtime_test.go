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
	"testing"
	"time"
)

func TestNewRealTimeClock(t *testing.T) {
	clock := NewRealTimeClock()
	if clock == nil {
		t.Fatal("NewRealTimeClock() returned nil")
	}
}

func TestRealTimeClock_Now(t *testing.T) {
	clock := NewRealTimeClock()

	before := time.Now()
	now := clock.Now()
	after := time.Now()

	// Clock's Now() should be between our before/after measurements
	if now.Before(before) || now.After(after) {
		t.Errorf("clock.Now() = %v, expected between %v and %v", now, before, after)
	}
}

func TestRealTimeClock_After(t *testing.T) {
	clock := NewRealTimeClock()
	duration := 50 * time.Millisecond

	start := time.Now()
	ch := clock.After(duration)

	// Channel should not be closed immediately
	select {
	case <-ch:
		t.Fatal("After() channel fired immediately")
	case <-time.After(10 * time.Millisecond):
		// Expected - channel hasn't fired yet
	}

	// Wait for the channel to fire
	receivedTime := <-ch
	elapsed := time.Since(start)

	// Should have waited approximately the specified duration
	if elapsed < duration {
		t.Errorf("After() fired too early: elapsed=%v, expected>=%v", elapsed, duration)
	}

	// Should not have waited much longer (allow 50ms tolerance)
	maxExpected := duration + 50*time.Millisecond
	if elapsed > maxExpected {
		t.Errorf("After() fired too late: elapsed=%v, expected<=%v", elapsed, maxExpected)
	}

	// Received time should be close to actual time
	now := time.Now()
	timeDiff := now.Sub(receivedTime)
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}
	if timeDiff > 50*time.Millisecond {
		t.Errorf("After() returned time %v differs too much from now %v (diff=%v)",
			receivedTime, now, timeDiff)
	}
}

func TestRealTimeClock_Sleep(t *testing.T) {
	clock := NewRealTimeClock()
	duration := 50 * time.Millisecond

	start := time.Now()
	clock.Sleep(duration)
	elapsed := time.Since(start)

	// Should have slept approximately the specified duration
	if elapsed < duration {
		t.Errorf("Sleep() returned too early: elapsed=%v, expected>=%v", elapsed, duration)
	}

	// Should not have slept much longer (allow 50ms tolerance)
	maxExpected := duration + 50*time.Millisecond
	if elapsed > maxExpected {
		t.Errorf("Sleep() returned too late: elapsed=%v, expected<=%v", elapsed, maxExpected)
	}
}

func TestRealTimeClock_ConcurrentUse(t *testing.T) {
	// RealTimeClock should be safe for concurrent use
	clock := NewRealTimeClock()

	done := make(chan bool)

	// Start multiple goroutines using the clock concurrently
	for i := 0; i < 10; i++ {
		go func() {
			_ = clock.Now()
			<-clock.After(1 * time.Millisecond)
			clock.Sleep(1 * time.Millisecond)
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		select {
		case <-done:
			// Success
		case <-time.After(1 * time.Second):
			t.Fatal("Concurrent operations timed out")
		}
	}
}

func TestRealTimeClock_ImplementsClockInterface(t *testing.T) {
	// Compile-time check that RealTimeClock implements Clock
	var _ Clock = (*RealTimeClock)(nil)
}
