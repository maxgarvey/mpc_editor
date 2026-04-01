package mpc_editor_test

import (
	"path/filepath"
	"testing"

	"github.com/maxgarvey/mpc_editor/internal/audio"
	"github.com/maxgarvey/mpc_editor/internal/midi"
)

func TestSlicerToMIDI_JavaParity(t *testing.T) {
	s, err := audio.OpenWAV(filepath.Join("testdata", "myLoop.wav"))
	if err != nil {
		t.Fatalf("OpenWAV: %v", err)
	}

	slicer := audio.NewSlicer(s)
	markers := slicer.Markers

	if markers.Size() != 9 {
		t.Fatalf("marker count = %d, want 9", markers.Size())
	}

	// Java test flow: select marker 4 (location 79872), then delete it.
	// Verify marker 4 location matches Java.
	markers.SelectMarker(4)
	if markers.SelectedIndex() != 4 {
		t.Fatalf("selected = %d, want 4", markers.SelectedIndex())
	}
	if loc := markers.Get(4).Location; loc != 79872 {
		t.Errorf("marker[4] location = %d, want 79872", loc)
	}

	// Delete marker 4 (same as Java test)
	markers.DeleteSelected()
	if markers.Size() != 8 {
		t.Fatalf("after delete: size = %d, want 8", markers.Size())
	}

	tempo := markers.Tempo(8)
	if tempo < 0 {
		t.Fatal("tempo out of range")
	}
	t.Logf("Tempo: %.2f BPM", tempo)

	// Export MIDI from the 8 remaining markers
	locations := markers.Locations()
	seq := midi.BuildFromMarkers(locations, tempo, s.Format.SampleRate, 96)

	// 8 notes → 16 events
	if len(seq.Events) != 16 {
		t.Fatalf("events = %d, want 16", len(seq.Events))
	}

	// Java expected ticks (8 note pairs + 1 end-of-track = 17 in Java, 16 events for us):
	// {0, 32, 97, 129, 190, 222, 288, 320, 381, 413, 478, 510, 575, 607, 673, 705}
	// Note: Java's 17th entry (705) is the end-of-track meta event.
	expectedTicks := []int{0, 32, 97, 129, 190, 222, 288, 320, 381, 413, 478, 510, 575, 607, 673, 705}

	// Collect all event ticks sorted (NOTE_OFF before NOTE_ON at same tick)
	type tickEvent struct {
		tick   int
		status byte
	}
	var events []tickEvent
	for _, ev := range seq.Events {
		events = append(events, tickEvent{ev.Tick, ev.Status})
	}
	// Sort: by tick, then NOTE_OFF (0x80) before NOTE_ON (0x90)
	for i := 0; i < len(events)-1; i++ {
		for j := i + 1; j < len(events); j++ {
			if events[j].tick < events[i].tick ||
				(events[j].tick == events[i].tick && events[j].status < events[i].status) {
				events[i], events[j] = events[j], events[i]
			}
		}
	}

	for i, exp := range expectedTicks {
		if i >= len(events) {
			t.Errorf("missing event[%d], want tick %d", i, exp)
			continue
		}
		if events[i].tick != exp {
			t.Errorf("event[%d] tick = %d, want %d", i, events[i].tick, exp)
		}
	}

	// Save and re-read round-trip
	path := filepath.Join(t.TempDir(), "sliced.mid")
	if err := seq.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	parsed, err := midi.ReadMIDI(path)
	if err != nil {
		t.Fatalf("ReadMIDI: %v", err)
	}
	if len(parsed.Events) != 16 {
		t.Errorf("re-read events = %d, want 16", len(parsed.Events))
	}

	// Verify Java's insertMarker behavior: after delete, selected=3.
	// Insert at midpoint of range from marker 3 → should produce location 73727.
	if markers.SelectedIndex() != 3 {
		t.Errorf("after delete: selected = %d, want 3", markers.SelectedIndex())
	}
	markers.InsertAtMidpoint()
	if markers.Size() != 9 {
		t.Errorf("after insert: size = %d, want 9", markers.Size())
	}
	// The midpoint should be at 73727 (Java assertion)
	if loc := markers.GetLocation(4); loc != 73727 {
		t.Errorf("inserted marker location = %d, want 73727", loc)
	}

	t.Log("Full Java parity verified: 9 markers, delete, MIDI export, insert")
}
