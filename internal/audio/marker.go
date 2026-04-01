package audio

import "sort"

// Marker represents a slice point location in sample frames.
type Marker struct {
	Location int
}

// LocationRange represents a range [From, To) in sample frames.
type LocationRange struct {
	From int
	To   int
}

// MidLocation returns the midpoint of the range.
func (r LocationRange) MidLocation() int {
	return (r.From + r.To) / 2
}

// Markers manages a collection of slice markers.
type Markers struct {
	markers      []Marker
	selected     int
	maxLocation  int
	samplingRate int
}

// NewMarkers creates an empty Markers collection.
func NewMarkers() *Markers {
	return &Markers{}
}

// Clear resets the markers collection with new audio parameters.
func (m *Markers) Clear(maxLocation, samplingRate int) {
	m.markers = m.markers[:0]
	m.selected = 0
	m.maxLocation = maxLocation
	m.samplingRate = samplingRate
}

// Add appends a marker at the given location and sorts.
func (m *Markers) Add(location int) {
	m.markers = append(m.markers, Marker{Location: location})
	m.sortMarkers()
}

// Size returns the number of markers.
func (m *Markers) Size() int {
	return len(m.markers)
}

// Get returns the marker at the given index.
func (m *Markers) Get(index int) *Marker {
	if index < 0 || index >= len(m.markers) {
		return nil
	}
	return &m.markers[index]
}

// GetLocation returns the location of the marker at index, or 0 / maxLocation for out-of-range.
func (m *Markers) GetLocation(index int) int {
	if index < 0 {
		return 0
	}
	if index >= len(m.markers) {
		return m.maxLocation
	}
	return m.markers[index].Location
}

// GetRangeFrom returns the range from marker at index to the next marker.
func (m *Markers) GetRangeFrom(index int) LocationRange {
	return LocationRange{
		From: m.GetLocation(index),
		To:   m.GetLocation(index + 1),
	}
}

// SelectedIndex returns the currently selected marker index.
func (m *Markers) SelectedIndex() int {
	return m.selected
}

// SelectedMarker returns the currently selected marker.
func (m *Markers) SelectedMarker() *Marker {
	return m.Get(m.selected)
}

// SelectMarker shifts the selection by the given amount.
func (m *Markers) SelectMarker(shift int) {
	if len(m.markers) == 0 {
		return
	}
	sel := m.selected + shift
	if sel < 0 {
		sel = len(m.markers) - 1
	}
	m.selected = sel % len(m.markers)
}

// NudgeMarker moves the selected marker by the given number of samples.
func (m *Markers) NudgeMarker(ticks int) {
	marker := m.SelectedMarker()
	if marker == nil {
		return
	}
	newLoc := marker.Location + ticks
	if newLoc < 0 {
		newLoc = 0
	}
	marker.Location = newLoc
}

// DeleteSelected removes the currently selected marker.
func (m *Markers) DeleteSelected() {
	if len(m.markers) == 0 || m.selected >= len(m.markers) {
		return
	}
	m.markers = append(m.markers[:m.selected], m.markers[m.selected+1:]...)
	if m.selected > 0 {
		m.selected--
	}
}

// InsertAtMidpoint inserts a new marker at the midpoint of the selected marker's range.
func (m *Markers) InsertAtMidpoint() {
	r := m.GetRangeFrom(m.selected)
	mid := r.MidLocation()
	m.Add(mid)
}

// Duration returns the total duration in seconds.
func (m *Markers) Duration() float64 {
	if m.samplingRate == 0 {
		return 0
	}
	return float64(m.maxLocation) / float64(m.samplingRate)
}

// Tempo estimates the BPM based on the number of beats and duration.
// Returns -1 if the tempo is outside the valid range (40-250 BPM).
func (m *Markers) Tempo(numberOfBeats int) float64 {
	duration := m.Duration()
	if duration == 0 {
		return -1
	}
	tempo := 60.0 * float64(numberOfBeats) / duration
	if tempo > 250 || tempo < 40 {
		return -1
	}
	return tempo
}

// Locations returns a copy of all marker locations.
func (m *Markers) Locations() []int {
	locs := make([]int, len(m.markers))
	for i, mk := range m.markers {
		locs[i] = mk.Location
	}
	return locs
}

func (m *Markers) sortMarkers() {
	sort.Slice(m.markers, func(i, j int) bool {
		return m.markers[i].Location < m.markers[j].Location
	})
}
