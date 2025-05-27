package client

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	pb "github.com/b-open-io/agent-master-engine/daemon/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// ConnectionType represents the type of connection (TCP or Unix socket)
type ConnectionType int

const (
	ConnectionTCP ConnectionType = iota
	ConnectionUnix
)

// ClientOptions contains configuration options for the client
type ClientOptions struct {
	// Connection settings
	Type    ConnectionType
	Address string

	// Retry settings
	MaxRetries        int
	RetryDelay        time.Duration
	RetryBackoffMultiplier float64

	// Keepalive settings
	KeepaliveTime    time.Duration
	KeepaliveTimeout time.Duration

	// Request timeout
	RequestTimeout time.Duration
}

// DefaultOptions returns sensible default options
func DefaultOptions() *ClientOptions {
	return &ClientOptions{
		Type:                   ConnectionTCP,
		Address:                "localhost:50051",
		MaxRetries:             3,
		RetryDelay:             time.Second,
		RetryBackoffMultiplier: 2.0,
		KeepaliveTime:          30 * time.Second,
		KeepaliveTimeout:       10 * time.Second,
		RequestTimeout:         30 * time.Second,
	}
}

// Client wraps the gRPC connection and provides simplified methods
type Client struct {
	conn    *grpc.ClientConn
	client  pb.AgentMasterDaemonClient
	options *ClientOptions
	mu      sync.RWMutex
	closed  bool
}

// NewClient creates a new client with the given options
func NewClient(opts *ClientOptions) *Client {
	if opts == nil {
		opts = DefaultOptions()
	}
	return &Client{
		options: opts,
	}
}

// Connect establishes a connection to the daemon
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return fmt.Errorf("already connected")
	}

	// Build dial options
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:    c.options.KeepaliveTime,
			Timeout: c.options.KeepaliveTimeout,
		}),
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff: backoff.Config{
				BaseDelay:  c.options.RetryDelay,
				Multiplier: c.options.RetryBackoffMultiplier,
				MaxDelay:   30 * time.Second,
			},
		}),
	}

	// Set up the appropriate dialer based on connection type
	var target string
	switch c.options.Type {
	case ConnectionUnix:
		target = "unix://" + c.options.Address
		dialOpts = append(dialOpts, grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			addr = strings.TrimPrefix(addr, "unix://")
			return net.Dial("unix", addr)
		}))
	case ConnectionTCP:
		target = c.options.Address
	default:
		return fmt.Errorf("unknown connection type: %v", c.options.Type)
	}

	// Establish connection
	conn, err := grpc.DialContext(ctx, target, dialOpts...)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	c.conn = conn
	c.client = pb.NewAgentMasterDaemonClient(conn)
	c.closed = false

	return nil
}

// Close closes the connection
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil
	}

	c.closed = true
	err := c.conn.Close()
	c.conn = nil
	c.client = nil
	return err
}

// IsConnected returns true if the client is connected
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn != nil && !c.closed
}

// withRetry executes a function with retry logic
func (c *Client) withRetry(ctx context.Context, fn func(context.Context) error) error {
	delay := c.options.RetryDelay
	for i := 0; i <= c.options.MaxRetries; i++ {
		err := fn(ctx)
		if err == nil {
			return nil
		}

		// Check if error is retryable
		if !isRetryable(err) {
			return err
		}

		// Don't retry if we're on the last attempt
		if i == c.options.MaxRetries {
			return err
		}

		// Wait before retrying
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			delay = time.Duration(float64(delay) * c.options.RetryBackoffMultiplier)
		}
	}
	return fmt.Errorf("max retries exceeded")
}

// isRetryable determines if an error should trigger a retry
func isRetryable(err error) bool {
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	
	switch st.Code() {
	case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted:
		return true
	default:
		return false
	}
}

// ensureConnected checks if connected and returns appropriate error
func (c *Client) ensureConnected() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}
	if c.closed {
		return fmt.Errorf("connection closed")
	}
	return nil
}

// Server Management Methods

