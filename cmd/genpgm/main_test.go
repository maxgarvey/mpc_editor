package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/maxgarvey/mpc_editor/internal/audio"
	"github.com/maxgarvey/mpc_editor/internal/pgm"
	"github.com/maxgarvey/mpc_editor/internal/seq"
)

// generate writes the probe set into a temp dir and returns the dir.
func generate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	var out bytes.Buffer
	if err := writeSet(dir, &out); err != nil {
		t.Fatal(err)
	}
	return dir
}

// openBoth returns a probe file as written (raw) and as OpenProgram sees it.
func openBoth(t *testing.T, dir, file string) (raw, repaired *pgm.Program) {
	t.Helper()
	path := filepath.Join(dir, file)
	raw, err := pgm.OpenProgramRaw(path)
	if err != nil {
		t.Fatalf("OpenProgramRaw(%s): %v", file, err)
	}
	repaired, err = pgm.OpenProgram(path)
	if err != nil {
		t.Fatalf("OpenProgram(%s): %v", file, err)
	}
	return raw, repaired
}

// consistent reports whether the note tables agree (RepairNoteMap is a no-op).
func consistent(p *pgm.Program) bool {
	return !p.Clone().RepairNoteMap()
}

func TestAllProbesParseAndAreFullSize(t *testing.T) {
	dir := generate(t)
	for _, pr := range probes() {
		info, err := os.Stat(filepath.Join(dir, pr.file))
		if err != nil {
			t.Fatal(err)
		}
		if info.Size() != pgm.ProgramFileSize {
			t.Errorf("%s: size %d, want %d", pr.file, info.Size(), pgm.ProgramFileSize)
		}
		_, repaired := openBoth(t, dir, pr.file)
		for i := range samplePads {
			if got := repaired.Pad(i).Layer(0).GetSampleName(); got != toneName(i) {
				t.Errorf("%s pad %d sample = %q, want %q", pr.file, i, got, toneName(i))
			}
		}
		if got := repaired.Pad(samplePads).Layer(0).GetSampleName(); got != "" {
			t.Errorf("%s: pad B1 has sample %q, want none", pr.file, got)
		}
		if len(pr.file) > 12 || strings.ToUpper(pr.file) != pr.file {
			t.Errorf("%s: name not 8.3 upper-case", pr.file)
		}
	}
}

func TestControlProbe(t *testing.T) {
	raw, _ := openBoth(t, generate(t), "P1_CTRL.PGM")
	if !consistent(raw) {
		t.Error("control tables inconsistent")
	}
	for i := range raw.PadCount() {
		if got := raw.Pad(i).GetMIDINote(); got != 35+i {
			t.Errorf("pad %d note = %d, want %d", i, got, 35+i)
		}
		if raw.Pad(i).UnknownConstByte() != 1 {
			t.Errorf("pad %d const byte != 1", i)
		}
	}
	for s := range 2 {
		sl := raw.Slider(s)
		if sl.UnknownConstByte() != 1 {
			t.Errorf("slider %d const byte != 1", s)
		}
		// Hardware default: 0x88 0x78 = -120..+120 as signed bytes.
		if r := sl.GetRange(pgm.SliderTuneRange); r.Low != 0x88 || r.High != 120 {
			t.Errorf("slider %d tune range = %+v", s, r)
		}
	}
}

func TestSwapProbeTablesDisagree(t *testing.T) {
	raw, repaired := openBoth(t, generate(t), "P2_SWAP.PGM")
	if consistent(raw) {
		t.Fatal("swap probe is consistent; want deliberate disagreement")
	}
	if raw.Pad(padA2).GetMIDINote() != 36 || raw.Pad(padA6).GetMIDINote() != 40 {
		t.Error("pad→note table changed; want control layout")
	}
	if raw.PadForNote(36) != padA6 || raw.PadForNote(40) != padA2 {
		t.Errorf("note→pad: 36→%d 40→%d, want 36→%d 40→%d", raw.PadForNote(36), raw.PadForNote(40), padA6, padA2)
	}
	// Only those two entries may disagree.
	for n := range 128 {
		if n == 36 || n == 40 {
			continue
		}
		want := -1
		if n >= 35 && n < 35+64 {
			want = n - 35
		}
		if got := raw.PadForNote(n); got != want {
			t.Errorf("note %d → %d, want %d", n, got, want)
		}
	}
	// OpenProgram repairs it, proving the library's normal behaviour is intact.
	if repaired.PadForNote(36) != padA2 || repaired.PadForNote(40) != padA6 {
		t.Error("OpenProgram did not repair the swap probe")
	}
}

