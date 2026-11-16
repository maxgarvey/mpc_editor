package seq

import (
	"bytes"
	"encoding/binary"
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

// factoryLikeSEQ builds a file whose events are given as raw 16-byte records,
// mirroring the record kinds found in Akai's factory sequences.
func factoryLikeSEQ(t *testing.T, records ...string) []byte {
	t.Helper()
	data := buildTestSEQ(97, 2, nil)
	data = data[:eventDataOffset]
	for _, r := range records {
		data = append(data, mustHex(t, r)...)
	}
	data = append(data, mustHex(t, "ff ff ff 7f ff ff ff ff ff ff ff ff ff ff ff ff")...)
	binary.LittleEndian.PutUint32(data[0:4], uint32(len(data)))
	return data
}

var factoryRecords = []string{
	"00 00 00 00 01 c0 00 00 00 00 00 00 00 00 00 00", // program change, track 1
	"00 00 00 00 01 82 01 00 00 00 00 00 00 00 00 00", // unknown 0x82
	"00 00 00 00 01 90 24 7f 0c 00 00 00 ff ff ff ff", // ff tail
	"00 00 00 00 07 c0 67 00 00 00 00 00 00 00 00 00", // program change after a note, same tick
	"30 00 00 00 06 90 33 11 09 00 00 5a 00 00 00 00", // tune variation +90
	"60 00 00 00 06 90 33 11 09 00 00 ec 00 00 00 00", // tune variation -20
	"60 00 00 00 03 90 2a 7f 11 00 01 0c 00 00 00 00", // variation type 1
	"c0 00 00 00 01 90 28 7f 0d 00 00 00 00 04 00 00", // recorded: pad index in byte 13
	"fc 05 00 00 01 82 02 00 00 00 00 00 00 00 00 00", // 0x82 mid-sequence
}

// Opening and saving an unmodified file must not change a single byte.
func TestWriteEventsFactoryRoundTripIsIdentical(t *testing.T) {
	data := factoryLikeSEQ(t, factoryRecords...)
	path := filepath.Join(t.TempDir(), "f.SEQ")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteEvents(path, s); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(path)
	if !bytes.Equal(out, data) {
		for i := range data {
			if i >= len(out) || out[i] != data[i] {
				t.Fatalf("first difference at 0x%X (event %d)", i, (i-eventDataOffset)/eventSize)
			}
		}
		t.Fatalf("length %d, want %d", len(out), len(data))
	}
}

// Deleting one note keeps every other record, in order, byte for byte.
func TestWriteEventsAfterEditKeepsOtherRecords(t *testing.T) {
	data := factoryLikeSEQ(t, factoryRecords...)
	path := filepath.Join(t.TempDir(), "f.SEQ")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	s, _ := Open(path)
	s.Events = append(s.Events[:7], s.Events[8:]...) // drop the recorded note
	if err := WriteEvents(path, s); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(path)
	want := append([]string{}, factoryRecords[:7]...)
	want = append(want, factoryRecords[8])
	for i, r := range want {
		off := eventDataOffset + i*eventSize
		if got := out[off : off+eventSize]; !bytes.Equal(got, mustHex(t, r)) {
			t.Errorf("record %d:\n got  % x\n want %s", i, got, r)
		}
	}
	if !isTerminator(out[eventDataOffset+len(want)*eventSize:]) {
		t.Error("terminator not after the last record")
	}
}

func TestWriteEventsDropsOnlyZeroVelocityNotes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "z.SEQ")
	if err := os.WriteFile(path, buildTestSEQ(120, 1, nil), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &Sequence{Events: []Event{
		{Tick: 0, Type: EventProgramChange, Note: 4},
		{Tick: 0, Type: EventNoteOn, Note: 36, Velocity: 0, Duration: 10},
		{Tick: 24, Type: EventNoteOn, Note: 36, Velocity: 90, Duration: 10},
	}}
	if err := WriteEvents(path, s); err != nil {
		t.Fatal(err)
	}
	got, _ := Open(path)
	if len(got.Events) != 2 || got.Events[0].Type != EventProgramChange || got.Events[1].Velocity != 90 {
		t.Fatalf("events = %+v", got.Events)
	}
}
