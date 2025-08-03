package hollow

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FileWatcher provides hot-reload support for local development
type FileWatcher struct {
	mu          sync.RWMutex
	watchedDirs map[string]*WatchedDirectory
	consumer    *Consumer
	logger      *slog.Logger
	config      *FileWatcherConfig
	stopChan    chan struct{}
	isRunning   bool
}

// WatchedDirectory represents a directory being watched
type WatchedDirectory struct {
	Path         string
	Pattern      string
	LastModified time.Time
	Files        map[string]time.Time
}

// FileWatcherConfig configures the file watcher
type FileWatcherConfig struct {
	PollInterval    time.Duration
	DebounceDelay   time.Duration
	FilePatterns    []string
	IgnorePatterns  []string
	MaxFileSize     int64
	Recursive       bool
	EnableLogging   bool
}

// FileWatcherOpt configures a FileWatcher
type FileWatcherOpt func(*FileWatcher)

// WithFileWatcherLogger sets the logger for the file watcher
func WithFileWatcherLogger(logger *slog.Logger) FileWatcherOpt {
	return func(fw *FileWatcher) {
		fw.logger = logger
	}
}

// WithFileWatcherConfig sets the configuration for the file watcher
func WithFileWatcherConfig(config *FileWatcherConfig) FileWatcherOpt {
	return func(fw *FileWatcher) {
		fw.config = config
	}
}

// NewFileWatcher creates a new file watcher
func NewFileWatcher(consumer *Consumer, opts ...FileWatcherOpt) *FileWatcher {
	fw := &FileWatcher{
		watchedDirs: make(map[string]*WatchedDirectory),
		consumer:    consumer,
		logger:      slog.Default(),
		config:      DefaultFileWatcherConfig(),
		stopChan:    make(chan struct{}),
	}
	
	for _, opt := range opts {
		opt(fw)
	}
	
	return fw
}

// DefaultFileWatcherConfig returns default configuration
func DefaultFileWatcherConfig() *FileWatcherConfig {
	return &FileWatcherConfig{
		PollInterval:    1 * time.Second,
		DebounceDelay:   500 * time.Millisecond,
		FilePatterns:    []string{"*.json", "*.yaml", "*.yml", "*.toml"},
		IgnorePatterns:  []string{".git", ".DS_Store", "*.tmp", "*.swp"},
		MaxFileSize:     10 * 1024 * 1024, // 10MB
		Recursive:       true,
		EnableLogging:   true,
	}
}

// WatchDirectory adds a directory to watch
func (fw *FileWatcher) WatchDirectory(path string) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	
	// Check if directory exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("directory does not exist: %s", absPath)
	}
	
	// Initialize watched directory
	watched := &WatchedDirectory{
		Path:         absPath,
		Pattern:      "*",
		LastModified: time.Now(),
		Files:        make(map[string]time.Time),
	}
	
	// Scan initial files
	if err := fw.scanDirectory(watched); err != nil {
		return fmt.Errorf("failed to scan directory: %w", err)
	}
	
	fw.watchedDirs[absPath] = watched
	
	if fw.config.EnableLogging {
		fw.logger.Info("Added directory to watch", "path", absPath, "files", len(watched.Files))
	}
	
	return nil
}

// UnwatchDirectory removes a directory from watching
func (fw *FileWatcher) UnwatchDirectory(path string) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	
	if _, exists := fw.watchedDirs[absPath]; !exists {
		return fmt.Errorf("directory not being watched: %s", absPath)
	}
	
	delete(fw.watchedDirs, absPath)
	
	if fw.config.EnableLogging {
		fw.logger.Info("Removed directory from watch", "path", absPath)
	}
	
	return nil
}

