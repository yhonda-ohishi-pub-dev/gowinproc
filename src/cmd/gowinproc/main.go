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
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/process"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/secrets"
)

var (
	configPath = flag.String("config", "config.yaml", "Path to configuration file")
	certsDir   = flag.String("certs", "certs", "Directory for certificates")
	keysDir    = flag.String("keys", "keys", "Directory for private keys")
	dataDir    = flag.String("data", "data", "Directory for data files (.env)")
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

	// Start configured processes
	for _, procConfig := range cfg.Processes {
		log.Printf("Starting process: %s", procConfig.Name)
		if _, err := processManager.StartProcess(procConfig.Name); err != nil {
			log.Printf("Warning: Failed to start %s: %v", procConfig.Name, err)
		} else {
			log.Printf("Process %s started successfully", procConfig.Name)
		}
	}

	// Initialize REST API server
	apiServer := api.NewServer(processManager)
	httpServer := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      apiServer,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
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
