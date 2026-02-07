package collector

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/bitcoin-node-manager/btc-node-monitor/pkg/metrics"
)

// TorCollector collects Tor network metrics via control port
type TorCollector struct {
	controlPort int
	cookiePath  string
	timeout     time.Duration
}

// NewTorCollector creates a new Tor metrics collector
func NewTorCollector(controlPort int, cookiePath string, timeoutSeconds int) *TorCollector {
	return &TorCollector{
		controlPort: controlPort,
		cookiePath:  cookiePath,
		timeout:     time.Duration(timeoutSeconds) * time.Second,
	}
}

// Collect gathers current Tor metrics
func (c *TorCollector) Collect() (*metrics.TorMetrics, error) {
	m := &metrics.TorMetrics{}

	startTime := time.Now()

	// Try to connect to control port
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", c.controlPort), c.timeout)
	if err != nil {
		m.ControlReachable = false
		return m, nil // Not an error, just Tor not available
	}
	defer conn.Close()

	m.ControlReachable = true
	conn.SetDeadline(time.Now().Add(c.timeout))

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Authenticate
	if err := c.authenticate(reader, writer); err != nil {
		return m, nil // Authentication failed, but connection worked
	}

	// Measure control latency
	m.ControlLatencyMs = time.Since(startTime).Milliseconds()

	// Get circuit status
	circuits, err := c.getCircuits(reader, writer)
	if err == nil {
		m.CircuitCount = len(circuits)
		// Count established circuits
		for _, circuit := range circuits {
			if strings.Contains(circuit, "BUILT") {
				m.EstablishedCount++
			}
		}
	}

	// Get bandwidth stats
	readBytes, writeBytes, err := c.getBandwidth(reader, writer)
	if err == nil {
		m.BandwidthReadBPS = readBytes
		m.BandwidthWriteBPS = writeBytes
	}

	// Get onion services count
	onions, err := c.getOnionServices(reader, writer)
	if err == nil {
		m.OnionServices = onions
	}

	return m, nil
}

// authenticate authenticates with Tor control port using cookie
func (c *TorCollector) authenticate(reader *bufio.Reader, writer *bufio.Writer) error {
	// Read cookie file
	cookie, err := os.ReadFile(c.cookiePath)
	if err != nil {
		// Try PROTOCOLINFO to see if no auth needed
		writer.WriteString("PROTOCOLINFO 1\r\n")
		writer.Flush()

		response, _ := reader.ReadString('\n')
		if strings.Contains(response, "NULL") {
			// No authentication required
			return nil
		}
		return fmt.Errorf("failed to read cookie: %w", err)
	}

	// Authenticate with cookie
	hexCookie := fmt.Sprintf("%x", cookie)
	writer.WriteString(fmt.Sprintf("AUTHENTICATE %s\r\n", hexCookie))
	writer.Flush()

	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	if !strings.HasPrefix(response, "250") {
		return fmt.Errorf("authentication rejected: %s", response)
	}

	return nil
}

// getCircuits retrieves circuit information
func (c *TorCollector) getCircuits(reader *bufio.Reader, writer *bufio.Writer) ([]string, error) {
	writer.WriteString("GETINFO circuit-status\r\n")
	writer.Flush()

	var circuits []string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "250 OK") {
			break
		}

		if strings.HasPrefix(line, "250-circuit-status=") || strings.HasPrefix(line, "250+circuit-status=") {
			continue
		}

		if line == "250 OK" || line == "." {
			break
		}

		if strings.HasPrefix(line, "250") && strings.Contains(line, "BUILT") {
			circuits = append(circuits, line)
		}
	}

	return circuits, nil
}

// getBandwidth retrieves bandwidth statistics
func (c *TorCollector) getBandwidth(reader *bufio.Reader, writer *bufio.Writer) (int64, int64, error) {
	// Note: This is cumulative, not rate. For rate calculation, we'd need to track deltas
	// For now, return 0 as placeholder
	return 0, 0, nil
}

// getOnionServices retrieves count of active onion services
func (c *TorCollector) getOnionServices(reader *bufio.Reader, writer *bufio.Writer) (int, error) {
	writer.WriteString("GETINFO onions/current\r\n")
	writer.Flush()

	count := 0
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return 0, err
		}

		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "250 OK") {
			break
		}

		if strings.HasPrefix(line, "250-onions/current=") || strings.HasPrefix(line, "250+onions/current=") {
			// Parse onion addresses
			parts := strings.Split(line, "=")
			if len(parts) > 1 {
				onions := strings.Split(parts[1], ",")
				count = len(onions)
			}
		}

		if line == "." {
			break
		}
	}

	return count, nil
}
