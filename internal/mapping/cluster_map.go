package mapping

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ClusterInfo contains cluster metadata
type ClusterInfo struct {
	Name  string `yaml:"name"`
	Notes string `yaml:"notes"`
}

// InfobaseInfo contains infobase metadata
type InfobaseInfo struct {
	Name        string `yaml:"name"`
	ClusterGUID string `yaml:"cluster_guid"`
	Notes       string `yaml:"notes"`
}

// ClusterMap maps GUIDs to human-readable names
type ClusterMap struct {
	Clusters  map[string]ClusterInfo  `yaml:"clusters"`
	Infobases map[string]InfobaseInfo `yaml:"infobases"`
}

// LoadClusterMap loads cluster_map.yaml
func LoadClusterMap(path string) (*ClusterMap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read cluster map: %w", err)
	}
	
	var cm ClusterMap
	if err := yaml.Unmarshal(data, &cm); err != nil {
		return nil, fmt.Errorf("failed to parse cluster map: %w", err)
	}
	
	// Initialize maps if nil
	if cm.Clusters == nil {
		cm.Clusters = make(map[string]ClusterInfo)
	}
	if cm.Infobases == nil {
		cm.Infobases = make(map[string]InfobaseInfo)
	}
	
	return &cm, nil
}

// GetClusterName returns cluster name by GUID
// Returns GUID itself if not found in map
func (cm *ClusterMap) GetClusterName(guid string) string {
	if info, ok := cm.Clusters[guid]; ok {
		return info.Name
	}
	return guid // Fallback to GUID
}

// GetInfobaseName returns infobase name by GUID
// Returns GUID itself if not found in map
func (cm *ClusterMap) GetInfobaseName(guid string) string {
	if info, ok := cm.Infobases[guid]; ok {
		return info.Name
	}
	return guid // Fallback to GUID
}

// GetClusterGUIDForInfobase returns cluster GUID for a given infobase GUID
func (cm *ClusterMap) GetClusterGUIDForInfobase(infobaseGUID string) string {
	if info, ok := cm.Infobases[infobaseGUID]; ok {
		return info.ClusterGUID
	}
	return "" // Unknown
}

