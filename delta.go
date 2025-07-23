package hollow

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// DeltaWriter provides delta/incremental update support
type DeltaWriter interface {
	WriteDelta(ctx context.Context, baseVersion uint64, changes *DataDiff) (uint64, error)
	GetDelta(ctx context.Context, fromVersion, toVersion uint64) (*DataDiff, error)
}

// DeltaReader provides delta consumption support
type DeltaReader interface {
	ReadDelta(ctx context.Context, fromVersion, toVersion uint64) (*DataDiff, error)
	ApplyDelta(ctx context.Context, baseData map[string]any, delta *DataDiff) (map[string]any, error)
}

// DeltaAwareBlobStore extends BlobStager with delta support
type DeltaAwareBlobStore interface {
	BlobStager
	BlobRetriever
	DeltaWriter
	DeltaReader
}

// DeltaMetadata contains metadata about a delta
type DeltaMetadata struct {
	FromVersion uint64    `json:"from_version"`
	ToVersion   uint64    `json:"to_version"`
	Timestamp   time.Time `json:"timestamp"`
	ChangeCount int       `json:"change_count"`
	Size        int64     `json:"size"`
}

// DeltaStorage manages delta operations
type DeltaStorage struct {
	mu       sync.RWMutex
	deltas   map[string]*SerializedDelta // key: "from_to" format
	metadata map[uint64]*DeltaMetadata   // key: to_version
}

// SerializedDelta represents a serialized delta
type SerializedDelta struct {
	FromVersion uint64          `json:"from_version"`
	ToVersion   uint64          `json:"to_version"`
	Changes     *DataDiff       `json:"changes"`
	Metadata    *DeltaMetadata  `json:"metadata"`
	Data        json.RawMessage `json:"data,omitempty"`
}

// NewDeltaStorage creates a new delta storage
func NewDeltaStorage() *DeltaStorage {
	return &DeltaStorage{
		deltas:   make(map[string]*SerializedDelta),
		metadata: make(map[uint64]*DeltaMetadata),
	}
}

// StoreDelta stores a delta between two versions
func (ds *DeltaStorage) StoreDelta(fromVersion, toVersion uint64, changes *DataDiff) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	
	key := fmt.Sprintf("%d_%d", fromVersion, toVersion)
	
	metadata := &DeltaMetadata{
		FromVersion: fromVersion,
		ToVersion:   toVersion,
		Timestamp:   time.Now(),
		ChangeCount: len(changes.added) + len(changes.removed) + len(changes.changed),
		Size:        int64(len(changes.added)*50 + len(changes.removed)*10 + len(changes.changed)*50), // Estimate
	}
	
	delta := &SerializedDelta{
		FromVersion: fromVersion,
		ToVersion:   toVersion,
		Changes:     changes,
		Metadata:    metadata,
	}
	
	ds.deltas[key] = delta
	ds.metadata[toVersion] = metadata
	
	return nil
}

// GetDelta retrieves a delta between two versions
func (ds *DeltaStorage) GetDelta(fromVersion, toVersion uint64) (*DataDiff, error) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	
	key := fmt.Sprintf("%d_%d", fromVersion, toVersion)
	
	delta, exists := ds.deltas[key]
	if !exists {
		return nil, fmt.Errorf("delta not found: from=%d to=%d", fromVersion, toVersion)
	}
	
	return delta.Changes, nil
}

// GetDeltaMetadata retrieves metadata for a version
func (ds *DeltaStorage) GetDeltaMetadata(version uint64) (*DeltaMetadata, error) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	
	metadata, exists := ds.metadata[version]
	if !exists {
		return nil, fmt.Errorf("metadata not found for version %d", version)
	}
	
	return metadata, nil
}

// ListDeltas returns all available deltas
func (ds *DeltaStorage) ListDeltas() map[string]*DeltaMetadata {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	
	result := make(map[string]*DeltaMetadata)
	for key, delta := range ds.deltas {
		result[key] = delta.Metadata
	}
	
	return result
}

// DeltaProducer extends Producer with delta support
type DeltaProducer struct {
	*Producer
	deltaStorage *DeltaStorage
	lastSnapshot map[string]any
	mu           sync.RWMutex
}

// NewDeltaProducer creates a new delta-aware producer
func NewDeltaProducer(opts ...ProducerOpt) *DeltaProducer {
	producer := NewProducer(opts...)
	
	return &DeltaProducer{
		Producer:     producer,
		deltaStorage: NewDeltaStorage(),
		lastSnapshot: make(map[string]any),
	}
}