// AddServer adds a new server configuration
func (c *Client) AddServer(ctx context.Context, name string, config *pb.ServerConfig) (*pb.ServerInfo, error) {
	if err := c.ensureConnected(); err != nil {
		return nil, err
	}

	var resp *pb.ServerResponse
	err := c.withRetry(ctx, func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, c.options.RequestTimeout)
		defer cancel()
		
		var err error
		resp, err = c.client.AddServer(ctx, &pb.AddServerRequest{
			Name:   name,
			Config: config,
		})
		return err
	})
	
	if err != nil {
		return nil, err
	}
	return resp.Server, nil
}

// UpdateServer updates an existing server configuration
func (c *Client) UpdateServer(ctx context.Context, name string, config *pb.ServerConfig) (*pb.ServerInfo, error) {
	if err := c.ensureConnected(); err != nil {
		return nil, err
	}

	var resp *pb.ServerResponse
	err := c.withRetry(ctx, func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, c.options.RequestTimeout)
		defer cancel()
		
		var err error
		resp, err = c.client.UpdateServer(ctx, &pb.UpdateServerRequest{
			Name:   name,
			Config: config,
		})
		return err
	})
	
	if err != nil {
		return nil, err
	}
	return resp.Server, nil
}

// RemoveServer removes a server configuration
func (c *Client) RemoveServer(ctx context.Context, name string) error {
	if err := c.ensureConnected(); err != nil {
		return err
	}

	return c.withRetry(ctx, func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, c.options.RequestTimeout)
		defer cancel()
		
		_, err := c.client.RemoveServer(ctx, &pb.RemoveServerRequest{
			Name: name,
		})
		return err
	})
}

// GetServer retrieves a server configuration
func (c *Client) GetServer(ctx context.Context, name string) (*pb.ServerInfo, error) {
	if err := c.ensureConnected(); err != nil {
		return nil, err
	}

	var resp *pb.ServerResponse
	err := c.withRetry(ctx, func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, c.options.RequestTimeout)
		defer cancel()
		
		var err error
		resp, err = c.client.GetServer(ctx, &pb.GetServerRequest{
			Name: name,
		})
		return err
	})
	
	if err != nil {
		return nil, err
	}
	return resp.Server, nil
}

// ListServers lists all server configurations
func (c *Client) ListServers(ctx context.Context, filter *pb.ServerFilter) ([]*pb.ServerInfo, error) {
	if err := c.ensureConnected(); err != nil {
		return nil, err
	}

	var resp *pb.ListServersResponse
	err := c.withRetry(ctx, func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, c.options.RequestTimeout)
		defer cancel()
		
		var err error
		resp, err = c.client.ListServers(ctx, &pb.ListServersRequest{
			Filter: filter,
		})
		return err
	})
	
	if err != nil {
		return nil, err
	}
	return resp.Servers, nil
}

// EnableServer enables a server
func (c *Client) EnableServer(ctx context.Context, name string) (*pb.ServerInfo, error) {
	if err := c.ensureConnected(); err != nil {
		return nil, err
	}

	var resp *pb.ServerResponse
	err := c.withRetry(ctx, func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, c.options.RequestTimeout)
		defer cancel()
		
		var err error
		resp, err = c.client.EnableServer(ctx, &pb.EnableServerRequest{
			Name: name,
		})
		return err
	})
	
	if err != nil {
		return nil, err
	}
	return resp.Server, nil
}

// DisableServer disables a server
func (c *Client) DisableServer(ctx context.Context, name string) (*pb.ServerInfo, error) {
	if err := c.ensureConnected(); err != nil {
		return nil, err
	}

	var resp *pb.ServerResponse
	err := c.withRetry(ctx, func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, c.options.RequestTimeout)
		defer cancel()
		
		var err error
		resp, err = c.client.DisableServer(ctx, &pb.DisableServerRequest{
			Name: name,
		})
		return err
	})
	
	if err != nil {
		return nil, err
	}
	return resp.Server, nil
}

