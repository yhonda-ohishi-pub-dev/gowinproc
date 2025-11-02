package tunnel

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sync"
	"time"

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

// registerTunnelURL registers the tunnel URL with Cloudflare Auth Worker
func (m *Manager) registerTunnelURL(tunnelURL string) {
	log.Printf("[Tunnel] Registering tunnel URL with Auth Worker: %s", tunnelURL)

	// Step 1: Authenticate and get token
	token, err := m.authenticate()
	if err != nil {
		log.Printf("[Tunnel] ERROR: Failed to authenticate: %v", err)
		return
	}

	// Step 2: Register tunnel URL
	reqBody := map[string]string{
		"clientId":  m.config.ClientID,
		"tunnelUrl": tunnelURL,
		"token":     token,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		log.Printf("[Tunnel] ERROR: Failed to marshal request: %v", err)
		return
	}

	registerURL := m.config.WorkerURL + "/tunnel/register"
	resp, err := http.Post(registerURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("[Tunnel] ERROR: Failed to register tunnel URL: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[Tunnel] ERROR: Failed to register tunnel URL (status %d): %s", resp.StatusCode, string(body))
		return
	}

	log.Printf("[Tunnel] Successfully registered tunnel URL: %s", tunnelURL)
}

// authenticate performs RSA authentication with Cloudflare Auth Worker
func (m *Manager) authenticate() (string, error) {
	// Load private key
	privateKey, err := loadPrivateKey(m.config.PrivateKeyPath)
	if err != nil {
		return "", fmt.Errorf("failed to load private key: %w", err)
	}

	// Generate challenge
	challenge := make([]byte, 32)
	if _, err := rand.Read(challenge); err != nil {
		return "", fmt.Errorf("failed to generate challenge: %w", err)
	}
	challengeB64 := base64.StdEncoding.EncodeToString(challenge)

	// Sign challenge
	hashed := sha256.Sum256(challenge)
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, 0, hashed[:])
	if err != nil {
		return "", fmt.Errorf("failed to sign challenge: %w", err)
	}
	signatureB64 := base64.StdEncoding.EncodeToString(signature)

	// Send authentication request
	reqBody := map[string]string{
		"clientId":  m.config.ClientID,
		"challenge": challengeB64,
		"signature": signatureB64,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal auth request: %w", err)
	}

	verifyURL := m.config.WorkerURL + "/verify"
	resp, err := http.Post(verifyURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to send auth request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("authentication failed (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response to get token
	var authResp struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return "", fmt.Errorf("failed to decode auth response: %w", err)
	}

	return authResp.Token, nil
}

// loadPrivateKey loads an RSA private key from a PEM file
func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	keyData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return privateKey, nil
}

// tunnelLogger is a simple logger for cloudflared output
type tunnelLogger struct {
	prefix string
}

func (l *tunnelLogger) Write(p []byte) (n int, err error) {
	log.Printf("%s%s", l.prefix, string(p))
	return len(p), nil
}
