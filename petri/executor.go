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
	"errors"
	"fmt"

	"github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/token"
)

// Standard errors for transition firing
var (
	// ErrTransitionNotResolved is returned when attempting to fire an unresolved transition
	ErrTransitionNotResolved = errors.New("transition not resolved")

	// ErrTransitionNotEnabled is returned when attempting to fire a disabled transition
	ErrTransitionNotEnabled = errors.New("transition not enabled")
)

// consumedToken tracks a token that was consumed from a place during firing.
// This information is used for rollback if the transition fails to fire.
type consumedToken struct {
	token *token.Token
	place *Place
	arc   *Arc
}

// Fire executes the transition's task and moves tokens through the Petri net.
//
// The firing process follows these steps:
//  1. Verify transition is resolved (pointers are set up)
//  2. Create trace span for observability
//  3. Check if transition is enabled (sufficient tokens + guard passes)
//  4. Consume input tokens from all input places (based on arc weights)
//  5. Execute the transition's task (if present)
//  6. Produce output tokens to all output places (based on arc weights and expressions)
//  7. Record metrics and logs
//
// If the task execution fails, consumed tokens are rolled back to their original places.
// If token production fails, the error is recorded but tokens cannot be rolled back
// (since the task has already executed).
//
// IMPORTANT - Trigger Behavior:
// This method does NOT check triggers. It only verifies:
//   - Transition is resolved
//   - Transition is enabled (tokens + guard)
//
// Trigger checking (timing/approval/events) is handled by the Engine in
// findEnabledTransitions(). This design allows:
//   - Direct firing for testing/debugging (bypass triggers)
//   - Force-firing in manual workflows
//   - Faster execution when called by engine (no redundant trigger check)
//
// If you need trigger-aware execution, use the Engine instead of calling Fire() directly.
// The engine will check Trigger.ShouldFire() and spawn goroutines for Trigger.AwaitFiring()
// before calling FireUnchecked().
//
// Thread Safety:
// Fire can be called concurrently from multiple goroutines. The Place token storage
// (channels) provides thread-safe token operations. Multiple transitions can fire
// concurrently as long as they have sufficient input tokens.
//
// Parameters:
//   - ctx: ExecutionContext with observability and error handling
//
// Returns an error if:
//   - Transition is not resolved
//   - Transition is not enabled
//   - Token consumption fails
//   - Task execution fails (with rollback)
//   - Token production fails (without rollback)
//
// This method still checks IsEnabled() for safety when called
// directly (e.g., in tests or manual firing). The engine uses FireUnchecked() to
// avoid redundant guard evaluation.
func (t *Transition) Fire(ctx *context.ExecutionContext) error {
	// Lock to prevent concurrent firing of the same transition
	t.fireMu.Lock()
	defer t.fireMu.Unlock()

	// 1. Verify transition is resolved
	if !t.resolved {
		return fmt.Errorf("%w: %s", ErrTransitionNotResolved, t.ID)
	}

	// 2. Create trace span
	span := ctx.Tracer.StartSpan("transition.fire." + t.ID)
	defer span.End()

	// 3. Check if enabled (includes guard evaluation)
	enabled, err := t.IsEnabled(ctx)
	if err != nil {
		ctx.Metrics.Inc("transition_guard_error_total")
		return fmt.Errorf("guard evaluation error for transition %s: %w", t.ID, err)
	}
	if !enabled {
		ctx.Metrics.Inc("transition_not_enabled_total")
		return fmt.Errorf("%w: %s", ErrTransitionNotEnabled, t.ID)
	}

	// Delegate to internal implementation
	return t.fireInternal(ctx, span)
}

