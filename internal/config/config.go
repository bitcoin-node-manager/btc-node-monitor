package config

import (
	"encoding/json"
	"os"
)

// Config represents the monitoring agent configuration
type Config struct {
	CollectionIntervalSeconds int           `json:"collection_interval_seconds"`
	RetentionDays             int           `json:"retention_days"`
	DataDir                   string        `json:"data_dir"`
	SocketPath                string        `json:"socket_path"`
	Bitcoin                   BitcoinConfig `json:"bitcoin"`
	Tor                       TorConfig     `json:"tor"`
	System                    SystemConfig  `json:"system"`
}

// BitcoinConfig contains Bitcoin Core monitoring settings
type BitcoinConfig struct {
	Enabled        bool   `json:"enabled"`
	CLIPath        string `json:"cli_path"`
	DataDir        string `json:"data_dir"`
	User           string `json:"user"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

// TorConfig contains Tor monitoring settings
type TorConfig struct {
	Enabled        bool   `json:"enabled"`
	ControlPort    int    `json:"control_port"`
	CookiePath     string `json:"cookie_path"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

// SystemConfig contains system monitoring settings
type SystemConfig struct {
	Enabled         bool   `json:"enabled"`
	MonitorDiskPath string `json:"monitor_disk_path"` // Path to monitor for disk metrics
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		CollectionIntervalSeconds: 30,
		RetentionDays:             30,
		DataDir:                   "/var/lib/bitcoin-monitor",
		SocketPath:                "/var/run/bitcoin-monitor.sock",
		Bitcoin: BitcoinConfig{
			Enabled:        true,
			CLIPath:        "/usr/local/bin/bitcoin-cli",
			DataDir:        "/var/lib/bitcoin",
			User:           "bitcoin",
			TimeoutSeconds: 10,
		},
		Tor: TorConfig{
			Enabled:        true,
			ControlPort:    9051,
			CookiePath:     "/var/lib/tor/control_auth_cookie",
			TimeoutSeconds: 10,
		},
		System: SystemConfig{
			Enabled:         true,
			MonitorDiskPath: "/var/lib/bitcoin",
		},
	}
}

// LoadConfig loads configuration from a JSON file
func LoadConfig(path string) (*Config, error) {
	// Start with default config
	cfg := DefaultConfig()

	// If file doesn't exist, return default config
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Unmarshal onto default config, so missing fields keep defaults
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Apply defaults for any zero-valued critical fields
	if cfg.Bitcoin.CLIPath == "" {
		cfg.Bitcoin.CLIPath = "/usr/local/bin/bitcoin-cli"
	}
	if cfg.Bitcoin.User == "" {
		cfg.Bitcoin.User = "bitcoin"
	}
	if cfg.Bitcoin.TimeoutSeconds == 0 {
		cfg.Bitcoin.TimeoutSeconds = 10
	}
	if cfg.Tor.ControlPort == 0 {
		cfg.Tor.ControlPort = 9051
	}
	if cfg.Tor.TimeoutSeconds == 0 {
		cfg.Tor.TimeoutSeconds = 10
	}

	return cfg, nil
}

// SaveConfig saves configuration to a JSON file
func SaveConfig(cfg *Config, path string) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
