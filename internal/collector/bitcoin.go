package collector

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/bitcoin-node-manager/btc-node-monitor/pkg/metrics"
)

// BitcoinCollector collects Bitcoin Core metrics via bitcoin-cli
type BitcoinCollector struct {
	cliPath string
	dataDir string
	user    string
	timeout time.Duration
}

// NewBitcoinCollector creates a new Bitcoin metrics collector
func NewBitcoinCollector(cliPath, dataDir, user string, timeoutSeconds int) *BitcoinCollector {
	return &BitcoinCollector{
		cliPath: cliPath,
		dataDir: dataDir,
		user:    user,
		timeout: time.Duration(timeoutSeconds) * time.Second,
	}
}

// Collect gathers current Bitcoin metrics
func (c *BitcoinCollector) Collect() (*metrics.BitcoinMetrics, error) {
	m := &metrics.BitcoinMetrics{}

	// Measure RPC latency with getblockchaininfo
	startTime := time.Now()
	blockchainInfo, err := c.getBlockchainInfo()
	if err != nil {
		return nil, fmt.Errorf("getblockchaininfo failed: %w", err)
	}
	m.RPCLatencyMs = time.Since(startTime).Milliseconds()

	// Parse blockchain info
	if blocks, ok := blockchainInfo["blocks"].(float64); ok {
		m.BlockHeight = int(blocks)
	}
	if headers, ok := blockchainInfo["headers"].(float64); ok {
		m.Headers = int(headers)
	}
	if progress, ok := blockchainInfo["verificationprogress"].(float64); ok {
		m.SyncProgress = progress
	}
	if ibd, ok := blockchainInfo["initialblockdownload"].(bool); ok {
		m.IBD = ibd
	}
	if pruned, ok := blockchainInfo["pruned"].(bool); ok {
		m.Pruned = pruned
	}
	if chain, ok := blockchainInfo["chain"].(string); ok {
		m.Chain = chain
	}
	if sizeOnDisk, ok := blockchainInfo["size_on_disk"].(float64); ok {
		m.ChainSizeBytes = int64(sizeOnDisk)
	}

	// Get network info
	networkInfo, err := c.getNetworkInfo()
	if err == nil {
		if connections, ok := networkInfo["connections"].(float64); ok {
			m.Peers = int(connections)
		}
		if connectionsIn, ok := networkInfo["connections_in"].(float64); ok {
			m.InboundPeers = int(connectionsIn)
		}
		if connectionsOut, ok := networkInfo["connections_out"].(float64); ok {
			m.OutboundPeers = int(connectionsOut)
		}
	}

	// Get mempool info
	mempoolInfo, err := c.getMempoolInfo()
	if err == nil {
		if size, ok := mempoolInfo["size"].(float64); ok {
			m.MempoolTxCount = int(size)
		}
		if bytes, ok := mempoolInfo["bytes"].(float64); ok {
			m.MempoolSizeBytes = int64(bytes)
		}
	}

	// Get uptime
	uptime, err := c.getUptime()
	if err == nil {
		m.UptimeSeconds = uptime
	}

	return m, nil
}

// runCLI executes bitcoin-cli command
func (c *BitcoinCollector) runCLI(args ...string) ([]byte, error) {
	// Build command: bitcoin-cli [args]
	// Agent runs as bitcoin user via systemd, so no sudo needed
	cmdArgs := []string{}
	if c.dataDir != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("-datadir=%s", c.dataDir))
	}
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.Command(c.cliPath, cmdArgs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Set timeout
	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	select {
	case err := <-done:
		if err != nil {
			return nil, fmt.Errorf("command failed: %w, stderr: %s", err, stderr.String())
		}
		return stdout.Bytes(), nil
	case <-time.After(c.timeout):
		cmd.Process.Kill()
		return nil, fmt.Errorf("command timed out after %v", c.timeout)
	}
}

// getBlockchainInfo executes getblockchaininfo RPC
func (c *BitcoinCollector) getBlockchainInfo() (map[string]interface{}, error) {
	output, err := c.runCLI("getblockchaininfo")
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse getblockchaininfo: %w", err)
	}

	return result, nil
}

// getNetworkInfo executes getnetworkinfo RPC
func (c *BitcoinCollector) getNetworkInfo() (map[string]interface{}, error) {
	output, err := c.runCLI("getnetworkinfo")
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse getnetworkinfo: %w", err)
	}

	return result, nil
}

// getMempoolInfo executes getmempoolinfo RPC
func (c *BitcoinCollector) getMempoolInfo() (map[string]interface{}, error) {
	output, err := c.runCLI("getmempoolinfo")
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse getmempoolinfo: %w", err)
	}

	return result, nil
}

// getUptime executes uptime RPC
func (c *BitcoinCollector) getUptime() (int, error) {
	output, err := c.runCLI("uptime")
	if err != nil {
		return 0, err
	}

	var uptime int
	if err := json.Unmarshal(output, &uptime); err != nil {
		return 0, fmt.Errorf("failed to parse uptime: %w", err)
	}

	return uptime, nil
}
