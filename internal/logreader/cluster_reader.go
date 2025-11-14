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
//              {<infobase_guid>,"<infobase_name>",...}
//              ...
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

