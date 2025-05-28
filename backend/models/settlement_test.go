package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSettlementDetailsValue(t *testing.T) {
	tests := []struct {
		name     string
		details  SettlementDetails
		expected string
		hasError bool
	}{
		{
			name:     "nil details",
			details:  nil,
			expected: "",
			hasError: false,
		},
		{
			name:     "empty details",
			details:  SettlementDetails{},
			expected: "{}",
			hasError: false,
		},
		{
			name: "details with data",
			details: SettlementDetails{
				"action": "created",
				"amount": 100.50,
				"notes":  "Initial settlement",
			},
			expected: `{"action":"created","amount":100.5,"notes":"Initial settlement"}`,
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := tt.details.Value()

			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.details == nil {
				if value != nil {
					t.Errorf("Expected nil value for nil details, got %v", value)
				}
				return
			}

			jsonBytes, ok := value.([]byte)
			if !ok {
				t.Errorf("Expected []byte, got %T", value)
				return
			}

			var result map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &result); err != nil {
				t.Errorf("Failed to unmarshal result: %v", err)
				return
			}

			var expected map[string]interface{}
			if err := json.Unmarshal([]byte(tt.expected), &expected); err != nil {
				t.Errorf("Failed to unmarshal expected: %v", err)
				return
			}

			// Compare the unmarshaled maps instead of raw JSON strings
			// since map iteration order is not guaranteed
			if len(result) != len(expected) {
				t.Errorf("Different map lengths: got %d, expected %d", len(result), len(expected))
				return
			}

			for key, expectedVal := range expected {
				if result[key] != expectedVal {
					t.Errorf("Key %s: got %v, expected %v", key, result[key], expectedVal)
				}
			}
		})
	}
}

func TestSettlementDetailsScan(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected SettlementDetails
		hasError bool
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
			hasError: false,
		},
		{
			name:     "empty JSON bytes",
			input:    []byte("{}"),
			expected: SettlementDetails{},
			hasError: false,
		},
		{
			name:  "valid JSON bytes",
			input: []byte(`{"action":"completed","amount":250.75}`),
			expected: SettlementDetails{
				"action": "completed",
				"amount": 250.75,
			},
			hasError: false,
		},
		{
			name:     "invalid JSON bytes",
			input:    []byte(`{"invalid": json}`),
			expected: nil,
			hasError: true,
		},
		{
			name:     "non-byte input",
			input:    "not bytes",
			expected: nil,
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var details SettlementDetails
			err := details.Scan(tt.input)

			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.expected == nil {
				if details != nil {
					t.Errorf("Expected nil details, got %v", details)
				}
				return
			}

			if len(details) != len(tt.expected) {
				t.Errorf("Different lengths: got %d, expected %d", len(details), len(tt.expected))
				return
			}

			for key, expectedVal := range tt.expected {
				if details[key] != expectedVal {
					t.Errorf("Key %s: got %v, expected %v", key, details[key], expectedVal)
				}
			}
		})
	}
}

func TestSettlementModel(t *testing.T) {
	now := time.Now()
	completedAt := now.Add(time.Hour)

	settlement := Settlement{
		ID:              "settlement-123",
		CreatorID:       "user-1",
		RecipientID:     "user-2",
		TotalAmount:     150.75,
		RemainingAmount: 50.25,
		Status:          "active",
		CreatedAt:       now,
		UpdatedAt:       now,
		CompletedAt:     &completedAt,
		Notes:           "Test settlement",
		Items: []SettlementItem{
			{
				ID:            1,
				SettlementID:  "settlement-123",
				TransactionID: "tx-1",
				AppliedAmount: 100.50,
				CreatedAt:     now,
				CreatedBy:     "user-1",
			},
		},
		History: []SettlementHistory{
			{
				ID:           1,
				SettlementID: "settlement-123",
				Action:       "created",
				ActorID:      "user-1",
				Details: SettlementDetails{
					"notes": "Initial creation",
				},
				CreatedAt: now,
			},
		},
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(settlement)
	if err != nil {
		t.Errorf("Failed to marshal settlement: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled Settlement
	if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
		t.Errorf("Failed to unmarshal settlement: %v", err)
	}

	// Verify key fields
	if unmarshaled.ID != settlement.ID {
		t.Errorf("ID mismatch: got %s, expected %s", unmarshaled.ID, settlement.ID)
	}

	if unmarshaled.TotalAmount != settlement.TotalAmount {
		t.Errorf("TotalAmount mismatch: got %f, expected %f", unmarshaled.TotalAmount, settlement.TotalAmount)
	}

	if len(unmarshaled.Items) != len(settlement.Items) {
		t.Errorf("Items length mismatch: got %d, expected %d", len(unmarshaled.Items), len(settlement.Items))
	}

	if len(unmarshaled.History) != len(settlement.History) {
		t.Errorf("History length mismatch: got %d, expected %d", len(unmarshaled.History), len(settlement.History))
	}
}

func TestSettlementSummaryModel(t *testing.T) {
	now := time.Now()

	summary := SettlementSummary{
		ID:            "test-settlement",
		CreatorID:     "user-1",
		CreatorName:   "Alice",
		RecipientID:   "user-2",
		RecipientName: "Bob",
		TotalAmount:   100.50,
		Status:        "active",
		CreatedAt:     now,
		ItemCount:     1,
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(summary)
	if err != nil {
		t.Errorf("Failed to marshal settlement summary: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled SettlementSummary
	if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
		t.Errorf("Failed to unmarshal settlement summary: %v", err)
	}

	// Verify all fields
	if unmarshaled.ID != summary.ID {
		t.Errorf("ID mismatch: got %s, expected %s", unmarshaled.ID, summary.ID)
	}

	if unmarshaled.CreatorName != summary.CreatorName {
		t.Errorf("CreatorName mismatch: got %s, expected %s", unmarshaled.CreatorName, summary.CreatorName)
	}

	if unmarshaled.ItemCount != summary.ItemCount {
		t.Errorf("ItemCount mismatch: got %d, expected %d", unmarshaled.ItemCount, summary.ItemCount)
	}
}
