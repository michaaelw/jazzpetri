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

package token

import (
	"testing"
)

// TestTokenData_Interface ensures that token types implement TokenData
func TestTokenData_Interface(t *testing.T) {
	tests := []struct {
		name      string
		tokenData TokenData
		wantType  string
	}{
		{
			name:      "IntToken implements TokenData",
			tokenData: &IntToken{Value: 42},
			wantType:  "IntToken",
		},
		{
			name:      "StringToken implements TokenData",
			tokenData: &StringToken{Value: "test"},
			wantType:  "StringToken",
		},
		{
			name:      "MapToken implements TokenData",
			tokenData: &MapToken{Value: map[string]interface{}{"key": "value"}},
			wantType:  "MapToken",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tokenData.Type(); got != tt.wantType {
				t.Errorf("Type() = %v, want %v", got, tt.wantType)
			}
		})
	}
}

// TestToken_Creation verifies token creation with ID and data
func TestToken_Creation(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		data     TokenData
		wantID   string
		wantType string
	}{
		{
			name:     "Create token with IntToken",
			id:       "token-1",
			data:     &IntToken{Value: 100},
			wantID:   "token-1",
			wantType: "IntToken",
		},
		{
			name:     "Create token with StringToken",
			id:       "token-2",
			data:     &StringToken{Value: "hello"},
			wantID:   "token-2",
			wantType: "StringToken",
		},
		{
			name:     "Create token with MapToken",
			id:       "token-3",
			data:     &MapToken{Value: map[string]interface{}{"count": 42}},
			wantID:   "token-3",
			wantType: "MapToken",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := Token{
				ID:   tt.id,
				Data: tt.data,
			}

			if token.ID != tt.wantID {
				t.Errorf("Token.ID = %v, want %v", token.ID, tt.wantID)
			}

			if token.Data == nil {
				t.Fatal("Token.Data is nil")
			}

			if token.Data.Type() != tt.wantType {
				t.Errorf("Token.Data.Type() = %v, want %v", token.Data.Type(), tt.wantType)
			}
		})
	}
}

// TestToken_NilData ensures proper handling of nil data
func TestToken_NilData(t *testing.T) {
	token := Token{
		ID:   "nil-token",
		Data: nil,
	}

	if token.Data != nil {
		t.Errorf("Expected nil data, got %v", token.Data)
	}
}
