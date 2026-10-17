package offset

import (
	"context"
	"fmt"
	"go.etcd.io/bbolt"
)

const initialTailBucket = "eventlog_initial_tail_v1"

func (s *BoltDBStore) EventLogTailInitialized(root string) (bool, error) {
	var done bool
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(initialTailBucket))
		done = b != nil && b.Get([]byte(root)) != nil
		return nil
	})
	return done, err
}

// Размеры файлов и маркер корня фиксируются одной транзакцией. Повторный запуск
// не пропускает новые файлы, появившиеся после первоначального snapshot.
func (s *BoltDBStore) InitializeEventLogTail(ctx context.Context, root string, sizes map[string]int64) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		markers, err := tx.CreateBucketIfNotExists([]byte(initialTailBucket))
		if err != nil {
			return err
		}
		if markers.Get([]byte(root)) != nil {
			return nil
		}
		b := tx.Bucket([]byte(eventlogBucketName))
		if b == nil {
			return fmt.Errorf("eventlog offsets missing")
		}
		for path, size := range sizes {
			if size < 0 {
				return fmt.Errorf("negative initial size")
			}
			if b.Get([]byte(path)) != nil {
				continue
			}
			raw, err := serializeEventLogOffset(&EventLogOffset{FilePath: path, OffsetBytes: size, InitialTail: size > 0})
			if err != nil {
				return err
			}
			if err = b.Put([]byte(path), raw); err != nil {
				return err
			}
		}
		return markers.Put([]byte(root), []byte{1})
	})
}
