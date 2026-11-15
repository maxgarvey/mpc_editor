package pgm

import (
	"path/filepath"
	"testing"
)

func TestSetNoteMapEntryLeavesTablesInconsistent(t *testing.T) {
	p := NewProgram()
	if err := p.SetNoteMapEntry(36, 5); err != nil {
		t.Fatal(err)
	}
	if got := p.PadForNote(36); got != 5 {
		t.Errorf("PadForNote(36) = %d, want 5", got)
	}
	if got := p.Pad(1).GetMIDINote(); got != 36 {
		t.Errorf("pad 1 note = %d, want 36 (pad→note untouched)", got)
	}

	path := filepath.Join(t.TempDir(), "raw.pgm")
	if err := p.Save(path); err != nil {
		t.Fatal(err)
	}
	raw, err := OpenProgramRaw(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := raw.PadForNote(36); got != 5 {
		t.Errorf("raw PadForNote(36) = %d, want 5", got)
	}
	repaired, err := OpenProgram(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := repaired.PadForNote(36); got != 1 {
		t.Errorf("repaired PadForNote(36) = %d, want 1", got)
	}
}

func TestSetNoteMapEntryRange(t *testing.T) {
	p := NewProgram()
	for _, tc := range []struct{ note, pad int }{{-1, 0}, {128, 0}, {0, -1}, {0, 65}} {
		if err := p.SetNoteMapEntry(tc.note, tc.pad); err == nil {
			t.Errorf("SetNoteMapEntry(%d, %d) = nil, want error", tc.note, tc.pad)
		}
	}
	if err := p.SetNoteMapEntry(0, noPad); err != nil {
		t.Errorf("SetNoteMapEntry(0, noPad) = %v", err)
	}
}

func TestOpenProgramRawErrors(t *testing.T) {
	if _, err := OpenProgramRaw(filepath.Join(t.TempDir(), "missing.pgm")); err == nil {
		t.Error("want error for missing file")
	}
}

func TestUnknownConstBytes(t *testing.T) {
	p := NewProgram()
	if got := p.Pad(3).UnknownConstByte(); got != 1 {
		t.Errorf("pad const byte = %d, want 1", got)
	}
	if got := p.Slider(1).UnknownConstByte(); got != 1 {
		t.Errorf("slider const byte = %d, want 1", got)
	}
	p.Pad(3).SetUnknownConstByte(0)
	p.Slider(1).SetUnknownConstByte(0)
	if got := p.Pad(3).UnknownConstByte(); got != 0 {
		t.Errorf("pad const byte = %d, want 0", got)
	}
	if got := p.Slider(1).UnknownConstByte(); got != 0 {
		t.Errorf("slider const byte = %d, want 0", got)
	}
	if got := p.Pad(2).UnknownConstByte(); got != 1 {
		t.Errorf("neighbouring pad const byte = %d, want 1", got)
	}
}
