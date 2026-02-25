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
	"strings"
)

// MultiError collects multiple errors into a single error.
// This is used by ParallelTask to report errors from multiple concurrent tasks.
//
// MultiError implements the error interface and provides a formatted error message
// that includes all collected errors.
//
// Example:
//
//	me := &MultiError{}
//	me.Add(errors.New("task 1 failed"))
//	me.Add(errors.New("task 2 failed"))
//	if me.HasErrors() {
//	    return me // Returns: "multiple errors: task 1 failed; task 2 failed"
//	}
type MultiError struct {
	// Errors is the list of collected errors
	Errors []error
}

// Add adds an error to the collection.
// If err is nil, it is not added.
// If err is itself a MultiError, its errors are flattened into this collection.
func (m *MultiError) Add(err error) {
	if err == nil {
		return
	}

	// Flatten nested MultiErrors
	if me, ok := err.(*MultiError); ok {
		m.Errors = append(m.Errors, me.Errors...)
		return
	}

	m.Errors = append(m.Errors, err)
}

// HasErrors returns true if any errors have been collected.
func (m *MultiError) HasErrors() bool {
	return len(m.Errors) > 0
}

// Error implements the error interface.
// Returns a formatted string with all error messages.
// Format: "multiple errors: error1; error2; error3"
func (m *MultiError) Error() string {
	if !m.HasErrors() {
		return ""
	}

	if len(m.Errors) == 1 {
		return m.Errors[0].Error()
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("multiple errors (%d): ", len(m.Errors)))

	for i, err := range m.Errors {
		if i > 0 {
			sb.WriteString("; ")
		}
		sb.WriteString(err.Error())
	}

	return sb.String()
}

// ErrorOrNil returns the MultiError if it contains errors, or nil if empty.
// This is useful for returning the error only when it contains actual errors.
//
// Example:
//
//	me := &MultiError{}
//	// ... collect errors ...
//	return me.ErrorOrNil()
func (m *MultiError) ErrorOrNil() error {
	if !m.HasErrors() {
		return nil
	}
	return m
}
