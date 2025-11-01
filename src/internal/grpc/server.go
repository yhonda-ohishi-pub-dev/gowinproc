package grpc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/process"
	pb "github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/proto"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/update"
)

// Server implements the ProcessManager gRPC service
type Server struct {
	pb.UnimplementedProcessManagerServer
	processManager *process.Manager
	updateManager  *update.Manager

	// Update watchers for streaming
	watchersMu sync.RWMutex
	watchers   map[string][]chan *pb.UpdateStatus
}

// NewServer creates a new gRPC server
func NewServer(processMgr *process.Manager, updateMgr *update.Manager) *Server {
	return &Server{
		processManager: processMgr,
		updateManager:  updateMgr,
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

// GetProcess returns detailed information about a specific process
func (s *Server) GetProcess(ctx context.Context, req *pb.GetProcessRequest) (*pb.ProcessInfo, error) {
	if req.ProcessName == "" {
		return nil, fmt.Errorf("process_name is required")
	}

	instances, err := s.processManager.GetProcessStatus(req.ProcessName)
	if err != nil {
		return nil, err
	}

	// Convert instances to protobuf format
	pbInstances := make([]*pb.ProcessInstance, len(instances))
	for i, inst := range instances {
		pbInstances[i] = &pb.ProcessInstance{
			Id:          inst.ID,
			ProcessName: inst.ProcessName,
			Pid:         int32(inst.PID),
			Status:      string(inst.GetStatus()),
			StartTime:   inst.StartTime.Unix(),
			Port:        int32(inst.Port),
			EnvFilePath: inst.EnvFilePath,
			Metrics:     nil, // TODO: Implement metrics collection
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

// RestartProcess restarts a process instance or all instances
func (s *Server) RestartProcess(ctx context.Context, req *pb.RestartProcessRequest) (*pb.ProcessInfo, error) {
	if req.ProcessName == "" {
		return nil, fmt.Errorf("process_name is required")
	}

	// Stop the process(es)
	stopReq := &pb.StopProcessRequest{
		ProcessName: req.ProcessName,
		InstanceId:  req.InstanceId,
		All:         req.InstanceId == "",
	}
	if _, err := s.StopProcess(ctx, stopReq); err != nil {
		return nil, err
	}

	// Wait a bit for graceful shutdown
	time.Sleep(2 * time.Second)

	// Start new instance(s)
	// TODO: Start the same number of instances that were stopped
	_, err := s.processManager.StartProcess(req.ProcessName)
	if err != nil {
		return nil, err
	}

	// Return updated process info
	return s.GetProcess(ctx, &pb.GetProcessRequest{ProcessName: req.ProcessName})
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

	// TODO: Implement actual metrics collection
	metricsInstances := make([]*pb.ProcessMetrics, 0)
	for _, inst := range instances {
		if req.InstanceId != "" && inst.ID != req.InstanceId {
			continue
		}

		metricsInstances = append(metricsInstances, &pb.ProcessMetrics{
			InstanceId:   inst.ID,
			CpuUsage:     0,  // TODO: Collect actual metrics
			MemoryUsage:  0,  // TODO: Collect actual metrics
			DiskRead:     0,  // TODO: Collect actual metrics
			DiskWrite:    0,  // TODO: Collect actual metrics
			NetworkRecv:  0,  // TODO: Collect actual metrics
			NetworkSent:  0,  // TODO: Collect actual metrics
			Uptime:       int64(time.Since(inst.StartTime).Seconds()),
		})
	}

	return &pb.Metrics{
		ProcessName: req.ProcessName,
		Instances:   metricsInstances,
		Aggregated:  nil, // TODO: Calculate aggregated metrics
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
		// Scale down - stop instances
		for i := 0; i < currentCount-targetCount; i++ {
			if len(instances) > 0 {
				if err := s.processManager.StopProcess(req.ProcessName, instances[i].ID); err != nil {
					return nil, fmt.Errorf("failed to scale down: %w", err)
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
