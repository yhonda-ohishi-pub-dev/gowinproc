package update

import (
	"fmt"
	"log"
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
	// Repository-level locks to prevent concurrent downloads of the same binary
	repoLocks      map[string]*sync.Mutex
	repoLocksMu    sync.Mutex
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
		repoLocks:      make(map[string]*sync.Mutex),
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
	log.Printf("[Update] Starting update for %s to version %s (force=%v)", processName, targetVersion, force)
	status := m.getUpdateStatus(processName)

	// Get repository from process manager config
	repository := m.processManager.GetProcessRepository(processName)
	if repository == "" {
		log.Printf("[Update] ERROR: Failed to get repository for process %s", processName)
		status.Stage = "failed"
		status.Error = "Failed to get repository for process"
		status.Completed = true
		m.setUpdateStatus(processName, status)
		return
	}
	log.Printf("[Update] Repository for %s: %s", processName, repository)

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
		log.Printf("[Update] ERROR: Failed to fetch version for %s: %v", processName, err)
		status.Stage = "failed"
		status.Error = fmt.Sprintf("Failed to fetch version: %v", err)
		status.Completed = true
		m.setUpdateStatus(processName, status)
		return
	}
	log.Printf("[Update] Target version info for %s: %s", processName, targetVersionInfo.Tag)

	// Check if already on target version
	currentVersion, _ := m.versionManager.GetCurrentVersion(processName)
	if currentVersion != nil && currentVersion.Tag == targetVersionInfo.Tag && !force {
		log.Printf("[Update] %s already on target version %s, skipping", processName, targetVersionInfo.Tag)
		status.Stage = "completed"
		status.Message = "Already on target version"
		status.Progress = 100
		status.Completed = true
		m.setUpdateStatus(processName, status)
		return
	}
	log.Printf("[Update] Current version for %s: %v, target: %s", processName, currentVersion, targetVersionInfo.Tag)

	// Stage 2: Download new binary (with repository-level locking)
	status.Stage = "downloading"
	status.Message = fmt.Sprintf("Downloading version %s", targetVersionInfo.Tag)
	status.Progress = 20
	m.setUpdateStatus(processName, status)

	binaryPath := m.getBinaryPathForRepository(repository, targetVersionInfo.Tag)

	// Acquire repository-level lock to prevent concurrent downloads
	repoLock := m.getRepoLock(repository)
	log.Printf("[Update] Acquiring repository lock for %s (process: %s)", repository, processName)
	repoLock.Lock()

	// Check if binary already exists (another process may have downloaded it)
	if _, err := os.Stat(binaryPath); err == nil {
		log.Printf("[Update] Binary already exists at %s (downloaded by another process)", binaryPath)
		repoLock.Unlock()
	} else {
		// Download the binary
		log.Printf("[Update] Starting download of %s version %s", processName, targetVersionInfo.Tag)
		progressCallback := func(downloaded, total int64) {
			if total > 0 {
				downloadProgress := float64(downloaded) / float64(total) * 50 // 20-70%
				status.Progress = 20 + downloadProgress
				status.Message = fmt.Sprintf("Downloading: %.1f MB / %.1f MB",
					float64(downloaded)/1024/1024, float64(total)/1024/1024)
				m.setUpdateStatus(processName, status)
			}
		}

		err := m.versionManager.DownloadVersion(targetVersionInfo, binaryPath, progressCallback)
		repoLock.Unlock() // Release lock after download completes

		if err != nil {
			log.Printf("[Update] ERROR: Failed to download binary for %s: %v", processName, err)
			status.Stage = "failed"
			status.Error = fmt.Sprintf("Failed to download binary: %v", err)
			status.Completed = true
			m.setUpdateStatus(processName, status)
			return
		}
		log.Printf("[Update] Download completed for %s, binary path: %s", processName, binaryPath)
	}

	// Stage 3: Start new instance (Hot Deploy)
	status.Stage = "starting_new"
	status.Message = "Starting new instance"
	status.Progress = 75
	m.setUpdateStatus(processName, status)
	log.Printf("[Update] Starting new instance for %s", processName)

	// TODO: Update process config with new binary path
	// This requires modifying the process manager to support binary path updates

	newInstance, err := m.processManager.StartProcess(processName)
	if err != nil {
		log.Printf("[Update] ERROR: Failed to start new instance for %s: %v", processName, err)
		status.Stage = "failed"
		status.Error = fmt.Sprintf("Failed to start new instance: %v", err)
		status.Completed = true
		m.setUpdateStatus(processName, status)
		return
	}
	log.Printf("[Update] New instance started for %s: ID=%s", processName, newInstance.ID)

	// Wait for new instance to be healthy
	log.Printf("[Update] Waiting 5 seconds for %s new instance to be healthy...", processName)
	time.Sleep(5 * time.Second)

	// Stage 4: Stop old instances gracefully
	status.Stage = "stopping_old"
	status.Message = "Gracefully stopping old instances"
	status.Progress = 85
	m.setUpdateStatus(processName, status)
	log.Printf("[Update] Gracefully stopping old instances for %s", processName)

	instances, _ := m.processManager.GetProcessStatus(processName)
	log.Printf("[Update] Found %d total instances for %s", len(instances), processName)
	stoppedCount := 0
	gracefulTimeout := 30 * time.Second // 30 seconds timeout for graceful shutdown

	for _, inst := range instances {
		log.Printf("[Update] Checking instance %s (status: %s, new instance: %s)", inst.ID, inst.GetStatus(), newInstance.ID)
		if inst.ID != newInstance.ID && inst.GetStatus() == models.StatusRunning {
			log.Printf("[Update] Gracefully stopping old instance %s for %s (timeout: %v)", inst.ID, processName, gracefulTimeout)

			// Use graceful shutdown instead of immediate kill
			if err := m.processManager.StopProcessGracefully(processName, inst.ID, gracefulTimeout); err != nil {
				// Log error but continue
				log.Printf("[Update] Warning: failed to gracefully stop old instance %s: %v", inst.ID, err)
			} else {
				stoppedCount++
				log.Printf("[Update] Successfully stopped old instance %s", inst.ID)
			}
		}
	}
	log.Printf("[Update] Stopped %d old instances for %s", stoppedCount, processName)

	// Stage 5: Update version tracking
	status.Stage = "updating_version"
	status.Message = "Updating version information"
	status.Progress = 95
	m.setUpdateStatus(processName, status)
	log.Printf("[Update] Updating version tracking for %s to %s", processName, targetVersionInfo.Tag)

	if err := m.versionManager.SetCurrentVersion(processName, targetVersionInfo); err != nil {
		log.Printf("[Update] Warning: failed to update version tracking for %s: %v", processName, err)
	}

	// Complete
	status.Stage = "completed"
	status.Message = fmt.Sprintf("Successfully updated to version %s", targetVersionInfo.Tag)
	status.Progress = 100
	status.Completed = true
	m.setUpdateStatus(processName, status)
	log.Printf("[Update] ✅ Update completed for %s: v%s", processName, targetVersionInfo.Tag)
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
// Format: binaries/processName/processName_version.exe
func (m *Manager) getBinaryPath(processName, version string) string {
	return filepath.Join(m.binariesDir, processName, fmt.Sprintf("%s_%s.exe", processName, version))
}

// getBinaryPathForRepository returns the path where a binary should be stored using repository name
// Format: binaries/repositoryName/repositoryName_version.exe
// Example: yhonda-ohishi-pub-dev/db_service -> binaries/db_service/db_service_v1.12.3.exe
func (m *Manager) getBinaryPathForRepository(repository, version string) string {
	// Extract repository name from full path (e.g., "yhonda-ohishi-pub-dev/db_service" -> "db_service")
	parts := filepath.ToSlash(repository)
	repoName := filepath.Base(parts)
	return filepath.Join(m.binariesDir, repoName, fmt.Sprintf("%s_%s.exe", repoName, version))
}

// CheckForUpdates checks for available updates for a process
func (m *Manager) CheckForUpdates(processName, repository string) (*models.VersionInfo, error) {
	return m.versionManager.CheckForUpdates(processName, repository)
}

// getRepoLock gets or creates a mutex for a specific repository
func (m *Manager) getRepoLock(repository string) *sync.Mutex {
	m.repoLocksMu.Lock()
	defer m.repoLocksMu.Unlock()

	if _, exists := m.repoLocks[repository]; !exists {
		m.repoLocks[repository] = &sync.Mutex{}
	}
	return m.repoLocks[repository]
}
