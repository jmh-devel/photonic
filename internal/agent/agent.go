package agent

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"

	pb "photonic/internal/proto/proto"
)

type Agent struct {
	config     *Config
	client     pb.PhotoSyncClient
	conn       *grpc.ClientConn
	agentID    string
	hostname   string
	status     pb.AgentStatus_Status
	tasks      map[string]*Task
	tasksMutex sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

type Config struct {
	ServerAddress            string   `json:"serverAddress"`
	AgentID                  string   `json:"agentId"`
	Hostname                 string   `json:"hostname"`
	PhotoDirectories         []string `json:"photoDirectories"`
	NFSMount                 string   `json:"nfsMount"`
	MaxConcurrentUploads     int      `json:"maxConcurrentUploads"`
	MaxConcurrentDownloads   int      `json:"maxConcurrentDownloads"`
	BatchSizeBytes           int64    `json:"batchSizeBytes"`
	HeartbeatInterval        int      `json:"heartbeatInterval"`
	EnableRawProcessing      bool     `json:"enableRawProcessing"`
	EnableMetadataExtraction bool     `json:"enableMetadataExtraction"`

	// Security
	TLSCertPath   string `json:"tlsCertPath"`
	TLSKeyPath    string `json:"tlsKeyPath"`
	CACertPath    string `json:"caCertPath"`
	SkipTLSVerify bool   `json:"skipTlsVerify"`
}

type Task struct {
	ID          string
	Type        string
	Status      string
	Progress    float32
	StartTime   time.Time
	Description string
	Error       string
}

func NewAgent(config *Config) (*Agent, error) {
	ctx, cancel := context.WithCancel(context.Background())

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	if config.Hostname == "" {
		config.Hostname = hostname
	}

	if config.AgentID == "" {
		config.AgentID = fmt.Sprintf("agent-%s-%d", hostname, time.Now().Unix())
	}

	agent := &Agent{
		config:   config,
		agentID:  config.AgentID,
		hostname: config.Hostname,
		status:   pb.AgentStatus_IDLE,
		tasks:    make(map[string]*Task),
		ctx:      ctx,
		cancel:   cancel,
	}

	return agent, nil
}

func (a *Agent) Start() error {
	// Establish gRPC connection
	conn, err := a.createGRPCConnection()
	if err != nil {
		return fmt.Errorf("failed to connect to server: %v", err)
	}
	a.conn = conn
	a.client = pb.NewPhotoSyncClient(conn)

	// Register with server
	if err := a.registerWithServer(); err != nil {
		return fmt.Errorf("failed to register with server: %v", err)
	}

	fmt.Printf("🤖 Photonic Agent %s started successfully\n", a.agentID)
	fmt.Printf("🏠 Hostname: %s\n", a.hostname)
	fmt.Printf("🔗 Server: %s\n", a.config.ServerAddress)
	fmt.Printf("📁 Watching directories: %v\n", a.config.PhotoDirectories)

	// Start background routines
	a.wg.Add(3)
	go a.heartbeatLoop()
	go a.taskProcessor()
	go a.photoScanner()

	return nil
}

func (a *Agent) Stop() error {
	fmt.Printf("🛑 Stopping Photonic Agent %s...\n", a.agentID)

	a.cancel()
	a.wg.Wait()

	if a.conn != nil {
		a.conn.Close()
	}

	fmt.Printf("✅ Agent %s stopped\n", a.agentID)
	return nil
}

func (a *Agent) createGRPCConnection() (*grpc.ClientConn, error) {
	var opts []grpc.DialOption

	// TLS Configuration
	if !a.config.SkipTLSVerify {
		tlsConfig, err := a.createTLSConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS config: %v", err)
		}
		creds := credentials.NewTLS(tlsConfig)
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithInsecure())
	}

	// Keepalive settings
	keepaliveParams := keepalive.ClientParameters{
		Time:                30 * time.Second,
		Timeout:             5 * time.Second,
		PermitWithoutStream: true,
	}
	opts = append(opts, grpc.WithKeepaliveParams(keepaliveParams))

	// Max message sizes for large photo transfers
	opts = append(opts,
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(100*1024*1024), // 100MB
			grpc.MaxCallSendMsgSize(100*1024*1024), // 100MB
		),
	)

	return grpc.Dial(a.config.ServerAddress, opts...)
}

