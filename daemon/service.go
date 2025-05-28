package daemon

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	engine "github.com/b-open-io/agent-master-engine"
	pb "github.com/b-open-io/agent-master-engine/daemon/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Service implements the gRPC service
type Service struct {
	pb.UnimplementedAgentMasterDaemonServer
	daemon *Daemon
}

// NewService creates a new service instance
func NewService(d *Daemon) *Service {
	return &Service{daemon: d}
}

// Server management

func (s *Service) AddServer(ctx context.Context, req *pb.AddServerRequest) (*pb.ServerResponse, error) {
	config := protoToServerConfig(req.Name, req.Config)
	
	if err := s.daemon.engine.AddServer(req.Name, config.ServerConfig); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to add server: %v", err)
	}
	
	server, err := s.daemon.engine.GetServer(req.Name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get server: %v", err)
	}
	
	return &pb.ServerResponse{
		Server: serverToProto(req.Name, *server),
	}, nil
}

func (s *Service) UpdateServer(ctx context.Context, req *pb.UpdateServerRequest) (*pb.ServerResponse, error) {
	config := protoToServerConfig(req.Name, req.Config)
	
	if err := s.daemon.engine.UpdateServer(req.Name, config.ServerConfig); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to update server: %v", err)
	}
	
	server, err := s.daemon.engine.GetServer(req.Name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get server: %v", err)
	}
	
	return &pb.ServerResponse{
		Server: serverToProto(req.Name, *server),
	}, nil
}

func (s *Service) RemoveServer(ctx context.Context, req *pb.RemoveServerRequest) (*emptypb.Empty, error) {
	if err := s.daemon.engine.RemoveServer(req.Name); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to remove server: %v", err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Service) GetServer(ctx context.Context, req *pb.GetServerRequest) (*pb.ServerResponse, error) {
	server, err := s.daemon.engine.GetServer(req.Name)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "server not found: %v", err)
	}
	
	return &pb.ServerResponse{
		Server: serverToProto(req.Name, *server),
	}, nil
}

// Enable/Disable are implemented in service_fixes.go

// ListServers is implemented in service_fixes.go as ListServersCorrected
func (s *Service) ListServers(ctx context.Context, req *pb.ListServersRequest) (*pb.ListServersResponse, error) {
	return s.ListServersCorrected(ctx, req)
}

// Sync operations

func (s *Service) SyncTo(ctx context.Context, req *pb.SyncToRequest) (*pb.SyncResult, error) {
	dest, err := s.daemon.engine.GetDestination(req.Destination)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "destination not found: %v", err)
	}
	
	options := engine.SyncOptions{
		Force:     req.Options.Force,
		DryRun:    req.Options.DryRun,
	}
	
	result, err := s.daemon.engine.SyncTo(ctx, dest, options)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "sync failed: %v", err)
	}
	
	return syncResultToProto(result), nil
}

func (s *Service) SyncToMultiple(ctx context.Context, req *pb.SyncToMultipleRequest) (*pb.MultiSyncResult, error) {
	var dests []engine.Destination
	for _, name := range req.Destinations {
		dest, err := s.daemon.engine.GetDestination(name)
		if err != nil {
			return nil, status.Errorf(codes.NotFound, "destination %s not found: %v", name, err)
		}
		dests = append(dests, dest)
	}
	
	options := engine.SyncOptions{
		Force:     req.Options.Force,
		DryRun:    req.Options.DryRun,
	}
	
	result, err := s.daemon.engine.SyncToMultiple(ctx, dests, options)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "multi-sync failed: %v", err)
	}
	
	// Convert to proto format
	protoResult := &pb.MultiSyncResult{
		Results:      make(map[string]*pb.SyncResult),
		TotalSuccess: 0,
		TotalFailed:  0,
	}
	
	for i, res := range result.Results {
		// Use destination name from the result
		destName := res.Destination
		protoResult.Results[destName] = syncResultToProto(&res)
		if res.Success {
			protoResult.TotalSuccess++
		} else {
			protoResult.TotalFailed++
		}
		_ = i // unused
	}
	
	return protoResult, nil
}

