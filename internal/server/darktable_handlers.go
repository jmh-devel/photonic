package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"photonic/internal/darktable"
	"photonic/internal/tasks"

	"github.com/gorilla/mux"
)

// DarktableStatsResponse represents darktable library statistics
type DarktableStatsResponse struct {
	TotalImages    int       `json:"total_images"`
	EditedImages   int       `json:"edited_images"`
	UneditedImages int       `json:"unedited_images"`
	FilmRolls      int       `json:"film_rolls"`
	EditPercentage float64   `json:"edit_percentage"`
	EarliestPhoto  time.Time `json:"earliest_photo,omitempty"`
	LatestPhoto    time.Time `json:"latest_photo,omitempty"`
	LastImport     time.Time `json:"last_import,omitempty"`
}

// EnrichedEventsResponse represents recent photo events with darktable correlation
type EnrichedEventsResponse struct {
	Events []tasks.EnrichedPhotoEvent `json:"events"`
	Count  int                        `json:"count"`
}

// RecentEditsResponse represents recently edited photos in darktable
type RecentEditsResponse struct {
	Photos []darktable.PhotoMetadata `json:"photos"`
	Count  int                       `json:"count"`
}

// setupDarktableRoutes adds darktable integration endpoints
func (s *Server) setupDarktableRoutes(r *mux.Router) {
	// Darktable statistics
	r.HandleFunc("/darktable/stats", s.handleDarktableStats).Methods("GET")

	// Recent darktable edits
	r.HandleFunc("/darktable/recent-edits", s.handleRecentEdits).Methods("GET")

	// Enriched photo events (filesystem + darktable)
	r.HandleFunc("/photo-events", s.handlePhotoEvents).Methods("GET")

	// SSE stream for real-time enriched events
	r.HandleFunc("/photo-events/stream", s.handlePhotoEventsStream).Methods("GET")

	// Photo details with darktable metadata
	r.HandleFunc("/photos/{path:.*}/details", s.handlePhotoDetails).Methods("GET")
}

// handleDarktableStats returns darktable library statistics
func (s *Server) handleDarktableStats(w http.ResponseWriter, r *http.Request) {
	if s.dualWatcher == nil {
		http.Error(w, "Darktable integration not available", http.StatusServiceUnavailable)
		return
	}

	stats, err := s.dualWatcher.GetDarktableStats()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get darktable stats: %v", err), http.StatusInternalServerError)
		return
	}

	response := DarktableStatsResponse{
		TotalImages:    stats["total_images"].(int),
		EditedImages:   stats["edited_images"].(int),
		UneditedImages: stats["unedited_images"].(int),
		FilmRolls:      stats["film_rolls"].(int),
		EditPercentage: stats["edit_percentage"].(float64),
	}

	// Add timestamp fields with proper darktable conversion
	if earliest, ok := stats["earliest_photo"].(time.Time); ok && !earliest.IsZero() {
		response.EarliestPhoto = earliest
	}
	if latest, ok := stats["latest_photo"].(time.Time); ok && !latest.IsZero() {
		response.LatestPhoto = latest
	}
	if lastImport, ok := stats["last_import"].(time.Time); ok && !lastImport.IsZero() {
		response.LastImport = lastImport
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		fmt.Printf("DEBUG: JSON encoding error: %v\n", err)
	}
}