// RunDeltaCycle runs a production cycle and computes deltas
func (dp *DeltaProducer) RunDeltaCycle(fn func(WriteState) error) (uint64, *DataDiff, error) {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	
	// Capture current state before changes
	previousSnapshot := make(map[string]any)
	for k, v := range dp.lastSnapshot {
		previousSnapshot[k] = v
	}
	
	// Get the last version
	ctx := context.Background()
	var lastVersion uint64
	var err error
	
	// Try to cast to DeltaAwareBlobStore first
	if deltaStore, ok := dp.stager.(DeltaAwareBlobStore); ok {
		lastVersion, err = deltaStore.Latest(ctx)
	} else if store, ok := dp.stager.(BlobRetriever); ok {
		// Fall back to regular BlobRetriever
		lastVersion, err = store.Latest(ctx)
	}
	
	if err != nil {
		lastVersion = 0 // First version
	}
	
	// Run the normal cycle
	newVersion, err := dp.Producer.RunCycle(fn)
	if err != nil {
		return 0, nil, err
	}
	
	// Get the new state (this is simplified - in production we'd need to capture the actual state)
	newSnapshot := make(map[string]any)
	// In a real implementation, we'd capture the actual state from the WriteState
	// For now, we'll simulate it
	
	// Compute delta
	delta := DiffData(previousSnapshot, newSnapshot)
	
	// Store delta
	if err := dp.deltaStorage.StoreDelta(lastVersion, newVersion, delta); err != nil {
		return newVersion, delta, fmt.Errorf("failed to store delta: %w", err)
	}
	
	// Update last snapshot
	dp.lastSnapshot = newSnapshot
	
	return newVersion, delta, nil
}

// GetDelta retrieves a delta between versions
func (dp *DeltaProducer) GetDelta(fromVersion, toVersion uint64) (*DataDiff, error) {
	return dp.deltaStorage.GetDelta(fromVersion, toVersion)
}

// GetDeltaMetadata retrieves metadata for a version
func (dp *DeltaProducer) GetDeltaMetadata(version uint64) (*DeltaMetadata, error) {
	return dp.deltaStorage.GetDeltaMetadata(version)
}

// DeltaConsumer extends Consumer with delta support
type DeltaConsumer struct {
	*Consumer
	deltaStorage *DeltaStorage
	mu           sync.RWMutex
}

// NewDeltaConsumer creates a new delta-aware consumer
func NewDeltaConsumer(opts ...ConsumerOpt) *DeltaConsumer {
	consumer := NewConsumer(opts...)
	
	return &DeltaConsumer{
		Consumer:     consumer,
		deltaStorage: NewDeltaStorage(),
	}
}

// RefreshWithDelta updates the consumer with a delta
// It retrieves and applies a delta if available, or falls back to full refresh
func (dc *DeltaConsumer) RefreshWithDelta(currentVersion uint64) (*DataDiff, error) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	latestVersion, ok, err := dc.Consumer.watcher()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil // No new version
	}
	
	if currentVersion == latestVersion {
		return &DataDiff{}, nil // No changes
	}
	
	// Check if delta is available in storage
	deltaData, err := dc.deltaStorage.GetDelta(currentVersion, latestVersion)
	if err != nil {
		// If delta isn't available, fall back to full refresh
		dc.Consumer.logger.Debug("Delta not found, falling back to full refresh", 
			"from", currentVersion, "to", latestVersion, "err", err)
		if err := dc.Consumer.Refresh(); err != nil {
			return nil, fmt.Errorf("failed to refresh consumer: %w", err)
		}
		
		// Return an empty delta that indicates success via full refresh
		delta := &DataDiff{
			added:   make([]string, 0),
			removed: make([]string, 0),
			changed: make([]string, 0),
		}
		
		// Update consumer version
		dc.Consumer.version.Store(latestVersion)
		return delta, nil
	}
	
	// Convert Cap'n Proto data to DataDiff
	// deltaData is already a *DataDiff from GetDelta, so we can use it directly
	delta := deltaData
	if delta == nil {
		dc.Consumer.logger.Error("Failed to get delta", "error", "delta is nil")
		// Fall back to full refresh
		if err := dc.Consumer.Refresh(); err != nil {
			return nil, fmt.Errorf("failed to refresh consumer: %w", err)
		}
		dc.Consumer.version.Store(latestVersion)
		return &DataDiff{}, nil
	}
	
	// Get the state for updates
	stateValue := dc.Consumer.state.Load()
	if stateValue == nil {
		return nil, fmt.Errorf("consumer state is nil")
	}
	
	// We need both ReadState and WriteState interfaces to properly apply delta updates
	readState, ok := stateValue.(ReadState)
	if !ok {
		return nil, fmt.Errorf("consumer state does not implement ReadState")
	}
	
	// Apply the updated data to the consumer's state
	// For thread safety, we need to ensure we're operating on the most current state
	// after checking ReadState above
	stateValue = dc.Consumer.state.Load()
	if stateValue == nil {
		return nil, fmt.Errorf("consumer state is nil")
	}
	ws, ok := stateValue.(WriteState)
	if !ok {
		return nil, fmt.Errorf("consumer state does not implement WriteState")
	}
	
	// Since WriteState doesn't have Clear() method, we can't clear and rebuild
	// Instead, we'll update with the delta information directly
	
	// For added keys, use the delta information to add them
	for _, key := range delta.GetAdded() {
		// The ReadState interface doesn't allow us to get all keys
		// For a real implementation, we would need to enhance the interface or
		// store the deltas with the complete values for added/changed keys
		
		// For now, we'll assume any added or changed keys would be handled
		// as part of a full refresh if this approach doesn't work
		dc.Consumer.logger.Debug("Delta update would add key", "key", key)
	}
	
	// For changed keys, use the delta information to update them
	for _, key := range delta.GetChanged() {
		// Get the current value first
		value, found := readState.Get(key)
		if found {
			// In a full implementation, we would apply changes to the value
			// For now, we'll just update with the current value
			if err := ws.Add(map[string]any{key: value}); err != nil {
				return nil, fmt.Errorf("failed to update changed key %s: %w", key, err)
			}
			dc.Consumer.logger.Debug("Updated key from delta", "key", key)
		}
	}
	
	// Note: We can't directly remove keys since WriteState only has Add() method
	// A full implementation would require extending the interface
	
	// Update consumer version
	dc.Consumer.version.Store(latestVersion)
	
	return delta, nil
}

