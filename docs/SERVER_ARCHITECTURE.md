# Photonic Server Architecture

## 🏗️ Server Enhancement Plan

### Current Server Status
The existing server (`internal/server/server.go`) is a minimal HTTP server with:
- **Health check** endpoint (`/healthz`)
- **Job listing** endpoint (`/jobs`) - shows recent jobs from SQLite
- **Event stream** endpoint (`/stream`) - Server-Sent Events for real-time job updates

### Enhanced Server Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    PHOTONIC HTTP SERVER                    │
├─────────────────┬─────────────────┬─────────────────────────┤
│   HTTP ROUTES   │   MIDDLEWARES   │      SERVICES           │
├─────────────────┼─────────────────┼─────────────────────────┤
│ • /api/photos   │ • CORS Support  │ • LibraryManager        │
│ • /api/jobs     │ • Auth (future) │ • FileSystemWatcher     │
│ • /api/import   │ • Rate Limiting │ • ProcessingEngine      │
│ • /api/watch    │ • Request Log   │ • MetadataExtractor     │
│ • /stream       │ • Error Handler │ • DatabaseManager       │
│ • /dashboard    │ • JSON/Form     │ • BackgroundWorker      │
└─────────────────┴─────────────────┴─────────────────────────┘
```

## 📡 API Endpoints Design

### **Photo Management**
```go
// GET /api/photos - List photos with advanced filtering
type PhotoListRequest struct {
    Limit      int       `json:"limit" form:"limit"`           // Default: 50, Max: 1000
    Offset     int       `json:"offset" form:"offset"`         // Pagination
    Collection string    `json:"collection" form:"collection"` // Filter by collection
    Keywords   []string  `json:"keywords" form:"keywords"`     // Filter by keywords
    Rating     int       `json:"rating" form:"rating"`         // Minimum star rating
    DateFrom   time.Time `json:"date_from" form:"date_from"`   // Capture date range
    DateTo     time.Time `json:"date_to" form:"date_to"`       
    Camera     string    `json:"camera" form:"camera"`         // Camera model filter
    Search     string    `json:"search" form:"search"`         // Full-text search
    SortBy     string    `json:"sort_by" form:"sort_by"`       // date, rating, filename
    SortOrder  string    `json:"sort_order" form:"sort_order"` // asc, desc
}

type PhotoResponse struct {
    ID             string             `json:"id"`
    FilePath       string             `json:"file_path"`
    Filename       string             `json:"filename"`
    Format         string             `json:"format"`
    FileSize       int64              `json:"file_size"`
    Rating         int                `json:"rating"`
    Keywords       []string           `json:"keywords"`
    Collections    []string           `json:"collections"`
    CameraData     CameraMetadata     `json:"camera_data"`
    Location       *GPS               `json:"location,omitempty"`
    ProcessingData ProcessingMetadata `json:"processing"`
    ThumbnailURL   string             `json:"thumbnail_url"`
    PreviewURL     string             `json:"preview_url"`
    CreatedAt      time.Time          `json:"created_at"`
    UpdatedAt      time.Time          `json:"updated_at"`
}
```

### **Import Management**
```go
// POST /api/import/scan - Scan directory for new photos
type ImportScanRequest struct {
    Path        string          `json:"path"`
    Recursive   bool            `json:"recursive"`
    FileFilter  []string        `json:"file_filter"` // [".CR2", ".NEF", ".DNG"]
    ImportRules []ImportRule    `json:"import_rules,omitempty"`
    DryRun      bool            `json:"dry_run"`     // Preview without importing
}

type ImportScanResponse struct {
    TotalFiles     int                `json:"total_files"`
    NewFiles       int                `json:"new_files"`  
    DuplicateFiles int                `json:"duplicate_files"`
    ErrorFiles     []ImportError      `json:"error_files"`
    PreviewFiles   []ImportCandidate  `json:"preview_files,omitempty"`
    EstimatedTime  string             `json:"estimated_time"`
}

// POST /api/import/execute - Execute import operation
type ImportExecuteRequest struct {
    ScanID      string `json:"scan_id"`
    Confirm     bool   `json:"confirm"`
    Background  bool   `json:"background"`  // Queue as background job
}
```

### **Folder Watching**
```go
// POST /api/watch/add - Add folder to watch list
type WatchAddRequest struct {
    Path            string          `json:"path"`
    Name            string          `json:"name"`           // Human-readable name
    Recursive       bool            `json:"recursive"`
    AutoImport      bool            `json:"auto_import"`
    AutoProcess     bool            `json:"auto_process"`
    ProcessingRules []ProcessingRule `json:"processing_rules"`
    ImportRules     []ImportRule    `json:"import_rules"`
}

