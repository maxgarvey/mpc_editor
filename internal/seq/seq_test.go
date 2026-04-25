package seq

import (
	"os"
	"testing"
)

// buildTestSEQ creates a minimal .SEQ binary for testing.
// The layout matches the real MPC 1000 format: fixed 0x1C10-byte prefix,
// then N × 16-byte events, then a 16-byte terminator.
func buildTestSEQ(bpm float64, bars int, events []Event) []byte {
	size := eventDataOffset + len(events)*eventSize + eventSize // +eventSize for terminator
	data := make([]byte, size)

	// Version string
	copy(data[versionOffset:], "MPC1000 SEQ 4.40")

	// Bars (little-endian uint16)
	barsVal := uint16(bars)
	data[barsOffset] = byte(barsVal)
	data[barsOffset+1] = byte(barsVal >> 8)

	// BPM x 10 (little-endian uint16)
	bpmVal := uint16(bpm * 10)
	data[bpmOffset] = byte(bpmVal)
	data[bpmOffset+1] = byte(bpmVal >> 8)

	// Track 0 (at trackDataOffset): name "Drums", MIDI channel 10.
	trackOff := trackDataOffset
	copy(data[trackOff:], "Drums\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00")
	data[trackOff+trackMIDIChanOff] = 10

	// Encode events
	off := eventDataOffset
	for _, ev := range events {
		b := encodeEvent(ev)
		copy(data[off:off+eventSize], b[:])
		off += eventSize
	}

	// 16-byte terminator: ff ff ff 7f ff ff ff ff ff ff ff ff ff ff ff ff
	copy(data[off:], []byte{
		0xFF, 0xFF, 0xFF, 0x7F, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	})

	return data
}

func TestParseHeader(t *testing.T) {
	data := buildTestSEQ(120.0, 4, nil)
	s, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}

	if s.Version != "MPC1000 SEQ 4.40" {
		t.Errorf("version = %q, want %q", s.Version, "MPC1000 SEQ 4.40")
	}
	if s.BPM != 120.0 {
		t.Errorf("bpm = %f, want 120.0", s.BPM)
	}
	if s.Bars != 4 {
		t.Errorf("bars = %d, want 4", s.Bars)
	}
}

func TestParseTrack(t *testing.T) {
	data := buildTestSEQ(120.0, 2, nil)
	s, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}

	if s.Tracks[0].Name != "Drums" {
		t.Errorf("track 0 name = %q, want %q", s.Tracks[0].Name, "Drums")
	}
	if s.Tracks[0].MIDIChannel != 10 {
		t.Errorf("track 0 midi channel = %d, want 10", s.Tracks[0].MIDIChannel)
	}
}

func TestParseEvents(t *testing.T) {
	events := []Event{
		{Tick: 0, Track: 0, Note: 36, Velocity: 100, Duration: 24},   // kick on step 0
		{Tick: 48, Track: 0, Note: 38, Velocity: 90, Duration: 24},   // snare on step 2
		{Tick: 96, Track: 0, Note: 42, Velocity: 80, Duration: 12},   // hihat on step 4
		{Tick: 384, Track: 0, Note: 36, Velocity: 100, Duration: 24}, // kick on bar 2, step 0
	}

	data := buildTestSEQ(120.0, 2, events)
	s, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}

	if len(s.Events) != 4 {
		t.Fatalf("got %d events, want 4", len(s.Events))
	}

	// Verify first event
	ev := s.Events[0]
	if ev.Tick != 0 {
		t.Errorf("event 0 tick = %d, want 0", ev.Tick)
	}
	if ev.Note != 36 {
		t.Errorf("event 0 note = %d, want 36", ev.Note)
	}
	if ev.Velocity != 100 {
		t.Errorf("event 0 velocity = %d, want 100", ev.Velocity)
	}

	// Verify tick on step 2 (tick 48)
	if s.Events[1].Tick != 48 {
		t.Errorf("event 1 tick = %d, want 48", s.Events[1].Tick)
	}
}