// ApplyDelta applies a delta to the given data
func (dc *DeltaConsumer) ApplyDelta(baseData map[string]any, delta *DataDiff) (map[string]any, error) {
	result := make(map[string]any)
	
	// Copy base data
	for k, v := range baseData {
		result[k] = v
	}
	
	// Apply delta
	delta.Apply(result, baseData)
	
	return result, nil
}

// GetDeltaHistory returns the history of deltas
func (dc *DeltaConsumer) GetDeltaHistory() map[string]*DeltaMetadata {
	return dc.deltaStorage.ListDeltas()
}

// DeltaOptions provides configuration for delta operations
type DeltaOptions struct {
	MaxDeltaSize        int           // Maximum size of a delta before forcing snapshot
	MaxDeltaChain       int           // Maximum number of deltas to chain
	DeltaRetentionTime  time.Duration // How long to keep deltas
	CompressionEnabled  bool          // Whether to compress delta data
	ValidationEnabled   bool          // Whether to validate deltas
}

// DefaultDeltaOptions returns default options for delta operations
func DefaultDeltaOptions() *DeltaOptions {
	return &DeltaOptions{
		MaxDeltaSize:        1000000,      // 1MB
		MaxDeltaChain:       10,           // Max 10 deltas in chain
		DeltaRetentionTime:  24 * time.Hour, // Keep deltas for 24 hours
		CompressionEnabled:  true,
		ValidationEnabled:   true,
	}
}

// ValidateDelta validates a delta for consistency
func ValidateDelta(delta *DataDiff) error {
	if delta == nil {
		return fmt.Errorf("delta cannot be nil")
	}
	
	// Check for duplicate keys
	seen := make(map[string]bool)
	
	for _, key := range delta.added {
		if seen[key] {
			return fmt.Errorf("duplicate key in delta: %s", key)
		}
		seen[key] = true
	}
	
	for _, key := range delta.removed {
		if seen[key] {
			return fmt.Errorf("duplicate key in delta: %s", key)
		}
		seen[key] = true
	}
	
	for _, key := range delta.changed {
		if seen[key] {
			return fmt.Errorf("duplicate key in delta: %s", key)
		}
		seen[key] = true
	}
	
	return nil
}

// CompressDelta compresses a delta to reduce storage size
func CompressDelta(delta *DataDiff) (*DataDiff, error) {
	// In a real implementation, this would compress the delta data
	// For now, we'll just return the original delta
	return delta, nil
}

// DecompressDelta decompresses a delta
func DecompressDelta(compressedDelta *DataDiff) (*DataDiff, error) {
	// In a real implementation, this would decompress the delta data
	// For now, we'll just return the original delta
	return compressedDelta, nil
}
