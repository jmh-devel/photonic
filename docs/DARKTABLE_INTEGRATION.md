# 🎯 **Darktable + Filesystem Integration Strategy**

## **The Dual Data Stream Architecture**

Your question about **watching folders vs. pulling darktable data** opens up a powerful **dual-stream monitoring system** that gives us the best of both worlds:

### **🗂️ Stream 1: Filesystem Watching** 
- **Real-time file events**: new photos, moves, deletions
- **Immediate detection**: catches imports before darktable processes them
- **File metadata**: size, timestamps, location changes
- **Raw file events**: detects when new photos arrive

### **🎨 Stream 2: Darktable Database** 
- **Processing status**: which photos have been edited
- **User activity**: ratings, color labels, tags
- **Export tracking**: when photos are exported
- **Edit history**: changes over time

## **🔄 How The Integration Works**

```
┌─────────────────┐    ┌─────────────────┐
│   Filesystem    │    │   Darktable     │
│     Events      │    │   Database      │
│                 │    │                 │
│ • New files     │    │ • Edit status   │
│ • Moves/renames │    │ • Ratings       │
│ • Deletions     │    │ • Tags/labels   │
│ • Size changes  │    │ • Export times  │
└─────────┬───────┘    └─────────┬───────┘
          │                      │
          └──────────┬───────────┘
                     │
            ┌────────▼────────┐
            │  DUAL WATCHER   │
            │                 │
            │ Correlates &    │
            │ Enriches Events │
            └────────┬────────┘
                     │
          ┌──────────▼──────────┐
          │   ENRICHED EVENTS   │
          │                     │
          │ File + DT metadata  │
          │ Processing status   │
          │ User activity       │
          └─────────────────────┘
```

## **📊 What You Get From This Integration**

### **Enhanced Photo Events**
Every filesystem event is enriched with darktable data:

```json
{
  "file_path": "/data/Photography/2024/11/29/IMG_1234.CR2",
  "event_type": "created",
  "event_time": "2024-11-29T16:15:30Z",
  "file_size": 25874193,
  
  "darktable_id": 12345,
  "is_in_darktable": true,
  "is_processed": false,
  "is_exported": false,
  
  "metadata": {
    "iso": 800,
    "aperture": 2.8,
    "focal_length": 85,
    "datetime_taken": "2024-11-29T14:30:45Z",
    "camera_make": "Canon",
    "camera_model": "EOS R5"
  }
}
```

### **Smart Detection Patterns**

**🆕 New Import Detection**
```
Filesystem: "new file created" 
+ Darktable: "not in database yet" 
= "Fresh import, needs processing"
```

**✏️ Edit Activity Detection**
```
Darktable: "history_end changed" 
+ Filesystem: "XMP file modified" 
= "User actively editing this photo"
```

**📤 Export Workflow Detection**
```
Darktable: "export_timestamp updated" 
+ Filesystem: "new JPEG created" 
= "Photo exported, ready for sharing"
```

## **🚀 Real-World Use Cases**

### **1. Smart Auto-Processing**
```go
// When new RAW files arrive that aren't in darktable yet
if event.IsNewImport && isRAWFile(event.FilePath) {
    // Auto-generate preview
    generatePreview(event.FilePath)
    
    // Add to processing queue
    scheduleRAWProcessing(event.FilePath)
}
```

### **2. Edit Progress Monitoring**
```go
// Track which photos are being actively worked on
if event.IsProcessed && event.EventType == "darktable_modified" {
    // Update progress dashboard
    updateEditingProgress(event.FilePath, event.Metadata.HistoryEnd)
    
    // Notify collaborators
    notifyTeam("Photo edited", event.FilePath)
}
```

### **3. Export Pipeline Automation**
```go
// Auto-process newly exported photos
if event.IsExported && event.EventType == "darktable_modified" {
    // Generate web versions
    createWebVersions(event.FilePath)
    
    // Upload to gallery
    uploadToGallery(event.FilePath, event.Metadata)
}
```

### **4. Library Health Monitoring**
```go
// Detect orphaned files or inconsistencies
if event.EventType == "deleted" && event.IsInDarktable {
    // Photo deleted but still in darktable database
    flagInconsistency("Orphaned database entry", event.DarktableID)
}
```

## **📡 API Endpoints Available**

### **Real-time Data**
```bash
# Live event stream (SSE)
curl -N http://localhost:8080/photo-events/stream

# Recent events with filtering
curl "http://localhost:8080/photo-events?since=2024-11-29T00:00:00Z&event_type=darktable_modified"
```

### **Darktable Insights**
```bash
# Library statistics
curl http://localhost:8080/darktable/stats
# Returns: total/edited counts, percentages, date ranges

# Recent edits
curl "http://localhost:8080/darktable/recent-edits?limit=20"
# Returns: recently modified photos with full metadata
```

### **Photo Details**
```bash
# Combined file + darktable information
curl "http://localhost:8080/photos/data/Photography/2024/11/29/IMG_1234.CR2/details"
```

## **⚡ Performance Benefits**

### **Read-Only Database Access**
- **No locking**: darktable can run simultaneously
- **No interference**: completely safe monitoring
- **Fast queries**: optimized for common patterns

### **Intelligent Polling**
- **30-second intervals**: catches changes quickly without spam
- **Change detection**: only processes actual modifications
- **Batch processing**: handles multiple changes efficiently

### **Event Correlation**
- **Immediate filesystem events**: instant response to file changes
- **Periodic darktable sync**: captures user activity
- **Combined intelligence**: filesystem + user intent

## **🎪 Your 11,000+ Photo Use Cases**

### **Batch Edit Monitoring**
Track which photos in a batch are complete:
```sql
-- Find unfinished edits in today's batch
SELECT file_path, darktable_id, is_processed 
FROM photo_events 
WHERE event_time > '2024-11-29' 
  AND is_in_darktable = true 
  AND is_processed = false;
```

### **Export Automation**
Auto-trigger downstream processing:
```sql
-- Find newly exported photos ready for web processing
SELECT file_path, metadata 
FROM photo_events 
WHERE event_type = 'darktable_modified' 
  AND is_exported = true 
  AND event_time > datetime('now', '-1 hour');
```

### **Library Analytics**
Understand your workflow patterns:
```sql
-- Editing velocity by day
SELECT date(event_time) as day, 
       count(*) as photos_edited 
FROM photo_events 
WHERE event_type = 'darktable_modified' 
  AND is_processed = true 
GROUP BY date(event_time) 
ORDER BY day DESC;
```

## **🎯 Next Steps**

1. **Start Integration**: Use the provided code to begin dual monitoring
2. **Customize Events**: Add specific triggers for your workflow needs  
3. **Build Dashboards**: Create real-time views of your photo activity
4. **Automate Workflows**: Set up processing chains based on events
5. **Scale Up**: Add more darktable libraries or watch additional paths

This gives you **unprecedented visibility** into both the technical (filesystem) and creative (darktable) sides of your photography workflow! 📸✨