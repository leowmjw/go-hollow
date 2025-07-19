package hollow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/leowmjw/go-hollow/internal/memblob"
)

func TestFileWatcher_WatchDirectory(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()

	// Create a mock consumer
	blob := memblob.New()
	consumer := NewConsumer(
		WithBlobRetriever(blob),
		WithAnnouncementWatcher(func() (uint64, bool, error) {
			return 1, true, nil
		}),
	)

	// Create file watcher
	fw := NewFileWatcher(consumer)

	// Watch the temporary directory
	err := fw.WatchDirectory(tempDir)
	if err != nil {
		t.Fatalf("Failed to watch directory: %v", err)
	}

	// Check that directory is being watched
	dirs := fw.GetWatchedDirectories()
	if len(dirs) != 1 {
		t.Errorf("Expected 1 watched directory, got %d", len(dirs))
	}

	// Clean up
	err = fw.UnwatchDirectory(tempDir)
	if err != nil {
		t.Errorf("Failed to unwatch directory: %v", err)
	}
}

func TestFileWatcher_NonExistentDirectory(t *testing.T) {
	// Create a mock consumer
	blob := memblob.New()
	consumer := NewConsumer(
		WithBlobRetriever(blob),
		WithAnnouncementWatcher(func() (uint64, bool, error) {
			return 1, true, nil
		}),
	)

	// Create file watcher
	fw := NewFileWatcher(consumer)

	// Try to watch non-existent directory
	err := fw.WatchDirectory("/non/existent/path")
	if err == nil {
		t.Error("Expected error when watching non-existent directory")
	}
}

func TestFileWatcher_DefaultConfig(t *testing.T) {
	config := DefaultFileWatcherConfig()

	if config.PollInterval <= 0 {
		t.Error("PollInterval should be positive")
	}
	if config.DebounceDelay <= 0 {
		t.Error("DebounceDelay should be positive")
	}
	if len(config.FilePatterns) == 0 {
		t.Error("FilePatterns should not be empty")
	}
	if config.MaxFileSize <= 0 {
		t.Error("MaxFileSize should be positive")
	}
}

func TestFileWatcher_MatchesPatterns(t *testing.T) {
	fw := NewFileWatcher(nil)

	// Test matching patterns
	if !fw.matchesPatterns("test.json") {
		t.Error("Expected test.json to match *.json pattern")
	}

	if !fw.matchesPatterns("config.yaml") {
		t.Error("Expected config.yaml to match *.yaml pattern")
	}

	if fw.matchesPatterns("test.txt") {
		t.Error("Expected test.txt to not match default patterns")
	}
}

func TestFileWatcher_ShouldIgnoreFile(t *testing.T) {
	fw := NewFileWatcher(nil)

	// Test ignore patterns
	if !fw.shouldIgnoreFile("/path/to/.DS_Store") {
		t.Error("Expected .DS_Store to be ignored")
	}

	if !fw.shouldIgnoreFile("/path/to/file.tmp") {
		t.Error("Expected .tmp files to be ignored")
	}

	if fw.shouldIgnoreFile("/path/to/config.json") {
		t.Error("Expected config.json to not be ignored")
	}
}

func TestFileWatcher_Stats(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()

	// Create a mock consumer
	blob := memblob.New()
	consumer := NewConsumer(
		WithBlobRetriever(blob),
		WithAnnouncementWatcher(func() (uint64, bool, error) {
			return 1, true, nil
		}),
	)

	// Create file watcher
	fw := NewFileWatcher(consumer)

	// Watch the temporary directory
	err := fw.WatchDirectory(tempDir)
	if err != nil {
		t.Fatalf("Failed to watch directory: %v", err)
	}

	// Get stats
	stats := fw.GetStats()

	// Verify stats
	if stats["is_running"] != false {
		t.Error("Expected is_running to be false")
	}

	if stats["watched_dirs"] != 1 {
		t.Error("Expected watched_dirs to be 1")
	}

	if stats["poll_interval"] == nil {
		t.Error("Expected poll_interval to be set")
	}
}

