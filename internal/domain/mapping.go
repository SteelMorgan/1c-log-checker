package domain

import (
	"time"

	"github.com/google/uuid"
)

// UserMapping represents a mapping from user GUID to user name
type UserMapping struct {
	InfobaseGUID  string    `json:"infobase_guid" db:"infobase_guid"`
	UserGUID      uuid.UUID `json:"user_guid" db:"user_guid"`
	UserName      string    `json:"user_name" db:"user_name"`
	Department    string    `json:"department,omitempty" db:"department"`
	Email         string    `json:"email,omitempty" db:"email"`
	IsActive      bool      `json:"is_active" db:"is_active"`
	SyncTimestamp time.Time `json:"sync_timestamp" db:"sync_timestamp"`
	Version       uint64    `json:"version" db:"version"`
}

// MetadataMapping represents a mapping from metadata GUID to metadata name
type MetadataMapping struct {
	InfobaseGUID  string    `json:"infobase_guid" db:"infobase_guid"`
	MetadataGUID  uuid.UUID `json:"metadata_guid" db:"metadata_guid"`
	MetadataName  string    `json:"metadata_name" db:"metadata_name"` // e.g., "Документ.ПоступлениеТоваров"
	MetadataType  string    `json:"metadata_type" db:"metadata_type"` // "Document", "Catalog", etc.
	ParentGUID    uuid.UUID `json:"parent_guid,omitempty" db:"parent_guid"`
	IsActive      bool      `json:"is_active" db:"is_active"`
	SyncTimestamp time.Time `json:"sync_timestamp" db:"sync_timestamp"`
	Version       uint64    `json:"version" db:"version"`
}

// DataMapping represents a mapping from data GUID to data presentation
type DataMapping struct {
	InfobaseGUID     string    `json:"infobase_guid" db:"infobase_guid"`
	DataGUID         uuid.UUID `json:"data_guid" db:"data_guid"`
	MetadataGUID     uuid.UUID `json:"metadata_guid" db:"metadata_guid"`
	DataPresentation string    `json:"data_presentation" db:"data_presentation"` // e.g., "ПоступлениеТоваров №00001 от 01.01.2025"
	IsDeleted        bool      `json:"is_deleted" db:"is_deleted"`
	ModifiedAt       time.Time `json:"modified_at" db:"modified_at"`
	SyncTimestamp    time.Time `json:"sync_timestamp" db:"sync_timestamp"`
	Version          uint64    `json:"version" db:"version"`
}

// MappingSyncStats tracks synchronization status
type MappingSyncStats struct {
	InfobaseGUID   string    `json:"infobase_guid" db:"infobase_guid"`
	SyncType       string    `json:"sync_type" db:"sync_type"` // "users", "metadata", "data"
	LastSyncTime   time.Time `json:"last_sync_time" db:"last_sync_time"`
	RecordsSynced  uint64    `json:"records_synced" db:"records_synced"`
	SyncDurationMs uint64    `json:"sync_duration_ms" db:"sync_duration_ms"`
	SyncStatus     string    `json:"sync_status" db:"sync_status"` // "success", "error", "partial"
	ErrorMessage   string    `json:"error_message,omitempty" db:"error_message"`
	Version        uint64    `json:"version" db:"version"`
}

// --- Sync Request/Response formats ---

// UserSyncRequest represents request to 1C for user mappings
type UserSyncRequest struct {
	InfobaseGUID  string    `json:"infobase_guid"`
	LastSyncTime  time.Time `json:"last_sync_time,omitempty"` // For incremental sync
	IncludeFields []string  `json:"include_fields,omitempty"`  // ["department", "email"]
}

// UserSyncResponse represents response from 1C with user mappings
type UserSyncResponse struct {
	InfobaseGUID string           `json:"infobase_guid"`
	Timestamp    time.Time        `json:"timestamp"`
	Users        []UserMappingDTO `json:"users"`
	TotalCount   int              `json:"total_count"`
	HasMore      bool             `json:"has_more"`      // For pagination
	NextOffset   int              `json:"next_offset"`   // For pagination
}

// UserMappingDTO is the data transfer object for user mapping (from 1C)
type UserMappingDTO struct {
	UserGUID   string `json:"user_guid"`   // UUID as string from 1C
	UserName   string `json:"user_name"`
	Department string `json:"department,omitempty"`
	Email      string `json:"email,omitempty"`
	IsActive   bool   `json:"is_active"`
}

// MetadataSyncRequest represents request to 1C for metadata mappings
type MetadataSyncRequest struct {
	InfobaseGUID  string    `json:"infobase_guid"`
	LastSyncTime  time.Time `json:"last_sync_time,omitempty"`
	MetadataTypes []string  `json:"metadata_types,omitempty"` // ["Document", "Catalog"]
}

// MetadataSyncResponse represents response from 1C with metadata mappings
type MetadataSyncResponse struct {
	InfobaseGUID string               `json:"infobase_guid"`
	Timestamp    time.Time            `json:"timestamp"`
	Metadata     []MetadataMappingDTO `json:"metadata"`
	TotalCount   int                  `json:"total_count"`
	HasMore      bool                 `json:"has_more"`
	NextOffset   int                  `json:"next_offset"`
}

// MetadataMappingDTO is the data transfer object for metadata mapping
type MetadataMappingDTO struct {
	MetadataGUID string `json:"metadata_guid"` // UUID as string from 1C
	MetadataName string `json:"metadata_name"`
	MetadataType string `json:"metadata_type"`
	ParentGUID   string `json:"parent_guid,omitempty"`
	IsActive     bool   `json:"is_active"`
}

// DataSyncRequest represents request to 1C for data object mappings
type DataSyncRequest struct {
	InfobaseGUID string    `json:"infobase_guid"`
	MetadataGUID string    `json:"metadata_guid,omitempty"` // Filter by metadata type
	LastSyncTime time.Time `json:"last_sync_time,omitempty"`
	Limit        int       `json:"limit,omitempty"` // Pagination limit
	Offset       int       `json:"offset,omitempty"`
}

// DataSyncResponse represents response from 1C with data object mappings
type DataSyncResponse struct {
	InfobaseGUID string           `json:"infobase_guid"`
	Timestamp    time.Time        `json:"timestamp"`
	Data         []DataMappingDTO `json:"data"`
	TotalCount   int              `json:"total_count"`
	HasMore      bool             `json:"has_more"`
	NextOffset   int              `json:"next_offset"`
}

// DataMappingDTO is the data transfer object for data mapping
type DataMappingDTO struct {
	DataGUID         string    `json:"data_guid"`
	MetadataGUID     string    `json:"metadata_guid"`
	DataPresentation string    `json:"data_presentation"`
	IsDeleted        bool      `json:"is_deleted"`
	ModifiedAt       time.Time `json:"modified_at"`
}
