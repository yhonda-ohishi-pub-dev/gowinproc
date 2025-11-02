package reflection

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// FieldDetail represents a field in a message
type FieldDetail struct {
	Name     string `json:"name"`
	Type     string `json:"type"`      // e.g., "string", "int32", "bool"
	Repeated bool   `json:"repeated"`  // true if this is a repeated field
	Number   int32  `json:"number"`    // field number
	Optional bool   `json:"optional"`  // true if this field is optional
}

// MessageDetail represents a protobuf message with its fields
type MessageDetail struct {
	Name   string        `json:"name"`
	Fields []FieldDetail `json:"fields"`
}

// MethodDetail represents a gRPC method with input/output types
type MethodDetail struct {
	Name       string `json:"name"`
	InputType  string `json:"input_type"`
	OutputType string `json:"output_type"`
}

// ServiceInfo represents a gRPC service information
type ServiceInfo struct {
	Services map[string][]MethodDetail  `json:"services"` // service name -> method details
	Messages map[string]MessageDetail   `json:"messages"` // message type name -> message schema
}

// ServiceMethod represents a service and its methods
type ServiceMethod struct {
	ServiceName string         `json:"service"`
	Methods     []MethodDetail `json:"methods"`
}

// Client is a gRPC reflection client
type Client struct {
	timeout time.Duration
}

// NewClient creates a new reflection client
func NewClient() *Client {
	return &Client{
		timeout: 5 * time.Second,
	}
}

// GetServiceInfo retrieves service information from a gRPC server
func (c *Client) GetServiceInfo(ctx context.Context, address string) (*ServiceInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Connect to the gRPC server
	conn, err := grpc.DialContext(ctx, address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", address, err)
	}
	defer conn.Close()

	// Create reflection client
	refClient := grpc_reflection_v1alpha.NewServerReflectionClient(conn)
	stream, err := refClient.ServerReflectionInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create reflection stream: %w", err)
	}

	// Request list of services
	if err := stream.Send(&grpc_reflection_v1alpha.ServerReflectionRequest{
		MessageRequest: &grpc_reflection_v1alpha.ServerReflectionRequest_ListServices{
			ListServices: "",
		},
	}); err != nil {
		return nil, fmt.Errorf("failed to send list services request: %w", err)
	}

	// Receive response
	resp, err := stream.Recv()
	if err != nil {
		return nil, fmt.Errorf("failed to receive response: %w", err)
	}

	// Parse service list
	serviceList, ok := resp.MessageResponse.(*grpc_reflection_v1alpha.ServerReflectionResponse_ListServicesResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type")
	}

	services := make(map[string][]MethodDetail)
	messages := make(map[string]MessageDetail)
	processedFiles := make(map[string]bool) // Track processed files to avoid duplicates

	for _, service := range serviceList.ListServicesResponse.Service {
		serviceName := service.Name

		// Skip reflection service itself
		if serviceName == "grpc.reflection.v1alpha.ServerReflection" {
			continue
		}

		// Request service descriptor to get methods
		if err := stream.Send(&grpc_reflection_v1alpha.ServerReflectionRequest{
			MessageRequest: &grpc_reflection_v1alpha.ServerReflectionRequest_FileContainingSymbol{
				FileContainingSymbol: serviceName,
			},
		}); err != nil {
			continue
		}

		methodResp, err := stream.Recv()
		if err != nil {
			continue
		}

		// Parse FileDescriptorProto to extract methods and messages
		methods, fileMessages := c.extractMethodsAndMessages(methodResp, serviceName, processedFiles)
		services[serviceName] = methods

		// Merge messages into the global messages map
		for msgName, msgDetail := range fileMessages {
			messages[msgName] = msgDetail
		}
	}

	return &ServiceInfo{
		Services: services,
		Messages: messages,
	}, nil
}

// extractMethods parses FileDescriptorProto from reflection response and extracts method details
func (c *Client) extractMethods(resp *grpc_reflection_v1alpha.ServerReflectionResponse, serviceName string) []MethodDetail {
	fdResp, ok := resp.MessageResponse.(*grpc_reflection_v1alpha.ServerReflectionResponse_FileDescriptorResponse)
	if !ok {
		return []MethodDetail{}
	}

	var methods []MethodDetail
	for _, fdBytes := range fdResp.FileDescriptorResponse.FileDescriptorProto {
		fd := &descriptorpb.FileDescriptorProto{}
		if err := proto.Unmarshal(fdBytes, fd); err != nil {
			continue
		}

		// Find the service in the file descriptor
		for _, svc := range fd.Service {
			fullServiceName := fd.GetPackage() + "." + svc.GetName()
			if fullServiceName == serviceName {
				// Extract all method details from this service
				for _, method := range svc.Method {
					methods = append(methods, MethodDetail{
						Name:       method.GetName(),
						InputType:  method.GetInputType(),
						OutputType: method.GetOutputType(),
					})
				}
			}
		}
	}

	return methods
}

