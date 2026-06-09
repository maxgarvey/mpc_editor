package pgm

import (
	"os"
	"path/filepath"
	"strings"
)

// SampleStatus represents the result of a sample import operation.
type SampleStatus int

const (
	SampleOK       SampleStatus = iota
	SampleRenamed               // filename was too long, was shortened
	SampleRejected              // invalid file (wrong extension, etc.)
	SampleIgnored               // skipped
	SampleNotFound              // referenced file not found on disk
)

// SampleRef represents a reference to a sample WAV file.
type SampleRef struct {
	Name     string // sample name without extension (max 16 chars)
	FilePath string // full path to the .wav file (empty if not found)
	Status   SampleStatus
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
func FindSample(name, dir string) SampleRef {
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

// FindSampleInDirs searches multiple directories for a sample, returning the first match.
func FindSampleInDirs(name string, dirs ...string) SampleRef {
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		ref := FindSample(name, dir)
		if ref.Status == SampleOK {
			return ref
		}
	}
	return SampleRef{Name: name, Status: SampleNotFound}
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
