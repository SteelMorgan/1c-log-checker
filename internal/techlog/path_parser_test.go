package techlog

import (
	"testing"
)

func TestExtractGUIDsFromPath(t *testing.T) {
	tests := []struct {
		name             string
		path             string
		wantClusterGUID  string
		wantInfobaseGUID string
		wantErr          bool
	}{
		{
			name:             "Valid Windows path with GUIDs",
			path:             `D:\TechLogs\b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3\d723aefd-7992-420d-b5f9-a273fd4146be\rphost_1234\25011408.log`,
			wantClusterGUID:  "b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3",
			wantInfobaseGUID: "d723aefd-7992-420d-b5f9-a273fd4146be",
			wantErr:          false,
		},
		{
			name:             "Valid Linux path with GUIDs",
			path:             `/var/log/1c/b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3/d723aefd-7992-420d-b5f9-a273fd4146be/rphost_5678/25011409.log`,
			wantClusterGUID:  "b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3",
			wantInfobaseGUID: "d723aefd-7992-420d-b5f9-a273fd4146be",
			wantErr:          false,
		},
		{
			name:             "Valid path with subdirectories in process folder",
			path:             `D:\TechLogs\b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3\d723aefd-7992-420d-b5f9-a273fd4146be\rphost_1234\subdir\25011408.log`,
			wantClusterGUID:  "b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3",
			wantInfobaseGUID: "d723aefd-7992-420d-b5f9-a273fd4146be",
			wantErr:          false,
		},
		{
			name:             "Valid path with uppercase GUIDs (should normalize to lowercase)",
			path:             `D:\TechLogs\B0881663-F2A7-4195-B7A2-F7F8E6C3A8F3\D723AEFD-7992-420D-B5F9-A273FD4146BE\rphost_1234\25011408.log`,
			wantClusterGUID:  "b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3",
			wantInfobaseGUID: "d723aefd-7992-420d-b5f9-a273fd4146be",
			wantErr:          false,
		},
		{
			name:    "Missing GUIDs - only one GUID",
			path:    `D:\TechLogs\b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3\rphost_1234\25011408.log`,
			wantErr: true,
		},
		{
			name:    "Missing GUIDs - no GUIDs at all",
			path:    `D:\TechLogs\cluster1\base1\rphost_1234\25011408.log`,
			wantErr: true,
		},
		{
			name:    "Path too short",
			path:    `rphost_1234\25011408.log`,
			wantErr: true,
		},
		{
			name:    "Empty path",
			path:    ``,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotClusterGUID, gotInfobaseGUID, err := ExtractGUIDsFromPath(tt.path)

			if (err != nil) != tt.wantErr {
				t.Errorf("extractGUIDsFromPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if gotClusterGUID != tt.wantClusterGUID {
					t.Errorf("extractGUIDsFromPath() clusterGUID = %v, want %v", gotClusterGUID, tt.wantClusterGUID)
				}
				if gotInfobaseGUID != tt.wantInfobaseGUID {
					t.Errorf("extractGUIDsFromPath() infobaseGUID = %v, want %v", gotInfobaseGUID, tt.wantInfobaseGUID)
				}
			}
		})
	}
}

func TestValidateTechLogPath(t *testing.T) {
	tests := []struct {
		name                 string
		path                 string
		expectedClusterGUID  string
		expectedInfobaseGUID string
		techLogDirs          []string
		wantErr              bool
		errContains          string
	}{
		{
			name:                 "Valid path with matching GUIDs",
			path:                 `D:\TechLogs\b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3\d723aefd-7992-420d-b5f9-a273fd4146be\rphost_1234\25011408.log`,
			expectedClusterGUID:  "b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3",
			expectedInfobaseGUID: "d723aefd-7992-420d-b5f9-a273fd4146be",
			techLogDirs:          []string{`D:\TechLogs`},
			wantErr:              false,
		},
		{
			name:                 "Valid path with case-insensitive matching",
			path:                 `D:\TechLogs\B0881663-F2A7-4195-B7A2-F7F8E6C3A8F3\D723AEFD-7992-420D-B5F9-A273FD4146BE\rphost_1234\25011408.log`,
			expectedClusterGUID:  "b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3",
			expectedInfobaseGUID: "d723aefd-7992-420d-b5f9-a273fd4146be",
			techLogDirs:          []string{`D:\TechLogs`},
			wantErr:              false,
		},
		{
			name:                 "Cluster GUID mismatch",
			path:                 `D:\TechLogs\aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa\d723aefd-7992-420d-b5f9-a273fd4146be\rphost_1234\25011408.log`,
			expectedClusterGUID:  "b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3",
			expectedInfobaseGUID: "d723aefd-7992-420d-b5f9-a273fd4146be",
			techLogDirs:          []string{`D:\TechLogs`},
			wantErr:              true,
			errContains:          "cluster_guid mismatch",
		},
		{
			name:                 "Infobase GUID mismatch",
			path:                 `D:\TechLogs\b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3\bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb\rphost_1234\25011408.log`,
			expectedClusterGUID:  "b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3",
			expectedInfobaseGUID: "d723aefd-7992-420d-b5f9-a273fd4146be",
			techLogDirs:          []string{`D:\TechLogs`},
			wantErr:              true,
			errContains:          "infobase_guid mismatch",
		},
		{
			name:                 "Missing GUIDs in path",
			path:                 `D:\TechLogs\cluster1\base1\rphost_1234\25011408.log`,
			expectedClusterGUID:  "b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3",
			expectedInfobaseGUID: "d723aefd-7992-420d-b5f9-a273fd4146be",
			techLogDirs:          []string{`D:\TechLogs`},
			wantErr:              true,
			errContains:          "expected 2 GUIDs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTechLogPath(tt.path, tt.expectedClusterGUID, tt.expectedInfobaseGUID, tt.techLogDirs)

			if (err != nil) != tt.wantErr {
				t.Errorf("validateTechLogPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.errContains != "" {
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("validateTechLogPath() error = %v, should contain %q", err, tt.errContains)
				}
			}
		})
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
