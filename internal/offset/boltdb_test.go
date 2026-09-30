package offset

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewBoltDBStore(t *testing.T) {
	// Create temporary directory
	tmpDir := filepath.Join(os.TempDir(), "boltdb_test")
	defer os.RemoveAll(tmpDir)
	
	dbPath := filepath.Join(tmpDir, "test.db")
	
	store, err := NewBoltDBStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create BoltDB store: %v", err)
	}
	defer store.Close()
	
	if store == nil {
		t.Fatal("Store should not be nil")
	}
}

func TestBoltDBStore_Get_NonExistent(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "boltdb_test_get")
	defer os.RemoveAll(tmpDir)
	
	dbPath := filepath.Join(tmpDir, "test.db")
	
	store, err := NewBoltDBStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create BoltDB store: %v", err)
	}
	defer store.Close()
	
	ctx := context.Background()
	offset, err := store.Get(ctx, "test", "nonexistent.txt")
	if err != nil {
		t.Errorf("Get() should not return error for non-existent key: %v", err)
	}
	
	if offset != 0 {
		t.Errorf("Expected offset 0 for non-existent key, got %d", offset)
	}
}

func TestBoltDBStore_SetAndGet(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "boltdb_test_setget")
	defer os.RemoveAll(tmpDir)
	
	dbPath := filepath.Join(tmpDir, "test.db")
	
	store, err := NewBoltDBStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create BoltDB store: %v", err)
	}
	defer store.Close()
	
	ctx := context.Background()
	
	// Set offset
	err = store.Set(ctx, "test", "file.txt", 12345)
	if err != nil {
		t.Fatalf("Set() failed: %v", err)
	}
	
	// Get offset
	offset, err := store.Get(ctx, "test", "file.txt")
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}
	
	if offset != 12345 {
		t.Errorf("Expected offset 12345, got %d", offset)
	}
}

func TestBoltDBStore_Delete(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "boltdb_test_delete")
	defer os.RemoveAll(tmpDir)
	
	dbPath := filepath.Join(tmpDir, "test.db")
	
	store, err := NewBoltDBStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create BoltDB store: %v", err)
	}
	defer store.Close()
	
	ctx := context.Background()
	
	// Set offset
	err = store.Set(ctx, "test", "file.txt", 100)
	if err != nil {
		t.Fatalf("Set() failed: %v", err)
	}
	
	// Delete offset
	err = store.Delete(ctx, "test", "file.txt")
	if err != nil {
		t.Fatalf("Delete() failed: %v", err)
	}
	
	// Get should return 0
	offset, err := store.Get(ctx, "test", "file.txt")
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}
	
	if offset != 0 {
		t.Errorf("Expected offset 0 after delete, got %d", offset)
	}
}

func TestBoltDBStore_List(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "boltdb_test_list")
	defer os.RemoveAll(tmpDir)
	
	dbPath := filepath.Join(tmpDir, "test.db")
	
	store, err := NewBoltDBStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create BoltDB store: %v", err)
	}
	defer store.Close()
	
	ctx := context.Background()
	
	// Set multiple offsets
	store.Set(ctx, "test", "file1.txt", 100)
	store.Set(ctx, "test", "file2.txt", 200)
	store.Set(ctx, "test", "file3.txt", 300)
	
	// List offsets
	offsets, err := store.List(ctx, "test")
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}
	
	if len(offsets) != 3 {
		t.Errorf("Expected 3 offsets, got %d", len(offsets))
	}
}

func TestBoltDBStore_GetTechLogOffset(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "boltdb_test_techlog")
	defer os.RemoveAll(tmpDir)
	
	dbPath := filepath.Join(tmpDir, "test.db")
	
	store, err := NewBoltDBStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create BoltDB store: %v", err)
	}
	defer store.Close()
	
	ctx := context.Background()
	
	// Get non-existent offset
	offset, err := store.GetTechLogOffset(ctx, "file.txt")
	if err != nil {
		t.Fatalf("GetTechLogOffset() failed: %v", err)
	}
	
	if offset != nil {
		t.Error("Expected nil offset for non-existent file")
	}
}

func TestBoltDBStore_SaveTechLogOffset(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "boltdb_test_savetechlog")
	defer os.RemoveAll(tmpDir)
	
	dbPath := filepath.Join(tmpDir, "test.db")
	
	store, err := NewBoltDBStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create BoltDB store: %v", err)
	}
	defer store.Close()
	
	ctx := context.Background()
	
	offset := &TechLogOffset{
		FilePath:     "file.txt",
		OffsetBytes:  1000,
		LastModified: 1234567890,
	}
	
	err = store.SaveTechLogOffset(ctx, offset)
	if err != nil {
		t.Fatalf("SaveTechLogOffset() failed: %v", err)
	}
	
	// Get it back
	retrieved, err := store.GetTechLogOffset(ctx, "file.txt")
	if err != nil {
		t.Fatalf("GetTechLogOffset() failed: %v", err)
	}
	
	if retrieved == nil {
		t.Fatal("Expected non-nil offset")
	}
	
	if retrieved.FilePath != "file.txt" {
		t.Errorf("Expected FilePath 'file.txt', got '%s'", retrieved.FilePath)
	}
	
	if retrieved.OffsetBytes != 1000 {
		t.Errorf("Expected OffsetBytes 1000, got %d", retrieved.OffsetBytes)
	}
}