func (a *Agent) createTLSConfig() (*tls.Config, error) {
	config := &tls.Config{}

	if a.config.CACertPath != "" {
		caCert, err := os.ReadFile(a.config.CACertPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA cert: %v", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to append CA cert")
		}
		config.RootCAs = caCertPool
	}

	if a.config.TLSCertPath != "" && a.config.TLSKeyPath != "" {
		cert, err := tls.LoadX509KeyPair(a.config.TLSCertPath, a.config.TLSKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load client cert: %v", err)
		}
		config.Certificates = []tls.Certificate{cert}
	}

	return config, nil
}

func (a *Agent) registerWithServer() error {
	req := &pb.RegisterAgentRequest{
		AgentId:  a.agentID,
		Hostname: a.hostname,
		Platform: fmt.Sprintf("%s/%s", os.Getenv("GOOS"), os.Getenv("GOARCH")),
		Capabilities: []string{
			"photo-upload",
			"photo-download",
			"metadata-extraction",
			"batch-processing",
		},
		Config: &pb.AgentConfig{
			PhotoDirectories:         a.config.PhotoDirectories,
			NfsMount:                 a.config.NFSMount,
			MaxConcurrentUploads:     int32(a.config.MaxConcurrentUploads),
			MaxConcurrentDownloads:   int32(a.config.MaxConcurrentDownloads),
			BatchSizeBytes:           a.config.BatchSizeBytes,
			HeartbeatInterval:        int32(a.config.HeartbeatInterval),
			EnableRawProcessing:      a.config.EnableRawProcessing,
			EnableMetadataExtraction: a.config.EnableMetadataExtraction,
		},
		Version: "1.0.0",
	}

	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()

	resp, err := a.client.RegisterAgent(ctx, req)
	if err != nil {
		return fmt.Errorf("registration failed: %v", err)
	}

	if !resp.Success {
		return fmt.Errorf("registration rejected: %s", resp.Message)
	}

	fmt.Printf("✅ Successfully registered with server: %s\n", resp.Message)
	return nil
}

func (a *Agent) heartbeatLoop() {
	defer a.wg.Done()

	ticker := time.NewTicker(time.Duration(a.config.HeartbeatInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.sendHeartbeat()
		}
	}
}

func (a *Agent) sendHeartbeat() {
	a.tasksMutex.RLock()
	var taskProgress []*pb.TaskProgress
	for _, task := range a.tasks {
		taskProgress = append(taskProgress, &pb.TaskProgress{
			TaskId:          task.ID,
			TaskType:        task.Type,
			ProgressPercent: task.Progress,
			StatusMessage:   task.Description,
		})
	}
	a.tasksMutex.RUnlock()

	req := &pb.HeartbeatRequest{
		AgentId: a.agentID,
		Status: &pb.AgentStatus{
			Status:          a.status,
			ActiveUploads:   0, // TODO: track actual counts
			ActiveDownloads: 0,
			QueueDepth:      int32(len(a.tasks)),
			CurrentTask:     a.getCurrentTaskDescription(),
			LastActivity:    nil, // TODO: track last activity
		},
		TaskProgress: taskProgress,
	}

	ctx, cancel := context.WithTimeout(a.ctx, 5*time.Second)
	defer cancel()

	resp, err := a.client.Heartbeat(ctx, req)
	if err != nil {
		fmt.Printf("❌ Heartbeat failed: %v\n", err)
		return
	}

	// Process new tasks from server
	for _, taskAssignment := range resp.NewTasks {
		a.addTask(taskAssignment)
	}

	// Cancel tasks if requested
	for _, taskID := range resp.CancelTasks {
		a.cancelTask(taskID)
	}
}

