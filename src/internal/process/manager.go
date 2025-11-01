package process

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/certs"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/secrets"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/pkg/models"
)

// Manager handles process lifecycle management
type Manager struct {
	config         *models.Config
	processes      map[string]*models.ManagedProcess
	certManager    *certs.Manager
	secretManager  *secrets.Manager
	versionManager VersionManager
	nextPort       int // Next port to try for auto-allocation
	mu             sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
}

// VersionManager interface for version management operations
type VersionManager interface {
	GetVersion(repository, tag string) (*models.Version, error)
	DownloadVersion(version *models.Version, destPath string, progressCallback func(downloaded, total int64)) error
	SetCurrentVersion(processName string, version *models.Version) error
}

// NewManager creates a new process manager
func NewManager(config *models.Config, certMgr *certs.Manager, secretMgr *secrets.Manager) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		config:        config,
		processes:     make(map[string]*models.ManagedProcess),
		certManager:   certMgr,
		secretManager: secretMgr,
		nextPort:      5001, // Start port allocation from 5001
		ctx:           ctx,
		cancel:        cancel,
	}
}

// SetVersionManager sets the version manager for binary downloads
func (m *Manager) SetVersionManager(versionMgr VersionManager) {
	m.versionManager = versionMgr
}

// Initialize initializes all configured processes
func (m *Manager) Initialize() error {
	for i := range m.config.Processes {
		procConfig := m.config.Processes[i]

		m.mu.Lock()
		m.processes[procConfig.Name] = &models.ManagedProcess{
			Config:    procConfig,
			Instances: make([]*models.ProcessInstance, 0),
		}
		m.mu.Unlock()

		// Generate certificates if they don't exist
		if !m.certManager.CertificateExists(procConfig.Name) {
			hosts := []string{"localhost", "127.0.0.1"}
			if _, _, err := m.certManager.GenerateForProcess(procConfig.Name, hosts); err != nil {
				return fmt.Errorf("failed to generate certificate for %s: %w", procConfig.Name, err)
			}
		}

		// Generate .env file if it doesn't exist or override is enabled
		certPath, keyPath := m.certManager.GetPaths(procConfig.Name)
		if m.config.Secrets.Override || !m.secretManager.EnvFileExists(procConfig.Name) {
			if err := m.secretManager.GenerateEnvFile(procConfig.Name, certPath, keyPath); err != nil {
				return fmt.Errorf("failed to generate env file for %s: %w", procConfig.Name, err)
			}
		}

		// Download binary if it doesn't exist
		if err := m.ensureBinaryExists(&procConfig); err != nil {
			return fmt.Errorf("failed to ensure binary for %s: %w", procConfig.Name, err)
		}
	}

	return nil
}

