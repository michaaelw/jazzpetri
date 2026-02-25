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
	"testing"
	"time"
)

func TestNewVirtualClock(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := NewVirtualClock(start)

	if clock == nil {
		t.Fatal("NewVirtualClock() returned nil")
	}

	if !clock.Now().Equal(start) {
		t.Errorf("clock.Now() = %v, expected %v", clock.Now(), start)
	}
}

func TestVirtualClock_Now(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := NewVirtualClock(start)

	// Initially should return start time
	if !clock.Now().Equal(start) {
		t.Errorf("initial Now() = %v, expected %v", clock.Now(), start)
	}

	// Should not advance without explicit call
	time.Sleep(10 * time.Millisecond)
	if !clock.Now().Equal(start) {
		t.Errorf("Now() after sleep = %v, expected %v (time should not auto-advance)", clock.Now(), start)
	}
}

func TestVirtualClock_AdvanceTo(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := NewVirtualClock(start)

	// Advance to a future time
	future := start.Add(1 * time.Hour)
	clock.AdvanceTo(future)

	if !clock.Now().Equal(future) {
		t.Errorf("Now() after AdvanceTo = %v, expected %v", clock.Now(), future)
	}

	// Advancing to past should be no-op
	past := start.Add(30 * time.Minute)
	clock.AdvanceTo(past)

	if !clock.Now().Equal(future) {
		t.Errorf("Now() after AdvanceTo(past) = %v, expected %v (should not go backward)", clock.Now(), future)
	}

	// Advancing to same time should be no-op
	clock.AdvanceTo(future)
	if !clock.Now().Equal(future) {
		t.Errorf("Now() after AdvanceTo(same) = %v, expected %v", clock.Now(), future)
	}
}

func TestVirtualClock_AdvanceBy(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := NewVirtualClock(start)

	// Advance by positive duration
	clock.AdvanceBy(1 * time.Hour)
	expected := start.Add(1 * time.Hour)

	if !clock.Now().Equal(expected) {
		t.Errorf("Now() after AdvanceBy(1h) = %v, expected %v", clock.Now(), expected)
	}

	// Advance by another duration
	clock.AdvanceBy(30 * time.Minute)
	expected = expected.Add(30 * time.Minute)

	if !clock.Now().Equal(expected) {
		t.Errorf("Now() after AdvanceBy(30m) = %v, expected %v", clock.Now(), expected)
	}

	// Advance by zero should be no-op
	clock.AdvanceBy(0)
	if !clock.Now().Equal(expected) {
		t.Errorf("Now() after AdvanceBy(0) = %v, expected %v", clock.Now(), expected)
	}

	// Advance by negative should be no-op
	clock.AdvanceBy(-1 * time.Hour)
	if !clock.Now().Equal(expected) {
		t.Errorf("Now() after AdvanceBy(-1h) = %v, expected %v (should not go backward)", clock.Now(), expected)
	}
}

func TestVirtualClock_After_FiresWhenTimeAdvances(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := NewVirtualClock(start)

	// Create timer for 5 seconds in future
	ch := clock.After(5 * time.Second)

	// Timer should not fire immediately
	select {
	case <-ch:
		t.Fatal("After() fired before time advanced")
	default:
		// Expected
	}

	// Advance time by 3 seconds (not enough)
	clock.AdvanceBy(3 * time.Second)

	select {
	case <-ch:
		t.Fatal("After() fired too early (at 3s instead of 5s)")
	default:
		// Expected
	}

	// Advance time by 2 more seconds (total 5 seconds)
	clock.AdvanceBy(2 * time.Second)

	// Now timer should fire
	receivedTime, ok := <-ch
	if !ok {
		t.Fatal("After() channel was closed without sending time")
	}

	expectedTime := start.Add(5 * time.Second)
	if !receivedTime.Equal(expectedTime) {
		t.Errorf("After() received time = %v, expected %v", receivedTime, expectedTime)
	}

	// Channel should be closed after firing
	_, ok = <-ch
	if ok {
		t.Fatal("After() channel should be closed after firing")
	}
}