// GET /api/watch/status - Get all watched folders and their status
type WatchStatusResponse struct {
    WatchedFolders []WatchedFolder `json:"watched_folders"`
    TotalEvents    int             `json:"total_events"`
    ProcessingJobs int             `json:"processing_jobs"`
}

type WatchedFolder struct {
    ID              string    `json:"id"`
    Path            string    `json:"path"`
    Name            string    `json:"name"`
    Status          string    `json:"status"`  // active, paused, error
    LastScan        time.Time `json:"last_scan"`
    FilesProcessed  int       `json:"files_processed"`
    FilesQueued     int       `json:"files_queued"`
    ErrorCount      int       `json:"error_count"`
}
```

### **Processing Jobs & Queue**
```go
// GET /api/jobs - Enhanced job listing with filtering
type JobListRequest struct {
    Status    string    `form:"status"`    // queued, running, completed, failed
    JobType   string    `form:"job_type"`  // timelapse, panoramic, import, processing
    DateFrom  time.Time `form:"date_from"`
    DateTo    time.Time `form:"date_to"`
    Limit     int       `form:"limit"`
}

// POST /api/jobs/batch - Create batch processing job
type BatchJobRequest struct {
    PhotoIDs        []string        `json:"photo_ids"`
    JobType         string          `json:"job_type"`         // batch_process, batch_export
    ProcessingRules ProcessingRule  `json:"processing_rule"`
    OutputSettings  OutputSettings  `json:"output_settings"`
    Priority        int             `json:"priority"`         // 1-10, higher = more urgent
}

// POST /api/jobs/:id/cancel - Cancel running job
// POST /api/jobs/:id/retry - Retry failed job
// GET /api/jobs/:id/logs - Get detailed job logs
```

## 🗄️ Enhanced Database Schema

```sql
-- Enhanced Photos Table
CREATE TABLE photos (
    id TEXT PRIMARY KEY,
    file_path TEXT UNIQUE NOT NULL,
    file_hash TEXT UNIQUE NOT NULL,
    filename TEXT NOT NULL,
    file_size INTEGER NOT NULL,
    format TEXT NOT NULL,
    
    -- Camera & Technical Metadata
    camera_make TEXT,
    camera_model TEXT,
    lens_model TEXT,
    iso INTEGER,
    aperture REAL,
    shutter_speed TEXT,
    focal_length REAL,
    
    -- GPS & Location
    gps_lat REAL,
    gps_lng REAL,
    gps_altitude REAL,
    location_name TEXT,
    
    -- User Data
    star_rating INTEGER DEFAULT 0,
    color_label TEXT,
    title TEXT,
    description TEXT,
    
    -- Processing Status
    processing_status TEXT DEFAULT 'raw', -- raw, processed, error
    darktable_processed BOOLEAN DEFAULT FALSE,
    has_xmp_sidecar BOOLEAN DEFAULT FALSE,
    
    -- AI Analysis (future)
    content_tags JSON,
    quality_score REAL,
    similarity_hash TEXT,
    faces_detected INTEGER DEFAULT 0,
    
    -- Timestamps
    capture_timestamp TIMESTAMP,
    import_timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Indexes
    CREATE INDEX idx_photos_capture_date ON photos(capture_timestamp);
    CREATE INDEX idx_photos_camera ON photos(camera_make, camera_model);
    CREATE INDEX idx_photos_rating ON photos(star_rating);
    CREATE INDEX idx_photos_processing ON photos(processing_status);
);

