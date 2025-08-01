package api

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "mutual-friend/api/proto"
	"mutual-friend/internal/service"
	"mutual-friend/pkg/config"
)

// FriendServiceServer implements the gRPC friend service
type FriendServiceServer struct {
	pb.UnimplementedFriendServiceServer
	friendService *service.FriendService
	eventService  *service.EventService
	config        *config.Config
}

// NewFriendServiceServer creates a new gRPC server instance
func NewFriendServiceServer(
	friendService *service.FriendService,
	eventService *service.EventService,
	config *config.Config,
) *FriendServiceServer {
	return &FriendServiceServer{
		friendService: friendService,
		eventService:  eventService,
		config:        config,
	}
}

// GRPCServer manages the gRPC server lifecycle
type GRPCServer struct {
	server        *grpc.Server
	listener      net.Listener
	config        *config.Config
	friendHandler *FriendServiceServer
}

// NewGRPCServer creates a new gRPC server with all necessary configurations
func NewGRPCServer(
	config *config.Config,
	friendService *service.FriendService,
	eventService *service.EventService,
) (*GRPCServer, error) {
	// Create listener
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", config.GRPC.Port))
	if err != nil {
		return nil, fmt.Errorf("failed to listen on port %d: %w", config.GRPC.Port, err)
	}

	// Create gRPC server with interceptors
	server := grpc.NewServer(
		grpc.UnaryInterceptor(loggingUnaryInterceptor),
		grpc.StreamInterceptor(loggingStreamInterceptor),
		grpc.MaxRecvMsgSize(4*1024*1024), // 4MB
		grpc.MaxSendMsgSize(4*1024*1024), // 4MB
	)

	// Create friend service handler
	friendHandler := NewFriendServiceServer(friendService, eventService, config)

	// Register services
	pb.RegisterFriendServiceServer(server, friendHandler)

	// Register health service
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus("friend.FriendService", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(server, healthServer)

	// Enable reflection in development
	if config.GRPC.Reflection {
		reflection.Register(server)
		log.Println("gRPC reflection enabled")
	}

	return &GRPCServer{
		server:        server,
		listener:      lis,
		config:        config,
		friendHandler: friendHandler,
	}, nil
}

// Start starts the gRPC server
func (s *GRPCServer) Start() error {
	log.Printf("Starting gRPC server on port %d", s.config.GRPC.Port)
	log.Printf("Services available:")
	log.Printf("  - friend.FriendService")
	log.Printf("  - grpc.health.v1.Health")
	if s.config.GRPC.Reflection {
		log.Printf("  - grpc.reflection.v1alpha.ServerReflection")
	}

	return s.server.Serve(s.listener)
}

// Stop gracefully stops the gRPC server
func (s *GRPCServer) Stop() {
	log.Println("Shutting down gRPC server...")
	s.server.GracefulStop()
}

// loggingUnaryInterceptor logs all unary RPC calls
func loggingUnaryInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	start := time.Now()
	
	// Call the handler
	resp, err := handler(ctx, req)
	
	duration := time.Since(start)
	
	// Log the request
	if err != nil {
		log.Printf("RPC %s failed in %v: %v", info.FullMethod, duration, err)
	} else {
		log.Printf("RPC %s completed in %v", info.FullMethod, duration)
	}
	
	return resp, err
}

// loggingStreamInterceptor logs all streaming RPC calls  
func loggingStreamInterceptor(
	srv interface{},
	stream grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	start := time.Now()
	
	// Call the handler
	err := handler(srv, stream)
	
	duration := time.Since(start)
	
	// Log the request
	if err != nil {
		log.Printf("Stream RPC %s failed in %v: %v", info.FullMethod, duration, err)
	} else {
		log.Printf("Stream RPC %s completed in %v", info.FullMethod, duration)
	}
	
	return err
}

// Helper function to convert errors to gRPC status errors
func toGRPCError(err error) error {
	if err == nil {
		return nil
	}

	// You can add more sophisticated error mapping here
	switch {
	case contains(err.Error(), "not found"):
		return status.Errorf(codes.NotFound, "%v", err)
	case contains(err.Error(), "already exists"):
		return status.Errorf(codes.AlreadyExists, "%v", err)
	case contains(err.Error(), "invalid"):
		return status.Errorf(codes.InvalidArgument, "%v", err)
	case contains(err.Error(), "unauthorized"):
		return status.Errorf(codes.Unauthenticated, "%v", err)
	case contains(err.Error(), "forbidden"):
		return status.Errorf(codes.PermissionDenied, "%v", err)
	default:
		return status.Errorf(codes.Internal, "%v", err)
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		(len(s) > len(substr) && 
			(s[:len(substr)] == substr || 
			 s[len(s)-len(substr):] == substr ||
			 containsInMiddle(s, substr))))
}

func containsInMiddle(s, substr string) bool {
	for i := 1; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// HealthCheck implements the health check RPC
func (s *FriendServiceServer) HealthCheck(
	ctx context.Context,
	req *emptypb.Empty,
) (*pb.HealthCheckResponse, error) {
	// Check if event service is healthy
	isHealthy := s.eventService.IsHealthy()
	
	status := "SERVING"
	message := "All services are healthy"
	
	if !isHealthy {
		status = "NOT_SERVING"
		message = "Event service is not healthy"
	}
	
	return &pb.HealthCheckResponse{
		Status:    status,
		Message:   message,
		Timestamp: timestamppb.New(time.Now()),
	}, nil
}