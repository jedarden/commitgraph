// Package watcher provides file system watching with debouncing.
package watcher

import (
	"log"
	"os"
	"path/filepath"
	"time"
)

// FileWatcher watches a file for changes and triggers callbacks with debouncing.
type FileWatcher struct {
	path         string
	pollInterval time.Duration
	debounce     time.Duration
	modTime      time.Time
	callback     func() error
	running      bool
}

// NewFileWatcher creates a new file watcher.
//
// Parameters:
// - path: Absolute path to the file to watch
// - pollInterval: How often to check for changes (e.g., 5s)
// - debounce: How long to wait after last change before triggering (e.g., 5s)
// - callback: Function to call when a change is detected (should be idempotent)
func NewFileWatcher(path string, pollInterval time.Duration, debounce time.Duration, callback func() error) *FileWatcher {
	return &FileWatcher{
		path:         path,
		pollInterval: pollInterval,
		debounce:     debounce,
		callback:     callback,
		running:      false,
	}
}

// Run starts the watch loop. Blocks until stopped.
//
// The loop:
// 1. Checks file modification time on each poll interval
// 2. If changed, waits for debounce period
// 3. Re-checks to avoid duplicate triggers
// 4. Calls callback if still changed
// 5. Handles callback errors by logging but continuing to watch
func (fw *FileWatcher) Run(initialRun bool) error {
	// Initialize modTime from current file state
	info, err := os.Stat(fw.path)
	if err != nil {
		return err
	}
	fw.modTime = info.ModTime()
	fw.running = true

	log.Printf("Watching file: %s (poll interval: %v, debounce: %v)\n", fw.path, fw.pollInterval, fw.debounce)

	// Initial run if requested
	if initialRun {
		log.Println("Running initial callback...")
		if err := fw.callback(); err != nil {
			log.Printf("Initial callback error (will continue watching): %v\n", err)
		}
	}

	ticker := time.NewTicker(fw.pollInterval)
	defer ticker.Stop()

	var debounceTimer *time.Timer

	for fw.running {
		select {
		case <-ticker.C:
			info, err := os.Stat(fw.path)
			if err != nil {
				// File might be temporarily unavailable, log and continue
				log.Printf("Error stating file (will retry): %v\n", err)
				continue
			}

			currentModTime := info.ModTime()
			if currentModTime.After(fw.modTime) {
				// File changed
				log.Printf("Detected file change: %s (mod time: %v)\n", fw.path, currentModTime)
				fw.modTime = currentModTime

				// Reset or start debounce timer
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				debounceTimer = time.AfterFunc(fw.debounce, func() {
					// Re-check file state to avoid duplicate triggers
					info, err := os.Stat(fw.path)
					if err != nil {
						log.Printf("Error stating file during debounce (will retry): %v\n", err)
						return
					}

					// Only trigger if this is still the latest change
					if info.ModTime().Equal(fw.modTime) {
						log.Println("Debounce period elapsed, triggering callback...")
						if err := fw.callback(); err != nil {
							log.Printf("Callback error (will continue watching): %v\n", err)
						} else {
							log.Println("Callback completed successfully")
						}
					}
				})
			}
		}
	}

	return nil
}

// Stop stops the watch loop.
func (fw *FileWatcher) Stop() {
	fw.running = false
	log.Println("File watcher stopped")
}

// ResolvePath resolves a path to an absolute path.
func ResolvePath(path string) (string, error) {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = home + path[1:]
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	return absPath, nil
}
