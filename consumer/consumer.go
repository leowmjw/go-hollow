package consumer

import (
	"context"
	"fmt"

	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/internal"
)

// Consumer handles data consumption and reading
type Consumer struct {
	readEngine     *internal.ReadStateEngine
	retriever      blob.BlobRetriever
	cursor         blob.VersionCursor
	typeFilter     *internal.TypeFilter
	memoryMode     internal.MemoryMode
	autoRefresh    bool
	watcherChannel chan int64
	serializer     internal.Serializer
}

// ConsumerOption configures a Consumer
type ConsumerOption func(*Consumer)

func WithBlobRetriever(retriever blob.BlobRetriever) ConsumerOption {
	return func(c *Consumer) {
		c.retriever = retriever
	}
}

func WithVersionCursor(cursor blob.VersionCursor) ConsumerOption {
	return func(c *Consumer) {
		c.cursor = cursor
		c.autoRefresh = true
	}
}

func WithAnnouncer(announcer blob.Announcer) ConsumerOption {
	return func(c *Consumer) {
		// Type-assert that announcer also implements VersionCursor
		if cursor, ok := announcer.(blob.VersionCursor); ok {
			c.cursor = cursor
			c.autoRefresh = true
		}
	}
}

// Deprecated: Use WithVersionCursor or WithAnnouncer instead
func WithAnnouncementWatcher(watcher interface{}) ConsumerOption {
	return func(c *Consumer) {
		// Try to convert to VersionCursor for backward compatibility
		if cursor, ok := watcher.(blob.VersionCursor); ok {
			c.cursor = cursor
			c.autoRefresh = true
		}
	}
}

func WithSerializer(serializer internal.Serializer) ConsumerOption {
	return func(c *Consumer) {
		c.serializer = serializer
	}
}

func WithTypeFilter(filter *internal.TypeFilter) ConsumerOption {
	return func(c *Consumer) {
		c.typeFilter = filter
	}
}

func WithMemoryMode(mode internal.MemoryMode) ConsumerOption {
	return func(c *Consumer) {
		c.memoryMode = mode
	}
}

// NewConsumer creates a new Consumer
func NewConsumer(opts ...ConsumerOption) *Consumer {
	c := &Consumer{
		readEngine:     internal.NewReadStateEngine(),
		memoryMode:     internal.HeapMemory,
		watcherChannel: make(chan int64, 10),
		serializer:     internal.NewTraditionalSerializer(), // Default to traditional for compatibility
	}

	for _, opt := range opts {
		opt(c)
	}

	// Validate incompatible options
	if c.typeFilter != nil && c.memoryMode == internal.SharedMemoryLazy {
		panic("Type filtering is incompatible with shared memory mode")
	}

	// Start watching for announcements if cursor is provided
	if c.autoRefresh && c.cursor != nil {
		go c.autoRefreshLoop()
	}

	return c
}

func (c *Consumer) autoRefreshLoop() {
	// Push-based updates - no polling, no busy wait
	// Check if cursor also supports subscriptions 
	if subscribable, ok := c.cursor.(blob.Subscribable); ok {
		sub, err := subscribable.Subscribe(10) // Buffer size 10
		if err != nil {
			return
		}
		defer sub.Close()
		
		for version := range sub.Updates() {
			currentVersion := c.readEngine.GetCurrentVersion()
			if version > currentVersion {
				// Auto-refresh to new version (idempotent call)
				c.TriggerRefreshTo(context.Background(), version)
			}
		}
	}
}

// TriggerRefresh refreshes to the latest available version
func (c *Consumer) TriggerRefresh(ctx context.Context) error {
	if c.cursor != nil {
		latestVersion := c.cursor.Latest()
		// Fallback: If watcher hasn't announced yet, use retriever's latest snapshot
		if latestVersion == 0 && c.retriever != nil {
			versions := c.retriever.ListVersions()
			if len(versions) > 0 {
				latestVersion = versions[len(versions)-1]
			}
		}
		return c.TriggerRefreshTo(ctx, latestVersion)
	}

	// If no watcher, refresh to latest version from retriever
	versions := c.retriever.ListVersions()
	if len(versions) == 0 {
		return fmt.Errorf("no versions available")
	}

	latestVersion := versions[len(versions)-1]
	return c.TriggerRefreshTo(ctx, latestVersion)
}

