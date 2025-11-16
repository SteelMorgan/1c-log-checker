package offset

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"go.etcd.io/bbolt"
)

const (
	bucketName        = "offsets"
	techlogBucketName = "techlog_offsets"
)

// BoltDBStore implements OffsetStore using BoltDB
type BoltDBStore struct {
	db *bbolt.DB
}

// NewBoltDBStore creates a new BoltDB offset store
func NewBoltDBStore(dbPath string) (*BoltDBStore, error) {
	// Try to open with short timeout
	db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{
		Timeout: 1 * time.Second,
	})
	if err != nil {
		// If file is locked, it means another process is holding it
		// This usually happens when previous process was killed without graceful shutdown
		// We can't automatically unlock it - user needs to stop the process manually
		return nil, fmt.Errorf("failed to open boltdb (file may be locked by another process): %w", err)
	}
	
	// Create buckets if not exists
	err = db.Update(func(tx *bbolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(bucketName)); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(techlogBucketName)); err != nil {
		return err
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create buckets: %w", err)
	}
	
	log.Info().
		Str("db_path", dbPath).
		Msg("BoltDB offset store initialized")
	
	return &BoltDBStore{db: db}, nil
}

// Get retrieves the offset for a given file
func (s *BoltDBStore) Get(ctx context.Context, sourceType, filePath string) (uint64, error) {
	var offset uint64
	
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}
		
		key := makeKey(sourceType, filePath)
		val := b.Get([]byte(key))
		if val == nil {
			offset = 0
			return nil
		}
		
		if len(val) < 8 {
			return fmt.Errorf("invalid offset value")
		}
		
		offset = binary.BigEndian.Uint64(val)
		return nil
	})
	
	if err != nil {
		return 0, fmt.Errorf("failed to get offset: %w", err)
	}
	
	return offset, nil
}

// Set stores the offset for a given file
func (s *BoltDBStore) Set(ctx context.Context, sourceType, filePath string, offset uint64) error {
	err := s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}
		
		key := makeKey(sourceType, filePath)
		val := make([]byte, 8)
		binary.BigEndian.PutUint64(val, offset)
		
		return b.Put([]byte(key), val)
	})
	
	if err != nil {
		return fmt.Errorf("failed to set offset: %w", err)
	}
	
	log.Debug().
		Str("source_type", sourceType).
		Str("file_path", filePath).
		Uint64("offset", offset).
		Msg("Offset updated")
	
	return nil
}

// Delete removes the offset for a given file
func (s *BoltDBStore) Delete(ctx context.Context, sourceType, filePath string) error {
	err := s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}
		
		key := makeKey(sourceType, filePath)
		return b.Delete([]byte(key))
	})
	
	if err != nil {
		return fmt.Errorf("failed to delete offset: %w", err)
	}
	
	return nil
}

// List returns all stored offsets
func (s *BoltDBStore) List(ctx context.Context) (map[string]uint64, error) {
	result := make(map[string]uint64)
	
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}
		
		return b.ForEach(func(k, v []byte) error {
			if len(v) >= 8 {
				offset := binary.BigEndian.Uint64(v)
				result[string(k)] = offset
			}
			return nil
		})
	})
	
	if err != nil {
		return nil, fmt.Errorf("failed to list offsets: %w", err)
	}
	
	return result, nil
}

// Close closes the BoltDB database
func (s *BoltDBStore) Close() error {
	log.Info().Msg("Closing BoltDB offset store")
	return s.db.Close()
}

// makeKey creates a composite key from source type and file path
func makeKey(sourceType, filePath string) string {
	return fmt.Sprintf("%s:%s", sourceType, filePath)
}

// TechLogOffset represents offset information for tech log files
type TechLogOffset struct {
	FilePath      string
	OffsetBytes   int64
	LastTimestamp time.Time
	LastLine      int64
}

