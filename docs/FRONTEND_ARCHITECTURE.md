# 🎨 **Photonic Frontend Architecture Guide**

## **Overview**

This document outlines two comprehensive frontend approaches for the Photonic photo management system: a **Web-based Frontend** for browser access and a **Native GTK+3 Desktop Application** for Linux desktop integration.

Both frontends will interface with our **FastAPI-style Go server** that provides:
- RESTful JSON APIs
- Real-time SSE (Server-Sent Events) streams  
- Photo processing job management
- Darktable integration monitoring

---

## 🌐 **Web Frontend Architecture**

### **Technology Stack: Gin + Templ + HTMX**

**Why This Stack:**
- **Gin**: Laravel-style routing and middleware in Go
- **Templ**: Type-safe HTML templating (compile-time checked)
- **HTMX**: Dynamic interactions without heavy JavaScript
- **Alpine.js**: Lightweight reactivity for complex components
- **Tailwind CSS**: Utility-first styling for rapid development

### **Project Structure**
```
web/
├── handlers/                    # Laravel-style controllers
│   ├── dashboard_handler.go
│   ├── photos_handler.go
│   ├── processing_handler.go
│   └── darktable_handler.go
├── templates/                   # Templ template files
│   ├── layouts/
│   │   ├── base.templ
│   │   └── dashboard.templ
│   ├── components/
│   │   ├── photo_card.templ
│   │   ├── progress_bar.templ
│   │   └── event_stream.templ
│   └── pages/
│       ├── dashboard.templ
│       ├── photo_library.templ
│       ├── processing_queue.templ
│       └── darktable_monitor.templ
├── static/
│   ├── css/
│   │   └── app.css            # Tailwind build output
│   ├── js/
│   │   ├── app.js             # Alpine.js components
│   │   └── sse-client.js      # Server-Sent Events handler
│   └── images/
├── middleware/
│   ├── auth.go
│   ├── cors.go
│   └── logging.go
└── routes/
    └── web.go                 # Route definitions
```

### **Core Web Components**

#### **1. Dashboard Overview**
```go
// handlers/dashboard_handler.go
func (h *DashboardHandler) Index(c *gin.Context) {
    stats := h.getDashboardStats()
    recentEvents := h.getRecentEvents(20)
    
    component := templates.Dashboard(stats, recentEvents)
    c.HTML(http.StatusOK, "", component)
}
```

```html
<!-- templates/pages/dashboard.templ -->
@templ Dashboard(stats DashboardStats, events []Event) {
    @layouts.Base("Dashboard") {
        <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
            @components.StatsCards(stats)
            @components.RecentActivity(events)
            @components.ProcessingQueue()
        </div>
        
        <!-- Real-time updates via HTMX + SSE -->
        <div hx-ext="sse" 
             sse-connect="/photo-events/stream"
             sse-swap="event_update"
             hx-swap="beforeend">
        </div>
    }
}
```

#### **2. Photo Library Browser**
```go
// Advanced photo grid with infinite scroll
func (h *PhotoHandler) Library(c *gin.Context) {
    page := c.DefaultQuery("page", "1")
    filter := c.Query("filter")
    
    photos := h.getPhotos(page, filter)
    
    if isHTMXRequest(c) {
        // Return only photo cards for infinite scroll
        component := templates.PhotoGrid(photos)
        c.HTML(http.StatusOK, "", component)
        return
    }
    
    // Full page render
    component := templates.PhotoLibrary(photos)
    c.HTML(http.StatusOK, "", component)
}
```

#### **3. Real-time Processing Monitor**
```html
<!-- Live processing updates -->
<div id="processing-monitor" 
     hx-ext="sse" 
     sse-connect="/jobs/stream">
     
    @for job := range processingJobs {
        @components.JobCard(job)
    }
</div>
```

#### **4. Darktable Integration Panel**
```go
// Real-time darktable activity monitoring
func (h *DarktableHandler) Monitor(c *gin.Context) {
    stats := h.darktableService.GetStats()
    recentEdits := h.darktableService.GetRecentEdits(50)
    
    component := templates.DarktableMonitor(stats, recentEdits)
    c.HTML(http.StatusOK, "", component)
}
```

### **Advanced Web Features**

#### **Photo Management Interface**
- **Smart Grid Layout**: Masonry-style responsive photo grid
- **Advanced Filtering**: Date, camera, processing status, darktable tags
- **Bulk Operations**: Batch processing, tagging, export
- **Drag & Drop**: File uploads with progress tracking
- **Keyboard Navigation**: Vim-style shortcuts for power users

