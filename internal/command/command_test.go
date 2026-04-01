package command

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/maxgarvey/mpc_editor/internal/pgm"
)

func testdataPath(name string) string {
	return filepath.Join("..", "..", "testdata", name)
}

// createTestWAV writes a minimal valid WAV file for testing.
func createTestWAV(t *testing.T, path string) {
	t.Helper()
	// Minimal 44-byte WAV header + 2 bytes of silence
	header := []byte{
		'R', 'I', 'F', 'F',
		38, 0, 0, 0, // chunk size = 38
		'W', 'A', 'V', 'E',
		'f', 'm', 't', ' ',
		16, 0, 0, 0, // subchunk1 size
		1, 0, // PCM
		1, 0, // mono
		0x44, 0xAC, 0, 0, // 44100 Hz
		0x88, 0x58, 0x01, 0, // byte rate
		2, 0, // block align
		16, 0, // bits per sample
		'd', 'a', 't', 'a',
		2, 0, 0, 0, // subchunk2 size = 2 bytes
		0, 0, // one sample of silence
	}
	if err := os.WriteFile(path, header, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestImportSamples(t *testing.T) {
	paths := []string{
		"/fake/path/kick.wav",
		"/fake/path/snare.wav",
		"/fake/path/readme.txt",
		"/fake/path/this_name_is_way_too_long_for_mpc.wav",
	}
	samples, result := ImportSamples(paths)

	if result.Imported != 4 {
		t.Errorf("imported = %d, want 4", result.Imported)
	}
	if result.Rejected != 1 {
		t.Errorf("rejected = %d, want 1", result.Rejected)
	}
	if result.Renamed != 1 {
		t.Errorf("renamed = %d, want 1", result.Renamed)
	}
	if len(samples) != 3 {
		t.Fatalf("samples = %d, want 3", len(samples))
	}
	if !result.HasError() {
		t.Error("expected HasError=true")
	}

	t.Log(result.Report())
}

func TestImportSamples_AllOK(t *testing.T) {
	paths := []string{"/fake/kick.wav", "/fake/snare.wav"}
	_, result := ImportSamples(paths)
	if result.HasError() {
		t.Error("expected no errors")
	}
	t.Log(result.Report())
}

func TestSimpleAssign_PerPad(t *testing.T) {
	prog := pgm.NewProgram()
	var matrix pgm.SampleMatrix

	samples := []*pgm.SampleRef{
		{Name: "kick", FilePath: "/fake/kick.wav", Status: pgm.SampleOK},
		{Name: "snare", FilePath: "/fake/snare.wav", Status: pgm.SampleOK},
		{Name: "hat", FilePath: "/fake/hat.wav", Status: pgm.SampleOK},
	}

	modified := SimpleAssign(prog, &matrix, samples, 0, AssignPerPad)
	if len(modified) != 3 {
		t.Errorf("modified pads = %d, want 3", len(modified))
	}

	// Check pad 0 layer 0 has "kick"
	if name := prog.Pad(0).Layer(0).GetSampleName(); name != "kick" {
		t.Errorf("pad 0 layer 0 = %q, want %q", name, "kick")
	}
	if name := prog.Pad(1).Layer(0).GetSampleName(); name != "snare" {
		t.Errorf("pad 1 layer 0 = %q, want %q", name, "snare")
	}
	if name := prog.Pad(2).Layer(0).GetSampleName(); name != "hat" {
		t.Errorf("pad 2 layer 0 = %q, want %q", name, "hat")
	}

	// Pad 0 layer 1 should be empty
	if ref := matrix.Get(0, 1); ref != nil {
		t.Error("pad 0 layer 1 should be nil")
	}
}

func TestSimpleAssign_PerLayer(t *testing.T) {
	prog := pgm.NewProgram()
	var matrix pgm.SampleMatrix

	samples := []*pgm.SampleRef{
		{Name: "kick1", FilePath: "/fake/kick1.wav", Status: pgm.SampleOK},
		{Name: "kick2", FilePath: "/fake/kick2.wav", Status: pgm.SampleOK},
		{Name: "kick3", FilePath: "/fake/kick3.wav", Status: pgm.SampleOK},
	}

	modified := SimpleAssign(prog, &matrix, samples, 0, AssignPerLayer)

	// All 3 samples should go to pad 0, layers 0-2
	if name := prog.Pad(0).Layer(0).GetSampleName(); name != "kick1" {
		t.Errorf("pad 0 layer 0 = %q, want %q", name, "kick1")
	}
	if name := prog.Pad(0).Layer(1).GetSampleName(); name != "kick2" {
		t.Errorf("pad 0 layer 1 = %q, want %q", name, "kick2")
	}
	if name := prog.Pad(0).Layer(2).GetSampleName(); name != "kick3" {
		t.Errorf("pad 0 layer 2 = %q, want %q", name, "kick3")
	}
	_ = modified
}

func TestSimpleAssign_StartOffset(t *testing.T) {
	prog := pgm.NewProgram()
	var matrix pgm.SampleMatrix

	samples := []*pgm.SampleRef{
		{Name: "snare", FilePath: "/fake/snare.wav", Status: pgm.SampleOK},
	}

	modified := SimpleAssign(prog, &matrix, samples, 4, AssignPerPad)
	if len(modified) != 1 || modified[0] != 4 {
		t.Errorf("modified = %v, want [4]", modified)
	}
	if name := prog.Pad(4).Layer(0).GetSampleName(); name != "snare" {
		t.Errorf("pad 4 layer 0 = %q, want %q", name, "snare")
	}
}

func TestMultisampleAssign(t *testing.T) {
	prog := pgm.NewProgram()
	var matrix pgm.SampleMatrix

	samples := []*pgm.SampleRef{
		{Name: "piano_C3", FilePath: "/fake/piano_C3.wav", Status: pgm.SampleOK},
		{Name: "piano_E3", FilePath: "/fake/piano_E3.wav", Status: pgm.SampleOK},
		{Name: "piano_G3", FilePath: "/fake/piano_G3.wav", Status: pgm.SampleOK},
	}

	modified, warnings := MultisampleAssign(prog, &matrix, samples)
	if len(modified) == 0 {
		t.Fatal("no pads modified")
	}

	// C3 = MIDI 60, pad index = 60-35 = 25
	if name := prog.Pad(25).Layer(0).GetSampleName(); name != "piano_C3" {
		t.Errorf("pad 25 = %q, want %q", name, "piano_C3")
	}

	// Check tuning on exact note is 0
	if tuning := prog.Pad(25).Layer(0).GetTuning(); tuning != 0 {
		t.Errorf("pad 25 tuning = %f, want 0", tuning)
	}

	t.Logf("modified %d pads, %d warnings", len(modified), len(warnings))
}

func TestExportProgram(t *testing.T) {
	// Create a program with a sample reference
	prog := pgm.NewProgram()
	prog.Pad(0).Layer(0).SetSampleName("chh")

	// Set up matrix with a real sample file
	var matrix pgm.SampleMatrix
	ref := &pgm.SampleRef{
		Name:     "chh",
		FilePath: testdataPath("chh.wav"),
		Status:   pgm.SampleOK,
	}
	matrix.Set(0, 0, ref)

	destDir := t.TempDir()
	result := ExportProgram(prog, &matrix, destDir, "test.pgm")

	if result.HasError() {
		t.Errorf("export errors: %v", result.Errors)
	}
	if result.Exported != 1 {
		t.Errorf("exported = %d, want 1", result.Exported)
	}

	// Verify files exist
	if _, err := os.Stat(filepath.Join(destDir, "test.pgm")); err != nil {
		t.Error("missing test.pgm")
	}
	if _, err := os.Stat(filepath.Join(destDir, "chh.wav")); err != nil {
		t.Error("missing chh.wav")
	}

	t.Log(result.Report())
}

func TestBatchCreate(t *testing.T) {
	// Create a temp directory tree:
	// root/
	//   drums/
	//     kick.wav
	//     snare.wav
	//   bass/
	//     bass.wav
	//   empty/
	//   existing/
	//     existing.pgm
	//     hat.wav

	root := t.TempDir()

	drumsDir := filepath.Join(root, "drums")
	bassDir := filepath.Join(root, "bass")
	emptyDir := filepath.Join(root, "empty")
	existingDir := filepath.Join(root, "existing")

	os.MkdirAll(drumsDir, 0755)
	os.MkdirAll(bassDir, 0755)
	os.MkdirAll(emptyDir, 0755)
	os.MkdirAll(existingDir, 0755)

	createTestWAV(t, filepath.Join(drumsDir, "kick.wav"))
	createTestWAV(t, filepath.Join(drumsDir, "snare.wav"))
	createTestWAV(t, filepath.Join(bassDir, "bass.wav"))
	createTestWAV(t, filepath.Join(existingDir, "hat.wav"))
	os.WriteFile(filepath.Join(existingDir, "existing.pgm"), make([]byte, 100), 0644)

	result := BatchCreate(root)

	if result.Created != 2 {
		t.Errorf("created = %d, want 2", result.Created)
	}
	if result.Skipped != 1 {
		t.Errorf("skipped = %d, want 1", result.Skipped)
	}
	if len(result.Errors) != 0 {
		t.Errorf("errors: %v", result.Errors)
	}

	// Verify .pgm files were created
	if _, err := os.Stat(filepath.Join(drumsDir, "drums.pgm")); err != nil {
		t.Error("missing drums.pgm")
	}
	if _, err := os.Stat(filepath.Join(bassDir, "bass.pgm")); err != nil {
		t.Error("missing bass.pgm")
	}

	// Verify the created programs are valid
	prog, err := pgm.OpenProgram(filepath.Join(drumsDir, "drums.pgm"))
	if err != nil {
		t.Fatalf("open drums.pgm: %v", err)
	}
	name := prog.Pad(0).Layer(0).GetSampleName()
	if name == "" {
		t.Error("drums pad 0 has no sample")
	}
	t.Logf("drums pad 0: %q", name)
	t.Log(result.Report())
}