// ensureBinaryExists checks if binary exists and downloads if necessary
func (m *Manager) ensureBinaryExists(procConfig *models.ProcessConfig) error {
	binaryPath := procConfig.BinaryPath
	if !filepath.IsAbs(binaryPath) {
		wd, _ := os.Getwd()
		binaryPath = filepath.Join(wd, binaryPath)
	}

	// Check if binary already exists
	if _, err := os.Stat(binaryPath); err == nil {
		fmt.Printf("[Binary] %s already exists\n", binaryPath)
		return nil
	}

	// Binary doesn't exist - download from GitHub
	if procConfig.Repository == "" {
		return fmt.Errorf("binary not found and no repository specified")
	}

	if m.versionManager == nil {
		return fmt.Errorf("binary not found and version manager not configured")
	}

	fmt.Printf("[Binary] Downloading binary for %s from repository %s...\n", procConfig.Name, procConfig.Repository)

	// Get latest release
	version, err := m.versionManager.GetVersion(procConfig.Repository, "latest")
	if err != nil {
		return fmt.Errorf("failed to get latest version: %w", err)
	}

	// Create version-specific binary path
	// e.g., ./binaries/db_service/v1.12.1/db_service.exe
	binaryDir := filepath.Dir(binaryPath)
	binaryName := filepath.Base(binaryPath)
	versionedBinaryDir := filepath.Join(binaryDir, version.Tag)
	versionedBinaryPath := filepath.Join(versionedBinaryDir, binaryName)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(versionedBinaryDir, 0755); err != nil {
		return fmt.Errorf("failed to create binary directory: %w", err)
	}

	// Download binary to versioned path
	progressCallback := func(downloaded, total int64) {
		if total > 0 {
			percent := float64(downloaded) / float64(total) * 100
			fmt.Printf("[Binary] Downloading %s %s: %.1f%% (%.1f MB / %.1f MB)\n",
				procConfig.Name, version.Tag, percent,
				float64(downloaded)/1024/1024, float64(total)/1024/1024)
		}
	}

	if err := m.versionManager.DownloadVersion(version, versionedBinaryPath, progressCallback); err != nil {
		return fmt.Errorf("failed to download binary: %w", err)
	}

	// Create symlink or copy to the configured path
	if err := os.MkdirAll(binaryDir, 0755); err != nil {
		return fmt.Errorf("failed to create binary directory: %w", err)
	}

	// On Windows, create a copy instead of symlink
	input, err := os.ReadFile(versionedBinaryPath)
	if err != nil {
		return fmt.Errorf("failed to read versioned binary: %w", err)
	}
	if err := os.WriteFile(binaryPath, input, 0755); err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}

	fmt.Printf("[Binary] Successfully downloaded %s %s to %s\n", procConfig.Name, version.Tag, versionedBinaryPath)
	fmt.Printf("[Binary] Created copy at %s\n", binaryPath)
	return nil
}

// StartProcess starts a new instance of a process
func (m *Manager) StartProcess(processName string) (*models.ProcessInstance, error) {
	m.mu.RLock()
	managedProc, exists := m.processes[processName]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("process %s not found", processName)
	}

	// Check if we can start more instances
	runningInstances := managedProc.GetRunningInstances()
	if len(runningInstances) >= managedProc.Config.MaxInstances {
		return nil, fmt.Errorf("maximum instances (%d) already running", managedProc.Config.MaxInstances)
	}

	// Find an available port (try up to 10 times from nextPort)
	m.mu.Lock()
	startPort := m.nextPort
	m.mu.Unlock()

	availablePort, err := findAvailablePort(startPort, 10)
	if err != nil {
		return nil, fmt.Errorf("failed to find available port: %w", err)
	}

	// Update nextPort to start searching from the next port next time
	m.mu.Lock()
	m.nextPort = availablePort + 1
	m.mu.Unlock()

	// Create process instance
	instance := &models.ProcessInstance{
		ID:          uuid.New().String(),
		ProcessName: processName,
		Status:      models.StatusStarting,
		StartTime:   time.Now(),
		Port:        availablePort,
		EnvFilePath: m.secretManager.GetEnvFilePath(processName),
	}

	// Determine binary path
	var binaryPath string
	if managedProc.Config.BinaryPath != "" {
		// Use configured binary path
		binaryPath = managedProc.Config.BinaryPath
		if !filepath.IsAbs(binaryPath) {
			binaryPath = filepath.Join(managedProc.Config.WorkDir, binaryPath)
		}
	} else {
		// Auto-detect latest binary from binaries directory
		detected, err := m.detectLatestBinary(processName, managedProc.Config.Repository)
		if err != nil {
			return nil, fmt.Errorf("failed to detect binary: %w", err)
		}
		binaryPath = detected
		log.Printf("Auto-detected binary for %s: %s", processName, binaryPath)
	}

	// Extract and record version from binary filename if version manager is set
	if m.versionManager != nil {
		if version := extractVersionFromFilename(binaryPath); version != "" {
			// Record the current version
			versionInfo := &models.Version{
				Tag: version,
			}
			if err := m.versionManager.SetCurrentVersion(processName, versionInfo); err != nil {
				log.Printf("Warning: failed to record version for %s: %v", processName, err)
			} else {
				log.Printf("Recorded version %s for %s (allocated port %d) from binary path", version, processName, availablePort)
			}
		}
	}

	cmd := exec.CommandContext(m.ctx, binaryPath, managedProc.Config.Args...)
	cmd.Dir = managedProc.Config.WorkDir

	// Capture stdout and stderr to log process output
	var stderrBuf bytes.Buffer
	cmd.Stdout = os.Stdout  // Forward stdout to main process stdout
	cmd.Stderr = &stderrBuf // Capture stderr for error logging

	// Load environment variables
	envVars, err := m.secretManager.LoadEnvFile(processName)
	if err != nil {
		return nil, fmt.Errorf("failed to load env file: %w", err)
	}

	// Set environment variables
	cmd.Env = os.Environ()
	for k, v := range envVars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Add GRPC_PORT environment variable for the process to use
	cmd.Env = append(cmd.Env, fmt.Sprintf("GRPC_PORT=%d", availablePort))
	log.Printf("Starting %s with auto-allocated GRPC_PORT=%d", processName, availablePort)

	// Start the process
	if err := cmd.Start(); err != nil {
		instance.SetStatus(models.StatusFailed)
		return nil, fmt.Errorf("failed to start process: %w", err)
	}

	instance.Command = cmd
	instance.PID = cmd.Process.Pid
	instance.StderrBuf = &stderrBuf
	instance.SetStatus(models.StatusRunning)

	// Add instance to managed process
	managedProc.AddInstance(instance)

	// Monitor process
	go m.monitorProcess(instance)

	return instance, nil
}

