package loadbalancer

import (
	"fmt"
	"io"
	"log"
	"net"
	"regexp"
	"sync"
	"sync/atomic"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/process"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/pkg/models"
)

// Balancer implements a gRPC load balancer
type Balancer struct {
	config         models.LoadBalancerConfig
	processManager *process.Manager
	server         *grpc.Server
	listener       net.Listener

	// Routing
	routes []*routeHandler

	// For round-robin strategy
	counters map[string]*uint64 // key: route index
}

// routeHandler represents a compiled route
type routeHandler struct {
	methodPatterns []*regexp.Regexp
	targetProcs    []string
	strategy       string
}

// NewBalancer creates a new load balancer
func NewBalancer(config models.LoadBalancerConfig, procMgr *process.Manager) (*Balancer, error) {
	if config.Protocol != "grpc" {
		return nil, fmt.Errorf("only gRPC protocol is supported, got: %s", config.Protocol)
	}

	lb := &Balancer{
		config:         config,
		processManager: procMgr,
		counters:       make(map[string]*uint64),
	}

	// Compile route patterns
	for i, route := range config.Routes {
		handler := &routeHandler{
			methodPatterns: make([]*regexp.Regexp, len(route.Methods)),
			targetProcs:    route.TargetProcesses,
			strategy:       route.Strategy,
		}

		for j, pattern := range route.Methods {
			re, err := regexp.Compile(pattern)
			if err != nil {
				return nil, fmt.Errorf("invalid method pattern %q in route %d: %w", pattern, i, err)
			}
			handler.methodPatterns[j] = re
		}

		lb.routes = append(lb.routes, handler)

		// Initialize counter for round-robin
		if route.Strategy == "round_robin" {
			counter := uint64(0)
			lb.counters[fmt.Sprintf("route_%d", i)] = &counter
		}
	}

	return lb, nil
}

// Start starts the load balancer server
func (lb *Balancer) Start() error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", lb.config.ListenPort))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", lb.config.ListenPort, err)
	}
	lb.listener = listener

	// Create gRPC server with unknown service handler
	lb.server = grpc.NewServer(
		grpc.UnknownServiceHandler(lb.proxyHandler),
	)

	log.Printf("Load balancer %q starting on port %d", lb.config.Name, lb.config.ListenPort)

	go func() {
		if err := lb.server.Serve(listener); err != nil {
			log.Printf("Load balancer %q server error: %v", lb.config.Name, err)
		}
	}()

	return nil
}

// Stop stops the load balancer
func (lb *Balancer) Stop() error {
	if lb.server != nil {
		lb.server.GracefulStop()
	}
	if lb.listener != nil {
		lb.listener.Close()
	}
	log.Printf("Load balancer %q stopped", lb.config.Name)
	return nil
}

// proxyHandler handles all incoming gRPC requests
func (lb *Balancer) proxyHandler(srv interface{}, stream grpc.ServerStream) error {
	// Get method name from stream context
	method, ok := grpc.MethodFromServerStream(stream)
	if !ok {
		return status.Error(codes.Internal, "failed to get method from stream")
	}

	// Find matching route
	route := lb.findRoute(method)
	if route == nil {
		return status.Errorf(codes.Unimplemented, "no route found for method: %s", method)
	}

	// Select backend based on strategy
	backend, err := lb.selectBackend(route)
	if err != nil {
		return status.Errorf(codes.Unavailable, "no available backend: %v", err)
	}

	// Create client connection to backend
	conn, err := grpc.Dial(backend, grpc.WithInsecure())
	if err != nil {
		return status.Errorf(codes.Unavailable, "failed to connect to backend %s: %v", backend, err)
	}
	defer conn.Close()

	// Proxy the request
	ctx := stream.Context()
	clientStream, err := conn.NewStream(ctx, &grpc.StreamDesc{
		StreamName:    method,
		ServerStreams: true,
		ClientStreams: true,
	}, method)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to create client stream: %v", err)
	}

	// Proxy metadata
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		ctx = metadata.NewOutgoingContext(ctx, md)
	}

	// Bidirectional streaming proxy
	var wg sync.WaitGroup
	var clientErr, serverErr error

	// Client -> Server
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			var msg interface{}
			if err := stream.RecvMsg(&msg); err != nil {
				clientErr = err
				return
			}
			if err := clientStream.SendMsg(msg); err != nil {
				clientErr = err
				return
			}
		}
	}()

	// Server -> Client
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			var msg interface{}
			if err := clientStream.RecvMsg(&msg); err != nil {
				serverErr = err
				return
			}
			if err := stream.SendMsg(msg); err != nil {
				serverErr = err
				return
			}
		}
	}()

	wg.Wait()

	// Check errors
	if clientErr != nil && clientErr != io.EOF {
		return clientErr
	}
	if serverErr != nil && serverErr != io.EOF {
		return serverErr
	}

	return nil
}

// findRoute finds a matching route for the given method
func (lb *Balancer) findRoute(method string) *routeHandler {
	for _, route := range lb.routes {
		for _, pattern := range route.methodPatterns {
			if pattern.MatchString(method) {
				return route
			}
		}
	}
	return nil
}

// selectBackend selects a backend based on the route strategy
func (lb *Balancer) selectBackend(route *routeHandler) (string, error) {
	// Get all healthy instances from target processes
	backends := lb.getHealthyBackends(route.targetProcs)
	if len(backends) == 0 {
		return "", fmt.Errorf("no healthy backends available")
	}

	switch route.strategy {
	case "primary":
		// Return first healthy backend
		return backends[0], nil

	case "round_robin":
		// Round-robin across all healthy backends
		counter := lb.counters[fmt.Sprintf("route_%p", route)]
		if counter == nil {
			c := uint64(0)
			counter = &c
			lb.counters[fmt.Sprintf("route_%p", route)] = counter
		}
		idx := atomic.AddUint64(counter, 1) % uint64(len(backends))
		return backends[idx], nil

	case "least_connections":
		// TODO: Implement connection tracking
		// For now, fallback to round-robin
		counter := lb.counters[fmt.Sprintf("route_%p", route)]
		if counter == nil {
			c := uint64(0)
			counter = &c
			lb.counters[fmt.Sprintf("route_%p", route)] = counter
		}
		idx := atomic.AddUint64(counter, 1) % uint64(len(backends))
		return backends[idx], nil

	default:
		return "", fmt.Errorf("unknown strategy: %s", route.strategy)
	}
}

// getHealthyBackends returns addresses of all healthy backend instances
func (lb *Balancer) getHealthyBackends(processNames []string) []string {
	var backends []string

	for _, procName := range processNames {
		instances, err := lb.processManager.GetProcessStatus(procName)
		if err != nil {
			log.Printf("Failed to get status for process %s: %v", procName, err)
			continue
		}

		for _, inst := range instances {
			if inst.GetStatus() == "running" && inst.Port > 0 {
				backends = append(backends, fmt.Sprintf("localhost:%d", inst.Port))
			}
		}
	}

	return backends
}
