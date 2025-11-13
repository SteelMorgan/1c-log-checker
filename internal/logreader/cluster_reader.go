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

// ReadClusterInfo reads cluster GUID from 1CV8Clst.lst file
// File format (partial): {b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3,"Локальный кластер",1541,...}
func ReadClusterInfo(regDir string) (*ClusterInfo, error) {
	// Path to 1CV8Clst.lst
	clstPath := filepath.Join(regDir, "1CV8Clst.lst")
	
	data, err := os.ReadFile(clstPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read 1CV8Clst.lst: %w", err)
	}
	
	content := string(data)
	
	// Extract cluster GUID using regex
	// Pattern: {<guid>,"<name>",<port>,...}
	// First occurrence is the cluster info
	guidPattern := regexp.MustCompile(`\{([a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}),"([^"]+)",(\d+)`)
	
	matches := guidPattern.FindStringSubmatch(content)
	if len(matches) < 4 {
		return nil, fmt.Errorf("failed to parse cluster info from 1CV8Clst.lst")
	}
	
	info := &ClusterInfo{
		GUID: matches[1],
		Name: matches[2],
		Port: 0, // We already know port from reg_<port>
	}
	
	log.Debug().
		Str("cluster_guid", info.GUID).
		Str("cluster_name", info.Name).
		Msg("Read cluster info from 1CV8Clst.lst")
	
	return info, nil
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