// StopProcess stops a specific instance of a process (forcefully)
func (m *Manager) StopProcess(processName, instanceID string) error {
	m.mu.RLock()
	managedProc, exists := m.processes[processName]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("process %s not found", processName)
	}

	instances := managedProc.GetInstances()
	var targetInstance *models.ProcessInstance

	for _, inst := range instances {
		if inst.ID == instanceID {
			targetInstance = inst
			break
		}
	}

	if targetInstance == nil {
		return fmt.Errorf("instance %s not found", instanceID)
	}

	targetInstance.SetStatus(models.StatusStopping)

	// Kill the process
	if targetInstance.Command != nil && targetInstance.Command.Process != nil {
		if err := targetInstance.Command.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
	}

	// Wait for process to exit
	if targetInstance.Command != nil {
		targetInstance.Command.Wait()
	}

	targetInstance.SetStatus(models.StatusStopped)
	managedProc.RemoveInstance(instanceID)

	return nil
}

// StopProcessGracefully stops a specific instance of a process gracefully
// It sends SIGTERM first, waits for graceful shutdown, then sends SIGKILL if needed
func (m *Manager) StopProcessGracefully(processName, instanceID string, timeout time.Duration) error {
	m.mu.RLock()
	managedProc, exists := m.processes[processName]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("process %s not found", processName)
	}

	instances := managedProc.GetInstances()
	var targetInstance *models.ProcessInstance

	for _, inst := range instances {
		if inst.ID == instanceID {
			targetInstance = inst
			break
		}
	}

	if targetInstance == nil {
		return fmt.Errorf("instance %s not found", instanceID)
	}

	if targetInstance.Command == nil || targetInstance.Command.Process == nil {
		// Process already stopped or not running
		targetInstance.SetStatus(models.StatusStopped)
		managedProc.RemoveInstance(instanceID)
		return nil
	}

	targetInstance.SetStatus(models.StatusStopping)
	log.Printf("[GracefulShutdown] Initiating graceful shutdown for %s (instance: %s, PID: %d, timeout: %v)",
		processName, instanceID, targetInstance.PID, timeout)

	// Send SIGTERM (on Windows, this is equivalent to sending a close signal)
	// Note: On Windows, Signal() is not fully implemented, so we use a different approach
	if err := m.sendTermSignal(targetInstance); err != nil {
		log.Printf("[GracefulShutdown] Failed to send termination signal to %s: %v, forcing kill", processName, err)
		return m.forceKillProcess(targetInstance, managedProc, instanceID)
	}

	// Wait for process to exit gracefully or timeout
	done := make(chan error, 1)
	go func() {
		if targetInstance.Command != nil {
			done <- targetInstance.Command.Wait()
		} else {
			done <- nil
		}
	}()

	select {
	case <-time.After(timeout):
		// Timeout reached, force kill
		log.Printf("[GracefulShutdown] Timeout reached for %s (instance: %s), forcing kill", processName, instanceID)
		return m.forceKillProcess(targetInstance, managedProc, instanceID)

	case err := <-done:
		// Process exited gracefully
		if err != nil {
			log.Printf("[GracefulShutdown] Process %s (instance: %s) exited with error: %v", processName, instanceID, err)
		} else {
			log.Printf("[GracefulShutdown] Process %s (instance: %s) exited gracefully", processName, instanceID)
		}
		targetInstance.SetStatus(models.StatusStopped)
		managedProc.RemoveInstance(instanceID)
		return nil
	}
}

