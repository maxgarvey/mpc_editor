package audio

import "testing"

func TestMarkersBasic(t *testing.T) {
	m := NewMarkers()
	m.Clear(100000, 44100)

	m.Add(1000)
	m.Add(5000)
	m.Add(3000)

	if m.Size() != 3 {
		t.Errorf("Size = %d, want 3", m.Size())
	}

	// Should be sorted
	locs := m.Locations()
	if locs[0] != 1000 || locs[1] != 3000 || locs[2] != 5000 {
		t.Errorf("locations = %v, want [1000, 3000, 5000]", locs)
	}
}

func TestMarkersGetRange(t *testing.T) {
	m := NewMarkers()
	m.Clear(100000, 44100)

	m.Add(1000)
	m.Add(5000)
	m.Add(10000)

	r := m.GetRangeFrom(0)
	if r.From != 1000 || r.To != 5000 {
		t.Errorf("range[0] = %+v, want {1000, 5000}", r)
	}

	r = m.GetRangeFrom(2)
	if r.From != 10000 || r.To != 100000 {
		t.Errorf("range[2] = %+v, want {10000, 100000} (to maxLocation)", r)
	}
}

func TestMarkersDuration(t *testing.T) {
	m := NewMarkers()
	m.Clear(44100, 44100) // 1 second
	if d := m.Duration(); d < 0.99 || d > 1.01 {
		t.Errorf("Duration = %f, want ~1.0", d)
	}
}

func TestMarkersTempo(t *testing.T) {
	m := NewMarkers()
	// 4 bars at 120 BPM = 8 beats in 4 seconds → 44100*4 samples
	m.Clear(44100*4, 44100)

	tempo := m.Tempo(8)
	if tempo < 119 || tempo > 121 {
		t.Errorf("Tempo(8) = %f, want ~120", tempo)
	}

	// Out of range
	tempo = m.Tempo(1)
	if tempo != -1 {
		t.Errorf("Tempo(1) = %f, want -1 (out of range)", tempo)
	}
}

func TestMarkersNudge(t *testing.T) {
	m := NewMarkers()
	m.Clear(100000, 44100)
	m.Add(5000)

	m.NudgeMarker(100)
	if m.Get(0).Location != 5100 {
		t.Errorf("after nudge +100: location = %d, want 5100", m.Get(0).Location)
	}

	m.NudgeMarker(-200)
	if m.Get(0).Location != 4900 {
		t.Errorf("after nudge -200: location = %d, want 4900", m.Get(0).Location)
	}
}

func TestMarkersDeleteAndInsert(t *testing.T) {
	m := NewMarkers()
	m.Clear(100000, 44100)
	m.Add(1000)
	m.Add(5000)
	m.Add(10000)

	// Select marker 1 (5000)
	m.SelectMarker(1)
	if m.SelectedIndex() != 1 {
		t.Fatal("selection should be 1")
	}

	m.DeleteSelected()
	if m.Size() != 2 {
		t.Errorf("after delete: size = %d, want 2", m.Size())
	}
	// Remaining should be [1000, 10000]
	locs := m.Locations()
	if locs[0] != 1000 || locs[1] != 10000 {
		t.Errorf("after delete: locations = %v, want [1000, 10000]", locs)
	}

	// Insert at midpoint of selected range
	m.InsertAtMidpoint()
	if m.Size() != 3 {
		t.Errorf("after insert: size = %d, want 3", m.Size())
	}
}

func TestLocationRange(t *testing.T) {
	r := LocationRange{From: 1000, To: 5000}
	if mid := r.MidLocation(); mid != 3000 {
		t.Errorf("MidLocation = %d, want 3000", mid)
	}
}
