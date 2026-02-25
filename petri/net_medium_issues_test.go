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
	"testing"
)

// ============================================================================
// Get Place/Transition/Arc by ID - Already Fixed, Verify It Works
// ============================================================================

// TestPetriNet_GetPlace_VerifyFixed tests:
// GetPlace() 
//
// THIS SHOULD PASS: GetPlace() already implemented
func TestPetriNet_GetPlace_VerifyFixed(t *testing.T) {
	// Arrange
	net := NewPetriNet("test-net", "Test Net")
	p1 := NewPlace("p1", "Place 1", 10)
	p2 := NewPlace("p2", "Place 2", 10)

	net.AddPlace(p1)
	net.AddPlace(p2)

	// Test 1: Get existing place
	place, err := net.GetPlace("p1")
	if err != nil {
		t.Errorf("GetPlace() failed for existing place: %v", err)
	}
	if place == nil {
		t.Errorf("GetPlace() returned nil for existing place")
	}
	if place != nil && place.ID != "p1" {
		t.Errorf("GetPlace() returned wrong place: expected p1, got %s", place.ID)
	}

	// Test 2: Get non-existent place
	_, err = net.GetPlace("nonexistent")
	if err == nil {
		t.Errorf("GetPlace() should return error for non-existent place")
	}

	// Test 3: Empty ID
	_, err = net.GetPlace("")
	if err == nil {
		t.Errorf("GetPlace() should return error for empty ID")
	}

	// Test 4: Nil net
	var nilNet *PetriNet
	_, err = nilNet.GetPlace("p1")
	if err == nil {
		t.Errorf("GetPlace() should return error for nil net")
	}

	t.Logf("PASS: GetPlace() works correctly")
}

// TestPetriNet_GetTransition_VerifyFixed tests:
// GetTransition() 
//
// THIS SHOULD PASS: GetTransition() already implemented
func TestPetriNet_GetTransition_VerifyFixed(t *testing.T) {
	// Arrange
	net := NewPetriNet("test-net", "Test Net")
	t1 := NewTransition("t1", "Transition 1")
	t2 := NewTransition("t2", "Transition 2")

	net.AddTransition(t1)
	net.AddTransition(t2)

	// Test 1: Get existing transition
	trans, err := net.GetTransition("t1")
	if err != nil {
		t.Errorf("GetTransition() failed for existing transition: %v", err)
	}
	if trans == nil {
		t.Errorf("GetTransition() returned nil for existing transition")
	}
	if trans != nil && trans.ID != "t1" {
		t.Errorf("GetTransition() returned wrong transition: expected t1, got %s", trans.ID)
	}

	// Test 2: Get non-existent transition
	_, err = net.GetTransition("nonexistent")
	if err == nil {
		t.Errorf("GetTransition() should return error for non-existent transition")
	}

	// Test 3: Empty ID
	_, err = net.GetTransition("")
	if err == nil {
		t.Errorf("GetTransition() should return error for empty ID")
	}

	// Test 4: Nil net
	var nilNet *PetriNet
	_, err = nilNet.GetTransition("t1")
	if err == nil {
		t.Errorf("GetTransition() should return error for nil net")
	}

	t.Logf("PASS: GetTransition() works correctly")
}