// GetTechLogOffset retrieves the offset for a tech log file
func (s *BoltDBStore) GetTechLogOffset(ctx context.Context, filePath string) (*TechLogOffset, error) {
	var offset *TechLogOffset
	
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(techlogBucketName))
		if b == nil {
			return fmt.Errorf("techlog bucket not found")
		}
		
		val := b.Get([]byte(filePath))
		if val == nil {
			offset = nil // No offset stored
			return nil
		}
		
		// Deserialize offset
		var err error
		offset, err = deserializeTechLogOffset(val)
		return err
	})
	
	if err != nil {
		return nil, fmt.Errorf("failed to get techlog offset: %w", err)
	}
	
	return offset, nil
}

// SaveTechLogOffset stores the offset for a tech log file
func (s *BoltDBStore) SaveTechLogOffset(ctx context.Context, offset *TechLogOffset) error {
	if offset == nil {
		return fmt.Errorf("offset cannot be nil")
	}
	
	err := s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(techlogBucketName))
		if b == nil {
			return fmt.Errorf("techlog bucket not found")
		}
		
		// Serialize offset
		val, err := serializeTechLogOffset(offset)
		if err != nil {
			return err
		}
		
		return b.Put([]byte(offset.FilePath), val)
	})
	
	if err != nil {
		return fmt.Errorf("failed to save techlog offset: %w", err)
	}
	
	log.Debug().
		Str("file_path", offset.FilePath).
		Int64("offset_bytes", offset.OffsetBytes).
		Int64("last_line", offset.LastLine).
		Time("last_timestamp", offset.LastTimestamp).
		Msg("TechLog offset updated")
	
	return nil
}

// serializeTechLogOffset serializes TechLogOffset to bytes
// Format: [8 bytes: OffsetBytes][8 bytes: LastLine][8 bytes: LastTimestamp UnixNano][...FilePath...]
func serializeTechLogOffset(offset *TechLogOffset) ([]byte, error) {
	// Calculate size: 8 (OffsetBytes) + 8 (LastLine) + 8 (Timestamp) + 4 (FilePath length) + len(FilePath)
	filePathBytes := []byte(offset.FilePath)
	buf := make([]byte, 8+8+8+4+len(filePathBytes))
	
	pos := 0
	
	// OffsetBytes (int64 = 8 bytes)
	binary.BigEndian.PutUint64(buf[pos:], uint64(offset.OffsetBytes))
	pos += 8
	
	// LastLine (int64 = 8 bytes)
	binary.BigEndian.PutUint64(buf[pos:], uint64(offset.LastLine))
	pos += 8
	
	// LastTimestamp (UnixNano as int64 = 8 bytes)
	binary.BigEndian.PutUint64(buf[pos:], uint64(offset.LastTimestamp.UnixNano()))
	pos += 8
	
	// FilePath length (uint32 = 4 bytes)
	binary.BigEndian.PutUint32(buf[pos:], uint32(len(filePathBytes)))
	pos += 4
	
	// FilePath
	copy(buf[pos:], filePathBytes)
	
	return buf, nil
}

// deserializeTechLogOffset deserializes bytes to TechLogOffset
func deserializeTechLogOffset(data []byte) (*TechLogOffset, error) {
	if len(data) < 28 { // Minimum: 8+8+8+4 = 28 bytes
		return nil, fmt.Errorf("invalid offset data: too short")
	}
	
	pos := 0
	
	// OffsetBytes
	offsetBytes := int64(binary.BigEndian.Uint64(data[pos:]))
	pos += 8
	
	// LastLine
	lastLine := int64(binary.BigEndian.Uint64(data[pos:]))
	pos += 8
	
	// LastTimestamp
	timestampNano := int64(binary.BigEndian.Uint64(data[pos:]))
	pos += 8
	
	// FilePath length
	filePathLen := binary.BigEndian.Uint32(data[pos:])
	pos += 4
	
	if len(data) < pos+int(filePathLen) {
		return nil, fmt.Errorf("invalid offset data: filepath length mismatch")
	}
	
	// FilePath
	filePath := string(data[pos : pos+int(filePathLen)])
	
	return &TechLogOffset{
		FilePath:      filePath,
		OffsetBytes:   offsetBytes,
		LastTimestamp: time.Unix(0, timestampNano),
		LastLine:      lastLine,
	}, nil
}

