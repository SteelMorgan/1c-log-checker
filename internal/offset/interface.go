package offset

import (
	"context"
)

// OffsetStore stores and retrieves file read offsets
// Implementations: BoltDB (primary), ClickHouse (optional mirror)
type OffsetStore interface {
	// Get retrieves the offset for a given file
	// Returns 0 if no offset is stored
	Get(ctx context.Context, sourceType, filePath string) (uint64, error)
	
	// Set stores the offset for a given file
	Set(ctx context.Context, sourceType, filePath string, offset uint64) error
	
	// Delete removes the offset for a given file
	Delete(ctx context.Context, sourceType, filePath string) error
	
	// List returns all stored offsets
	List(ctx context.Context) (map[string]uint64, error)
	
	// Close closes the offset store
	Close() error
}

// FileInfo contains metadata about a log file
type FileInfo struct {
	Path   string
	Inode  uint64
	Size   uint64
	Offset uint64
}

