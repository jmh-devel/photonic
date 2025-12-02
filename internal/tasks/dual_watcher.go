package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"photonic/internal/darktable"
	"photonic/internal/storage"
)

// DualWatcher combines filesystem watching with darktable database monitoring
type DualWatcher struct {
	fsWatcher      *FileSystemWatcher
	darktableDB    *darktable.DarktableDB
	store          *storage.Store
	watchInterval  time.Duration
	enrichedPhotos chan EnrichedPhotoEvent
	ctx            context.Context
	cancel         context.CancelFunc
}

// EnrichedPhotoEvent combines filesystem events with darktable metadata
type EnrichedPhotoEvent struct {
	// Filesystem data
	FilePath  string    `json:"file_path"`
	EventType string    `json:"event_type"` // "created", "modified", "deleted"
	EventTime time.Time `json:"event_time"`
	FileSize  int64     `json:"file_size"`

	// Darktable metadata (if available)
	DarktableID   *int                     `json:"darktable_id,omitempty"`
	IsInDarktable bool                     `json:"is_in_darktable"`
	Metadata      *darktable.PhotoMetadata `json:"metadata,omitempty"`

	// Analysis flags
	IsNewImport bool `json:"is_new_import"` // New file, not in darktable yet
	IsProcessed bool `json:"is_processed"`  // Has darktable edits
	IsExported  bool `json:"is_exported"`   // Has been exported from darktable
}

// NewDualWatcher creates a combined filesystem + darktable watcher
func NewDualWatcher(
	watchPaths []string,
	store *storage.Store,
	darktableConfigPath string,
) (*DualWatcher, error) {

	// Setup filesystem watcher
	fsWatcher, err := NewFileSystemWatcher(watchPaths)
	if err != nil {
		return nil, fmt.Errorf("failed to create filesystem watcher: %w", err)
	}

	// Setup darktable database connection
	dtDB, err := darktable.NewDarktableDB(darktableConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create darktable db connection: %w", err)
	}

	if err := dtDB.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to darktable db: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &DualWatcher{
		fsWatcher:      fsWatcher,
		darktableDB:    dtDB,
		store:          store,
		watchInterval:  30 * time.Second, // Check darktable every 30s
		enrichedPhotos: make(chan EnrichedPhotoEvent, 100),
		ctx:            ctx,
		cancel:         cancel,
	}, nil
}

// Start begins monitoring both filesystem and darktable database
func (dw *DualWatcher) Start() error {
	log.Println("🚀 Starting dual filesystem + darktable watcher...")

	// Start filesystem watcher
	if err := dw.fsWatcher.Start(); err != nil {
		return fmt.Errorf("failed to start filesystem watcher: %w", err)
	}

	// Start darktable database monitoring
	go dw.watchDarktableChanges()

	// Start event correlation processor
	go dw.processEvents()

	log.Println("✅ Dual watcher started successfully")
	return nil
}

// Stop shuts down all monitoring
func (dw *DualWatcher) Stop() error {
	dw.cancel()

	var errs []error

	if err := dw.fsWatcher.Stop(); err != nil {
		errs = append(errs, fmt.Errorf("filesystem watcher stop error: %w", err))
	}

	if err := dw.darktableDB.Close(); err != nil {
		errs = append(errs, fmt.Errorf("darktable db close error: %w", err))
	}

	close(dw.enrichedPhotos)

	if len(errs) > 0 {
		return fmt.Errorf("stop errors: %v", errs)
	}

	log.Println("🛑 Dual watcher stopped")
	return nil
}

// GetEnrichedEvents returns the channel of enriched photo events
func (dw *DualWatcher) GetEnrichedEvents() <-chan EnrichedPhotoEvent {
	return dw.enrichedPhotos
}