func TestBuildGrid(t *testing.T) {
	events := []Event{
		{Tick: 0, Track: 0, Note: 36, Velocity: 100, Duration: 24},
		{Tick: 24, Track: 0, Note: 42, Velocity: 70, Duration: 12},
		{Tick: 48, Track: 0, Note: 38, Velocity: 90, Duration: 24},
		{Tick: 96, Track: 0, Note: 36, Velocity: 100, Duration: 24},
	}

	data := buildTestSEQ(120.0, 2, events)
	s, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}

	grid := BuildGrid(s, 1, nil)

	if grid.Bar != 1 {
		t.Errorf("grid bar = %d, want 1", grid.Bar)
	}
	if grid.TotalBars != 2 {
		t.Errorf("grid total bars = %d, want 2", grid.TotalBars)
	}
	if len(grid.Rows) != 1 {
		t.Fatalf("grid rows = %d, want 1 (only track 0 has events)", len(grid.Rows))
	}

	row := grid.Rows[0]
	if row.TrackName != "Drums" {
		t.Errorf("row track name = %q, want %q", row.TrackName, "Drums")
	}

	// Step 0 should be active (tick 0)
	if !row.Steps[0].Active {
		t.Error("step 0 should be active")
	}
	if row.Steps[0].Note != 36 {
		t.Errorf("step 0 note = %d, want 36", row.Steps[0].Note)
	}

	// Step 1 should be active (tick 24)
	if !row.Steps[1].Active {
		t.Error("step 1 should be active")
	}

	// Step 2 should be active (tick 48)
	if !row.Steps[2].Active {
		t.Error("step 2 should be active")
	}

	// Step 3 should be inactive
	if row.Steps[3].Active {
		t.Error("step 3 should be inactive")
	}

	// Step 4 should be active (tick 96)
	if !row.Steps[4].Active {
		t.Error("step 4 should be active")
	}
}

func TestBuildGridBar2(t *testing.T) {
	events := []Event{
		{Tick: 0, Track: 0, Note: 36, Velocity: 100, Duration: 24},
		{Tick: 384, Track: 0, Note: 38, Velocity: 90, Duration: 24}, // bar 2, step 0
	}

	data := buildTestSEQ(120.0, 2, events)
	s, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}

	grid := BuildGrid(s, 2, nil)
	if len(grid.Rows) != 1 {
		t.Fatalf("grid rows = %d, want 1", len(grid.Rows))
	}

	// Only step 0 of bar 2 should be active
	if !grid.Rows[0].Steps[0].Active {
		t.Error("bar 2 step 0 should be active")
	}
	if grid.Rows[0].Steps[0].Note != 38 {
		t.Errorf("bar 2 step 0 note = %d, want 38", grid.Rows[0].Steps[0].Note)
	}
	if grid.Rows[0].Steps[1].Active {
		t.Error("bar 2 step 1 should be inactive")
	}
}

func TestNoteName(t *testing.T) {
	tests := []struct {
		note byte
		want string
	}{
		{60, "C4"},
		{61, "C#4"},
		{69, "A4"},
		{36, "C2"},
		{127, "G9"},
		{0, "C-1"},
	}

	for _, tt := range tests {
		got := NoteName(tt.note)
		if got != tt.want {
			t.Errorf("NoteName(%d) = %q, want %q", tt.note, got, tt.want)
		}
	}
}

func TestParseFileTooSmall(t *testing.T) {
	data := make([]byte, 100)
	_, err := Parse(data)
	if err == nil {
		t.Error("expected error for too-small file")
	}
}

func TestWriteEventsRoundTrip(t *testing.T) {
	events := []Event{
		{Tick: 0, Track: 0, Note: 36, Velocity: 100, Duration: 23},
		{Tick: 48, Track: 0, Note: 38, Velocity: 90, Duration: 23},
		{Tick: 96, Track: 0, Note: 42, Velocity: 80, Duration: 12},
	}
	data := buildTestSEQ(120.0, 2, events)
	s, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}

	// Write to a temp file and re-parse.
	tmp := t.TempDir() + "/roundtrip.SEQ"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteEvents(tmp, s); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(tmp)
	if err != nil {
		t.Fatal(err)
	}

	if len(s2.Events) != len(events) {
		t.Fatalf("got %d events after round-trip, want %d", len(s2.Events), len(events))
	}
	for i, ev := range s2.Events {
		orig := events[i]
		if ev.Tick != orig.Tick {
			t.Errorf("event %d: tick %d want %d", i, ev.Tick, orig.Tick)
		}
		if ev.Note != orig.Note {
			t.Errorf("event %d: note %d want %d", i, ev.Note, orig.Note)
		}
		if ev.Velocity != orig.Velocity {
			t.Errorf("event %d: velocity %d want %d", i, ev.Velocity, orig.Velocity)
		}
		if ev.Duration != orig.Duration {
			t.Errorf("event %d: duration %d want %d", i, ev.Duration, orig.Duration)
		}
	}
}
