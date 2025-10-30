package models

import "time"

// Config represents the main configuration for gowinproc
type Config struct {
	Server    ServerConfig     `yaml:"server"`
	Processes []ProcessConfig  `yaml:"processes"`
	Secrets   SecretsConfig    `yaml:"secrets"`
	GitHub    GitHubConfig     `yaml:"github"`
	Tunnel    *TunnelConfig    `yaml:"tunnel,omitempty"`
}

// ServerConfig contains the server configuration
type ServerConfig struct {
	Host         string        `yaml:"host"`
	Port         int           `yaml:"port"`
	GRPCPort     int           `yaml:"grpc_port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}

// ProcessConfig contains configuration for a managed process
type ProcessConfig struct {
	Name         string            `yaml:"name"`
	Repository   string            `yaml:"repository"`
	BinaryPath   string            `yaml:"binary_path"`
	Args         []string          `yaml:"args,omitempty"`
	Env          map[string]string `yaml:"env,omitempty"`
	WorkDir      string            `yaml:"work_dir,omitempty"`
	Port         int               `yaml:"port"`
	HealthCheck  HealthCheckConfig `yaml:"health_check"`
	AutoRestart  bool              `yaml:"auto_restart"`
	MaxInstances int               `yaml:"max_instances"`
}

// HealthCheckConfig contains health check configuration
type HealthCheckConfig struct {
	Enabled  bool          `yaml:"enabled"`
	Endpoint string        `yaml:"endpoint"`
	Interval time.Duration `yaml:"interval"`
	Timeout  time.Duration `yaml:"timeout"`
	Retries  int           `yaml:"retries"`
}

// SecretsConfig contains secret management configuration
type SecretsConfig struct {
	Mode       string                  `yaml:"mode"` // "standalone" or "cloudflare"
	Cloudflare *CloudflareSecretsConfig `yaml:"cloudflare,omitempty"`
}

// CloudflareSecretsConfig contains Cloudflare-specific secret configuration
type CloudflareSecretsConfig struct {
	WorkerURL      string `yaml:"worker_url"`
	PrivateKeyPath string `yaml:"private_key_path"`
}

// GitHubConfig contains GitHub integration configuration
type GitHubConfig struct {
	Mode       string                 `yaml:"mode"` // "standalone" or "cloudflare"
	Cloudflare *CloudflareGitHubConfig `yaml:"cloudflare,omitempty"`
	UpdateMode UpdateModeConfig       `yaml:"update_mode"`
}

// CloudflareGitHubConfig contains Cloudflare GitHub webhook configuration
type CloudflareGitHubConfig struct {
	WorkerURL      string `yaml:"worker_url"`
	PrivateKeyPath string `yaml:"private_key_path"`
}

// UpdateModeConfig specifies how updates are triggered
type UpdateModeConfig struct {
	Polling *PollingConfig `yaml:"polling,omitempty"`
	Webhook *WebhookConfig `yaml:"webhook,omitempty"`
}

// PollingConfig contains polling mode configuration
type PollingConfig struct {
	Enabled  bool          `yaml:"enabled"`
	Interval time.Duration `yaml:"interval"`
}

// WebhookConfig contains webhook mode configuration
type WebhookConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

// TunnelConfig contains Cloudflare Tunnel configuration
type TunnelConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Port     int    `yaml:"port"`
	Protocol string `yaml:"protocol"` // "http2" or "quic"
}
