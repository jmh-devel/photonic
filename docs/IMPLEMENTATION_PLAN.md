# Photonic Server Enhancement - Implementation Plan

## 🎯 Immediate Next Steps (Ready to Implement)

### **Phase 1: Enhanced Database Schema (Week 1)**

#### **Step 1.1: Create Database Migration System**
```go
// internal/storage/migrations.go
type Migration struct {
    Version     int
    Description string
    SQL         string
}

func (s *Store) RunMigrations() error {
    // Check current schema version
    // Run pending migrations in order
    // Update schema version tracking
}
```

#### **Step 1.2: Add Photos Metadata Table**
```sql
-- Migration 001: Add photos table
CREATE TABLE photos (
    id TEXT PRIMARY KEY,
    file_path TEXT UNIQUE NOT NULL,
    file_hash TEXT UNIQUE NOT NULL,
    filename TEXT NOT NULL,
    file_size INTEGER NOT NULL,
    format TEXT NOT NULL,
    camera_make TEXT,
    camera_model TEXT,
    star_rating INTEGER DEFAULT 0,
    import_timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    capture_timestamp TIMESTAMP
);

CREATE INDEX idx_photos_path ON photos(file_path);
CREATE INDEX idx_photos_camera ON photos(camera_make, camera_model);
```

