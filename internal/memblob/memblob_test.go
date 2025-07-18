package memblob

import (
	"context"
	"testing"
)

func TestMemBlobStageAndCommit(t *testing.T) {
	store := New()
	ctx := context.Background()
	data := []byte("test data")
	
	// Stage data
	if err := store.Stage(ctx, 1, data); err != nil {
		t.Fatalf("stage failed: %v", err)
	}
	
	// Commit data
	if err := store.Commit(ctx, 1); err != nil {
		t.Fatalf("commit failed: %v", err)
	}
	
	// Verify latest version
	latest, err := store.Latest(ctx)
	if err != nil {
		t.Fatalf("latest failed: %v", err)
	}
	if latest != 1 {
		t.Fatalf("expected latest version 1, got %d", latest)
	}
	
	// Retrieve data
	retrieved, err := store.Retrieve(ctx, 1)
	if err != nil {
		t.Fatalf("retrieve failed: %v", err)
	}
	
	if string(retrieved) != string(data) {
		t.Fatalf("expected %s, got %s", string(data), string(retrieved))
	}
}

func TestMemBlobCommitWithoutStage(t *testing.T) {
	store := New()
	ctx := context.Background()
	
	// Try to commit without staging
	err := store.Commit(ctx, 1)
	if err == nil {
		t.Fatal("expected error when committing without staging")
	}
}

func TestMemBlobRetrieveNonExistent(t *testing.T) {
	store := New()
	ctx := context.Background()
	
	// Try to retrieve non-existent version
	_, err := store.Retrieve(ctx, 999)
	if err == nil {
		t.Fatal("expected error when retrieving non-existent version")
	}
}

func TestMemBlobMultipleVersions(t *testing.T) {
	store := New()
	ctx := context.Background()
	
	// Stage and commit multiple versions
	for i := 1; i <= 3; i++ {
		data := []byte("data version " + string(rune('0'+i)))
		if err := store.Stage(ctx, uint64(i), data); err != nil {
			t.Fatalf("stage version %d failed: %v", i, err)
		}
		if err := store.Commit(ctx, uint64(i)); err != nil {
			t.Fatalf("commit version %d failed: %v", i, err)
		}
	}
	
	// Verify latest version
	latest, err := store.Latest(ctx)
	if err != nil {
		t.Fatalf("latest failed: %v", err)
	}
	if latest != 3 {
		t.Fatalf("expected latest version 3, got %d", latest)
	}
	
	// Verify all versions are retrievable
	for i := 1; i <= 3; i++ {
		data, err := store.Retrieve(ctx, uint64(i))
		if err != nil {
			t.Fatalf("retrieve version %d failed: %v", i, err)
		}
		expected := "data version " + string(rune('0'+i))
		if string(data) != expected {
			t.Fatalf("expected %s, got %s", expected, string(data))
		}
	}
}

func TestMemBlobDataIsolation(t *testing.T) {
	store := New()
	ctx := context.Background()
	
	// Stage data
	original := []byte("original data")
	if err := store.Stage(ctx, 1, original); err != nil {
		t.Fatalf("stage failed: %v", err)
	}
	
	// Modify original data
	original[0] = 'X'
	
	// Commit and retrieve
	if err := store.Commit(ctx, 1); err != nil {
		t.Fatalf("commit failed: %v", err)
	}
	
	retrieved, err := store.Retrieve(ctx, 1)
	if err != nil {
		t.Fatalf("retrieve failed: %v", err)
	}
	
	// Should still be original data, not modified
	if string(retrieved) != "original data" {
		t.Fatalf("expected 'original data', got '%s'", string(retrieved))
	}
	
	// Modify retrieved data
	retrieved[0] = 'Y'
	
	// Retrieve again to ensure isolation
	retrieved2, err := store.Retrieve(ctx, 1)
	if err != nil {
		t.Fatalf("second retrieve failed: %v", err)
	}
	
	if string(retrieved2) != "original data" {
		t.Fatalf("expected 'original data', got '%s'", string(retrieved2))
	}
}

func TestMemBlobLatestWithEmptyStore(t *testing.T) {
	store := New()
	ctx := context.Background()
	
	latest, err := store.Latest(ctx)
	if err != nil {
		t.Fatalf("latest failed: %v", err)
	}
	if latest != 0 {
		t.Fatalf("expected latest version 0 for empty store, got %d", latest)
	}
}

func TestMemBlobOutOfOrderCommit(t *testing.T) {
	store := New()
	ctx := context.Background()
	
	// Stage versions 1 and 3
	if err := store.Stage(ctx, 1, []byte("v1")); err != nil {
		t.Fatalf("stage v1 failed: %v", err)
	}
	if err := store.Stage(ctx, 3, []byte("v3")); err != nil {
		t.Fatalf("stage v3 failed: %v", err)
	}
	
	// Commit v3 first
	if err := store.Commit(ctx, 3); err != nil {
		t.Fatalf("commit v3 failed: %v", err)
	}
	
	// Then commit v1
	if err := store.Commit(ctx, 1); err != nil {
		t.Fatalf("commit v1 failed: %v", err)
	}
	
	// Latest should still be 3
	latest, err := store.Latest(ctx)
	if err != nil {
		t.Fatalf("latest failed: %v", err)
	}
	if latest != 3 {
		t.Fatalf("expected latest version 3, got %d", latest)
	}
}
