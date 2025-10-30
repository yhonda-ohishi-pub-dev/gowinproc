package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/process"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/update"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/pkg/models"
)

// Server represents the REST API server
type Server struct {
	processManager *process.Manager
	updateManager  *update.Manager
	mux            *http.ServeMux
}

// NewServer creates a new API server
func NewServer(processMgr *process.Manager, updateMgr *update.Manager) *Server {
	s := &Server{
		processManager: processMgr,
		updateManager:  updateMgr,
		mux:            http.NewServeMux(),
	}

	s.registerRoutes()
	return s
}

// ServeHTTP implements http.Handler interface
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// registerRoutes registers all API routes
func (s *Server) registerRoutes() {
	// Process management
	s.mux.HandleFunc("/api/v1/processes", s.handleListProcesses)
	s.mux.HandleFunc("/api/v1/processes/", s.handleProcessRoute)

	// Health check
	s.mux.HandleFunc("/health", s.handleHealth)
}

// handleListProcesses handles GET /api/v1/processes
func (s *Server) handleListProcesses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	processes := s.processManager.ListProcesses()

	response := map[string]interface{}{
		"processes": processes,
		"count":     len(processes),
	}

	s.writeJSON(w, http.StatusOK, response)
}

// handleProcessRoute handles routes for specific processes
func (s *Server) handleProcessRoute(w http.ResponseWriter, r *http.Request) {
	// Extract process name from path
	// Path format: /api/v1/processes/{name}/{action}
	path := r.URL.Path[len("/api/v1/processes/"):]

	if path == "" {
		s.handleListProcesses(w, r)
		return
	}

	// Parse path segments
	var processName, action string
	fmt.Sscanf(path, "%s/%s", &processName, &action)

	if processName == "" {
		s.writeError(w, http.StatusBadRequest, "process name is required")
		return
	}

	switch action {
	case "status":
		s.handleProcessStatus(w, r, processName)
	case "start":
		s.handleProcessStart(w, r, processName)
	case "stop":
		s.handleProcessStop(w, r, processName)
	case "update":
		s.handleProcessUpdate(w, r, processName)
	case "version":
		s.handleProcessVersion(w, r, processName)
	case "rollback":
		s.handleProcessRollback(w, r, processName)
	default:
		s.writeError(w, http.StatusNotFound, "action not found")
	}
}

// handleProcessStatus handles GET /api/v1/processes/{name}/status
func (s *Server) handleProcessStatus(w http.ResponseWriter, r *http.Request, processName string) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	instances, err := s.processManager.GetProcessStatus(processName)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	response := map[string]interface{}{
		"process":   processName,
		"instances": instances,
		"count":     len(instances),
	}

	s.writeJSON(w, http.StatusOK, response)
}

// handleProcessStart handles POST /api/v1/processes/{name}/start
func (s *Server) handleProcessStart(w http.ResponseWriter, r *http.Request, processName string) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	instance, err := s.processManager.StartProcess(processName)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to start process: %v", err))
		return
	}

	response := map[string]interface{}{
		"message":  "process started successfully",
		"instance": instance,
	}

	s.writeJSON(w, http.StatusOK, response)
}

// handleProcessStop handles POST /api/v1/processes/{name}/stop
func (s *Server) handleProcessStop(w http.ResponseWriter, r *http.Request, processName string) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Parse request body for instance ID
	var req struct {
		InstanceID string `json:"instance_id"`
		All        bool   `json:"all"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.All {
		if err := s.processManager.StopAllInstances(processName); err != nil {
			s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to stop processes: %v", err))
			return
		}
	} else if req.InstanceID != "" {
		if err := s.processManager.StopProcess(processName, req.InstanceID); err != nil {
			s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to stop process: %v", err))
			return
		}
	} else {
		s.writeError(w, http.StatusBadRequest, "either instance_id or all must be specified")
		return
	}

	response := map[string]interface{}{
		"message": "process stopped successfully",
	}

	s.writeJSON(w, http.StatusOK, response)
}

// handleProcessUpdate handles POST /api/v1/processes/{name}/update
func (s *Server) handleProcessUpdate(w http.ResponseWriter, r *http.Request, processName string) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req models.UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// If body is empty, use defaults
		req = models.UpdateRequest{
			ProcessName: processName,
			Force:       false,
		}
	} else {
		req.ProcessName = processName
	}

	if err := s.updateManager.UpdateProcess(req.ProcessName, req.Version, req.Force); err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to start update: %v", err))
		return
	}

	response := map[string]interface{}{
		"message": "update started",
		"process": processName,
	}

	s.writeJSON(w, http.StatusAccepted, response)
}

// handleProcessVersion handles GET /api/v1/processes/{name}/version
func (s *Server) handleProcessVersion(w http.ResponseWriter, r *http.Request, processName string) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Check for update status
	if status, exists := s.updateManager.GetUpdateStatus(processName); exists {
		response := map[string]interface{}{
			"process":       processName,
			"update_status": status,
		}
		s.writeJSON(w, http.StatusOK, response)
		return
	}

	// Return version info
	// TODO: Implement full version info retrieval
	response := map[string]interface{}{
		"process": processName,
		"message": "no update in progress",
	}

	s.writeJSON(w, http.StatusOK, response)
}

// handleProcessRollback handles POST /api/v1/processes/{name}/rollback
func (s *Server) handleProcessRollback(w http.ResponseWriter, r *http.Request, processName string) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req models.RollbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// If body is empty, rollback to previous version
		req = models.RollbackRequest{
			ProcessName: processName,
		}
	} else {
		req.ProcessName = processName
	}

	if err := s.updateManager.RollbackProcess(req.ProcessName, req.Version); err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to rollback: %v", err))
		return
	}

	response := map[string]interface{}{
		"message": "rollback started",
		"process": processName,
	}

	s.writeJSON(w, http.StatusAccepted, response)
}

// handleHealth handles GET /health
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	response := map[string]interface{}{
		"status": "healthy",
	}

	s.writeJSON(w, http.StatusOK, response)
}

// writeJSON writes a JSON response
func (s *Server) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError writes an error response
func (s *Server) writeError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, map[string]interface{}{
		"error": message,
	})
}
