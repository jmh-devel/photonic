package grpcserver

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "photonic/internal/proto/proto"
)

type PhotoSyncServer struct {
	pb.UnimplementedPhotoSyncServer

	// Agent management
	agents      map[string]*ConnectedAgent
	agentsMutex sync.RWMutex

	// Photo storage
	storageRoot string
	uploadDir   string

	// Server stats
	serverID   string
	startTime  time.Time
	stats      *ServerStats
	statsMutex sync.RWMutex

	// Sync queue
	syncQueue *SyncQueue

	// Configuration
	config *ServerConfig
}

type ConnectedAgent struct {
	AgentID      string
	Hostname     string
	Platform     string
	Capabilities []string
	Config       *pb.AgentConfig
	Status       *pb.AgentStatus
	LastSeen     time.Time
	TaskQueue    []*pb.TaskAssignment
}

type ServerStats struct {
	TotalAgents      int
	ActiveAgents     int
	TotalConnections int64
	PhotosProcessed  int64
	BytesTransferred int64
	ActiveTransfers  int
	ErrorCount       int64
}

type SyncQueue struct {
	items      []*QueueItem
	itemsMutex sync.RWMutex
	stats      *QueueStats
}

type QueueItem struct {
	ID         string
	AgentID    string
	Type       pb.QueueItemType
	FilePath   string
	FileSize   int64
	Status     pb.QueueItemStatus
	Priority   int32
	QueuedAt   time.Time
	StartedAt  *time.Time
	RetryCount int32
	Error      string
}

type QueueStats struct {
	PendingCount       int32
	ProcessingCount    int32
	CompletedCount     int32
	FailedCount        int32
	TotalBytesPending  int64
	AverageProcessTime float64
}

type ServerConfig struct {
	Port                 int
	StorageRoot          string
	MaxConcurrentUploads int
	MaxFileSize          int64
	RequireChecksums     bool
}

func NewPhotoSyncServer(config *ServerConfig) *PhotoSyncServer {
	if config.StorageRoot == "" {
		config.StorageRoot = "./photonic-storage"
	}

	uploadDir := filepath.Join(config.StorageRoot, "uploads")
	os.MkdirAll(uploadDir, 0755)

	return &PhotoSyncServer{
		agents:      make(map[string]*ConnectedAgent),
		storageRoot: config.StorageRoot,
		uploadDir:   uploadDir,
		serverID:    fmt.Sprintf("photonic-server-%d", time.Now().Unix()),
		startTime:   time.Now(),
		stats:       &ServerStats{},
		syncQueue:   &SyncQueue{items: make([]*QueueItem, 0), stats: &QueueStats{}},
		config:      config,
	}
}

// NewPhotoSyncServerSimple creates a PhotoSyncServer with just a storage path
func NewPhotoSyncServerSimple(storageRoot string) (*PhotoSyncServer, error) {
	config := &ServerConfig{
		StorageRoot:          storageRoot,
		MaxFileSize:          500 * 1024 * 1024, // 500MB
		MaxConcurrentUploads: 10,
		RequireChecksums:     true,
	}
	return NewPhotoSyncServer(config), nil
}

func (s *PhotoSyncServer) Start(port int) error {
	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %v", port, err)
	}

	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(100*1024*1024), // 100MB
		grpc.MaxSendMsgSize(100*1024*1024), // 100MB
	)

	pb.RegisterPhotoSyncServer(grpcServer, s)

	fmt.Printf("🚀 Photonic gRPC Server starting on port %d\n", port)
	fmt.Printf("📁 Storage root: %s\n", s.storageRoot)
	fmt.Printf("🆔 Server ID: %s\n", s.serverID)

	return grpcServer.Serve(listen)
}

// Agent Management