func TestVirtualClock_After_FiresImmediatelyIfPastDeadline(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := NewVirtualClock(start)

	// Advance time first
	clock.AdvanceBy(10 * time.Second)

	// Create timer for past deadline (0 seconds = immediately)
	ch := clock.After(0)

	// Should fire immediately
	select {
	case receivedTime := <-ch:
		expectedTime := start.Add(10 * time.Second)
		if !receivedTime.Equal(expectedTime) {
			t.Errorf("After(0) received time = %v, expected %v", receivedTime, expectedTime)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("After(0) did not fire immediately")
	}
}

func TestVirtualClock_After_MultipleTimers(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := NewVirtualClock(start)

	// Create multiple timers with different deadlines
	timer1 := clock.After(1 * time.Second)
	timer2 := clock.After(2 * time.Second)
	timer3 := clock.After(3 * time.Second)

	// None should fire yet
	select {
	case <-timer1:
		t.Fatal("timer1 fired too early")
	case <-timer2:
		t.Fatal("timer2 fired too early")
	case <-timer3:
		t.Fatal("timer3 fired too early")
	default:
		// Expected
	}

	// Advance by 1.5 seconds - only timer1 should fire
	clock.AdvanceBy(1500 * time.Millisecond)

	select {
	case <-timer1:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timer1 did not fire")
	}

	select {
	case <-timer2:
		t.Fatal("timer2 fired too early")
	case <-timer3:
		t.Fatal("timer3 fired too early")
	default:
		// Expected
	}

	// Advance by 1 more second (total 2.5s) - timer2 should fire
	clock.AdvanceBy(1 * time.Second)

	select {
	case <-timer2:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timer2 did not fire")
	}

	select {
	case <-timer3:
		t.Fatal("timer3 fired too early")
	default:
		// Expected
	}

	// Advance by 1 more second (total 3.5s) - timer3 should fire
	clock.AdvanceBy(1 * time.Second)

	select {
	case <-timer3:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timer3 did not fire")
	}
}

func TestVirtualClock_Sleep(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := NewVirtualClock(start)

	sleepDone := make(chan bool)
	sleepStarted := make(chan bool)

	// Start goroutine that will sleep
	go func() {
		sleepStarted <- true
		clock.Sleep(5 * time.Second)
		sleepDone <- true
	}()

	// Wait for sleep to start
	<-sleepStarted
	time.Sleep(10 * time.Millisecond) // Give Sleep() time to register

	// Sleep should not complete yet
	select {
	case <-sleepDone:
		t.Fatal("Sleep() returned before time advanced")
	case <-time.After(50 * time.Millisecond):
		// Expected
	}

	// Advance time by 3 seconds (not enough)
	clock.AdvanceBy(3 * time.Second)

	select {
	case <-sleepDone:
		t.Fatal("Sleep() returned too early (at 3s instead of 5s)")
	case <-time.After(50 * time.Millisecond):
		// Expected
	}

	// Advance time by 2 more seconds (total 5 seconds)
	clock.AdvanceBy(2 * time.Second)

	// Sleep should complete now
	select {
	case <-sleepDone:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Sleep() did not return after time advanced")
	}
}

func TestVirtualClock_Sleep_ZeroOrNegative(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := NewVirtualClock(start)

	// Sleep(0) should return immediately
	clock.Sleep(0)

	// Sleep(negative) should return immediately
	clock.Sleep(-1 * time.Second)

	// If we get here, the test passed (didn't block)
}

func TestVirtualClock_Sleep_MultipleSleeps(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := NewVirtualClock(start)

	done1 := make(chan bool)
	done2 := make(chan bool)
	done3 := make(chan bool)

	// Start multiple sleeps with different durations
	go func() {
		clock.Sleep(1 * time.Second)
		done1 <- true
	}()
	go func() {
		clock.Sleep(2 * time.Second)
		done2 <- true
	}()
	go func() {
		clock.Sleep(3 * time.Second)
		done3 <- true
	}()

	// Give sleeps time to register
	time.Sleep(50 * time.Millisecond)

	// Advance by 1.5 seconds - only first sleep should complete
	clock.AdvanceBy(1500 * time.Millisecond)

	select {
	case <-done1:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("first Sleep(1s) did not complete")
	}

	select {
	case <-done2:
		t.Fatal("second Sleep(2s) completed too early")
	case <-done3:
		t.Fatal("third Sleep(3s) completed too early")
	case <-time.After(50 * time.Millisecond):
		// Expected
	}

	// Advance by 1 more second - second sleep should complete
	clock.AdvanceBy(1 * time.Second)

	select {
	case <-done2:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("second Sleep(2s) did not complete")
	}

	// Advance by 1 more second - third sleep should complete
	clock.AdvanceBy(1 * time.Second)

	select {
	case <-done3:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("third Sleep(3s) did not complete")
	}
}

func TestVirtualClock_ConcurrentUse(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := NewVirtualClock(start)

	var wg sync.WaitGroup

	// Create multiple timers concurrently
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ch := clock.After(time.Duration(id) * time.Second)
			<-ch
		}(i)
	}

	// Give goroutines time to register timers
	time.Sleep(50 * time.Millisecond)

	// Advance time to fire all timers
	clock.AdvanceBy(20 * time.Second)

	// Wait for all goroutines to complete
	done := make(chan bool)
	go func() {
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("Concurrent timer operations timed out")
	}
}