// FireUnchecked executes the transition without checking IsEnabled().
// This is used by the engine which has already verified enablement,
// avoiding wasteful double guard evaluation.
//
// Wasteful double IsEnabled() check
// The engine calls IsEnabled() to find enabled transitions, then Fire()
// calls it again. For expensive guards (DB queries, API calls), this doubles
// execution time. This method skips the redundant check.
//
// IMPORTANT - When This Is Called:
// The Engine calls this method AFTER it has verified:
//   - Structural enablement (sufficient tokens in input places)
//   - Guard passes (business logic returns true)
//   - Trigger is ready (Trigger.ShouldFire() returned true, or AwaitFiring() completed)
//
// This method skips all validation for performance, assuming the caller has already
// verified the transition is ready to fire. It proceeds directly to:
//   1. Consuming tokens
//   2. Executing task
//   3. Producing output tokens
//
// Only call this if you've already verified IsEnabled() returns true AND the trigger
// is ready. Calling this on a disabled transition will cause token consumption to fail.
//
// Thread Safety: Same as Fire() - safe for concurrent calls.
//
// This method is exported for use by the engine package. Applications should
// use Fire() instead, which includes safety checks.
func (t *Transition) FireUnchecked(ctx *context.ExecutionContext) error {
	// Lock to prevent concurrent firing of the same transition
	t.fireMu.Lock()
	defer t.fireMu.Unlock()

	// 1. Verify transition is resolved
	if !t.resolved {
		return fmt.Errorf("%w: %s", ErrTransitionNotResolved, t.ID)
	}

	// 2. Create trace span
	span := ctx.Tracer.StartSpan("transition.fire." + t.ID)
	defer span.End()

	// Skip IsEnabled() check - caller already verified

	// Delegate to internal implementation
	return t.fireInternal(ctx, span)
}

// fireInternal contains the core firing logic shared by Fire() and fireUnchecked().
// This avoids code duplication between the two methods.
func (t *Transition) fireInternal(ctx *context.ExecutionContext, span context.Span) error {

	// Record transition fired event (before execution)
	if ctx.EventLog != nil && ctx.EventBuilder != nil {
		event := ctx.EventBuilder.NewEvent(context.EventTransitionFired, t.netID)
		t.setEventFields(event, t.ID, t.Name, "")
		ctx.EventLog.Append(event)
	}

	// 4. Consume input tokens (collect for processing)
	inputTokens, err := t.consumeInputTokens()
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to consume tokens: %w", err)
	}

	// 5. Execute task (if present)
	if t.Task != nil {
		// Create context with first input token (or empty token)
		taskCtx := ctx
		if len(inputTokens) > 0 {
			taskCtx = ctx.WithToken(inputTokens[0].token)
		}

		if err := t.Task.Execute(taskCtx); err != nil {
			// Record transition failed event
			if ctx.EventLog != nil && ctx.EventBuilder != nil {
				event := ctx.EventBuilder.NewEvent(context.EventTransitionFailed, t.netID)
				t.setEventFields(event, t.ID, t.Name, err.Error())
				ctx.EventLog.Append(event)
			}

			// ROLLBACK: Return consumed tokens to input places
			// Check for rollback errors
			if rollbackErr := t.rollbackTokens(inputTokens); rollbackErr != nil {
				// CRITICAL: Rollback failed - tokens may be lost
				// Log the error with full context
				ctx.Logger.Error("CRITICAL: token rollback failed after task error", map[string]interface{}{
					"net_id":          t.netID,
					"transition_id":   t.ID,
					"transition_name": t.Name,
					"operation":       "rollback",
					"task_error":      err.Error(),
					"rollback_error":  rollbackErr.Error(),
				})

				// Report to error handler if available
				if ctx.ErrorHandler != nil {
					ctx.ErrorHandler.Handle(ctx, fmt.Errorf("CRITICAL: rollback failed for transition %s: %w", t.ID, rollbackErr))
				}

				// Return combined error indicating both task failure and rollback failure
				span.RecordError(rollbackErr)
				ctx.Metrics.Inc("transition_fire_errors_total")
				ctx.Metrics.Inc("transition_rollback_errors_total")
				return fmt.Errorf("task execution failed: %w, and rollback failed: %v", err, rollbackErr)
			}

			span.RecordError(err)
			ctx.Metrics.Inc("transition_fire_errors_total")
			return fmt.Errorf("task execution failed: %w", err)
		}
	}

	// 6. Produce output tokens (apply arc expressions)
	if err := t.produceOutputTokens(inputTokens, ctx); err != nil {
		// Cannot rollback after task executed
		span.RecordError(err)
		ctx.ErrorHandler.Handle(ctx, err)
		ctx.Metrics.Inc("transition_fire_errors_total")
		return fmt.Errorf("failed to produce tokens: %w", err)
	}

	// 7. Record successful firing
	ctx.Metrics.Inc("transition_fire_total")
	ctx.Logger.Info("transition fired", map[string]interface{}{
		"net_id":          t.netID,
		"transition_id":   t.ID,
		"transition_name": t.Name,
		"operation":       "fire",
	})

	return nil
}

