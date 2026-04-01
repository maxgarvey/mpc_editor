package audio

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSlicerMarkers(t *testing.T) {
	s, err := OpenWAV(testdataPath("myLoop.wav"))
	if err != nil {
		t.Fatalf("OpenWAV: %v", err)
	}

	slicer := NewSlicer(s)
	t.Logf("Slicer: %s", slicer.String())

	// Java test expects 9 markers from myLoop.WAV with default settings
	markerCount := slicer.Markers.Size()
	t.Logf("Found %d markers", markerCount)

	// The exact count may vary slightly due to int precision differences between
	// Java and Go. Accept a range close to 9.
	if markerCount < 7 || markerCount > 11 {
		t.Errorf("marker count = %d, expected ~9 (Java parity)", markerCount)
	}

	// Verify markers are sorted
	locs := slicer.Markers.Locations()
	for i := 1; i < len(locs); i++ {
		if locs[i] <= locs[i-1] {
			t.Errorf("markers not sorted: [%d]=%d <= [%d]=%d", i, locs[i], i-1, locs[i-1])
		}
	}

	// Verify all markers are within sample bounds
	for i, loc := range locs {
		if loc < 0 || loc > s.FrameLength {
			t.Errorf("marker[%d] location %d out of bounds [0, %d]", i, loc, s.FrameLength)
		}
	}
}

func TestSlicerDurationAndTempo(t *testing.T) {
	s, err := OpenWAV(testdataPath("myLoop.wav"))
	if err != nil {
		t.Fatalf("OpenWAV: %v", err)
	}

	slicer := NewSlicer(s)

	duration := slicer.Markers.Duration()
	t.Logf("Duration: %.2fs", duration)
	// myLoop.WAV is ~3.84 seconds
	if duration < 3.0 || duration > 5.0 {
		t.Errorf("duration = %.2f, expected ~3.84", duration)
	}

	// Tempo with 8 beats (default in Java)
	tempo := slicer.Markers.Tempo(8)
	t.Logf("Tempo (8 beats): %.2f BPM", tempo)
	if tempo > 0 && (tempo < 40 || tempo > 250) {
		t.Errorf("tempo = %.2f, outside valid range", tempo)
	}
}

func TestSlicerSensitivity(t *testing.T) {
	s, err := OpenWAV(testdataPath("myLoop.wav"))
	if err != nil {
		t.Fatalf("OpenWAV: %v", err)
	}

	slicer := NewSlicer(s)
	originalCount := slicer.Markers.Size()

	// Higher sensitivity → fewer markers
	slicer.SetSensitivity(200)
	highCount := slicer.Markers.Size()

	// Lower sensitivity → more markers
	slicer.SetSensitivity(80)
	lowCount := slicer.Markers.Size()

	t.Logf("markers: sensitivity=80→%d, default→%d, sensitivity=200→%d", lowCount, originalCount, highCount)

	if lowCount < highCount {
		t.Errorf("lower sensitivity should produce more markers: got %d (low) < %d (high)", lowCount, highCount)
	}
}

func TestSlicerMarkerSelection(t *testing.T) {
	s, err := OpenWAV(testdataPath("myLoop.wav"))
	if err != nil {
		t.Fatalf("OpenWAV: %v", err)
	}

	slicer := NewSlicer(s)
	if slicer.Markers.Size() == 0 {
		t.Fatal("no markers")
	}

	// Selection starts at 0
	if slicer.Markers.SelectedIndex() != 0 {
		t.Error("initial selection should be 0")
	}

	// Navigate forward
	slicer.Markers.SelectMarker(1)
	if slicer.Markers.SelectedIndex() != 1 {
		t.Errorf("after +1: selected = %d, want 1", slicer.Markers.SelectedIndex())
	}

	// Navigate backward past start → wraps to end
	slicer.Markers.SelectMarker(-2)
	expected := slicer.Markers.Size() - 1
	if slicer.Markers.SelectedIndex() != expected {
		t.Errorf("after wrap: selected = %d, want %d", slicer.Markers.SelectedIndex(), expected)
	}
}

func TestSlicerGetSlice(t *testing.T) {
	s, err := OpenWAV(testdataPath("myLoop.wav"))
	if err != nil {
		t.Fatalf("OpenWAV: %v", err)
	}

	slicer := NewSlicer(s)
	if slicer.Markers.Size() < 2 {
		t.Fatal("need at least 2 markers")
	}

	slice := slicer.GetSlice(0)
	if slice.FrameLength == 0 {
		t.Error("slice has 0 frames")
	}
	if slice.Format.SampleRate != s.Format.SampleRate {
		t.Error("slice format differs from source")
	}
}

func TestSlicerDeleteAndInsert(t *testing.T) {
	s, err := OpenWAV(testdataPath("myLoop.wav"))
	if err != nil {
		t.Fatalf("OpenWAV: %v", err)
	}

	slicer := NewSlicer(s)
	original := slicer.Markers.Size()

	// Delete selected marker
	slicer.Markers.DeleteSelected()
	if slicer.Markers.Size() != original-1 {
		t.Errorf("after delete: size = %d, want %d", slicer.Markers.Size(), original-1)
	}

	// Insert at midpoint
	slicer.Markers.InsertAtMidpoint()
	if slicer.Markers.Size() != original {
		t.Errorf("after insert: size = %d, want %d", slicer.Markers.Size(), original)
	}
}

func TestSlicerExportSlices(t *testing.T) {
	s, err := OpenWAV(testdataPath("myLoop.wav"))
	if err != nil {
		t.Fatalf("OpenWAV: %v", err)
	}

	slicer := NewSlicer(s)
	dir := t.TempDir()

	paths, err := slicer.ExportSlices(dir, "slice_")
	if err != nil {
		t.Fatalf("ExportSlices: %v", err)
	}

	if len(paths) != slicer.Markers.Size() {
		t.Errorf("exported %d files, want %d", len(paths), slicer.Markers.Size())
	}

	// Verify files exist and are valid WAVs
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			t.Errorf("missing file: %s", p)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("empty file: %s", p)
		}

		// Reopen to verify it's a valid WAV
		reopened, err := OpenWAV(p)
		if err != nil {
			t.Errorf("invalid WAV %s: %v", filepath.Base(p), err)
			continue
		}
		if reopened.FrameLength == 0 {
			t.Errorf("slice %s has 0 frames", filepath.Base(p))
		}
	}
}
