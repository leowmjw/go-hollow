// Package memblob provides an in-memory implementation of blob storage.
package memblob

import (
	"context"
	"fmt"
	"sync"
)

// Store is an in-memory blob store.
type Store struct {
	mu      sync.RWMutex
	blobs   map[uint64][]byte
	staged  map[uint64][]byte
	latest  uint64
}

// New creates a new in-memory blob store.
func New() *Store {
	return &Store{
		blobs:  make(map[uint64][]byte),
		staged: make(map[uint64][]byte),
	}
}

// Stage stages data for a given version.
func (s *Store) Stage(ctx context.Context, version uint64, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Copy data to avoid external mutations
	staged := make([]byte, len(data))
	copy(staged, data)
	
	s.staged[version] = staged
	return nil
}

// Commit commits staged data for a given version.
func (s *Store) Commit(ctx context.Context, version uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	data, ok := s.staged[version]
	if !ok {
		return fmt.Errorf("no staged data for version %d", version)
	}
	
	// Move from staged to committed
	s.blobs[version] = data
	delete(s.staged, version)
	
	if version > s.latest {
		s.latest = version
	}
	
	return nil
}

// Retrieve retrieves data for a given version.
func (s *Store) Retrieve(ctx context.Context, version uint64) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	data, ok := s.blobs[version]
	if !ok {
		return nil, fmt.Errorf("no data for version %d", version)
	}
	
	// Return a copy to avoid external mutations
	result := make([]byte, len(data))
	copy(result, data)
	return result, nil
}

// Latest returns the latest version available.
func (s *Store) Latest(ctx context.Context) (uint64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	return s.latest, nil
}
