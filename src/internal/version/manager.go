package version

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/github"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/pkg/models"
)

// Manager handles version tracking and management
type Manager struct {
	dataDir      string
	githubClient *github.Client
	versions     map[string]*models.VersionInfo
	mu           sync.RWMutex
}

// NewManager creates a new version manager
func NewManager(dataDir string, githubToken string) (*Manager, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	return &Manager{
		dataDir:      dataDir,
		githubClient: github.NewClient(githubToken),
		versions:     make(map[string]*models.VersionInfo),
	}, nil
}

// LoadVersionInfo loads version information for a process
func (m *Manager) LoadVersionInfo(processName string) (*models.VersionInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check cache first
	if info, exists := m.versions[processName]; exists {
		return info, nil
	}

	// Load from disk
	versionFile := m.getVersionFilePath(processName)
	if _, err := os.Stat(versionFile); os.IsNotExist(err) {
		// No version info exists yet
		info := &models.VersionInfo{
			ProcessName: processName,
		}
		m.versions[processName] = info
		return info, nil
	}

	data, err := os.ReadFile(versionFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read version file: %w", err)
	}

	var info models.VersionInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("failed to parse version file: %w", err)
	}

	m.versions[processName] = &info
	return &info, nil
}

// SaveVersionInfo saves version information for a process
func (m *Manager) SaveVersionInfo(info *models.VersionInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.versions[info.ProcessName] = info

	versionFile := m.getVersionFilePath(info.ProcessName)
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal version info: %w", err)
	}

	if err := os.WriteFile(versionFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write version file: %w", err)
	}

	return nil
}

// SetCurrentVersion sets the current version for a process
func (m *Manager) SetCurrentVersion(processName string, version *models.Version) error {
	info, err := m.LoadVersionInfo(processName)
	if err != nil {
		return err
	}

	info.CurrentVersion = version

	// Add to history if not already present
	found := false
	for _, v := range info.History {
		if v.Tag == version.Tag {
			found = true
			break
		}
	}
	if !found {
		info.History = append([]models.Version{*version}, info.History...)
		// Keep only last 10 versions in history
		if len(info.History) > 10 {
			info.History = info.History[:10]
		}
	}

	return m.SaveVersionInfo(info)
}

// CheckForUpdates checks if there's a newer version available
func (m *Manager) CheckForUpdates(processName, repository string) (*models.VersionInfo, error) {
	info, err := m.LoadVersionInfo(processName)
	if err != nil {
		return nil, err
	}

	// Fetch latest release from GitHub
	latestVersion, err := m.githubClient.GetLatestRelease(repository)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest release: %w", err)
	}

	info.LatestVersion = latestVersion

	// Check if update is available
	if info.CurrentVersion == nil {
		info.UpdateAvailable = true
	} else {
		info.UpdateAvailable = info.CurrentVersion.Tag != latestVersion.Tag
	}

	// Save updated info
	if err := m.SaveVersionInfo(info); err != nil {
		return nil, err
	}

	return info, nil
}

// GetVersion fetches a specific version from GitHub
func (m *Manager) GetVersion(repository, tag string) (*models.Version, error) {
	return m.githubClient.GetRelease(repository, tag)
}

// ListVersions lists available versions from GitHub
func (m *Manager) ListVersions(repository string, limit int) ([]models.Version, error) {
	return m.githubClient.ListReleases(repository, limit)
}

// GetPreviousVersion returns the previous version from history
func (m *Manager) GetPreviousVersion(processName string) (*models.Version, error) {
	info, err := m.LoadVersionInfo(processName)
	if err != nil {
		return nil, err
	}

	if len(info.History) < 2 {
		return nil, fmt.Errorf("no previous version available")
	}

	// Return the second item in history (first is current)
	prev := info.History[1]
	return &prev, nil
}

// DownloadVersion downloads a version binary
func (m *Manager) DownloadVersion(version *models.Version, destPath string, progressCallback func(downloaded, total int64)) error {
	if version.AssetURL == "" {
		return fmt.Errorf("no asset URL available for version %s", version.Tag)
	}

	return m.githubClient.DownloadAsset(version.AssetURL, destPath, progressCallback)
}

// getVersionFilePath returns the path to the version info file
func (m *Manager) getVersionFilePath(processName string) string {
	return filepath.Join(m.dataDir, fmt.Sprintf("%s.version.json", processName))
}

// GetCurrentVersion returns the current version for a process
func (m *Manager) GetCurrentVersion(processName string) (*models.Version, error) {
	info, err := m.LoadVersionInfo(processName)
	if err != nil {
		return nil, err
	}

	if info.CurrentVersion == nil {
		return nil, fmt.Errorf("no current version set")
	}

	return info.CurrentVersion, nil
}
