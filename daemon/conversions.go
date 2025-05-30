package daemon

import (
	"encoding/json"
	"time"

	engine "github.com/b-open-io/agent-master-engine"
	pb "github.com/b-open-io/agent-master-engine/daemon/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Convert engine types to proto types

func engineConfigToProto(config *engine.Config) *pb.Config {
	servers := make(map[string]*pb.ServerConfig)
	for name, server := range config.Servers {
		servers[name] = &pb.ServerConfig{
			Transport: server.Transport,
			Command:   server.Command,
			Args:      server.Args,
			Url:       server.URL,
			Env:       server.Env,
			Enabled:   server.Internal.Enabled,
		}
	}
	
	return &pb.Config{
		Servers: servers,
	}
}

func protoToEngineConfig(config *pb.Config) *engine.Config {
	servers := make(map[string]engine.ServerWithMetadata)
	for name, server := range config.Servers {
		servers[name] = engine.ServerWithMetadata{
			ServerConfig: engine.ServerConfig{
				Transport: server.Transport,
				Command:   server.Command,
				Args:      server.Args,
				URL:       server.Url,
				Env:       server.Env,
			},
			Internal: engine.InternalMetadata{
				Enabled: server.Enabled,
			},
		}
	}
	
	return &engine.Config{
		Servers: servers,
	}
}

func serverConfigToProto(name string, s engine.ServerWithMetadata) *pb.ServerInfo {
	// Convert metadata interface{} map to string map
	metadata := make(map[string]string)
	for k, v := range s.Metadata {
		if str, ok := v.(string); ok {
			metadata[k] = str
		} else {
			// Convert non-string values to JSON
			if data, err := json.Marshal(v); err == nil {
				metadata[k] = string(data)
			}
		}
	}
	
	return &pb.ServerInfo{
		Name: name,
		Config: &pb.ServerConfig{
			Transport: s.Transport,
			Command:   s.Command,
			Args:      s.Args,
			Url:       s.URL,
			Env:       s.Env,
			Enabled:   s.Internal.Enabled,
			Metadata:  metadata,
		},
		CreatedAt: timestamppb.New(s.Internal.LastModified),
		UpdatedAt: timestamppb.New(s.Internal.LastModified),
		Source:    s.Internal.Source,
	}
}

func protoToServerConfig(name string, config *pb.ServerConfig) engine.ServerWithMetadata {
	// Convert string metadata back to interface{} map
	metadata := make(map[string]interface{})
	for k, v := range config.Metadata {
		metadata[k] = v
	}
	
	return engine.ServerWithMetadata{
		ServerConfig: engine.ServerConfig{
			Transport: config.Transport,
			Command:   config.Command,
			Args:      config.Args,
			URL:       config.Url,
			Env:       config.Env,
			Metadata:  metadata,
		},
		Internal: engine.InternalMetadata{
			Enabled: config.Enabled,
		},
	}
}

func syncResultToProto(r *engine.SyncResult) *pb.SyncResult {
	if r == nil {
		return &pb.SyncResult{
			Success:   false,
			Message:   "No result returned from sync operation",
			Timestamp: timestamppb.New(time.Now()),
		}
	}
	
	result := &pb.SyncResult{
		Success:       r.Success,
		Message:       formatSyncMessage(r),
		ServersSynced: int32(r.ServersAdded + r.ServersUpdated),
		Timestamp:     timestamppb.New(time.Now()),
	}
	
	// Convert errors
	for _, e := range r.Errors {
		result.Errors = append(result.Errors, e.Error)
	}
	
	return result
}

func formatSyncMessage(r *engine.SyncResult) string {
	if r.Success {
		return "Sync completed successfully"
	}
	return "Sync failed with errors"
}

func autoSyncConfigToEngine(config *pb.AutoSyncConfig) engine.AutoSyncSettings {
	return engine.AutoSyncSettings{
		Enabled:       config.Enabled,
		WatchInterval: time.Duration(config.WatchIntervalMs) * time.Millisecond,
		DebounceDelay: time.Duration(config.DebounceDelayMs) * time.Millisecond,
		Destinations:  config.TargetWhitelist,
	}
}

func autoSyncStatusFromEngine(status *engine.AutoSyncStatus) *pb.AutoSyncStatus {
	result := &pb.AutoSyncStatus{
		Enabled:         status.Enabled,
		Running:         status.Running,
		LastSync:        timestamppb.New(status.LastSync),
		WatchIntervalMs: status.WatchInterval.Milliseconds(),
	}
	
	return result
}

// Helper to convert server info from engine format
func serverInfoToProto(info engine.ServerInfo) *pb.ServerInfo {
	return &pb.ServerInfo{
		Name: info.Name,
		Config: &pb.ServerConfig{
			Enabled: info.Enabled,
		},
	}
}

// Helper used by service layer
func serverToProto(name string, server engine.ServerWithMetadata) *pb.ServerInfo {
	return serverConfigToProto(name, server)
}

// Convert multi-sync results
func multiSyncResultToProto(mr *engine.MultiSyncResult) *pb.SyncResult {
	result := &pb.SyncResult{
		Success: allSyncSuccessful(mr),
		Message: formatMultiSyncMessage(mr),
		Timestamp: timestamppb.New(time.Now()),
	}
	
	// Aggregate errors from all destinations
	for _, dr := range mr.Results {
		for _, err := range dr.Errors {
			result.Errors = append(result.Errors, err.Error)
		}
	}
	
	return result
}

func allSyncSuccessful(mr *engine.MultiSyncResult) bool {
	for _, result := range mr.Results {
		if !result.Success {
			return false
		}
	}
	return true
}

func formatMultiSyncMessage(mr *engine.MultiSyncResult) string {
	if allSyncSuccessful(mr) {
		return "All syncs completed successfully"
	}
	return "Some syncs failed"
}

// Project conversion functions
func engineProjectInfoToPB(project *engine.ProjectInfo) *pb.ProjectInfo {
	return &pb.ProjectInfo{
		Name: project.Name,
		Path: project.Path,
		Type: "",  // ProjectInfo doesn't have Type field in engine
		Config: &pb.ProjectConfig{
			Name:     project.Name,
			Type:     "",
			Metadata: make(map[string]string),
			Servers:  []*pb.ServerConfig{}, // Will be populated from project.Servers if needed
		},
		DetectedAt: timestamppb.Now(), // Use current time as ProjectInfo doesn't have DetectedAt
	}
}

func engineProjectConfigToPBInfo(project *engine.ProjectConfig) *pb.ProjectInfo {
	return &pb.ProjectInfo{
		Name: project.Name,
		Path: project.Path,
		Type: "",  // ProjectConfig doesn't have Type field in engine
		Config: engineProjectConfigToPB(project),
		DetectedAt: timestamppb.Now(), // Use current time
	}
}

func pbToEngineProjectConfig(config *pb.ProjectConfig) *engine.ProjectConfig {
	servers := make(map[string]engine.ServerWithMetadata)
	for _, srv := range config.Servers {
		// Convert map[string]string to map[string]interface{}
		metadata := make(map[string]interface{})
		for k, v := range srv.Metadata {
			metadata[k] = v
		}
		
		servers[srv.Command] = engine.ServerWithMetadata{
			ServerConfig: engine.ServerConfig{
				Transport: srv.Transport,
				Command:   srv.Command,
				Args:      srv.Args,
				URL:       srv.Url,
				Env:       srv.Env,
				Metadata:  metadata,
			},
			Internal: engine.InternalMetadata{
				Enabled: srv.Enabled,
			},
		}
	}
	
	// Convert string metadata to interface{} metadata
	metadata := make(map[string]interface{})
	for k, v := range config.Metadata {
		metadata[k] = v
	}
	
	return &engine.ProjectConfig{
		Name:     config.Name,
		Path:     "", // Will be set by caller
		Servers:  servers,
		Metadata: metadata,
	}
}

func engineProjectConfigToPB(config *engine.ProjectConfig) *pb.ProjectConfig {
	servers := make([]*pb.ServerConfig, 0, len(config.Servers))
	for _, srv := range config.Servers {
		// Convert map[string]interface{} to map[string]string
		metadata := make(map[string]string)
		for k, v := range srv.Metadata {
			if str, ok := v.(string); ok {
				metadata[k] = str
			} else {
				// Convert non-string values to JSON
				if jsonBytes, err := json.Marshal(v); err == nil {
					metadata[k] = string(jsonBytes)
				}
			}
		}
		
		servers = append(servers, &pb.ServerConfig{
			Transport: srv.Transport,
			Command:   srv.Command,
			Args:      srv.Args,
			Url:       srv.URL,
			Env:       srv.Env,
			Enabled:   srv.Internal.Enabled,
			Metadata:  metadata,
		})
	}
	
	// Convert interface{} metadata to string metadata
	metadata := make(map[string]string)
	for k, v := range config.Metadata {
		if str, ok := v.(string); ok {
			metadata[k] = str
		} else {
			// Convert non-string values to JSON
			if jsonBytes, err := json.Marshal(v); err == nil {
				metadata[k] = string(jsonBytes)
			}
		}
	}
	
	return &pb.ProjectConfig{
		Name:     config.Name,
		Type:     "", // ProjectConfig doesn't have Type field in engine
		Metadata: metadata,
		Servers:  servers,
	}
}

// Backup conversion functions
func engineBackupInfoToPB(backup *engine.BackupInfo) *pb.BackupInfo {
	return &pb.BackupInfo{
		Id:          backup.ID,
		Description: backup.Description,
		Path:        backup.Path,
		CreatedAt:   timestamppb.New(backup.Timestamp),
		SizeBytes:   backup.Size,
	}
}