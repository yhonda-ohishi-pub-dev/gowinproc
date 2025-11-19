package grpc

import (
	"context"
	"encoding/json"
	"fmt"

	pb "github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/proto"
	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/handlers"
)

// TunnelServiceServer implements the TunnelService gRPC service
type TunnelServiceServer struct {
	pb.UnimplementedTunnelServiceServer
	registryHandler *handlers.RegistryHandler
	invokeHandler   *handlers.GrpcInvokeHandler
}

// NewTunnelServiceServer creates a new TunnelService server
func NewTunnelServiceServer(
	regHandler *handlers.RegistryHandler,
	invHandler *handlers.GrpcInvokeHandler,
) *TunnelServiceServer {
	return &TunnelServiceServer{
		registryHandler: regHandler,
		invokeHandler:   invHandler,
	}
}

// GetRegistry implements TunnelService.GetRegistry
func (s *TunnelServiceServer) GetRegistry(
	ctx context.Context,
	req *pb.TunnelRegistryRequest,
) (*pb.TunnelRegistryResponse, error) {
	// Get registry data using existing handler logic
	registry := s.registryHandler.GetRegistryData()

	// Convert to protobuf response
	response := &pb.TunnelRegistryResponse{
		ProxyBaseUrl:       registry.ProxyBaseURL,
		AvailableProcesses: make([]*pb.TunnelProcessInfo, 0, len(registry.AvailableProcesses)),
		Timestamp:          registry.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
	}

	// Convert each process info
	for _, proc := range registry.AvailableProcesses {
		pbProc := &pb.TunnelProcessInfo{}
		pbProc.Name = proc.Name
		pbProc.DisplayName = proc.DisplayName
		pbProc.Status = proc.Status
		pbProc.Instances = int32(proc.Instances)
		pbProc.ProxyPath = proc.ProxyPath
		pbProc.Repository = proc.Repository
		pbProc.CurrentPorts = make([]int32, 0, len(proc.CurrentPorts))
		pbProc.Services = make([]*pb.ServiceDetail, 0, len(proc.Services))
		pbProc.Messages = make(map[string]*pb.MessageDetail)

		// Convert current ports
		for _, port := range proc.CurrentPorts {
			pbProc.CurrentPorts = append(pbProc.CurrentPorts, int32(port))
		}

		// Convert services
		for _, svc := range proc.Services {
			pbSvc := &pb.ServiceDetail{}
			pbSvc.Name = svc.Name
			pbSvc.Methods = make([]*pb.MethodDetail, 0, len(svc.Methods))

			for _, method := range svc.Methods {
				pbMethod := &pb.MethodDetail{}
				pbMethod.Name = method.Name
				pbMethod.InputType = method.InputType
				pbMethod.OutputType = method.OutputType
				pbSvc.Methods = append(pbSvc.Methods, pbMethod)
			}

			pbProc.Services = append(pbProc.Services, pbSvc)
		}

		// Convert messages
		for msgName, msg := range proc.Messages {
			pbMsg := &pb.MessageDetail{}
			pbMsg.Name = msg.Name
			pbMsg.Fields = make([]*pb.FieldDetail, 0, len(msg.Fields))

			for _, field := range msg.Fields {
				pbField := &pb.FieldDetail{}
				pbField.Name = field.Name
				pbField.Type = field.Type
				pbField.Repeated = field.Repeated
				pbField.Number = field.Number
				pbField.Optional = field.Optional
				pbMsg.Fields = append(pbMsg.Fields, pbField)
			}

			pbProc.Messages[msgName] = pbMsg
		}

		response.AvailableProcesses = append(response.AvailableProcesses, pbProc)
	}

	return response, nil
}

// InvokeMethod implements TunnelService.InvokeMethod
func (s *TunnelServiceServer) InvokeMethod(
	ctx context.Context,
	req *pb.TunnelInvokeRequest,
) (*pb.TunnelInvokeResponse, error) {
	// Parse JSON data string into map
	var dataMap map[string]interface{}
	if req.Data != "" {
		if err := json.Unmarshal([]byte(req.Data), &dataMap); err != nil {
			return &pb.TunnelInvokeResponse{
				Success: false,
				Error:   fmt.Sprintf("Invalid JSON data: %v", err),
			}, nil
		}
	} else {
		dataMap = make(map[string]interface{})
	}

	// Create handler request
	handlerReq := handlers.InvokeRequest{
		Process: req.Process,
		Service: req.Service,
		Method:  req.Method,
		Data:    dataMap,
	}

	// Invoke using existing handler logic
	result, err := s.invokeHandler.InvokeMethodDirect(ctx, &handlerReq)
	if err != nil {
		return &pb.TunnelInvokeResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Convert result to JSON string
	resultJSON, err := json.Marshal(result.Data)
	if err != nil {
		return &pb.TunnelInvokeResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to marshal result: %v", err),
		}, nil
	}

	return &pb.TunnelInvokeResponse{
		Success: result.Success,
		Data:    string(resultJSON),
		Error:   result.Error,
	}, nil
}
