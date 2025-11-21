package logreader

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

// ClusterInfo contains cluster metadata from 1CV8Clst.lst
type ClusterInfo struct {
	GUID string
	Name string
	Port int
}

// InfobaseInfo contains infobase metadata from 1CV8Clst.lst
type InfobaseInfo struct {
	GUID string
	Name string
}

// ClusterFileData contains all data from 1CV8Clst.lst file
type ClusterFileData struct {
	Cluster   *ClusterInfo
	Infobases map[string]InfobaseInfo // Map: GUID -> InfobaseInfo
}

// ReadClusterInfo reads cluster GUID and name from 1CV8Clst.lst file
// File format: {<cluster_guid>,"<cluster_name>",<port>,...}
//
//	{<infobase_guid>,"<infobase_name>",...}
//	...
//
// First record is cluster, subsequent records are infobases
func ReadClusterInfo(regDir string) (*ClusterInfo, error) {
	data, err := ReadClusterFileData(regDir)
	if err != nil {
		return nil, err
	}
	return data.Cluster, nil
}

// ReadClusterFileData reads all data from 1CV8Clst.lst file
// Returns cluster info and map of infobases (GUID -> InfobaseInfo)
func ReadClusterFileData(regDir string) (*ClusterFileData, error) {
	// Path to 1CV8Clst.lst
	clstPath := filepath.Join(regDir, "1CV8Clst.lst")

	data, err := os.ReadFile(clstPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read 1CV8Clst.lst: %w", err)
	}

	content := string(data)

	// Pattern for parsing records: {<guid>,"<name>",...}
	// GUID can be in format xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	// Name is in quotes and can contain any characters except quotes
	recordPattern := regexp.MustCompile(`\{([a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}),"([^"]+)"`)

	matches := recordPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no records found in 1CV8Clst.lst")
	}

	// First record is cluster info
	if len(matches) < 1 {
		return nil, fmt.Errorf("failed to parse cluster info from 1CV8Clst.lst")
	}

	clusterInfo := &ClusterInfo{
		GUID: matches[0][1],
		Name: matches[0][2],
		Port: 0, // We already know port from reg_<port>
	}

	// Subsequent records are infobases
	infobases := make(map[string]InfobaseInfo)
	for i := 1; i < len(matches); i++ {
		infobaseGUID := matches[i][1]
		infobaseName := matches[i][2]
		infobases[infobaseGUID] = InfobaseInfo{
			GUID: infobaseGUID,
			Name: infobaseName,
		}
	}

	log.Debug().
		Str("cluster_guid", clusterInfo.GUID).
		Str("cluster_name", clusterInfo.Name).
		Int("infobases_count", len(infobases)).
		Msg("Read cluster and infobases info from 1CV8Clst.lst")

	return &ClusterFileData{
		Cluster:   clusterInfo,
		Infobases: infobases,
	}, nil
}

// GetInfobaseName reads infobase name from 1CV8Clst.lst by GUID
func GetInfobaseName(regDir, infobaseGUID string) (string, error) {
	data, err := ReadClusterFileData(regDir)
	if err != nil {
		return "", err
	}

	if infobase, ok := data.Infobases[infobaseGUID]; ok {
		return infobase.Name, nil
	}

	return "", fmt.Errorf("infobase GUID %s not found in 1CV8Clst.lst", infobaseGUID)
}

// GetClusterGUIDForReg reads cluster GUID for a given reg_<port> directory
func GetClusterGUIDForReg(regDir string) (string, error) {
	info, err := ReadClusterInfo(regDir)
	if err != nil {
		// Fallback: extract port from directory name and use as identifier
		regName := filepath.Base(regDir)
		if strings.HasPrefix(strings.ToLower(regName), "reg_") {
			log.Warn().
				Err(err).
				Str("reg_dir", regName).
				Msg("Failed to read cluster GUID, using reg_<port> as identifier")
			return regName, nil
		}
		return "", err
	}

	return info.GUID, nil
}

