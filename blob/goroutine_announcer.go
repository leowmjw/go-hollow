package blob

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// goroutineSubscription implements the Subscription interface for GoroutineAnnouncer
type goroutineSubscription struct {
	ch       chan int64
	announcer *GoroutineAnnouncer
	closed   bool
	mu       sync.Mutex
}

func (s *goroutineSubscription) Updates() <-chan int64 {
	return s.ch
}

func (s *goroutineSubscription) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return
	}
	s.closed = true
	
	s.announcer.Unsubscribe(s.ch)
	// Don't close the channel here as it might be closed elsewhere
	// The announcer cleanup should handle channel closing if needed
}

// GoroutineAnnouncer implements Announcer, VersionCursor, and Subscribable using goroutines
type GoroutineAnnouncer struct {
	mu              sync.RWMutex
	latestVersion   int64
	pinnedVersion   int64
	isPinned        bool
	subscribers     []chan int64
	ctx             context.Context
	cancel          context.CancelFunc
	announceQueue   chan int64
}

// NewGoroutineAnnouncer creates a new goroutine-based announcer
func NewGoroutineAnnouncer() *GoroutineAnnouncer {
	ctx, cancel := context.WithCancel(context.Background())
	
	announcer := &GoroutineAnnouncer{
		subscribers:   make([]chan int64, 0),
		ctx:           ctx,
		cancel:        cancel,
		announceQueue: make(chan int64, 100), // Buffered to handle bursts
	}
	
	// Start the announcement worker goroutine
	go announcer.worker()
	
	return announcer
}

// worker runs in a separate goroutine to handle announcements
func (ga *GoroutineAnnouncer) worker() {
	ticker := time.NewTicker(50 * time.Millisecond) // Check for announcements every 50ms
	defer ticker.Stop()
	
	for {
		select {
		case <-ga.ctx.Done():
			return
			
		case version := <-ga.announceQueue:
			ga.processAnnouncement(version)
			
		case <-ticker.C:
			// Periodic check - could be used for health checks or cleanup
			ga.cleanupDeadSubscribers()
		}
	}
}

func (ga *GoroutineAnnouncer) processAnnouncement(version int64) {
	ga.mu.Lock()
	defer ga.mu.Unlock()
	
	if version >= ga.latestVersion {
		ga.latestVersion = version
		
		// Notify all subscribers
		deadSubscribers := make([]int, 0)
		
		for i, subscriber := range ga.subscribers {
			// Check if channel is still valid before sending
			if !ga.safeChannelSend(subscriber, version) {
				// Channel is full, closed, or send failed - mark for removal
				deadSubscribers = append(deadSubscribers, i)
			}
		}
		
		// Remove dead subscribers (iterate in reverse to maintain indices)
		for i := len(deadSubscribers) - 1; i >= 0; i-- {
			idx := deadSubscribers[i]
			ga.subscribers = append(ga.subscribers[:idx], ga.subscribers[idx+1:]...)
		}
	}
}

// safeChannelSend safely sends a version to a channel, handling panics from closed channels
func (ga *GoroutineAnnouncer) safeChannelSend(ch chan int64, version int64) bool {
	defer func() {
		if r := recover(); r != nil {
			// Channel was closed, ignore the panic
		}
	}()
	
	select {
	case ch <- version:
		return true
	default:
		// Channel is full or blocked
		return false
	}
}

func (ga *GoroutineAnnouncer) cleanupDeadSubscribers() {
	ga.mu.Lock()
	defer ga.mu.Unlock()
	
	activeSubscribers := make([]chan int64, 0, len(ga.subscribers))
	
	for _, subscriber := range ga.subscribers {
		select {
		case <-ga.ctx.Done():
			return
		default:
			// Check if channel is still open
			if subscriber != nil {
				activeSubscribers = append(activeSubscribers, subscriber)
			}
		}
	}
	
	ga.subscribers = activeSubscribers
}

// Announce implements Announcer interface
func (ga *GoroutineAnnouncer) Announce(version int64) error {
    // Update latest version immediately to avoid races with consumers querying it
    ga.mu.Lock()
    if version > ga.latestVersion {
        ga.latestVersion = version
    }
    ga.mu.Unlock()

    select {
    case ga.announceQueue <- version:
        return nil
    case <-ga.ctx.Done():
        return context.Canceled
    case <-time.After(1 * time.Second):
        return fmt.Errorf("announcement queue is full")
    }
}

// Latest implements VersionCursor interface
func (ga *GoroutineAnnouncer) Latest() int64 {
	ga.mu.RLock()
	defer ga.mu.RUnlock()
	
	if ga.isPinned {
		return ga.pinnedVersion
	}
	return ga.latestVersion
}

// GetLatestVersion is deprecated, use Latest() instead
func (ga *GoroutineAnnouncer) GetLatestVersion() int64 {
	return ga.Latest()
}