// planBlobs creates an optimized plan for transitioning from current to target version
func (c *Consumer) planBlobs(from, to int64) ([]*blob.Blob, error) {
	var chain []*blob.Blob
	
	// Walk backwards from 'to' until we hit a snapshot or 'from'
	cur := to
	for cur > from {
		if snap := c.retriever.RetrieveSnapshotBlob(cur); snap != nil {
			// Found the anchor - add snapshot first
			chain = append([]*blob.Blob{snap}, chain...)
			break
		}
		if delta := c.retriever.RetrieveDeltaBlob(cur - 1); delta != nil {
			// Add delta to front of chain
			chain = append([]*blob.Blob{delta}, chain...)
			cur = delta.FromVersion
		} else {
			return nil, fmt.Errorf("gap at version %d", cur-1)
		}
	}
	
	return chain, nil
}

// TriggerRefreshTo refreshes to a specific version using optimized blob planning
func (c *Consumer) TriggerRefreshTo(ctx context.Context, targetVersion int64) error {
	currentVersion := c.readEngine.GetCurrentVersion()

	if targetVersion == currentVersion {
		return nil // Already at target version
	}

	// Use optimized planning for blob retrieval
	blobs, err := c.planBlobs(currentVersion, targetVersion)
	if err != nil {
		return err
	}

	// Apply blobs in sequence
	for _, b := range blobs {
		if b.Type == blob.SnapshotBlob {
			if err := c.loadSnapshot(b); err != nil {
				return err
			}
		} else if b.Type == blob.DeltaBlob {
			if err := c.applyDelta(b); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *Consumer) loadSnapshot(blob *blob.Blob) error {
	// Use the real serializer to deserialize the blob data
	ctx := context.Background()
	deserializedData, err := c.serializer.Deserialize(ctx, blob.Data)
	if err != nil {
		return fmt.Errorf("failed to deserialize snapshot blob: %w", err)
	}

	// Create read state from deserialized data
	readState := internal.NewReadState(blob.Version)
	
	// Get the traditional data format for compatibility with existing API
	data := deserializedData.GetData()
	
	// Apply type filtering if configured
	readStateData := readState.GetAllData()
	for typeName, records := range data {
		if c.typeFilter == nil || c.typeFilter.ShouldInclude(typeName) {
			// Add the actual deserialized data to the read state
			readStateData[typeName] = records
		}
	}

	c.readEngine.SetCurrentState(readState)
	return nil
}



func (c *Consumer) applyDelta(deltaBlob *blob.Blob) error {
	// Get current state to merge delta into
	currentState := c.readEngine.GetCurrentState()
	
	// Deserialize the delta using the real serializer
	ctx := context.Background()
	deltaSet, err := c.serializer.DeserializeDelta(ctx, deltaBlob.Data)
	if err != nil {
		return fmt.Errorf("failed to deserialize delta blob: %w", err)
	}
	
	// Create new state with delta version
	readState := internal.NewReadState(deltaBlob.ToVersion)
	newData := readState.GetAllData()
	
	// If we have a current state, copy its data first (state accumulation)
	if currentState != nil && !currentState.IsInvalidated() {
		currentData := currentState.GetAllData()
		for typeName, records := range currentData {
			// Copy existing records to maintain state accumulation
			newData[typeName] = make([]interface{}, len(records))
			copy(newData[typeName], records)
		}
	}
	
	// Apply the real delta changes
	for typeName, typeDelta := range deltaSet.Deltas {
		// Skip types that are filtered out
		if c.typeFilter != nil && !c.typeFilter.ShouldInclude(typeName) {
			continue
		}
		
		// Ensure we have a slice for this type
		if newData[typeName] == nil {
			newData[typeName] = make([]interface{}, 0)
		}
		
		// Apply delta records for this type
		for _, deltaRecord := range typeDelta.Records {
			switch deltaRecord.Operation {
			case internal.DeltaAdd:
				// Add new record
				newData[typeName] = append(newData[typeName], deltaRecord.Value)
			case internal.DeltaUpdate:
				// Update existing record (simplified - would need ordinal tracking)
				if deltaRecord.Ordinal < len(newData[typeName]) {
					newData[typeName][deltaRecord.Ordinal] = deltaRecord.Value
				}
			case internal.DeltaDelete:
				// Delete record (simplified - would need ordinal tracking)
				if deltaRecord.Ordinal < len(newData[typeName]) {
					// Remove by replacing with nil or shrinking slice
					newData[typeName] = append(newData[typeName][:deltaRecord.Ordinal], 
						newData[typeName][deltaRecord.Ordinal+1:]...)
				}
			}
		}
	}
	
	c.readEngine.SetCurrentState(readState)
	return nil
}



// GetCurrentVersion returns the current version
func (c *Consumer) GetCurrentVersion() int64 {
	return c.readEngine.GetCurrentVersion()
}

// GetStateEngine returns the read state engine
func (c *Consumer) GetStateEngine() *internal.ReadStateEngine {
	return c.readEngine
}

// GetRetriever returns the blob retriever
func (c *Consumer) GetRetriever() blob.BlobRetriever {
	return c.retriever
}
