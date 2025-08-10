package blob

import (
	"sync"
)

// Announcer publishes new versions (producer-side interface)
type Announcer interface {
	Announce(version int64) error
}

// VersionCursor provides read/update control over a version pointer (consumer-side interface)
type VersionCursor interface {
	Latest() int64                       // Get the current version (respects pinning)
	Pin(version int64)                   // Fix the cursor to a specific version
	Unpin()                             // Release the pin, return to tracking latest
	Pinned() (version int64, ok bool)    // Get pinned version if pinned
}

// Subscription represents an active subscription to version updates
type Subscription interface {
	Updates() <-chan int64  // Read-only channel for version updates
	Close()                 // Terminate the subscription and free resources
}

// Subscribable can create subscriptions for version updates
type Subscribable interface {
	Subscribe(bufferSize int) (Subscription, error)
}

// subscription implements the Subscription interface
type subscription struct {
	ch   chan int64
	feed *InMemoryFeed
}

func (s *subscription) Updates() <-chan int64 {
	return s.ch
}

func (s *subscription) Close() {
	s.feed.mu.Lock()
	defer s.feed.mu.Unlock()
	
	delete(s.feed.subs, s)
	close(s.ch)
}

// InMemoryFeed is an in-memory implementation that composes all announcement interfaces
type InMemoryFeed struct {
	mu              sync.RWMutex
	latestVersion   int64
	pinnedVersion   int64
	isPinned        bool
	subs            map[*subscription]struct{} // Active subscriptions
}

// NewInMemoryFeed creates a new in-memory announcement system
func NewInMemoryFeed() *InMemoryFeed {
	return &InMemoryFeed{
		subs: make(map[*subscription]struct{}),
	}
}

// NewInMemoryAnnouncement creates a new in-memory announcement system (deprecated, use NewInMemoryFeed)
func NewInMemoryAnnouncement() *InMemoryFeed {
	return NewInMemoryFeed()
}

// Announcer interface implementation
func (f *InMemoryFeed) Announce(version int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	if version > f.latestVersion {
		f.latestVersion = version
		
		// Notify all subscribers
		for sub := range f.subs {
			select {
			case sub.ch <- version:
			default:
				// Channel is full, skip (non-blocking)
			}
		}
	}
	
	return nil
}

// VersionCursor interface implementation
func (f *InMemoryFeed) Latest() int64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	if f.isPinned {
		return f.pinnedVersion
	}
	return f.latestVersion
}

func (f *InMemoryFeed) Pin(version int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	f.isPinned = true
	f.pinnedVersion = version
}

func (f *InMemoryFeed) Unpin() {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	f.isPinned = false
	f.pinnedVersion = 0
}

func (f *InMemoryFeed) Pinned() (version int64, ok bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.pinnedVersion, f.isPinned
}

// Subscribable interface implementation
func (f *InMemoryFeed) Subscribe(bufferSize int) (Subscription, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	sub := &subscription{
		ch:   make(chan int64, bufferSize),
		feed: f,
	}
	f.subs[sub] = struct{}{}
	
	return sub, nil
}

// Backward compatibility methods for old interface

// GetLatestVersion is deprecated, use Latest() instead
func (f *InMemoryFeed) GetLatestVersion() int64 {
	return f.Latest()
}

// IsPinned is deprecated, use Pinned() instead
func (f *InMemoryFeed) IsPinned() bool {
	_, ok := f.Pinned()
	return ok
}

// GetPinnedVersion is deprecated, use Pinned() instead
func (f *InMemoryFeed) GetPinnedVersion() int64 {
	version, _ := f.Pinned()
	return version
}

// SubscribeChannel is deprecated for direct channel usage, use Subscribe(bufferSize) instead
func (f *InMemoryFeed) SubscribeChannel(ch chan int64) {
	// This is a legacy method - consumers should use the new Subscribe(bufferSize) method
	// For backward compatibility, we'll store this channel in a simple way
	// Note: This doesn't integrate well with the new subscription system
	f.mu.Lock()
	defer f.mu.Unlock()
	
	// Create a fake subscription to maintain the channel
	sub := &subscription{ch: ch, feed: f}
	f.subs[sub] = struct{}{}
}

// Unsubscribe is deprecated
func (f *InMemoryFeed) Unsubscribe(ch chan int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	// Find and remove the subscription with this channel
	for sub := range f.subs {
		if sub.ch == ch {
			delete(f.subs, sub)
			break
		}
	}
}
