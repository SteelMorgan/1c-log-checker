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
	bucketName = "offsets"
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
	
	// Create bucket if not exists
	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		return err
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create bucket: %w", err)
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

