package pgm

import (
	"os"
	"path/filepath"
	"testing"
)

func testdataPath(name string) string {
	return filepath.Join("..", "..", "testdata", name)
}

func TestOpenProgram(t *testing.T) {
	prog, err := OpenProgram(testdataPath("test.pgm"))
	if err != nil {
		t.Fatalf("OpenProgram: %v", err)
	}

	if prog.PadCount() != 64 {
		t.Errorf("PadCount = %d, want 64", prog.PadCount())
	}

	// Verify header
	header := prog.buf.GetInt(headerFileSize)
	if header != ProgramFileSize {
		t.Errorf("file size header = 0x%X, want 0x%X", header, ProgramFileSize)
	}

	version := prog.buf.GetString(headerFileType)
	if version != fileVersion {
		t.Errorf("file version = %q, want %q", version, fileVersion)
	}
}

func TestPadLayerRead(t *testing.T) {
	prog, err := OpenProgram(testdataPath("test.pgm"))
	if err != nil {
		t.Fatalf("OpenProgram: %v", err)
	}

	// Read pad 0, layer 0 — should have sample "1KSN_001"
	pad0 := prog.Pad(0)
	layer0 := pad0.Layer(0)
	name := layer0.GetSampleName()
	if name != "1KSN_001" {
		t.Errorf("pad0.layer0 sample name = %q, want %q", name, "1KSN_001")
	}

	// Check level
	level := layer0.GetLevel()
	if level < 0 || level > 100 {
		t.Errorf("pad0.layer0 level = %d, out of range", level)
	}

	// Check tuning is readable (should be a reasonable value)
	tuning := layer0.GetTuning()
	if tuning < -36 || tuning > 36 {
		t.Errorf("pad0.layer0 tuning = %f, out of range", tuning)
	}
}

func TestPadMIDINote(t *testing.T) {
	prog, err := OpenProgram(testdataPath("test.pgm"))
	if err != nil {
		t.Fatalf("OpenProgram: %v", err)
	}

	pad0 := prog.Pad(0)
	note := pad0.GetMIDINote()
	if note < 0 || note > 127 {
		t.Errorf("pad0 MIDI note = %d, out of range", note)
	}

	// Set and re-read
	pad0.SetMIDINote(60) // Middle C
	if got := pad0.GetMIDINote(); got != 60 {
		t.Errorf("after SetMIDINote(60): got %d", got)
	}
}

func TestLayerSetSampleName(t *testing.T) {
	prog, err := OpenProgram(testdataPath("test.pgm"))
	if err != nil {
		t.Fatalf("OpenProgram: %v", err)
	}

	pad0 := prog.Pad(0)
	layer0 := pad0.Layer(0)

	if err := layer0.SetSampleName("NEWSAMPLE"); err != nil {
		t.Fatal(err)
	}
	if got := layer0.GetSampleName(); got != "NEWSAMPLE" {
		t.Errorf("after SetSampleName: got %q, want %q", got, "NEWSAMPLE")
	}
}

func TestLayerTuning(t *testing.T) {
	prog := NewProgram()
	layer := prog.Pad(0).Layer(0)

	layer.SetTuning(12.5)
	if got := layer.GetTuning(); got != 12.5 {
		t.Errorf("SetTuning(12.5): got %f", got)
	}

	layer.SetTuning(-3.25)
	if got := layer.GetTuning(); got != -3.25 {
		t.Errorf("SetTuning(-3.25): got %f", got)
	}
}

func TestProgramSaveAndReopen(t *testing.T) {
	prog, err := OpenProgram(testdataPath("test.pgm"))
	if err != nil {
		t.Fatalf("OpenProgram: %v", err)
	}

	// Modify something
	if err := prog.Pad(0).Layer(0).SetSampleName("MODIFIED"); err != nil {
		t.Fatal(err)
	}
	prog.Pad(0).Layer(0).SetTuning(5.0)

	// Save to temp file
	tmp := filepath.Join(t.TempDir(), "out.pgm")
	if err := prog.Save(tmp); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify file size
	info, err := os.Stat(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != ProgramFileSize {
		t.Errorf("saved file size = %d, want %d", info.Size(), ProgramFileSize)
	}

	// Reopen and verify modifications persisted
	prog2, err := OpenProgram(tmp)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}

	name := prog2.Pad(0).Layer(0).GetSampleName()
	if name != "MODIFIED" {
		t.Errorf("reopened sample name = %q, want %q", name, "MODIFIED")
	}

	tuning := prog2.Pad(0).Layer(0).GetTuning()
	if tuning != 5.0 {
		t.Errorf("reopened tuning = %f, want 5.0", tuning)
	}
}