func (a *Agent) getCurrentTaskDescription() string {
	a.tasksMutex.RLock()
	defer a.tasksMutex.RUnlock()

	for _, task := range a.tasks {
		if task.Status == "processing" {
			return task.Description
		}
	}
	return "idle"
}

func (a *Agent) addTask(assignment *pb.TaskAssignment) {
	a.tasksMutex.Lock()
	defer a.tasksMutex.Unlock()

	task := &Task{
		ID:          assignment.TaskId,
		Type:        assignment.TaskType,
		Status:      "queued",
		Progress:    0.0,
		StartTime:   time.Now(),
		Description: fmt.Sprintf("Task %s queued", assignment.TaskType),
	}

	a.tasks[assignment.TaskId] = task
	fmt.Printf("📋 New task assigned: %s (%s)\n", assignment.TaskId, assignment.TaskType)
}

func (a *Agent) cancelTask(taskID string) {
	a.tasksMutex.Lock()
	defer a.tasksMutex.Unlock()

	if task, exists := a.tasks[taskID]; exists {
		task.Status = "cancelled"
		task.Description = "Task cancelled by server"
		fmt.Printf("🚫 Task cancelled: %s\n", taskID)
	}
}

func (a *Agent) taskProcessor() {
	defer a.wg.Done()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.processQueuedTasks()
		}
	}
}

func (a *Agent) processQueuedTasks() {
	a.tasksMutex.Lock()
	defer a.tasksMutex.Unlock()

	for _, task := range a.tasks {
		if task.Status == "queued" {
			// Start processing the task
			task.Status = "processing"
			task.Description = fmt.Sprintf("Processing %s task", task.Type)

			go func(t *Task) {
				// Simulate task processing
				for i := 0; i < 100; i++ {
					if t.Status == "cancelled" {
						return
					}
					t.Progress = float32(i)
					time.Sleep(100 * time.Millisecond)
				}

				a.tasksMutex.Lock()
				t.Status = "completed"
				t.Progress = 100.0
				t.Description = fmt.Sprintf("Task %s completed", t.Type)
				a.tasksMutex.Unlock()

				fmt.Printf("✅ Task completed: %s\n", t.ID)
			}(task)

			// Only process one task at a time for now
			break
		}
	}
}

func (a *Agent) photoScanner() {
	defer a.wg.Done()

	// Scan for photos every 30 seconds
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Initial scan
	a.scanPhotoDirectories()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.scanPhotoDirectories()
		}
	}
}

func (a *Agent) scanPhotoDirectories() {
	fmt.Printf("🔍 Scanning photo directories...\n")

	for _, dir := range a.config.PhotoDirectories {
		// TODO: Implement photo discovery
		fmt.Printf("📁 Scanning directory: %s\n", dir)

		// This would be replaced with actual photo discovery logic
		req := &pb.DiscoverPhotosRequest{
			AgentId:         a.agentID,
			Directories:     []string{dir},
			Recursive:       true,
			IncludeRaw:      a.config.EnableRawProcessing,
			ExtractMetadata: a.config.EnableMetadataExtraction,
		}

		ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
		resp, err := a.client.DiscoverPhotos(ctx, req)
		cancel()

		if err != nil {
			fmt.Printf("❌ Photo discovery failed for %s: %v\n", dir, err)
			continue
		}

		fmt.Printf("📸 Found %d photos in %s\n", resp.TotalCount, dir)
	}
}

func (a *Agent) GetStatus() map[string]interface{} {
	a.tasksMutex.RLock()
	defer a.tasksMutex.RUnlock()

	return map[string]interface{}{
		"agentId":     a.agentID,
		"hostname":    a.hostname,
		"status":      a.status.String(),
		"taskCount":   len(a.tasks),
		"server":      a.config.ServerAddress,
		"directories": a.config.PhotoDirectories,
	}
}
