package pgm

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// SampleStatus represents the result of a sample import operation.
type SampleStatus int

const (
	SampleOK       SampleStatus = iota
	SampleRenamed                // filename was too long, was shortened
	SampleRejected              // invalid file (wrong extension, etc.)
	SampleIgnored               // skipped
	SampleNotFound              // referenced file not found on disk
)

// SampleRef represents a reference to a sample WAV file.
type SampleRef struct {
	Name       string       // sample name without extension (max 16 chars)
	FilePath   string       // full path to the .wav file (empty if not found)
	Status     SampleStatus
}

// ImportSample validates a file for use as an MPC sample.
// Returns a SampleRef with appropriate status.
// If the filename (without extension) exceeds 16 chars, it is truncated and status is SampleRenamed.
func ImportSample(path string) SampleRef {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".wav" {
		return SampleRef{
			Name:   filepath.Base(path),
			Status: SampleRejected,
		}
	}

	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	status := SampleOK
	if len(name) > 16 {
		name = name[:16]
		status = SampleRenamed
	}

	return SampleRef{
		Name:     name,
		FilePath: path,
		Status:   status,
	}
}

// FindSample looks for a sample WAV file in the given directory.
// The MPC stores sample names without extension, so we append ".wav" / ".WAV".
func FindSample(name string, dir string) SampleRef {
	if name == "" {
		return SampleRef{Status: SampleNotFound}
	}

	// Try exact name with common extensions
	for _, ext := range []string{".wav", ".WAV", ".Wav"} {
		path := filepath.Join(dir, name+ext)
		if _, err := os.Stat(path); err == nil {
			return SampleRef{
				Name:     name,
				FilePath: path,
				Status:   SampleOK,
			}
		}
	}

	return SampleRef{
		Name:   name,
		Status: SampleNotFound,
	}
}

// EscapeName truncates and cleans a filename to fit the MPC's 16-char limit.
func EscapeName(name string, maxLen int) string {
	if maxLen <= 0 {
		maxLen = 16
	}
	if len(name) <= maxLen {
		return name
	}
	return name[:maxLen]
}

// SampleMatrix is a 64x4 grid of sample references (one per pad per layer).
type SampleMatrix [64][4]*SampleRef

// Set assigns a sample to a specific pad and layer.
func (m *SampleMatrix) Set(pad, layer int, ref *SampleRef) {
	m[pad][layer] = ref
}

// Get returns the sample reference at a specific pad and layer.
func (m *SampleMatrix) Get(pad, layer int) *SampleRef {
	return m[pad][layer]
}

// Clear removes all sample references.
func (m *SampleMatrix) Clear() {
	for i := range m {
		for j := range m[i] {
			m[i][j] = nil
		}
	}
}

// CollectAll returns all non-nil sample references.
func (m *SampleMatrix) CollectAll() []*SampleRef {
	var refs []*SampleRef
	seen := make(map[string]bool)
	for i := range m {
		for j := range m[i] {
			ref := m[i][j]
			if ref != nil && ref.FilePath != "" && !seen[ref.FilePath] {
				refs = append(refs, ref)
				seen[ref.FilePath] = true
			}
		}
	}
	return refs
}

// CopySample copies a sample file to a destination directory.
func CopySample(ref *SampleRef, destDir string) error {
	if ref == nil || ref.FilePath == "" {
		return fmt.Errorf("no file path")
	}
	src, err := os.Open(ref.FilePath)
	if err != nil {
		return err
	}
	defer src.Close()

	destPath := filepath.Join(destDir, filepath.Base(ref.FilePath))
	dst, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}