// TestPetriNet_GetArc_Missing tests:
// GetArc() should exist to retrieve arcs by ID
//
// EXPECTED FAILURE: GetArc() method doesn't exist
// Expected behavior: New GetArc() method similar to GetPlace/GetTransition
func TestPetriNet_GetArc_Missing(t *testing.T) {
	// Arrange
	net := NewPetriNet("test-net", "Test Net")
	p1 := NewPlace("p1", "Place 1", 10)
	t1 := NewTransition("t1", "Transition 1")

	net.AddPlace(p1)
	net.AddTransition(t1)

	arc := NewArc("a1", "p1", "t1", 1)
	net.AddArc(arc)

	// Act & Assert
	// EXPECTED FAILURE: GetArc() method doesn't exist - won't compile
	/*
	// Test 1: Get existing arc
	retrievedArc, err := net.GetArc("a1")
	if err != nil {
		t.Errorf("GetArc() failed for existing arc: %v", err)
	}
	if retrievedArc == nil {
		t.Errorf("GetArc() returned nil for existing arc")
	}
	if retrievedArc != nil && retrievedArc.ID != "a1" {
		t.Errorf("GetArc() returned wrong arc: expected a1, got %s", retrievedArc.ID)
	}

	// Test 2: Get non-existent arc
	_, err = net.GetArc("nonexistent")
	if err == nil {
		t.Errorf("GetArc() should return error for non-existent arc")
	}

	// Test 3: Empty ID
	_, err = net.GetArc("")
	if err == nil {
		t.Errorf("GetArc() should return error for empty ID")
	}

	// Test 4: Nil net
	var nilNet *PetriNet
	_, err = nilNet.GetArc("a1")
	if err == nil {
		t.Errorf("GetArc() should return error for nil net")
	}
	*/

	t.Logf("EXPECTED FAILURE: GetArc() method doesn't exist yet")
	t.Logf("After fix: Implement GetArc(id string) (*Arc, error)")
	t.Logf("Should follow same pattern as GetPlace() and GetTransition()")
}

// ============================================================================
// PetriNet.Resolve() Doesn't Validate Arc Endpoints
// ============================================================================

// TestPetriNet_Resolve_InvalidArcEndpoints tests:
// Resolve() doesn't check if arc endpoints actually exist
//
// EXPECTED FAILURE: Resolve() succeeds with invalid endpoints
// Expected behavior: Resolve() should return error for invalid arcs
func TestPetriNet_Resolve_InvalidArcEndpoints(t *testing.T) {
	testCases := []struct {
		name        string
		setupNet    func() *PetriNet
		shouldError bool
		errorMsg    string
	}{
		{
			name: "arc with non-existent source place",
			setupNet: func() *PetriNet {
				net := NewPetriNet("test", "Test")
				// Add transition but NOT the source place
				t1 := NewTransition("t1", "Trans 1")
				net.AddTransition(t1)
				// Arc from non-existent place
				arc := NewArc("a1", "nonexistent", "t1", 1)
				net.AddArc(arc)
				return net
			},
			shouldError: true,
			errorMsg:    "source does not exist",
		},
		{
			name: "arc with non-existent target transition",
			setupNet: func() *PetriNet {
				net := NewPetriNet("test", "Test")
				// Add place but NOT the target transition
				p1 := NewPlace("p1", "Place 1", 10)
				net.AddPlace(p1)
				// Arc to non-existent transition
				arc := NewArc("a1", "p1", "nonexistent", 1)
				net.AddArc(arc)
				return net
			},
			shouldError: true,
			errorMsg:    "target does not exist",
		},
		{
			name: "arc with both endpoints non-existent",
			setupNet: func() *PetriNet {
				net := NewPetriNet("test", "Test")
				// Arc with both endpoints missing
				arc := NewArc("a1", "nonexistent1", "nonexistent2", 1)
				net.AddArc(arc)
				return net
			},
			shouldError: true,
			errorMsg:    "source does not exist",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			net := tc.setupNet()

			// Act
			err := net.Resolve()

			// Assert
			if tc.shouldError && err == nil {
				t.Errorf("EXPECTED FAILURE: Resolve() should return error for %s", tc.name)
				t.Logf("After fix: Resolve() should validate arc endpoints exist")
			}

			if !tc.shouldError && err != nil {
				t.Errorf("Resolve() returned unexpected error: %v", err)
			}

			if err != nil {
				t.Logf("Error: %v", err)
			}
		})
	}

	t.Logf("EXPECTED FAILURE: Resolve() doesn't validate arc endpoints")
	t.Logf("After fix: Validate() or Resolve() should check arc endpoints exist")
}

