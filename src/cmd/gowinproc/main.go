package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"google.golang.org/grpc"

	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/api"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/certs"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/cloudflare"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/config"
	grpcserver "github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/grpc"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/poller"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/process"
	pb "github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/proto"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/secrets"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/systray"
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

	// Ensure RSA key pair exists for Cloudflare integration
	privateKeyPath := filepath.Join(*keysDir, "client_private.pem")
	publicKeyPath := filepath.Join(*keysDir, "client_public.pem")
	if err := cloudflare.EnsureKeyPairExists(privateKeyPath, publicKeyPath, 2048); err != nil {
		log.Fatalf("Failed to ensure RSA key pair: %v", err)
	}
	log.Printf("RSA key pair ready (private: %s, public: %s)", privateKeyPath, publicKeyPath)

	// Fetch repository list from Cloudflare Auth Worker if in cloudflare mode
	var repositoryList []string
	if cfg.GitHub.Mode == "cloudflare" && cfg.Secrets.Mode == "cloudflare" {
		log.Printf("Fetching repository list from Cloudflare Auth Worker...")
		authClient, err := cloudflare.NewAuthClient(
			cfg.Secrets.Cloudflare.WorkerURL,
			cfg.Secrets.Cloudflare.PrivateKeyPath,
			"gowinproc",
		)
		if err != nil {
			log.Fatalf("Failed to create auth client: %v", err)
		}

		authResult, err := authClient.Authenticate()
		if err != nil {
			log.Fatalf("Failed to authenticate with Cloudflare: %v", err)
		}

		repositoryList = authResult.RepoList
		log.Printf("Retrieved %d repositories from Cloudflare Auth Worker:", len(repositoryList))
		for _, repo := range repositoryList {
			log.Printf("  - %s", repo)
		}
	}

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

	// Initialize gRPC server
	grpcAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.GRPCPort)
	grpcSrv := grpc.NewServer()
	grpcServiceServer := grpcserver.NewServer(processManager, updateManager)
	pb.RegisterProcessManagerServer(grpcSrv, grpcServiceServer)

	// Wrap gRPC server with gRPC-Web
	wrappedGrpc := grpcweb.WrapServer(grpcSrv,
		grpcweb.WithCorsForRegisteredEndpointsOnly(false),
		grpcweb.WithOriginFunc(func(origin string) bool {
			// Allow all origins for development
			// TODO: Configure allowed origins in production
			return true
		}),
		grpcweb.WithWebsockets(true),
		grpcweb.WithWebsocketOriginFunc(func(req *http.Request) bool {
			return true
		}),
	)

	// Initialize webhook handler
	webhookHandler := webhook.NewHandler(updateManager, "")

	// Initialize REST API server with webhook routes and gRPC-Web
	apiServer := api.NewServer(processManager, updateManager)
	mux := http.NewServeMux()
	mux.Handle("/api/", apiServer)
	mux.HandleFunc("/webhook/github", webhookHandler.HandleGitHubWebhook)
	mux.HandleFunc("/webhook/cloudflare", webhookHandler.HandleCloudflareWebhook)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	})

	// Create HTTP server with gRPC-Web support
	httpServer := &http.Server{
		Addr: fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add CORS headers for all requests
			origin := r.Header.Get("Origin")
			if origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, x-grpc-web, x-user-agent")
				w.Header().Set("Access-Control-Expose-Headers", "grpc-status, grpc-message")
			}

			// Handle preflight requests
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			// Check if this is a gRPC-Web request
			if wrappedGrpc.IsGrpcWebRequest(r) {
				wrappedGrpc.ServeHTTP(w, r)
				return
			}
			// Otherwise, use the regular HTTP mux
			mux.ServeHTTP(w, r)
		}),
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
		// Use repository list from Cloudflare if available, otherwise use config processes
		var pollerProcs []poller.ProcessConfig
		if len(repositoryList) > 0 {
			// Use Cloudflare repository list
			log.Printf("Using %d repositories from Cloudflare for polling", len(repositoryList))
			pollerProcs = make([]poller.ProcessConfig, len(repositoryList))
			for i, repo := range repositoryList {
				// Extract repository name for process name (e.g., "owner/repo" -> "repo")
				// For now, use the full repository path as the name
				pollerProcs[i] = poller.ProcessConfig{
					Name:       repo,
					Repository: repo,
				}
			}
		} else {
			// Fall back to config file processes
			log.Printf("Using %d processes from config file for polling", len(cfg.Processes))
			pollerProcs = make([]poller.ProcessConfig, len(cfg.Processes))
			for i, proc := range cfg.Processes {
				pollerProcs[i] = poller.ProcessConfig{
					Name:       proc.Name,
					Repository: proc.Repository,
				}
			}
		}

		// Create GitHub poller (uses GitHub API directly, no worker URL needed)
		githubPoller = poller.NewGitHubPoller(
			cfg.GitHub.UpdateMode.Polling.Interval,
			updateManager,
			pollerProcs,
		)
		githubPoller.Start()
	}

	// Start gRPC server (native, non-Web)
	listener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", grpcAddr, err)
	}

	go func() {
		log.Printf("gRPC server listening on %s", grpcAddr)
		if err := grpcSrv.Serve(listener); err != nil {
			log.Fatalf("gRPC server error: %v", err)
		}
	}()

	// Start HTTP server with gRPC-Web support
	go func() {
		log.Printf("REST API server listening on %s (with gRPC-Web support)", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Start system tray icon
	log.Println("Starting system tray icon...")
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	restAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	trayManager := systray.NewManager(restAddr, grpcAddr, func() {
		// Quit callback from system tray
		sigChan <- syscall.SIGTERM
	})
	trayManager.Start()

	// Wait for interrupt signal
	<-sigChan

	log.Println("Shutting down gracefully...")

	// Stop system tray
	trayManager.Stop()

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

	// Shutdown gRPC server
	log.Println("Stopping gRPC server...")
	grpcSrv.GracefulStop()

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
