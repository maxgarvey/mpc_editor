package seq

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateParseRoundTrip(t *testing.T) {
	// Deliberately unsorted: Create must sort by tick before encoding.
	events := []Event{
		{Tick: 384, Track: 0, Type: EventNoteOn, Note: 38, Velocity: 100, Duration: 24},
		{Tick: 0, Track: 0, Type: EventNoteOn, Note: 36, Velocity: 127, Duration: 24},
		{Tick: 96, Track: 0, Type: EventNoteOn, Note: 42, Velocity: 64, Duration: 12},
	}
	data := Create(120.5, 2, "MyTrack", "MyKit", true, events)

	path := filepath.Join(t.TempDir(), "created.SEQ")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open created file: %v", err)
	}

	if s.BPM != 120.5 {
		t.Errorf("BPM = %v, want 120.5", s.BPM)
	}
	if s.Bars != 2 {
		t.Errorf("Bars = %d, want 2", s.Bars)
	}
	if !s.Loop {
		t.Error("Loop should be true")
	}
	if s.Tracks[0].Name != "MyTrack" {
		t.Errorf("track 0 name = %q, want MyTrack", s.Tracks[0].Name)
	}
	if s.Tracks[1].Name != "Track02" {
		t.Errorf("track 1 name = %q, want Track02", s.Tracks[1].Name)
	}
	if s.Tracks[0].MIDIChannel == 0 {
		t.Error("track 0 should be active (non-zero MIDI channel)")
	}

	if len(s.Events) != 3 {
		t.Fatalf("len(Events) = %d, want 3", len(s.Events))
	}
	wantTicks := []uint32{0, 96, 384}
	wantNotes := []byte{36, 42, 38}
	for i, ev := range s.Events {
		if ev.Tick != wantTicks[i] {
			t.Errorf("event %d tick = %d, want %d (sorted)", i, ev.Tick, wantTicks[i])
		}
		if ev.Note != wantNotes[i] {
			t.Errorf("event %d note = %d, want %d", i, ev.Note, wantNotes[i])
		}
		if ev.Type != EventNoteOn {
			t.Errorf("event %d type = %#x, want NoteOn", i, ev.Type)
		}
	}
}

func TestCreate_NoLoopNoNames(t *testing.T) {
	data := Create(90, 1, "", "", false, nil)
	path := filepath.Join(t.TempDir(), "empty.SEQ")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if s.Loop {
		t.Error("Loop should be false")
	}
	if s.Tracks[0].Name != "Track01" {
		t.Errorf("track 0 name = %q, want default Track01", s.Tracks[0].Name)
	}
	if len(s.Events) != 0 {
		t.Errorf("len(Events) = %d, want 0", len(s.Events))
	}
}

func TestPatchLoop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "patch.SEQ")
	if err := os.WriteFile(path, Create(120, 1, "", "", false, nil), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := PatchLoop(path, true); err != nil {
		t.Fatalf("PatchLoop(true): %v", err)
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if !s.Loop {
		t.Error("Loop should be true after PatchLoop(true)")
	}

	if err := PatchLoop(path, false); err != nil {
		t.Fatalf("PatchLoop(false): %v", err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.Loop {
		t.Error("Loop should be false after PatchLoop(false)")
	}
}

func TestPatchFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "patch.SEQ")
	if err := os.WriteFile(path, Create(120, 1, "", "", false, nil), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := PatchFile(path, 87.3, 4); err != nil {
		t.Fatalf("PatchFile: %v", err)
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.BPM != 87.3 {
		t.Errorf("BPM = %v, want 87.3", s.BPM)
	}
	if s.Bars != 4 {
		t.Errorf("Bars = %d, want 4", s.Bars)
	}
}

func TestPatchErrors(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.SEQ")
	if err := PatchLoop(missing, true); err == nil {
		t.Error("PatchLoop on missing file should error")
	}
	if err := PatchFile(missing, 120, 1); err == nil {
		t.Error("PatchFile on missing file should error")
	}

	tiny := filepath.Join(t.TempDir(), "tiny.SEQ")
	if err := os.WriteFile(tiny, []byte("too small"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := PatchLoop(tiny, true); err == nil {
		t.Error("PatchLoop on truncated file should error")
	}
	if err := PatchFile(tiny, 120, 1); err == nil {
		t.Error("PatchFile on truncated file should error")
	}
}
