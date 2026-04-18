package device

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// MPCDevice represents a detected MPC 1000 connected via USB.
type MPCDevice struct {
	VolumeName  string // volume label, e.g. "MPC_CF"
	MountPath   string // full path, e.g. "/Volumes/MPC_CF"
	HasAutoload bool   // true if AUTOLOAD directory exists
	PGMCount    int    // number of .pgm files in volume root
}

// Detector polls for MPC 1000 volumes mounted under a base path.
type Detector struct {
	basePath string // default "/Volumes"
	interval time.Duration
	mu       sync.RWMutex
	current  *MPCDevice
}

// Option configures a Detector.
type Option func(*Detector)

// WithBasePath overrides the default "/Volumes" scan path (useful for tests).
func WithBasePath(path string) Option {
	return func(d *Detector) {
		d.basePath = path
	}
}

// WithInterval overrides the default 3-second polling interval.
func WithInterval(interval time.Duration) Option {
	return func(d *Detector) {
		d.interval = interval
	}
}

// New creates a Detector with the given options.
func New(opts ...Option) *Detector {
	d := &Detector{
		basePath: "/Volumes",
		interval: 3 * time.Second,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Start begins background polling. It blocks until ctx is cancelled.
func (d *Detector) Start(ctx context.Context) {
	// Do an initial scan immediately.
	d.scan()

	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.scan()
		}
	}
}

// Current returns the currently detected device, or nil if none.
func (d *Detector) Current() *MPCDevice {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.current
}

// Scan runs an immediate one-shot detection and returns the result.
func (d *Detector) Scan() *MPCDevice {
	d.scan()
	return d.Current()
}

func (d *Detector) scan() {
	dev := detectMPC(d.basePath)

	d.mu.Lock()
	prev := d.current
	d.current = dev
	d.mu.Unlock()

	// Log transitions.
	if prev == nil && dev != nil {
		log.Printf("MPC detected: %s at %s (%d programs)", dev.VolumeName, dev.MountPath, dev.PGMCount)
	} else if prev != nil && dev == nil {
		log.Printf("MPC disconnected (was %s)", prev.VolumeName)
	}
}

// detectMPC scans the base path for a volume that looks like an MPC 1000 CF card.
// Two heuristics are tried: presence of an AUTOLOAD directory, or a volume name
// that contains "MPC" (case-insensitive, e.g. MPC1000DISK, MPC_CF, MPC1000).
func detectMPC(basePath string) *MPCDevice {
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return nil
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		name := e.Name()
		volPath := filepath.Join(basePath, name)

		// Check for the AUTOLOAD directory — strong MPC 1000 signal.
		autoloadPath := filepath.Join(volPath, "AUTOLOAD")
		hasAutoload := false
		if info, err := os.Stat(autoloadPath); err == nil && info.IsDir() {
			hasAutoload = true
		}

		// Volume name contains "MPC" — matches MPC1000DISK, MPC_CF, MPC1000, etc.
		nameMatchesMPC := strings.Contains(strings.ToUpper(name), "MPC")

		if !hasAutoload && !nameMatchesMPC {
			continue
		}

		// Count .pgm files in the volume root (shallow).
		pgmCount := countPGMFiles(volPath)

		return &MPCDevice{
			VolumeName:  name,
			MountPath:   volPath,
			HasAutoload: hasAutoload,
			PGMCount:    pgmCount,
		}
	}

	return nil
}

// countPGMFiles counts .pgm files in a directory (non-recursive).
func countPGMFiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && strings.ToLower(filepath.Ext(e.Name())) == ".pgm" {
			count++
		}
	}
	return count
}
