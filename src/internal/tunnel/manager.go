package tunnel

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/pkg/models"
)

// Manager handles Cloudflare Tunnel (cloudflared) operations
type Manager struct {
	config  *models.TunnelConfig
	cmd     *exec.Cmd
	running bool
	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewManager creates a new tunnel manager
func NewManager(config *models.TunnelConfig) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		config: config,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start starts the cloudflared tunnel
func (m *Manager) Start() error {
	if !m.config.Enabled {
		log.Println("Cloudflare Tunnel is disabled")
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("tunnel is already running")
	}

	// Check if cloudflared is installed
	cloudflaredPath, err := m.findCloudflared()
	if err != nil {
		return fmt.Errorf("cloudflared not found: %w", err)
	}

	log.Printf("Starting Cloudflare Tunnel on port %d with protocol %s", m.config.Port, m.config.Protocol)

	// Build cloudflared command
	// cloudflared tunnel --url http://localhost:8080 --protocol http2
	args := []string{
		"tunnel",
		"--url", fmt.Sprintf("http://localhost:%d", m.config.Port),
		"--protocol", m.config.Protocol,
		"--no-autoupdate",
	}

	m.cmd = exec.CommandContext(m.ctx, cloudflaredPath, args...)

	// Set up output logging
	m.cmd.Stdout = &tunnelLogger{prefix: "[cloudflared] "}
	m.cmd.Stderr = &tunnelLogger{prefix: "[cloudflared] "}

	// Start the process
	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start cloudflared: %w", err)
	}

	m.running = true

	// Monitor the process
	go m.monitor()

	log.Println("Cloudflare Tunnel started successfully")
	return nil
}

// Stop stops the cloudflared tunnel
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	log.Println("Stopping Cloudflare Tunnel...")

	m.cancel()

	if m.cmd != nil && m.cmd.Process != nil {
		if err := m.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill cloudflared: %w", err)
		}
	}

	// Wait for process to exit
	if m.cmd != nil {
		m.cmd.Wait()
	}

	m.running = false
	log.Println("Cloudflare Tunnel stopped")
	return nil
}

// IsRunning returns whether the tunnel is running
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// monitor monitors the cloudflared process
func (m *Manager) monitor() {
	if m.cmd == nil {
		return
	}

	err := m.cmd.Wait()

	m.mu.Lock()
	m.running = false
	m.mu.Unlock()

	if err != nil {
		log.Printf("Cloudflare Tunnel exited with error: %v", err)
	} else {
		log.Println("Cloudflare Tunnel exited")
	}

	// Auto-restart if context is not cancelled
	select {
	case <-m.ctx.Done():
		// Context cancelled, don't restart
		return
	default:
		// Auto-restart after 5 seconds
		log.Println("Restarting Cloudflare Tunnel in 5 seconds...")
		time.Sleep(5 * time.Second)
		if err := m.Start(); err != nil {
			log.Printf("Failed to restart tunnel: %v", err)
		}
	}
}

// findCloudflared finds the cloudflared executable
func (m *Manager) findCloudflared() (string, error) {
	// Check common locations on Windows
	commonPaths := []string{
		"cloudflared.exe",
		"C:\\Program Files\\cloudflared\\cloudflared.exe",
		"C:\\Program Files (x86)\\cloudflared\\cloudflared.exe",
		filepath.Join(os.Getenv("USERPROFILE"), ".cloudflared", "cloudflared.exe"),
	}

	// Check in PATH
	if path, err := exec.LookPath("cloudflared"); err == nil {
		return path, nil
	}

	// Check common paths
	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("cloudflared.exe not found in PATH or common locations")
}

// tunnelLogger is a simple logger for cloudflared output
type tunnelLogger struct {
	prefix string
}

func (l *tunnelLogger) Write(p []byte) (n int, err error) {
	log.Printf("%s%s", l.prefix, string(p))
	return len(p), nil
}
