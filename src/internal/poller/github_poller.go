package poller

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/update"
)

// GitHubPoller polls GitHub API for version updates
type GitHubPoller struct {
	interval      time.Duration
	updateManager *update.Manager
	processes     []ProcessConfig
	httpClient    *http.Client
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

// ProcessConfig represents a process to poll for updates
type ProcessConfig struct {
	Name       string
	Repository string
}

// GitHubRelease represents a GitHub release API response
type GitHubRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	PublishedAt string `json:"published_at"`
}

// NewGitHubPoller creates a new GitHub version poller
func NewGitHubPoller(interval time.Duration, updateMgr *update.Manager, processes []ProcessConfig) *GitHubPoller {
	ctx, cancel := context.WithCancel(context.Background())

	return &GitHubPoller{
		interval:      interval,
		updateManager: updateMgr,
		processes:     processes,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start starts the polling loop
func (p *GitHubPoller) Start() {
	log.Printf("Starting GitHub version poller (interval: %v)", p.interval)

	p.wg.Add(1)
	go p.pollLoop()
}

// Stop stops the polling loop
func (p *GitHubPoller) Stop() {
	log.Println("Stopping GitHub version poller...")
	p.cancel()
	p.wg.Wait()
	log.Println("GitHub version poller stopped")
}

// pollLoop runs the polling loop
func (p *GitHubPoller) pollLoop() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	// Poll immediately on start
	p.pollAll()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.pollAll()
		}
	}
}

// pollAll polls version information for all configured processes
func (p *GitHubPoller) pollAll() {
	for _, proc := range p.processes {
		if err := p.pollProcess(proc); err != nil {
			log.Printf("Failed to poll %s: %v", proc.Name, err)
		}
	}
}

// pollProcess polls version information for a single process
func (p *GitHubPoller) pollProcess(proc ProcessConfig) error {
	// Fetch latest release from GitHub API
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", proc.Repository)

	req, err := http.NewRequestWithContext(p.ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set GitHub API headers
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "gowinproc")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// No releases published yet
		log.Printf("No releases found for %s (repository may not have any releases yet)", proc.Repository)
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	log.Printf("Found release for %s: %s", proc.Repository, release.TagName)

	// Check if update is needed
	if err := p.checkAndUpdate(proc.Name, release.TagName); err != nil {
		return fmt.Errorf("failed to check/update: %w", err)
	}

	return nil
}

// checkAndUpdate checks if an update is needed and triggers it
func (p *GitHubPoller) checkAndUpdate(processName, latestVersion string) error {
	// Check if an update is already in progress
	if status, exists := p.updateManager.GetUpdateStatus(processName); exists && !status.Completed {
		// Update already in progress
		return nil
	}

	// Find repository for this process
	var repository string
	for _, proc := range p.processes {
		if proc.Name == processName {
			repository = proc.Repository
			break
		}
	}
	if repository == "" {
		return fmt.Errorf("repository not found for process %s", processName)
	}

	// Check for available updates through update manager
	versionInfo, err := p.updateManager.CheckForUpdates(processName, repository)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	// If update is available, trigger automatic update
	if versionInfo != nil && versionInfo.UpdateAvailable {
		log.Printf("Update available for %s: %s -> %s, triggering auto-update",
			processName,
			versionInfo.CurrentVersion.Tag,
			versionInfo.LatestVersion.Tag)

		// Trigger update with latest version
		if err := p.updateManager.UpdateProcess(processName, versionInfo.LatestVersion.Tag, false); err != nil {
			return fmt.Errorf("failed to trigger update: %w", err)
		}

		log.Printf("Auto-update triggered for %s to version %s", processName, versionInfo.LatestVersion.Tag)
	} else if versionInfo != nil && versionInfo.CurrentVersion != nil {
		log.Printf("Process %s is up-to-date at version %s", processName, versionInfo.CurrentVersion.Tag)
	}

	return nil
}
