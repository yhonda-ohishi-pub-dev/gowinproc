package grpc

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	gopsutilProcess "github.com/shirou/gopsutil/v4/process"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/process"
	pb "github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/proto"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/update"
)

// Server implements the ProcessManager gRPC service
type Server struct {
	pb.UnimplementedProcessManagerServer
	processManager *process.Manager
	updateManager  *update.Manager
	repositories   []string

	// Update watchers for streaming
	watchersMu sync.RWMutex
	watchers   map[string][]chan *pb.UpdateStatus
}

// NewServer creates a new gRPC server
func NewServer(processMgr *process.Manager, updateMgr *update.Manager, repos []string) *Server {
	return &Server{
		processManager: processMgr,
		updateManager:  updateMgr,
		repositories:   repos,
		watchers:       make(map[string][]chan *pb.UpdateStatus),
	}
}

// ListProcesses returns a list of all managed processes
func (s *Server) ListProcesses(ctx context.Context, req *pb.ListProcessesRequest) (*pb.ListProcessesResponse, error) {
	processes := s.processManager.ListProcesses()

	return &pb.ListProcessesResponse{
		ProcessNames: processes,
		Count:        int32(len(processes)),
	}, nil
}

// collectMetrics collects CPU and memory metrics for a process
func collectMetrics(pid int) *pb.ProcessMetrics {
	proc, err := gopsutilProcess.NewProcess(int32(pid))
	if err != nil {
		return nil
	}

	cpuPercent, _ := proc.CPUPercent()
	memInfo, _ := proc.MemoryInfo()

	var memoryUsage uint64
	if memInfo != nil {
		memoryUsage = memInfo.RSS
	}

	return &pb.ProcessMetrics{
		InstanceId:  "", // Will be set by caller
		CpuUsage:    cpuPercent,
		MemoryUsage: memoryUsage,
		DiskRead:    0, // TODO: Implement disk metrics
		DiskWrite:   0,
		NetworkRecv: 0, // TODO: Implement network metrics
		NetworkSent: 0,
		Uptime:      0, // Will be set by caller
	}
}

// GetProcess returns detailed information about a specific process
func (s *Server) GetProcess(ctx context.Context, req *pb.GetProcessRequest) (*pb.ProcessInfo, error) {
	if req.ProcessName == "" {
		return nil, fmt.Errorf("process_name is required")
	}

	instances, err := s.processManager.GetProcessStatus(req.ProcessName)
	if err != nil {
		return nil, err
	}

	// Log instance count for debugging restart behavior
	log.Printf("[GetProcess] %s: Returning %d instances", req.ProcessName, len(instances))
	for _, inst := range instances {
		log.Printf("[GetProcess]   Instance %s: PID=%d, Status=%s", inst.ID, inst.PID, inst.GetStatus())
	}

	// Convert instances to protobuf format
	pbInstances := make([]*pb.ProcessInstance, len(instances))
	for i, inst := range instances {
		// Collect metrics for this instance
		metrics := collectMetrics(inst.PID)
		if metrics != nil {
			metrics.InstanceId = inst.ID
			metrics.Uptime = int64(time.Since(inst.StartTime).Seconds())
		}

		pbInstances[i] = &pb.ProcessInstance{
			Id:          inst.ID,
			ProcessName: inst.ProcessName,
			Pid:         int32(inst.PID),
			Status:      string(inst.GetStatus()),
			StartTime:   inst.StartTime.Unix(),
			Port:        int32(inst.Port),
			EnvFilePath: inst.EnvFilePath,
			Metrics:     metrics,
		}
	}

	// Get process config
	// TODO: Add method to get process config from manager
	config := &pb.ProcessConfig{
		Name: req.ProcessName,
		// Fill in other fields from actual config
	}

	return &pb.ProcessInfo{
		Name:          req.ProcessName,
		Instances:     pbInstances,
		InstanceCount: int32(len(instances)),
		Config:        config,
	}, nil
}

