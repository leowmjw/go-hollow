package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/consumer"
	"github.com/leowmjw/go-hollow/internal"
	"github.com/leowmjw/go-hollow/producer"
)

// User represents a user record with primary key
type User struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Status   string `json:"status"`
}

func TestPrimaryKeyFullIntegration(t *testing.T) {
	// Create infrastructure
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewInMemoryAnnouncement()

	// Create producer with primary key configuration
	prod := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
		producer.WithPrimaryKey("*main.User", "UserID"),       // Primary key is UserID field
		producer.WithSerializationMode(internal.ZeroCopyMode), // Use zero-copy Cap'n Proto
	)

	// Create consumer
	cons := consumer.NewConsumer(
		consumer.WithBlobRetriever(blobStore),
		consumer.WithAnnouncementWatcher(announcer),
		consumer.WithSerializer(internal.NewCapnProtoSerializer()), // Match producer's serialization mode
	)

	ctx := context.Background()

	// Test 1: Create initial users
	t.Log("=== Test 1: Creating initial users ===")
	version1, _ := prod.RunCycle(ctx, func(ws *internal.WriteState) {
		ws.Add(&User{UserID: 1001, Username: "alice", Email: "alice@example.com", Status: "active"})
		ws.Add(&User{UserID: 1002, Username: "bob", Email: "bob@example.com", Status: "active"})
		ws.Add(&User{UserID: 1003, Username: "charlie", Email: "charlie@example.com", Status: "inactive"})
	})

	if version1 == 0 {
		t.Fatal("Expected version1 > 0")
	}
	t.Logf("✓ Created version %d", version1)

	// Test 2: Update existing users and add new ones (primary key deduplication)
	t.Log("=== Test 2: Updating users with primary key deduplication ===")
	version2, _ := prod.RunCycle(ctx, func(ws *internal.WriteState) {
		// Update user 1001 (email change) - should be treated as update due to primary key
		ws.Add(&User{UserID: 1001, Username: "alice", Email: "alice.new@example.com", Status: "active"})
		// Update user 1002 (status change) - should be treated as update
		ws.Add(&User{UserID: 1002, Username: "bob", Email: "bob@example.com", Status: "suspended"})
		// Add new user
		ws.Add(&User{UserID: 1004, Username: "diana", Email: "diana@example.com", Status: "active"})
	})

	if version2 <= version1 {
		t.Fatalf("Expected version2 (%d) > version1 (%d)", version2, version1)
	}
	t.Logf("✓ Created version %d", version2)

	// Test 3: Consumer follows delta chain
	t.Log("=== Test 3: Consumer delta traversal ===")
	err := cons.TriggerRefresh(ctx)
	if err != nil {
		t.Fatalf("Consumer refresh failed: %v", err)
	}

	currentVersion := cons.GetCurrentVersion()
	if currentVersion != version2 {
		t.Errorf("Expected consumer at version %d, got %d", version2, currentVersion)
	}
	t.Logf("✓ Consumer successfully refreshed to version %d", currentVersion)

	// Test 4: Verify delta blob structure
	t.Log("=== Test 4: Verifying blob structure ===")

	// Check snapshot blob exists
	snapshotBlob := blobStore.RetrieveSnapshotBlob(version1)
	if snapshotBlob == nil {
		t.Fatal("Expected snapshot blob for version 1")
	}
	if snapshotBlob.Type != blob.SnapshotBlob {
		t.Errorf("Expected SnapshotBlob, got %v", snapshotBlob.Type)
	}
	t.Logf("✓ Snapshot blob verified for version %d", version1)

	// Check delta blob exists
	deltaBlob := blobStore.RetrieveDeltaBlob(version1)
	if deltaBlob == nil {
		t.Fatal("Expected delta blob from version 1")
	}
	if deltaBlob.Type != blob.DeltaBlob {
		t.Errorf("Expected DeltaBlob, got %v", deltaBlob.Type)
	}
	if deltaBlob.FromVersion != version1 || deltaBlob.ToVersion != version2 {
		t.Errorf("Expected delta from %d to %d, got %d to %d", version1, version2, deltaBlob.FromVersion, deltaBlob.ToVersion)
	}
	t.Logf("✓ Delta blob verified from version %d to %d", deltaBlob.FromVersion, deltaBlob.ToVersion)

	// Test 5: Identical data hash optimization
	t.Log("=== Test 5: Testing data hash optimization ===")
	version3, _ := prod.RunCycle(ctx, func(ws *internal.WriteState) {
		// Identical data to version 2
		ws.Add(&User{UserID: 1001, Username: "alice", Email: "alice.new@example.com", Status: "active"})
		ws.Add(&User{UserID: 1002, Username: "bob", Email: "bob@example.com", Status: "suspended"})
		ws.Add(&User{UserID: 1004, Username: "diana", Email: "diana@example.com", Status: "active"})
	})

	if version3 != version2 {
		t.Errorf("Expected identical data to reuse version %d, got %d", version2, version3)
	}
	t.Logf("✓ Data hash optimization working: version %d reused", version3)

	// Test 6: Remove users (by omission) and verify new version
	t.Log("=== Test 6: Testing user removal by omission ===")
	version4, _ := prod.RunCycle(ctx, func(ws *internal.WriteState) {
		// Only include users 1001 and 1004 (remove 1002 by omission, add new user)
		ws.Add(&User{UserID: 1001, Username: "alice", Email: "alice.new@example.com", Status: "active"})
		ws.Add(&User{UserID: 1004, Username: "diana", Email: "diana@example.com", Status: "active"})
		ws.Add(&User{UserID: 1005, Username: "eve", Email: "eve@example.com", Status: "active"})
	})

	if version4 <= version3 {
		t.Fatalf("Expected version4 (%d) > version3 (%d)", version4, version3)
	}
	t.Logf("✓ Created version %d with user removal and addition", version4)

	// Test 7: Verify zero-copy serialization mode
	t.Log("=== Test 7: Verifying zero-copy serialization ===")
	latestBlob := blobStore.RetrieveDeltaBlob(version3) // Delta from version3 to version4
	if latestBlob == nil {
		t.Fatal("Expected delta blob for latest version")
	}

	// Check metadata for serialization mode
	if mode, exists := latestBlob.Metadata["serialization_mode"]; exists {
		expectedMode := fmt.Sprintf("%d", internal.ZeroCopyMode)
		if mode != expectedMode {
			t.Errorf("Expected zero-copy mode %s, got %s", expectedMode, mode)
		}
		t.Logf("✓ Zero-copy serialization confirmed: mode %s", mode)
	} else {
		t.Error("Serialization mode metadata missing")
	}

	t.Log("✅ Primary key full integration test completed successfully")
}

