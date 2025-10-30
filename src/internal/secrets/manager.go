package secrets

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/cloudflare"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/pkg/models"
)

// Manager handles secret management and .env file generation
type Manager struct {
	config  *models.Config
	dataDir string
}

// NewManager creates a new secrets manager
func NewManager(config *models.Config, dataDir string) (*Manager, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	return &Manager{
		config:  config,
		dataDir: dataDir,
	}, nil
}

// GenerateEnvFile generates a .env file for a process
func (m *Manager) GenerateEnvFile(processName string, certPath, keyPath string) error {
	envPath := filepath.Join(m.dataDir, fmt.Sprintf("%s.env", processName))

	// Find process config
	var processConfig *models.ProcessConfig
	for i := range m.config.Processes {
		if m.config.Processes[i].Name == processName {
			processConfig = &m.config.Processes[i]
			break
		}
	}

	if processConfig == nil {
		return fmt.Errorf("process %s not found in configuration", processName)
	}

	// Build environment variables map
	envVars := make(map[string]string)

	// Add process-specific env vars from config
	for k, v := range processConfig.Env {
		envVars[k] = v
	}

	// Add certificate paths
	envVars["CERT_FILE"] = certPath
	envVars["KEY_FILE"] = keyPath

	// Add process metadata
	envVars["PROCESS_NAME"] = processName
	envVars["PROCESS_PORT"] = fmt.Sprintf("%d", processConfig.Port)

	// Fetch secrets from Cloudflare if configured
	if m.config.Secrets.Mode == "cloudflare" {
		cloudflareEnv, err := m.fetchCloudflareSecrets(processName)
		if err != nil {
			return fmt.Errorf("failed to fetch cloudflare secrets: %w", err)
		}
		// Merge cloudflare secrets (they override local config)
		for k, v := range cloudflareEnv {
			envVars[k] = v
		}
	}

	// Write .env file
	return m.writeEnvFile(envPath, envVars)
}

// writeEnvFile writes environment variables to a .env file
func (m *Manager) writeEnvFile(path string, envVars map[string]string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create env file: %w", err)
	}
	defer file.Close()

	for k, v := range envVars {
		// Escape values that contain special characters
		escapedValue := escapeEnvValue(v)
		if _, err := fmt.Fprintf(file, "%s=%s\n", k, escapedValue); err != nil {
			return fmt.Errorf("failed to write env variable: %w", err)
		}
	}

	return nil
}

// escapeEnvValue escapes special characters in environment variable values
func escapeEnvValue(value string) string {
	// If value contains spaces, newlines, or quotes, wrap in quotes and escape
	if strings.ContainsAny(value, " \n\r\t\"'") {
		value = strings.ReplaceAll(value, "\\", "\\\\")
		value = strings.ReplaceAll(value, "\"", "\\\"")
		value = strings.ReplaceAll(value, "\n", "\\n")
		value = strings.ReplaceAll(value, "\r", "\\r")
		value = strings.ReplaceAll(value, "\t", "\\t")
		return fmt.Sprintf("\"%s\"", value)
	}
	return value
}

// LoadEnvFile loads environment variables from a .env file
func (m *Manager) LoadEnvFile(processName string) (map[string]string, error) {
	envPath := filepath.Join(m.dataDir, fmt.Sprintf("%s.env", processName))

	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("env file does not exist: %s", envPath)
	}

	return godotenv.Read(envPath)
}

// GetEnvFilePath returns the path to a process's .env file
func (m *Manager) GetEnvFilePath(processName string) string {
	return filepath.Join(m.dataDir, fmt.Sprintf("%s.env", processName))
}

// fetchCloudflareSecrets fetches secrets from Cloudflare Workers
func (m *Manager) fetchCloudflareSecrets(processName string) (map[string]string, error) {
	if m.config.Secrets.Cloudflare == nil {
		return nil, fmt.Errorf("cloudflare configuration not set")
	}

	// Create Cloudflare auth client
	authClient, err := cloudflare.NewAuthClient(
		m.config.Secrets.Cloudflare.WorkerURL,
		m.config.Secrets.Cloudflare.PrivateKeyPath,
		"gowinproc", // client ID
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth client: %w", err)
	}

	// Fetch secrets for the process
	secrets, err := authClient.GetSecrets(processName)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch secrets: %w", err)
	}

	return secrets, nil
}

// EnvFileExists checks if a .env file exists for a process
func (m *Manager) EnvFileExists(processName string) bool {
	envPath := m.GetEnvFilePath(processName)
	_, err := os.Stat(envPath)
	return err == nil
}
