package collector

import (
	"log"
	"time"

	"github.com/bitcoin-node-manager/btc-node-monitor/internal/config"
	"github.com/bitcoin-node-manager/btc-node-monitor/pkg/metrics"
)

// Collector orchestrates all metric collection
type Collector struct {
	config  *config.Config
	system  *SystemCollector
	bitcoin *BitcoinCollector
	tor     *TorCollector
}

// NewCollector creates a new metrics collector
func NewCollector(cfg *config.Config) *Collector {
	return &Collector{
		config:  cfg,
		system:  NewSystemCollector(cfg.System.MonitorDiskPath),
		bitcoin: NewBitcoinCollector(cfg.Bitcoin.CLIPath, cfg.Bitcoin.DataDir, cfg.Bitcoin.User, cfg.Bitcoin.TimeoutSeconds),
		tor:     NewTorCollector(cfg.Tor.ControlPort, cfg.Tor.CookiePath, cfg.Tor.TimeoutSeconds),
	}
}

// Collect gathers all enabled metrics
func (c *Collector) Collect() *metrics.Sample {
	sample := &metrics.Sample{
		Timestamp: time.Now().UTC(),
	}

	// System metrics (always collect if enabled)
	if c.config.System.Enabled {
		systemMetrics, err := c.system.Collect()
		if err != nil {
			log.Printf("[WARN] Failed to collect system metrics: %v", err)
		} else {
			sample.System = systemMetrics
		}
	}

	// Bitcoin metrics
	if c.config.Bitcoin.Enabled {
		bitcoinMetrics, err := c.bitcoin.Collect()
		if err != nil {
			log.Printf("[WARN] Failed to collect Bitcoin metrics: %v", err)
		} else {
			sample.Bitcoin = bitcoinMetrics
		}
	}

	// Tor metrics
	if c.config.Tor.Enabled {
		torMetrics, err := c.tor.Collect()
		if err != nil {
			log.Printf("[WARN] Failed to collect Tor metrics: %v", err)
		} else {
			sample.Tor = torMetrics
		}
	}

	return sample
}