// TestPetriNet_Resolve_NilEndpoints tests:
// Arcs with nil source/target pointers
//
// EXPECTED FAILURE: May panic with nil pointers
// Expected behavior: Should return error for nil endpoints
func TestPetriNet_Resolve_NilEndpoints(t *testing.T) {
	// Arrange
	net := NewPetriNet("test", "Test")

	// Create arc directly (bypassing normal construction)
	arc := &Arc{
		ID:       "a1",
		SourceID: "p1",
		TargetID: "t1",
		Weight:   1,
		// Source and Target are nil pointers
	}
	net.Arcs["a1"] = arc

	// Act
	err := net.Resolve()

	// Assert
	// EXPECTED FAILURE: May panic or succeed with nil pointers
	if err == nil {
		t.Logf("EXPECTED FAILURE: Resolve() succeeded with nil arc endpoints")
		t.Logf("After fix: Should return error for nil endpoints")
	}

	t.Logf("After fix: Resolve() should validate arc endpoints are not nil")
}

// ============================================================================
// No Validation for Duplicate IDs
// ============================================================================

// TestPetriNet_AddPlace_DuplicateID tests:
// Can add multiple places with same ID
//
// EXPECTED FAILURE: AddPlace() with duplicate ID may succeed or silently overwrite
// Expected behavior: AddPlace() should return error on duplicate ID
func TestPetriNet_AddPlace_DuplicateID(t *testing.T) {
	// Arrange
	net := NewPetriNet("test", "Test")
	p1 := NewPlace("p1", "Place 1", 10)

	// Add first place
	err := net.AddPlace(p1)
	if err != nil {
		t.Fatalf("First AddPlace() failed: %v", err)
	}

	// Act: Try to add another place with same ID
	p1Duplicate := NewPlace("p1", "Place 1 Duplicate", 20)
	err = net.AddPlace(p1Duplicate)

	// Assert
	if err == nil {
		t.Errorf("EXPECTED FAILURE: AddPlace() should return error for duplicate ID")
		t.Logf("After fix: Return error like 'place p1 already exists'")
	} else {
		t.Logf("PASS: AddPlace() correctly returns error for duplicate: %v", err)
	}

	// Verify first place wasn't overwritten
	place, _ := net.GetPlace("p1")
	if place != nil && place.Name == "Place 1 Duplicate" {
		t.Errorf("UNEXPECTED: Duplicate silently overwrote original place")
	}
}

// TestPetriNet_AddTransition_DuplicateID tests:
// Can add multiple transitions with same ID
//
// EXPECTED FAILURE: AddTransition() with duplicate ID may succeed or silently overwrite
// Expected behavior: AddTransition() should return error on duplicate ID
func TestPetriNet_AddTransition_DuplicateID(t *testing.T) {
	// Arrange
	net := NewPetriNet("test", "Test")
	t1 := NewTransition("t1", "Transition 1")

	// Add first transition
	err := net.AddTransition(t1)
	if err != nil {
		t.Fatalf("First AddTransition() failed: %v", err)
	}

	// Act: Try to add another transition with same ID
	t1Duplicate := NewTransition("t1", "Transition 1 Duplicate")
	err = net.AddTransition(t1Duplicate)

	// Assert
	if err == nil {
		t.Errorf("EXPECTED FAILURE: AddTransition() should return error for duplicate ID")
		t.Logf("After fix: Return error like 'transition t1 already exists'")
	} else {
		t.Logf("PASS: AddTransition() correctly returns error for duplicate: %v", err)
	}

	// Verify first transition wasn't overwritten
	trans, _ := net.GetTransition("t1")
	if trans != nil && trans.Name == "Transition 1 Duplicate" {
		t.Errorf("UNEXPECTED: Duplicate silently overwrote original transition")
	}
}