// sendTermSignal sends termination signal to a process
// First tries HTTP shutdown endpoint (if port is available), then falls back to taskkill
func (m *Manager) sendTermSignal(instance *models.ProcessInstance) error {
	// Try HTTP shutdown endpoint first (platform-independent approach)
	if instance.Port > 0 {
		shutdownURL := fmt.Sprintf("http://localhost:%d/shutdown", instance.Port)
		client := &http.Client{Timeout: 5 * time.Second}

		req, err := http.NewRequest("POST", shutdownURL, nil)
		if err == nil {
			resp, err := client.Do(req)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					log.Printf("[GracefulShutdown] HTTP shutdown signal sent successfully to port %d", instance.Port)
					return nil
				}
				log.Printf("[GracefulShutdown] HTTP shutdown endpoint returned status %d", resp.StatusCode)
			} else {
				log.Printf("[GracefulShutdown] Failed to call HTTP shutdown endpoint: %v", err)
			}
		}
	}

	// Fallback to taskkill (will likely fail on Windows for graceful shutdown)
	// On Windows, use taskkill without /F to allow graceful shutdown
	// Note: This approach has limitations on Windows as taskkill sends WM_CLOSE
	// which is not caught by Go's signal.Notify
	log.Printf("[GracefulShutdown] Falling back to taskkill for PID %d", instance.PID)
	cmd := exec.Command("taskkill", "/PID", fmt.Sprintf("%d", instance.PID))
	return cmd.Run()
}

// forceKillProcess forcefully kills a process
func (m *Manager) forceKillProcess(instance *models.ProcessInstance, managedProc *models.ManagedProcess, instanceID string) error {
	log.Printf("[GracefulShutdown] Force killing process (PID: %d)", instance.PID)

	if instance.Command != nil && instance.Command.Process != nil {
		if err := instance.Command.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
	}

	// Wait for process to exit
	if instance.Command != nil {
		instance.Command.Wait()
	}

	instance.SetStatus(models.StatusStopped)
	managedProc.RemoveInstance(instanceID)
	return nil
}

// StopAllInstances stops all instances of a process
func (m *Manager) StopAllInstances(processName string) error {
	m.mu.RLock()
	managedProc, exists := m.processes[processName]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("process %s not found", processName)
	}

	instances := managedProc.GetInstances()
	var errs []error

	for _, inst := range instances {
		if err := m.StopProcess(processName, inst.ID); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to stop some instances: %v", errs)
	}

	return nil
}

// GetProcessStatus returns the status of all instances of a process
func (m *Manager) GetProcessStatus(processName string) ([]*models.ProcessInstance, error) {
	m.mu.RLock()
	managedProc, exists := m.processes[processName]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("process %s not found", processName)
	}

	return managedProc.GetInstances(), nil
}

// ListProcesses returns all managed processes
func (m *Manager) ListProcesses() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.processes))
	for name := range m.processes {
		names = append(names, name)
	}
	return names
}

// GetProcessRepository returns the repository for a process
func (m *Manager) GetProcessRepository(processName string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if proc, exists := m.processes[processName]; exists {
		return proc.Config.Repository
	}
	return ""
}

// Shutdown shuts down all processes
func (m *Manager) Shutdown() error {
	m.cancel()

	m.mu.RLock()
	processNames := make([]string, 0, len(m.processes))
	for name := range m.processes {
		processNames = append(processNames, name)
	}
	m.mu.RUnlock()

	var errs []error
	for _, name := range processNames {
		if err := m.StopAllInstances(name); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errs)
	}

	return nil
}

