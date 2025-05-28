package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/b-open-io/agent-master-engine/daemon"
)

var (
	// Version info (set by ldflags)
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

func main() {
	var (
		configPath    = flag.String("config", "", "path to daemon config file")
		storagePath   = flag.String("storage", "", "path to storage directory (default: ~/.agent-master)")
		logLevel      = flag.String("log-level", "info", "log level (debug, info, warn, error)")
		logFile       = flag.String("log-file", "", "log file path (default: stdout)")
		socketPath    = flag.String("socket", "", "unix socket path (default: /tmp/agent-master-daemon.sock)")
		port          = flag.Int("port", 0, "TCP port to listen on (overrides socket)")
		idleTimeout   = flag.Duration("idle-timeout", 0, "idle timeout (0 = disabled)")
		version       = flag.Bool("version", false, "show version information")
		systemd       = flag.Bool("systemd", false, "enable systemd integration")
	)

	flag.Parse()

	if *version {
		fmt.Printf("agent-master-daemon %s (%s) built %s\n", Version, GitCommit, BuildDate)
		os.Exit(0)
	}

	// Set up context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("Received signal: %v", sig)
		cancel()
	}()

	// Configure daemon
	config := daemon.Config{
		StoragePath:   *storagePath,
		SocketPath:    *socketPath,
		Port:          *port,
		LogLevel:      *logLevel,
		LogFile:       *logFile,
		IdleTimeout:   *idleTimeout,
		EnableSystemd: *systemd,
	}

	// Load config file if provided
	if *configPath != "" {
		if err := config.LoadFromFile(*configPath); err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
	}

	// Set version info in daemon package
	daemon.Version = Version
	daemon.GitCommit = GitCommit
	daemon.BuildDate = BuildDate

	// Create and start daemon
	d, err := daemon.New(config)
	if err != nil {
		log.Fatalf("Failed to create daemon: %v", err)
	}

	log.Printf("Starting agent-master-daemon %s", Version)
	if err := d.Run(ctx); err != nil {
		log.Fatalf("Daemon failed: %v", err)
	}

	log.Println("Daemon stopped")
}