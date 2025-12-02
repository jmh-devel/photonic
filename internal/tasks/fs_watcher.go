package tasks

import (
	"log"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileSystemEvent represents a file system change
type FileSystemEvent struct {
	Path      string    `json:"path"`
	Operation string    `json:"operation"` // "created", "modified", "deleted"
	Time      time.Time `json:"time"`
	Size      int64     `json:"size"`
}

// FileSystemWatcher monitors directories for changes
type FileSystemWatcher struct {
	watcher   *fsnotify.Watcher
	Events    chan FileSystemEvent
	watchDirs []string
	done      chan bool
}

// NewFileSystemWatcher creates a new filesystem watcher
func NewFileSystemWatcher(watchPaths []string) (*FileSystemWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	fsw := &FileSystemWatcher{
		watcher:   watcher,
		Events:    make(chan FileSystemEvent, 100),
		watchDirs: watchPaths,
		done:      make(chan bool),
	}

	return fsw, nil
}

// Start begins monitoring the configured directories
func (fsw *FileSystemWatcher) Start() error {
	// Add watch directories
	for _, dir := range fsw.watchDirs {
		err := fsw.watcher.Add(dir)
		if err != nil {
			return err
		}
		log.Printf("👀 Watching directory: %s", dir)
	}

	// Start event processing goroutine
	go fsw.processEvents()

	return nil
}

// Stop stops the filesystem watcher
func (fsw *FileSystemWatcher) Stop() error {
	close(fsw.done)
	close(fsw.Events)
	return fsw.watcher.Close()
}

// processEvents handles filesystem events and converts them to our format
func (fsw *FileSystemWatcher) processEvents() {
	for {
		select {
		case event, ok := <-fsw.watcher.Events:
			if !ok {
				return
			}

			// Convert fsnotify event to our format
			var operation string
			switch {
			case event.Op&fsnotify.Create == fsnotify.Create:
				operation = "created"
			case event.Op&fsnotify.Write == fsnotify.Write:
				operation = "modified"
			case event.Op&fsnotify.Remove == fsnotify.Remove:
				operation = "deleted"
			case event.Op&fsnotify.Rename == fsnotify.Rename:
				operation = "renamed"
			case event.Op&fsnotify.Chmod == fsnotify.Chmod:
				continue // Skip permission changes
			default:
				continue
			}

			// Only process photo/video files
			if !isPhotoFile(event.Name) {
				continue
			}

			// Get file size (if file still exists)
			var size int64
			if operation != "deleted" {
				if stat, err := filepath.Abs(event.Name); err == nil {
					if info, err := filepath.Glob(stat); err == nil && len(info) > 0 {
						// File exists, we can get size later if needed
						size = 0 // For now, just set to 0
					}
				}
			}

			fsEvent := FileSystemEvent{
				Path:      event.Name,
				Operation: operation,
				Time:      time.Now(),
				Size:      size,
			}

			// Send event (non-blocking)
			select {
			case fsw.Events <- fsEvent:
			default:
				log.Printf("⚠️ Event buffer full, dropping event for %s", event.Name)
			}

		case err, ok := <-fsw.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("❌ Filesystem watcher error: %v", err)

		case <-fsw.done:
			return
		}
	}
}

// isPhotoFile checks if the file is a photo/video file we care about
func isPhotoFile(path string) bool {
	ext := filepath.Ext(path)
	switch ext {
	case ".jpg", ".jpeg", ".png", ".tiff", ".tif", ".bmp", ".gif":
		return true
	case ".cr2", ".cr3", ".nef", ".arw", ".dng", ".raf", ".orf", ".rw2":
		return true
	case ".mp4", ".mov", ".avi", ".mkv", ".webm", ".3gp":
		return true
	case ".xmp": // Darktable sidecar files
		return true
	default:
		return false
	}
}
