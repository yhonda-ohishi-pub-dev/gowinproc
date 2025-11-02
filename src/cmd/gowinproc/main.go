package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc"

	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/api"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/certs"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/cloudflare"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/config"
	grpcserver "github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/grpc"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/handlers"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/loadbalancer"
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

	// Setup log file
	logFile, err := os.OpenFile("gowinproc.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer logFile.Close()

	// Log to both file and console
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(multiWriter)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// Check for multiple instances using lock file
	lockFilePath := filepath.Join(os.TempDir(), "gowinproc.lock")
	lockFile, err := os.OpenFile(lockFilePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
	if err != nil {
		if os.IsExist(err) {
			// Lock file exists, kill the old process and take over
			log.Printf("Lock file exists, killing old gowinproc process...")
			lockContent, readErr := os.ReadFile(lockFilePath)
			if readErr == nil {
				oldPID, parseErr := strconv.Atoi(strings.TrimSpace(string(lockContent)))
				if parseErr == nil && oldPID != os.Getpid() {
					log.Printf("Killing old process PID: %d", oldPID)
					killCmd := exec.Command("taskkill", "/F", "/PID", strconv.Itoa(oldPID))
					if killErr := killCmd.Run(); killErr != nil {
						log.Printf("Warning: Failed to kill old process: %v", killErr)
					} else {
						log.Printf("Successfully killed old process PID: %d", oldPID)
					}
				}
			}
			// Remove old lock file
			os.Remove(lockFilePath)
			// Try to create lock file again
			lockFile, err = os.OpenFile(lockFilePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
			if err != nil {
				log.Fatalf("Failed to create lock file after cleanup: %v", err)
			}
		} else {
			log.Fatalf("Failed to create lock file: %v", err)
		}
	}
	defer func() {
		lockFile.Close()
		os.Remove(lockFilePath)
	}()
	fmt.Fprintf(lockFile, "%d", os.Getpid())
	log.Printf("Lock file created with PID: %d", os.Getpid())

	// Clean up existing processes on startup
	log.Printf("Cleaning up existing processes...")
	if err := cleanupExistingProcesses(); err != nil {
		log.Printf("Warning: Failed to cleanup existing processes: %v", err)
	}

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

	// Get GitHub token from Cloudflare, flag, or environment
	token := *githubToken
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}

	// Try to get GITHUB_TOKEN from Cloudflare if in cloudflare mode and token is still empty
	if token == "" && cfg.Secrets.Mode == "cloudflare" {
		log.Printf("Fetching GITHUB_TOKEN from Cloudflare...")
		authClient, err := cloudflare.NewAuthClient(
			cfg.Secrets.Cloudflare.WorkerURL,
			cfg.Secrets.Cloudflare.PrivateKeyPath,
			"gowinproc",
		)
		if err == nil {
			authResult, err := authClient.Authenticate()
			if err == nil {
				if githubToken, exists := authResult.SecretData["GITHUB_TOKEN"]; exists {
					token = githubToken
					log.Printf("GitHub token retrieved from Cloudflare")
				} else {
					log.Printf("Warning: GITHUB_TOKEN not found in Cloudflare secrets")
				}
			} else {
				log.Printf("Warning: Failed to fetch secrets from Cloudflare: %v", err)
			}
		}
	}

	if token != "" {
		log.Printf("GitHub API authentication enabled (rate limit: 5000/hour)")
	} else {
		log.Printf("Warning: No GitHub token configured (rate limit: 60/hour)")
	}

	// Initialize version manager
	versionManager, err := version.NewManager(*dataDir, token)
	if err != nil {
		log.Fatalf("Failed to create version manager: %v", err)
	}
	log.Printf("Version manager initialized")

	// Initialize process manager
	processManager := process.NewManager(cfg, certManager, secretManager)
	processManager.SetVersionManager(versionManager)
	if err := processManager.Initialize(); err != nil {
		log.Fatalf("Failed to initialize process manager: %v", err)
	}
	log.Printf("Process manager initialized with %d processes", len(cfg.Processes))

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

	// Initialize gRPC server with dynamic port if needed
	grpcPort := cfg.Server.GRPCPort
	grpcAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, grpcPort)

	// Try to listen on the configured port, if it fails, find an available port
	listener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Printf("Port %d is in use, finding available port...", grpcPort)
		// Find an available port starting from grpcPort + 1
		for port := grpcPort + 1; port < grpcPort + 100; port++ {
			testAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, port)
			listener, err = net.Listen("tcp", testAddr)
			if err == nil {
				grpcPort = port
				grpcAddr = testAddr
				log.Printf("Using alternative gRPC port: %d", grpcPort)
				break
			}
		}
		if listener == nil {
			log.Fatalf("Failed to find available gRPC port")
		}
	}

	grpcSrv := grpc.NewServer()
	grpcServiceServer := grpcserver.NewServer(processManager, updateManager, repositoryList)
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

	// Setup signal channel for shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

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

	// gRPC レジストリエンドポイント - プロキシ可能なプロセス一覧を返す
	registryHandler := handlers.NewRegistryHandler(processManager, cfg.Server.Host, cfg.Server.Port)
	mux.HandleFunc("/api/grpc/registry", registryHandler.GetRegistry)

	// gRPC Invokeエンドポイント - JSON経由でgRPCメソッドを実行
	grpcInvokeHandler := handlers.NewGrpcInvokeHandler(processManager)
	mux.HandleFunc("/api/grpc/invoke", grpcInvokeHandler.InvokeMethod)

	// gRPC-Web プロキシハンドラー - ネイティブgRPCバックエンドへのプロキシ
	grpcProxyHandler := handlers.NewGrpcProxyHandler(processManager)

	// Graceful shutdown endpoint (for replacing running instances)
	mux.HandleFunc("/shutdown", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		log.Println("Received graceful shutdown request via HTTP")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"shutting down"}`))

		// Trigger shutdown asynchronously
		go func() {
			time.Sleep(500 * time.Millisecond) // Allow response to be sent
			sigChan <- syscall.SIGTERM
		}()
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
				// Handle client disconnection gracefully without logging errors
				// This is normal behavior during hot reloads and page refreshes
				defer func() {
					if r := recover(); r != nil {
						// Suppress panic from client disconnection
						log.Printf("Debug: gRPC-Web request recovered from panic (client likely disconnected): %v", r)
					}
				}()

				// Dynamic gRPC-Web proxy: route to managed processes
				// Format: /proxy/{process_name}/{grpc_service_path}
				if len(r.URL.Path) > 7 && r.URL.Path[:7] == "/proxy/" {
					// Extract process name from path
					pathParts := r.URL.Path[7:] // Remove "/proxy/"
					slashIdx := -1
					for i, ch := range pathParts {
						if ch == '/' {
							slashIdx = i
							break
						}
					}

					if slashIdx > 0 {
						processName := pathParts[:slashIdx]
						grpcPath := pathParts[slashIdx:] // Keep the leading /

						// Use gRPC-Web proxy handler to forward to native gRPC backend
						grpcProxyHandler.ProxyRequest(w, r, processName, grpcPath)
						return
					}
				}

				// Default: forward to gowinproc's own gRPC server
				wrappedGrpc.ServeHTTP(w, r)
				return
			}
			// Otherwise, use the regular HTTP mux
			mux.ServeHTTP(w, r)
		}),
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		// Don't close idle connections too quickly - allow for hot reloads
		IdleTimeout: 120 * time.Second,
		// Disable HTTP/2 connection management that can cause issues with gRPC-Web
		MaxHeaderBytes: 1 << 20,
	}

	// Initialize and start load balancers if configured
	var lbManager *loadbalancer.Manager
	if len(cfg.LoadBalancers) > 0 {
		lbManager, err = loadbalancer.NewManager(cfg.LoadBalancers, processManager)
		if err != nil {
			log.Fatalf("Failed to create load balancer manager: %v", err)
		}
		if err := lbManager.Start(); err != nil {
			log.Fatalf("Failed to start load balancers: %v", err)
		}
		log.Printf("Load balancers initialized and started")
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
		// Always use config file processes (not Cloudflare repository list)
		// Config processes have proper process names that match running instances
		log.Printf("Using %d processes from config file for polling", len(cfg.Processes))
		pollerProcs := make([]poller.ProcessConfig, len(cfg.Processes))
		for i, proc := range cfg.Processes {
			pollerProcs[i] = poller.ProcessConfig{
				Name:       proc.Name,
				Repository: proc.Repository,
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

	// Stop load balancers
	if lbManager != nil {
		if err := lbManager.Stop(); err != nil {
			log.Printf("Load balancer shutdown error: %v", err)
		}
	}

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

// cleanupExistingProcesses kills any existing gowinproc and db_service processes
func cleanupExistingProcesses() error {
	// Get current process PID to avoid killing ourselves
	currentPID := os.Getpid()

	// Process names to kill
	processNames := []string{"gowinproc.exe", "gowinproc-gui.exe", "db_service.exe"}

	for _, procName := range processNames {
		// Use tasklist to find processes
		cmd := fmt.Sprintf("tasklist /FI \"IMAGENAME eq %s\" /FO CSV /NH", procName)
		output, err := exec.Command("cmd", "/C", cmd).Output()
		if err != nil {
			continue
		}

		// Parse output and kill processes
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}

			// CSV format: "ImageName","PID","SessionName","SessionNumber","MemUsage"
			parts := strings.Split(line, ",")
			if len(parts) < 2 {
				continue
			}

			// Extract PID (remove quotes)
			pidStr := strings.Trim(parts[1], "\"")
			pid, err := strconv.Atoi(pidStr)
			if err != nil {
				continue
			}

			// Skip current process
			if pid == currentPID {
				log.Printf("Skipping current process: %s (PID %d)", procName, pid)
				continue
			}

			// Kill the process
			log.Printf("Killing existing process: %s (PID %d)", procName, pid)
			killCmd := exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid))
			if err := killCmd.Run(); err != nil {
				log.Printf("Warning: Failed to kill %s (PID %d): %v", procName, pid, err)
			} else {
				log.Printf("Successfully killed %s (PID %d)", procName, pid)
			}
		}
	}

	// Wait a moment for processes to terminate
	time.Sleep(1 * time.Second)

	return nil
}