func TestVirtualClock_PendingTimers(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := NewVirtualClock(start)

	if clock.PendingTimers() != 0 {
		t.Errorf("initial PendingTimers() = %d, expected 0", clock.PendingTimers())
	}

	// Create some timers
	clock.After(1 * time.Second)
	clock.After(2 * time.Second)
	clock.After(3 * time.Second)

	if clock.PendingTimers() != 3 {
		t.Errorf("PendingTimers() after creating 3 = %d, expected 3", clock.PendingTimers())
	}

	// Fire one timer
	clock.AdvanceBy(1500 * time.Millisecond)

	if clock.PendingTimers() != 2 {
		t.Errorf("PendingTimers() after firing 1 = %d, expected 2", clock.PendingTimers())
	}

	// Fire remaining timers
	clock.AdvanceBy(2 * time.Second)

	if clock.PendingTimers() != 0 {
		t.Errorf("PendingTimers() after firing all = %d, expected 0", clock.PendingTimers())
	}
}

func TestVirtualClock_PendingSleeps(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := NewVirtualClock(start)

	if clock.PendingSleeps() != 0 {
		t.Errorf("initial PendingSleeps() = %d, expected 0", clock.PendingSleeps())
	}

	// Start some sleeps
	for i := 0; i < 3; i++ {
		go clock.Sleep(time.Duration(i+1) * time.Second)
	}

	// Give sleeps time to register
	time.Sleep(50 * time.Millisecond)

	if clock.PendingSleeps() != 3 {
		t.Errorf("PendingSleeps() after starting 3 = %d, expected 3", clock.PendingSleeps())
	}

	// Fire one sleep
	clock.AdvanceBy(1500 * time.Millisecond)
	time.Sleep(10 * time.Millisecond) // Give goroutine time to exit

	if clock.PendingSleeps() != 2 {
		t.Errorf("PendingSleeps() after firing 1 = %d, expected 2", clock.PendingSleeps())
	}

	// Fire remaining sleeps
	clock.AdvanceBy(2 * time.Second)
	time.Sleep(10 * time.Millisecond) // Give goroutines time to exit

	if clock.PendingSleeps() != 0 {
		t.Errorf("PendingSleeps() after firing all = %d, expected 0", clock.PendingSleeps())
	}
}

func TestVirtualClock_ImplementsClockInterface(t *testing.T) {
	// Compile-time check that VirtualClock implements Clock
	var _ Clock = (*VirtualClock)(nil)
}

func TestVirtualClock_DeterministicBehavior(t *testing.T) {
	// Virtual clock should be deterministic - same sequence of operations
	// should produce same results

	runTest := func() []time.Time {
		start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		clock := NewVirtualClock(start)

		results := make([]time.Time, 0)

		// Create timers
		t1 := clock.After(1 * time.Second)
		t2 := clock.After(2 * time.Second)

		// Advance and collect results
		clock.AdvanceBy(1 * time.Second)
		results = append(results, <-t1)

		clock.AdvanceBy(1 * time.Second)
		results = append(results, <-t2)

		return results
	}

	// Run test twice
	results1 := runTest()
	results2 := runTest()

	// Results should be identical
	if len(results1) != len(results2) {
		t.Fatal("Deterministic test produced different result counts")
	}

	for i := range results1 {
		if !results1[i].Equal(results2[i]) {
			t.Errorf("Result %d differs: run1=%v, run2=%v", i, results1[i], results2[i])
		}
	}
}