// watchDarktableChanges monitors darktable database for modifications
func (dw *DualWatcher) watchDarktableChanges() {
	ticker := time.NewTicker(dw.watchInterval)
	defer ticker.Stop()

	lastCheck := time.Now().Add(-dw.watchInterval)

	for {
		select {
		case <-dw.ctx.Done():
			return
		case <-ticker.C:
			// Get recent darktable modifications
			modified, err := dw.darktableDB.GetRecentlyModified(lastCheck, 50)
			if err != nil {
				log.Printf("❌ Error checking darktable modifications: %v", err)
				continue
			}

			if len(modified) > 0 {
				log.Printf("📸 Found %d darktable modifications", len(modified))

				for _, photo := range modified {
					dw.enrichedPhotos <- EnrichedPhotoEvent{
						FilePath:      photo.FullPath,
						EventType:     "darktable_modified",
						EventTime:     photo.ChangeTime,
						DarktableID:   &photo.ID,
						IsInDarktable: true,
						Metadata:      &photo,
						IsProcessed:   photo.IsEdited,
						IsExported:    !photo.ExportTime.IsZero(),
					}
				}
			}

			lastCheck = time.Now()
		}
	}
}

// processEvents correlates filesystem events with darktable data
func (dw *DualWatcher) processEvents() {
	for {
		select {
		case <-dw.ctx.Done():
			return
		case fsEvent := <-dw.fsWatcher.Events:
			// Enrich filesystem event with darktable data
			enriched := dw.enrichFileSystemEvent(fsEvent)

			// Store in database for tracking
			if err := dw.storeEvent(enriched); err != nil {
				log.Printf("❌ Error storing event: %v", err)
			}

			// Send enriched event
			dw.enrichedPhotos <- enriched
		}
	}
}

// enrichFileSystemEvent adds darktable metadata to filesystem events
func (dw *DualWatcher) enrichFileSystemEvent(fsEvent FileSystemEvent) EnrichedPhotoEvent {
	enriched := EnrichedPhotoEvent{
		FilePath:      fsEvent.Path,
		EventType:     fsEvent.Operation,
		EventTime:     fsEvent.Time,
		FileSize:      fsEvent.Size,
		IsInDarktable: false,
		IsNewImport:   false,
		IsProcessed:   false,
		IsExported:    false,
	}

	// Try to find this file in darktable database
	// We need to check by folder + filename
	folderPath := filepath.Dir(fsEvent.Path)
	filename := filepath.Base(fsEvent.Path)

	// Query darktable for this specific photo
	photos, err := dw.darktableDB.GetByFolder(folderPath)
	if err != nil {
		log.Printf("⚠️ Error querying darktable for folder %s: %v", folderPath, err)
		return enriched
	}

	// Find matching photo by filename
	for _, photo := range photos {
		if photo.Filename == filename {
			enriched.DarktableID = &photo.ID
			enriched.IsInDarktable = true
			enriched.Metadata = &photo
			enriched.IsProcessed = photo.IsEdited
			enriched.IsExported = !photo.ExportTime.IsZero()
			break
		}
	}

	// Detect new imports
	if !enriched.IsInDarktable && fsEvent.Operation == "created" {
		enriched.IsNewImport = true
	}

	return enriched
}

// storeEvent saves the enriched event to database
func (dw *DualWatcher) storeEvent(event EnrichedPhotoEvent) error {
	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	query := `
		INSERT INTO photo_events (
			file_path, event_type, event_time, file_size,
			darktable_id, is_in_darktable, is_processed, is_exported,
			event_data, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
	`

	var darktableID interface{}
	if event.DarktableID != nil {
		darktableID = *event.DarktableID
	}

	_, err = dw.store.DB.Exec(query,
		event.FilePath,
		event.EventType,
		event.EventTime,
		event.FileSize,
		darktableID,
		event.IsInDarktable,
		event.IsProcessed,
		event.IsExported,
		string(eventData),
	)

	return err
}

// GetDarktableStats returns current darktable library statistics
func (dw *DualWatcher) GetDarktableStats() (map[string]interface{}, error) {
	return dw.darktableDB.GetStats()
}

// GetRecentDarktableEdits returns recently edited photos
func (dw *DualWatcher) GetRecentDarktableEdits(limit int) ([]darktable.PhotoMetadata, error) {
	return dw.darktableDB.GetEditedPhotos(limit)
}

// GetDarktableDB provides access to the darktable database for external queries
func (dw *DualWatcher) GetDarktableDB() *darktable.DarktableDB {
	return dw.darktableDB
}
