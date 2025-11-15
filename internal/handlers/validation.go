package handlers

import (
	"fmt"
	"regexp"
)

var guidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// ValidationError represents a validation error with instructions
type ValidationError struct {
	Field        string   `json:"field"`
	Message      string   `json:"message"`
	Instructions []string `json:"instructions"`
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidateGUID validates GUID format and returns helpful error if invalid
func ValidateGUID(guid string, fieldName string) error {
	if guid == "" {
		return &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("%s is required", fieldName),
			Instructions: []string{
				"1. Read the file: configs/cluster_map.yaml using the Read tool",
				"2. Find the appropriate GUID in the 'clusters' or 'infobases' section",
				"3. Use the exact GUID value from the file",
				"4. Example GUID format: 'af4fcd7c-0a86-11e7-8e5a-00155d000b0b'",
			},
		}
	}

	if !guidPattern.MatchString(guid) {
		return &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("Invalid %s format. Must be a valid UUID.", fieldName),
			Instructions: []string{
				"1. Read the file: configs/cluster_map.yaml using the Read tool",
				"2. Extract the GUID from the appropriate section (clusters or infobases)",
				"3. GUID format must be: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
				"4. Example: 'af4fcd7c-0a86-11e7-8e5a-00155d000b0b'",
				"5. NEVER use placeholder values like 'your-guid-here'",
			},
		}
	}

	// Check for common placeholder patterns
	placeholders := []string{
		"your-guid-here",
		"your-cluster-guid-here",
		"your-infobase-guid-here",
		"example-guid",
		"placeholder",
		"00000000-0000-0000-0000-000000000000",
	}

	for _, placeholder := range placeholders {
		if guid == placeholder {
			return &ValidationError{
				Field:   fieldName,
				Message: "Placeholder GUID detected. You must use actual GUID from configs/cluster_map.yaml",
				Instructions: []string{
					"ERROR: You are using a placeholder GUID instead of reading the actual configuration!",
					"",
					"REQUIRED STEPS:",
					"1. Use the Read tool to read file: configs/cluster_map.yaml",
					"2. Look for the 'clusters:' section to get cluster_guid",
					"3. Look for the 'infobases:' section to get infobase_guid",
					"4. Use the EXACT GUID values from the file",
					"",
					"If cluster_map.yaml is empty:",
					"- Inform the user that configuration is required",
					"- Refer them to docs/guides/get-guids.md for instructions",
				},
			}
		}
	}

	return nil
}

// ValidateTimeRange validates that from is before to
func ValidateTimeRange(from, to string) error {
	if from == "" {
		return &ValidationError{
			Field:   "from",
			Message: "Start time (from) is required",
			Instructions: []string{
				"Provide 'from' parameter in ISO 8601 format",
				"Example: '2025-11-15T14:00:00'",
			},
		}
	}

	if to == "" {
		return &ValidationError{
			Field:   "to",
			Message: "End time (to) is required",
			Instructions: []string{
				"Provide 'to' parameter in ISO 8601 format",
				"Example: '2025-11-15T15:00:00'",
			},
		}
	}

	return nil
}

// ValidateMode validates output mode parameter
func ValidateMode(mode string) error {
	if mode == "" {
		return nil // Default will be used
	}

	if mode != "minimal" && mode != "full" {
		return &ValidationError{
			Field:   "mode",
			Message: "Invalid mode. Must be 'minimal' or 'full'",
			Instructions: []string{
				"Use mode='minimal' (default) for token efficiency",
				"Use mode='full' only if minimal data is insufficient",
				"",
				"Token optimization best practice:",
				"1. Always start with mode='minimal'",
				"2. Only switch to mode='full' if you need additional fields",
				"3. Minimal mode saves ~60-70% tokens",
			},
		}
	}

	return nil
}