// consumeInputTokens removes tokens from input places based on arc weights.
// Returns a slice of consumedToken structs that track which token came from which place.
// This information is used for rollback if the transition fails to fire.
//
// If consumption fails mid-way (insufficient tokens), all already-consumed tokens
// are rolled back to their original places.
func (t *Transition) consumeInputTokens() ([]consumedToken, error) {
	var consumed []consumedToken

	for _, arc := range t.inputArcs {
		place := arc.Source.(*Place)

		// Consume arc.Weight tokens from this place
		for i := 0; i < arc.Weight; i++ {
			tok, err := place.RemoveToken()
			if err != nil {
				// Rollback already consumed tokens
				// Check rollback errors during consumption failure
				if rollbackErr := t.rollbackTokens(consumed); rollbackErr != nil {
					// Double failure: couldn't consume AND couldn't rollback
					return nil, fmt.Errorf("failed to consume token from place %s: %w, and rollback also failed: %v", place.ID, err, rollbackErr)
				}
				return nil, fmt.Errorf("failed to consume token from place %s: %w", place.ID, err)
			}
			consumed = append(consumed, consumedToken{
				token: tok,
				place: place,
				arc:   arc,
			})
		}
	}

	return consumed, nil
}

// rollbackTokens returns consumed tokens to their original input places.
// This is called when task execution fails to restore the Petri net to its
// original state before the firing attempt.
//
// Tokens are returned in reverse order to maintain FIFO semantics as much as possible.
//
// Token Rollback Ignores AddToken Errors
// Previously, errors from AddToken() during rollback were silently ignored, potentially
// causing token loss. Now we detect and return rollback errors so they can be handled
// appropriately by the caller.
//
// Returns an error if any token cannot be returned to its place (e.g., place is at capacity).
// In case of error, the rollback is partially completed and tokens may be in an inconsistent
// state. The caller should log this as a critical error.
func (t *Transition) rollbackTokens(consumed []consumedToken) error {
	var firstError error

	// Attempt to return all tokens, even if some fail
	// We track the first error but continue trying to rollback remaining tokens
	for i := len(consumed) - 1; i >= 0; i-- {
		if err := consumed[i].place.AddToken(consumed[i].token); err != nil {
			if firstError == nil {
				// Record first error with context
				firstError = fmt.Errorf("rollback failed for place %s (token %s): %w",
					consumed[i].place.ID, consumed[i].token.ID, err)
			}
			// Continue trying to rollback remaining tokens
		}
	}

	return firstError
}

// produceOutputTokens creates tokens for output places based on arc weights and expressions.
// Each output arc produces arc.Weight tokens. If the arc has an expression, it's applied
// to transform the first input token. If no expression is set, the first input token is
// used directly. If there are no input tokens, an empty token is created.
//
// Parameters:
//   - inputTokens: Tokens that were consumed from input places
//   - ctx: ExecutionContext for generating new token IDs
//
// Returns an error if any output place is at capacity and cannot accept tokens.
func (t *Transition) produceOutputTokens(inputTokens []consumedToken, ctx *context.ExecutionContext) error {
	for _, arc := range t.outputArcs {
		place := arc.Target.(*Place)

		// Produce arc.Weight tokens to this place
		for i := 0; i < arc.Weight; i++ {
			var outputToken *token.Token

			// Apply arc expression if present
			if arc.Expression != nil && len(inputTokens) > 0 {
				outputToken = arc.Expression(inputTokens[0].token)
			} else if len(inputTokens) > 0 {
				// No expression, use first input token
				outputToken = inputTokens[0].token
			} else {
				// No input tokens, create empty token
				// Use atomic counter-based ID generation
				outputToken = &token.Token{
					ID:   token.GenerateTokenID(),
					Data: &MapToken{Value: make(map[string]interface{})},
				}
			}

			if err := place.AddToken(outputToken); err != nil {
				return fmt.Errorf("failed to add token to place %s: %w", place.ID, err)
			}
		}
	}

	return nil
}