func TestDupeProbeCollision(t *testing.T) {
	raw, repaired := openBoth(t, generate(t), "P3_DUPE.PGM")
	if !consistent(raw) {
		t.Error("dupe probe should be consistent (36→A3 is a valid choice)")
	}
	if raw.Pad(padA2).GetMIDINote() != 36 || raw.Pad(padA3).GetMIDINote() != 36 {
		t.Error("A2 and A3 should both be note 36")
	}
	if raw.PadForNote(36) != padA3 {
		t.Errorf("note 36 → %d, want A3 (%d), not the lowest pad", raw.PadForNote(36), padA3)
	}
	if raw.PadForNote(37) != -1 {
		t.Errorf("note 37 → %d, want no pad", raw.PadForNote(37))
	}
	if repaired.PadForNote(36) != padA3 {
		t.Error("OpenProgram changed the consistent collision winner")
	}
}

func TestConstByteProbes(t *testing.T) {
	dir := generate(t)
	pad0, _ := openBoth(t, dir, "P4_PAD0.PGM")
	for i := range pad0.PadCount() {
		if pad0.Pad(i).UnknownConstByte() != 0 {
			t.Errorf("P4 pad %d const byte != 0", i)
		}
	}
	if pad0.Slider(0).UnknownConstByte() != 1 {
		t.Error("P4 should leave slider bytes at 1")
	}
	sld0, _ := openBoth(t, dir, "P5_SLD0.PGM")
	for s := range 2 {
		if sld0.Slider(s).UnknownConstByte() != 0 {
			t.Errorf("P5 slider %d const byte != 0", s)
		}
	}
	if sld0.Pad(0).UnknownConstByte() != 1 {
		t.Error("P5 should leave pad bytes at 1")
	}

	// Each probe differs from the control only in its target bytes.
	ctrl, _ := os.ReadFile(filepath.Join(dir, "P1_CTRL.PGM"))
	for file, want := range map[string]int{"P4_PAD0.PGM": 64, "P5_SLD0.PGM": 2, "P2_SWAP.PGM": 2} {
		b, _ := os.ReadFile(filepath.Join(dir, file))
		if got := len(pgm.DiffBytes(ctrl, b)); got != want {
			t.Errorf("%s differs from control in %d bytes, want %d", file, got, want)
		}
	}
}

func TestRemapProbe(t *testing.T) {
	raw, _ := openBoth(t, generate(t), "P6_REMAP.PGM")
	if !consistent(raw) {
		t.Error("remap probe inconsistent")
	}
	for i := range raw.PadCount() {
		if raw.Pad(i).GetMIDINote() != 60+i || raw.PadForNote(60+i) != i {
			t.Errorf("pad %d: note %d, note→pad %d", i, raw.Pad(i).GetMIDINote(), raw.PadForNote(60+i))
		}
	}
	if raw.PadForNote(35) != -1 {
		t.Error("note 35 should map to no pad")
	}
}