// StartProcess starts a new instance of a process
func (s *Server) StartProcess(ctx context.Context, req *pb.StartProcessRequest) (*pb.ProcessInfo, error) {
	if req.ProcessName == "" {
		return nil, fmt.Errorf("process_name is required")
	}

	_, err := s.processManager.StartProcess(req.ProcessName)
	if err != nil {
		return nil, err
	}

	// Return updated process info
	return s.GetProcess(ctx, &pb.GetProcessRequest{ProcessName: req.ProcessName})
}

// StopProcess stops a process instance or all instances
func (s *Server) StopProcess(ctx context.Context, req *pb.StopProcessRequest) (*pb.Empty, error) {
	if req.ProcessName == "" {
		return nil, fmt.Errorf("process_name is required")
	}

	if req.All || req.InstanceId == "" {
		// Stop all instances
		if err := s.processManager.StopAllInstances(req.ProcessName); err != nil {
			return nil, err
		}
	} else {
		// Stop specific instance
		if err := s.processManager.StopProcess(req.ProcessName, req.InstanceId); err != nil {
			return nil, err
		}
	}

	return &pb.Empty{}, nil
}

// RestartProcess restarts a process instance or all instances using hot restart
// (start new instances first, then stop old ones to minimize downtime)
func (s *Server) RestartProcess(ctx context.Context, req *pb.RestartProcessRequest) (*pb.ProcessInfo, error) {
	if req.ProcessName == "" {
		return nil, fmt.Errorf("process_name is required")
	}

	// Get current instances before restart
	oldInstances, err := s.processManager.GetProcessStatus(req.ProcessName)
	if err != nil {
		return nil, fmt.Errorf("failed to get current instances: %w", err)
	}

	if len(oldInstances) == 0 {
		return nil, fmt.Errorf("no instances found for process %s", req.ProcessName)
	}

	log.Printf("[HOT RESTART] Starting hot restart for %s: %d old instances", req.ProcessName, len(oldInstances))
	for _, inst := range oldInstances {
		log.Printf("[HOT RESTART]   Old instance: %s (PID: %d)", inst.ID, inst.PID)
	}

	// HOT RESTART: Start new instance(s) first
	numInstances := len(oldInstances)
	if req.InstanceId != "" {
		numInstances = 1 // Only restart one instance
	}

	newInstanceIDs := make([]string, 0, numInstances)
	for i := 0; i < numInstances; i++ {
		instance, err := s.processManager.StartProcess(req.ProcessName)
		if err != nil {
			// Rollback: stop newly started instances
			for _, id := range newInstanceIDs {
				s.processManager.StopProcess(req.ProcessName, id)
			}
			return nil, fmt.Errorf("failed to start new instance %d: %w", i+1, err)
		}
		newInstanceIDs = append(newInstanceIDs, instance.ID)
		log.Printf("[HOT RESTART] Started new instance %d/%d: %s (PID: %d)", i+1, numInstances, instance.ID, instance.PID)
	}

	// Wait for new instances to be ready and verify they are actually running
	maxRetries := 10
	retryDelay := 500 * time.Millisecond
	minWaitTime := 1 * time.Second // Minimum wait time to ensure UI can see the transition state

	log.Printf("[HOT RESTART] Starting health check for new instances (max %d retries, min wait time: %v)", maxRetries, minWaitTime)
	healthCheckStart := time.Now()

	for retry := 0; retry < maxRetries; retry++ {
		time.Sleep(retryDelay)

		// Check if all new instances are running and healthy
		allHealthy := true
		newInstances, err := s.processManager.GetProcessStatus(req.ProcessName)
		if err != nil {
			log.Printf("[HOT RESTART] Retry %d/%d: Failed to get process status: %v", retry+1, maxRetries, err)
			continue
		}

		log.Printf("[HOT RESTART] Retry %d/%d: Total instances found: %d (old + new)", retry+1, maxRetries, len(newInstances))
		for _, inst := range newInstances {
			log.Printf("[HOT RESTART]   Instance %s: PID=%d, Status=%s", inst.ID, inst.PID, inst.GetStatus())
		}

		// Verify each new instance is present and running
		for _, newID := range newInstanceIDs {
			found := false
			for _, inst := range newInstances {
				if inst.ID == newID {
					found = true
					// Check if process is actually running (PID > 0)
					if inst.PID <= 0 {
						log.Printf("[HOT RESTART] Retry %d/%d: Instance %s has invalid PID: %d", retry+1, maxRetries, newID, inst.PID)
						allHealthy = false
						break
					}
					// Process must be in running state
					if inst.GetStatus() != "running" {
						log.Printf("[HOT RESTART] Retry %d/%d: Instance %s not running: %s", retry+1, maxRetries, newID, inst.GetStatus())
						allHealthy = false
						break
					}
				}
			}
			if !found {
				log.Printf("[HOT RESTART] Retry %d/%d: New instance %s not found in status", retry+1, maxRetries, newID)
				allHealthy = false
				break
			}
		}

		if allHealthy {
			// Check if minimum wait time has elapsed
			elapsed := time.Since(healthCheckStart)
			if elapsed < minWaitTime {
				remainingWait := minWaitTime - elapsed
				log.Printf("[HOT RESTART] New instances are healthy after %d retries, but waiting additional %v for UI visibility (total wait: %v)", retry+1, remainingWait, minWaitTime)
				time.Sleep(remainingWait)
			} else {
				log.Printf("[HOT RESTART] All new instances are healthy after %d retries (elapsed: %v)", retry+1, elapsed)
			}
			break
		}

		// If this is the last retry and still not healthy, rollback
		if retry == maxRetries-1 {
			log.Printf("[HOT RESTART] Health check failed after %d retries, rolling back", maxRetries)
			for _, id := range newInstanceIDs {
				s.processManager.StopProcess(req.ProcessName, id)
			}
			return nil, fmt.Errorf("new instances failed to become healthy after %d retries", maxRetries)
		}
	}

	// Get final status of all instances
	newInstances, err := s.processManager.GetProcessStatus(req.ProcessName)
	if err != nil {
		return nil, fmt.Errorf("failed to verify new instances: %w", err)
	}

	newInstanceMap := make(map[string]bool)
	for _, inst := range newInstances {
		newInstanceMap[inst.ID] = true
	}

	// Stop old instance(s) gracefully
	log.Printf("[HOT RESTART] Stopping old instances...")
	for _, oldInst := range oldInstances {
		// Skip if this is a newly started instance
		if newInstanceMap[oldInst.ID] && contains(newInstanceIDs, oldInst.ID) {
			log.Printf("[HOT RESTART] Skipping new instance %s (not an old instance)", oldInst.ID)
			continue
		}

		// If specific instance requested, only stop that one
		if req.InstanceId != "" && oldInst.ID != req.InstanceId {
			log.Printf("[HOT RESTART] Skipping instance %s (not the requested instance)", oldInst.ID)
			continue
		}

		// Stop old instance with graceful timeout (reduced to 3s for faster response)
		log.Printf("[HOT RESTART] Stopping old instance %s (PID: %d)", oldInst.ID, oldInst.PID)
		if err := s.processManager.StopProcessGracefully(req.ProcessName, oldInst.ID, 3*time.Second); err != nil {
			// Log error but continue - new instances are already running
			log.Printf("[HOT RESTART] Warning: failed to stop old instance %s: %v", oldInst.ID, err)
		} else {
			log.Printf("[HOT RESTART] Successfully stopped old instance %s", oldInst.ID)
		}
	}

	log.Printf("[HOT RESTART] Hot restart completed for %s", req.ProcessName)

	// Return updated process info
	return s.GetProcess(ctx, &pb.GetProcessRequest{ProcessName: req.ProcessName})
}

