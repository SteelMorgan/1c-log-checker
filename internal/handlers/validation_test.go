package handlers

import (
	"testing"
)

func TestValidateGUID_Empty(t *testing.T) {
	err := ValidateGUID("", "cluster_guid")
	if err == nil {
		t.Error("Expected error for empty GUID")
	}
	
	validationErr, ok := err.(*ValidationError)
	if !ok {
		t.Error("Expected ValidationError")
	}
	
	if validationErr.Field != "cluster_guid" {
		t.Errorf("Expected Field 'cluster_guid', got '%s'", validationErr.Field)
	}
	
	if len(validationErr.Instructions) == 0 {
		t.Error("Expected instructions in error")
	}
}

func TestValidateGUID_InvalidFormat(t *testing.T) {
	tests := []string{
		"not-a-guid",
		"12345",
		"af4fcd7c-0a86-11e7-8e5a", // Too short
		"af4fcd7c-0a86-11e7-8e5a-00155d000b0b-extra", // Too long
		"af4fcd7c-0a86-11e7-8e5a-00155d000b0", // Invalid length
	}
	
	for _, guid := range tests {
		t.Run(guid, func(t *testing.T) {
			err := ValidateGUID(guid, "cluster_guid")
			if err == nil {
				t.Errorf("Expected error for invalid GUID: %s", guid)
			}
		})
	}
}

func TestValidateGUID_Valid(t *testing.T) {
	validGUIDs := []string{
		"af4fcd7c-0a86-11e7-8e5a-00155d000b0b",
		"00000000-0000-0000-0000-000000000001",
		"12345678-1234-1234-1234-123456789abc",
		"ABCDEF00-ABCD-ABCD-ABCD-ABCDEFABCDEF",
	}
	
	for _, guid := range validGUIDs {
		t.Run(guid, func(t *testing.T) {
			err := ValidateGUID(guid, "cluster_guid")
			if err != nil {
				t.Errorf("Unexpected error for valid GUID: %v", err)
			}
		})
	}
}

func TestValidateGUID_Placeholder(t *testing.T) {
	placeholders := []string{
		"your-guid-here",
		"your-cluster-guid-here",
		"your-infobase-guid-here",
		"example-guid",
		"placeholder",
		"00000000-0000-0000-0000-000000000000",
	}
	
	for _, placeholder := range placeholders {
		t.Run(placeholder, func(t *testing.T) {
			err := ValidateGUID(placeholder, "cluster_guid")
			if err == nil {
				t.Errorf("Expected error for placeholder: %s", placeholder)
			}
			
			validationErr, ok := err.(*ValidationError)
			if !ok {
				t.Error("Expected ValidationError")
			}
			
			if !contains(validationErr.Message, "placeholder") && !contains(validationErr.Message, "Placeholder") {
				t.Errorf("Expected placeholder detection in error message: %s", validationErr.Message)
			}
		})
	}
}

func TestValidateTimeRange_EmptyFrom(t *testing.T) {
	err := ValidateTimeRange("", "2025-01-15T10:00:00")
	if err == nil {
		t.Error("Expected error for empty 'from'")
	}
	
	validationErr, ok := err.(*ValidationError)
	if !ok {
		t.Error("Expected ValidationError")
	}
	
	if validationErr.Field != "from" {
		t.Errorf("Expected Field 'from', got '%s'", validationErr.Field)
	}
}

func TestValidateTimeRange_EmptyTo(t *testing.T) {
	err := ValidateTimeRange("2025-01-15T10:00:00", "")
	if err == nil {
		t.Error("Expected error for empty 'to'")
	}
	
	validationErr, ok := err.(*ValidationError)
	if !ok {
		t.Error("Expected ValidationError")
	}
	
	if validationErr.Field != "to" {
		t.Errorf("Expected Field 'to', got '%s'", validationErr.Field)
	}
}

func TestValidateTimeRange_Valid(t *testing.T) {
	err := ValidateTimeRange("2025-01-15T10:00:00", "2025-01-15T11:00:00")
	if err != nil {
		t.Errorf("Unexpected error for valid time range: %v", err)
	}
}

func TestValidateMode_Empty(t *testing.T) {
	// Empty mode should be valid (default will be used)
	err := ValidateMode("")
	if err != nil {
		t.Errorf("Unexpected error for empty mode: %v", err)
	}
}

func TestValidateMode_Valid(t *testing.T) {
	validModes := []string{"minimal", "full"}
	
	for _, mode := range validModes {
		t.Run(mode, func(t *testing.T) {
			err := ValidateMode(mode)
			if err != nil {
				t.Errorf("Unexpected error for valid mode '%s': %v", mode, err)
			}
		})
	}
}

func TestValidateMode_Invalid(t *testing.T) {
	invalidModes := []string{"invalid", "both", "none", "all"}
	
	for _, mode := range invalidModes {
		t.Run(mode, func(t *testing.T) {
			err := ValidateMode(mode)
			if err == nil {
				t.Errorf("Expected error for invalid mode: %s", mode)
			}
			
			validationErr, ok := err.(*ValidationError)
			if !ok {
				t.Error("Expected ValidationError")
			}
			
			if validationErr.Field != "mode" {
				t.Errorf("Expected Field 'mode', got '%s'", validationErr.Field)
			}
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{
		Field:   "test_field",
		Message: "test message",
	}
	
	errorStr := err.Error()
	if errorStr == "" {
		t.Error("Error() should not return empty string")
	}
	
	if !contains(errorStr, "test_field") {
		t.Errorf("Error message should contain field name: %s", errorStr)
	}
	
	if !contains(errorStr, "test message") {
		t.Errorf("Error message should contain message: %s", errorStr)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > len(substr) && (s[:len(substr)] == substr || 
		s[len(s)-len(substr):] == substr || 
		containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

