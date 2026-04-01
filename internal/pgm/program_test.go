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
	prog.Pad(0).Layer(0).SetSampleName("MODIFIED")
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
	prog.Pad(0).Layer(0).SetSampleName("ORIGINAL")

	// Clone should be unaffected
	cloneName := clone.Pad(0).Layer(0).GetSampleName()
	if cloneName == "ORIGINAL" {
		t.Error("clone was modified when original was changed — not a deep copy")
	}
}