// FindRegDirByClusterGUID searches for reg_<port> directory that contains the given cluster_guid
// Searches in provided searchDirs (e.g., /mnt/logs in Docker which is mounted from srvinfo)
func FindRegDirByClusterGUID(clusterGUID string, searchDirs []string) (string, error) {
	// Normalize cluster_guid to lowercase for comparison
	clusterGUIDLower := strings.ToLower(clusterGUID)

	log.Info().
		Str("cluster_guid", clusterGUID).
		Strs("search_dirs", searchDirs).
		Msg("FindRegDirByClusterGUID called")

	// searchDirs already contains srvinfo paths (e.g., /mnt/logs in Docker = C:\Program Files\1cv8\srvinfo on host)
	// No need to extract or parse - use directly
	uniqueSearchPaths := searchDirs

	// Search in each srvinfo directory
	for _, srvinfoPath := range uniqueSearchPaths {
		log.Info().
			Str("srvinfo_path", srvinfoPath).
			Msg("Checking srvinfo path")

		if _, err := os.Stat(srvinfoPath); os.IsNotExist(err) {
			log.Warn().
				Str("srvinfo_path", srvinfoPath).
				Msg("srvinfo path does not exist")
			continue // Skip if directory doesn't exist
		}

		// Look for reg_* directories
		entries, err := os.ReadDir(srvinfoPath)
		if err != nil {
			log.Debug().Err(err).Str("path", srvinfoPath).Msg("Failed to read srvinfo directory")
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			regName := strings.ToLower(entry.Name())
			if !strings.HasPrefix(regName, "reg_") {
				continue
			}

			regDir := filepath.Join(srvinfoPath, entry.Name())

			// Try to read cluster GUID from this reg_<port> directory
			regClusterGUID, err := GetClusterGUIDForReg(regDir)
			if err != nil {
				log.Debug().Err(err).Str("reg_dir", regDir).Msg("Failed to read cluster GUID from reg directory")
				continue
			}

			// Compare GUIDs (case-insensitive)
			if strings.ToLower(regClusterGUID) == clusterGUIDLower {
				log.Debug().
					Str("cluster_guid", clusterGUID).
					Str("reg_dir", regDir).
					Msg("Found reg directory for cluster GUID")
				return regDir, nil
			}
		}
	}

	return "", fmt.Errorf("reg_<port> directory not found for cluster_guid: %s", clusterGUID)
}

// GetClusterAndInfobaseNames reads cluster_name and infobase_name from 1CV8Clst.lst
// by searching for reg_<port> directory that contains the given cluster_guid
func GetClusterAndInfobaseNames(clusterGUID, infobaseGUID string, searchDirs []string) (clusterName, infobaseName string, err error) {
	// Find reg_<port> directory for this cluster_guid
	regDir, err := FindRegDirByClusterGUID(clusterGUID, searchDirs)
	if err != nil {
		log.Debug().
			Err(err).
			Str("cluster_guid", clusterGUID).
			Msg("Failed to find reg directory, cluster and infobase names will be empty")
		return "", "", nil // Return empty strings, not error (graceful degradation)
	}

	// Read cluster file data
	clusterFileData, err := ReadClusterFileData(regDir)
	if err != nil {
		log.Debug().
			Err(err).
			Str("reg_dir", regDir).
			Msg("Failed to read cluster file data, cluster and infobase names will be empty")
		return "", "", nil // Return empty strings, not error (graceful degradation)
	}

	clusterName = clusterFileData.Cluster.Name

	// Try to get infobase name
	if infobase, ok := clusterFileData.Infobases[infobaseGUID]; ok {
		infobaseName = infobase.Name
		log.Debug().
			Str("cluster_guid", clusterGUID).
			Str("cluster_name", clusterName).
			Str("infobase_guid", infobaseGUID).
			Str("infobase_name", infobaseName).
			Msg("Found cluster and infobase names from 1CV8Clst.lst")
	} else {
		log.Debug().
			Str("infobase_guid", infobaseGUID).
			Msg("Infobase GUID not found in 1CV8Clst.lst, infobase name will be empty")
	}

	return clusterName, infobaseName, nil
}