#### **Step 1.3: Add Watched Folders Table**
```sql
-- Migration 002: Add watched folders
CREATE TABLE watched_folders (
    id TEXT PRIMARY KEY,
    path TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    auto_import BOOLEAN DEFAULT TRUE,
    status TEXT DEFAULT 'active',
    last_scan TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### **Phase 2: Basic HTTP API Extensions (Week 1-2)**

#### **Step 2.1: Add Photo Listing Endpoint**
```go
// internal/server/handlers.go
func (h *Handlers) ListPhotos(w http.ResponseWriter, r *http.Request) {
    limit := getIntParam(r, "limit", 50)
    offset := getIntParam(r, "offset", 0)
    
    photos, err := h.store.ListPhotos(limit, offset)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]any{
        "photos": photos,
        "total":  len(photos),
        "limit":  limit,
        "offset": offset,
    })
}
```

#### **Step 2.2: Add Directory Scanning Endpoint**
```go
func (h *Handlers) ScanDirectory(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Path      string   `json:"path"`
        Recursive bool     `json:"recursive"`
        FileFilter []string `json:"file_filter"`
        DryRun    bool     `json:"dry_run"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    
    result, err := h.scanner.ScanDirectory(req.Path, req)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(result)
}
```

### **Phase 3: Directory Scanner Service (Week 2)**

#### **Step 3.1: Create PhotoScanner Service**
```go
// internal/library/scanner.go
type PhotoScanner struct {
    store *storage.Store
    log   *slog.Logger
}

type ScanResult struct {
    TotalFiles     int           `json:"total_files"`
    NewFiles       int           `json:"new_files"`
    ExistingFiles  int           `json:"existing_files"`
    ErrorFiles     []ScanError   `json:"error_files"`
    ScanTime       time.Duration `json:"scan_time"`
}

type ScanError struct {
    FilePath string `json:"file_path"`
    Error    string `json:"error"`
}

func (ps *PhotoScanner) ScanDirectory(path string, opts ScanOptions) (*ScanResult, error) {
    start := time.Now()
    result := &ScanResult{}
    
    err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
        if err != nil {
            result.ErrorFiles = append(result.ErrorFiles, ScanError{
                FilePath: filePath, 
                Error: err.Error(),
            })
            return nil // Continue walking
        }
        
        if !ps.isPhotoFile(filePath) {
            return nil
        }
        
        result.TotalFiles++
        
        // Check if already exists in database
        exists, err := ps.store.PhotoExists(filePath)
        if err != nil {
            result.ErrorFiles = append(result.ErrorFiles, ScanError{
                FilePath: filePath,
                Error: fmt.Sprintf("Database check failed: %v", err),
            })
            return nil
        }
        
        if exists {
            result.ExistingFiles++
        } else {
            result.NewFiles++
            
            if !opts.DryRun {
                if err := ps.importPhoto(filePath); err != nil {
                    result.ErrorFiles = append(result.ErrorFiles, ScanError{
                        FilePath: filePath,
                        Error: fmt.Sprintf("Import failed: %v", err),
                    })
                }
            }
        }
        
        return nil
    })
    
    result.ScanTime = time.Since(start)
    return result, err
}
```

#### **Step 3.2: Add Basic Metadata Extraction**
```go
func (ps *PhotoScanner) importPhoto(filePath string) error {
    // Calculate file hash for duplicate detection
    hash, err := ps.calculateFileHash(filePath)
    if err != nil {
        return fmt.Errorf("hash calculation failed: %w", err)
    }
    
    // Get file info
    stat, err := os.Stat(filePath)
    if err != nil {
        return fmt.Errorf("file stat failed: %w", err)
    }
    
    // Basic metadata extraction (can be enhanced with ExifTool later)
    photo := &storage.Photo{
        ID:           generatePhotoID(filePath),
        FilePath:     filePath,
        FileHash:     hash,
        Filename:     filepath.Base(filePath),
        FileSize:     stat.Size(),
        Format:       strings.ToUpper(strings.TrimPrefix(filepath.Ext(filePath), ".")),
        ImportTime:   time.Now(),
    }
    
    return ps.store.InsertPhoto(photo)
}
```

### **Phase 4: File System Watcher (Week 2-3)**

#### **Step 4.1: Basic Folder Watcher**
```go
// internal/library/watcher.go
import "github.com/fsnotify/fsnotify"

type FolderWatcher struct {
    watcher    *fsnotify.Watcher
    scanner    *PhotoScanner
    watched    map[string]*WatchConfig
    importChan chan string
    stopChan   chan struct{}
    log        *slog.Logger
}

type WatchConfig struct {
    Path       string
    Name       string
    AutoImport bool
    Recursive  bool
}

func NewFolderWatcher(scanner *PhotoScanner, log *slog.Logger) (*FolderWatcher, error) {
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        return nil, err
    }
    
    fw := &FolderWatcher{
        watcher:    watcher,
        scanner:    scanner,
        watched:    make(map[string]*WatchConfig),
        importChan: make(chan string, 100),
        stopChan:   make(chan struct{}),
        log:        log,
    }
    
    go fw.processEvents()
    go fw.processImports()
    
    return fw, nil
}

func (fw *FolderWatcher) AddWatchFolder(config WatchConfig) error {
    fw.watched[config.Path] = &config
    
    if config.Recursive {
        return filepath.Walk(config.Path, func(path string, info os.FileInfo, err error) error {
            if err != nil {
                return err
            }
            if info.IsDir() {
                return fw.watcher.Add(path)
            }
            return nil
        })
    } else {
        return fw.watcher.Add(config.Path)
    }
}

func (fw *FolderWatcher) processEvents() {
    for {
        select {
        case event := <-fw.watcher.Events:
            if event.Op&fsnotify.Create == fsnotify.Create {
                if fw.isPhotoFile(event.Name) {
                    fw.log.Info("New photo detected", "file", event.Name)
                    fw.importChan <- event.Name
                }
            }
        case err := <-fw.watcher.Errors:
            fw.log.Error("Watcher error", "error", err)
        case <-fw.stopChan:
            return
        }
    }
}
```

### **Phase 5: Enhanced Server Setup (Week 3)**

#### **Step 5.1: Update Server with New Services**
```go
// internal/server/server.go - Enhanced version
type Server struct {
    store    *storage.Store
    pipeline *pipeline.Pipeline  
    scanner  *library.PhotoScanner
    watcher  *library.FolderWatcher
    log      *slog.Logger
}

func NewServer(store *storage.Store, pipeline *pipeline.Pipeline, log *slog.Logger) (*Server, error) {
    scanner := library.NewPhotoScanner(store, log)
    watcher, err := library.NewFolderWatcher(scanner, log)
    if err != nil {
        return nil, err
    }
    
    return &Server{
        store:   store,
        pipeline: pipeline,
        scanner: scanner,
        watcher: watcher,
        log:     log,
    }, nil
}

func (s *Server) setupRoutes() *http.ServeMux {
    mux := http.NewServeMux()
    
    // Existing endpoints
    mux.HandleFunc("/healthz", s.handleHealth)
    mux.HandleFunc("/jobs", s.handleJobs)
    mux.HandleFunc("/stream", s.handleStream)
    
    // New photo management endpoints
    mux.HandleFunc("/api/photos", s.handlePhotos)
    mux.HandleFunc("/api/photos/import", s.handleImport)
    mux.HandleFunc("/api/scan", s.handleScan)
    mux.HandleFunc("/api/watch", s.handleWatch)
    
    return mux
}
```

## 🧪 Testing Strategy

### **Manual Testing Commands**
```bash
# Test photo scanning
curl -X POST http://localhost:8080/api/scan \
  -H "Content-Type: application/json" \
  -d '{"path": "/path/to/photos", "recursive": true, "dry_run": true}'

# Test photo listing
curl http://localhost:8080/api/photos?limit=10&offset=0

# Test folder watching
curl -X POST http://localhost:8080/api/watch \
  -H "Content-Type: application/json" \
  -d '{"path": "/watch/folder", "name": "Test Watch", "auto_import": true}'
```

### **Integration Testing**
```go
// internal/server/server_test.go
func TestPhotoScanning(t *testing.T) {
    // Create test server with in-memory database
    // Create temporary photo directory with test files
    // Call scan endpoint
    // Verify photos were added to database
}

func TestFolderWatching(t *testing.T) {
    // Setup test watcher
    // Add watch folder
    // Copy new photo to watched folder  
    // Verify photo was automatically imported
}
```

## 🚀 Ready to Implement

This plan provides a clear, incremental path to transform the existing server into a photo library management system. Each phase builds on the previous one and can be implemented and tested independently.

**Start with Phase 1** - the database enhancements are the foundation for everything else and can be implemented immediately without breaking existing functionality.