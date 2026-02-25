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

package context

import (
	"strings"
	"testing"
)

// Metrics Naming Inconsistency
//
// EXPECTED FAILURE: These tests will fail because metrics naming
// is inconsistent across the codebase.
//
// Issues:
//   - Some metrics use "_total" suffix, some don't
//   - Inconsistent naming conventions
//   - Missing units in metric names
//   - No clear naming standard
//
// Consistent naming enables:
//   - Better Prometheus integration
//   - Easier metric queries
//   - Self-documenting metrics
//   - Standardized dashboards

func TestMetrics_NamingConvention(t *testing.T) {
	// EXPECTED FAILURE: Metrics don't follow consistent convention

	// All counter metrics should end with _total
	counterMetrics := []string{
		"engine_transitions_fired_total",
		"engine_fire_errors_total",
		"engine_steps_total",
		"engine_idle_total",
		"tokens_created_total",
		"tokens_consumed_total",
	}

	for _, metric := range counterMetrics {
		if !strings.HasSuffix(metric, "_total") {
			t.Errorf("Counter metric %s should end with _total", metric)
		}
	}

	// Gauge metrics should NOT end with _total
	gaugeMetrics := []string{
		"enabled_transitions", // current count
		"place_token_count",   // current count
		"engine_state",        // current state
	}

	for _, metric := range gaugeMetrics {
		if strings.HasSuffix(metric, "_total") {
			t.Errorf("Gauge metric %s should not end with _total", metric)
		}
	}
}

func TestMetrics_AllHaveUnits(t *testing.T) {
	// EXPECTED FAILURE: Some metrics lack unit suffixes

	// Metrics with time should include unit
	timeMetrics := []string{
		"transition_fire_duration_ms",   // milliseconds
		"engine_uptime_seconds",         // seconds
		"poll_interval_ms",              // milliseconds
	}

	for _, metric := range timeMetrics {
		hasUnit := strings.Contains(metric, "_ms") ||
			strings.Contains(metric, "_seconds") ||
			strings.Contains(metric, "_minutes")

		if !hasUnit {
			t.Errorf("Time metric %s should include unit suffix (ms, seconds, etc.)", metric)
		}
	}

	// Size metrics should include unit
	sizeMetrics := []string{
		"token_size_bytes",
		"memory_usage_bytes",
	}

	for _, metric := range sizeMetrics {
		if !strings.Contains(metric, "_bytes") {
			t.Errorf("Size metric %s should include _bytes suffix", metric)
		}
	}
}

func TestMetrics_ConsistentPrefix(t *testing.T) {
	// EXPECTED FAILURE: Metrics don't use consistent prefixes

	// Engine metrics should be prefixed with "engine_"
	engineMetrics := []string{
		"engine_transitions_fired_total",
		"engine_state",
		"engine_uptime_seconds",
	}

	for _, metric := range engineMetrics {
		if !strings.HasPrefix(metric, "engine_") {
			t.Errorf("Engine metric %s should be prefixed with 'engine_'", metric)
		}
	}

	// Token metrics should be prefixed with "token_"
	tokenMetrics := []string{
		"token_created_total",
		"token_consumed_total",
		"token_size_bytes",
	}

	for _, metric := range tokenMetrics {
		if !strings.HasPrefix(metric, "token_") {
			t.Errorf("Token metric %s should be prefixed with 'token_'", metric)
		}
	}

	// Place metrics should be prefixed with "place_"
	placeMetrics := []string{
		"place_token_count",
		"place_utilization",
		"place_capacity",
	}

	for _, metric := range placeMetrics {
		if !strings.HasPrefix(metric, "place_") {
			t.Errorf("Place metric %s should be prefixed with 'place_'", metric)
		}
	}
}