#### **Real-time Updates**
```javascript
// sse-client.js - Enhanced SSE handling
class PhotonicSSE {
    constructor() {
        this.connections = new Map();
    }
    
    connect(endpoint, handlers) {
        const source = new EventSource(endpoint);
        
        source.addEventListener('photo_event', (e) => {
            const event = JSON.parse(e.data);
            this.updatePhotoStatus(event);
            this.showNotification(event);
        });
        
        source.addEventListener('job_progress', (e) => {
            const progress = JSON.parse(e.data);
            this.updateProgressBar(progress);
        });
        
        this.connections.set(endpoint, source);
    }
}
```

#### **Interactive Photo Viewer**
- **Lightbox Gallery**: Full-screen photo viewing with metadata overlay
- **Zoom & Pan**: High-resolution image navigation
- **Comparison Mode**: Before/after editing comparisons
- **EXIF Display**: Camera settings, GPS data, processing history

---

## 🖥️ **GTK+3 Native Desktop Application**

### **Technology Stack: gotk3 + Gio + Custom Widgets**

**Why GTK+3:**
- **Native Linux Integration**: System notifications, file associations
- **Performance**: Direct memory access, no browser overhead  
- **Professional UI**: Consistent with Linux desktop environments
- **Custom Widgets**: Specialized photo management controls

### **Application Architecture**
```
desktop/
├── main.go                     # Application entry point
├── app/
│   ├── application.go          # GTK Application setup
│   ├── window.go              # Main window management
│   └── config.go              # Settings and preferences
├── widgets/                    # Custom GTK widgets
│   ├── photo_grid.go          # High-performance photo grid
│   ├── thumbnail_view.go      # Async thumbnail loading
│   ├── metadata_panel.go      # EXIF and darktable data display
│   ├── processing_monitor.go  # Real-time job progress
│   └── timeline_view.go       # Chronological photo browser
├── services/
│   ├── photo_service.go       # Photo management logic
│   ├── darktable_service.go   # Darktable integration
│   ├── processing_service.go  # Job queue management
│   └── thumbnail_service.go   # Image processing
├── dialogs/
│   ├── preferences.go         # Settings dialog
│   ├── batch_process.go       # Batch operation wizard
│   └── export.go             # Export configuration
└── resources/
    ├── ui/
    │   ├── main_window.glade  # Glade UI files
    │   ├── preferences.glade
    │   └── batch_process.glade
    ├── icons/
    └── styles/
        └── photonic.css       # GTK CSS styling
```

### **Core Desktop Components**

#### **1. Main Application Window**
```go
// app/application.go
type PhotonicApp struct {
    app         *gtk.Application
    mainWindow  *MainWindow
    photoStore  *PhotoStore
    processor   *ProcessingService
}

func NewPhotonicApp() (*PhotonicApp, error) {
    app, err := gtk.ApplicationNew("com.photonic.app", glib.APPLICATION_FLAGS_NONE)
    if err != nil {
        return nil, err
    }
    
    photonicApp := &PhotonicApp{
        app: app,
    }
    
    app.Connect("activate", photonicApp.onActivate)
    app.Connect("startup", photonicApp.onStartup)
    
    return photonicApp, nil
}

func (app *PhotonicApp) onActivate() {
    window := NewMainWindow(app)
    window.Present()
}
```

#### **2. High-Performance Photo Grid**
```go
// widgets/photo_grid.go
type PhotoGrid struct {
    *gtk.FlowBox
    photos        []Photo
    thumbnails    *ThumbnailCache
    loadQueue     chan ThumbnailRequest
    visibleRange  Range
}

func NewPhotoGrid() *PhotoGrid {
    flowBox, _ := gtk.FlowBoxNew()
    
    grid := &PhotoGrid{
        FlowBox:   flowBox,
        loadQueue: make(chan ThumbnailRequest, 100),
    }
    
    // Configure for performance
    flowBox.SetSelectionMode(gtk.SELECTION_MULTIPLE)
    flowBox.SetRowSpacing(4)
    flowBox.SetColumnSpacing(4)
    flowBox.SetMaxChildrenPerLine(10)
    
    // Async thumbnail loading
    go grid.thumbnailWorker()
    
    // Virtual scrolling for large collections
    grid.setupVirtualScrolling()
    
    return grid
}

// Efficient loading of only visible thumbnails
func (pg *PhotoGrid) thumbnailWorker() {
    for req := range pg.loadQueue {
        if pg.isInViewport(req.Index) {
            thumbnail := pg.thumbnails.Get(req.Photo.Path)
            pg.updateThumbnail(req.Index, thumbnail)
        }
    }
}
```

