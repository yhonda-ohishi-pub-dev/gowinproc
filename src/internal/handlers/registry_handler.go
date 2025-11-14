package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/process"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/reflection"
)

// FieldDetail represents a field in a message
type FieldDetail struct {
	Name     string `json:"name"`
	Type     string `json:"type"`      // e.g., "string", "int32", "bool"
	Repeated bool   `json:"repeated"`  // true if this is a repeated field
	Number   int32  `json:"number"`    // field number
	Optional bool   `json:"optional"`  // true if this field is optional
}

// MessageDetail represents a protobuf message with its fields
type MessageDetail struct {
	Name   string        `json:"name"`
	Fields []FieldDetail `json:"fields"`
}

// MethodDetail represents a gRPC method with input/output types
type MethodDetail struct {
	Name       string `json:"name"`
	InputType  string `json:"input_type"`
	OutputType string `json:"output_type"`
}

// ServiceDetail represents a gRPC service with its methods
type ServiceDetail struct {
	Name    string         `json:"name"`
	Methods []MethodDetail `json:"methods"`
}

// ProcessInfo represents information about a proxiable process
type ProcessInfo struct {
	Name         string                   `json:"name"`
	DisplayName  string                   `json:"display_name"`
	Status       string                   `json:"status"`
	Instances    int                      `json:"instances"`
	ProxyPath    string                   `json:"proxy_path"`
	Repository   string                   `json:"repository"`
	CurrentPorts []int                    `json:"current_ports,omitempty"`
	Services     []ServiceDetail          `json:"services,omitempty"` // gRPC services with methods from reflection
	Messages     map[string]MessageDetail `json:"messages,omitempty"` // Message schemas (message type name -> schema)
}

// RegistryResponse is the response for the dynamic proxy registry endpoint
type RegistryResponse struct {
	ProxyBaseURL       string        `json:"proxy_base_url"`
	AvailableProcesses []ProcessInfo `json:"available_processes"`
	Timestamp          time.Time     `json:"timestamp"`
}

// CachedServiceInfo stores both services and message schemas
type CachedServiceInfo struct {
	Services []ServiceDetail
	Messages map[string]MessageDetail
}

// RegistryHandler handles the /api/grpc/registry endpoint
type RegistryHandler struct {
	processManager   *process.Manager
	baseURL          string
	reflectionClient *reflection.Client
	servicesCache    map[string]*CachedServiceInfo // process name -> services with methods and message schemas
	cacheMu          sync.RWMutex
	cacheExpiry      time.Time
}

// NewRegistryHandler creates a new registry handler
func NewRegistryHandler(procMgr *process.Manager, host string, port int) *RegistryHandler {
	return &RegistryHandler{
		processManager:   procMgr,
		baseURL:          fmt.Sprintf("http://%s:%d", host, port),
		reflectionClient: reflection.NewClient(),
		servicesCache:    make(map[string]*CachedServiceInfo),
	}
}

