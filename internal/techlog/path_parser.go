package techlog

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// GUID regex pattern (RFC 4122 format)
// Example: b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3
var guidRegex = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// ExtractGUIDsFromPath extracts cluster_guid and infobase_guid from techlog file path
//
// Expected path structure:
//
//	<base>/<cluster_guid>/<infobase_guid>/<process_pid>/<filename>
//	Example: D:\TechLogs\b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3\d723aefd-7992-420d-b5f9-a273fd4146be\rphost_1234\25011408.log
//
// Returns:
//
//	clusterGUID  - GUID of the cluster (parent directory level -2 from process dir)
//	infobaseGUID - GUID of the infobase (parent directory level -1 from process dir)
//	error        - if GUIDs cannot be extracted
func ExtractGUIDsFromPath(filePath string) (clusterGUID, infobaseGUID string, err error) {
	// Normalize path separators to forward slashes
	normalizedPath := filepath.ToSlash(filePath)

	// Split path into parts
	parts := strings.Split(normalizedPath, "/")

	// Remove empty parts (from leading/trailing slashes)
	var cleanParts []string
	for _, part := range parts {
		if part != "" {
			cleanParts = append(cleanParts, part)
		}
	}

	if len(cleanParts) < 2 {
		return "", "", fmt.Errorf("path too short to contain GUIDs (expected at least 2 parts, got %d): %s", len(cleanParts), filePath)
	}

	// Search for GUIDs starting from the end
	// We need to find 2 GUIDs in parent directories
	// For directory paths: last 2 parts should be GUIDs
	// For file paths: skip filename, then last 2 parts should be GUIDs
	var foundGUIDs []string

	// Start from the end, search backwards
	// Check if last part looks like a filename (has extension) - if so, skip it
	startIndex := len(cleanParts) - 1
	lastPart := cleanParts[startIndex]
	if strings.Contains(lastPart, ".") && !guidRegex.MatchString(strings.ToLower(lastPart)) {
		// Last part looks like a filename, skip it
		startIndex = len(cleanParts) - 2
	}

	// Search backwards from startIndex
	for i := startIndex; i >= 0 && len(foundGUIDs) < 2; i-- {
		part := strings.ToLower(cleanParts[i])
		if guidRegex.MatchString(part) {
			foundGUIDs = append(foundGUIDs, part)
		}
	}

	// We expect exactly 2 GUIDs
	if len(foundGUIDs) < 2 {
		return "", "", fmt.Errorf("expected 2 GUIDs in path, found %d: %s", len(foundGUIDs), filePath)
	}

	// foundGUIDs are in reverse order (from end to beginning)
	// foundGUIDs[0] = infobase_guid (closer to end, level -1)
	// foundGUIDs[1] = cluster_guid (further from end, level -2)
	infobaseGUID = foundGUIDs[0]
	clusterGUID = foundGUIDs[1]

	return clusterGUID, infobaseGUID, nil
}