// Start begins watching for file changes
func (fw *FileWatcher) Start(ctx context.Context) error {
	fw.mu.Lock()
	if fw.isRunning {
		fw.mu.Unlock()
		return fmt.Errorf("file watcher is already running")
	}
	fw.isRunning = true
	fw.mu.Unlock()
	
	if fw.config.EnableLogging {
		fw.logger.Info("Starting file watcher", "poll_interval", fw.config.PollInterval)
	}
	
	ticker := time.NewTicker(fw.config.PollInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			fw.mu.Lock()
			fw.isRunning = false
			fw.mu.Unlock()
			return ctx.Err()
		case <-fw.stopChan:
			fw.mu.Lock()
			fw.isRunning = false
			fw.mu.Unlock()
			return nil
		case <-ticker.C:
			fw.checkForChanges()
		}
	}
}

// Stop stops the file watcher
func (fw *FileWatcher) Stop() {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	
	if !fw.isRunning {
		return
	}
	
	close(fw.stopChan)
	fw.stopChan = make(chan struct{})
	
	if fw.config.EnableLogging {
		fw.logger.Info("Stopped file watcher")
	}
}

// checkForChanges scans watched directories for changes
func (fw *FileWatcher) checkForChanges() {
	fw.mu.RLock()
	watchedDirs := make(map[string]*WatchedDirectory)
	for k, v := range fw.watchedDirs {
		watchedDirs[k] = v
	}
	fw.mu.RUnlock()
	
	for _, watched := range watchedDirs {
		if err := fw.scanDirectory(watched); err != nil {
			fw.logger.Error("Failed to scan directory", "path", watched.Path, "error", err)
			continue
		}
	}
}

// scanDirectory scans a directory for file changes
func (fw *FileWatcher) scanDirectory(watched *WatchedDirectory) error {
	changes := make([]string, 0)
	newFiles := make(map[string]time.Time)
	
	walkFunc := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Skip directories
		if info.IsDir() {
			return nil
		}
		
		// Skip if file is too large
		if info.Size() > fw.config.MaxFileSize {
			return nil
		}
		
		// Check if file matches patterns
		if !fw.matchesPatterns(filepath.Base(path)) {
			return nil
		}
		
		// Check if file should be ignored
		if fw.shouldIgnoreFile(path) {
			return nil
		}
		
		modTime := info.ModTime()
		newFiles[path] = modTime
		
		// Check if file is new or modified
		if lastModTime, exists := watched.Files[path]; !exists || modTime.After(lastModTime) {
			changes = append(changes, path)
		}
		
		return nil
	}
	
	var err error
	if fw.config.Recursive {
		err = filepath.Walk(watched.Path, walkFunc)
	} else {
		entries, readErr := os.ReadDir(watched.Path)
		if readErr != nil {
			return readErr
		}
		
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			
			info, infoErr := entry.Info()
			if infoErr != nil {
				continue
			}
			
			path := filepath.Join(watched.Path, entry.Name())
			walkFunc(path, info, nil)
		}
	}
	
	if err != nil {
		return err
	}
	
	// Update watched files
	watched.Files = newFiles
	
	// Handle changes
	if len(changes) > 0 {
		fw.handleFileChanges(changes)
	}
	
	return nil
}

// matchesPatterns checks if a filename matches any of the configured patterns
func (fw *FileWatcher) matchesPatterns(filename string) bool {
	for _, pattern := range fw.config.FilePatterns {
		if matched, _ := filepath.Match(pattern, filename); matched {
			return true
		}
	}
	return false
}

// shouldIgnoreFile checks if a file should be ignored
func (fw *FileWatcher) shouldIgnoreFile(path string) bool {
	filename := filepath.Base(path)
	
	for _, pattern := range fw.config.IgnorePatterns {
		if matched, _ := filepath.Match(pattern, filename); matched {
			return true
		}
	}
	
	return false
}

// handleFileChanges processes file changes
func (fw *FileWatcher) handleFileChanges(changes []string) {
	if fw.config.EnableLogging {
		fw.logger.Info("Detected file changes", "count", len(changes), "files", changes)
	}
	
	// Debounce changes to avoid rapid successive updates
	time.Sleep(fw.config.DebounceDelay)
	
	// Trigger consumer refresh
	if err := fw.consumer.Refresh(); err != nil {
		fw.logger.Error("Failed to refresh consumer after file changes", "error", err)
	} else {
		if fw.config.EnableLogging {
			fw.logger.Info("Consumer refreshed after file changes", "version", fw.consumer.CurrentVersion())
		}
	}
}