// Destination Management Methods

// RegisterDestination registers a new destination
func (c *Client) RegisterDestination(ctx context.Context, name string, destType pb.DestinationType, path string, options map[string]string) error {
	if err := c.ensureConnected(); err != nil {
		return err
	}

	return c.withRetry(ctx, func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, c.options.RequestTimeout)
		defer cancel()
		
		_, err := c.client.RegisterDestination(ctx, &pb.RegisterDestinationRequest{
			Name:    name,
			Type:    destType,
			Path:    path,
			Options: options,
		})
		return err
	})
}

// RemoveDestination removes a destination
func (c *Client) RemoveDestination(ctx context.Context, name string) error {
	if err := c.ensureConnected(); err != nil {
		return err
	}

	return c.withRetry(ctx, func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, c.options.RequestTimeout)
		defer cancel()
		
		_, err := c.client.RemoveDestination(ctx, &pb.RemoveDestinationRequest{
			Name: name,
		})
		return err
	})
}

// ListDestinations lists all available destinations
func (c *Client) ListDestinations(ctx context.Context) (map[string]*pb.DestinationInfo, error) {
	if err := c.ensureConnected(); err != nil {
		return nil, err
	}

	var resp *pb.ListDestinationsResponse
	err := c.withRetry(ctx, func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, c.options.RequestTimeout)
		defer cancel()
		
		var err error
		resp, err = c.client.ListDestinations(ctx, &emptypb.Empty{})
		return err
	})
	
	if err != nil {
		return nil, err
	}
	return resp.Destinations, nil
}

// Sync Operations

// SyncTo syncs configuration to a specific destination
func (c *Client) SyncTo(ctx context.Context, destination string, options *pb.SyncOptions) (*pb.SyncResult, error) {
	if err := c.ensureConnected(); err != nil {
		return nil, err
	}

	var resp *pb.SyncResult
	err := c.withRetry(ctx, func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, c.options.RequestTimeout)
		defer cancel()
		
		var err error
		resp, err = c.client.SyncTo(ctx, &pb.SyncToRequest{
			Destination: destination,
			Options:     options,
		})
		return err
	})
	
	return resp, err
}

// SyncToMultiple syncs configuration to multiple destinations
func (c *Client) SyncToMultiple(ctx context.Context, destinations []string, options *pb.SyncOptions) (*pb.MultiSyncResult, error) {
	if err := c.ensureConnected(); err != nil {
		return nil, err
	}

	var resp *pb.MultiSyncResult
	err := c.withRetry(ctx, func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, c.options.RequestTimeout)
		defer cancel()
		
		var err error
		resp, err = c.client.SyncToMultiple(ctx, &pb.SyncToMultipleRequest{
			Destinations: destinations,
			Options:      options,
		})
		return err
	})
	
	return resp, err
}

// PreviewSync previews what would be synced to a destination
func (c *Client) PreviewSync(ctx context.Context, destination string) (*pb.SyncPreview, error) {
	if err := c.ensureConnected(); err != nil {
		return nil, err
	}

	var resp *pb.SyncPreview
	err := c.withRetry(ctx, func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, c.options.RequestTimeout)
		defer cancel()
		
		var err error
		resp, err = c.client.PreviewSync(ctx, &pb.PreviewSyncRequest{
			Destination: destination,
		})
		return err
	})
	
	return resp, err
}

// Auto-sync Management

// StartAutoSync starts the auto-sync feature
func (c *Client) StartAutoSync(ctx context.Context, config *pb.AutoSyncConfig) error {
	if err := c.ensureConnected(); err != nil {
		return err
	}

	return c.withRetry(ctx, func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, c.options.RequestTimeout)
		defer cancel()
		
		_, err := c.client.StartAutoSync(ctx, config)
		return err
	})
}

// StopAutoSync stops the auto-sync feature
func (c *Client) StopAutoSync(ctx context.Context) error {
	if err := c.ensureConnected(); err != nil {
		return err
	}

	return c.withRetry(ctx, func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, c.options.RequestTimeout)
		defer cancel()
		
		_, err := c.client.StopAutoSync(ctx, &emptypb.Empty{})
		return err
	})
}

