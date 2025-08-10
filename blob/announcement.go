package blob

import (
	"sync"
)

// Announcer interface for announcing new versions
type Announcer interface {
	AnnouncementWatcher
	Announce(version int64) error
	Subscribe(ch chan int64)
	Unsubscribe(ch chan int64)
}

// AnnouncementWatcher interface for watching announcements
type AnnouncementWatcher interface {
	GetLatestVersion() int64
	Pin(version int64)
	Unpin()
	IsPinned() bool
	GetPinnedVersion() int64
}

// InMemoryAnnouncement is an in-memory implementation of announcer and watcher
type InMemoryAnnouncement struct {
	mu              sync.RWMutex
	latestVersion   int64
	pinnedVersion   int64
	isPinned        bool
	watchers        []chan int64
}

// NewInMemoryAnnouncement creates a new in-memory announcement system
func NewInMemoryAnnouncement() *InMemoryAnnouncement {
	return &InMemoryAnnouncement{
		watchers: make([]chan int64, 0),
	}
}

func (a *InMemoryAnnouncement) Announce(version int64) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	if version > a.latestVersion {
		a.latestVersion = version
		
		// Notify watchers
		for _, watcher := range a.watchers {
			select {
			case watcher <- version:
			default:
				// Channel is full, skip
			}
		}
	}
	
	return nil
}

func (a *InMemoryAnnouncement) GetLatestVersion() int64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	if a.isPinned {
		return a.pinnedVersion
	}
	return a.latestVersion
}

func (a *InMemoryAnnouncement) Pin(version int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	a.isPinned = true
	a.pinnedVersion = version
}

func (a *InMemoryAnnouncement) Unpin() {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	a.isPinned = false
	a.pinnedVersion = 0
}

func (a *InMemoryAnnouncement) IsPinned() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.isPinned
}

func (a *InMemoryAnnouncement) GetPinnedVersion() int64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.pinnedVersion
}

// Subscribe adds a channel to receive version notifications
func (a *InMemoryAnnouncement) Subscribe(ch chan int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.watchers = append(a.watchers, ch)
}

// Unsubscribe removes a channel from notifications
func (a *InMemoryAnnouncement) Unsubscribe(ch chan int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	for i, watcher := range a.watchers {
		if watcher == ch {
			a.watchers = append(a.watchers[:i], a.watchers[i+1:]...)
			break
		}
	}
}

// AddWatcher adds a channel to receive version notifications (deprecated, use Subscribe)
func (a *InMemoryAnnouncement) AddWatcher(ch chan int64) {
	a.Subscribe(ch)
}

// RemoveWatcher removes a channel from notifications (deprecated, use Unsubscribe)
func (a *InMemoryAnnouncement) RemoveWatcher(ch chan int64) {
	a.Unsubscribe(ch)
}
