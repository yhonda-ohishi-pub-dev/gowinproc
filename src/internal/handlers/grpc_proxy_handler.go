package handlers

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/process"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// GrpcProxyHandler handles dynamic gRPC-Web proxy requests
type GrpcProxyHandler struct {
	processManager *process.Manager
	wrapperCache   map[string]*grpcweb.WrappedGrpcServer
	connCache      map[string]*grpc.ClientConn
	cacheMu        sync.RWMutex
}

// NewGrpcProxyHandler creates a new gRPC proxy handler
func NewGrpcProxyHandler(procMgr *process.Manager) *GrpcProxyHandler {
	return &GrpcProxyHandler{
		processManager: procMgr,
		wrapperCache:   make(map[string]*grpcweb.WrappedGrpcServer),
		connCache:      make(map[string]*grpc.ClientConn),
	}
}

// director is a function that modifies the context and returns the backend connection
type director func(ctx context.Context, fullMethodName string) (context.Context, *grpc.ClientConn, error)

// transparentHandler creates a transparent proxy handler
func (h *GrpcProxyHandler) transparentHandler(conn *grpc.ClientConn) grpc.StreamHandler {
	return func(srv interface{}, serverStream grpc.ServerStream) error {
		// Get method name from stream context
		fullMethodName, ok := grpc.MethodFromServerStream(serverStream)
		if !ok {
			return fmt.Errorf("failed to get method name from stream")
		}

		// Extract metadata from incoming stream
		md, ok := metadata.FromIncomingContext(serverStream.Context())
		if !ok {
			md = metadata.New(nil)
		}

		// Create outgoing context with metadata
		ctx := metadata.NewOutgoingContext(serverStream.Context(), md)

		// Create client stream to backend
		clientStream, err := conn.NewStream(ctx, &grpc.StreamDesc{
			StreamName:    fullMethodName,
			ServerStreams: true,
			ClientStreams: true,
		}, fullMethodName)
		if err != nil {
			return fmt.Errorf("failed to create backend stream: %w", err)
		}

		// Proxy messages bidirectionally
		errChan := make(chan error, 2)

		// Forward client -> server
		go func() {
			for {
				msg := make([]byte, 65536) // Max gRPC message size
				if err := serverStream.RecvMsg(&msg); err != nil {
					if err == io.EOF {
						clientStream.CloseSend()
						errChan <- nil
						return
					}
					errChan <- err
					return
				}
				if err := clientStream.SendMsg(msg); err != nil {
					errChan <- err
					return
				}
			}
		}()

		// Forward server -> client
		go func() {
			for {
				msg := make([]byte, 65536)
				if err := clientStream.RecvMsg(&msg); err != nil {
					if err == io.EOF {
						errChan <- nil
						return
					}
					errChan <- err
					return
				}
				if err := serverStream.SendMsg(msg); err != nil {
					errChan <- err
					return
				}
			}
		}()

		// Wait for completion
		err1 := <-errChan
		err2 := <-errChan

		if err1 != nil {
			return err1
		}
		return err2
	}
}

// GetOrCreateWrapper gets or creates a gRPC-Web wrapper for a process
func (h *GrpcProxyHandler) GetOrCreateWrapper(processName string, port int) (*grpcweb.WrappedGrpcServer, error) {
	cacheKey := fmt.Sprintf("%s:%d", processName, port)

	// Check cache first
	h.cacheMu.RLock()
	if wrapper, ok := h.wrapperCache[cacheKey]; ok {
		h.cacheMu.RUnlock()
		return wrapper, nil
	}
	h.cacheMu.RUnlock()

	// Create new connection and wrapper
	h.cacheMu.Lock()
	defer h.cacheMu.Unlock()

	// Double-check after acquiring write lock
	if wrapper, ok := h.wrapperCache[cacheKey]; ok {
		return wrapper, nil
	}

	// Create gRPC client connection to backend process
	targetAddress := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := grpc.Dial(targetAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", processName, err)
	}

	// Create a transparent proxy gRPC server
	// This server will forward all calls to the backend connection
	proxyServer := grpc.NewServer(
		grpc.UnknownServiceHandler(h.transparentHandler(conn)),
	)

	// Wrap the proxy server with gRPC-Web
	wrapper := grpcweb.WrapServer(proxyServer,
		grpcweb.WithCorsForRegisteredEndpointsOnly(false),
		grpcweb.WithOriginFunc(func(origin string) bool {
			return true // Allow all origins
		}),
		grpcweb.WithWebsockets(true),
		grpcweb.WithWebsocketOriginFunc(func(req *http.Request) bool {
			return true
		}),
	)

	// Store in cache
	h.wrapperCache[cacheKey] = wrapper
	h.connCache[cacheKey] = conn

	log.Printf("Created gRPC-Web wrapper for %s (port %d)", processName, port)
	return wrapper, nil
}

// ProxyRequest proxies a gRPC-Web request to a native gRPC backend
func (h *GrpcProxyHandler) ProxyRequest(w http.ResponseWriter, r *http.Request, processName string, grpcPath string) {
	// Get process port dynamically
	instances, err := h.processManager.GetProcessStatus(processName)
	if err != nil || len(instances) == 0 || instances[0].Port <= 0 {
		http.Error(w, fmt.Sprintf("Process %s is not running or has no port assigned", processName), http.StatusServiceUnavailable)
		return
	}

	port := instances[0].Port

	// Get or create gRPC-Web wrapper
	wrapper, err := h.GetOrCreateWrapper(processName, port)
	if err != nil {
		log.Printf("Failed to create wrapper for %s: %v", processName, err)
		http.Error(w, fmt.Sprintf("Failed to create proxy: %v", err), http.StatusInternalServerError)
		return
	}

	// Modify request path to remove /proxy/{processName} prefix
	r.URL.Path = grpcPath

	// Proxy the request through gRPC-Web wrapper
	wrapper.ServeHTTP(w, r)
}
