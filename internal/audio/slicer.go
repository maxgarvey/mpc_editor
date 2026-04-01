package audio

import (
	"fmt"
	"path/filepath"
)

const (
	DefaultWindowSize            = 1024
	DefaultOverlapRatio          = 1
	DefaultLocalEnergyWindowSize = 43
	DefaultSensitivity           = 130
)

// Slicer performs beat detection on an audio sample and manages slice markers.
type Slicer struct {
	sample                *Sample
	channels              [][]int
	windowSize            int
	overlapRatio          int
	localEnergyWindowSize int
	sensitivity           int
	Markers               *Markers
}

// NewSlicer creates a Slicer for the given sample with default parameters.
func NewSlicer(sample *Sample) *Slicer {
	return NewSlicerWithParams(sample, DefaultWindowSize, DefaultOverlapRatio, DefaultLocalEnergyWindowSize)
}

// NewSlicerWithParams creates a Slicer with custom parameters.
func NewSlicerWithParams(sample *Sample, windowSize, overlapRatio, localEnergyWindowSize int) *Slicer {
	s := &Slicer{
		sample:                sample,
		channels:              sample.AsSamples(),
		windowSize:            windowSize,
		overlapRatio:          overlapRatio,
		localEnergyWindowSize: localEnergyWindowSize,
		sensitivity:           DefaultSensitivity,
		Markers:               NewMarkers(),
	}
	s.ExtractMarkers()
	return s
}

// ExtractMarkers runs beat detection and populates the markers.
func (s *Slicer) ExtractMarkers() {
	s.Markers.Clear(s.sample.FrameLength, s.sample.Format.SampleRate)

	step := s.windowSize / s.overlapRatio

	energyHistory := s.energyHistory()
	if energyHistory == nil || len(energyHistory) < s.localEnergyWindowSize {
		return
	}

	samplesL := s.samplesL()
	lastState := false
	for i := 0; i < len(energyHistory); i++ {
		e := energyHistory[i]
		localE := s.localEnergy(i, energyHistory)
		c := float64(s.sensitivity) / 100.0
		if float64(e) > c*float64(localE) {
			// Got a beat
			location := i * step
			if !lastState {
				adjusted := nearestZeroCrossing(samplesL, location, s.windowSize)
				s.Markers.Add(adjusted)
				lastState = true
			}
		} else {
			lastState = false
		}
	}
}

// SetSensitivity updates the sensitivity and re-runs detection.
func (s *Slicer) SetSensitivity(sensitivity int) {
	s.sensitivity = sensitivity
	s.ExtractMarkers()
}

// GetSensitivity returns the current sensitivity.
func (s *Slicer) GetSensitivity() int {
	return s.sensitivity
}

// GetSelectedSlice returns the sample data for the currently selected slice.
func (s *Slicer) GetSelectedSlice() *Sample {
	return s.GetSlice(s.Markers.SelectedIndex())
}

// GetSlice returns the sample data for the slice at the given marker index.
func (s *Slicer) GetSlice(markerIndex int) *Sample {
	r := s.Markers.GetRangeFrom(markerIndex)
	return s.sample.SubRegion(r.From, r.To)
}

// ExportSlices exports each slice as a separate WAV file.
// Returns the list of created file paths.
func (s *Slicer) ExportSlices(dir, prefix string) ([]string, error) {
	var paths []string
	n := s.Markers.Size()
	for i := 0; i < n; i++ {
		slice := s.GetSlice(i)
		path := filepath.Join(dir, fmt.Sprintf("%s%d.wav", prefix, i))
		if err := slice.SaveWAV(path); err != nil {
			return paths, fmt.Errorf("export slice %d: %w", i, err)
		}
		paths = append(paths, path)
	}
	return paths, nil
}

// AdjustNearestZeroCrossing finds the nearest zero crossing for a given location.
func (s *Slicer) AdjustNearestZeroCrossing(location, excursion int) int {
	return nearestZeroCrossing(s.samplesL(), location, excursion)
}

// Sample returns the underlying audio sample.
func (s *Slicer) Sample() *Sample {
	return s.sample
}

// Channels returns the decoded per-channel sample data.
func (s *Slicer) Channels() [][]int {
	return s.channels
}

func (s *Slicer) String() string {
	return fmt.Sprintf("Slicer: %.2fs (%d samples), %d markers",
		s.Markers.Duration(), s.sample.FrameLength, s.Markers.Size())
}
