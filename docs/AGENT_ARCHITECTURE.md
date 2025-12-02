# Photonic Distributed Agent Architecture

## 🎯 Vision: Industry-Grade Distributed Photo Management

Transform photonic from a single-machine tool into a **distributed photo ecosystem** that provides seamless photo management across multiple machines with intelligent buffering, synchronization, and processing capabilities.

## 🏗️ Architecture Overview

```
┌─────────────────────────────────────┐    ┌──────────────────────────────────────┐
│           AGENT NODES               │    │            SERVER NODE               │
│  ┌─────────────────────────────┐    │    │  ┌─────────────────────────────────┐ │
│  │        YODA (Mobile)        │    │    │  │         JMH (Central)           │ │
│  │ ┌─────────────────────────┐ │    │◄──►│  │ ┌─────────────────────────────┐ │ │
│  │ │   Local Buffer Queue    │ │    │    │  │ │    Master Photo Store       │ │ │
│  │ │       1.7TB             │ │    │    │  │ │         6TB+                │ │ │
│  │ │                         │ │    │    │  │ │                             │ │ │
│  │ │ - New photo detection   │ │    │    │  │ │ - Darktable integration     │ │ │
│  │ │ - Metadata extraction   │ │    │    │  │ │ - NFS export                │ │ │
│  │ │ - Conflict resolution   │ │    │    │  │ │ - Processing pipelines      │ │ │
│  │ │ - Sync queue management │ │    │    │  │ │ - REST API                  │ │ │
│  │ │ - Intelligent batching  │ │    │    │  │ │ - WebSocket real-time       │ │ │
│  │ └─────────────────────────┘ │    │    │  │ └─────────────────────────────┘ │ │
│  └─────────────────────────────┘    │    │  └─────────────────────────────────┘ │
│                                     │    │                                      │
│  ┌─────────────────────────────┐    │    │  ┌─────────────────────────────────┐ │
│  │     ASCENDER (Static)       │    │    │  │       Management Web UI         │ │
│  │ ┌─────────────────────────┐ │    │    │  │ ┌─────────────────────────────┐ │ │
│  │ │   Archive Storage       │ │    │    │  │ │ - Agent status dashboard    │ │ │
│  │ │       4TB               │ │    │    │  │ │ - Sync progress monitoring  │ │ │
│  │ │                         │ │    │    │  │ │ - Queue management          │ │ │
│  │ │ - Long-term storage     │ │    │    │  │ │ - Conflict resolution UI    │ │ │
│  │ │ - Backup operations     │ │    │    │  │ │ - Network topology view     │ │ │
│  │ │ - Archive retrieval     │ │    │    │  │ └─────────────────────────────┘ │ │
│  │ └─────────────────────────┘ │    │    │  └─────────────────────────────────┘ │
│  └─────────────────────────────┘    │    └──────────────────────────────────────┘
└─────────────────────────────────────┘

    Communication Protocols:
    ═══════════════════════════════
    📡 gRPC: High-performance binary protocol for sync operations
    🌐 WebSocket: Real-time status updates and monitoring
    📁 NFS: Transparent file access when on local network
    🔐 mTLS: Mutual authentication and encryption
```

## 🎯 Core Components

### 1. **Photonic Agent** (`photonic agent`)
- **Autonomous operation** when disconnected from server
- **Intelligent photo detection** using filesystem watchers
- **Metadata extraction** and local processing
- **Queue management** with priority and batching
- **Conflict resolution** using content hashing and EXIF data
- **Network-aware synchronization** (bandwidth detection, retry logic)

### 2. **Photonic Server** (`photonic server`)
- **Central coordination** of all agent nodes
- **Master photo repository** with deduplication
- **NFS export** for transparent access
- **Processing orchestration** (panoramic, timelapse, etc.)
- **Web dashboard** for monitoring and management

### 3. **Sync Protocol** (gRPC-based)
- **Binary efficient** for large photo transfers
- **Resumable transfers** with chunking
- **Delta synchronization** (only changed files)
- **Compression** for network efficiency
- **Parallel streams** for maximum throughput

## 🛠️ Technical Implementation

