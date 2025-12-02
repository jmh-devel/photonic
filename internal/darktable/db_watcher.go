package darktable

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// darktableToUnixTime converts darktable timestamp (nanoseconds since 0001-01-01) to Unix timestamp
func darktableToUnixTime(dtNanos int64) time.Time {
	if dtNanos <= 0 {
		return time.Time{}
	}

	// Darktable stores nanoseconds since "0001-01-01 00:00:00"
	// Convert to seconds since 0001-01-01
	secondsSince0001 := dtNanos / 1000000000

	// Create time from year 1 and add the seconds
	year1 := time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
	return year1.Add(time.Duration(secondsSince0001) * time.Second)
}

// DarktableDB provides read-only access to darktable's database
type DarktableDB struct {
	libraryPath string
	dataPath    string
	libraryDB   *sql.DB
	dataDB      *sql.DB
}

// PhotoMetadata represents enhanced photo info from darktable
type PhotoMetadata struct {
	ID            int       `json:"id"`
	Filename      string    `json:"filename"`
	Folder        string    `json:"folder"`
	FullPath      string    `json:"full_path"`
	DateTimeTaken time.Time `json:"datetime_taken"`
	Flags         int       `json:"flags"`
	IsEdited      bool      `json:"is_edited"`
	HistoryEnd    int       `json:"history_end"`
	Width         int       `json:"width"`
	Height        int       `json:"height"`
	Maker         string    `json:"maker"`
	Model         string    `json:"model"`
	ISO           float64   `json:"iso"`
	Aperture      float64   `json:"aperture"`
	Exposure      float64   `json:"exposure"`
	FocalLength   float64   `json:"focal_length"`
	ImportTime    time.Time `json:"import_time"`
	ChangeTime    time.Time `json:"change_time"`
	ExportTime    time.Time `json:"export_time"`
	Tags          []string  `json:"tags"`
	ColorLabel    int       `json:"color_label"`
}

// Collection represents a darktable collection filter
type Collection struct {
	Name        string           `json:"name"`
	Rules       []CollectionRule `json:"rules"`
	Count       int              `json:"count"`
	LastUpdated time.Time        `json:"last_updated"`
}

// CollectionRule represents a single filter rule
type CollectionRule struct {
	Field    string `json:"field"`    // "folder", "tag", "rating", "format", etc.
	Operator string `json:"operator"` // "=", "!=", "LIKE", ">", "<", etc.
	Value    string `json:"value"`    // The filter value
}

// NewDarktableDB creates a new read-only connection to darktable databases
func NewDarktableDB(configPath string) (*DarktableDB, error) {
	if configPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		configPath = filepath.Join(homeDir, ".config", "darktable")
	}

	libraryPath := filepath.Join(configPath, "library.db")
	dataPath := filepath.Join(configPath, "data.db")

	// Verify files exist
	if _, err := os.Stat(libraryPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("darktable library.db not found at %s", libraryPath)
	}
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("darktable data.db not found at %s", dataPath)
	}

	db := &DarktableDB{
		libraryPath: libraryPath,
		dataPath:    dataPath,
	}

	return db, nil
}

// Connect opens read-only connections to both databases
func (db *DarktableDB) Connect() error {
	var err error

	// Open library database (read-only)
	db.libraryDB, err = sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=ro", db.libraryPath))
	if err != nil {
		return fmt.Errorf("failed to open library database: %w", err)
	}

	// Open data database (read-only)
	db.dataDB, err = sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=ro", db.dataPath))
	if err != nil {
		db.libraryDB.Close()
		return fmt.Errorf("failed to open data database: %w", err)
	}

	// Test connections
	if err := db.libraryDB.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("library database ping failed: %w", err)
	}

	if err := db.dataDB.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("data database ping failed: %w", err)
	}

	log.Println("✅ Connected to darktable databases (read-only)")
	return nil
}

