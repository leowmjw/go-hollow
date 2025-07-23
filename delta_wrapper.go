package hollow

import (
	"context"
	
	"github.com/leowmjw/go-hollow/internal/memblob"
)

// DeltaAwareMemBlobStore wraps a memblob.Store to make it satisfy the DeltaAwareBlobStore interface
type DeltaAwareMemBlobStore struct {
	*memblob.Store
	deltaStorage *DeltaStorage
}

// NewDeltaAwareMemBlobStore creates a new DeltaAwareMemBlobStore
func NewDeltaAwareMemBlobStore() *DeltaAwareMemBlobStore {
	return &DeltaAwareMemBlobStore{
		Store:        memblob.New(),
		deltaStorage: NewDeltaStorage(),
	}
}

// WriteDelta writes a delta between versions
func (d *DeltaAwareMemBlobStore) WriteDelta(ctx context.Context, baseVersion uint64, changes *DataDiff) (uint64, error) {
	// In a real implementation, this would serialize and store the delta
	// For now, we'll just return the next version number
	latestVersion, err := d.Latest(ctx)
	if err != nil {
		return 0, err
	}
	
	newVersion := latestVersion + 1
	
	// Store the delta in the internal storage
	if err := d.deltaStorage.StoreDelta(baseVersion, newVersion, changes); err != nil {
		return 0, err
	}
	
	return newVersion, nil
}

// ReadDelta reads a delta between versions
func (d *DeltaAwareMemBlobStore) ReadDelta(ctx context.Context, fromVersion, toVersion uint64) (*DataDiff, error) {
	return d.deltaStorage.GetDelta(fromVersion, toVersion)
}

// ApplyDelta applies a delta to base data
func (d *DeltaAwareMemBlobStore) ApplyDelta(ctx context.Context, baseData map[string]any, delta *DataDiff) (map[string]any, error) {
	// Create a copy of the base data
	result := make(map[string]any, len(baseData))
	for k, v := range baseData {
		result[k] = v
	}
	
	// Remove items
	for _, key := range delta.GetRemoved() {
		delete(result, key)
	}
	
	// Handle changes and additions - we need access to the actual values
	// In a real implementation, we would use the values from the delta
	// Here we're simplifying since DataDiff only has keys, not values
	
	return result, nil
}

// GetDelta is an alias for ReadDelta
func (d *DeltaAwareMemBlobStore) GetDelta(ctx context.Context, fromVersion, toVersion uint64) (*DataDiff, error) {
	return d.ReadDelta(ctx, fromVersion, toVersion)
}