// GetWatchedDirectories returns the list of watched directories
func (fw *FileWatcher) GetWatchedDirectories() []string {
	fw.mu.RLock()
	defer fw.mu.RUnlock()
	
	dirs := make([]string, 0, len(fw.watchedDirs))
	for path := range fw.watchedDirs {
		dirs = append(dirs, path)
	}
	
	return dirs
}

// IsRunning returns whether the file watcher is running
func (fw *FileWatcher) IsRunning() bool {
	fw.mu.RLock()
	defer fw.mu.RUnlock()
	return fw.isRunning
}

// GetStats returns statistics about the file watcher
func (fw *FileWatcher) GetStats() map[string]interface{} {
	fw.mu.RLock()
	defer fw.mu.RUnlock()
	
	totalFiles := 0
	for _, watched := range fw.watchedDirs {
		totalFiles += len(watched.Files)
	}
	
	return map[string]interface{}{
		"is_running":        fw.isRunning,
		"watched_dirs":      len(fw.watchedDirs),
		"total_files":       totalFiles,
		"poll_interval":     fw.config.PollInterval,
		"debounce_delay":    fw.config.DebounceDelay,
		"file_patterns":     fw.config.FilePatterns,
		"ignore_patterns":   fw.config.IgnorePatterns,
		"max_file_size":     fw.config.MaxFileSize,
		"recursive":         fw.config.Recursive,
	}
}

// HotReloadConsumer extends Consumer with hot-reload capabilities
type HotReloadConsumer struct {
	*Consumer
	fileWatcher *FileWatcher
	mu          sync.RWMutex
}

// NewHotReloadConsumer creates a consumer with hot-reload support
func NewHotReloadConsumer(watchDirs []string, opts ...ConsumerOpt) (*HotReloadConsumer, error) {
	consumer := NewConsumer(opts...)
	
	fileWatcher := NewFileWatcher(consumer,
		WithFileWatcherConfig(&FileWatcherConfig{
			PollInterval:    2 * time.Second,
			DebounceDelay:   1 * time.Second,
			FilePatterns:    []string{"*.json", "*.yaml", "*.yml", "*.toml", "*.conf"},
			IgnorePatterns:  []string{".git", ".DS_Store", "*.tmp", "*.swp", "*.log"},
			MaxFileSize:     50 * 1024 * 1024, // 50MB
			Recursive:       true,
			EnableLogging:   true,
		}),
	)
	
	// Add directories to watch
	for _, dir := range watchDirs {
		if err := fileWatcher.WatchDirectory(dir); err != nil {
			return nil, fmt.Errorf("failed to watch directory %s: %w", dir, err)
		}
	}
	
	return &HotReloadConsumer{
		Consumer:    consumer,
		fileWatcher: fileWatcher,
	}, nil
}

// StartHotReload starts the hot-reload functionality
func (hrc *HotReloadConsumer) StartHotReload(ctx context.Context) error {
	return hrc.fileWatcher.Start(ctx)
}

// StopHotReload stops the hot-reload functionality
func (hrc *HotReloadConsumer) StopHotReload() {
	hrc.fileWatcher.Stop()
}

// GetHotReloadStats returns hot-reload statistics
func (hrc *HotReloadConsumer) GetHotReloadStats() map[string]interface{} {
	return hrc.fileWatcher.GetStats()
}

// AddWatchDirectory adds a directory to the watch list
func (hrc *HotReloadConsumer) AddWatchDirectory(path string) error {
	return hrc.fileWatcher.WatchDirectory(path)
}

// RemoveWatchDirectory removes a directory from the watch list
func (hrc *HotReloadConsumer) RemoveWatchDirectory(path string) error {
	return hrc.fileWatcher.UnwatchDirectory(path)
}