// handleRecentEdits returns recently edited photos from darktable
func (s *Server) handleRecentEdits(w http.ResponseWriter, r *http.Request) {
	if s.dualWatcher == nil {
		http.Error(w, "Darktable integration not available", http.StatusServiceUnavailable)
		return
	}

	// Parse limit parameter
	limitStr := r.URL.Query().Get("limit")
	limit := 50 // default
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}

	photos, err := s.dualWatcher.GetRecentDarktableEdits(limit)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get recent edits: %v", err), http.StatusInternalServerError)
		return
	}

	response := RecentEditsResponse{
		Photos: photos,
		Count:  len(photos),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handlePhotoEvents returns enriched photo events
func (s *Server) handlePhotoEvents(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	sinceStr := r.URL.Query().Get("since")
	eventType := r.URL.Query().Get("event_type")

	limit := 100 // default
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}

	var since time.Time
	if sinceStr != "" {
		if parsed, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = parsed
		} else {
			http.Error(w, "Invalid since format, use RFC3339", http.StatusBadRequest)
			return
		}
	} else {
		since = time.Now().Add(-24 * time.Hour) // Default: last 24 hours
	}

	// Query photo events from database
	query := `
		SELECT file_path, event_type, event_time, file_size, 
			   darktable_id, is_in_darktable, is_processed, is_exported,
			   event_data
		FROM photo_events
		WHERE event_time >= ?
	`
	args := []interface{}{since}

	if eventType != "" {
		query += " AND event_type = ?"
		args = append(args, eventType)
	}

	query += " ORDER BY event_time DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.store.DB.Query(query, args...)
	if err != nil {
		http.Error(w, fmt.Sprintf("Database query failed: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var events []tasks.EnrichedPhotoEvent
	for rows.Next() {
		var event tasks.EnrichedPhotoEvent
		var darktableID *int
		var eventDataJSON string

		err := rows.Scan(
			&event.FilePath,
			&event.EventType,
			&event.EventTime,
			&event.FileSize,
			&darktableID,
			&event.IsInDarktable,
			&event.IsProcessed,
			&event.IsExported,
			&eventDataJSON,
		)
		if err != nil {
			continue // skip malformed rows
		}

		event.DarktableID = darktableID

		// Parse enriched metadata if available
		if eventDataJSON != "" {
			var enrichedEvent tasks.EnrichedPhotoEvent
			if err := json.Unmarshal([]byte(eventDataJSON), &enrichedEvent); err == nil {
				event.Metadata = enrichedEvent.Metadata
				event.IsNewImport = enrichedEvent.IsNewImport
			}
		}

		events = append(events, event)
	}

	response := EnrichedEventsResponse{
		Events: events,
		Count:  len(events),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handlePhotoEventsStream provides real-time SSE stream of enriched events
func (s *Server) handlePhotoEventsStream(w http.ResponseWriter, r *http.Request) {
	if s.dualWatcher == nil {
		http.Error(w, "Photo monitoring not available", http.StatusServiceUnavailable)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Get enriched events channel
	events := s.dualWatcher.GetEnrichedEvents()

	// Send initial connection confirmation
	fmt.Fprintf(w, "data: {\"type\": \"connected\", \"message\": \"Photo events stream connected\"}\n\n")
	w.(http.Flusher).Flush()

	// Stream events
	for {
		select {
		case event, ok := <-events:
			if !ok {
				return // channel closed
			}

			// Convert to JSON
			eventData, err := json.Marshal(map[string]interface{}{
				"type":  "photo_event",
				"event": event,
			})
			if err != nil {
				continue
			}

			// Send SSE event
			fmt.Fprintf(w, "data: %s\n\n", eventData)
			w.(http.Flusher).Flush()

		case <-r.Context().Done():
			return // client disconnected
		}
	}
}

// handlePhotoDetails returns detailed information about a specific photo
func (s *Server) handlePhotoDetails(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	photoPath := vars["path"]

	if photoPath == "" {
		http.Error(w, "Photo path required", http.StatusBadRequest)
		return
	}

	// Get file info
	fileInfo, err := os.Stat(photoPath)
	if err != nil {
		http.Error(w, "Photo not found", http.StatusNotFound)
		return
	}

	response := map[string]interface{}{
		"file_path":    photoPath,
		"file_size":    fileInfo.Size(),
		"modified_at":  fileInfo.ModTime(),
		"exists":       true,
		"in_darktable": false,
	}

	// Try to get darktable metadata
	if s.dualWatcher != nil {
		folderPath := filepath.Dir(photoPath)
		filename := filepath.Base(photoPath)

		photos, err := s.dualWatcher.GetDarktableDB().GetByFolder(folderPath)
		if err == nil {
			for _, photo := range photos {
				if photo.Filename == filename {
					response["in_darktable"] = true
					response["darktable_metadata"] = photo
					break
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
