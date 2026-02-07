package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/bitcoin-node-manager/btc-node-monitor/internal/storage"
	"github.com/bitcoin-node-manager/btc-node-monitor/pkg/metrics"
)

// Server handles Unix socket queries
type Server struct {
	socketPath string
	storage    *storage.Storage
	listener   net.Listener
	status     *metrics.AgentStatus
	startTime  time.Time
}

// NewServer creates a new query server
func NewServer(socketPath string, storage *storage.Storage, version string) *Server {
	return &Server{
		socketPath: socketPath,
		storage:    storage,
		status: &metrics.AgentStatus{
			Running: true,
			Version: version,
		},
		startTime: time.Now(),
	}
}

// Start starts the Unix socket server
func (s *Server) Start() error {
	// Remove existing socket if it exists
	os.Remove(s.socketPath)

	// Create Unix listener
	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("failed to create socket: %w", err)
	}

	// Set socket permissions
	if err := os.Chmod(s.socketPath, 0660); err != nil {
		listener.Close()
		return fmt.Errorf("failed to set socket permissions: %w", err)
	}

	s.listener = listener
	log.Printf("[INFO] Socket server listening on %s", s.socketPath)

	// Accept connections
	go s.acceptConnections()

	return nil
}

// acceptConnections handles incoming connections
func (s *Server) acceptConnections() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			log.Printf("[WARN] Failed to accept connection: %v", err)
			continue
		}

		go s.handleConnection(conn)
	}
}

// handleConnection processes a client connection
func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Set timeout
	conn.SetDeadline(time.Now().Add(10 * time.Second))

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		log.Printf("[WARN] Failed to read from connection: %v", err)
		return
	}

	line = strings.TrimSpace(line)
	parts := strings.Fields(line)

	if len(parts) == 0 {
		s.writeError(conn, "empty command")
		return
	}

	command := strings.ToUpper(parts[0])

	switch command {
	case "GET":
		s.handleGet(conn, parts[1:])
	default:
		s.writeError(conn, fmt.Sprintf("unknown command: %s", command))
	}
}

// handleGet handles GET commands
func (s *Server) handleGet(conn net.Conn, args []string) {
	if len(args) == 0 {
		s.writeError(conn, "GET requires subcommand")
		return
	}

	subcommand := strings.ToLower(args[0])

	switch subcommand {
	case "status":
		s.handleGetStatus(conn)
	case "current":
		s.handleGetCurrent(conn)
	case "metrics":
		s.handleGetMetrics(conn, args[1:])
	case "config":
		s.handleGetConfig(conn)
	default:
		s.writeError(conn, fmt.Sprintf("unknown GET subcommand: %s", subcommand))
	}
}

// handleGetStatus returns agent status
func (s *Server) handleGetStatus(conn net.Conn) {
	s.status.UptimeSeconds = int64(time.Since(s.startTime).Seconds())

	data, err := json.Marshal(s.status)
	if err != nil {
		s.writeError(conn, fmt.Sprintf("failed to marshal status: %v", err))
		return
	}

	conn.Write(append(data, '\n'))
}

// handleGetCurrent returns the most recent sample
func (s *Server) handleGetCurrent(conn net.Conn) {
	sample, err := s.storage.GetCurrent()
	if err != nil {
		s.writeError(conn, fmt.Sprintf("failed to get current sample: %v", err))
		return
	}

	if sample == nil {
		s.writeError(conn, "no samples available")
		return
	}

	data, err := json.Marshal(sample)
	if err != nil {
		s.writeError(conn, fmt.Sprintf("failed to marshal sample: %v", err))
		return
	}

	conn.Write(append(data, '\n'))
}

// handleGetMetrics returns historical metrics
func (s *Server) handleGetMetrics(conn net.Conn, args []string) {
	if len(args) < 2 {
		s.writeError(conn, "GET metrics requires start and end time (ISO8601)")
		return
	}

	startTime, err := time.Parse(time.RFC3339, args[0])
	if err != nil {
		s.writeError(conn, fmt.Sprintf("invalid start time: %v", err))
		return
	}

	endTime, err := time.Parse(time.RFC3339, args[1])
	if err != nil {
		s.writeError(conn, fmt.Sprintf("invalid end time: %v", err))
		return
	}

	samples, err := s.storage.Query(startTime, endTime)
	if err != nil {
		s.writeError(conn, fmt.Sprintf("failed to query metrics: %v", err))
		return
	}

	data, err := json.Marshal(samples)
	if err != nil {
		s.writeError(conn, fmt.Sprintf("failed to marshal samples: %v", err))
		return
	}

	conn.Write(append(data, '\n'))
}

// handleGetConfig returns current configuration (placeholder)
func (s *Server) handleGetConfig(conn net.Conn) {
	// TODO: Return actual config
	conn.Write([]byte("{}\n"))
}

// writeError writes an error response
func (s *Server) writeError(conn net.Conn, message string) {
	errResp := map[string]string{
		"error": message,
	}
	data, _ := json.Marshal(errResp)
	conn.Write(append(data, '\n'))
}

// UpdateStatus updates the agent status
func (s *Server) UpdateStatus(collectionCount, errorCount int64, lastCollectionTime time.Time) {
	s.status.CollectionCount = collectionCount
	s.status.ErrorCount = errorCount
	s.status.LastCollectionTime = lastCollectionTime
}

// Stop stops the server
func (s *Server) Stop() error {
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}
