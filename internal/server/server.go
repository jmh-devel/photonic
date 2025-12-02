package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"photonic/internal/pipeline"
	"photonic/internal/storage"
	"photonic/internal/tasks"

	"github.com/gorilla/mux"
)

// Server wraps HTTP server with enhanced photo monitoring
type Server struct {
	addr        string
	store       *storage.Store
	pipeline    *pipeline.Pipeline
	dualWatcher *tasks.DualWatcher
	log         *slog.Logger
	server      *http.Server
}

// NewServer creates a new enhanced server with dual watching capabilities
func NewServer(
	addr string,
	store *storage.Store,
	pipe *pipeline.Pipeline,
	watchPaths []string,
	darktableConfigPath string,
	log *slog.Logger,
) (*Server, error) {

	s := &Server{
		addr:     addr,
		store:    store,
		pipeline: pipe,
		log:      log,
	}

	// Setup dual watcher if paths provided
	if len(watchPaths) > 0 {
		dualWatcher, err := tasks.NewDualWatcher(watchPaths, store, darktableConfigPath)
		if err != nil {
			log.Warn("Failed to setup dual watcher", "error", err)
		} else {
			s.dualWatcher = dualWatcher
			log.Info("Dual watcher initialized", "paths", watchPaths)
		}
	}

	return s, nil
}

// Start begins the server and monitoring services
func (s *Server) Start(ctx context.Context) error {
	// Start dual watcher if available
	if s.dualWatcher != nil {
		if err := s.dualWatcher.Start(); err != nil {
			s.log.Error("Failed to start dual watcher", "error", err)
			return err
		}
	}

	// Setup routes
	r := mux.NewRouter()
	s.setupRoutes(r)
	s.setupDarktableRoutes(r)

	s.server = &http.Server{
		Addr:    s.addr,
		Handler: r,
	}

	// Graceful shutdown
	go func() {
		<-ctx.Done()
		s.log.Info("Shutting down server...")

		if s.dualWatcher != nil {
			s.dualWatcher.Stop()
		}

		ctxShutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.server.Shutdown(ctxShutdown)
	}()

	s.log.Info("Server starting", "addr", s.addr)
	err := s.server.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// setupRoutes configures basic HTTP routes
func (s *Server) setupRoutes(r *mux.Router) {
	r.HandleFunc("/healthz", s.handleHealth).Methods("GET")
	r.HandleFunc("/jobs", s.handleJobs).Methods("GET")
	r.HandleFunc("/stream", s.handleJobStream).Methods("GET")
}

// Serve is the legacy function for backward compatibility
func Serve(ctx context.Context, addr string, store *storage.Store, pipe *pipeline.Pipeline, log *slog.Logger) error {
	server, err := NewServer(addr, store, pipe, nil, "", log)
	if err != nil {
		return err
	}
	return server.Start(ctx)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (s *Server) handleJobs(w http.ResponseWriter, r *http.Request) {
	recs, err := s.store.RecentJobs(100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(recs)
}

func (s *Server) handleJobStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	resCh, unsubscribe := s.pipeline.Subscribe()
	defer unsubscribe()
	for {
		select {
		case <-r.Context().Done():
			return
		case res, ok := <-resCh:
			if !ok {
				return
			}
			payload, _ := json.Marshal(res)
			_, _ = w.Write([]byte("data: " + string(payload) + "\n\n"))
			flusher.Flush()
		}
	}
}