// Helper function to check if slice contains string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// GetMetrics returns metrics for a process
func (s *Server) GetMetrics(ctx context.Context, req *pb.GetMetricsRequest) (*pb.Metrics, error) {
	if req.ProcessName == "" {
		return nil, fmt.Errorf("process_name is required")
	}

	instances, err := s.processManager.GetProcessStatus(req.ProcessName)
	if err != nil {
		return nil, err
	}

	// Collect metrics for each instance
	metricsInstances := make([]*pb.ProcessMetrics, 0)
	var totalCPU float64
	var totalMemory uint64

	for _, inst := range instances {
		if req.InstanceId != "" && inst.ID != req.InstanceId {
			continue
		}

		metrics := collectMetrics(inst.PID)
		if metrics != nil {
			metrics.InstanceId = inst.ID
			metrics.Uptime = int64(time.Since(inst.StartTime).Seconds())

			metricsInstances = append(metricsInstances, metrics)
			totalCPU += metrics.CpuUsage
			totalMemory += metrics.MemoryUsage
		}
	}

	// Calculate aggregated metrics
	aggregated := &pb.AggregatedMetrics{
		TotalCpuUsage:     totalCPU,
		TotalMemoryUsage:  totalMemory,
		TotalDiskRead:     0,
		TotalDiskWrite:    0,
		TotalNetworkRecv:  0,
		TotalNetworkSent:  0,
		InstanceCount:     int32(len(metricsInstances)),
	}

	return &pb.Metrics{
		ProcessName: req.ProcessName,
		Instances:   metricsInstances,
		Aggregated:  aggregated,
	}, nil
}