// Auto-sync management

func (s *Service) StartAutoSync(ctx context.Context, req *pb.AutoSyncConfig) (*emptypb.Empty, error) {
	config := autoSyncConfigToEngine(req)
	
	// Convert to engine.AutoSyncConfig
	engineConfig := engine.AutoSyncConfig{
		Enabled:       config.Enabled,
		WatchInterval: config.WatchInterval,
		DebounceDelay: config.DebounceDelay,
		TargetWhitelist: config.Destinations,
	}
	
	if err := s.daemon.engine.StartAutoSync(engineConfig); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to start auto-sync: %v", err)
	}
	
	return &emptypb.Empty{}, nil
}

func (s *Service) StopAutoSync(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
	if err := s.daemon.engine.StopAutoSync(); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to stop auto-sync: %v", err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Service) GetAutoSyncStatus(ctx context.Context, req *emptypb.Empty) (*pb.AutoSyncStatus, error) {
	autoSyncStatus, err := s.daemon.engine.GetAutoSyncStatus()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get auto-sync status: %v", err)
	}
	
	return autoSyncStatusFromEngine(autoSyncStatus), nil
}

// Daemon lifecycle

func (s *Service) GetStatus(ctx context.Context, req *emptypb.Empty) (*pb.DaemonStatus, error) {
	uptime := time.Since(s.daemon.startTime)
	
	// Get auto-sync status from engine
	autoSyncRunning := false
	if autoSyncStatus, err := s.daemon.engine.GetAutoSyncStatus(); err == nil && autoSyncStatus != nil {
		autoSyncRunning = autoSyncStatus.Running
	}
	
	// Get version info
	versionInfo := GetVersionInfo()
	versionString := versionInfo.Version
	if versionInfo.GitCommit != "unknown" {
		versionString = fmt.Sprintf("%s (%s)", versionInfo.Version, versionInfo.GitCommit)
	}
	
	return &pb.DaemonStatus{
		Running:          true,
		Version:          versionString,
		StartTime:        timestamppb.New(s.daemon.startTime),
		UptimeSeconds:    int64(uptime.Seconds()),
		ActiveConnections: int32(atomic.LoadInt64(&s.daemon.connections)),
		AutoSyncRunning:  autoSyncRunning,
		LastError:        "",
	}, nil
}

func (s *Service) Shutdown(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
	s.daemon.logger.Info("shutdown requested via gRPC")
	
	// Trigger graceful shutdown
	go func() {
		time.Sleep(100 * time.Millisecond) // Give time for response
		s.daemon.shutdown()
	}()
	
	return &emptypb.Empty{}, nil
}

// Config management

func (s *Service) GetConfig(ctx context.Context, req *emptypb.Empty) (*pb.Config, error) {
	config, err := s.daemon.engine.GetConfig()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get config: %v", err)
	}
	if config == nil {
		return nil, status.Error(codes.NotFound, "no configuration loaded")
	}
	return engineConfigToProto(config), nil
}

func (s *Service) SetConfig(ctx context.Context, req *pb.Config) (*emptypb.Empty, error) {
	config := protoToEngineConfig(req)
	s.daemon.engine.SetConfig(config)
	return &emptypb.Empty{}, nil
}

func (s *Service) LoadConfig(ctx context.Context, req *pb.LoadConfigRequest) (*emptypb.Empty, error) {
	if err := s.daemon.engine.LoadConfig(req.Path); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to load config: %v", err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Service) SaveConfig(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
	if err := s.daemon.engine.SaveConfig(); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to save config: %v", err)
	}
	return &emptypb.Empty{}, nil
}

// Helper to format addresses
func formatAddresses(listener string, port int) []string {
	var addrs []string
	if listener != "" {
		addrs = append(addrs, fmt.Sprintf("unix://%s", listener))
	}
	if port > 0 {
		addrs = append(addrs, fmt.Sprintf("tcp://localhost:%d", port))
	}
	return addrs
}