-- Collections System
CREATE TABLE collections (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    parent_id TEXT REFERENCES collections(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    photo_count INTEGER DEFAULT 0
);

CREATE TABLE photo_collections (
    photo_id TEXT REFERENCES photos(id) ON DELETE CASCADE,
    collection_id TEXT REFERENCES collections(id) ON DELETE CASCADE,
    added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (photo_id, collection_id)
);

-- Keywords & Tagging
CREATE TABLE keywords (
    id TEXT PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    category TEXT,
    description TEXT,
    usage_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE photo_keywords (
    photo_id TEXT REFERENCES photos(id) ON DELETE CASCADE,
    keyword_id TEXT REFERENCES keywords(id) ON DELETE CASCADE,
    confidence REAL DEFAULT 1.0, -- For AI-generated tags
    added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (photo_id, keyword_id)
);

-- Watched Folders
CREATE TABLE watched_folders (
    id TEXT PRIMARY KEY,
    path TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    recursive BOOLEAN DEFAULT TRUE,
    auto_import BOOLEAN DEFAULT TRUE,
    auto_process BOOLEAN DEFAULT FALSE,
    rules_json TEXT, -- Processing and import rules
    status TEXT DEFAULT 'active', -- active, paused, error
    last_scan TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Processing Templates & Rules  
CREATE TABLE processing_templates (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    template_type TEXT NOT NULL, -- darktable_preset, batch_operation, ai_workflow
    template_data JSON NOT NULL, -- Template configuration
    auto_apply_rules JSON, -- Conditions for automatic application
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Enhanced Jobs Table
ALTER TABLE processing_jobs ADD COLUMN priority INTEGER DEFAULT 5;
ALTER TABLE processing_jobs ADD COLUMN parent_job_id TEXT;
ALTER TABLE processing_jobs ADD COLUMN batch_id TEXT;
ALTER TABLE processing_jobs ADD COLUMN progress_percent INTEGER DEFAULT 0;
ALTER TABLE processing_jobs ADD COLUMN estimated_completion TIMESTAMP;
```

## 🔧 Service Architecture

### **LibraryManager Service**
```go
type LibraryManager struct {
    db             *storage.Store
    watchers       map[string]*fsnotify.Watcher
    importQueue    chan ImportJob
    metadataCache  *lru.Cache
    thumbnailCache *lru.Cache
}

func (lm *LibraryManager) ScanDirectory(path string, opts ScanOptions) (*ScanResult, error)
func (lm *LibraryManager) ImportPhotos(candidates []ImportCandidate) (*ImportResult, error)
func (lm *LibraryManager) AddWatchFolder(config WatchConfig) error
func (lm *LibraryManager) RemoveWatchFolder(id string) error
func (lm *LibraryManager) GetPhotoMetadata(id string) (*PhotoMetadata, error)
func (lm *LibraryManager) UpdatePhotoRating(id string, rating int) error
func (lm *LibraryManager) SearchPhotos(query SearchQuery) (*PhotoSearchResult, error)
```

### **BackgroundWorker Service**
```go
type BackgroundWorker struct {
    queues     map[string]*PriorityQueue
    workers    []*Worker
    jobStore   *storage.Store
    pipeline   *pipeline.Pipeline
}

type PriorityQueue struct {
    High   chan Job
    Medium chan Job  
    Low    chan Job
}

func (bw *BackgroundWorker) QueueJob(job Job, priority Priority) error
func (bw *BackgroundWorker) CancelJob(jobID string) error
func (bw *BackgroundWorker) GetJobStatus(jobID string) (*JobStatus, error)
func (bw *BackgroundWorker) StartWorkers(concurrency int) error
```

## 🚀 Implementation Steps

### **Step 1: Database Schema Enhancement**
1. Create migration system for database schema updates
2. Add new tables for photos, collections, keywords, watched folders
3. Migrate existing job data to new schema
4. Create database indexes for performance

### **Step 2: HTTP Server Enhancement**  
1. Implement new API endpoints with proper validation
2. Add middleware for CORS, logging, error handling
3. Create thumbnail generation and serving
4. Implement Server-Sent Events for real-time updates

### **Step 3: Library Management Services**
1. Build LibraryManager with photo discovery and import
2. Implement file system watchers for automatic monitoring
3. Create metadata extraction pipeline with ExifTool integration
4. Build search and filtering system with full-text search

### **Step 4: Background Processing System**
1. Enhanced job queue with priority levels
2. Background worker pool with configurable concurrency
3. Job cancellation and retry mechanisms
4. Progress tracking and ETA calculation

### **Step 5: Web Dashboard (Future)**
1. React-based web interface for library management
2. Photo grid with filtering and search
3. Drag-and-drop import interface
4. Real-time job monitoring dashboard

This architecture transforms Photonic from a CLI tool into a comprehensive photo library management system while maintaining backward compatibility with existing functionality.