// GetRegistry handles GET /api/grpc/registry
func (h *RegistryHandler) GetRegistry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.processManager == nil {
		http.Error(w, "Process manager not initialized", http.StatusServiceUnavailable)
		return
	}

	// Get all processes from ProcessManager
	allProcesses := h.processManager.ListProcesses()
	availableProcesses := []ProcessInfo{}

	for _, procName := range allProcesses {
		instances, err := h.processManager.GetProcessStatus(procName)
		if err != nil {
			continue
		}

		runningCount := 0
		ports := []int{}
		repository := h.processManager.GetProcessRepository(procName)

		for _, inst := range instances {
			if inst.Status == "running" && inst.Port > 0 {
				runningCount++
				ports = append(ports, inst.Port)
			}
		}

		status := "stopped"
		if runningCount > 0 {
			status = "running"
		}

		// Get gRPC services and message schemas from reflection (with caching)
		// Skip reflection for db_service and desktop_server processes
		var services []ServiceDetail
		var messages map[string]MessageDetail
		isDBService := len(procName) >= 10 && procName[:10] == "db_service"
		isDesktopServer := procName == "desktop_server"
		if runningCount > 0 && len(ports) > 0 && !isDBService && !isDesktopServer {
			cachedInfo := h.getServicesWithCache(procName, ports[0])
			if cachedInfo != nil {
				services = cachedInfo.Services
				messages = cachedInfo.Messages
			}
		}

		processInfo := ProcessInfo{
			Name:         procName,
			DisplayName:  h.getDisplayName(procName),
			Status:       status,
			Instances:    runningCount,
			ProxyPath:    "/proxy/" + procName,
			Repository:   repository,
			CurrentPorts: ports,
			Services:     services,
			Messages:     messages,
		}

		availableProcesses = append(availableProcesses, processInfo)
	}

	response := RegistryResponse{
		ProxyBaseURL:       h.baseURL,
		AvailableProcesses: availableProcesses,
		Timestamp:          time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// getDisplayName generates a display name from process name
func (h *RegistryHandler) getDisplayName(name string) string {
	// Simple conversion: db_service_local -> DB Service Local
	// TODO: More sophisticated implementation
	return name
}

// getServicesWithCache retrieves gRPC services and message schemas from cache or via reflection
func (h *RegistryHandler) getServicesWithCache(processName string, port int) *CachedServiceInfo {
	// Check cache first (5 minute TTL)
	h.cacheMu.RLock()
	if time.Now().Before(h.cacheExpiry) {
		if cachedInfo, ok := h.servicesCache[processName]; ok {
			h.cacheMu.RUnlock()
			return cachedInfo
		}
	}
	h.cacheMu.RUnlock()

	// Cache miss or expired, fetch from reflection
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	address := fmt.Sprintf("127.0.0.1:%d", port)
	serviceInfo, err := h.reflectionClient.GetServiceInfo(ctx, address)
	if err != nil {
		log.Printf("Failed to get services for %s via reflection: %v", processName, err)
		return nil
	}

	// Convert map[string][]reflection.MethodDetail to []ServiceDetail
	var services []ServiceDetail
	for serviceName, reflectionMethods := range serviceInfo.Services {
		// Skip db_service.* services
		if len(serviceName) >= 11 && serviceName[:11] == "db_service." {
			continue
		}

		// Convert []reflection.MethodDetail to []MethodDetail
		methods := make([]MethodDetail, len(reflectionMethods))
		for i, rm := range reflectionMethods {
			methods[i] = MethodDetail{
				Name:       rm.Name,
				InputType:  rm.InputType,
				OutputType: rm.OutputType,
			}
		}
		services = append(services, ServiceDetail{
			Name:    serviceName,
			Methods: methods,
		})
	}

	// Convert reflection message schemas to handler message schemas
	messages := make(map[string]MessageDetail)
	for msgName, reflectionMsg := range serviceInfo.Messages {
		// Skip .db_service.* messages
		if len(msgName) >= 12 && msgName[:12] == ".db_service." {
			continue
		}

		// Convert []reflection.FieldDetail to []FieldDetail
		fields := make([]FieldDetail, len(reflectionMsg.Fields))
		for i, rf := range reflectionMsg.Fields {
			fields[i] = FieldDetail{
				Name:     rf.Name,
				Type:     rf.Type,
				Repeated: rf.Repeated,
				Number:   rf.Number,
				Optional: rf.Optional,
			}
		}
		messages[msgName] = MessageDetail{
			Name:   reflectionMsg.Name,
			Fields: fields,
		}
	}

	// Create cached info
	cachedInfo := &CachedServiceInfo{
		Services: services,
		Messages: messages,
	}

	// Update cache
	h.cacheMu.Lock()
	h.servicesCache[processName] = cachedInfo
	h.cacheExpiry = time.Now().Add(5 * time.Minute)
	h.cacheMu.Unlock()

	return cachedInfo
}