// setEventFields is a helper to set fields on events created by EventBuilder.
// It uses type assertion to set fields without creating a circular dependency.
func (t *Transition) setEventFields(event interface{}, transitionID, transitionName, errorMsg string) {
	// Try to set fields using type assertion
	// This works because state.Event has these fields
	type eventSetter interface {
		SetTransitionID(id string)
		SetError(err string)
		SetMetadata(m map[string]interface{})
	}

	if e, ok := event.(eventSetter); ok {
		e.SetTransitionID(transitionID)
		if errorMsg != "" {
			e.SetError(errorMsg)
		}
		e.SetMetadata(map[string]interface{}{
			"transition_name": transitionName,
		})
	}
}

// BatchFire executes the transition multiple times up to batchSize.
// Batch token processing for improved performance.
//
// This method is useful for:
//   - Processing multiple independent items in a batch
//   - Reducing function call overhead for high-throughput workflows
//   - Batch task execution (e.g., bulk database inserts)
//
// Each fire is atomic - if a fire fails, tokens are rolled back for that
// specific fire, but previous fires remain committed. The method continues
// firing until either:
//   - batchSize fires have succeeded
//   - The transition is no longer enabled
//   - A fire fails
//
// Thread Safety:
// BatchFire can be called concurrently. However, the batch is processed
// sequentially within this call. Concurrent BatchFire calls will compete
// for tokens.
//
// Parameters:
//   - ctx: ExecutionContext with observability and error handling
//   - batchSize: Maximum number of times to fire the transition
//
// Returns:
//   - int: Number of successful fires
//   - error: Error from the first failed fire, or nil if all succeeded
//
// Example:
//
//	fired, err := transition.BatchFire(ctx, 10)
//	if err != nil {
//	    // Some fires failed, but 'fired' successful fires were committed
//	    log.Printf("Batch fired %d times before error: %v", fired, err)
//	}
func (t *Transition) BatchFire(ctx *context.ExecutionContext, batchSize int) (int, error) {
	if batchSize <= 0 {
		return 0, fmt.Errorf("batch size must be positive, got %d", batchSize)
	}

	// Create trace span for batch operation
	span := ctx.Tracer.StartSpan("transition.batch_fire." + t.ID)
	defer span.End()
	span.SetAttribute("batch_size", batchSize)

	successCount := 0

	// Fire up to batchSize times
	for i := 0; i < batchSize; i++ {
		// Check if transition is still enabled
		enabled, err := t.IsEnabled(ctx)
		if err != nil {
			// Guard error - stop batch and return error
			ctx.Metrics.Inc("transition_guard_error_total")
			return successCount, fmt.Errorf("guard evaluation error during batch firing: %w", err)
		}
		if !enabled {
			// No more tokens available - stop batch
			break
		}

		// Fire once using the unchecked variant (we already checked IsEnabled)
		if err := t.FireUnchecked(ctx); err != nil {
			// Fire failed - stop batch and return error
			span.SetAttribute("fired_count", successCount)
			span.SetAttribute("failed_at", i)
			span.RecordError(err)
			ctx.Metrics.Inc("batch_fire_partial_total")
			return successCount, fmt.Errorf("batch fire failed at iteration %d: %w", i, err)
		}

		successCount++
	}

	// All requested fires succeeded (or stopped early due to no tokens)
	span.SetAttribute("fired_count", successCount)
	ctx.Metrics.Inc("batch_fire_total")
	ctx.Metrics.Add("batch_fire_count", float64(successCount))

	if successCount < batchSize {
		// Stopped early due to no more tokens - this is not an error
		ctx.Logger.Debug("batch fire completed with fewer fires than requested", map[string]interface{}{
			"net_id":          t.netID,
			"transition_id":   t.ID,
			"transition_name": t.Name,
			"operation":       "batch_fire",
			"requested":       batchSize,
			"actual":          successCount,
			"reason":          "transition not enabled",
		})
	} else {
		ctx.Logger.Debug("batch fire completed", map[string]interface{}{
			"net_id":          t.netID,
			"transition_id":   t.ID,
			"transition_name": t.Name,
			"operation":       "batch_fire",
			"fired":           successCount,
		})
	}

	return successCount, nil
}

// MapToken is a simple map-based token data type for testing and simple workflows.
type MapToken struct {
	Value map[string]interface{}
}

// Type returns the type identifier for MapToken.
func (m *MapToken) Type() string {
	return "map"
}