// extractMethodsAndMessages parses FileDescriptorProto and extracts both methods and message schemas
func (c *Client) extractMethodsAndMessages(resp *grpc_reflection_v1alpha.ServerReflectionResponse, serviceName string, processedFiles map[string]bool) ([]MethodDetail, map[string]MessageDetail) {
	fdResp, ok := resp.MessageResponse.(*grpc_reflection_v1alpha.ServerReflectionResponse_FileDescriptorResponse)
	if !ok {
		return []MethodDetail{}, map[string]MessageDetail{}
	}

	var methods []MethodDetail
	messages := make(map[string]MessageDetail)

	for _, fdBytes := range fdResp.FileDescriptorResponse.FileDescriptorProto {
		fd := &descriptorpb.FileDescriptorProto{}
		if err := proto.Unmarshal(fdBytes, fd); err != nil {
			continue
		}

		// Skip if we've already processed this file
		fileName := fd.GetName()
		if processedFiles[fileName] {
			continue
		}
		processedFiles[fileName] = true

		// Extract methods for the requested service
		for _, svc := range fd.Service {
			fullServiceName := fd.GetPackage() + "." + svc.GetName()
			if fullServiceName == serviceName {
				for _, method := range svc.Method {
					methods = append(methods, MethodDetail{
						Name:       method.GetName(),
						InputType:  method.GetInputType(),
						OutputType: method.GetOutputType(),
					})
				}
			}
		}

		// Extract all message schemas from this file
		for _, msgType := range fd.MessageType {
			fullMsgName := "." + fd.GetPackage() + "." + msgType.GetName()

			var fields []FieldDetail
			for _, field := range msgType.Field {
				fieldType := c.getFieldTypeName(field)
				fields = append(fields, FieldDetail{
					Name:     field.GetName(),
					Type:     fieldType,
					Repeated: field.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REPEATED,
					Number:   field.GetNumber(),
					Optional: field.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL,
				})
			}

			messages[fullMsgName] = MessageDetail{
				Name:   fullMsgName,
				Fields: fields,
			}
		}
	}

	return methods, messages
}

// getFieldTypeName converts FieldDescriptorProto type to a readable string
func (c *Client) getFieldTypeName(field *descriptorpb.FieldDescriptorProto) string {
	switch field.GetType() {
	case descriptorpb.FieldDescriptorProto_TYPE_DOUBLE:
		return "double"
	case descriptorpb.FieldDescriptorProto_TYPE_FLOAT:
		return "float"
	case descriptorpb.FieldDescriptorProto_TYPE_INT64:
		return "int64"
	case descriptorpb.FieldDescriptorProto_TYPE_UINT64:
		return "uint64"
	case descriptorpb.FieldDescriptorProto_TYPE_INT32:
		return "int32"
	case descriptorpb.FieldDescriptorProto_TYPE_FIXED64:
		return "fixed64"
	case descriptorpb.FieldDescriptorProto_TYPE_FIXED32:
		return "fixed32"
	case descriptorpb.FieldDescriptorProto_TYPE_BOOL:
		return "bool"
	case descriptorpb.FieldDescriptorProto_TYPE_STRING:
		return "string"
	case descriptorpb.FieldDescriptorProto_TYPE_BYTES:
		return "bytes"
	case descriptorpb.FieldDescriptorProto_TYPE_UINT32:
		return "uint32"
	case descriptorpb.FieldDescriptorProto_TYPE_SFIXED32:
		return "sfixed32"
	case descriptorpb.FieldDescriptorProto_TYPE_SFIXED64:
		return "sfixed64"
	case descriptorpb.FieldDescriptorProto_TYPE_SINT32:
		return "sint32"
	case descriptorpb.FieldDescriptorProto_TYPE_SINT64:
		return "sint64"
	case descriptorpb.FieldDescriptorProto_TYPE_MESSAGE:
		return field.GetTypeName() // Returns full message type name like ".package.MessageName"
	case descriptorpb.FieldDescriptorProto_TYPE_ENUM:
		return field.GetTypeName() // Returns full enum type name
	default:
		return "unknown"
	}
}

// GetSimpleServiceList returns just the service names without detailed method info
func (c *Client) GetSimpleServiceList(ctx context.Context, address string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", address, err)
	}
	defer conn.Close()

	refClient := grpc_reflection_v1alpha.NewServerReflectionClient(conn)
	stream, err := refClient.ServerReflectionInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create reflection stream: %w", err)
	}

	if err := stream.Send(&grpc_reflection_v1alpha.ServerReflectionRequest{
		MessageRequest: &grpc_reflection_v1alpha.ServerReflectionRequest_ListServices{
			ListServices: "",
		},
	}); err != nil {
		return nil, fmt.Errorf("failed to send list services request: %w", err)
	}

	resp, err := stream.Recv()
	if err != nil {
		return nil, fmt.Errorf("failed to receive response: %w", err)
	}

	serviceList, ok := resp.MessageResponse.(*grpc_reflection_v1alpha.ServerReflectionResponse_ListServicesResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type")
	}

	var services []string
	for _, service := range serviceList.ListServicesResponse.Service {
		if service.Name != "grpc.reflection.v1alpha.ServerReflection" {
			services = append(services, service.Name)
		}
	}

	return services, nil
}