// ScaleProcess scales a process to a target number of instances
func (s *Server) ScaleProcess(ctx context.Context, req *pb.ScaleProcessRequest) (*pb.ProcessInfo, error) {
	if req.ProcessName == "" {
		return nil, fmt.Errorf("process_name is required")
	}

	instances, err := s.processManager.GetProcessStatus(req.ProcessName)
	if err != nil {
		return nil, err
	}

	currentCount := len(instances)
	targetCount := int(req.TargetInstances)

	if targetCount > currentCount {
		// Scale up - start more instances
		for i := 0; i < targetCount-currentCount; i++ {
			if _, err := s.processManager.StartProcess(req.ProcessName); err != nil {
				return nil, fmt.Errorf("failed to scale up: %w", err)
			}
		}
	} else if targetCount < currentCount {
		// Scale down - stop instances gracefully
		instancesToStop := currentCount - targetCount
		for i := 0; i < instancesToStop; i++ {
			// Get fresh instance list each time
			currentInstances, err := s.processManager.GetProcessStatus(req.ProcessName)
			if err != nil {
				return nil, fmt.Errorf("failed to get instances during scale down: %w", err)
			}

			if len(currentInstances) > targetCount {
				// Stop the last instance gracefully
				lastIdx := len(currentInstances) - 1
				if err := s.processManager.StopProcessGracefully(req.ProcessName, currentInstances[lastIdx].ID, 10*time.Second); err != nil {
					return nil, fmt.Errorf("failed to scale down instance %s: %w", currentInstances[lastIdx].ID, err)
				}
			}
		}
	}

	// Return updated process info
	return s.GetProcess(ctx, &pb.GetProcessRequest{ProcessName: req.ProcessName})
}

// UpdateAllProcesses updates all processes
func (s *Server) UpdateAllProcesses(ctx context.Context, req *pb.UpdateAllRequest) (*pb.UpdateResponse, error) {
	// TODO: Implement update all processes
	return &pb.UpdateResponse{
		Success: false,
		Message: "UpdateAllProcesses not yet implemented",
	}, nil
}

