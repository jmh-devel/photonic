package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

type WebServer struct {
	port     int
	upgrader websocket.Upgrader
	clients  map[*websocket.Conn]bool
	hub      *WebSocketHub
}

type WebSocketHub struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan []byte
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
}

type DashboardData struct {
	ServerStats    ServerStats   `json:"serverStats"`
	AgentStatuses  []AgentStatus `json:"agentStatuses"`
	QueueStats     QueueStats    `json:"queueStats"`
	RecentActivity []Activity    `json:"recentActivity"`
	SystemMetrics  SystemMetrics `json:"systemMetrics"`
	Timestamp      time.Time     `json:"timestamp"`
}

type ServerStats struct {
	ActiveAgents     int     `json:"activeAgents"`
	TotalConnections int     `json:"totalConnections"`
	StorageUsed      int64   `json:"storageUsed"`
	StorageAvailable int64   `json:"storageAvailable"`
	CPUUsage         float64 `json:"cpuUsage"`
	MemoryUsage      float64 `json:"memoryUsage"`
	ActiveTransfers  int     `json:"activeTransfers"`
	NetworkBandwidth float64 `json:"networkBandwidth"`
}

type AgentStatus struct {
	AgentID         string    `json:"agentId"`
	Hostname        string    `json:"hostname"`
	Status          string    `json:"status"`
	ActiveUploads   int       `json:"activeUploads"`
	ActiveDownloads int       `json:"activeDownloads"`
	BytesUploaded   int64     `json:"bytesUploaded"`
	BytesDownloaded int64     `json:"bytesDownloaded"`
	QueueDepth      int       `json:"queueDepth"`
	LastActivity    time.Time `json:"lastActivity"`
	Errors          []string  `json:"errors"`
}

type QueueStats struct {
	PendingCount       int     `json:"pendingCount"`
	ProcessingCount    int     `json:"processingCount"`
	CompletedCount     int     `json:"completedCount"`
	FailedCount        int     `json:"failedCount"`
	TotalBytesPending  int64   `json:"totalBytesPending"`
	AverageProcessTime float64 `json:"averageProcessTime"`
}

