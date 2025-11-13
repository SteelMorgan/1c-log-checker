package logreader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
)

// LogLocation represents a discovered log location
type LogLocation struct {
	BasePath     string // Path to 1Cv8Log directory
	ClusterGUID  string // Extracted from path
	InfobaseGUID string // Extracted from path
	LgfFile      string // Path to 1Cv8.lgf
	LgpFiles     []string // Paths to .lgp files
}

// ScanForLogs recursively scans directories for 1C event log files
// Searches for 1Cv8.lgf files and extracts GUIDs from directory structure
func ScanForLogs(rootDirs []string) ([]LogLocation, error) {
	var locations []LogLocation
	
	for _, rootDir := range rootDirs {
		log.Info().Str("root_dir", rootDir).Msg("Scanning for event logs...")
		
		err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				// Skip inaccessible directories
				log.Warn().Err(err).Str("path", path).Msg("Skipping inaccessible path")
				return nil
			}
			
			// Look for 1Cv8.lgf files
			if !info.IsDir() && strings.ToLower(info.Name()) == "1cv8.lgf" {
				location, err := extractLogLocation(path)
				if err != nil {
					log.Warn().Err(err).Str("path", path).Msg("Failed to extract log location")
					return nil
				}
				
				locations = append(locations, *location)
				log.Info().
					Str("cluster_guid", location.ClusterGUID).
					Str("infobase_guid", location.InfobaseGUID).
					Int("lgp_files", len(location.LgpFiles)).
					Msg("Found event log")
			}
			
			return nil
		})
		
		if err != nil {
			return nil, fmt.Errorf("failed to walk directory %s: %w", rootDir, err)
		}
	}
	
	log.Info().Int("total_locations", len(locations)).Msg("Log scan complete")
	return locations, nil
}

// extractLogLocation extracts log location info from 1Cv8.lgf path
// Expected path structure:
// C:\Program Files\1cv8\srvinfo\reg_<port>\<infobase_guid>\1Cv8Log\1Cv8.lgf
// Where:
//   - reg_<port>: cluster port (e.g. reg_1541)
//   - <infobase_guid>: GUID информационной базы (e.g. d723aefd-7992-420d-b5f9-a273fd4146be)
//   - 1Cv8Log: standard log directory name
func extractLogLocation(lgfPath string) (*LogLocation, error) {
	// Get directory containing 1Cv8.lgf (should be 1Cv8Log)
	basePath := filepath.Dir(lgfPath)
	
	// Check if directory name is 1Cv8Log
	if strings.ToLower(filepath.Base(basePath)) != "1cv8log" {
		return nil, fmt.Errorf("unexpected directory structure: expected 1Cv8Log, got %s", filepath.Base(basePath))
	}
	
	// Extract infobase GUID from parent directory
	// Path: .../reg_<port>/<infobase_guid>/1Cv8Log
	parentDir := filepath.Dir(basePath)
	infobaseGUID := filepath.Base(parentDir)
	
	// Validate GUID format (xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx)
	guidPattern := func(s string) bool {
		parts := strings.Split(s, "-")
		return len(parts) == 5 &&
			len(parts[0]) == 8 &&
			len(parts[1]) == 4 &&
			len(parts[2]) == 4 &&
			len(parts[3]) == 4 &&
			len(parts[4]) == 12
	}
	
	if !guidPattern(infobaseGUID) {
		return nil, fmt.Errorf("invalid infobase GUID format: %s", infobaseGUID)
	}
	
	// Extract cluster GUID from 1CV8Clst.lst file
	regDir := filepath.Dir(parentDir)
	clusterGUID, err := GetClusterGUIDForReg(regDir)
	if err != nil {
		log.Warn().
			Err(err).
			Str("reg_dir", regDir).
			Msg("Failed to get cluster GUID, using reg_<port> as fallback")
		
		regName := filepath.Base(regDir)
		if strings.HasPrefix(strings.ToLower(regName), "reg_") {
			clusterGUID = regName
		} else {
			clusterGUID = "unknown"
		}
	}
	
	// Find all .lgp files
	lgpPattern := filepath.Join(basePath, "*.lgp")
	lgpFiles, err := filepath.Glob(lgpPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to find lgp files: %w", err)
	}
	
	location := &LogLocation{
		BasePath:     basePath,
		ClusterGUID:  clusterGUID,
		InfobaseGUID: infobaseGUID,
		LgfFile:      lgfPath,
		LgpFiles:     lgpFiles,
	}
	
	return location, nil
}