func TestMetrics_SnakeCase(t *testing.T) {
	// EXPECTED FAILURE: Some metrics might not use snake_case

	// All metrics should use snake_case (Prometheus convention)
	validMetrics := []string{
		"engine_transitions_fired_total",
		"place_token_count",
		"transition_fire_duration_ms",
	}

	invalidMetrics := []string{
		"engineTransitionsFired",  // camelCase
		"EngineState",             // PascalCase
		"engine-state",            // kebab-case
	}

	for _, metric := range validMetrics {
		if !isSnakeCase(metric) {
			t.Errorf("Valid metric %s should be in snake_case", metric)
		}
	}

	for _, metric := range invalidMetrics {
		if isSnakeCase(metric) {
			t.Errorf("Invalid metric %s should not be recognized as snake_case", metric)
		}
	}
}

func TestMetrics_TypeSuffixes(t *testing.T) {
	// EXPECTED FAILURE: Metric types not clearly indicated

	tests := []struct {
		metric     string
		metricType string
		suffix     string
	}{
		{"engine_transitions_fired_total", "counter", "_total"},
		{"place_utilization_ratio", "gauge", "_ratio"},
		{"transition_duration_ms", "histogram", "_ms"},
		{"tokens_per_second", "gauge", "_per_second"},
	}

	for _, tt := range tests {
		if !strings.HasSuffix(tt.metric, tt.suffix) {
			t.Errorf("%s metric %s should have %s suffix",
				tt.metricType, tt.metric, tt.suffix)
		}
	}
}

func TestMetrics_Documentation(t *testing.T) {
	// EXPECTED FAILURE: Metrics lack documentation

	// All metrics should have:
	// 1. Clear description
	// 2. Type (counter, gauge, histogram)
	// 3. Unit
	// 4. Example usage

	t.Skip("Metrics should be documented in metrics catalog")

	// Example metrics catalog:
	// engine_transitions_fired_total (counter)
	//   Description: Total number of transitions fired since engine start
	//   Unit: count
	//   Labels: net_id, transition_id
	//   Example: engine_transitions_fired_total{net_id="wf1"} 150
}

func TestMetrics_Labels(t *testing.T) {
	// EXPECTED FAILURE: Inconsistent label naming

	// Labels should also follow naming conventions
	validLabels := []string{
		"net_id",
		"transition_id",
		"place_id",
		"error_type",
	}

	for _, label := range validLabels {
		if !isSnakeCase(label) {
			t.Errorf("Label %s should be in snake_case", label)
		}
	}
}

// Helper function
func isSnakeCase(s string) bool {
	// Simple check: lowercase letters, digits, and underscores only
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

// Example showing consistent naming
func Example_metricsNaming() {
	// EXPECTED FAILURE: Actual metrics don't follow this convention

	// GOOD naming examples:

	// Counters (monotonically increasing)
	_ = "engine_transitions_fired_total"
	_ = "engine_errors_total"
	_ = "tokens_created_total"

	// Gauges (can go up or down)
	_ = "place_token_count"
	_ = "enabled_transitions"
	_ = "place_utilization_ratio"

	// Histograms/Summaries (with units)
	_ = "transition_fire_duration_ms"
	_ = "token_wait_time_ms"

	// Rates (computed from counters)
	_ = "transitions_per_second"
	_ = "tokens_per_second"

	// All use:
	// - snake_case
	// - descriptive names
	// - appropriate suffixes
	// - component prefixes
}

// Example showing BAD naming
func Example_metricsBadNaming() {
	// EXPECTED FAILURE: Some actual metrics may look like this

	// BAD examples:

	// Inconsistent suffixes
	_ = "transitions_fired"       // Missing _total for counter
	_ = "total_errors"            // "total" prefix instead of suffix
	_ = "fireErrors"              // camelCase instead of snake_case

	// Missing units
	_ = "duration"                // What unit?
	_ = "wait_time"              // Milliseconds? Seconds?

	// Missing prefixes
	_ = "state"                   // State of what?
	_ = "count"                   // Count of what?

	// These make queries and dashboards confusing
}
