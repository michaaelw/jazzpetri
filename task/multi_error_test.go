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
	"errors"
	"strings"
	"testing"
)

// TestMultiError_Add tests adding errors to MultiError
func TestMultiError_Add(t *testing.T) {
	// Arrange
	me := &MultiError{}
	err1 := errors.New("error 1")
	err2 := errors.New("error 2")

	// Act
	me.Add(err1)
	me.Add(err2)

	// Assert
	if len(me.Errors) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(me.Errors))
	}
	if me.Errors[0] != err1 {
		t.Errorf("Expected first error to be %v, got %v", err1, me.Errors[0])
	}
	if me.Errors[1] != err2 {
		t.Errorf("Expected second error to be %v, got %v", err2, me.Errors[1])
	}
}

// TestMultiError_Add_Nil tests that nil errors are not added
func TestMultiError_Add_Nil(t *testing.T) {
	// Arrange
	me := &MultiError{}

	// Act
	me.Add(nil)

	// Assert
	if len(me.Errors) != 0 {
		t.Errorf("Expected 0 errors, got %d", len(me.Errors))
	}
}

// TestMultiError_Add_Flatten tests that nested MultiErrors are flattened
func TestMultiError_Add_Flatten(t *testing.T) {
	// Arrange
	me1 := &MultiError{}
	me1.Add(errors.New("error 1"))
	me1.Add(errors.New("error 2"))

	me2 := &MultiError{}
	me2.Add(errors.New("error 3"))

	// Act
	me2.Add(me1)

	// Assert
	if len(me2.Errors) != 3 {
		t.Errorf("Expected 3 errors, got %d", len(me2.Errors))
	}
}

// TestMultiError_HasErrors tests checking if errors exist
func TestMultiError_HasErrors(t *testing.T) {
	tests := []struct {
		name     string
		errors   []error
		expected bool
	}{
		{
			name:     "no errors",
			errors:   []error{},
			expected: false,
		},
		{
			name:     "one error",
			errors:   []error{errors.New("error 1")},
			expected: true,
		},
		{
			name:     "multiple errors",
			errors:   []error{errors.New("error 1"), errors.New("error 2")},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			me := &MultiError{}
			for _, err := range tt.errors {
				me.Add(err)
			}

			// Act
			result := me.HasErrors()

			// Assert
			if result != tt.expected {
				t.Errorf("Expected HasErrors() to be %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestMultiError_Error tests error message formatting
func TestMultiError_Error(t *testing.T) {
	tests := []struct {
		name     string
		errors   []error
		contains []string
	}{
		{
			name:     "no errors",
			errors:   []error{},
			contains: []string{},
		},
		{
			name:     "single error",
			errors:   []error{errors.New("single error")},
			contains: []string{"single error"},
		},
		{
			name:     "multiple errors",
			errors:   []error{errors.New("error 1"), errors.New("error 2")},
			contains: []string{"multiple errors", "error 1", "error 2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			me := &MultiError{}
			for _, err := range tt.errors {
				me.Add(err)
			}

			// Act
			result := me.Error()

			// Assert
			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected error message to contain '%s', got: %s", expected, result)
				}
			}
		})
	}
}

// TestMultiError_ErrorOrNil tests returning nil when no errors
func TestMultiError_ErrorOrNil(t *testing.T) {
	tests := []struct {
		name     string
		errors   []error
		expectNil bool
	}{
		{
			name:     "no errors returns nil",
			errors:   []error{},
			expectNil: true,
		},
		{
			name:     "with errors returns error",
			errors:   []error{errors.New("error 1")},
			expectNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			me := &MultiError{}
			for _, err := range tt.errors {
				me.Add(err)
			}

			// Act
			result := me.ErrorOrNil()

			// Assert
			if tt.expectNil && result != nil {
				t.Errorf("Expected nil, got: %v", result)
			}
			if !tt.expectNil && result == nil {
				t.Error("Expected error, got nil")
			}
		})
	}
}