func TestPrimaryKeyWithoutConfiguration(t *testing.T) {
	// Test that producers work normally when no primary key is configured

	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewInMemoryAnnouncement()

	// Create producer WITHOUT primary key configuration
	prod := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
		// No WithPrimaryKey calls - should use traditional behavior
	)

	ctx := context.Background()

	version1, _ := prod.RunCycle(ctx, func(ws *internal.WriteState) {
		ws.Add(&User{UserID: 2001, Username: "test1", Email: "test1@example.com", Status: "active"})
		ws.Add(&User{UserID: 2002, Username: "test2", Email: "test2@example.com", Status: "active"})
	})

	if version1 == 0 {
		t.Fatal("Expected version1 > 0")
	}

	// Add different data - should create new version
	version2, _ := prod.RunCycle(ctx, func(ws *internal.WriteState) {
		ws.Add(&User{UserID: 2001, Username: "test1", Email: "test1@example.com", Status: "active"})
		ws.Add(&User{UserID: 2002, Username: "test2", Email: "test2@example.com", Status: "active"})
		ws.Add(&User{UserID: 2003, Username: "test3", Email: "test3@example.com", Status: "active"})
	})

	if version2 <= version1 {
		t.Fatalf("Expected version2 (%d) > version1 (%d)", version2, version1)
	}

	// Add identical data - should reuse version (traditional behavior)
	version3, _ := prod.RunCycle(ctx, func(ws *internal.WriteState) {
		ws.Add(&User{UserID: 2001, Username: "test1", Email: "test1@example.com", Status: "active"})
		ws.Add(&User{UserID: 2002, Username: "test2", Email: "test2@example.com", Status: "active"})
		ws.Add(&User{UserID: 2003, Username: "test3", Email: "test3@example.com", Status: "active"})
	})

	if version3 != version2 {
		t.Errorf("Expected identical data to reuse version %d, got %d", version2, version3)
	}

	t.Log("✅ Traditional producer behavior (without primary keys) working correctly")
}