### Agent State Machine
```go
type AgentState int

const (
    OFFLINE     AgentState = iota // Disconnected from server
    CONNECTING                    // Attempting connection
    SYNCING                       // Active synchronization
    MONITORING                    // Watching for new photos
    CONFLICT                      // Requires user intervention
    ERROR                         // Error state requiring attention
)
```

### Queue Management
```go
type SyncQueue struct {
    PendingPhotos   []PhotoItem
    InProgressBatch SyncBatch
    CompletedItems  []PhotoItem
    ConflictItems   []ConflictPhoto
    RetryQueue      []PhotoItem
    
    BatchSize       int           // Photos per batch
    MaxConcurrency  int           // Parallel transfers
    BandwidthLimit  int64         // bytes/sec
    RetryPolicy     RetryConfig   // Exponential backoff
}
```

### Server Discovery & Registration
```go
type AgentRegistration struct {
    AgentID         string        // Unique agent identifier
    Hostname        string        // Agent machine name
    StorageCapacity int64         // Available buffer space
    NetworkInfo     NetworkConfig // Connection details
    Capabilities    []string      // Supported operations
    LastSeen        time.Time     // Health monitoring
}
```

## 🚀 Advanced Features

### 1. **Intelligent Batching**
- **EXIF-aware grouping**: Batch photos from same shoot/day
- **Size optimization**: Balance transfer efficiency vs memory
- **Priority queuing**: Recent photos sync first
- **Bandwidth adaptation**: Adjust batch size based on network speed

### 2. **Conflict Resolution**
- **Content-based deduplication**: SHA-256 hashing
- **EXIF timestamp analysis**: Choose best version
- **Manual resolution interface**: Web UI for user decisions
- **Version preservation**: Keep multiple versions when uncertain

### 3. **Network Optimization**
- **Connection pooling**: Reuse gRPC connections
- **Compression**: LZ4 for photos, gzip for metadata
- **Adaptive quality**: Reduce transfer quality on slow networks
- **Resume capability**: Continue interrupted transfers

### 4. **Security Model**
- **mTLS authentication**: Certificate-based agent identity
- **Token-based authorization**: Scoped permissions per agent
- **Encrypted storage**: AES-256 for sensitive metadata
- **Audit logging**: Complete sync operation history

## 🎮 Operation Modes

### **Home Mode** (Local Network)
```bash
# Server provides NFS export
# Agent operates in pass-through mode
# Real-time sync with minimal queuing
# Direct darktable access via NFS mount
```

### **Roaming Mode** (Disconnected)
```bash
# Agent buffers all new photos locally
# Metadata extraction and organization
# Conflict pre-resolution 
# Intelligent queue building
```

### **Sync Mode** (Connected)
```bash
# High-throughput batch transfer
# Parallel stream processing
# Real-time progress reporting
# Automatic conflict resolution
```

### **Archive Mode** (Long-term Storage)
```bash
# Automated backup to secondary nodes
# Checksum verification
# Redundant storage management
# Retrieval on demand
```

## 📡 Communication Protocols

### gRPC Service Definitions

```protobuf
service PhotonicSync {
    // Agent registration and heartbeat
    rpc RegisterAgent(AgentInfo) returns (RegistrationResponse);
    rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse);
    
    // Photo synchronization
    rpc SyncBatch(stream PhotoBatch) returns (stream SyncProgress);
    rpc ResolveConflicts(ConflictResolution) returns (ConflictResult);
    
    // Queue management
    rpc GetSyncStatus(StatusRequest) returns (SyncStatus);
    rpc PauseSyncing(PauseRequest) returns (PauseResponse);
    
    // Real-time updates
    rpc StreamEvents(EventFilter) returns (stream SystemEvent);
}
```

### WebSocket API for Real-time Dashboard
```go
type WebSocketMessage struct {
    Type      string    `json:"type"`      // "agent_status", "sync_progress", "conflict"
    Timestamp time.Time `json:"timestamp"`
    AgentID   string    `json:"agent_id"`
    Data      any       `json:"data"`
}
```

## 🔧 Configuration System