// UpdateProcess updates a specific process
func (s *Server) UpdateProcess(ctx context.Context, req *pb.UpdateProcessRequest) (*pb.UpdateResponse, error) {
	if req.ProcessName == "" {
		return nil, fmt.Errorf("process_name is required")
	}

	err := s.updateManager.UpdateProcess(req.ProcessName, req.Version, req.Force)
	if err != nil {
		return &pb.UpdateResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.UpdateResponse{
		Success: true,
		Message: "Update started successfully",
		UpdateId: fmt.Sprintf("upd_%s_%d", req.ProcessName, time.Now().Unix()),
		Processes: []*pb.ProcessUpdateStatus{
			{
				Name:          req.ProcessName,
				CurrentVersion: "unknown", // TODO: Get current version
				TargetVersion:  req.Version,
				Status:        "updating",
			},
		},
	}, nil
}

// GetProcessVersion returns version information for a process
func (s *Server) GetProcessVersion(ctx context.Context, req *pb.GetVersionRequest) (*pb.VersionInfo, error) {
	if req.ProcessName == "" {
		return nil, fmt.Errorf("process_name is required")
	}

	instances, err := s.processManager.GetProcessStatus(req.ProcessName)
	if err != nil {
		return nil, err
	}

	instanceVersions := make([]*pb.InstanceVersion, len(instances))
	for i, inst := range instances {
		instanceVersions[i] = &pb.InstanceVersion{
			Id:      inst.ID,
			Version: "unknown", // TODO: Track version per instance
			Uptime:  int64(time.Since(inst.StartTime).Seconds()),
		}
	}

	return &pb.VersionInfo{
		ProcessName:     req.ProcessName,
		CurrentVersion:  "unknown", // TODO: Implement version tracking
		LatestVersion:   "unknown", // TODO: Get from update manager
		UpdateAvailable: false,     // TODO: Check if update available
		Instances:       instanceVersions,
	}, nil
}

// ListAvailableUpdates returns a list of available updates
func (s *Server) ListAvailableUpdates(ctx context.Context, req *pb.ListUpdatesRequest) (*pb.ListUpdatesResponse, error) {
	// TODO: Implement list available updates
	return &pb.ListUpdatesResponse{
		Updates: []*pb.UpdateAvailable{},
	}, nil
}

// RollbackProcess rolls back a process to a previous version
func (s *Server) RollbackProcess(ctx context.Context, req *pb.RollbackRequest) (*pb.RollbackResponse, error) {
	if req.ProcessName == "" {
		return nil, fmt.Errorf("process_name is required")
	}

	err := s.updateManager.RollbackProcess(req.ProcessName, req.Version)
	if err != nil {
		return &pb.RollbackResponse{
			Success: false,
		}, err
	}

	return &pb.RollbackResponse{
		Success:     true,
		ProcessName: req.ProcessName,
		FromVersion: "unknown", // TODO: Track versions
		ToVersion:   req.Version,
		RollbackId:  fmt.Sprintf("rbk_%s_%d", req.ProcessName, time.Now().Unix()),
	}, nil
}

// WatchUpdate streams update progress for a specific update
func (s *Server) WatchUpdate(req *pb.WatchUpdateRequest, stream pb.ProcessManager_WatchUpdateServer) error {
	if req.UpdateId == "" {
		return fmt.Errorf("update_id is required")
	}

	// Create a channel for this watcher
	ch := make(chan *pb.UpdateStatus, 10)

	// Register watcher
	s.watchersMu.Lock()
	if s.watchers[req.UpdateId] == nil {
		s.watchers[req.UpdateId] = make([]chan *pb.UpdateStatus, 0)
	}
	s.watchers[req.UpdateId] = append(s.watchers[req.UpdateId], ch)
	s.watchersMu.Unlock()

	// Cleanup on exit
	defer func() {
		s.watchersMu.Lock()
		watchers := s.watchers[req.UpdateId]
		for i, w := range watchers {
			if w == ch {
				s.watchers[req.UpdateId] = append(watchers[:i], watchers[i+1:]...)
				break
			}
		}
		s.watchersMu.Unlock()
		close(ch)
	}()

	// Stream updates to client
	for {
		select {
		case status, ok := <-ch:
			if !ok {
				return nil
			}
			if err := stream.Send(status); err != nil {
				return err
			}
			// If update is completed or failed, close stream
			if status.Status == "completed" || status.Status == "failed" {
				return nil
			}
		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}

// BroadcastUpdateStatus broadcasts an update status to all watchers
func (s *Server) BroadcastUpdateStatus(updateID string, status *pb.UpdateStatus) {
	s.watchersMu.RLock()
	defer s.watchersMu.RUnlock()

	watchers := s.watchers[updateID]
	for _, ch := range watchers {
		select {
		case ch <- status:
		default:
			// Channel full, skip this update
		}
	}
}

// ListRepositories returns the list of monitored repositories
func (s *Server) ListRepositories(ctx context.Context, req *pb.ListRepositoriesRequest) (*pb.ListRepositoriesResponse, error) {
	return &pb.ListRepositoriesResponse{
		Repositories: s.repositories,
		Count:        int32(len(s.repositories)),
	}, nil
}
