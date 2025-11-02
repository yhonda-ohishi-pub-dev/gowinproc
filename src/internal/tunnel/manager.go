package tunnel

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sync"
	"syscall"
	"time"

	"github.com/yhonda-ohishi-pub-dev/go_auth/pkg/authclient"
	"github.com/yhonda-ohishi-pub-dev/go_auth/pkg/keygen"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/pkg/models"
)

// Manager handles Cloudflare Tunnel (cloudflared) operations
type Manager struct {
	config    *models.TunnelConfig
	cmd       *exec.Cmd
	running   bool
	tunnelURL string // Current tunnel URL (https://xxx.trycloudflare.com)
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
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

	// Windows: Hide console window for cloudflared
	m.cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}

	// Set up output logging with URL extraction
	stdoutPipe, err := m.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderrPipe, err := m.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start goroutines to parse output and extract tunnel URL
	go m.parseOutput(stdoutPipe, "[cloudflared] ")
	go m.parseOutput(stderrPipe, "[cloudflared] ")

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
	m.tunnelURL = "" // Clear tunnel URL
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

// parseOutput parses cloudflared output to extract tunnel URL
func (m *Manager) parseOutput(reader io.Reader, prefix string) {
	scanner := bufio.NewScanner(reader)
	// Regular expression to match trycloudflare.com URLs
	urlRegex := regexp.MustCompile(`https://[a-zA-Z0-9-]+\.trycloudflare\.com`)

	for scanner.Scan() {
		line := scanner.Text()
		log.Printf("%s%s", prefix, line)

		// Try to extract tunnel URL
		if matches := urlRegex.FindString(line); matches != "" {
			m.mu.Lock()
			if m.tunnelURL == "" {
				m.tunnelURL = matches
				log.Printf("[Tunnel] Detected tunnel URL: %s", m.tunnelURL)

				// Register URL with Cloudflare Auth Worker (if configured)
				if m.config.ClientID != "" && m.config.WorkerURL != "" && m.config.PrivateKeyPath != "" {
					go m.registerTunnelURL(m.tunnelURL)
				}
			}
			m.mu.Unlock()
		}
	}
}

// GetTunnelURL returns the current tunnel URL
func (m *Manager) GetTunnelURL() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tunnelURL
}

// registerTunnelURL registers the tunnel URL with Cloudflare Auth Worker using go_auth library
func (m *Manager) registerTunnelURL(tunnelURL string) {
	log.Printf("[Tunnel] Registering tunnel URL with Auth Worker: %s", tunnelURL)

	// Load private key
	privateKey, err := keygen.LoadPrivateKey(m.config.PrivateKeyPath)
	if err != nil {
		log.Printf("[Tunnel] ERROR: Failed to load private key: %v", err)
		return
	}

	// Create auth client with Tunnel URL
	client, err := authclient.NewClient(authclient.ClientConfig{
		BaseURL:    m.config.WorkerURL,
		ClientID:   m.config.ClientID,
		PrivateKey: privateKey,
		TunnelUrl:  tunnelURL,
	})
	if err != nil {
		log.Printf("[Tunnel] ERROR: Failed to create auth client: %v", err)
		return
	}

	// Authenticate (this will register the Tunnel URL)
	result, err := client.Authenticate()
	if err != nil {
		log.Printf("[Tunnel] ERROR: Failed to authenticate and register: %v", err)
		return
	}

	log.Printf("[Tunnel] Successfully registered tunnel URL: %s (token: %s)", tunnelURL, result.Token)
}

// tunnelLogger is a simple logger for cloudflared output
type tunnelLogger struct {
	prefix string
}

func (l *tunnelLogger) Write(p []byte) (n int, err error) {
	log.Printf("%s%s", l.prefix, string(p))
	return len(p), nil
}
