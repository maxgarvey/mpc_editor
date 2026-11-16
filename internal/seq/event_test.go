package seq

import (
	"encoding/hex"
	"strings"
	"testing"
)

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(strings.ReplaceAll(s, " ", ""))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Records modelled on Akai's factory sequences and hardware-recorded files.
var eventFixtures = []struct {
	name string
	hex  string
	want Event
}{
	{"recorded note, pad index in byte 13", "c0 00 00 00 01 90 28 7f 0d 00 00 00 00 04 00 00",
		Event{Tick: 192, Track: 0, Type: EventNoteOn, Status: 0x90, Note: 40, Velocity: 127, Duration: 13}},
	{"tune variation +90", "30 1b 00 00 06 90 33 11 09 00 00 5a 00 00 00 00",
		Event{Tick: 0x1b30, Track: 5, Type: EventNoteOn, Status: 0x90, Note: 51, Velocity: 17, Duration: 9, VariationValue: 90}},
	{"tune variation -20 (0xEC)", "00 00 00 00 0d 90 5b 64 2e 00 00 ec 00 00 00 00",
		Event{Tick: 0, Track: 12, Type: EventNoteOn, Status: 0x90, Note: 91, Velocity: 100, Duration: 46, VariationValue: -20}},
	{"variation type 1", "00 06 00 00 03 90 2a 7f 11 00 01 0c 00 00 00 00",
		Event{Tick: 0x600, Track: 2, Type: EventNoteOn, Status: 0x90, Note: 42, Velocity: 127, Duration: 17, VariationType: 1, VariationValue: 12}},
	{"ff ff ff ff tail", "00 00 00 00 05 90 41 7f 6b 01 00 00 ff ff ff ff",
		Event{Tick: 0, Track: 4, Type: EventNoteOn, Status: 0x90, Note: 65, Velocity: 127, Duration: 363}},
	{"program change", "00 00 00 00 07 c0 67 00 00 00 00 00 00 00 00 00",
		Event{Tick: 0, Track: 6, Type: EventProgramChange, Status: 0xC0, Note: 103}},
	{"unknown 0x82", "fc 05 00 00 01 82 02 00 00 00 00 00 00 00 00 00",
		Event{Tick: 1532, Track: 0, Type: 0x80, Status: 0x82, Note: 2}},
}

func TestParseEventFields(t *testing.T) {
	for _, f := range eventFixtures {
		got := parseEvent(mustHex(t, f.hex))
		got.raw, got.hasRaw = [eventSize]byte{}, false
		if got != f.want {
			t.Errorf("%s:\n got  %+v\n want %+v", f.name, got, f.want)
		}
	}
}

// Duration is 16-bit: bytes 10-11 belong to Note Variation, so a non-zero
// variation must not inflate the duration (it did when bytes 8-11 were read
// as a uint32).
func TestParseEventDurationIgnoresVariation(t *testing.T) {
	ev := parseEvent(mustHex(t, "00 00 00 00 01 90 24 7f 2e 00 00 ec 00 00 00 00"))
	if ev.Duration != 46 {
		t.Fatalf("duration %d, want 46", ev.Duration)
	}
}

func TestEncodeEventUnchangedIsIdentical(t *testing.T) {
	for _, f := range eventFixtures {
		in := mustHex(t, f.hex)
		out := encodeEvent(parseEvent(in))
		if string(out[:]) != string(in) {
			t.Errorf("%s: re-encode changed bytes\n in  % x\n out % x", f.name, in, out[:])
		}
	}
}

func TestEncodeEventEditKeepsUnmodelledBytes(t *testing.T) {
	ev := parseEvent(mustHex(t, "00 00 00 00 05 90 41 7f 6b 01 02 f6 ff ff ff ff"))
	ev.Velocity = 64
	ev.Tick = 96
	out := encodeEvent(ev)
	want := mustHex(t, "60 00 00 00 05 90 41 40 6b 01 02 f6 ff ff ff ff")
	if string(out[:]) != string(want) {
		t.Fatalf("got  % x\nwant % x", out[:], want)
	}
}

func TestEncodeEventNoteChangeUpdatesPadIndex(t *testing.T) {
	ev := parseEvent(mustHex(t, "00 00 00 00 01 90 28 7f 0d 00 00 00 00 04 00 00"))
	ev.Note = 37
	out := encodeEvent(ev)
	if out[6] != 37 || out[13] != 1 {
		t.Fatalf("note/pad = %d/%d, want 37/1", out[6], out[13])
	}
}

func TestEncodeEventNew(t *testing.T) {
	cases := []struct {
		ev   Event
		want string
	}{
		{Event{Tick: 384, Track: 1, Type: EventNoteOn, Note: 40, Velocity: 100, Duration: 65535, VariationValue: -1},
			"80 01 00 00 02 90 28 64 ff ff 00 ff 00 04 00 00"},
		{Event{Tick: 0, Note: 36, Velocity: 1, Duration: 1}, // zero Type = note
			"00 00 00 00 01 90 24 01 01 00 00 00 00 00 00 00"},
		{Event{Track: 2, Type: EventProgramChange, Note: 5},
			"00 00 00 00 03 c0 05 00 00 00 00 00 00 00 00 00"},
	}
	for i, c := range cases {
		out := encodeEvent(c.ev)
		if want := mustHex(t, c.want); string(out[:]) != string(want) {
			t.Errorf("case %d: got % x\n         want % x", i, out[:], want)
		}
	}
}

func TestIsNote(t *testing.T) {
	if !(Event{}).IsNote() || !(Event{Type: EventNoteOn}).IsNote() {
		t.Error("zero/NoteOn should be notes")
	}
	if (Event{Type: EventProgramChange}).IsNote() || (Event{Type: 0x80}).IsNote() {
		t.Error("program change / 0x82 should not be notes")
	}
}
