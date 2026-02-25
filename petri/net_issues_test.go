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

// TestPetriNet_GetPlace tests:
// No way to retrieve individual nodes by ID after construction
//
// EXPECTED FAILURE: GetPlace() method does not exist
// This test verifies the ability to retrieve a place by its ID,
// which is essential for:
// - Dynamic workflow inspection
// - State queries
// - External integrations
// - Debugging and tooling
func TestPetriNet_GetPlace(t *testing.T) {
	// Arrange
	net := NewPetriNet("test-net", "Test Network")
	place1 := NewPlace("p1", "Place One", 10)
	place2 := NewPlace("p2", "Place Two", 20)

	if err := net.AddPlace(place1); err != nil {
		t.Fatalf("Failed to add place1: %v", err)
	}
	if err := net.AddPlace(place2); err != nil {
		t.Fatalf("Failed to add place2: %v", err)
	}

	tests := []struct {
		name      string
		placeID   string
		wantPlace *Place
		wantErr   bool
	}{
		{
			name:      "retrieve existing place",
			placeID:   "p1",
			wantPlace: place1,
			wantErr:   false,
		},
		{
			name:      "retrieve second place",
			placeID:   "p2",
			wantPlace: place2,
			wantErr:   false,
		},
		{
			name:      "place not found",
			placeID:   "p999",
			wantPlace: nil,
			wantErr:   true,
		},
		{
			name:      "empty ID",
			placeID:   "",
			wantPlace: nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got, err := net.GetPlace(tt.placeID)

			// Assert
			if tt.wantErr {
				if err == nil {
					t.Errorf("GetPlace(%q) should return error for non-existent place", tt.placeID)
				}
			} else {
				if err != nil {
					t.Errorf("GetPlace(%q) failed: %v", tt.placeID, err)
				}
				if got != tt.wantPlace {
					t.Errorf("GetPlace(%q) = %v, want %v", tt.placeID, got, tt.wantPlace)
				}
			}
		})
	}
}

// TestPetriNet_GetPlace_NotResolved tests:
// GetPlace should fail gracefully if called on unresolved net
//
// EXPECTED FAILURE: GetPlace() method does not exist
func TestPetriNet_GetPlace_NotResolved(t *testing.T) {
	// Arrange
	net := NewPetriNet("test-net", "Test Network")
	place := NewPlace("p1", "Place One", 10)

	if err := net.AddPlace(place); err != nil {
		t.Fatalf("Failed to add place: %v", err)
	}

	// Do NOT call Resolve()
	if net.Resolved() {
		t.Fatal("Net should not be resolved")
	}

	// Act & Assert
	got, err := net.GetPlace("p1")
	if err != nil {
		t.Errorf("GetPlace should work on unresolved net: %v", err)
	}
	if got != place {
		t.Errorf("GetPlace returned wrong place")
	}
}

// TestPetriNet_GetTransition tests:
// No way to retrieve individual transitions by ID after construction
//
// EXPECTED FAILURE: GetTransition() method does not exist
func TestPetriNet_GetTransition(t *testing.T) {
	// Arrange
	net := NewPetriNet("test-net", "Test Network")
	trans1 := NewTransition("t1", "Transition One")
	trans2 := NewTransition("t2", "Transition Two")

	if err := net.AddTransition(trans1); err != nil {
		t.Fatalf("Failed to add trans1: %v", err)
	}
	if err := net.AddTransition(trans2); err != nil {
		t.Fatalf("Failed to add trans2: %v", err)
	}

	tests := []struct {
		name           string
		transitionID   string
		wantTransition *Transition
		wantErr        bool
	}{
		{
			name:           "retrieve existing transition",
			transitionID:   "t1",
			wantTransition: trans1,
			wantErr:        false,
		},
		{
			name:           "retrieve second transition",
			transitionID:   "t2",
			wantTransition: trans2,
			wantErr:        false,
		},
		{
			name:           "transition not found",
			transitionID:   "t999",
			wantTransition: nil,
			wantErr:        true,
		},
		{
			name:           "empty ID",
			transitionID:   "",
			wantTransition: nil,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got, err := net.GetTransition(tt.transitionID)

			// Assert
			if tt.wantErr {
				if err == nil {
					t.Errorf("GetTransition(%q) should return error for non-existent transition", tt.transitionID)
				}
			} else {
				if err != nil {
					t.Errorf("GetTransition(%q) failed: %v", tt.transitionID, err)
				}
				if got != tt.wantTransition {
					t.Errorf("GetTransition(%q) = %v, want %v", tt.transitionID, got, tt.wantTransition)
				}
			}
		})
	}
}