// GetAutoSyncStatus gets the current auto-sync status
func (c *Client) GetAutoSyncStatus(ctx context.Context) (*pb.AutoSyncStatus, error) {
	if err := c.ensureConnected(); err != nil {
		return nil, err
	}

	var resp *pb.AutoSyncStatus
	err := c.withRetry(ctx, func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, c.options.RequestTimeout)
		defer cancel()
		
		var err error
		resp, err = c.client.GetAutoSyncStatus(ctx, &emptypb.Empty{})
		return err
	})
	
	return resp, err
}

// Configuration

// GetConfig retrieves the current configuration
func (c *Client) GetConfig(ctx context.Context) (*pb.Config, error) {
	if err := c.ensureConnected(); err != nil {
		return nil, err
	}

	var resp *pb.Config
	err := c.withRetry(ctx, func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, c.options.RequestTimeout)
		defer cancel()
		
		var err error
		resp, err = c.client.GetConfig(ctx, &emptypb.Empty{})
		return err
	})
	
	return resp, err
}

// SetConfig updates the configuration
func (c *Client) SetConfig(ctx context.Context, config *pb.Config) error {
	if err := c.ensureConnected(); err != nil {
		return err
	}

	return c.withRetry(ctx, func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, c.options.RequestTimeout)
		defer cancel()
		
		_, err := c.client.SetConfig(ctx, config)
		return err
	})
}

// LoadConfig loads configuration from a file
func (c *Client) LoadConfig(ctx context.Context, path string) error {
	if err := c.ensureConnected(); err != nil {
		return err
	}

	return c.withRetry(ctx, func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, c.options.RequestTimeout)
		defer cancel()
		
		_, err := c.client.LoadConfig(ctx, &pb.LoadConfigRequest{
			Path: path,
		})
		return err
	})
}

// SaveConfig saves the current configuration
func (c *Client) SaveConfig(ctx context.Context) error {
	if err := c.ensureConnected(); err != nil {
		return err
	}

	return c.withRetry(ctx, func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, c.options.RequestTimeout)
		defer cancel()
		
		_, err := c.client.SaveConfig(ctx, &emptypb.Empty{})
		return err
	})
}

// Daemon Lifecycle

// GetStatus retrieves the daemon status
func (c *Client) GetStatus(ctx context.Context) (*pb.DaemonStatus, error) {
	if err := c.ensureConnected(); err != nil {
		return nil, err
	}

	var resp *pb.DaemonStatus
	err := c.withRetry(ctx, func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, c.options.RequestTimeout)
		defer cancel()
		
		var err error
		resp, err = c.client.GetStatus(ctx, &emptypb.Empty{})
		return err
	})
	
	return resp, err
}

// Shutdown requests the daemon to shut down
func (c *Client) Shutdown(ctx context.Context) error {
	if err := c.ensureConnected(); err != nil {
		return err
	}

	// Don't retry shutdown
	ctx, cancel := context.WithTimeout(ctx, c.options.RequestTimeout)
	defer cancel()
	
	_, err := c.client.Shutdown(ctx, &emptypb.Empty{})
	return err
}

// Events

// EventHandler is a function that handles events
type EventHandler func(*pb.Event) error

// Subscribe subscribes to events from the daemon
func (c *Client) Subscribe(ctx context.Context, eventTypes []pb.EventType, handler EventHandler) error {
	if err := c.ensureConnected(); err != nil {
		return err
	}

	stream, err := c.client.Subscribe(ctx, &pb.SubscribeRequest{
		Types: eventTypes,
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	// Handle events in a goroutine
	go func() {
		for {
			event, err := stream.Recv()
			if err != nil {
				// Stream closed
				return
			}
			
			if handler != nil {
				if err := handler(event); err != nil {
					// Handler error, but continue processing
					continue
				}
			}
		}
	}()

	return nil
}