func (s *PhotoSyncServer) RegisterAgent(ctx context.Context, req *pb.RegisterAgentRequest) (*pb.RegisterAgentResponse, error) {
	s.agentsMutex.Lock()
	defer s.agentsMutex.Unlock()

	fmt.Printf("🤖 Agent registration request from %s (%s)\n", req.Hostname, req.AgentId)

	agent := &ConnectedAgent{
		AgentID:      req.AgentId,
		Hostname:     req.Hostname,
		Platform:     req.Platform,
		Capabilities: req.Capabilities,
		Config:       req.Config,
		Status: &pb.AgentStatus{
			Status: pb.AgentStatus_IDLE,
		},
		LastSeen:  time.Now(),
		TaskQueue: make([]*pb.TaskAssignment, 0),
	}

	s.agents[req.AgentId] = agent

	s.statsMutex.Lock()
	s.stats.TotalAgents = len(s.agents)
	s.stats.TotalConnections++
	s.statsMutex.Unlock()

	return &pb.RegisterAgentResponse{
		Success: true,
		Message: fmt.Sprintf("Agent %s registered successfully", req.AgentId),
		ServerConfig: &pb.ServerConfig{
			ServerId:         s.serverID,
			PreferredFormats: []string{"CR2", "NEF", "ARW", "JPEG", "TIFF"},
			MaxFileSize:      s.config.MaxFileSize,
			ConcurrentLimit:  int32(s.config.MaxConcurrentUploads),
			RequireChecksums: s.config.RequireChecksums,
		},
	}, nil
}

func (s *PhotoSyncServer) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	s.agentsMutex.Lock()
	defer s.agentsMutex.Unlock()

	agent, exists := s.agents[req.AgentId]
	if !exists {
		return nil, status.Errorf(codes.NotFound, "Agent %s not registered", req.AgentId)
	}

	// Update agent status
	agent.Status = req.Status
	agent.LastSeen = time.Now()

	// Count active agents
	activeCount := 0
	for _, a := range s.agents {
		if time.Since(a.LastSeen) < 60*time.Second {
			activeCount++
		}
	}

	s.statsMutex.Lock()
	s.stats.ActiveAgents = activeCount
	s.statsMutex.Unlock()

	// TODO: Generate new tasks based on queue and agent capabilities
	newTasks := []*pb.TaskAssignment{}

	return &pb.HeartbeatResponse{
		Success:     true,
		NewTasks:    newTasks,
		CancelTasks: []string{}, // TODO: implement task cancellation
		ServerStatus: &pb.ServerStatus{
			ServerId:         s.serverID,
			StartTime:        nil, // TODO: convert to protobuf timestamp
			ActiveAgents:     int32(activeCount),
			TotalConnections: int32(s.stats.TotalConnections),
			ActiveTransfers:  int32(s.stats.ActiveTransfers),
		},
	}, nil
}

func (s *PhotoSyncServer) UpdateAgentStatus(ctx context.Context, req *pb.UpdateAgentStatusRequest) (*pb.UpdateAgentStatusResponse, error) {
	s.agentsMutex.Lock()
	defer s.agentsMutex.Unlock()

	agent, exists := s.agents[req.AgentId]
	if !exists {
		return &pb.UpdateAgentStatusResponse{
			Success: false,
			Message: "Agent not found",
		}, nil
	}

	agent.Status = req.Status
	agent.LastSeen = time.Now()

	return &pb.UpdateAgentStatusResponse{
		Success: true,
		Message: "Status updated",
	}, nil
}

// Photo Discovery and Metadata