// TestPetriNet_AddArc_DuplicateID tests:
// Can add multiple arcs with same ID
//
// EXPECTED FAILURE: AddArc() with duplicate ID may succeed or silently overwrite
// Expected behavior: AddArc() should return error on duplicate ID
func TestPetriNet_AddArc_DuplicateID(t *testing.T) {
	// Arrange
	net := NewPetriNet("test", "Test")
	p1 := NewPlace("p1", "Place 1", 10)
	p2 := NewPlace("p2", "Place 2", 10)
	t1 := NewTransition("t1", "Transition 1")

	net.AddPlace(p1)
	net.AddPlace(p2)
	net.AddTransition(t1)

	// Add first arc
	arc1 := NewArc("a1", "p1", "t1", 1)
	err := net.AddArc(arc1)
	if err != nil {
		t.Fatalf("First AddArc() failed: %v", err)
	}

	// Act: Try to add another arc with same ID (but different endpoints)
	arc1Duplicate := NewArc("a1", "p2", "t1", 2)
	err = net.AddArc(arc1Duplicate)

	// Assert
	if err == nil {
		t.Errorf("EXPECTED FAILURE: AddArc() should return error for duplicate ID")
		t.Logf("After fix: Return error like 'arc a1 already exists'")
	} else {
		t.Logf("PASS: AddArc() correctly returns error for duplicate: %v", err)
	}

	// Verify first arc wasn't overwritten
	arc := net.Arcs["a1"]
	if arc != nil && arc.SourceID == "p2" {
		t.Errorf("UNEXPECTED: Duplicate silently overwrote original arc")
	}
}

// TestPetriNet_DuplicateIDs_AfterResolve tests:
// Duplicate IDs should be caught before Resolve()
//
// EXPECTED FAILURE: May cause issues during Resolve()
// Expected behavior: Should be caught during Add operations
func TestPetriNet_DuplicateIDs_AfterResolve(t *testing.T) {
	// Arrange
	net := NewPetriNet("test", "Test")
	p1 := NewPlace("duplicate", "Place", 10)
	t1 := NewTransition("duplicate", "Transition") // Same ID as place

	// Try to add both (different types, same ID)
	err1 := net.AddPlace(p1)
	err2 := net.AddTransition(t1)

	if err1 == nil && err2 == nil {
		// Both succeeded - this could cause confusion
		t.Logf("WARNING: Place and Transition with same ID 'duplicate' both added")
		t.Logf("Consider: Should IDs be unique across all node types?")

		// Try to resolve
		err := net.Resolve()
		if err != nil {
			t.Logf("Resolve() failed (expected): %v", err)
		} else {
			t.Logf("Resolve() succeeded with duplicate IDs across types")
		}
	}

	t.Logf("Consider: Should place/transition IDs be in separate namespaces or global namespace?")
}

// TestPetriNet_ValidateBeforeResolve tests:
// Validate() should catch duplicate IDs
//
// Tests if Validate() catches structural issues
func TestPetriNet_ValidateBeforeResolve(t *testing.T) {
	// Arrange: Create net with valid structure
	net := NewPetriNet("test", "Test")
	p1 := NewPlace("p1", "Place 1", 10)
	t1 := NewTransition("t1", "Transition 1")
	arc := NewArc("a1", "p1", "t1", 1)

	net.AddPlace(p1)
	net.AddTransition(t1)
	net.AddArc(arc)

	// Act: Validate before resolve
	err := net.Validate()

	// Assert: Should succeed
	if err != nil {
		t.Errorf("Validate() failed on valid net: %v", err)
	}

	// Now try with invalid arc
	invalidNet := NewPetriNet("test2", "Test 2")
	invalidNet.AddPlace(p1)
	invalidArc := NewArc("a2", "p1", "nonexistent", 1)
	invalidNet.AddArc(invalidArc)

	err = invalidNet.Validate()

	// Validate() should catch the missing target
	if err == nil {
		t.Errorf("EXPECTED FAILURE: Validate() should catch missing arc target")
		t.Logf("After fix: Validate() should check all arc endpoints exist")
	}

	t.Logf("After fix: Validate() should comprehensively check net structure")
}