// monitorProcess monitors a process and handles auto-restart
func (m *Manager) monitorProcess(instance *models.ProcessInstance) {
	if instance.Command == nil {
		return
	}

	// Wait for process to exit
	err := instance.Command.Wait()

	// If process exited, update status and log error details
	if err != nil {
		instance.SetStatus(models.StatusFailed)

		// Log the error with stderr output if available
		var stderrOutput string
		if instance.StderrBuf != nil && instance.StderrBuf.Len() > 0 {
			stderrOutput = instance.StderrBuf.String()
		}

		if stderrOutput != "" {
			log.Printf("[ERROR] Process %s (port %d, PID %d) failed with error: %v\nStderr output:\n%s",
				instance.ProcessName, instance.Port, instance.PID, err, stderrOutput)
		} else {
			log.Printf("[ERROR] Process %s (port %d, PID %d) failed with error: %v (no stderr output)",
				instance.ProcessName, instance.Port, instance.PID, err)
		}
	} else {
		instance.SetStatus(models.StatusStopped)
		log.Printf("Process %s (port %d, PID %d) stopped normally",
			instance.ProcessName, instance.Port, instance.PID)
	}

	// Get managed process config
	m.mu.RLock()
	managedProc := m.processes[instance.ProcessName]
	m.mu.RUnlock()

	// Remove instance from list
	managedProc.RemoveInstance(instance.ID)

	// Auto-restart if configured
	if managedProc.Config.AutoRestart && instance.GetStatus() == models.StatusFailed {
		// Wait a bit before restarting
		time.Sleep(5 * time.Second)

		// Try to restart
		if _, err := m.StartProcess(instance.ProcessName); err != nil {
			log.Printf("Failed to auto-restart %s: %v", instance.ProcessName, err)
		}
	}
}

// extractVersionFromFilename extracts version string from binary filename
// Supports formats: processname_v1.2.3.exe or processname_1.2.3.exe
func extractVersionFromFilename(binaryPath string) string {
	filename := filepath.Base(binaryPath)
	// Match patterns like: db_service_v1.12.1.exe or db_service_1.12.1.exe
	re := regexp.MustCompile(`_v?(\d+\.\d+\.\d+)\.exe$`)
	matches := re.FindStringSubmatch(filename)
	if len(matches) > 1 {
		return "v" + matches[1] // Always return with 'v' prefix
	}
	return ""
}

// detectLatestBinary finds the latest binary version in binaries directory
// Returns the path to the latest version based on semantic versioning
func (m *Manager) detectLatestBinary(processName, repository string) (string, error) {
	// Extract binary name from repository (e.g., "owner/repo" -> "repo")
	parts := filepath.SplitList(repository)
	var binaryName string
	if len(parts) > 0 && filepath.Base(repository) != repository {
		binaryName = filepath.Base(repository)
	} else {
		binaryName = processName
	}

	// Check binaries/binaryName directory
	binDir := filepath.Join("binaries", binaryName)
	entries, err := os.ReadDir(binDir)
	if err != nil {
		return "", fmt.Errorf("failed to read binaries directory %s: %w", binDir, err)
	}

	// Find all versioned binaries
	var latestPath string
	var latestVersion string

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fullPath := filepath.Join(binDir, entry.Name())
		version := extractVersionFromFilename(fullPath)
		if version == "" {
			continue
		}

		// Simple string comparison works for semantic versioning (v1.12.2 > v1.12.1)
		if latestVersion == "" || version > latestVersion {
			latestVersion = version
			latestPath = fullPath
		}
	}

	if latestPath == "" {
		return "", fmt.Errorf("no versioned binary found in %s", binDir)
	}

	return latestPath, nil
}

// findAvailablePort finds an available port by trying to bind to it
// Tries up to maxAttempts times, starting from startPort
func findAvailablePort(startPort int, maxAttempts int) (int, error) {
	for i := 0; i < maxAttempts; i++ {
		port := startPort + i

		// Try to listen on the port
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			// Port is in use, try next one
			continue
		}

		// Port is available, close the listener and return the port
		listener.Close()
		return port, nil
	}

	return 0, fmt.Errorf("no available port found after %d attempts starting from %d", maxAttempts, startPort)
}