### Agent Configuration (`agent.yaml`)
```yaml
agent:
  id: "yoda-mobile-01"
  server_endpoints: ["jmh.local:8082", "10.227.198.120:8082"]
  buffer_path: "/data/photonic/buffer"
  max_buffer_size: "1.5TB"
  
network:
  max_bandwidth: "100MB/s"
  compression: true
  parallel_transfers: 4
  
sync:
  batch_size: 50
  retry_attempts: 3
  conflict_resolution: "auto" # auto, manual, server_wins
  
security:
  cert_path: "/etc/photonic/agent.crt"
  key_path: "/etc/photonic/agent.key"
  ca_path: "/etc/photonic/ca.crt"
```

### Server Configuration (`server.yaml`)
```yaml
server:
  bind_address: "0.0.0.0:8082"
  storage_path: "/data/Photography"
  nfs_export: true
  web_ui_port: 8083
  
storage:
  deduplication: true
  compression: "lz4"
  backup_nodes: ["ascender.local"]
  
limits:
  max_agents: 10
  max_concurrent_syncs: 5
  max_transfer_size: "10GB"
```

## 🎯 Performance Targets

### Throughput Goals
- **Local Network**: 1GB/s+ sustained transfer rates
- **Internet**: Adaptive to available bandwidth with QoS
- **Concurrent Operations**: 100+ photos processing simultaneously
- **Queue Processing**: 1000+ photos/minute during sync

### Reliability Goals
- **99.9% uptime** for server components
- **Zero data loss** during transfers
- **<5 second** conflict detection and reporting
- **<1 minute** for automatic failover and recovery

## 🚀 Implementation Roadmap

### Phase 1: Core Agent Framework
1. **Agent binary**: Basic sync queue and server communication
2. **gRPC protocol**: Core sync operations
3. **Configuration system**: YAML-based setup
4. **Basic web UI**: Agent status dashboard

### Phase 2: Advanced Sync Features  
1. **Intelligent batching**: EXIF-aware grouping
2. **Conflict resolution**: Automated and manual modes
3. **Network optimization**: Compression and adaptive quality
4. **Security implementation**: mTLS and encryption

### Phase 3: Production Features
1. **High availability**: Multi-server support
2. **Monitoring & alerting**: Prometheus metrics
3. **Backup orchestration**: Multi-node redundancy
4. **Performance optimization**: Zero-copy operations

### Phase 4: Enterprise Features
1. **Multi-tenant support**: Organization isolation
2. **API versioning**: Backward compatibility
3. **Plugin architecture**: Custom processing pipelines
4. **Cloud integration**: AWS S3, Azure Blob storage

## 🏭 Industry-Grade Features

### Observability & Monitoring
```go
// Prometheus metrics
var (
    photosProcessed = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "photonic_photos_processed_total",
            Help: "Total number of photos processed",
        },
        []string{"agent_id", "operation"},
    )
    
    syncLatency = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "photonic_sync_duration_seconds",
            Help: "Sync operation duration",
        },
        []string{"agent_id"},
    )
)
```

### Health Checks & Circuit Breakers
```go
type HealthCheck struct {
    DiskSpace     bool `json:"disk_space"`
    NetworkConn   bool `json:"network_connectivity"`  
    ServerReach   bool `json:"server_reachable"`
    QueueSize     int  `json:"queue_size"`
    LastSyncTime  time.Time `json:"last_sync"`
}
```

### Graceful Degradation
- **Partial connectivity**: Continue with reduced functionality
- **Storage pressure**: Automatic cleanup of old processed files
- **Network issues**: Intelligent retry with exponential backoff
- **Resource constraints**: Dynamic adjustment of concurrency

## 🎯 Success Metrics

This architecture will create a **professional-grade distributed photo management system** that:

1. **Scales effortlessly** from hobbyist to professional workflows
2. **Handles network partitions** gracefully with intelligent queuing
3. **Provides enterprise reliability** with comprehensive monitoring
4. **Offers seamless user experience** with transparent file access
5. **Ensures data integrity** with cryptographic verification

The result: **A photo management system that rivals commercial enterprise solutions** while remaining open-source and self-hosted! 🚀

---

## 🔧 Next Steps

1. **Implement core agent framework** with gRPC communication
2. **Build sync queue management** with intelligent batching
3. **Create web dashboard** for monitoring and control
4. **Test with real photo workloads** across multiple machines
5. **Optimize for performance** and reliability

This architecture transforms photonic from a tool into a **platform** - the foundation for distributed creative workflows! 💫