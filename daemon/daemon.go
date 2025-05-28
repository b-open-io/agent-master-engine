package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	engine "github.com/b-open-io/agent-master-engine"
	pb "github.com/b-open-io/agent-master-engine/daemon/proto"
	"github.com/coreos/go-systemd/v22/daemon"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

// Version variables are defined in version.go

// Daemon represents the agent-master daemon
type Daemon struct {
	config    Config
	engine    engine.Engine
	server    *grpc.Server
	listener  net.Listener
	logger    *slog.Logger
	lockFile  *LockFile
	
	// State
	startTime    time.Time
	connections  int64
	lastActivity time.Time
	mu           sync.RWMutex
	
	// Shutdown
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// New creates a new daemon instance
func New(config Config) (*Daemon, error) {
	// Set defaults
	config.SetDefaults()
	
	// Create logger
	logger, err := createLogger(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}
	
	// Create engine
	eng, err := engine.NewEngine(
		engine.WithFileStorage(config.StoragePath),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create engine: %w", err)
	}
	
	// Load existing config
	configPath := filepath.Join(config.StoragePath, "config.json")
	if err := eng.LoadConfig(configPath); err != nil && !os.IsNotExist(err) {
		logger.Error("Failed to load config", "error", err)
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	return &Daemon{
		config:       config,
		engine:       eng,
		logger:       logger,
		startTime:    time.Now(),
		lastActivity: time.Now(),
		ctx:          ctx,
		cancel:       cancel,
	}, nil
}

// Run starts the daemon and blocks until shutdown
func (d *Daemon) Run(ctx context.Context) error {
	// Merge contexts
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	
	// Acquire lock
	d.lockFile = NewLockFile(d.getLockPath())
	if err := d.lockFile.TryAcquire(); err != nil {
		return fmt.Errorf("daemon already running: %w", err)
	}
	defer d.lockFile.Release()
	
	// Create listener
	listener, err := d.createListener()
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}
	d.listener = listener
	defer listener.Close()
	
	// Create gRPC server
	d.server = grpc.NewServer(
		grpc.UnaryInterceptor(d.unaryInterceptor),
		grpc.StreamInterceptor(d.streamInterceptor),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    30 * time.Second,
			Timeout: 10 * time.Second,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	
	// Register service
	service := NewService(d)
	pb.RegisterAgentMasterDaemonServer(d.server, service)
	
	// Enable reflection for debugging
	reflection.Register(d.server)
	
	// Start background tasks
	d.wg.Add(2)
	go d.idleMonitor()
	go d.autoSyncMonitor()
	
	// Systemd notification
	if d.config.EnableSystemd {
		daemon.SdNotify(false, daemon.SdNotifyReady)
		d.wg.Add(1)
		go d.systemdWatchdog()
	}
	
	// Log startup
	versionInfo := GetVersionInfo()
	d.logger.Info("Daemon started",
		"version", versionInfo.Version,
		"commit", versionInfo.GitCommit,
		"build_date", versionInfo.BuildDate,
		"storage", d.config.StoragePath,
		"address", d.getListenAddress(),
	)
	
	// Serve
	errCh := make(chan error, 1)
	go func() {
		errCh <- d.server.Serve(listener)
	}()
	
	// Wait for shutdown or error
	select {
	case err := <-errCh:
		if err != grpc.ErrServerStopped {
			return err
		}
	case <-ctx.Done():
		d.logger.Info("Shutdown requested")
	}
	
	// Graceful shutdown
	d.shutdown()
	
	return nil
}

// shutdown performs graceful shutdown
func (d *Daemon) shutdown() {
	// Notify systemd
	if d.config.EnableSystemd {
		daemon.SdNotify(false, daemon.SdNotifyStopping)
	}
	
	// Stop accepting new connections
	if d.server != nil {
		d.server.GracefulStop()
	}
	
	// Cancel context
	d.cancel()
	
	// Wait for goroutines
	done := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		d.logger.Info("Graceful shutdown complete")
	case <-time.After(10 * time.Second):
		d.logger.Warn("Shutdown timeout, forcing")
		if d.server != nil {
			d.server.Stop()
		}
	}
	
	// Stop auto-sync if running
	if status, _ := d.engine.GetAutoSyncStatus(); status != nil && status.Running {
		d.logger.Info("Stopping auto-sync")
		d.engine.StopAutoSync()
	}
}

// createListener creates the network listener
func (d *Daemon) createListener() (net.Listener, error) {
	if d.config.Port > 0 {
		// TCP listener
		addr := fmt.Sprintf(":%d", d.config.Port)
		return net.Listen("tcp", addr)
	}
	
	// Unix socket
	// Remove old socket if exists
	os.Remove(d.config.SocketPath)
	
	listener, err := net.Listen("unix", d.config.SocketPath)
	if err != nil {
		return nil, err
	}
	
	// Set permissions
	if err := os.Chmod(d.config.SocketPath, 0600); err != nil {
		listener.Close()
		return nil, err
	}
	
	return listener, nil
}

// unaryInterceptor logs calls and updates activity
func (d *Daemon) unaryInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	start := time.Now()
	
	// Update activity
	d.updateActivity()
	
	// Track connection
	atomic.AddInt64(&d.connections, 1)
	defer atomic.AddInt64(&d.connections, -1)
	
	// Call handler
	resp, err := handler(ctx, req)
	
	// Log call
	d.logger.Debug("RPC call",
		"method", info.FullMethod,
		"duration", time.Since(start),
		"error", err,
	)
	
	return resp, err
}

