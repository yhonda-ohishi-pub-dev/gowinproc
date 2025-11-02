package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"time"

	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/process"
)

// InvokeRequest represents a gRPC method invocation request
type InvokeRequest struct {
	Process string                 `json:"process"` // Process name (e.g., "db_service_local")
	Service string                 `json:"service"` // Service name (e.g., "db_service.db_ChiikiMasterService")
	Method  string                 `json:"method"`  // Method name (e.g., "List")
	Data    map[string]interface{} `json:"data"`    // Request data as JSON
}

// InvokeResponse represents a gRPC method invocation response
type InvokeResponse struct {
	Success bool                   `json:"success"`
	Data    map[string]interface{} `json:"data,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

// GrpcInvokeHandler handles dynamic gRPC method invocation via JSON
type GrpcInvokeHandler struct {
	processManager *process.Manager
}

// NewGrpcInvokeHandler creates a new gRPC invoke handler
func NewGrpcInvokeHandler(procMgr *process.Manager) *GrpcInvokeHandler {
	return &GrpcInvokeHandler{
		processManager: procMgr,
	}
}

// InvokeMethod handles POST /api/grpc/invoke
func (h *GrpcInvokeHandler) InvokeMethod(w http.ResponseWriter, r *http.Request) {
	// CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req InvokeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Process == "" || req.Service == "" || req.Method == "" {
		h.sendError(w, "Missing required fields: process, service, method", http.StatusBadRequest)
		return
	}

	// Get process port
	instances, err := h.processManager.GetProcessStatus(req.Process)
	if err != nil || len(instances) == 0 || instances[0].Port <= 0 {
		h.sendError(w, fmt.Sprintf("Process %s is not running", req.Process), http.StatusServiceUnavailable)
		return
	}

	port := instances[0].Port
	address := fmt.Sprintf("127.0.0.1:%d", port)

	log.Printf("Invoking gRPC method: %s/%s on %s (port %d)", req.Service, req.Method, req.Process, port)

	// Invoke gRPC method using grpcurl
	result, err := h.invokeWithGrpcurl(address, req.Service, req.Method, req.Data)
	if err != nil {
		h.sendError(w, fmt.Sprintf("Failed to invoke method: %v", err), http.StatusInternalServerError)
		return
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(InvokeResponse{
		Success: true,
		Data:    result,
	})
}

// invokeWithGrpcurl invokes a gRPC method using the grpcurl command-line tool
func (h *GrpcInvokeHandler) invokeWithGrpcurl(address, service, method string, data map[string]interface{}) (map[string]interface{}, error) {
	// Convert data to JSON string
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	// Build grpcurl command
	fullMethod := fmt.Sprintf("%s/%s", service, method)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "grpcurl",
		"-plaintext",
		"-format", "json",
		"-d", string(dataJSON),
		address,
		fullMethod,
	)

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute command
	err = cmd.Run()
	if err != nil {
		log.Printf("grpcurl error: %v, stderr: %s", err, stderr.String())
		return nil, fmt.Errorf("grpcurl failed: %v, stderr: %s", err, stderr.String())
	}

	// Parse JSON response
	var result map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w, output: %s", err, stdout.String())
	}

	return result, nil
}

// sendError sends an error response
func (h *GrpcInvokeHandler) sendError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(InvokeResponse{
		Success: false,
		Error:   message,
	})
	log.Printf("gRPC invoke error: %s", message)
}
