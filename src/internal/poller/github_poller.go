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

// GitHubPoller polls github-webhook-worker for version updates
type GitHubPoller struct {
	workerURL     string
	interval      time.Duration
	updateManager *update.Manager
	processes     []ProcessConfig
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

// ProcessConfig represents a process to poll for updates
type ProcessConfig struct {
	Name       string
	Repository string
}

// VersionResponse represents the response from github-webhook-worker
type VersionResponse struct {
	Repository string `json:"repository"`
	Version    string `json:"version"`
	UpdatedAt  string `json:"updated_at"`
}

// NewGitHubPoller creates a new GitHub version poller
func NewGitHubPoller(workerURL string, interval time.Duration, updateMgr *update.Manager, processes []ProcessConfig) *GitHubPoller {
	ctx, cancel := context.WithCancel(context.Background())

	return &GitHubPoller{
		workerURL:     workerURL,
		interval:      interval,
		updateManager: updateMgr,
		processes:     processes,
		ctx:           ctx,
		cancel:        cancel,
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
	// Fetch version from github-webhook-worker
	url := fmt.Sprintf("%s/version/%s", p.workerURL, proc.Repository)

	req, err := http.NewRequestWithContext(p.ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch version: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var versionResp VersionResponse
	if err := json.NewDecoder(resp.Body).Decode(&versionResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Check if update is needed
	if err := p.checkAndUpdate(proc.Name, versionResp.Version); err != nil {
		return fmt.Errorf("failed to check/update: %w", err)
	}

	return nil
}

// checkAndUpdate checks if an update is needed and triggers it
func (p *GitHubPoller) checkAndUpdate(processName, latestVersion string) error {
	// Check for available updates
	// TODO: Get current version and compare
	// For now, we'll just check if an update is already in progress

	if status, exists := p.updateManager.GetUpdateStatus(processName); exists && !status.Completed {
		// Update already in progress
		return nil
	}

	// TODO: Compare versions and trigger update if needed
	// This requires integrating with version manager to get current version

	return nil
}