func TestNewProgramHeader(t *testing.T) {
	prog := NewProgram()

	header := prog.buf.GetInt(headerFileSize)
	if header != ProgramFileSize {
		t.Errorf("new program file size header = 0x%X, want 0x%X", header, ProgramFileSize)
	}

	version := prog.buf.GetString(headerFileType)
	if version != fileVersion {
		t.Errorf("new program version = %q, want %q", version, fileVersion)
	}
}

func TestIterateAllPadsAndLayers(t *testing.T) {
	prog, err := OpenProgram(testdataPath("test.pgm"))
	if err != nil {
		t.Fatalf("OpenProgram: %v", err)
	}

	// Iterate all 64 pads and 4 layers each — should not panic
	sampleCount := 0
	for i := 0; i < prog.PadCount(); i++ {
		pad := prog.Pad(i)
		for j := 0; j < pad.LayerCount(); j++ {
			layer := pad.Layer(j)
			name := layer.GetSampleName()
			if name != "" {
				sampleCount++
			}
		}
	}
	// test.pgm should have at least some samples assigned
	if sampleCount == 0 {
		t.Error("no samples found in test.pgm — expected at least some")
	}
	t.Logf("found %d non-empty sample names across 64 pads x 4 layers", sampleCount)
}

func TestClone(t *testing.T) {
	prog, err := OpenProgram(testdataPath("test.pgm"))
	if err != nil {
		t.Fatalf("OpenProgram: %v", err)
	}

	clone := prog.Clone()

	// Modify original
	_ = prog.Pad(0).Layer(0).SetSampleName("ORIGINAL") //nolint:errcheck // test setup, name is short

	// Clone should be unaffected
	cloneName := clone.Pad(0).Layer(0).GetSampleName()
	if cloneName == "ORIGINAL" {
		t.Error("clone was modified when original was changed — not a deep copy")
	}
}

// assertNoteMapConsistent checks that every pad's note maps back to a pad
// playing that note, and every assigned note→pad entry is valid.
func assertNoteMapConsistent(t *testing.T, prog *Program) {
	t.Helper()
	for i := range padCount {
		n := prog.Pad(i).GetMIDINote()
		got := prog.PadForNote(n)
		if got < 0 || prog.Pad(got).GetMIDINote() != n {
			t.Errorf("pad %d plays note %d, but note→pad says %d", i, n, got)
		}
	}
	for n := range midiNoteCount {
		if pad := prog.PadForNote(n); pad >= 0 && prog.Pad(pad).GetMIDINote() != n {
			t.Errorf("note %d → pad %d, but that pad plays note %d", n, pad, prog.Pad(pad).GetMIDINote())
		}
	}
}

func TestNewProgramNoteMapAndConstBytes(t *testing.T) {
	prog := NewProgram()
	assertNoteMapConsistent(t, prog)
	if got := prog.PadForNote(35); got != 0 {
		t.Errorf("PadForNote(35) = %d, want 0", got)
	}
	if got := prog.PadForNote(0); got != -1 {
		t.Errorf("PadForNote(0) = %d, want -1 (unassigned)", got)
	}
	if got := prog.buf.GetByte(midiNoteToPad); got != noPad {
		t.Errorf("raw note→pad[0] = %d, want %d", got, noPad)
	}
	for i := range padCount {
		if got := prog.buf.GetByte(prog.Pad(i).base + padConstOneOffset); got != 1 {
			t.Errorf("pad %d byte 0x65 = %d, want 1", i, got)
		}
	}
	for s := range slidersPerProg {
		if got := prog.buf.GetByte(prog.Slider(s).base() + sliderConstOneOffset); got != 1 {
			t.Errorf("slider %d byte 1 = %d, want 1", s, got)
		}
	}
}

func TestNewProgramMatchesHardwareConstBytes(t *testing.T) {
	hw, err := OpenProgram(testdataPath("chromatic.pgm"))
	if err != nil {
		t.Fatalf("OpenProgram: %v", err)
	}
	prog := NewProgram()
	for i := range padCount {
		off := hw.Pad(i).base + padConstOneOffset
		if prog.buf.GetByte(off) != hw.buf.GetByte(off) {
			t.Errorf("pad %d byte 0x65: new=%d hardware=%d", i, prog.buf.GetByte(off), hw.buf.GetByte(off))
		}
	}
	for n := range midiNoteCount {
		if prog.PadForNote(n) != hw.PadForNote(n) {
			t.Errorf("note %d: new→pad %d, chromatic.pgm→pad %d", n, prog.PadForNote(n), hw.PadForNote(n))
		}
	}
}

