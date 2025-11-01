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

var (
	serverAddr = flag.String("server", "localhost:9090", "gRPC server address")
)

func main() {
	flag.Parse()

	// Connect to gRPC server
	conn, err := grpc.NewClient(*serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewProcessManagerClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Example 1: List all processes
	fmt.Println("=== List Processes ===")
	listResp, err := client.ListProcesses(ctx, &pb.ListProcessesRequest{})
	if err != nil {
		log.Fatalf("ListProcesses failed: %v", err)
	}
	fmt.Printf("Found %d processes:\n", listResp.Count)
	for _, name := range listResp.ProcessNames {
		fmt.Printf("  - %s\n", name)
	}

	// Example 2: Get process info
	if len(listResp.ProcessNames) > 0 {
		processName := listResp.ProcessNames[0]
		fmt.Printf("\n=== Get Process Info: %s ===\n", processName)

		procInfo, err := client.GetProcess(ctx, &pb.GetProcessRequest{
			ProcessName: processName,
		})
		if err != nil {
			log.Printf("GetProcess failed: %v", err)
		} else {
			fmt.Printf("Name: %s\n", procInfo.Name)
			fmt.Printf("Instance Count: %d\n", procInfo.InstanceCount)
			for _, inst := range procInfo.Instances {
				fmt.Printf("  Instance %s:\n", inst.Id)
				fmt.Printf("    PID: %d\n", inst.Pid)
				fmt.Printf("    Status: %s\n", inst.Status)
				fmt.Printf("    Port: %d\n", inst.Port)
				fmt.Printf("    Start Time: %s\n", time.Unix(inst.StartTime, 0).Format(time.RFC3339))
			}
		}

		// Example 3: Get metrics
		fmt.Printf("\n=== Get Metrics: %s ===\n", processName)
		metrics, err := client.GetMetrics(ctx, &pb.GetMetricsRequest{
			ProcessName: processName,
		})
		if err != nil {
			log.Printf("GetMetrics failed: %v", err)
		} else {
			fmt.Printf("Process: %s\n", metrics.ProcessName)
			for _, inst := range metrics.Instances {
				fmt.Printf("  Instance %s:\n", inst.InstanceId)
				fmt.Printf("    CPU Usage: %.2f%%\n", inst.CpuUsage)
				fmt.Printf("    Memory Usage: %d bytes\n", inst.MemoryUsage)
				fmt.Printf("    Uptime: %d seconds\n", inst.Uptime)
			}
		}

		// Example 4: Get version info
		fmt.Printf("\n=== Get Version Info: %s ===\n", processName)
		versionInfo, err := client.GetProcessVersion(ctx, &pb.GetVersionRequest{
			ProcessName: processName,
		})
		if err != nil {
			log.Printf("GetProcessVersion failed: %v", err)
		} else {
			fmt.Printf("Current Version: %s\n", versionInfo.CurrentVersion)
			fmt.Printf("Latest Version: %s\n", versionInfo.LatestVersion)
			fmt.Printf("Update Available: %v\n", versionInfo.UpdateAvailable)
		}
	}

	// Example 5: Start a new process instance (commented out for safety)
	// fmt.Printf("\n=== Start Process Instance ===\n")
	// startResp, err := client.StartProcess(ctx, &pb.StartProcessRequest{
	// 	ProcessName: "my-service",
	// })
	// if err != nil {
	// 	log.Printf("StartProcess failed: %v", err)
	// } else {
	// 	fmt.Printf("Started process: %s with %d instances\n", startResp.Name, startResp.InstanceCount)
	// }

	// Example 6: Update a process (commented out for safety)
	// fmt.Printf("\n=== Update Process ===\n")
	// updateResp, err := client.UpdateProcess(ctx, &pb.UpdateProcessRequest{
	// 	ProcessName: "my-service",
	// 	Version:     "", // empty = latest
	// 	Force:       false,
	// 	Strategy:    "rolling",
	// })
	// if err != nil {
	// 	log.Printf("UpdateProcess failed: %v", err)
	// } else {
	// 	fmt.Printf("Update initiated: %s\n", updateResp.UpdateId)
	// 	fmt.Printf("Success: %v\n", updateResp.Success)
	// 	fmt.Printf("Message: %s\n", updateResp.Message)
	//
	// 	// Watch update progress
	// 	if updateResp.Success {
	// 		fmt.Printf("\n=== Watch Update Progress ===\n")
	// 		stream, err := client.WatchUpdate(ctx, &pb.WatchUpdateRequest{
	// 			UpdateId: updateResp.UpdateId,
	// 		})
	// 		if err != nil {
	// 			log.Printf("WatchUpdate failed: %v", err)
	// 		} else {
	// 			for {
	// 				status, err := stream.Recv()
	// 				if err != nil {
	// 					break
	// 				}
	// 				fmt.Printf("[%s] %s: %d%% - %s\n",
	// 					time.Unix(status.Timestamp, 0).Format(time.RFC3339),
	// 					status.ProcessName,
	// 					status.Progress,
	// 					status.Message,
	// 				)
	// 				if status.Status == "completed" || status.Status == "failed" {
	// 					break
	// 				}
	// 			}
	// 		}
	// 	}
	// }

	fmt.Println("\n=== gRPC Client Test Complete ===")
}