// TestPetriNet_GetArc tests:
// No way to retrieve individual arcs by ID after construction
//
// EXPECTED FAILURE: GetArc() method does not exist
func TestPetriNet_GetArc(t *testing.T) {
	// Arrange
	net := NewPetriNet("test-net", "Test Network")
	place := NewPlace("p1", "Place One", 10)
	trans := NewTransition("t1", "Transition One")

	if err := net.AddPlace(place); err != nil {
		t.Fatalf("Failed to add place: %v", err)
	}
	if err := net.AddTransition(trans); err != nil {
		t.Fatalf("Failed to add transition: %v", err)
	}

	arc1 := NewArc("a1", "p1", "t1", 1)
	arc2 := NewArc("a2", "t1", "p1", 2)

	if err := net.AddArc(arc1); err != nil {
		t.Fatalf("Failed to add arc1: %v", err)
	}
	if err := net.AddArc(arc2); err != nil {
		t.Fatalf("Failed to add arc2: %v", err)
	}

	tests := []struct {
		name    string
		arcID   string
		wantArc *Arc
		wantErr bool
	}{
		{
			name:    "retrieve existing input arc",
			arcID:   "a1",
			wantArc: arc1,
			wantErr: false,
		},
		{
			name:    "retrieve existing output arc",
			arcID:   "a2",
			wantArc: arc2,
			wantErr: false,
		},
		{
			name:    "arc not found",
			arcID:   "a999",
			wantArc: nil,
			wantErr: true,
		},
		{
			name:    "empty ID",
			arcID:   "",
			wantArc: nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got, err := net.GetArc(tt.arcID)

			// Assert
			if tt.wantErr {
				if err == nil {
					t.Errorf("GetArc(%q) should return error for non-existent arc", tt.arcID)
				}
			} else {
				if err != nil {
					t.Errorf("GetArc(%q) failed: %v", tt.arcID, err)
				}
				if got != tt.wantArc {
					t.Errorf("GetArc(%q) = %v, want %v", tt.arcID, got, tt.wantArc)
				}
			}
		})
	}
}

// TestPetriNet_GettersWithNilNet tests:
// Getter methods should handle nil receiver gracefully
//
// EXPECTED FAILURE: Getter methods do not exist
func TestPetriNet_GettersWithNilNet(t *testing.T) {
	var net *PetriNet

	t.Run("GetPlace on nil net", func(t *testing.T) {
		_, err := net.GetPlace("p1")
		if err == nil {
			t.Error("GetPlace on nil net should return error")
		}
	})

	t.Run("GetTransition on nil net", func(t *testing.T) {
		_, err := net.GetTransition("t1")
		if err == nil {
			t.Error("GetTransition on nil net should return error")
		}
	})

	t.Run("GetArc on nil net", func(t *testing.T) {
		_, err := net.GetArc("a1")
		if err == nil {
			t.Error("GetArc on nil net should return error")
		}
	})
}

// TestPetriNet_GetPlace_ThreadSafety tests:
// Getter methods should be thread-safe for concurrent reads
//
// EXPECTED FAILURE: GetPlace() method does not exist
func TestPetriNet_GetPlace_ThreadSafety(t *testing.T) {
	// Arrange
	net := NewPetriNet("test-net", "Test Network")

	// Add 100 places
	for i := 0; i < 100; i++ {
		place := NewPlace(string(rune('a'+i)), "Place", 10)
		if err := net.AddPlace(place); err != nil {
			t.Fatalf("Failed to add place: %v", err)
		}
	}

	// Act: Concurrently read places
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				placeID := string(rune('a' + (j % 100)))
				_, err := net.GetPlace(placeID)
				if err != nil {
					t.Errorf("GetPlace failed: %v", err)
				}
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