// Pin implements AnnouncementWatcher interface
func (ga *GoroutineAnnouncer) Pin(version int64) {
	ga.mu.Lock()
	defer ga.mu.Unlock()
	
	ga.isPinned = true
	ga.pinnedVersion = version
	
	// Announce the pinned version to all subscribers
	ga.announceVersionLocked(version)
}

// Unpin implements AnnouncementWatcher interface
func (ga *GoroutineAnnouncer) Unpin() {
	ga.mu.Lock()
	defer ga.mu.Unlock()
	
	ga.isPinned = false
	ga.pinnedVersion = 0
	
	// Announce the latest version to all subscribers
	ga.announceVersionLocked(ga.latestVersion)
}

// IsPinned implements AnnouncementWatcher interface
func (ga *GoroutineAnnouncer) IsPinned() bool {
	ga.mu.RLock()
	defer ga.mu.RUnlock()
	return ga.isPinned
}

// Pinned implements VersionCursor interface
func (ga *GoroutineAnnouncer) Pinned() (version int64, ok bool) {
	ga.mu.RLock()
	defer ga.mu.RUnlock()
	return ga.pinnedVersion, ga.isPinned
}

// GetPinnedVersion is deprecated, use Pinned() instead
func (ga *GoroutineAnnouncer) GetPinnedVersion() int64 {
	version, _ := ga.Pinned()
	return version
}

// Subscribe implements Subscribable interface
func (ga *GoroutineAnnouncer) Subscribe(bufferSize int) (Subscription, error) {
	ch := make(chan int64, bufferSize)
	
	ga.mu.Lock()
	defer ga.mu.Unlock()
	ga.subscribers = append(ga.subscribers, ch)
	
	sub := &goroutineSubscription{
		ch:        ch,
		announcer: ga,
	}
	
	return sub, nil
}

// SubscribeChannel adds a channel to receive version notifications (deprecated)
func (ga *GoroutineAnnouncer) SubscribeChannel(ch chan int64) {
	ga.mu.Lock()
	defer ga.mu.Unlock()
	ga.subscribers = append(ga.subscribers, ch)
}

// Unsubscribe removes a channel from notifications
func (ga *GoroutineAnnouncer) Unsubscribe(ch chan int64) {
	ga.mu.Lock()
	defer ga.mu.Unlock()
	
	for i, subscriber := range ga.subscribers {
		if subscriber == ch {
			ga.subscribers = append(ga.subscribers[:i], ga.subscribers[i+1:]...)
			break
		}
	}
}

// GetSubscriberCount returns the number of active subscribers
func (ga *GoroutineAnnouncer) GetSubscriberCount() int {
	ga.mu.RLock()
	defer ga.mu.RUnlock()
	return len(ga.subscribers)
}

// Close stops the announcer and cancels all goroutines
func (ga *GoroutineAnnouncer) Close() error {
	ga.cancel()
	
	// Close all subscriber channels safely
	ga.mu.Lock()
	defer ga.mu.Unlock()
	
	for _, subscriber := range ga.subscribers {
		// Safely close channels, handling potential double-close
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Channel already closed, ignore panic
				}
			}()
			close(subscriber)
		}()
	}
	ga.subscribers = nil
	
	// Close announce queue safely
	func() {
		defer func() {
			if r := recover(); r != nil {
				// Queue already closed, ignore panic
			}
		}()
		close(ga.announceQueue)
	}()
	
	return nil
}

// announceVersionLocked announces a version to all subscribers (caller must hold lock)
func (ga *GoroutineAnnouncer) announceVersionLocked(version int64) {
	deadSubscribers := make([]int, 0)
	
	for i, subscriber := range ga.subscribers {
		// Check if channel is still valid before sending
		if !ga.safeChannelSend(subscriber, version) {
			// Channel is full, closed, or send failed - mark for removal
			deadSubscribers = append(deadSubscribers, i)
		}
	}
	
	// Remove dead subscribers (iterate in reverse to maintain indices)
	for i := len(deadSubscribers) - 1; i >= 0; i-- {
		idx := deadSubscribers[i]
		ga.subscribers = append(ga.subscribers[:idx], ga.subscribers[idx+1:]...)
	}
}

// WaitForVersion waits for a specific version to be announced (with timeout)
func (ga *GoroutineAnnouncer) WaitForVersion(targetVersion int64, timeout time.Duration) error {
	// Check current version first
	if ga.GetLatestVersion() >= targetVersion {
		return nil
	}
	
	// Subscribe to announcements
	ch := make(chan int64, 10)
	ga.SubscribeChannel(ch)
	defer ga.Unsubscribe(ch)
	
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	
	for {
		select {
		case version := <-ch:
			if version >= targetVersion {
				return nil
			}
		case <-timer.C:
			return fmt.Errorf("timeout waiting for version %d", targetVersion)
		case <-ga.ctx.Done():
			return context.Canceled
		}
	}
}
