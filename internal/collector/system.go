package collector

import (
	"time"

	"github.com/bitcoin-node-manager/btc-node-monitor/pkg/metrics"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

// SystemCollector collects system metrics
type SystemCollector struct {
	diskPath string
	lastNet  *net.IOCountersStat
	lastDisk *disk.IOCountersStat
	lastTime time.Time
}

// NewSystemCollector creates a new system metrics collector
func NewSystemCollector(diskPath string) *SystemCollector {
	return &SystemCollector{
		diskPath: diskPath,
		lastTime: time.Now(),
	}
}

// Collect gathers current system metrics
func (c *SystemCollector) Collect() (*metrics.SystemMetrics, error) {
	m := &metrics.SystemMetrics{}

	// CPU percentage
	if cpuPercent, err := cpu.Percent(0, false); err == nil && len(cpuPercent) > 0 {
		m.CPUPercent = cpuPercent[0]
	}

	// Memory
	if vmStat, err := mem.VirtualMemory(); err == nil {
		m.MemoryTotalBytes = int64(vmStat.Total)
		m.MemoryUsedBytes = int64(vmStat.Used)
		m.MemoryAvailBytes = int64(vmStat.Available)
	}

	// Disk usage
	if diskStat, err := disk.Usage(c.diskPath); err == nil {
		m.DiskTotalBytes = int64(diskStat.Total)
		m.DiskUsedBytes = int64(diskStat.Used)
		m.DiskAvailBytes = int64(diskStat.Free)
	}

	// Disk I/O rates
	if diskIO, err := disk.IOCounters(); err == nil {
		// Sum all disk I/O
		var totalRead, totalWrite uint64
		for _, d := range diskIO {
			totalRead += d.ReadBytes
			totalWrite += d.WriteBytes
		}

		now := time.Now()
		if c.lastDisk != nil {
			elapsed := now.Sub(c.lastTime).Seconds()
			if elapsed > 0 {
				m.DiskReadBPS = int64(float64(totalRead-c.lastDisk.ReadBytes) / elapsed)
				m.DiskWriteBPS = int64(float64(totalWrite-c.lastDisk.WriteBytes) / elapsed)
			}
		}

		c.lastDisk = &disk.IOCountersStat{
			ReadBytes:  totalRead,
			WriteBytes: totalWrite,
		}
	}

	// Network I/O rates
	if netIO, err := net.IOCounters(false); err == nil && len(netIO) > 0 {
		now := time.Now()
		if c.lastNet != nil {
			elapsed := now.Sub(c.lastTime).Seconds()
			if elapsed > 0 {
				m.NetRxBPS = int64(float64(netIO[0].BytesRecv-c.lastNet.BytesRecv) / elapsed)
				m.NetTxBPS = int64(float64(netIO[0].BytesSent-c.lastNet.BytesSent) / elapsed)
			}
		}

		c.lastNet = &netIO[0]
		c.lastTime = now
	}

	// Load averages
	if loadStat, err := load.Avg(); err == nil {
		m.LoadAvg1m = loadStat.Load1
		m.LoadAvg5m = loadStat.Load5
		m.LoadAvg15m = loadStat.Load15
	}

	// Uptime
	if uptime, err := host.Uptime(); err == nil {
		m.UptimeSeconds = int64(uptime)
	}

	return m, nil
}
