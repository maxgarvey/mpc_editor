package seq

import (
	"encoding/binary"
	"testing"
)

// buildTestSEQ creates a minimal .SEQ binary for testing.
func buildTestSEQ(bpm float64, bars int, events []Event) []byte {
	// Allocate enough space for header + tracks + events + terminator.
	size := eventDataOffset + len(events)*eventSize + eventSize // +8 for terminator
	data := make([]byte, size)

	// Version string
	copy(data[versionOffset:], "MPC1000 SEQ 4.40")

	// Bars
	binary.LittleEndian.PutUint16(data[barsOffset:], uint16(bars))

	// BPM x 10
	binary.LittleEndian.PutUint16(data[bpmOffset:], uint16(bpm*10))

	// Track names: put a name on track 0
	trackOff := trackDataOffset
	copy(data[trackOff:], "Drums\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00")
	data[trackOff+16] = 10 // MIDI channel 10
	data[trackOff+17] = 0  // program 0

	// Encode events
	off := eventDataOffset
	for _, ev := range events {
		encodeTestEvent(data[off:off+eventSize], ev)
		off += eventSize
	}

	// Terminator: ff ff ff 7f ff ff ff ff (byte 3 = 0x7F, not 0xFF)
	copy(data[off:], []byte{0xFF, 0xFF, 0xFF, 0x7F, 0xFF, 0xFF, 0xFF, 0xFF})

	return data
}

// encodeTestEvent writes an event into 8 bytes using the bit-packed format.
func encodeTestEvent(b []byte, ev Event) {
	// Tick: low 16 bits in bytes 0-1, overflow in byte 2 low nibble
	binary.LittleEndian.PutUint16(b[0:2], uint16(ev.Tick&0xFFFF))
	b[2] = byte((ev.Tick >> 16) & 0x0F)

	// Duration scattered: we need to encode it back
	// dur = ((byte2&0xF0)<<6) + ((byte3&0xC0)<<2) + byte5 - track*4
	// So: dur + track*4 = ((byte2&0xF0)<<6) + ((byte3&0xC0)<<2) + byte5
	adjDur := ev.Duration + uint16(ev.Track*4)
	durByte5 := byte(adjDur & 0xFF)
	adjDur >>= 8
	durByte3Bits := byte((adjDur & 0x03) << 6)
	adjDur >>= 2
	durByte2Bits := byte((adjDur & 0x0F) << 4)

	b[2] |= durByte2Bits // merge with tick overflow in low nibble

	// Track in byte 3 bits 0-5, duration bits in 6-7
	b[3] = byte(ev.Track&0x3F) | durByte3Bits

	// Byte 4: MIDI channel (1 for NoteOn; ≥0x80 for CC/PC/etc.)
	b[4] = 0x01

	// Duration low byte
	b[5] = durByte5

	// Note at byte 6 bits 0-6
	b[6] = ev.Note & 0x7F

	// Velocity at byte 7 bits 0-6
	b[7] = ev.Velocity & 0x7F
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
