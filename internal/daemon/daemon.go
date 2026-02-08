// Package daemon provides the background daemon service for Ugudu
package daemon

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/arcslash/ugudu/internal/api"
	"github.com/arcslash/ugudu/internal/config"
	"github.com/arcslash/ugudu/internal/logger"
	"github.com/arcslash/ugudu/internal/manager"
)

const (
	// DefaultSocketPath is the default Unix socket path
	DefaultSocketPath = "/var/run/ugudu/ugudu.sock"
	// UserSocketPath is used when running as non-root
	UserSocketPath = ".ugudu/ugudu.sock"
	// PidFileName is the name of the PID file
	PidFileName = "ugudu.pid"
)

// Daemon is the background service that manages all Ugudu operations
type Daemon struct {
	manager   *manager.Manager
	logger    *logger.Logger
	apiServer *api.Server

	socketServer *http.Server
	tcpServer    *http.Server

	socketPath  string
	pidPath     string
	tcpAddr     string // Optional TCP address for remote access
	tcpListener net.Listener

	listener net.Listener
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

// Config holds daemon configuration
type Config struct {
	DataDir    string
	SocketPath string
	TCPAddr    string // Optional: "host:port" for HTTP access
	LogLevel   string
}

// New creates a new daemon instance
func New(cfg Config) (*Daemon, error) {
	// Load Ugudu config and apply to environment for provider auto-discovery
	if uguduCfg, err := config.Load(); err == nil {
		uguduCfg.ApplyToEnvironment()
	}

	// Determine socket path
	socketPath := cfg.SocketPath
	if socketPath == "" {
		// Try system path first, fall back to user home
		if os.Geteuid() == 0 {
			socketPath = DefaultSocketPath
		} else {
			home, _ := os.UserHomeDir()
			socketPath = filepath.Join(home, UserSocketPath)
		}
	}

	// Ensure socket directory exists
	socketDir := filepath.Dir(socketPath)
	if err := os.MkdirAll(socketDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create socket directory: %w", err)
	}

	// Setup data directory
	dataDir := cfg.DataDir
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".ugudu", "data")
	}

	// Create logger
	log := logger.New(cfg.LogLevel, os.Stdout)

	// Create manager
	mgrCfg := manager.Config{
		DataDir:   dataDir,
		LogLevel:  cfg.LogLevel,
		LogFormat: "text",
	}
	mgr, err := manager.New(mgrCfg, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create manager: %w", err)
	}

	// Create API server
	apiServer := api.NewServer(mgr, log)

	ctx, cancel := context.WithCancel(context.Background())

	return &Daemon{
		manager:    mgr,
		logger:     log,
		apiServer:  apiServer,
		socketPath: socketPath,
		pidPath:    filepath.Join(socketDir, PidFileName),
		tcpAddr:    cfg.TCPAddr,
		ctx:        ctx,
		cancel:     cancel,
	}, nil
}

// Start begins the daemon
func (d *Daemon) Start() error {
	// Check if already running
	if d.isRunning() {
		return fmt.Errorf("daemon already running (pid file: %s)", d.pidPath)
	}

	// Remove stale socket
	os.Remove(d.socketPath)

	// Create Unix socket listener
	listener, err := net.Listen("unix", d.socketPath)
	if err != nil {
		return fmt.Errorf("failed to create socket: %w", err)
	}
	d.listener = listener

	// Set socket permissions
	if err := os.Chmod(d.socketPath, 0660); err != nil {
		d.logger.Warn("failed to set socket permissions", "error", err)
	}

	// Write PID file
	if err := d.writePID(); err != nil {
		d.logger.Warn("failed to write pid file", "error", err)
	}

	// Start the manager (initializes context)
	if err := d.manager.Start(d.ctx); err != nil {
		return fmt.Errorf("failed to start manager: %w", err)
	}

	// Create socket HTTP server - long timeout for multi-agent tasks
	d.socketServer = &http.Server{
		Handler:      d.apiServer.Handler(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 600 * time.Second,
	}

	d.logger.Info("daemon starting", "socket", d.socketPath)

	// Start Unix socket server
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		if err := d.socketServer.Serve(d.listener); err != http.ErrServerClosed {
			d.logger.Error("socket server error", "error", err)
		}
	}()

	// Optionally start TCP server for remote access
	if d.tcpAddr != "" {
		tcpListener, err := net.Listen("tcp", d.tcpAddr)
		if err != nil {
			d.logger.Error("failed to create TCP listener", "addr", d.tcpAddr, "error", err)
		} else {
			d.tcpListener = tcpListener
			d.tcpServer = &http.Server{
				Handler:      d.apiServer.Handler(),
				ReadTimeout:  30 * time.Second,
				WriteTimeout: 600 * time.Second,
			}
			d.wg.Add(1)
			go func() {
				defer d.wg.Done()
				d.logger.Info("HTTP server starting", "addr", d.tcpAddr)
				if err := d.tcpServer.Serve(d.tcpListener); err != http.ErrServerClosed {
					d.logger.Error("TCP server error", "error", err)
				}
			}()
		}
	}

	d.logger.Info("daemon started successfully")
	return nil
}

// Run starts the daemon and blocks until shutdown
func (d *Daemon) Run() error {
	if err := d.Start(); err != nil {
		return err
	}

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		d.logger.Info("received signal", "signal", sig)
	case <-d.ctx.Done():
	}

	return d.Stop()
}

// Stop gracefully shuts down the daemon
func (d *Daemon) Stop() error {
	d.logger.Info("daemon stopping...")
	d.cancel()

	// Use a short timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shutdown both servers in parallel
	var shutdownWg sync.WaitGroup

	if d.socketServer != nil {
		shutdownWg.Add(1)
		go func() {
			defer shutdownWg.Done()
			d.socketServer.Shutdown(ctx)
		}()
	}

	if d.tcpServer != nil {
		shutdownWg.Add(1)
		go func() {
			defer shutdownWg.Done()
			d.tcpServer.Shutdown(ctx)
		}()
	}

	// Wait for servers to shutdown
	shutdownWg.Wait()

	// Close listeners
	if d.listener != nil {
		d.listener.Close()
	}
	if d.tcpListener != nil {
		d.tcpListener.Close()
	}

	// Stop all teams
	for _, t := range d.manager.ListTeams() {
		d.manager.StopTeam(t.Name)
	}

	// Close manager
	d.manager.Stop()

	// Remove socket and PID file
	os.Remove(d.socketPath)
	os.Remove(d.pidPath)

	// Wait for goroutines with timeout
	done := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Clean shutdown
	case <-time.After(2 * time.Second):
		d.logger.Warn("shutdown timeout, forcing exit")
	}

	d.logger.Info("daemon stopped")
	return nil
}

// Status returns the daemon status
func (d *Daemon) Status() map[string]interface{} {
	return map[string]interface{}{
		"running":  true,
		"socket":   d.socketPath,
		"tcp_addr": d.tcpAddr,
		"teams":    len(d.manager.ListTeams()),
		"manager":  d.manager.Status(),
	}
}

func (d *Daemon) isRunning() bool {
	data, err := os.ReadFile(d.pidPath)
	if err != nil {
		return false
	}

	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		return false
	}

	// Check if process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Send signal 0 to check if process exists
	return process.Signal(syscall.Signal(0)) == nil
}

func (d *Daemon) writePID() error {
	return os.WriteFile(d.pidPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0644)
}

// GetSocketPath returns the socket path for this daemon
func (d *Daemon) GetSocketPath() string {
	return d.socketPath
}
