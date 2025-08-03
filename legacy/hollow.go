// Package hollow provides ultra-fast, read-only in-memory datasets
// that are synchronised by snapshots and deltas.
package hollow

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
)

// Producer manages the creation and staging of data snapshots.
type Producer struct {
	mu      sync.RWMutex
	stager  BlobStager
	logger  *slog.Logger
	metrics MetricsCollector
	version atomic.Uint64
}

// Consumer manages the consumption and refresh of data snapshots.
type Consumer struct {
	retriever BlobRetriever
	watcher   AnnouncementWatcher
	logger    *slog.Logger
	state     atomic.Value
	version   atomic.Uint64
}

// WriteState represents the state during a write operation.
type WriteState interface {
	Add(v any) error
}

// ReadState represents the state during a read operation.
type ReadState interface {
	Get(key any) (any, bool)
	Size() int
}

// BlobStager handles staging of blob data.
type BlobStager interface {
	Stage(ctx context.Context, version uint64, data []byte) error
	Commit(ctx context.Context, version uint64) error
}

// BlobRetriever handles retrieval of blob data.
type BlobRetriever interface {
	Retrieve(ctx context.Context, version uint64) ([]byte, error)
	Latest(ctx context.Context) (uint64, error)
}

// AnnouncementWatcher watches for new version announcements.
type AnnouncementWatcher func() (version uint64, ok bool, err error)

// MetricsCollector collects metrics during operations.
type MetricsCollector func(m Metrics)

// Metrics represents collected metrics.
type Metrics struct {
	Version     uint64
	RecordCount int
	ByteSize    int64
}

// DataDiff represents differences between two datasets.
type DataDiff struct {
	added   []string
	removed []string
	changed []string
}

// IsEmpty returns true if the diff has no changes.
func (d *DataDiff) IsEmpty() bool {
	return len(d.added) == 0 && len(d.removed) == 0 && len(d.changed) == 0
}

// GetAdded returns the added keys.
func (d *DataDiff) GetAdded() []string {
	return d.added
}

// GetRemoved returns the removed keys.
func (d *DataDiff) GetRemoved() []string {
	return d.removed
}

// GetChanged returns the changed keys.
func (d *DataDiff) GetChanged() []string {
	return d.changed
}
