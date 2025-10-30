package update

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/process"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/version"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/pkg/models"
)

// Manager handles update operations with Hot Deploy
type Manager struct {
	processManager *process.Manager
	versionManager *version.Manager
	binariesDir    string
	updates        map[string]*models.UpdateStatus
	mu             sync.RWMutex
}

// NewManager creates a new update manager
func NewManager(processMgr *process.Manager, versionMgr *version.Manager, binariesDir string) (*Manager, error) {
	if err := os.MkdirAll(binariesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create binaries directory: %w", err)
	}

	return &Manager{
		processManager: processMgr,
		versionManager: versionMgr,
		binariesDir:    binariesDir,
		updates:        make(map[string]*models.UpdateStatus),
	}, nil
}

// UpdateProcess updates a process to a specific version (or latest if version is empty)
func (m *Manager) UpdateProcess(processName, targetVersion string, force bool) error {
	// Check if update is already in progress
	m.mu.RLock()
	if status, exists := m.updates[processName]; exists && !status.Completed {
		m.mu.RUnlock()
		return fmt.Errorf("update already in progress for %s", processName)
	}
	m.mu.RUnlock()

	// Initialize update status
	status := &models.UpdateStatus{
		ProcessName: processName,
		Stage:       "initializing",
		Progress:    0,
		Completed:   false,
	}
	m.setUpdateStatus(processName, status)

	// Run update in background
	go m.performUpdate(processName, targetVersion, force)

	return nil
}

// performUpdate performs the actual update operation
func (m *Manager) performUpdate(processName, targetVersion string, force bool) {
	status := m.getUpdateStatus(processName)

	// Get process config
	processList := m.processManager.ListProcesses()
	var repository string
	for _, name := range processList {
		if name == processName {
			// TODO: Get repository from process config
			// For now, we'll need to pass this through the config
			break
		}
	}

	// Stage 1: Fetch version information
	status.Stage = "fetching_version"
	status.Message = "Fetching version information from GitHub"
	status.Progress = 10
	m.setUpdateStatus(processName, status)

	var targetVersionInfo *models.Version
	var err error

	if targetVersion == "" {
		// Get latest version
		targetVersionInfo, err = m.versionManager.GetVersion(repository, "latest")
	} else {
		// Get specific version
		targetVersionInfo, err = m.versionManager.GetVersion(repository, targetVersion)
	}

	if err != nil {
		status.Stage = "failed"
		status.Error = fmt.Sprintf("Failed to fetch version: %v", err)
		status.Completed = true
		m.setUpdateStatus(processName, status)
		return
	}

	// Check if already on target version
	currentVersion, _ := m.versionManager.GetCurrentVersion(processName)
	if currentVersion != nil && currentVersion.Tag == targetVersionInfo.Tag && !force {
		status.Stage = "completed"
		status.Message = "Already on target version"
		status.Progress = 100
		status.Completed = true
		m.setUpdateStatus(processName, status)
		return
	}

	// Stage 2: Download new binary
	status.Stage = "downloading"
	status.Message = fmt.Sprintf("Downloading version %s", targetVersionInfo.Tag)
	status.Progress = 20
	m.setUpdateStatus(processName, status)

	binaryPath := m.getBinaryPath(processName, targetVersionInfo.Tag)
	progressCallback := func(downloaded, total int64) {
		if total > 0 {
			downloadProgress := float64(downloaded) / float64(total) * 50 // 20-70%
			status.Progress = 20 + downloadProgress
			status.Message = fmt.Sprintf("Downloading: %.1f MB / %.1f MB",
				float64(downloaded)/1024/1024, float64(total)/1024/1024)
			m.setUpdateStatus(processName, status)
		}
	}

	if err := m.versionManager.DownloadVersion(targetVersionInfo, binaryPath, progressCallback); err != nil {
		status.Stage = "failed"
		status.Error = fmt.Sprintf("Failed to download binary: %v", err)
		status.Completed = true
		m.setUpdateStatus(processName, status)
		return
	}

	// Stage 3: Start new instance (Hot Deploy)
	status.Stage = "starting_new"
	status.Message = "Starting new instance"
	status.Progress = 75
	m.setUpdateStatus(processName, status)

	// TODO: Update process config with new binary path
	// This requires modifying the process manager to support binary path updates

	newInstance, err := m.processManager.StartProcess(processName)
	if err != nil {
		status.Stage = "failed"
		status.Error = fmt.Sprintf("Failed to start new instance: %v", err)
		status.Completed = true
		m.setUpdateStatus(processName, status)
		return
	}

	// Wait for new instance to be healthy
	time.Sleep(5 * time.Second)

	// Stage 4: Stop old instances
	status.Stage = "stopping_old"
	status.Message = "Stopping old instances"
	status.Progress = 85
	m.setUpdateStatus(processName, status)

	instances, _ := m.processManager.GetProcessStatus(processName)
	for _, inst := range instances {
		if inst.ID != newInstance.ID && inst.GetStatus() == models.StatusRunning {
			if err := m.processManager.StopProcess(processName, inst.ID); err != nil {
				// Log error but continue
				fmt.Printf("Warning: failed to stop old instance %s: %v\n", inst.ID, err)
			}
		}
	}

	// Stage 5: Update version tracking
	status.Stage = "updating_version"
	status.Message = "Updating version information"
	status.Progress = 95
	m.setUpdateStatus(processName, status)

	if err := m.versionManager.SetCurrentVersion(processName, targetVersionInfo); err != nil {
		fmt.Printf("Warning: failed to update version tracking: %v\n", err)
	}

	// Complete
	status.Stage = "completed"
	status.Message = fmt.Sprintf("Successfully updated to version %s", targetVersionInfo.Tag)
	status.Progress = 100
	status.Completed = true
	m.setUpdateStatus(processName, status)
}

// RollbackProcess rolls back a process to a previous version
func (m *Manager) RollbackProcess(processName, targetVersion string) error {
	var versionToRollback *models.Version
	var err error

	if targetVersion == "" {
		// Rollback to previous version
		versionToRollback, err = m.versionManager.GetPreviousVersion(processName)
		if err != nil {
			return fmt.Errorf("failed to get previous version: %w", err)
		}
	} else {
		// Rollback to specific version
		info, err := m.versionManager.LoadVersionInfo(processName)
		if err != nil {
			return fmt.Errorf("failed to load version info: %w", err)
		}

		// Find version in history
		for _, v := range info.History {
			if v.Tag == targetVersion {
				versionToRollback = &v
				break
			}
		}

		if versionToRollback == nil {
			return fmt.Errorf("version %s not found in history", targetVersion)
		}
	}

	// Perform update to the rollback version
	return m.UpdateProcess(processName, versionToRollback.Tag, true)
}

// GetUpdateStatus returns the current update status for a process
func (m *Manager) GetUpdateStatus(processName string) (*models.UpdateStatus, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status, exists := m.updates[processName]
	return status, exists
}

// setUpdateStatus sets the update status for a process
func (m *Manager) setUpdateStatus(processName string, status *models.UpdateStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updates[processName] = status
}

// getUpdateStatus gets the update status for a process
func (m *Manager) getUpdateStatus(processName string) *models.UpdateStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.updates[processName]
}

// getBinaryPath returns the path where a binary should be stored
func (m *Manager) getBinaryPath(processName, version string) string {
	return filepath.Join(m.binariesDir, processName, version, fmt.Sprintf("%s.exe", processName))
}

// CheckForUpdates checks for available updates for a process
func (m *Manager) CheckForUpdates(processName, repository string) (*models.VersionInfo, error) {
	return m.versionManager.CheckForUpdates(processName, repository)
}