func TestSetMIDINoteUpdatesNoteMap(t *testing.T) {
	prog := NewProgram()        // pad i plays 35+i
	prog.Pad(0).SetMIDINote(60) // pad 25 also plays 60
	assertNoteMapConsistent(t, prog)
	if got := prog.PadForNote(60); got != 0 {
		t.Errorf("PadForNote(60) = %d, want 0 (most recently assigned)", got)
	}
	if got := prog.PadForNote(35); got != -1 {
		t.Errorf("PadForNote(35) = %d, want -1 after its only pad moved", got)
	}

	// Moving pad 0 away from the shared note falls back to the other pad.
	prog.Pad(0).SetMIDINote(10)
	assertNoteMapConsistent(t, prog)
	if got := prog.PadForNote(60); got != 25 {
		t.Errorf("PadForNote(60) = %d, want 25", got)
	}

	// Setting the same note again is a no-op.
	prog.Pad(0).SetMIDINote(10)
	if got := prog.PadForNote(10); got != 0 {
		t.Errorf("PadForNote(10) = %d, want 0", got)
	}
}

func TestPadForNoteOutOfRange(t *testing.T) {
	prog := NewProgram()
	for _, n := range []int{-1, 128} {
		if got := prog.PadForNote(n); got != -1 {
			t.Errorf("PadForNote(%d) = %d, want -1", n, got)
		}
	}
}

func TestRepairNoteMapZeroedTable(t *testing.T) {
	prog := NewProgram()
	for n := range midiNoteCount {
		prog.buf.SetByte(midiNoteToPad+n, 0) // what old versions wrote
	}
	if !prog.RepairNoteMap() {
		t.Fatal("RepairNoteMap reported no change on a zeroed table")
	}
	assertNoteMapConsistent(t, prog)
	if got := prog.PadForNote(0); got != -1 {
		t.Errorf("PadForNote(0) = %d, want -1", got)
	}
	if prog.RepairNoteMap() {
		t.Error("second RepairNoteMap reported a change")
	}
}

func TestOpenProgramRepairsStaleEntry(t *testing.T) {
	// test.pgm has note 37 → pad 0 although pad 0 plays 60.
	raw, err := os.ReadFile(testdataPath("test.pgm"))
	if err != nil {
		t.Fatal(err)
	}
	if raw[midiNoteToPad+37] != 0 {
		t.Fatalf("fixture changed: raw note→pad[37] = %d, want 0", raw[midiNoteToPad+37])
	}
	prog, err := OpenProgram(testdataPath("test.pgm"))
	if err != nil {
		t.Fatal(err)
	}
	assertNoteMapConsistent(t, prog)
	if got := prog.PadForNote(37); got != -1 {
		t.Errorf("PadForNote(37) = %d, want -1", got)
	}
	// Valid entries are left alone: note 60 is on pads 0 and 36; file says 36.
	if got := prog.PadForNote(60); got != 36 {
		t.Errorf("PadForNote(60) = %d, want 36 (preserved from file)", got)
	}
}

func TestOpenProgramConsistentFileUnchanged(t *testing.T) {
	raw, err := os.ReadFile(testdataPath("chromatic.pgm"))
	if err != nil {
		t.Fatal(err)
	}
	prog, err := OpenProgram(testdataPath("chromatic.pgm"))
	if err != nil {
		t.Fatal(err)
	}
	if string(prog.buf.data) != string(raw) {
		t.Error("opening a consistent hardware file changed its bytes")
	}
}

func TestResetToChromaticKeepsNoteMapConsistent(t *testing.T) {
	// Mirrors the "reset to chromatic" action: renumber every pad of a
	// non-chromatic program one SetMIDINote call at a time. The result must
	// match a fresh program's tables byte for byte.
	prog, err := OpenProgram(testdataPath("test.pgm"))
	if err != nil {
		t.Fatal(err)
	}
	for i := range padCount {
		prog.Pad(i).SetMIDINote(35 + i)
	}
	assertNoteMapConsistent(t, prog)
	fresh := NewProgram()
	for n := range midiNoteCount {
		got, want := prog.PadForNote(n), fresh.PadForNote(n)
		if got != want {
			t.Errorf("PadForNote(%d) = %d, want %d", n, got, want)
		}
	}
}

func TestNewProgramMatchesHardwareDefaults(t *testing.T) {
	// chromatic.pgm is a new program saved by an MPC 1000. NewProgram must
	// reproduce it exactly: pad, envelope, filter, mixer and slider defaults,
	// both MIDI tables and the "always 1" bytes.
	raw, err := os.ReadFile(testdataPath("chromatic.pgm"))
	if err != nil {
		t.Fatal(err)
	}
	diffs := DiffBytes(raw, NewProgram().buf.Data())
	for i, d := range diffs {
		if i == 10 {
			t.Errorf("... and %d more", len(diffs)-10)
			break
		}
		t.Errorf("0x%04X %s: hardware %d, NewProgram %d", d.Offset, d.Field, d.A, d.B)
	}
}
