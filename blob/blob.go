package blob

import (
	"context"
	"errors"
	"sync"
)

// BlobType represents the type of blob
type BlobType int

const (
	SnapshotBlob BlobType = iota
	DeltaBlob
	ReverseBlob
)

// Blob represents a serialized data blob
type Blob struct {
	Type        BlobType
	Version     int64
	FromVersion int64
	ToVersion   int64
	Data        []byte
	Checksum    uint64
	// Metadata for extensibility (e.g., serialization mode, compression, etc.)
	Metadata    map[string]string
}

// BlobStore interface for storing and retrieving blobs
type BlobStore interface {
	// Store a blob
	Store(ctx context.Context, blob *Blob) error
	
	// Retrieve a snapshot blob
	RetrieveSnapshotBlob(version int64) *Blob
	
	// Retrieve a delta blob
	RetrieveDeltaBlob(fromVersion int64) *Blob
	
	// Retrieve a reverse delta blob
	RetrieveReverseBlob(toVersion int64) *Blob
	
	// Remove a snapshot
	RemoveSnapshot(version int64) error
	
	// List available versions
	ListVersions() []int64
}

// BlobRetriever interface for consuming blobs
type BlobRetriever interface {
	// Retrieve a snapshot blob
	RetrieveSnapshotBlob(version int64) *Blob
	
	// Retrieve a delta blob
	RetrieveDeltaBlob(fromVersion int64) *Blob
	
	// Retrieve a reverse delta blob
	RetrieveReverseBlob(toVersion int64) *Blob
	
	// List available versions
	ListVersions() []int64
}

// InMemoryBlobStore is an in-memory implementation of BlobStore
type InMemoryBlobStore struct {
	mu       sync.RWMutex
	snapshots map[int64]*Blob
	deltas    map[int64]*Blob // keyed by FromVersion
	reverses  map[int64]*Blob // keyed by ToVersion
}

// NewInMemoryBlobStore creates a new in-memory blob store
func NewInMemoryBlobStore() *InMemoryBlobStore {
	return &InMemoryBlobStore{
		snapshots: make(map[int64]*Blob),
		deltas:    make(map[int64]*Blob),
		reverses:  make(map[int64]*Blob),
	}
}

func (s *InMemoryBlobStore) Store(ctx context.Context, blob *Blob) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	switch blob.Type {
	case SnapshotBlob:
		s.snapshots[blob.Version] = blob
	case DeltaBlob:
		s.deltas[blob.FromVersion] = blob
	case ReverseBlob:
		s.reverses[blob.ToVersion] = blob
	default:
		return errors.New("unknown blob type")
	}
	
	return nil
}

func (s *InMemoryBlobStore) RetrieveSnapshotBlob(version int64) *Blob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshots[version]
}

func (s *InMemoryBlobStore) RetrieveDeltaBlob(fromVersion int64) *Blob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.deltas[fromVersion]
}

func (s *InMemoryBlobStore) RetrieveReverseBlob(toVersion int64) *Blob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.reverses[toVersion]
}

func (s *InMemoryBlobStore) RemoveSnapshot(version int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.snapshots, version)
	return nil
}

func (s *InMemoryBlobStore) ListVersions() []int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	versions := make([]int64, 0, len(s.snapshots))
	for version := range s.snapshots {
		versions = append(versions, version)
	}
	
	// Sort versions
	for i := 0; i < len(versions); i++ {
		for j := i + 1; j < len(versions); j++ {
			if versions[i] > versions[j] {
				versions[i], versions[j] = versions[j], versions[i]
			}
		}
	}
	
	return versions
}