func (s *PhotoSyncServer) DiscoverPhotos(ctx context.Context, req *pb.DiscoverPhotosRequest) (*pb.DiscoverPhotosResponse, error) {
	fmt.Printf("📁 Photo discovery request from agent %s for directories: %v\n", req.AgentId, req.Directories)

	photos := []*pb.PhotoInfo{}
	scanID := fmt.Sprintf("scan-%d", time.Now().Unix())

	// Scan each directory for photos
	for _, dir := range req.Directories {
		fmt.Printf("🔍 Scanning directory: %s\n", dir)

		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				fmt.Printf("⚠️ Error accessing %s: %v\n", path, err)
				return nil // Continue scanning other files
			}

			if info.IsDir() {
				return nil // Skip directories
			}

			// Check if it's a supported photo format
			ext := strings.ToLower(filepath.Ext(path))
			supportedFormats := []string{".jpg", ".jpeg", ".png", ".tiff", ".tif", ".cr2", ".nef", ".arw", ".orf", ".dng", ".raw"}

			isPhoto := false
			for _, format := range supportedFormats {
				if ext == format {
					isPhoto = true
					break
				}
			}

			if !isPhoto {
				return nil
			}

			// Create photo info using correct field names
			relPath, _ := filepath.Rel(dir, path)
			photoInfo := &pb.PhotoInfo{
				FilePath:     path,
				RelativePath: relPath,
				FileSize:     info.Size(),
				IsRaw:        strings.Contains(ext, "cr2") || strings.Contains(ext, "nef") || strings.Contains(ext, "arw"),
			}

			photos = append(photos, photoInfo)
			return nil
		})

		if err != nil {
			fmt.Printf("❌ Failed to scan directory %s: %v\n", dir, err)
		}
	}

	fmt.Printf("📸 Found %d photos total\n", len(photos))

	return &pb.DiscoverPhotosResponse{
		Photos:     photos,
		TotalCount: int32(len(photos)),
		ScanId:     scanID,
	}, nil
}

func (s *PhotoSyncServer) GetPhotoMetadata(ctx context.Context, req *pb.GetPhotoMetadataRequest) (*pb.GetPhotoMetadataResponse, error) {
	// TODO: Extract metadata from specified photos
	return &pb.GetPhotoMetadataResponse{
		Photos: []*pb.PhotoInfo{},
		Errors: []string{},
	}, nil
}

func (s *PhotoSyncServer) SyncPhotoMetadata(ctx context.Context, req *pb.SyncPhotoMetadataRequest) (*pb.SyncPhotoMetadataResponse, error) {
	// TODO: Implement metadata synchronization with conflict resolution
	return &pb.SyncPhotoMetadataResponse{
		Conflicts:   []*pb.ConflictResolution{},
		SyncedCount: 0,
		Errors:      []string{},
	}, nil
}

// Binary Photo Transfer

func (s *PhotoSyncServer) UploadPhoto(stream pb.PhotoSync_UploadPhotoServer) error {
	var uploadMeta *pb.UploadPhotoMeta
	var file *os.File
	var hash = sha256.New()
	var bytesReceived int64
	var startTime = time.Now()

	defer func() {
		if file != nil {
			file.Close()
		}
	}()

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Errorf(codes.Internal, "Failed to receive upload data: %v", err)
		}

		switch content := req.Content.(type) {
		case *pb.UploadPhotoRequest_Meta:
			uploadMeta = content.Meta

			// Create upload file
			fileName := fmt.Sprintf("%s_%s", uploadMeta.UploadId, filepath.Base(uploadMeta.PhotoInfo.FilePath))
			filePath := filepath.Join(s.uploadDir, fileName)

			file, err = os.Create(filePath)
			if err != nil {
				return status.Errorf(codes.Internal, "Failed to create upload file: %v", err)
			}

			fmt.Printf("📤 Starting upload: %s (%d bytes)\n", uploadMeta.PhotoInfo.FilePath, uploadMeta.TotalSize)

		case *pb.UploadPhotoRequest_Chunk:
			if file == nil || uploadMeta == nil {
				return status.Errorf(codes.InvalidArgument, "Received chunk before metadata")
			}

			// Write chunk to file and update hash
			n, err := file.Write(content.Chunk)
			if err != nil {
				return status.Errorf(codes.Internal, "Failed to write chunk: %v", err)
			}

			hash.Write(content.Chunk)
			bytesReceived += int64(n)
		}
	}

	if file == nil || uploadMeta == nil {
		return status.Errorf(codes.InvalidArgument, "No metadata received")
	}

	// Verify checksum if provided
	calculatedHash := fmt.Sprintf("%x", hash.Sum(nil))
	if uploadMeta.PhotoInfo.ChecksumSha256 != "" && calculatedHash != uploadMeta.PhotoInfo.ChecksumSha256 {
		os.Remove(file.Name())
		return status.Errorf(codes.DataLoss, "Checksum verification failed")
	}

	uploadTime := time.Since(startTime).Seconds()

	// Update server stats
	s.statsMutex.Lock()
	s.stats.PhotosProcessed++
	s.stats.BytesTransferred += bytesReceived
	s.statsMutex.Unlock()

	fmt.Printf("✅ Upload completed: %s (%d bytes in %.2fs)\n",
		uploadMeta.PhotoInfo.FilePath, bytesReceived, uploadTime)

	return stream.SendAndClose(&pb.UploadPhotoResponse{
		Success:           true,
		Message:           "Upload completed successfully",
		FilePath:          file.Name(),
		ChecksumSha256:    calculatedHash,
		BytesReceived:     bytesReceived,
		UploadTimeSeconds: float32(uploadTime),
	})
}