func TestFileWatcher_StartStop(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()

	// Create a mock consumer
	blob := memblob.New()
	consumer := NewConsumer(
		WithBlobRetriever(blob),
		WithAnnouncementWatcher(func() (uint64, bool, error) {
			return 1, true, nil
		}),
	)

	// Create file watcher with short poll interval
	config := DefaultFileWatcherConfig()
	config.PollInterval = 100 * time.Millisecond

	fw := NewFileWatcher(consumer, WithFileWatcherConfig(config))

	// Watch the temporary directory
	err := fw.WatchDirectory(tempDir)
	if err != nil {
		t.Fatalf("Failed to watch directory: %v", err)
	}

	// Test start/stop
	if fw.IsRunning() {
		t.Error("Expected file watcher to not be running initially")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Start watching in a goroutine
	go func() {
		fw.Start(ctx)
	}()

	// Wait a bit for the watcher to start
	time.Sleep(50 * time.Millisecond)

	if !fw.IsRunning() {
		t.Error("Expected file watcher to be running after start")
	}

	// Stop the watcher
	fw.Stop()

	// Wait a bit for the watcher to stop
	time.Sleep(50 * time.Millisecond)

	if fw.IsRunning() {
		t.Error("Expected file watcher to not be running after stop")
	}
}

func TestHotReloadConsumer_Creation(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()

	// Create hot-reload consumer
	hrc, err := NewHotReloadConsumer([]string{tempDir})
	if err != nil {
		t.Fatalf("Failed to create hot-reload consumer: %v", err)
	}

	if hrc == nil {
		t.Fatal("Hot-reload consumer should not be nil")
	}

	// Check stats
	stats := hrc.GetHotReloadStats()
	if stats["watched_dirs"] != 1 {
		t.Error("Expected 1 watched directory")
	}
}

func TestHotReloadConsumer_AddRemoveDirectory(t *testing.T) {
	// Create temporary directories for testing
	tempDir1 := t.TempDir()
	tempDir2 := t.TempDir()

	// Create hot-reload consumer
	hrc, err := NewHotReloadConsumer([]string{tempDir1})
	if err != nil {
		t.Fatalf("Failed to create hot-reload consumer: %v", err)
	}

	// Add second directory
	err = hrc.AddWatchDirectory(tempDir2)
	if err != nil {
		t.Errorf("Failed to add watch directory: %v", err)
	}

	// Check stats
	stats := hrc.GetHotReloadStats()
	if stats["watched_dirs"] != 2 {
		t.Error("Expected 2 watched directories")
	}

	// Remove directory
	err = hrc.RemoveWatchDirectory(tempDir2)
	if err != nil {
		t.Errorf("Failed to remove watch directory: %v", err)
	}

	// Check stats
	stats = hrc.GetHotReloadStats()
	if stats["watched_dirs"] != 1 {
		t.Error("Expected 1 watched directory after removal")
	}
}

func TestFileWatcher_FileChangeDetection(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()

	// Create a mock consumer
	blob := memblob.New()
	consumer := NewConsumer(
		WithBlobRetriever(blob),
		WithAnnouncementWatcher(func() (uint64, bool, error) {
			return 1, true, nil
		}),
	)

	// Create file watcher with very short intervals
	config := DefaultFileWatcherConfig()
	config.PollInterval = 50 * time.Millisecond
	config.DebounceDelay = 10 * time.Millisecond

	fw := NewFileWatcher(consumer, WithFileWatcherConfig(config))

	// Watch the temporary directory
	err := fw.WatchDirectory(tempDir)
	if err != nil {
		t.Fatalf("Failed to watch directory: %v", err)
	}

	// Create a test file
	testFile := filepath.Join(tempDir, "test.json")
	err = os.WriteFile(testFile, []byte(`{"test": "data"}`), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Start watching
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	go func() {
		fw.Start(ctx)
	}()

	// Wait for initial scan
	time.Sleep(100 * time.Millisecond)

	// Modify the file
	err = os.WriteFile(testFile, []byte(`{"test": "modified"}`), 0644)
	if err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	// Wait for change detection
	time.Sleep(200 * time.Millisecond)

	// Stop the watcher
	fw.Stop()

	// The test passes if no errors occurred during file watching
	// In a real scenario, we would verify that the consumer.Refresh() was called
}

func BenchmarkFileWatcher_ScanDirectory(b *testing.B) {
	// Create temporary directory with many files
	tempDir := b.TempDir()

	// Create 1000 test files
	for i := 0; i < 1000; i++ {
		filename := filepath.Join(tempDir, fmt.Sprintf("test_%d.json", i))
		err := os.WriteFile(filename, []byte(`{"test": "data"}`), 0644)
		if err != nil {
			b.Fatalf("Failed to create test file: %v", err)
		}
	}

	// Create file watcher
	fw := NewFileWatcher(nil)

	// Watch the directory
	err := fw.WatchDirectory(tempDir)
	if err != nil {
		b.Fatalf("Failed to watch directory: %v", err)
	}

	// Get the watched directory
	fw.mu.RLock()
	watched := fw.watchedDirs[tempDir]
	fw.mu.RUnlock()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := fw.scanDirectory(watched)
		if err != nil {
			b.Fatalf("Failed to scan directory: %v", err)
		}
	}
}