// streamInterceptor handles streaming calls
func (d *Daemon) streamInterceptor(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	start := time.Now()
	
	// Update activity
	d.updateActivity()
	
	// Track connection
	atomic.AddInt64(&d.connections, 1)
	defer atomic.AddInt64(&d.connections, -1)
	
	// Call handler
	err := handler(srv, ss)
	
	// Log call
	d.logger.Debug("Stream RPC",
		"method", info.FullMethod,
		"duration", time.Since(start),
		"error", err,
	)
	
	return err
}

// updateActivity updates the last activity timestamp
func (d *Daemon) updateActivity() {
	d.mu.Lock()
	d.lastActivity = time.Now()
	d.mu.Unlock()
}

// idleMonitor shuts down after idle timeout
func (d *Daemon) idleMonitor() {
	defer d.wg.Done()
	
	if d.config.IdleTimeout <= 0 {
		<-d.ctx.Done()
		return
	}
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			d.mu.RLock()
			idle := time.Since(d.lastActivity)
			connections := atomic.LoadInt64(&d.connections)
			d.mu.RUnlock()
			
			// Don't timeout if clients connected or auto-sync running
			if connections > 0 {
				continue
			}
			
			if status, _ := d.engine.GetAutoSyncStatus(); status != nil && status.Running {
				continue
			}
			
			if idle > d.config.IdleTimeout {
				d.logger.Info("Idle timeout reached", "idle", idle)
				d.cancel()
				return
			}
			
		case <-d.ctx.Done():
			return
		}
	}
}

// autoSyncMonitor starts auto-sync if configured
func (d *Daemon) autoSyncMonitor() {
	defer d.wg.Done()
	
	// Check if auto-sync should start
	status, err := d.engine.GetAutoSyncStatus()
	if err != nil {
		d.logger.Error("Failed to get auto-sync status", "error", err)
		return
	}
	
	if status.Enabled && !status.Running {
		d.logger.Info("Starting auto-sync")
		
		config := engine.AutoSyncConfig{
			Enabled:       status.Enabled,
			WatchInterval: status.WatchInterval,
			DebounceDelay: 500 * time.Millisecond,
		}
		
		if err := d.engine.StartAutoSync(config); err != nil {
			d.logger.Error("Failed to start auto-sync", "error", err)
		}
	}
	
	// Wait for shutdown
	<-d.ctx.Done()
}

// systemdWatchdog sends watchdog notifications
func (d *Daemon) systemdWatchdog() {
	defer d.wg.Done()
	
	interval, err := daemon.SdWatchdogEnabled(false)
	if err != nil || interval == 0 {
		return
	}
	
	d.logger.Debug("Systemd watchdog enabled", "interval", interval)
	
	ticker := time.NewTicker(interval / 2)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			daemon.SdNotify(false, daemon.SdNotifyWatchdog)
		case <-d.ctx.Done():
			return
		}
	}
}

// Helper methods

func (d *Daemon) getLockPath() string {
	return filepath.Join(os.TempDir(), "agent-master-daemon.lock")
}

func (d *Daemon) getListenAddress() string {
	if d.config.Port > 0 {
		return fmt.Sprintf("tcp://:%d", d.config.Port)
	}
	return fmt.Sprintf("unix://%s", d.config.SocketPath)
}

// createLogger creates the daemon logger
func createLogger(config Config) (*slog.Logger, error) {
	var level slog.Level
	switch config.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	
	opts := &slog.HandlerOptions{
		Level: level,
	}
	
	var handler slog.Handler
	if config.LogFile != "" {
		file, err := os.OpenFile(config.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, err
		}
		handler = slog.NewJSONHandler(file, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}
	
	return slog.New(handler), nil
}