// ValidateTechLogPath validates that the path starts with one of the base directories from techLogDirs,
// followed by cluster_guid and infobase_guid that match the expected values.
// If validation fails, returns an error with the correct path suggestion.
func ValidateTechLogPath(path string, expectedClusterGUID, expectedInfobaseGUID string, techLogDirs []string) error {
	// Normalize path separators
	normalizedPath := filepath.ToSlash(path)
	normalizedPath = strings.TrimSuffix(normalizedPath, "/")

	// Normalize expected GUIDs to lowercase
	expectedClusterGUID = strings.ToLower(expectedClusterGUID)
	expectedInfobaseGUID = strings.ToLower(expectedInfobaseGUID)

	// Check if path starts with one of the base directories (if techLogDirs provided)
	var baseDir string
	var found bool
	var cleanParts []string

	if len(techLogDirs) > 0 {
		// Validate against base directories
		for _, base := range techLogDirs {
			normalizedBase := filepath.ToSlash(base)
			normalizedBase = strings.TrimSuffix(normalizedBase, "/")
			if strings.HasPrefix(normalizedPath, normalizedBase) {
				baseDir = normalizedBase
				found = true
				break
			}
		}

		if !found {
			// Path doesn't start with any base directory
			// Generate correct path suggestion
			correctPath := filepath.ToSlash(techLogDirs[0])
			correctPath = strings.TrimSuffix(correctPath, "/")
			correctPath = fmt.Sprintf("%s/%s/%s", correctPath, expectedClusterGUID, expectedInfobaseGUID)
			return fmt.Errorf(
				"path does not start with any base directory from TECHLOG_DIRS. For cluster '%s' and infobase '%s' the correct path should be: %s",
				expectedClusterGUID,
				expectedInfobaseGUID,
				correctPath,
			)
		}

		// Extract the part after base directory
		relativePath := strings.TrimPrefix(normalizedPath, baseDir)
		relativePath = strings.TrimPrefix(relativePath, "/")

		// Split into parts
		parts := strings.Split(relativePath, "/")

		// Remove empty parts
		for _, part := range parts {
			if part != "" {
				cleanParts = append(cleanParts, part)
			}
		}

		// Check if we have at least 2 parts (cluster_guid and infobase_guid)
		if len(cleanParts) < 2 {
			correctPath := fmt.Sprintf("%s/%s/%s", baseDir, expectedClusterGUID, expectedInfobaseGUID)
			return fmt.Errorf(
				"path is too short after base directory. Expected format: <base_dir>/<cluster_guid>/<infobase_guid>. For cluster '%s' and infobase '%s' the correct path should be: %s",
				expectedClusterGUID,
				expectedInfobaseGUID,
				correctPath,
			)
		}
	} else {
		// No base directories provided, just extract GUIDs from path
		// Split path into parts
		parts := strings.Split(normalizedPath, "/")

		// Remove empty parts
		for _, part := range parts {
			if part != "" {
				cleanParts = append(cleanParts, part)
			}
		}

		// Check if we have at least 2 parts (cluster_guid and infobase_guid)
		if len(cleanParts) < 2 {
			return fmt.Errorf(
				"path is too short. Expected format: <base_dir>/<cluster_guid>/<infobase_guid>. Path should contain at least cluster_guid and infobase_guid",
			)
		}
	}

	// Check cluster_guid (first part after base, or last-1 if no base)
	clusterGUID := strings.ToLower(cleanParts[0])
	if clusterGUID != expectedClusterGUID {
		var correctPath string
		if baseDir != "" {
			correctPath = fmt.Sprintf("%s/%s/%s", baseDir, expectedClusterGUID, expectedInfobaseGUID)
		} else if len(techLogDirs) > 0 {
			correctPath = fmt.Sprintf("%s/%s/%s", filepath.ToSlash(techLogDirs[0]), expectedClusterGUID, expectedInfobaseGUID)
		} else {
			correctPath = fmt.Sprintf("<base_dir>/%s/%s", expectedClusterGUID, expectedInfobaseGUID)
		}
		return fmt.Errorf(
			"cluster_guid mismatch: path contains '%s' but expected '%s'. For cluster '%s' and infobase '%s' the correct path should be: %s",
			clusterGUID,
			expectedClusterGUID,
			expectedClusterGUID,
			expectedInfobaseGUID,
			correctPath,
		)
	}

	// Check infobase_guid (second part after base, or last if no base)
	infobaseGUID := strings.ToLower(cleanParts[1])
	if infobaseGUID != expectedInfobaseGUID {
		var correctPath string
		if baseDir != "" {
			correctPath = fmt.Sprintf("%s/%s/%s", baseDir, expectedClusterGUID, expectedInfobaseGUID)
		} else if len(techLogDirs) > 0 {
			correctPath = fmt.Sprintf("%s/%s/%s", filepath.ToSlash(techLogDirs[0]), expectedClusterGUID, expectedInfobaseGUID)
		} else {
			correctPath = fmt.Sprintf("<base_dir>/%s/%s", expectedClusterGUID, expectedInfobaseGUID)
		}
		return fmt.Errorf(
			"infobase_guid mismatch: path contains '%s' but expected '%s'. For cluster '%s' and infobase '%s' the correct path should be: %s",
			infobaseGUID,
			expectedInfobaseGUID,
			expectedClusterGUID,
			expectedInfobaseGUID,
			correctPath,
		)
	}

	return nil
}
