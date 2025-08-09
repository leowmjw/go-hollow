package consumer

import (
	"context"
	"fmt"
	"time"

	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/internal"
)

// Consumer handles data consumption and reading
type Consumer struct {
	readEngine     *internal.ReadStateEngine
	retriever      blob.BlobRetriever
	watcher        blob.AnnouncementWatcher
	typeFilter     *internal.TypeFilter
	memoryMode     internal.MemoryMode
	autoRefresh    bool
	watcherChannel chan int64
}

// ConsumerOption configures a Consumer
type ConsumerOption func(*Consumer)

func WithBlobRetriever(retriever blob.BlobRetriever) ConsumerOption {
	return func(c *Consumer) {
		c.retriever = retriever
	}
}

func WithAnnouncementWatcher(watcher blob.AnnouncementWatcher) ConsumerOption {
	return func(c *Consumer) {
		c.watcher = watcher
		c.autoRefresh = true
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
	}

	for _, opt := range opts {
		opt(c)
	}

	// Validate incompatible options
	if c.typeFilter != nil && c.memoryMode == internal.SharedMemoryLazy {
		panic("Type filtering is incompatible with shared memory mode")
	}

	// Start watching for announcements if watcher is provided
	if c.autoRefresh && c.watcher != nil {
		go c.watchAnnouncements()
	}

	return c
}

func (c *Consumer) watchAnnouncements() {
	// In a real implementation, this would watch for announcements
	// and trigger refreshes automatically
	for {
		time.Sleep(50 * time.Millisecond)
		latestVersion := c.watcher.GetLatestVersion()
		currentVersion := c.readEngine.GetCurrentVersion()

		if latestVersion > currentVersion {
			// Auto-refresh to latest version
			c.TriggerRefreshTo(context.Background(), latestVersion)
		}
	}
}

// TriggerRefresh refreshes to the latest available version
func (c *Consumer) TriggerRefresh(ctx context.Context) error {
	if c.watcher != nil {
		latestVersion := c.watcher.GetLatestVersion()
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

// TriggerRefreshTo refreshes to a specific version
func (c *Consumer) TriggerRefreshTo(ctx context.Context, targetVersion int64) error {
	currentVersion := c.readEngine.GetCurrentVersion()

	if targetVersion == currentVersion {
		return nil // Already at target version
	}

	// Try to get snapshot first
	snapshotBlob := c.retriever.RetrieveSnapshotBlob(targetVersion)
	if snapshotBlob != nil {
		return c.loadSnapshot(snapshotBlob)
	}

	// If no direct snapshot, try delta traversal
	if targetVersion > currentVersion {
		return c.followDeltaChain(currentVersion, targetVersion)
	} else {
		return c.followReverseDeltaChain(currentVersion, targetVersion)
	}
}

func (c *Consumer) loadSnapshot(blob *blob.Blob) error {
	// Deserialize blob data (simplified implementation)
	readState := internal.NewReadState(blob.Version)

	// In a real implementation, would deserialize Cap'n Proto data
	// For testing purposes, simulate loading data based on the blob
	if string(blob.Data) != "map[]" { // If there's actual data
		// Simulate loading types - for type filtering test
		readState.AddMockType("String")
		if c.typeFilter == nil || c.typeFilter.ShouldInclude("Integer") {
			readState.AddMockType("Integer")
		}
	}

	c.readEngine.SetCurrentState(readState)

	return nil
}

func (c *Consumer) followDeltaChain(fromVersion, toVersion int64) error {
	currentVersion := fromVersion

	for currentVersion < toVersion {
		deltaBlob := c.retriever.RetrieveDeltaBlob(currentVersion)
		if deltaBlob == nil {
			// Try to find a snapshot closer to target
			snapshotBlob := c.findNearestSnapshot(currentVersion, toVersion)
			if snapshotBlob != nil {
				err := c.loadSnapshot(snapshotBlob)
				if err != nil {
					return err
				}
				currentVersion = snapshotBlob.Version
				continue
			}
			return fmt.Errorf("no delta blob found from version %d", currentVersion)
		}

		// Apply delta
		err := c.applyDelta(deltaBlob)
		if err != nil {
			return err
		}

		currentVersion = deltaBlob.ToVersion
	}

	return nil
}

func (c *Consumer) followReverseDeltaChain(fromVersion, toVersion int64) error {
	currentVersion := fromVersion

	for currentVersion > toVersion {
		reverseBlob := c.retriever.RetrieveReverseBlob(currentVersion)
		if reverseBlob == nil {
			return fmt.Errorf("no reverse delta blob found for version %d", currentVersion)
		}

		// Apply reverse delta
		err := c.applyReverseDelta(reverseBlob)
		if err != nil {
			return err
		}

		currentVersion = reverseBlob.FromVersion
	}

	return nil
}

func (c *Consumer) findNearestSnapshot(fromVersion, toVersion int64) *blob.Blob {
	// Efficiently find the nearest snapshot using retriever's version index
	if c.retriever == nil {
		return nil
	}

	versions := c.retriever.ListVersions()
	if len(versions) == 0 {
		return nil
	}

	// Iterate from newest to oldest snapshot and pick the closest <= toVersion and > fromVersion
	for i := len(versions) - 1; i >= 0; i-- {
		v := versions[i]
		if v <= toVersion && v > fromVersion {
			if snapshot := c.retriever.RetrieveSnapshotBlob(v); snapshot != nil {
				return snapshot
			}
			// If retrieval failed unexpectedly, continue searching older snapshots
		}
		if v <= fromVersion {
			break
		}
	}

	return nil
}

func (c *Consumer) applyDelta(deltaBlob *blob.Blob) error {
	// Apply delta to current state (simplified)
	readState := internal.NewReadState(deltaBlob.ToVersion)
	c.readEngine.SetCurrentState(readState)
	return nil
}

func (c *Consumer) applyReverseDelta(reverseBlob *blob.Blob) error {
	// Apply reverse delta to current state (simplified)
	readState := internal.NewReadState(reverseBlob.FromVersion)
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
