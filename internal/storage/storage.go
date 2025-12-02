package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Store wraps SQLite-backed persistence for jobs and groups.
type Store struct {
	DB *sql.DB // Export for direct database access
}

// New opens (or creates) the database at path and ensures schema.
func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	s := &Store{DB: db}
	if err := s.ensureSchema(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) ensureSchema() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS processing_jobs (
            id TEXT PRIMARY KEY,
            job_type TEXT NOT NULL,
            status TEXT NOT NULL,
            input_path TEXT,
            output_path TEXT,
            options_json TEXT,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            started_at TIMESTAMP,
            completed_at TIMESTAMP,
            error_message TEXT
        );`,
		`CREATE TABLE IF NOT EXISTS job_results (
            job_id TEXT,
            meta_json TEXT,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        );`,
		`CREATE TABLE IF NOT EXISTS image_groups (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            job_id TEXT,
            group_type TEXT,
            detection_method TEXT,
            base_path TEXT,
            image_count INTEGER,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        );`,
		`CREATE TABLE IF NOT EXISTS image_metadata (
            file_path TEXT PRIMARY KEY,
            camera_make TEXT,
            camera_model TEXT,
            focal_length REAL,
            aperture REAL,
            iso INTEGER,
            exposure_time TEXT,
            gps_lat REAL,
            gps_lon REAL,
            timestamp TEXT,
            width INTEGER,
            height INTEGER
        );`,
		`CREATE TABLE IF NOT EXISTS photo_events (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            file_path TEXT NOT NULL,
            event_type TEXT NOT NULL,
            event_time TIMESTAMP NOT NULL,
            file_size INTEGER,
            darktable_id INTEGER,
            is_in_darktable BOOLEAN DEFAULT FALSE,
            is_processed BOOLEAN DEFAULT FALSE,
            is_exported BOOLEAN DEFAULT FALSE,
            event_data TEXT,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        );`,
		`CREATE INDEX IF NOT EXISTS idx_photo_events_file_path ON photo_events(file_path);`,
		`CREATE INDEX IF NOT EXISTS idx_photo_events_event_type ON photo_events(event_type);`,
		`CREATE INDEX IF NOT EXISTS idx_photo_events_darktable_id ON photo_events(darktable_id);`,
	}
	for _, stmt := range stmts {
		if _, err := s.DB.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

// Close closes the underlying DB.
func (s *Store) Close() error {
	if s == nil || s.DB == nil {
		return nil
	}
	return s.DB.Close()
}

// JobRecord captures persisted job info.
type JobRecord struct {
	ID          string
	JobType     string
	Status      string
	InputPath   string
	OutputPath  string
	OptionsJSON string
	Error       string
	CreatedAt   time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
}

// ImageGroupRecord captures persisted grouping info.
type ImageGroupRecord struct {
	JobID           string
	GroupType       string
	DetectionMethod string
	BasePath        string
	ImageCount      int
}

// ImageMetadata captures basic EXIF/GPS info.
type ImageMetadata struct {
	FilePath     string
	CameraMake   string
	CameraModel  string
	FocalLength  float64
	Aperture     float64
	ISO          int
	ExposureTime string
	GPSLat       float64
	GPSLon       float64
	Timestamp    string
	Width        int
	Height       int
}

// RecordJobQueued inserts a pending job.
func (s *Store) RecordJobQueued(rec JobRecord) error {
	if s == nil {
		return nil
	}
	_, err := s.DB.Exec(`INSERT OR REPLACE INTO processing_jobs (id, job_type, status, input_path, output_path, options_json) VALUES (?, ?, ?, ?, ?, ?);`,
		rec.ID, rec.JobType, rec.Status, rec.InputPath, rec.OutputPath, rec.OptionsJSON)
	return err
}

// RecordJobStart marks a job as running.
func (s *Store) RecordJobStart(id string) error {
	if s == nil {
		return nil
	}
	_, err := s.DB.Exec(`UPDATE processing_jobs SET status='running', started_at=CURRENT_TIMESTAMP WHERE id=?;`, id)
	return err
}

// RecordJobResult finalizes a job with status and meta.
func (s *Store) RecordJobResult(id string, status string, meta map[string]any, errMsg string) error {
	if s == nil {
		return nil
	}
	metaJSON, _ := json.Marshal(meta)
	_, err := s.DB.Exec(`UPDATE processing_jobs SET status=?, completed_at=CURRENT_TIMESTAMP, error_message=? WHERE id=?;`, status, errMsg, id)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(`INSERT INTO job_results (job_id, meta_json) VALUES (?, ?);`, id, string(metaJSON))
	return err
}

// RecentJobs returns the latest jobs up to limit.
func (s *Store) RecentJobs(limit int) ([]JobRecord, error) {
	if s == nil {
		return nil, errors.New("store not initialized")
	}
	rows, err := s.DB.Query(`SELECT id, job_type, status, input_path, output_path, options_json, created_at, started_at, completed_at, error_message FROM processing_jobs ORDER BY created_at DESC LIMIT ?;`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var recs []JobRecord
	for rows.Next() {
		var rec JobRecord
		var created time.Time
		var started, completed sql.NullTime
		var errorMsg sql.NullString
		if err := rows.Scan(&rec.ID, &rec.JobType, &rec.Status, &rec.InputPath, &rec.OutputPath, &rec.OptionsJSON, &created, &started, &completed, &errorMsg); err != nil {
			return nil, err
		}
		rec.CreatedAt = created
		if started.Valid {
			rec.StartedAt = &started.Time
		}
		if completed.Valid {
			rec.CompletedAt = &completed.Time
		}
		if errorMsg.Valid {
			rec.Error = errorMsg.String
		}
		recs = append(recs, rec)
	}
	return recs, nil
}

// JobMeta fetches the last meta blob for a job.
func (s *Store) JobMeta(id string) (map[string]any, error) {
	if s == nil {
		return nil, errors.New("store not initialized")
	}
	var metaJSON string
	err := s.DB.QueryRow(`SELECT meta_json FROM job_results WHERE job_id=? ORDER BY created_at DESC LIMIT 1;`, id).Scan(&metaJSON)
	if err != nil {
		return nil, err
	}
	var meta map[string]any
	if err := json.Unmarshal([]byte(metaJSON), &meta); err != nil {
		return nil, fmt.Errorf("unmarshal meta: %w", err)
	}
	return meta, nil
}

// RecordGroup persists a discovered image group.
func (s *Store) RecordGroup(rec ImageGroupRecord) error {
	if s == nil {
		return nil
	}
	_, err := s.DB.Exec(`INSERT INTO image_groups (job_id, group_type, detection_method, base_path, image_count) VALUES (?, ?, ?, ?, ?);`,
		rec.JobID, rec.GroupType, rec.DetectionMethod, rec.BasePath, rec.ImageCount)
	return err
}

// RecordImageMetadata stores EXIF/GPS details if available.
func (s *Store) RecordImageMetadata(meta ImageMetadata) error {
	if s == nil {
		return nil
	}
	_, err := s.DB.Exec(`INSERT OR REPLACE INTO image_metadata (file_path, camera_make, camera_model, focal_length, aperture, iso, exposure_time, gps_lat, gps_lon, timestamp, width, height)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`,
		meta.FilePath, meta.CameraMake, meta.CameraModel, meta.FocalLength, meta.Aperture, meta.ISO, meta.ExposureTime, meta.GPSLat, meta.GPSLon, meta.Timestamp, meta.Width, meta.Height)
	return err
}
