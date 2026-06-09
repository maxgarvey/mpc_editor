package audio

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsTranscodable(t *testing.T) {
	tests := []struct {
		ext  string
		want bool
	}{
		{".mp3", true},
		{".MP3", true},
		{".flac", true},
		{".aiff", true},
		{".wav", false}, // already WAV, no transcode needed
		{".txt", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsTranscodable(tt.ext); got != tt.want {
			t.Errorf("IsTranscodable(%q) = %v, want %v", tt.ext, got, tt.want)
		}
	}
}

func TestNormalizeWAVForMPC_AlreadyCompatible(t *testing.T) {
	// chh.wav is 16-bit 44100 Hz, so normalization is a plain byte copy.
	src := testdataPath("chh.wav")
	dst := filepath.Join(t.TempDir(), "copy.wav")

	if err := NormalizeWAVForMPC(src, dst); err != nil {
		t.Fatalf("NormalizeWAVForMPC: %v", err)
	}

	srcData, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	dstData, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if len(srcData) != len(dstData) {
		t.Errorf("copy size = %d, want %d", len(dstData), len(srcData))
	}
}

func TestNormalizeWAVForMPC_BadSource(t *testing.T) {
	// An unreadable header falls through to the ffmpeg path, which must fail
	// for garbage input regardless of whether ffmpeg is installed.
	src := filepath.Join(t.TempDir(), "garbage.wav")
	if err := os.WriteFile(src, []byte("not a wav"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "out.wav")
	if err := NormalizeWAVForMPC(src, dst); err == nil {
		t.Error("NormalizeWAVForMPC on garbage input should return an error")
	}
}

func TestCopyFileRaw_MissingSource(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "out.bin")
	if err := copyFileRaw(filepath.Join(t.TempDir(), "missing"), dst); err == nil {
		t.Error("copyFileRaw with missing source should return an error")
	}
}
