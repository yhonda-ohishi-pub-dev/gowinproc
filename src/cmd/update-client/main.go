package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pb "github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/proto"
)

func main() {
	processName := flag.String("process", "", "Process name to update")
	version := flag.String("version", "", "Target version (empty = latest)")
	force := flag.Bool("force", false, "Force update even if already on target version")
	server := flag.String("server", "localhost:50051", "gRPC server address")
	flag.Parse()

	if *processName == "" {
		log.Fatal("--process is required")
	}

	// Connect to gRPC server
	conn, err := grpc.Dial(*server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewProcessManagerClient(conn)

	// Call UpdateProcess
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.UpdateProcessRequest{
		ProcessName: *processName,
		Version:     *version,
		Force:       *force,
	}

	fmt.Printf("Requesting update for %s...\n", *processName)
	if *version != "" {
		fmt.Printf("Target version: %s\n", *version)
	} else {
		fmt.Println("Target version: latest")
	}
	fmt.Printf("Force: %v\n", *force)

	resp, err := client.UpdateProcess(ctx, req)
	if err != nil {
		log.Fatalf("Update failed: %v", err)
	}

	if resp.Success {
		fmt.Printf("✓ Update started successfully\n")
		fmt.Printf("Update ID: %s\n", resp.UpdateId)
		fmt.Printf("Message: %s\n", resp.Message)
	} else {
		fmt.Printf("✗ Update failed: %s\n", resp.Message)
	}
}