func TestTones(t *testing.T) {
	dir := generate(t)
	for i := range samplePads {
		s, err := audio.OpenWAV(filepath.Join(dir, toneName(i)+".WAV"))
		if err != nil {
			t.Fatal(err)
		}
		if s.Format != (audio.Format{SampleRate: 44100, Channels: 1, BitsPerSample: 16}) {
			t.Errorf("pad %d format = %+v", i, s.Format)
		}
		wantFrames := beepCount(i) * (beepFrames + gapFrames)
		if s.FrameLength != wantFrames {
			t.Errorf("pad %d frames = %d, want %d", i, s.FrameLength, wantFrames)
		}
		if len(toneName(i)) > 16 {
			t.Errorf("%s exceeds the 16-char sample name field", toneName(i))
		}
	}
	if toneSummary(0) != "1 beep(s), low" || toneSummary(9) != "2 beep(s), high" || toneSummary(15) != "4 beep(s), very high" {
		t.Errorf("unexpected summaries: %q %q %q", toneSummary(0), toneSummary(9), toneSummary(15))
	}
	if rowPitch(4) != 2*rowPitch(0) {
		t.Error("rows should be an octave apart")
	}
}

func TestSeqFiles(t *testing.T) {
	dir := generate(t)
	for _, sf := range seqFiles() {
		data, err := os.ReadFile(filepath.Join(dir, sf.file))
		if err != nil {
			t.Fatal(err)
		}
		s, err := seq.Parse(data)
		if err != nil {
			t.Fatalf("%s: %v", sf.file, err)
		}
		if len(s.Events) != 2 || s.Events[0].Note != 36 || s.Events[1].Note != 40 || s.Events[1].Tick != 192 {
			t.Errorf("%s events = %+v", sf.file, s.Events)
		}
		if _, err := os.Stat(filepath.Join(dir, sf.program+".PGM")); err != nil {
			t.Errorf("%s names program %s, which is not generated", sf.file, sf.program)
		}
	}
}

func TestDiffFiles(t *testing.T) {
	dir := generate(t)
	var out bytes.Buffer
	if err := diffFiles(filepath.Join(dir, "P1_CTRL.PGM"), filepath.Join(dir, "P2_SWAP.PGM"), &out); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, want := range []string{"note→pad table: note 36", "note→pad table: note 40", "2 byte(s) differ"} {
		if !strings.Contains(s, want) {
			t.Errorf("diff output missing %q:\n%s", want, s)
		}
	}

	short := filepath.Join(dir, "short.pgm")
	if err := os.WriteFile(short, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := diffFiles(filepath.Join(dir, "P1_CTRL.PGM"), short, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "sizes differ") {
		t.Errorf("want size warning, got %q", out.String())
	}
	if err := diffFiles(filepath.Join(dir, "nope.pgm"), short, &out); err == nil {
		t.Error("want error for missing first file")
	}
	if err := diffFiles(short, filepath.Join(dir, "nope.pgm"), &out); err == nil {
		t.Error("want error for missing second file")
	}
}

func TestRun(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "probes")
	var stdout, stderr bytes.Buffer
	if code := run([]string{dir}, &stdout, &stderr); code != 0 {
		t.Fatalf("run(dir) = %d, stderr %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "P2_SWAP.PGM") {
		t.Errorf("listing missing probe: %q", stdout.String())
	}
	a, b := filepath.Join(dir, "P1_CTRL.PGM"), filepath.Join(dir, "P4_PAD0.PGM")
	stdout.Reset()
	if code := run([]string{"-diff", a, b}, &stdout, &stderr); code != 0 {
		t.Fatalf("run(-diff) = %d", code)
	}
	if !strings.Contains(stdout.String(), "64 byte(s) differ") {
		t.Errorf("diff output: %q", stdout.String())
	}

	for _, args := range [][]string{{}, {"a", "b"}, {"-diff", a}, {"-bogus"}} {
		if code := run(args, &stdout, &stderr); code != 2 {
			t.Errorf("run(%q) = %d, want 2", args, code)
		}
	}
	if code := run([]string{"-diff", a, filepath.Join(dir, "missing")}, &stdout, &stderr); code != 1 {
		t.Errorf("diff with missing file = %d, want 1", code)
	}
	// A regular file where the output directory should be.
	if code := run([]string{a}, &stdout, &stderr); code != 1 {
		t.Errorf("run(file as dir) = %d, want 1", code)
	}
}
