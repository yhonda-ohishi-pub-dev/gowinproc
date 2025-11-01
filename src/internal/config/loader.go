package config

import (
	"fmt"
	"os"
	"time"

	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/pkg/models"
	"gopkg.in/yaml.v3"
)

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*models.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg models.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	setDefaults(&cfg)

	// Validate configuration
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets default values for configuration
func setDefaults(cfg *models.Config) {
	// Server defaults
	if cfg.Server.Host == "" {
		cfg.Server.Host = "localhost"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.GRPCPort == 0 {
		cfg.Server.GRPCPort = 50051
	}
	if cfg.Server.ReadTimeout == 0 {
		cfg.Server.ReadTimeout = 30 * time.Second
	}
	if cfg.Server.WriteTimeout == 0 {
		cfg.Server.WriteTimeout = 30 * time.Second
	}

	// Secrets defaults
	if cfg.Secrets.Mode == "" {
		cfg.Secrets.Mode = "standalone"
	}

	// GitHub defaults
	if cfg.GitHub.Mode == "" {
		cfg.GitHub.Mode = "standalone"
	}

	// Update mode defaults
	if cfg.GitHub.UpdateMode.Polling == nil && cfg.GitHub.UpdateMode.Webhook == nil {
		// Default to polling mode
		cfg.GitHub.UpdateMode.Polling = &models.PollingConfig{
			Enabled:  true,
			Interval: 5 * time.Minute,
		}
	}

	// Process defaults
	for i := range cfg.Processes {
		p := &cfg.Processes[i]
		if p.WorkDir == "" {
			p.WorkDir = "."
		}
		if p.MaxInstances == 0 {
			p.MaxInstances = 1
		}
		if p.HealthCheck.Interval == 0 {
			p.HealthCheck.Interval = 30 * time.Second
		}
		if p.HealthCheck.Timeout == 0 {
			p.HealthCheck.Timeout = 5 * time.Second
		}
		if p.HealthCheck.Retries == 0 {
			p.HealthCheck.Retries = 3
		}
	}
}

// validate validates the configuration
func validate(cfg *models.Config) error {
	if len(cfg.Processes) == 0 {
		return fmt.Errorf("at least one process must be configured")
	}

	// Validate secrets configuration
	if cfg.Secrets.Mode == "cloudflare" {
		if cfg.Secrets.Cloudflare == nil {
			return fmt.Errorf("cloudflare secrets configuration is required when mode is 'cloudflare'")
		}
		if cfg.Secrets.Cloudflare.WorkerURL == "" {
			return fmt.Errorf("cloudflare worker URL is required")
		}
		if cfg.Secrets.Cloudflare.PrivateKeyPath == "" {
			return fmt.Errorf("cloudflare private key path is required")
		}
	}

	// Validate GitHub configuration
	if cfg.GitHub.Mode == "cloudflare" {
		// GitHub Cloudflare config is optional if secrets mode is also cloudflare
		// (In that case, we use the secrets worker URL for both secrets and repo list)
		if cfg.GitHub.Cloudflare != nil {
			if cfg.GitHub.Cloudflare.WorkerURL == "" {
				return fmt.Errorf("github worker URL is required when github.cloudflare is specified")
			}
			if cfg.GitHub.Cloudflare.PrivateKeyPath == "" {
				return fmt.Errorf("github private key path is required when github.cloudflare is specified")
			}
		} else if cfg.Secrets.Mode != "cloudflare" {
			// If secrets mode is not cloudflare, we need github.cloudflare config
			return fmt.Errorf("cloudflare github configuration is required when mode is 'cloudflare' and secrets mode is not 'cloudflare'")
		}
	}

	// Validate processes
	for i, p := range cfg.Processes {
		if p.Name == "" {
			return fmt.Errorf("process[%d]: name is required", i)
		}
		if p.Repository == "" {
			return fmt.Errorf("process[%d]: repository is required", i)
		}
		// binary_path is now optional - will be auto-detected if not provided
		// port is now optional - will be auto-allocated if not provided
		if p.MaxInstances < 1 {
			return fmt.Errorf("process[%d]: max_instances must be at least 1", i)
		}
	}

	// Validate tunnel configuration
	if cfg.Tunnel != nil && cfg.Tunnel.Enabled {
		if cfg.Tunnel.Port == 0 {
			cfg.Tunnel.Port = cfg.Server.Port
		}
		if cfg.Tunnel.Protocol == "" {
			cfg.Tunnel.Protocol = "http2"
		}
		if cfg.Tunnel.Protocol != "http2" && cfg.Tunnel.Protocol != "quic" {
			return fmt.Errorf("tunnel protocol must be 'http2' or 'quic'")
		}
	}

	return nil
}
