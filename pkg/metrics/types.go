package metrics

import "time"

// Sample represents a complete metrics snapshot at a point in time
type Sample struct {
	Timestamp time.Time       `json:"timestamp"`
	System    *SystemMetrics  `json:"system,omitempty"`
	Bitcoin   *BitcoinMetrics `json:"bitcoin,omitempty"`
	Tor       *TorMetrics     `json:"tor,omitempty"`
}

// SystemMetrics contains host system performance data
type SystemMetrics struct {
	CPUPercent       float64 `json:"cpu_percent"`
	MemoryUsedBytes  int64   `json:"memory_used_bytes"`
	MemoryTotalBytes int64   `json:"memory_total_bytes"`
	MemoryAvailBytes int64   `json:"memory_avail_bytes"`
	DiskUsedBytes    int64   `json:"disk_used_bytes"`
	DiskTotalBytes   int64   `json:"disk_total_bytes"`
	DiskAvailBytes   int64   `json:"disk_avail_bytes"`
	DiskReadBPS      int64   `json:"disk_read_bps"`      // Bytes per second
	DiskWriteBPS     int64   `json:"disk_write_bps"`     // Bytes per second
	NetRxBPS         int64   `json:"net_rx_bps"`         // Bytes per second
	NetTxBPS         int64   `json:"net_tx_bps"`         // Bytes per second
	LoadAvg1m        float64 `json:"load_avg_1m"`
	LoadAvg5m        float64 `json:"load_avg_5m"`
	LoadAvg15m       float64 `json:"load_avg_15m"`
	UptimeSeconds    int64   `json:"uptime_seconds"`
}

// BitcoinMetrics contains Bitcoin Core node data
type BitcoinMetrics struct {
	BlockHeight      int     `json:"block_height"`
	Headers          int     `json:"headers"`
	SyncProgress     float64 `json:"sync_progress"`      // 0.0 to 1.0
	IBD              bool    `json:"ibd"`                // Initial Block Download
	Peers            int     `json:"peers"`
	InboundPeers     int     `json:"inbound_peers"`
	OutboundPeers    int     `json:"outbound_peers"`
	MempoolTxCount   int     `json:"mempool_tx_count"`
	MempoolSizeBytes int64   `json:"mempool_size_bytes"`
	ChainSizeBytes   int64   `json:"chain_size_bytes"`
	UptimeSeconds    int     `json:"uptime_seconds"`
	RPCLatencyMs     int64   `json:"rpc_latency_ms"`     // Time to execute getblockchaininfo
	Pruned           bool    `json:"pruned"`
	Chain            string  `json:"chain"`              // "main", "test", "regtest"
}

// TorMetrics contains Tor network data
type TorMetrics struct {
	ControlReachable  bool   `json:"control_reachable"`
	CircuitCount      int    `json:"circuit_count"`
	EstablishedCount  int    `json:"established_count"`
	BandwidthReadBPS  int64  `json:"bandwidth_read_bps"`  // Bytes per second
	BandwidthWriteBPS int64  `json:"bandwidth_write_bps"` // Bytes per second
	OnionServices     int    `json:"onion_services"`
	ControlLatencyMs  int64  `json:"control_latency_ms"`
}

// AgentStatus represents the current state of the monitoring agent
type AgentStatus struct {
	Running            bool      `json:"running"`
	UptimeSeconds      int64     `json:"uptime_seconds,omitempty"`
	CollectionCount    int64     `json:"collection_count,omitempty"`
	LastCollectionTime time.Time `json:"last_collection_time,omitempty"`
	ErrorCount         int64     `json:"error_count,omitempty"`
	Version            string    `json:"version,omitempty"`
}
