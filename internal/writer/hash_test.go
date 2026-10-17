package writer

import (
	"github.com/SteelMorgan/1c-log-checker/internal/domain"
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
			hash1 := eventHash(t, tt.data)
			hash2 := eventHash(t, tt.data)

			// Hash should be consistent
			if hash1 != hash2 {
				t.Errorf("Hash should be consistent: %s != %s", hash1, hash2)
			}

			// Hash should not be empty
			if hash1 == "" {
				t.Error("Hash should not be empty")
			}

			// Hash should be hex string (40 chars for SHA1)
			if len(hash1) != 40 {
				t.Errorf("Hash should be 40 characters (SHA1), got %d", len(hash1))
			}
		})
	}
}

func TestCalculateHash_DifferentInputs(t *testing.T) {
	hash1 := eventHash(t, "test1")
	hash2 := eventHash(t, "test2")

	// Different inputs should produce different hashes
	if hash1 == hash2 {
		t.Error("Different inputs should produce different hashes")
	}
}

func TestCalculateHash_CaseSensitive(t *testing.T) {
	hash1 := eventHash(t, "Test")
	hash2 := eventHash(t, "test")

	// Case should matter
	if hash1 == hash2 {
		t.Error("Hash should be case-sensitive")
	}
}

func eventHash(t *testing.T, text string) string {
	t.Helper()
	hash, err := calculateEventLogHash(&domain.EventLogRecord{Comment: text})
	if err != nil {
		t.Fatal(err)
	}
	return hash
}