type Activity struct {
	Timestamp   time.Time `json:"timestamp"`
	AgentID     string    `json:"agentId"`
	Action      string    `json:"action"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
}

type SystemMetrics struct {
	PhotosProcessed      int64   `json:"photosProcessed"`
	TotalPhotosManaged   int64   `json:"totalPhotosManaged"`
	AverageUploadSpeed   float64 `json:"averageUploadSpeed"`
	AverageDownloadSpeed float64 `json:"averageDownloadSpeed"`
	ErrorRate            float64 `json:"errorRate"`
	UptimeSeconds        int64   `json:"uptimeSeconds"`
}

func NewWebServer(port int) *WebServer {
	hub := &WebSocketHub{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}

	return &WebServer{
		port: port,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for development
			},
		},
		clients: make(map[*websocket.Conn]bool),
		hub:     hub,
	}
}

func (ws *WebServer) Start(ctx context.Context) error {
	go ws.hub.run()
	go ws.broadcastMetrics(ctx)

	router := mux.NewRouter()

	// Static files and templates
	router.HandleFunc("/", ws.handleDashboard).Methods("GET")
	router.HandleFunc("/api/stats", ws.handleAPIStats).Methods("GET")
	router.HandleFunc("/api/agents", ws.handleAPIAgents).Methods("GET")
	router.HandleFunc("/api/queue", ws.handleAPIQueue).Methods("GET")
	router.HandleFunc("/ws", ws.handleWebSocket).Methods("GET")

	// Static assets
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("web/static/"))))

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", ws.port),
		Handler: router,
	}

	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
	}()

	fmt.Printf("🌐 Photonic Web Dashboard starting on port %d\n", ws.port)
	fmt.Printf("📊 Dashboard: http://localhost:%d\n", ws.port)
	return server.ListenAndServe()
}

func (ws *WebServer) handleDashboard(w http.ResponseWriter, r *http.Request) {
	tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Photonic Dashboard</title>
    <style>
        :root {
            --bg-primary: #0f172a;
            --bg-secondary: #1e293b;
            --bg-tertiary: #334155;
            --text-primary: #f8fafc;
            --text-secondary: #cbd5e1;
            --accent: #3b82f6;
            --accent-hover: #2563eb;
            --success: #10b981;
            --warning: #f59e0b;
            --error: #ef4444;
            --border: #475569;
        }
        
        * { margin: 0; padding: 0; box-sizing: border-box; }
        
        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            background: var(--bg-primary);
            color: var(--text-primary);
            overflow-x: hidden;
        }
        
        .header {
            background: var(--bg-secondary);
            padding: 1rem 2rem;
            border-bottom: 1px solid var(--border);
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        
        .logo {
            font-size: 1.5rem;
            font-weight: bold;
            color: var(--accent);
        }
        
        .status-indicator {
            display: flex;
            align-items: center;
            gap: 0.5rem;
        }
        
        .status-dot {
            width: 10px;
            height: 10px;
            border-radius: 50%;
            background: var(--success);
            animation: pulse 2s infinite;
        }
        
        @keyframes pulse {
            0% { opacity: 1; }
            50% { opacity: 0.5; }
            100% { opacity: 1; }
        }
        
        .dashboard {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 1rem;
            padding: 2rem;
        }
        
        .card {
            background: var(--bg-secondary);
            border: 1px solid var(--border);
            border-radius: 8px;
            padding: 1.5rem;
        }
        
        .card-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 1rem;
            padding-bottom: 0.5rem;
            border-bottom: 1px solid var(--border);
        }
        
        .card-title {
            font-size: 1.1rem;
            font-weight: 600;
        }
        
        .metric {
            display: flex;
            justify-content: space-between;
            padding: 0.5rem 0;
        }
        
        .metric-value {
            font-weight: 600;
            color: var(--accent);
        }
        
        .agents-grid {
            display: grid;
            gap: 0.5rem;
        }
        
        .agent-card {
            background: var(--bg-tertiary);
            padding: 1rem;
            border-radius: 6px;
            border-left: 4px solid var(--success);
        }
        
        .agent-card.busy { border-left-color: var(--warning); }
        .agent-card.error { border-left-color: var(--error); }
        
        .agent-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 0.5rem;
        }
        
        .agent-name {
            font-weight: 600;
        }
        
        .agent-status {
            padding: 0.25rem 0.5rem;
            border-radius: 4px;
            font-size: 0.8rem;
            background: var(--success);
            color: white;
        }
        
        .queue-item {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 0.5rem 0;
            border-bottom: 1px solid var(--border);
        }
        
        .queue-item:last-child { border-bottom: none; }
        
        .progress-bar {
            width: 100%;
            height: 6px;
            background: var(--bg-tertiary);
            border-radius: 3px;
            overflow: hidden;
        }
        
        .progress-fill {
            height: 100%;
            background: var(--accent);
            transition: width 0.3s ease;
        }
        
        .activity-list {
            max-height: 300px;
            overflow-y: auto;
        }
        
        .activity-item {
            padding: 0.75rem;
            border-bottom: 1px solid var(--border);
            display: flex;
            gap: 1rem;
        }
        
        .activity-timestamp {
            color: var(--text-secondary);
            font-size: 0.8rem;
            white-space: nowrap;
        }
        
        .connection-status {
            position: fixed;
            top: 1rem;
            right: 1rem;
            padding: 0.5rem 1rem;
            border-radius: 4px;
            font-size: 0.9rem;
            z-index: 1000;
        }
        
        .connected {
            background: var(--success);
            color: white;
        }
        
        .disconnected {
            background: var(--error);
            color: white;
        }
    </style>
</head>
<body>
    <div class="connection-status disconnected" id="connectionStatus">Connecting...</div>
    
    <header class="header">
        <div class="logo">📸 Photonic Dashboard</div>
        <div class="status-indicator">
            <div class="status-dot"></div>
            <span>System Online</span>
        </div>
    </header>
    
    <main class="dashboard">
        <div class="card">
            <div class="card-header">
                <h3 class="card-title">Server Statistics</h3>
                <span id="lastUpdate">--</span>
            </div>
            <div class="metrics">
                <div class="metric">
                    <span>Active Agents</span>
                    <span class="metric-value" id="activeAgents">--</span>
                </div>
                <div class="metric">
                    <span>Total Connections</span>
                    <span class="metric-value" id="totalConnections">--</span>
                </div>
                <div class="metric">
                    <span>Storage Used</span>
                    <span class="metric-value" id="storageUsed">--</span>
                </div>
                <div class="metric">
                    <span>Active Transfers</span>
                    <span class="metric-value" id="activeTransfers">--</span>
                </div>
                <div class="metric">
                    <span>CPU Usage</span>
                    <span class="metric-value" id="cpuUsage">--</span>
                </div>
                <div class="metric">
                    <span>Memory Usage</span>
                    <span class="metric-value" id="memoryUsage">--</span>
                </div>
            </div>
        </div>
        
        <div class="card">
            <div class="card-header">
                <h3 class="card-title">Connected Agents</h3>
            </div>
            <div class="agents-grid" id="agentsGrid">
                <!-- Dynamic agent cards -->
            </div>
        </div>
        
        <div class="card">
            <div class="card-header">
                <h3 class="card-title">Sync Queue</h3>
            </div>
            <div class="metrics">
                <div class="metric">
                    <span>Pending</span>
                    <span class="metric-value" id="queuePending">--</span>
                </div>
                <div class="metric">
                    <span>Processing</span>
                    <span class="metric-value" id="queueProcessing">--</span>
                </div>
                <div class="metric">
                    <span>Completed</span>
                    <span class="metric-value" id="queueCompleted">--</span>
                </div>
                <div class="metric">
                    <span>Failed</span>
                    <span class="metric-value" id="queueFailed">--</span>
                </div>
            </div>
            <div class="progress-bar">
                <div class="progress-fill" id="queueProgress" style="width: 0%"></div>
            </div>
        </div>
        
        <div class="card">
            <div class="card-header">
                <h3 class="card-title">Recent Activity</h3>
            </div>
            <div class="activity-list" id="activityList">
                <!-- Dynamic activity items -->
            </div>
        </div>
    </main>
    
    <script>
        class PhotonicDashboard {
            constructor() {
                this.ws = null;
                this.reconnectAttempts = 0;
                this.maxReconnectAttempts = 5;
                this.connect();
            }
            
            connect() {
                const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
                const wsURL = protocol + '//' + window.location.host + '/ws';
                
                this.ws = new WebSocket(wsURL);
                
                this.ws.onopen = () => {
                    console.log('WebSocket connected');
                    this.reconnectAttempts = 0;
                    document.getElementById('connectionStatus').textContent = 'Connected';
                    document.getElementById('connectionStatus').className = 'connection-status connected';
                };
                
                this.ws.onmessage = (event) => {
                    const data = JSON.parse(event.data);
                    this.updateDashboard(data);
                };
                
                this.ws.onclose = () => {
                    console.log('WebSocket disconnected');
                    document.getElementById('connectionStatus').textContent = 'Disconnected';
                    document.getElementById('connectionStatus').className = 'connection-status disconnected';
                    this.reconnect();
                };
                
                this.ws.onerror = (error) => {
                    console.error('WebSocket error:', error);
                };
            }
            
            reconnect() {
                if (this.reconnectAttempts < this.maxReconnectAttempts) {
                    this.reconnectAttempts++;
                    console.log('Attempting to reconnect... (' + this.reconnectAttempts + '/' + this.maxReconnectAttempts + ')');
                    setTimeout(() => this.connect(), 3000);
                } else {
                    console.log('Max reconnection attempts reached');
                    document.getElementById('connectionStatus').textContent = 'Connection Failed';
                }
            }
            
            updateDashboard(data) {
                // Update server stats
                const stats = data.serverStats;
                document.getElementById('activeAgents').textContent = stats.activeAgents;
                document.getElementById('totalConnections').textContent = stats.totalConnections;
                document.getElementById('storageUsed').textContent = this.formatBytes(stats.storageUsed);
                document.getElementById('activeTransfers').textContent = stats.activeTransfers;
                document.getElementById('cpuUsage').textContent = stats.cpuUsage.toFixed(1) + '%';
                document.getElementById('memoryUsage').textContent = stats.memoryUsage.toFixed(1) + '%';
                
                // Update queue stats
                const queue = data.queueStats;
                document.getElementById('queuePending').textContent = queue.pendingCount;
                document.getElementById('queueProcessing').textContent = queue.processingCount;
                document.getElementById('queueCompleted').textContent = queue.completedCount;
                document.getElementById('queueFailed').textContent = queue.failedCount;
                
                const total = queue.pendingCount + queue.processingCount + queue.completedCount + queue.failedCount;
                const progress = total > 0 ? (queue.completedCount / total) * 100 : 0;
                document.getElementById('queueProgress').style.width = progress + '%';
                
                // Update agents
                this.updateAgents(data.agentStatuses);
                
                // Update recent activity
                this.updateActivity(data.recentActivity);
                
                // Update timestamp
                document.getElementById('lastUpdate').textContent = new Date(data.timestamp).toLocaleTimeString();
            }
            
            updateAgents(agents) {
                const container = document.getElementById('agentsGrid');
                container.innerHTML = '';
                
                agents.forEach(agent => {
                    const agentCard = document.createElement('div');
                    agentCard.className = 'agent-card ' + agent.status.toLowerCase();
                    
                    agentCard.innerHTML = '
                        <div class="agent-header">
                            <div class="agent-name">' + agent.hostname + '</div>
                            <div class="agent-status">' + agent.status + '</div>
                        </div>
                        <div class="metric">
                            <span>Uploads</span>
                            <span>' + agent.activeUploads + '</span>
                        </div>
                        <div class="metric">
                            <span>Downloads</span>
                            <span>' + agent.activeDownloads + '</span>
                        </div>
                        <div class="metric">
                            <span>Queue</span>
                            <span>' + agent.queueDepth + '</span>
                        </div>
                    ';
                    
                    container.appendChild(agentCard);
                });
            }
            
            updateActivity(activities) {
                const container = document.getElementById('activityList');
                container.innerHTML = '';
                
                activities.forEach(activity => {
                    const item = document.createElement('div');
                    item.className = 'activity-item';
                    
                    item.innerHTML = '
                        <div class="activity-timestamp">' + new Date(activity.timestamp).toLocaleTimeString() + '</div>
                        <div>
                            <div><strong>' + activity.action + '</strong></div>
                            <div>' + activity.description + '</div>
                        </div>
                    ';
                    
                    container.appendChild(item);
                });
            }
            
            formatBytes(bytes) {
                if (bytes === 0) return '0 B';
                const k = 1024;
                const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
                const i = Math.floor(Math.log(bytes) / Math.log(k));
                return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
            }
        }
        
        // Initialize dashboard
        new PhotonicDashboard();
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(tmpl))
}

func (ws *WebServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := ws.upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("WebSocket upgrade failed: %v\n", err)
		return
	}

	ws.hub.register <- conn

	go func() {
		defer func() {
			ws.hub.unregister <- conn
			conn.Close()
		}()

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}()
}

func (ws *WebServer) handleAPIStats(w http.ResponseWriter, r *http.Request) {
	data := ws.generateDashboardData()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (ws *WebServer) handleAPIAgents(w http.ResponseWriter, r *http.Request) {
	// TODO: Get real agent data from gRPC server
	agents := []AgentStatus{
		{
			AgentID:         "agent-001",
			Hostname:        "photo-workstation-1",
			Status:          "active",
			ActiveUploads:   3,
			ActiveDownloads: 1,
			BytesUploaded:   1024 * 1024 * 250,
			BytesDownloaded: 1024 * 1024 * 100,
			QueueDepth:      15,
			LastActivity:    time.Now(),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agents)
}

func (ws *WebServer) handleAPIQueue(w http.ResponseWriter, r *http.Request) {
	// TODO: Get real queue data from gRPC server
	queueStats := QueueStats{
		PendingCount:       45,
		ProcessingCount:    8,
		CompletedCount:     1203,
		FailedCount:        12,
		TotalBytesPending:  1024 * 1024 * 1024 * 5,
		AverageProcessTime: 15.5,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(queueStats)
}

func (ws *WebServer) generateDashboardData() DashboardData {
	// TODO: Connect to actual gRPC server for real data
	return DashboardData{
		ServerStats: ServerStats{
			ActiveAgents:     3,
			TotalConnections: 5,
			StorageUsed:      1024 * 1024 * 1024 * 450,  // 450 GB
			StorageAvailable: 1024 * 1024 * 1024 * 1550, // 1.5 TB
			CPUUsage:         25.5,
			MemoryUsage:      45.2,
			ActiveTransfers:  12,
			NetworkBandwidth: 85.3,
		},
		AgentStatuses: []AgentStatus{
			{
				AgentID:         "agent-001",
				Hostname:        "photo-workstation-1",
				Status:          "active",
				ActiveUploads:   3,
				ActiveDownloads: 1,
				BytesUploaded:   1024 * 1024 * 250,
				BytesDownloaded: 1024 * 1024 * 100,
				QueueDepth:      15,
				LastActivity:    time.Now(),
			},
			{
				AgentID:         "agent-002",
				Hostname:        "photo-workstation-2",
				Status:          "busy",
				ActiveUploads:   5,
				ActiveDownloads: 2,
				BytesUploaded:   1024 * 1024 * 500,
				BytesDownloaded: 1024 * 1024 * 200,
				QueueDepth:      25,
				LastActivity:    time.Now(),
			},
		},
		QueueStats: QueueStats{
			PendingCount:       45,
			ProcessingCount:    8,
			CompletedCount:     1203,
			FailedCount:        12,
			TotalBytesPending:  1024 * 1024 * 1024 * 5,
			AverageProcessTime: 15.5,
		},
		RecentActivity: []Activity{
			{
				Timestamp:   time.Now().Add(-2 * time.Minute),
				AgentID:     "agent-001",
				Action:      "Photo Upload",
				Description: "Uploaded IMG_2023.CR2 (25.3 MB)",
				Status:      "completed",
			},
			{
				Timestamp:   time.Now().Add(-5 * time.Minute),
				AgentID:     "agent-002",
				Action:      "Batch Sync",
				Description: "Processing batch of 50 photos",
				Status:      "processing",
			},
		},
		SystemMetrics: SystemMetrics{
			PhotosProcessed:      279891,
			TotalPhotosManaged:   279891,
			AverageUploadSpeed:   12.5,
			AverageDownloadSpeed: 25.8,
			ErrorRate:            0.05,
			UptimeSeconds:        86400 * 5, // 5 days
		},
		Timestamp: time.Now(),
	}
}

func (ws *WebServer) broadcastMetrics(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			data := ws.generateDashboardData()
			jsonData, err := json.Marshal(data)
			if err == nil {
				ws.hub.broadcast <- jsonData
			}
		}
	}
}

func (h *WebSocketHub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			fmt.Printf("WebSocket client connected. Total: %d\n", len(h.clients))

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
				fmt.Printf("WebSocket client disconnected. Total: %d\n", len(h.clients))
			}

		case message := <-h.broadcast:
			for client := range h.clients {
				if err := client.WriteMessage(websocket.TextMessage, message); err != nil {
					delete(h.clients, client)
					client.Close()
				}
			}
		}
	}
}