func (s *PhotoSyncServer) DownloadPhoto(req *pb.DownloadPhotoRequest, stream pb.PhotoSync_DownloadPhotoServer) error {
	// TODO: Implement photo download with streaming
	fmt.Printf("📥 Download request from agent %s for file: %s\n", req.AgentId, req.FilePath)

	// This is a placeholder implementation
	return status.Errorf(codes.Unimplemented, "Download not yet implemented")
}

// Batch Operations

func (s *PhotoSyncServer) BatchSync(ctx context.Context, req *pb.BatchSyncRequest) (*pb.BatchSyncResponse, error) {
	// TODO: Implement batch synchronization
	fmt.Printf("📦 Batch sync request from agent %s with %d photos\n", req.AgentId, len(req.Photos))

	return &pb.BatchSyncResponse{
		BatchId:         req.BatchId,
		Status:          pb.BatchStatus_BATCH_QUEUED,
		TotalPhotos:     int32(len(req.Photos)),
		CompletedPhotos: 0,
		FailedPhotos:    0,
		ProgressPercent: 0,
	}, nil
}

func (s *PhotoSyncServer) GetSyncQueue(ctx context.Context, req *pb.GetSyncQueueRequest) (*pb.GetSyncQueueResponse, error) {
	s.syncQueue.itemsMutex.RLock()
	defer s.syncQueue.itemsMutex.RUnlock()

	// Filter by agent if specified
	var items []*pb.QueueItem
	for _, item := range s.syncQueue.items {
		if req.AgentId == "" || item.AgentID == req.AgentId {
			pbItem := &pb.QueueItem{
				ItemId:       item.ID,
				AgentId:      item.AgentID,
				ItemType:     item.Type,
				FilePath:     item.FilePath,
				FileSize:     item.FileSize,
				Status:       item.Status,
				Priority:     item.Priority,
				RetryCount:   item.RetryCount,
				ErrorMessage: item.Error,
			}
			items = append(items, pbItem)
		}
	}

	// Limit results if requested
	if req.Limit > 0 && int32(len(items)) > req.Limit {
		items = items[:req.Limit]
	}

	return &pb.GetSyncQueueResponse{
		QueueItems: items,
		TotalCount: int32(len(s.syncQueue.items)),
		Stats: &pb.QueueStats{
			PendingCount:          s.syncQueue.stats.PendingCount,
			ProcessingCount:       s.syncQueue.stats.ProcessingCount,
			CompletedCount:        s.syncQueue.stats.CompletedCount,
			FailedCount:           s.syncQueue.stats.FailedCount,
			TotalBytesPending:     s.syncQueue.stats.TotalBytesPending,
			AverageProcessingTime: float32(s.syncQueue.stats.AverageProcessTime),
		},
	}, nil
}

