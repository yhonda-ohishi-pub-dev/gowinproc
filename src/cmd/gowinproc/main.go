package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/api"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/certs"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/config"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/poller"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/process"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/secrets"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/tunnel"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/update"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/version"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/webhook"
)

var (
	configPath  = flag.String("config", "config.yaml", "Path to configuration file")
	certsDir    = flag.String("certs", "certs", "Directory for certificates")
	keysDir     = flag.String("keys", "keys", "Directory for private keys")
	dataDir     = flag.String("data", "data", "Directory for data files (.env)")
	binariesDir = flag.String("binaries", "binaries", "Directory for binary versions")
	githubToken = flag.String("github-token", "", "GitHub personal access token (or set GITHUB_TOKEN env var)")
)

func main() {
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("gowinproc starting...")
	log.Printf("Mode: Secrets=%s, GitHub=%s", cfg.Secrets.Mode, cfg.GitHub.Mode)

	// Initialize certificate manager
	certManager, err := certs.NewManager(*certsDir, *keysDir)
	if err != nil {
		log.Fatalf("Failed to create certificate manager: %v", err)
	}
	log.Printf("Certificate manager initialized (certs: %s, keys: %s)", *certsDir, *keysDir)

	// Initialize secrets manager
	secretManager, err := secrets.NewManager(cfg, *dataDir)
	if err != nil {
		log.Fatalf("Failed to create secrets manager: %v", err)
	}
	log.Printf("Secrets manager initialized (data: %s)", *dataDir)

	// Initialize process manager
	processManager := process.NewManager(cfg, certManager, secretManager)
	if err := processManager.Initialize(); err != nil {
		log.Fatalf("Failed to initialize process manager: %v", err)
	}
	log.Printf("Process manager initialized with %d processes", len(cfg.Processes))

	// Get GitHub token from flag or environment
	token := *githubToken
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}

	// Initialize version manager
	versionManager, err := version.NewManager(*dataDir, token)
	if err != nil {
		log.Fatalf("Failed to create version manager: %v", err)
	}
	log.Printf("Version manager initialized")

	// Initialize update manager
	updateManager, err := update.NewManager(processManager, versionManager, *binariesDir)
	if err != nil {
		log.Fatalf("Failed to create update manager: %v", err)
	}
	log.Printf("Update manager initialized (binaries: %s)", *binariesDir)

	// Start configured processes
	for _, procConfig := range cfg.Processes {
		log.Printf("Starting process: %s", procConfig.Name)
		if _, err := processManager.StartProcess(procConfig.Name); err != nil {
			log.Printf("Warning: Failed to start %s: %v", procConfig.Name, err)
		} else {
			log.Printf("Process %s started successfully", procConfig.Name)
		}
	}

	// Initialize webhook handler
	webhookHandler := webhook.NewHandler(updateManager, "")

	// Initialize REST API server with webhook routes
	apiServer := api.NewServer(processManager, updateManager)
	mux := http.NewServeMux()
	mux.Handle("/api/", apiServer)
	mux.HandleFunc("/webhook/github", webhookHandler.HandleGitHubWebhook)
	mux.HandleFunc("/webhook/cloudflare", webhookHandler.HandleCloudflareWebhook)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	})

	httpServer := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start Cloudflare Tunnel if enabled
	var tunnelManager *tunnel.Manager
	if cfg.Tunnel != nil && cfg.Tunnel.Enabled {
		tunnelManager = tunnel.NewManager(cfg.Tunnel)
		if err := tunnelManager.Start(); err != nil {
			log.Printf("Warning: Failed to start Cloudflare Tunnel: %v", err)
		}
	}

	// Start GitHub version poller if configured
	var githubPoller *poller.GitHubPoller
	if cfg.GitHub.UpdateMode.Polling != nil && cfg.GitHub.UpdateMode.Polling.Enabled {
		pollerProcs := make([]poller.ProcessConfig, len(cfg.Processes))
		for i, proc := range cfg.Processes {
			pollerProcs[i] = poller.ProcessConfig{
				Name:       proc.Name,
				Repository: proc.Repository,
			}
		}
		githubPoller = poller.NewGitHubPoller(
			cfg.GitHub.Cloudflare.WorkerURL,
			cfg.GitHub.UpdateMode.Polling.Interval,
			updateManager,
			pollerProcs,
		)
		githubPoller.Start()
	}

	// Start HTTP server
	go func() {
		log.Printf("REST API server listening on %s", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down gracefully...")

	// Stop GitHub poller
	if githubPoller != nil {
		githubPoller.Stop()
	}

	// Stop Cloudflare Tunnel
	if tunnelManager != nil {
		if err := tunnelManager.Stop(); err != nil {
			log.Printf("Tunnel shutdown error: %v", err)
		}
	}

	// Shutdown HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	// Shutdown all processes
	if err := processManager.Shutdown(); err != nil {
		log.Printf("Process manager shutdown error: %v", err)
	}

	log.Println("Shutdown complete")
}
