package daemon

import (
	"context"

	engine "github.com/b-open-io/agent-master-engine"
	pb "github.com/b-open-io/agent-master-engine/daemon/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Enable/Disable server methods (using proper engine methods)
func (s *Service) EnableServer(ctx context.Context, req *pb.EnableServerRequest) (*pb.ServerResponse, error) {
	// Enable the server using the proper engine method
	if err := s.daemon.engine.EnableServer(req.Name); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to enable server: %v", err)
	}
	
	// Get updated server to return
	server, err := s.daemon.engine.GetServer(req.Name)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "server not found after enable: %v", err)
	}
	
	// Return updated server
	return &pb.ServerResponse{
		Server: serverToProto(req.Name, *server),
	}, nil
}

func (s *Service) DisableServer(ctx context.Context, req *pb.DisableServerRequest) (*pb.ServerResponse, error) {
	// Disable the server using the proper engine method
	if err := s.daemon.engine.DisableServer(req.Name); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to disable server: %v", err)
	}
	
	// Get updated server to return
	server, err := s.daemon.engine.GetServer(req.Name)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "server not found after disable: %v", err)
	}
	
	// Return updated server
	return &pb.ServerResponse{
		Server: serverToProto(req.Name, *server),
	}, nil
}

// Fixed ListServers to match actual field names
func (s *Service) ListServersCorrected(ctx context.Context, req *pb.ListServersRequest) (*pb.ListServersResponse, error) {
	// Convert proto filter to engine filter
	filter := engine.ServerFilter{}
	if req.Filter != nil {
		if req.Filter.EnabledOnly {
			enabled := true
			filter.Enabled = &enabled
		}
		filter.Transport = req.Filter.Transport
		filter.Source = req.Filter.Source
		// Note: name_pattern is not supported by engine filter
	}
	
	servers, err := s.daemon.engine.ListServers(filter)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list servers: %v", err)
	}
	
	var protoServers []*pb.ServerInfo
	for _, srv := range servers {
		protoServers = append(protoServers, &pb.ServerInfo{
			Name: srv.Name,
			Config: &pb.ServerConfig{
				Enabled: srv.Enabled,
			},
		})
	}
	
	return &pb.ListServersResponse{
		Servers: protoServers,
	}, nil
}