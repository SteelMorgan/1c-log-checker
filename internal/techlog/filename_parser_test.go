package techlog

import (
	"testing"
	"time"
)

func TestExtractTimestampFromFilename_Valid(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected time.Time
	}{
		{
			name:     "simple filename",
			filename: "25011408.log",
			expected: time.Date(2025, 1, 14, 8, 0, 0, 0, time.Local),
		},
		{
			name:     "rphost filename",
			filename: "rphost_1234_25011408.log",
			expected: time.Date(2025, 1, 14, 8, 0, 0, 0, time.Local),
		},
		{
			name:     "1cv8c filename",
			filename: "1cv8c_5678_25011409.log",
			expected: time.Date(2025, 1, 14, 9, 0, 0, 0, time.Local),
		},
		{
			name:     "with path",
			filename: "/path/to/logs/25011410.log",
			expected: time.Date(2025, 1, 14, 10, 0, 0, 0, time.Local),
		},
		{
			name:     "with .zip extension",
			filename: "25011411.zip",
			expected: time.Date(2025, 1, 14, 11, 0, 0, 0, time.Local),
		},
		{
			name:     "with .gz extension",
			filename: "25011412.gz",
			expected: time.Date(2025, 1, 14, 12, 0, 0, 0, time.Local),
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExtractTimestampFromFilename(tt.filename)
			if err != nil {
				t.Fatalf("ExtractTimestampFromFilename() failed: %v", err)
			}
			
			// Compare year, month, day, hour (ignore minutes, seconds, nanoseconds)
			if result.Year() != tt.expected.Year() ||
				result.Month() != tt.expected.Month() ||
				result.Day() != tt.expected.Day() ||
				result.Hour() != tt.expected.Hour() {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestExtractTimestampFromFilename_Invalid(t *testing.T) {
	tests := []struct {
		name     string
		filename string
	}{
		{
			name:     "no timestamp",
			filename: "file.log",
		},
		{
			name:     "short timestamp",
			filename: "250114.log",
		},
		{
			name:     "invalid month",
			filename: "25131408.log",
		},
		{
			name:     "invalid day",
			filename: "25013208.log",
		},
		{
			name:     "invalid hour",
			filename: "25011424.log",
		},
		{
			name:     "empty filename",
			filename: "",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ExtractTimestampFromFilename(tt.filename)
			if err == nil {
				t.Errorf("Expected error for filename '%s'", tt.filename)
			}
		})
	}
}

func TestExtractTimestampFromFilename_YearConversion(t *testing.T) {
	// Test year conversion: 00-99 → 2000-2099
	tests := []struct {
		filename string
		expectedYear int
	}{
		{"00011408.log", 2000},
		{"99011408.log", 2099},
		{"25011408.log", 2025},
	}
	
	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result, err := ExtractTimestampFromFilename(tt.filename)
			if err != nil {
				t.Fatalf("ExtractTimestampFromFilename() failed: %v", err)
			}
			
			if result.Year() != tt.expectedYear {
				t.Errorf("Expected year %d, got %d", tt.expectedYear, result.Year())
			}
		})
	}
}

