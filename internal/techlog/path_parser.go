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

// extractGUIDsFromPath extracts cluster_guid and infobase_guid from techlog file path
//
// Expected path structure:
//   <base>/<cluster_guid>/<infobase_guid>/<process_pid>/<filename>
//   Example: D:\TechLogs\b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3\d723aefd-7992-420d-b5f9-a273fd4146be\rphost_1234\25011408.log
//
// Returns:
//   clusterGUID  - GUID of the cluster (parent directory level -2 from process dir)
//   infobaseGUID - GUID of the infobase (parent directory level -1 from process dir)
//   error        - if GUIDs cannot be extracted
func extractGUIDsFromPath(filePath string) (clusterGUID, infobaseGUID string, err error) {
	// Normalize path separators to forward slashes
	normalizedPath := filepath.ToSlash(filePath)

	// Split path into parts
	parts := strings.Split(normalizedPath, "/")

	if len(parts) < 4 {
		return "", "", fmt.Errorf("path too short to contain GUIDs (expected at least 4 parts, got %d): %s", len(parts), filePath)
	}

	// Search for GUIDs starting from the end (skip filename)
	// We need to find 2 GUIDs in parent directories
	var foundGUIDs []string

	// Start from second-to-last element (skip filename), search backwards
	// Stop when we find 2 GUIDs or reach the beginning
	for i := len(parts) - 2; i >= 0 && len(foundGUIDs) < 2; i-- {
		part := strings.ToLower(parts[i])
		if guidRegex.MatchString(part) {
			foundGUIDs = append(foundGUIDs, part)
		}
	}

	// We expect exactly 2 GUIDs
	if len(foundGUIDs) < 2 {
		return "", "", fmt.Errorf("expected 2 GUIDs in path, found %d: %s", len(foundGUIDs), filePath)
	}

	// foundGUIDs are in reverse order (from file to root)
	// foundGUIDs[0] = infobase_guid (closer to file, level -1)
	// foundGUIDs[1] = cluster_guid (further from file, level -2)
	infobaseGUID = foundGUIDs[0]
	clusterGUID = foundGUIDs[1]

	return clusterGUID, infobaseGUID, nil
}

// validateTechLogPath validates that the path contains both cluster_guid and infobase_guid
// and that they match the expected values
func validateTechLogPath(path string, expectedClusterGUID, expectedInfobaseGUID string) error {
	// Extract GUIDs from path
	clusterGUID, infobaseGUID, err := extractGUIDsFromPath(path)
	if err != nil {
		return fmt.Errorf("path validation failed: %w", err)
	}

	// Normalize to lowercase for comparison
	clusterGUID = strings.ToLower(clusterGUID)
	infobaseGUID = strings.ToLower(infobaseGUID)
	expectedClusterGUID = strings.ToLower(expectedClusterGUID)
	expectedInfobaseGUID = strings.ToLower(expectedInfobaseGUID)

	// Check cluster_guid
	if clusterGUID != expectedClusterGUID {
		return fmt.Errorf(
			"cluster_guid mismatch: path contains '%s' but expected '%s'",
			clusterGUID,
			expectedClusterGUID,
		)
	}

	// Check infobase_guid
	if infobaseGUID != expectedInfobaseGUID {
		return fmt.Errorf(
			"infobase_guid mismatch: path contains '%s' but expected '%s'",
			infobaseGUID,
			expectedInfobaseGUID,
		)
	}

	return nil
}
