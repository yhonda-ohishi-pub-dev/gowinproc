package process

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/certs"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/secrets"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/pkg/models"
)

// Manager handles process lifecycle management
type Manager struct {
	config        *models.Config
	processes     map[string]*models.ManagedProcess
	certManager   *certs.Manager
	secretManager *secrets.Manager
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewManager creates a new process manager
func NewManager(config *models.Config, certMgr *certs.Manager, secretMgr *secrets.Manager) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		config:        config,
		processes:     make(map[string]*models.ManagedProcess),
		certManager:   certMgr,
		secretManager: secretMgr,
		ctx:           ctx,
		cancel:        cancel,
	}
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

		// Generate .env file if it doesn't exist
		certPath, keyPath := m.certManager.GetPaths(procConfig.Name)
		if !m.secretManager.EnvFileExists(procConfig.Name) {
			if err := m.secretManager.GenerateEnvFile(procConfig.Name, certPath, keyPath); err != nil {
				return fmt.Errorf("failed to generate env file for %s: %w", procConfig.Name, err)
			}
		}
	}

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

	// Create process instance
	instance := &models.ProcessInstance{
		ID:          uuid.New().String(),
		ProcessName: processName,
		Status:      models.StatusStarting,
		StartTime:   time.Now(),
		Port:        managedProc.Config.Port,
		EnvFilePath: m.secretManager.GetEnvFilePath(processName),
	}

	// Prepare command
	binaryPath := managedProc.Config.BinaryPath
	if !filepath.IsAbs(binaryPath) {
		binaryPath = filepath.Join(managedProc.Config.WorkDir, binaryPath)
	}

	cmd := exec.CommandContext(m.ctx, binaryPath, managedProc.Config.Args...)
	cmd.Dir = managedProc.Config.WorkDir

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

	// Start the process
	if err := cmd.Start(); err != nil {
		instance.SetStatus(models.StatusFailed)
		return nil, fmt.Errorf("failed to start process: %w", err)
	}

	instance.Command = cmd
	instance.PID = cmd.Process.Pid
	instance.SetStatus(models.StatusRunning)

	// Add instance to managed process
	managedProc.AddInstance(instance)

	// Monitor process
	go m.monitorProcess(instance)

	return instance, nil
}

// StopProcess stops a specific instance of a process
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

	// If process exited, update status
	if err != nil {
		instance.SetStatus(models.StatusFailed)
	} else {
		instance.SetStatus(models.StatusStopped)
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
			// Log error (TODO: implement proper logging)
			fmt.Printf("Failed to auto-restart %s: %v\n", instance.ProcessName, err)
		}
	}
}
