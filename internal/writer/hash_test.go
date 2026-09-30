package writer

import (
	"testing"
)

func TestCalculateHash(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		expected string // We'll check length and format
	}{
		{
			name: "empty string",
			data: "",
		},
		{
			name: "simple string",
			data: "test data",
		},
		{
			name: "long string",
			data: "this is a very long string that should be hashed properly and consistently",
		},
		{
			name: "special characters",
			data: "test@#$%^&*()data",
		},
		{
			name: "unicode",
			data: "тест данных",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1 := CalculateHash(tt.data)
			hash2 := CalculateHash(tt.data)
			
			// Hash should be consistent
			if hash1 != hash2 {
				t.Errorf("Hash should be consistent: %s != %s", hash1, hash2)
			}
			
			// Hash should not be empty
			if hash1 == "" {
				t.Error("Hash should not be empty")
			}
			
			// Hash should be hex string (64 chars for SHA256)
			if len(hash1) != 64 {
				t.Errorf("Hash should be 64 characters (SHA256), got %d", len(hash1))
			}
		})
	}
}

func TestCalculateHash_DifferentInputs(t *testing.T) {
	hash1 := CalculateHash("test1")
	hash2 := CalculateHash("test2")
	
	// Different inputs should produce different hashes
	if hash1 == hash2 {
		t.Error("Different inputs should produce different hashes")
	}
}

func TestCalculateHash_CaseSensitive(t *testing.T) {
	hash1 := CalculateHash("Test")
	hash2 := CalculateHash("test")
	
	// Case should matter
	if hash1 == hash2 {
		t.Error("Hash should be case-sensitive")
	}
}