#### **3. Real-time Processing Monitor**
```go
// widgets/processing_monitor.go
type ProcessingMonitor struct {
    *gtk.Box
    jobList     *gtk.ListBox
    progressBar *gtk.ProgressBar
    statusLabel *gtk.Label
    sseClient   *SSEClient
}

func (pm *ProcessingMonitor) connectToServer() {
    pm.sseClient.Connect("http://localhost:8080/jobs/stream", map[string]func([]byte){
        "job_started": pm.onJobStarted,
        "job_progress": pm.onJobProgress,
        "job_completed": pm.onJobCompleted,
    })
}

func (pm *ProcessingMonitor) onJobProgress(data []byte) {
    var progress JobProgress
    json.Unmarshal(data, &progress)
    
    // Update UI on main thread
    glib.IdleAdd(func() {
        pm.progressBar.SetFraction(progress.Percentage / 100.0)
        pm.statusLabel.SetText(progress.Status)
    })
}
```

#### **4. Advanced Metadata Panel**
```go
// widgets/metadata_panel.go
type MetadataPanel struct {
    *gtk.ScrolledWindow
    container     *gtk.Box
    exifTree      *gtk.TreeView
    darktableInfo *gtk.Grid
    historyView   *gtk.TextView
}

func (mp *MetadataPanel) DisplayPhoto(photo Photo) {
    // EXIF data in tree view
    mp.populateEXIF(photo.EXIF)
    
    // Darktable processing info
    if photo.DarktableMetadata != nil {
        mp.showDarktableInfo(photo.DarktableMetadata)
    }
    
    // Processing history
    mp.showProcessingHistory(photo.ProcessingHistory)
}
```

### **Advanced Desktop Features**

#### **Performance Optimizations**
- **Virtual Scrolling**: Only render visible photos in large collections
- **Async Thumbnail Generation**: Background image processing
- **Memory Management**: Intelligent image cache with LRU eviction
- **GPU Acceleration**: OpenGL-accelerated image rendering where available

#### **Native Integration**
```go
// Desktop notifications
func (app *PhotonicApp) showNotification(title, message string) {
    notification := gio.NotificationNew(title)
    notification.SetBody(message)
    notification.SetIcon(app.appIcon)
    app.app.SendNotification("photonic", notification)
}

// File associations
func (app *PhotonicApp) registerFileAssociations() {
    // Register as default handler for RAW files
    // Integrate with nautilus/file managers
}

// System tray integration
func (app *PhotonicApp) setupSystemTray() {
    // Background processing indicator
    // Quick access to common functions
}
```

#### **Specialized Photo Widgets**
- **Timeline Browser**: Chronological photo navigation with date scrubber
- **Comparison View**: Side-by-side before/after editing
- **Batch Processing Wizard**: Step-by-step bulk operation interface
- **Collection Manager**: Darktable collection browser with live updates

---

## 🚀 **Implementation Roadmap**

### **Phase 1: Web Frontend Foundation** (Week 1-2)
1. **Setup Gin + Templ + HTMX stack**
2. **Basic dashboard with photo grid**
3. **Real-time SSE integration**
4. **Photo upload and basic metadata display**

### **Phase 2: Advanced Web Features** (Week 3-4)
1. **Advanced photo filtering and search**
2. **Batch processing interface**
3. **Darktable integration panel**
4. **Responsive design and mobile support**

### **Phase 3: GTK+3 Desktop App** (Week 5-8)
1. **Basic GTK application structure**
2. **High-performance photo grid widget**
3. **Native file system integration**
4. **Real-time processing monitor**

### **Phase 4: Advanced Desktop Features** (Week 9-12)
1. **Custom photo management widgets**
2. **Performance optimizations**
3. **System integration features**
4. **Cross-platform compatibility testing**

---

## 📊 **Comparison Matrix**

| Feature | Web Frontend | Desktop GTK+3 |
|---------|-------------|---------------|
| **Deployment** | Browser-based, any OS | Native Linux binary |
| **Performance** | Good (DOM limitations) | Excellent (native) |
| **UI Flexibility** | High (CSS/JS) | Medium (GTK themes) |
| **System Integration** | Limited | Full (notifications, files) |
| **Development Speed** | Fast (web standards) | Medium (GTK learning) |
| **Maintenance** | Easy (web updates) | Medium (binary distribution) |
| **User Experience** | Familiar web UX | Native desktop UX |
| **Resource Usage** | Medium (browser) | Low (native) |

---

## 🎯 **Recommendation**

**Start with the Web Frontend** for rapid prototyping and user feedback, then **develop the GTK+3 Desktop App** for power users who need maximum performance and native integration.

**Shared Architecture Benefits:**
- Same Go backend serves both frontends
- Consistent API design
- Real-time features work in both environments
- Darktable integration benefits both approaches

This dual approach gives us **maximum reach** (web) and **maximum performance** (native), letting users choose their preferred interface while maintaining a unified photo management experience! 🎨📸