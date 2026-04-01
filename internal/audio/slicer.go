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

// --- internal algorithms ---

func (s *Slicer) samplesL() []int {
	return s.channels[0]
}

func (s *Slicer) energyHistory() []int64 {
	samplesL := s.samplesL()
	var samplesR []int
	if len(s.channels) > 1 {
		samplesR = s.channels[1]
	} else {
		samplesR = samplesL
	}

	n := len(samplesL)
	step := s.windowSize / s.overlapRatio
	energyFrameNumber := n / step
	if energyFrameNumber < 1 {
		return nil
	}

	energy := make([]int64, energyFrameNumber)
	windowIndex := 0
	for i := 0; i+s.windowSize < n; i += step {
		var sum int64
		for j := 0; j < s.windowSize; j++ {
			l := int64(samplesL[i+j])
			r := int64(samplesR[i+j])
			sum += l*l + r*r
		}
		if windowIndex < len(energy) {
			energy[windowIndex] = sum
		}
		windowIndex++
	}
	return energy
}

func (s *Slicer) localEnergy(i int, energyHistory []int64) int64 {
	n := len(energyHistory)
	m := s.localEnergyWindowSize

	var from, to int
	if i < m {
		from = 0
		to = m
	} else if i+m < n {
		from = i
		to = i + m
	} else {
		from = n - m
		to = n
	}

	var sum int64
	for j := from; j < to; j++ {
		sum += energyHistory[j]
	}
	return sum / int64(m)
}

func nearestZeroCrossing(samples []int, index, excursion int) int {
	if index == 0 {
		return 0
	}
	i := index
	lo := index - excursion
	if lo < 0 {
		lo = 0
	}
	for !isZeroCross(samples, i) && i > lo {
		i--
	}
	return i
}

func isZeroCross(samples []int, index int) bool {
	if index == 0 {
		return true
	}
	if index >= len(samples)-1 {
		return true
	}
	a := samples[index-1]
	b := samples[index]
	return (a > 0 && b < 0) || (a < 0 && b > 0) || (a == 0 && b != 0)
}
