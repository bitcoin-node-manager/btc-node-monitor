package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bitcoin-node-manager/btc-node-monitor/internal/collector"
	"github.com/bitcoin-node-manager/btc-node-monitor/internal/config"
	"github.com/bitcoin-node-manager/btc-node-monitor/internal/server"
	"github.com/bitcoin-node-manager/btc-node-monitor/internal/storage"
)

const version = "0.1.2"

func main() {
	// Parse flags
	configPath := flag.String("config", "/var/lib/bitcoin-monitor/config.json", "Path to configuration file")
	showVersion := flag.Bool("version", false, "Show version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("btc-monitor version %s\n", version)
		os.Exit(0)
	}

	// Setup logging
	log.SetPrefix("[btc-monitor] ")
	log.SetFlags(log.LstdFlags)

	log.Printf("[INFO] Bitcoin Node Monitor v%s starting...", version)

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("[ERROR] Failed to load config: %v", err)
	}

	log.Printf("[INFO] Loaded configuration from %s", *configPath)
	log.Printf("[INFO] Collection interval: %ds, Retention: %d days", cfg.CollectionIntervalSeconds, cfg.RetentionDays)

	// Create data directory
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		log.Fatalf("[ERROR] Failed to create data directory: %v", err)
	}

	// Initialize storage
	stor, err := storage.NewStorage(cfg.DataDir, cfg.RetentionDays)
	if err != nil {
		log.Fatalf("[ERROR] Failed to initialize storage: %v", err)
	}
	defer stor.Close()

	log.Printf("[INFO] Storage initialized at %s", cfg.DataDir)

	// Initialize collector
	coll := collector.NewCollector(cfg)
	log.Printf("[INFO] Collector initialized (System: %v, Bitcoin: %v, Tor: %v)",
		cfg.System.Enabled, cfg.Bitcoin.Enabled, cfg.Tor.Enabled)

	// Initialize server
	srv := server.NewServer(cfg.SocketPath, stor, version)
	if err := srv.Start(); err != nil {
		log.Fatalf("[ERROR] Failed to start server: %v", err)
	}
	defer srv.Stop()

	log.Printf("[INFO] Server started on %s", cfg.SocketPath)

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Collection ticker
	ticker := time.NewTicker(time.Duration(cfg.CollectionIntervalSeconds) * time.Second)
	defer ticker.Stop()

	// Stats
	var collectionCount, errorCount int64

	log.Printf("[INFO] Starting collection loop...")

	// Initial collection
	collectAndStore(coll, stor, &collectionCount, &errorCount, srv)

	// Main loop
	for {
		select {
		case <-ticker.C:
			collectAndStore(coll, stor, &collectionCount, &errorCount, srv)

		case sig := <-sigChan:
			log.Printf("[INFO] Received signal %v, shutting down...", sig)
			return
		}
	}
}

// collectAndStore performs collection and storage
func collectAndStore(coll *collector.Collector, stor *storage.Storage, collectionCount, errorCount *int64, srv *server.Server) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[ERROR] Panic during collection: %v", r)
			*errorCount++
		}
	}()

	// Collect metrics
	sample := coll.Collect()

	// Write to storage
	if err := stor.Write(sample); err != nil {
		log.Printf("[ERROR] Failed to write sample: %v", err)
		*errorCount++
		return
	}

	*collectionCount++

	// Update server status
	srv.UpdateStatus(*collectionCount, *errorCount, sample.Timestamp)

	// Log summary
	if *collectionCount%10 == 0 {
		log.Printf("[INFO] Collected %d samples (%d errors)", *collectionCount, *errorCount)
	}
}