// Status and Monitoring

func (s *PhotoSyncServer) GetAgentStatus(ctx context.Context, req *pb.GetAgentStatusRequest) (*pb.GetAgentStatusResponse, error) {
	s.agentsMutex.RLock()
	defer s.agentsMutex.RUnlock()

	agent, exists := s.agents[req.AgentId]
	if !exists {
		return nil, status.Errorf(codes.NotFound, "Agent not found")
	}

	response := &pb.GetAgentStatusResponse{
		Status: agent.Status,
	}

	// Include queue items if requested
	if req.IncludeQueue {
		s.syncQueue.itemsMutex.RLock()
		var queueItems []*pb.QueueItem
		for _, item := range s.syncQueue.items {
			if item.AgentID == req.AgentId {
				queueItems = append(queueItems, &pb.QueueItem{
					ItemId:   item.ID,
					AgentId:  item.AgentID,
					ItemType: item.Type,
					FilePath: item.FilePath,
					FileSize: item.FileSize,
					Status:   item.Status,
					Priority: item.Priority,
				})
			}
		}
		response.QueueItems = queueItems
		s.syncQueue.itemsMutex.RUnlock()
	}

	return response, nil
}

func (s *PhotoSyncServer) GetServerStats(ctx context.Context, req *pb.GetServerStatsRequest) (*pb.GetServerStatsResponse, error) {
	s.statsMutex.RLock()
	stats := *s.stats
	s.statsMutex.RUnlock()

	response := &pb.GetServerStatsResponse{
		Status: &pb.ServerStatus{
			ServerId:         s.serverID,
			ActiveAgents:     int32(stats.ActiveAgents),
			TotalConnections: int32(stats.TotalConnections),
			ActiveTransfers:  int32(stats.ActiveTransfers),
		},
		Metrics: []*pb.ServerMetric{
			{
				MetricName: "photos_processed",
				Value:      float64(stats.PhotosProcessed),
				Unit:       "count",
			},
			{
				MetricName: "bytes_transferred",
				Value:      float64(stats.BytesTransferred),
				Unit:       "bytes",
			},
		},
	}

	// Include agent statuses if requested
	if req.IncludeAgents {
		s.agentsMutex.RLock()
		var agentStatuses []*pb.AgentStatus
		for _, agent := range s.agents {
			agentStatuses = append(agentStatuses, agent.Status)
		}
		response.AgentStatuses = agentStatuses
		s.agentsMutex.RUnlock()
	}

	// Include queue stats if requested
	if req.IncludeQueueStats {
		s.syncQueue.itemsMutex.RLock()
		response.QueueStats = &pb.QueueStats{
			PendingCount:    s.syncQueue.stats.PendingCount,
			ProcessingCount: s.syncQueue.stats.ProcessingCount,
			CompletedCount:  s.syncQueue.stats.CompletedCount,
			FailedCount:     s.syncQueue.stats.FailedCount,
		}
		s.syncQueue.itemsMutex.RUnlock()
	}

	return response, nil
}

func (s *PhotoSyncServer) GetConnectedAgents() map[string]*ConnectedAgent {
	s.agentsMutex.RLock()
	defer s.agentsMutex.RUnlock()

	agents := make(map[string]*ConnectedAgent)
	for id, agent := range s.agents {
		agents[id] = agent
	}
	return agents
}

// GetConnectedAgentsCount returns the number of currently connected agents
func (s *PhotoSyncServer) GetConnectedAgentsCount() int {
	s.agentsMutex.RLock()
	defer s.agentsMutex.RUnlock()
	return len(s.agents)
}

// RegisterWithServer registers this PhotoSyncServer with a gRPC server
func (s *PhotoSyncServer) RegisterWithServer(grpcServer *grpc.Server) {
	pb.RegisterPhotoSyncServer(grpcServer, s)
}
