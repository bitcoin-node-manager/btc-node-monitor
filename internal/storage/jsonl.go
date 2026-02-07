package storage

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bitcoin-node-manager/btc-node-monitor/pkg/metrics"
)

// Storage handles JSON Lines file storage with rotation
type Storage struct {
	dataDir     string
	currentFile *os.File
	currentDay  string
	retention   int // days
}

// NewStorage creates a new storage handler
func NewStorage(dataDir string, retentionDays int) (*Storage, error) {
	// Create data directory if it doesn't exist
	metricsDir := filepath.Join(dataDir, "metrics")
	if err := os.MkdirAll(metricsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create metrics directory: %w", err)
	}

	s := &Storage{
		dataDir:   metricsDir,
		retention: retentionDays,
	}

	// Open current day's file
	if err := s.rotateIfNeeded(); err != nil {
		return nil, err
	}

	// Clean up old files
	go s.cleanupOldFiles()

	return s, nil
}

// Write writes a sample to storage
func (s *Storage) Write(sample *metrics.Sample) error {
	// Check if rotation needed
	if err := s.rotateIfNeeded(); err != nil {
		return err
	}

	// Marshal to JSON
	data, err := json.Marshal(sample)
	if err != nil {
		return fmt.Errorf("failed to marshal sample: %w", err)
	}

	// Write line
	if _, err := s.currentFile.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write sample: %w", err)
	}

	// Flush to disk
	return s.currentFile.Sync()
}

// Query retrieves samples within a time range
func (s *Storage) Query(startTime, endTime time.Time) ([]*metrics.Sample, error) {
	var samples []*metrics.Sample

	// Find all relevant files
	files, err := s.getFilesForTimeRange(startTime, endTime)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		fileSamples, err := s.readFile(file, startTime, endTime)
		if err != nil {
			// Log warning but continue
			fmt.Printf("[WARN] Failed to read file %s: %v\n", file, err)
			continue
		}
		samples = append(samples, fileSamples...)
	}

	// Sort by timestamp
	sort.Slice(samples, func(i, j int) bool {
		return samples[i].Timestamp.Before(samples[j].Timestamp)
	})

	return samples, nil
}

// GetCurrent retrieves the most recent sample
func (s *Storage) GetCurrent() (*metrics.Sample, error) {
	// Try to read last line from current file
	if s.currentFile == nil {
		return nil, fmt.Errorf("no current file open")
	}

	// Seek to beginning
	if _, err := s.currentFile.Seek(0, 0); err != nil {
		return nil, err
	}

	var lastSample *metrics.Sample
	scanner := bufio.NewScanner(s.currentFile)

	for scanner.Scan() {
		var sample metrics.Sample
		if err := json.Unmarshal(scanner.Bytes(), &sample); err != nil {
			continue // Skip malformed lines
		}
		lastSample = &sample
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return lastSample, nil
}

// rotateIfNeeded checks if file rotation is needed and performs it
func (s *Storage) rotateIfNeeded() error {
	now := time.Now().UTC()
	currentDay := now.Format("2006-01-02")

	if currentDay == s.currentDay && s.currentFile != nil {
		return nil // No rotation needed
	}

	// Close current file
	if s.currentFile != nil {
		s.currentFile.Close()

		// Compress previous day's file in background
		oldPath := filepath.Join(s.dataDir, s.currentDay+".jsonl")
		go compressFile(oldPath)
	}

	// Open new file
	newPath := filepath.Join(s.dataDir, currentDay+".jsonl")
	file, err := os.OpenFile(newPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open metrics file: %w", err)
	}

	s.currentFile = file
	s.currentDay = currentDay

	return nil
}

// getFilesForTimeRange returns files that may contain data for the time range
func (s *Storage) getFilesForTimeRange(startTime, endTime time.Time) ([]string, error) {
	var files []string

	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".jsonl") && !strings.HasSuffix(name, ".jsonl.gz") {
			continue
		}

		// Extract date from filename
		dateStr := strings.TrimSuffix(strings.TrimSuffix(name, ".gz"), ".jsonl")
		fileDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}

		// Check if file is within range
		fileEndOfDay := fileDate.Add(24 * time.Hour)
		if fileEndOfDay.Before(startTime) || fileDate.After(endTime) {
			continue
		}

		files = append(files, filepath.Join(s.dataDir, name))
	}

	sort.Strings(files)
	return files, nil
}

// readFile reads samples from a file (handles .gz)
func (s *Storage) readFile(path string, startTime, endTime time.Time) ([]*metrics.Sample, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var reader io.Reader = file

	// Handle gzip files
	if strings.HasSuffix(path, ".gz") {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return nil, err
		}
		defer gzReader.Close()
		reader = gzReader
	}

	var samples []*metrics.Sample
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		var sample metrics.Sample
		if err := json.Unmarshal(scanner.Bytes(), &sample); err != nil {
			continue // Skip malformed lines
		}

		// Filter by time range
		if sample.Timestamp.Before(startTime) || sample.Timestamp.After(endTime) {
			continue
		}

		samples = append(samples, &sample)
	}

	return samples, scanner.Err()
}

// cleanupOldFiles removes files older than retention period
func (s *Storage) cleanupOldFiles() {
	cutoff := time.Now().UTC().AddDate(0, 0, -s.retention)

	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		fmt.Printf("[WARN] Failed to read data directory: %v\n", err)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".jsonl.gz") {
			continue
		}

		// Extract date from filename
		dateStr := strings.TrimSuffix(strings.TrimSuffix(name, ".gz"), ".jsonl")
		fileDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}

		// Delete if older than retention
		if fileDate.Before(cutoff) {
			path := filepath.Join(s.dataDir, name)
			if err := os.Remove(path); err != nil {
				fmt.Printf("[WARN] Failed to delete old file %s: %v\n", name, err)
			} else {
				fmt.Printf("[INFO] Deleted old metrics file: %s\n", name)
			}
		}
	}
}

// compressFile compresses a .jsonl file with gzip
func compressFile(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return // File doesn't exist
	}

	// Open source file
	src, err := os.Open(path)
	if err != nil {
		fmt.Printf("[WARN] Failed to open file for compression: %v\n", err)
		return
	}
	defer src.Close()

	// Create destination file
	dst, err := os.Create(path + ".gz")
	if err != nil {
		fmt.Printf("[WARN] Failed to create compressed file: %v\n", err)
		return
	}
	defer dst.Close()

	// Compress
	gzWriter := gzip.NewWriter(dst)
	defer gzWriter.Close()

	if _, err := io.Copy(gzWriter, src); err != nil {
		fmt.Printf("[WARN] Failed to compress file: %v\n", err)
		return
	}

	// Close and delete original
	src.Close()
	gzWriter.Close()
	dst.Close()

	if err := os.Remove(path); err != nil {
		fmt.Printf("[WARN] Failed to delete original file: %v\n", err)
	} else {
		fmt.Printf("[INFO] Compressed metrics file: %s\n", filepath.Base(path))
	}
}

// Close closes the storage
func (s *Storage) Close() error {
	if s.currentFile != nil {
		return s.currentFile.Close()
	}
	return nil
}