// Close closes database connections
func (db *DarktableDB) Close() error {
	var errs []error

	if db.libraryDB != nil {
		if err := db.libraryDB.Close(); err != nil {
			errs = append(errs, fmt.Errorf("library db close error: %w", err))
		}
	}

	if db.dataDB != nil {
		if err := db.dataDB.Close(); err != nil {
			errs = append(errs, fmt.Errorf("data db close error: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}

	return nil
}

// GetStats returns basic statistics about the photo library
func (db *DarktableDB) GetStats() (map[string]interface{}, error) {
	query := `
		SELECT 
			COUNT(*) as total_images,
			COUNT(CASE WHEN history_end > 0 THEN 1 END) as edited_images,
			COUNT(DISTINCT film_id) as film_rolls,
			MIN(datetime_taken) as earliest_photo,
			MAX(datetime_taken) as latest_photo,
			MAX(import_timestamp) as last_import
		FROM images
	`

	row := db.libraryDB.QueryRow(query)
	var totalImages, editedImages, filmRolls int
	var earliestPhoto, latestPhoto, lastImport int64

	err := row.Scan(&totalImages, &editedImages, &filmRolls, &earliestPhoto, &latestPhoto, &lastImport)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	// darktable stores datetime_taken as nanoseconds since "0001-01-01 00:00:00"
	// Convert to Unix timestamps by dividing by 1,000,000 (ns to seconds) and
	// adding the epoch offset
	const epochOffset = -62135596800 // Unix timestamp of "0001-01-01 00:00:00"

	stats := map[string]interface{}{
		"total_images":    totalImages,
		"edited_images":   editedImages,
		"unedited_images": totalImages - editedImages,
		"film_rolls":      filmRolls,
		"edit_percentage": float64(editedImages) / float64(totalImages) * 100,
	}

	if earliestPhoto > 0 {
		unixTime := (earliestPhoto / 1000000) + epochOffset
		stats["earliest_photo"] = time.Unix(unixTime, 0)
	}
	if latestPhoto > 0 {
		unixTime := (latestPhoto / 1000000) + epochOffset
		stats["latest_photo"] = time.Unix(unixTime, 0)
	}
	if lastImport > 0 {
		unixTime := (lastImport / 1000000) + epochOffset
		stats["last_import"] = time.Unix(unixTime, 0)
	}

	return stats, nil
}

// GetRecentlyModified returns photos recently modified in darktable
func (db *DarktableDB) GetRecentlyModified(since time.Time, limit int) ([]PhotoMetadata, error) {
	query := `
		SELECT 
			i.id, i.filename, f.folder, i.datetime_taken, i.flags, i.history_end,
			i.width, i.height, i.iso, i.aperture, i.exposure, i.focal_length,
			i.import_timestamp, i.change_timestamp, i.export_timestamp
		FROM images i
		JOIN film_rolls f ON i.film_id = f.id
		WHERE i.change_timestamp > ?
		ORDER BY i.change_timestamp DESC
		LIMIT ?
	`

	sinceUnix := since.Unix()
	rows, err := db.libraryDB.Query(query, sinceUnix, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recently modified: %w", err)
	}
	defer rows.Close()

	var photos []PhotoMetadata
	for rows.Next() {
		var p PhotoMetadata
		var importTs, changeTs, exportTs, dateTimeTakenTs int64

		err := rows.Scan(
			&p.ID, &p.Filename, &p.Folder, &dateTimeTakenTs, &p.Flags, &p.HistoryEnd,
			&p.Width, &p.Height, &p.ISO, &p.Aperture, &p.Exposure, &p.FocalLength,
			&importTs, &changeTs, &exportTs,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		p.FullPath = filepath.Join(p.Folder, p.Filename)
		p.IsEdited = p.HistoryEnd > 0

		if dateTimeTakenTs > 0 {
			// Convert darktable nanoseconds since 0001-01-01 to Unix timestamp
			unixTime := (dateTimeTakenTs / 1000000) + (-62135596800)
			p.DateTimeTaken = time.Unix(unixTime, 0)
		}
		if importTs > 0 {
			p.ImportTime = time.Unix(importTs, 0)
		}
		if changeTs > 0 {
			p.ChangeTime = time.Unix(changeTs, 0)
		}
		if exportTs > 0 {
			p.ExportTime = time.Unix(exportTs, 0)
		}

		photos = append(photos, p)
	}

	return photos, nil
}

// GetByFolder returns all photos in a specific folder
func (db *DarktableDB) GetByFolder(folderPath string) ([]PhotoMetadata, error) {
	query := `
		SELECT 
			i.id, i.filename, f.folder, i.datetime_taken, i.flags, i.history_end,
			i.width, i.height, i.iso, i.aperture, i.exposure, i.focal_length
		FROM images i
		JOIN film_rolls f ON i.film_id = f.id
		WHERE f.folder = ?
		ORDER BY i.datetime_taken ASC
	`

	rows, err := db.libraryDB.Query(query, folderPath)
	if err != nil {
		return nil, fmt.Errorf("failed to query folder %s: %w", folderPath, err)
	}
	defer rows.Close()

	var photos []PhotoMetadata
	for rows.Next() {
		var p PhotoMetadata
		var dateTimeTakenTs int64
		err := rows.Scan(
			&p.ID, &p.Filename, &p.Folder, &dateTimeTakenTs, &p.Flags, &p.HistoryEnd,
			&p.Width, &p.Height, &p.ISO, &p.Aperture, &p.Exposure, &p.FocalLength,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		p.FullPath = filepath.Join(p.Folder, p.Filename)
		p.IsEdited = p.HistoryEnd > 0

		if dateTimeTakenTs > 0 {
			// Convert darktable nanoseconds since 0001-01-01 to Unix timestamp
			unixTime := (dateTimeTakenTs / 1000000) + (-62135596800)
			p.DateTimeTaken = time.Unix(unixTime, 0)
		}

		photos = append(photos, p)
	}

	return photos, nil
}

// GetEditedPhotos returns photos that have been edited in darktable
func (db *DarktableDB) GetEditedPhotos(limit int) ([]PhotoMetadata, error) {
	query := `
		SELECT 
			i.id, i.filename, f.folder, i.datetime_taken, i.flags, i.history_end,
			i.change_timestamp
		FROM images i
		JOIN film_rolls f ON i.film_id = f.id
		WHERE i.history_end > 0
		ORDER BY i.change_timestamp DESC
		LIMIT ?
	`

	rows, err := db.libraryDB.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query edited photos: %w", err)
	}
	defer rows.Close()

	var photos []PhotoMetadata
	for rows.Next() {
		var p PhotoMetadata
		var changeTs, dateTimeTakenTs int64

		err := rows.Scan(
			&p.ID, &p.Filename, &p.Folder, &dateTimeTakenTs, &p.Flags, &p.HistoryEnd,
			&changeTs,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		p.FullPath = filepath.Join(p.Folder, p.Filename)
		p.IsEdited = true
		if dateTimeTakenTs > 0 {
			// Convert darktable nanoseconds since 0001-01-01 to Unix timestamp
			unixTime := (dateTimeTakenTs / 1000000) + (-62135596800)
			p.DateTimeTaken = time.Unix(unixTime, 0)
		}
		if changeTs > 0 {
			p.ChangeTime = time.Unix(changeTs, 0)
		}

		photos = append(photos, p)
	}

	return photos, nil
}

// WatchForChanges monitors the darktable database for modifications
func (db *DarktableDB) WatchForChanges(interval time.Duration, callback func([]PhotoMetadata)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	lastCheck := time.Now().Add(-interval) // Start from one interval ago

	for range ticker.C {
		modified, err := db.GetRecentlyModified(lastCheck, 100)
		if err != nil {
			log.Printf("Error checking for modifications: %v", err)
			continue
		}

		if len(modified) > 0 {
			log.Printf("📸 Found %d darktable modifications since %v", len(modified), lastCheck)
			callback(modified)
		}

		lastCheck = time.Now()
	}
}
