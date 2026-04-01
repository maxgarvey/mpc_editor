package pgm

import (
	"path/filepath"
	"testing"
)

func TestImportSample_OK(t *testing.T) {
	ref := ImportSample(testdataPath("chh.wav"))
	if ref.Status != SampleOK {
		t.Errorf("status = %d, want SampleOK", ref.Status)
	}
	if ref.Name != "chh" {
		t.Errorf("name = %q, want %q", ref.Name, "chh")
	}
}

func TestImportSample_InvalidExtension(t *testing.T) {
	ref := ImportSample("/some/path/file.mp3")
	if ref.Status != SampleRejected {
		t.Errorf("status = %d, want SampleRejected", ref.Status)
	}
}

func TestImportSample_TooLongName(t *testing.T) {
	// Simulate a file with a >16 char name
	ref := ImportSample("/some/path/chh45678901234567.wav")
	if ref.Status != SampleRenamed {
		t.Errorf("status = %d, want SampleRenamed", ref.Status)
	}
	if len(ref.Name) > 16 {
		t.Errorf("name %q longer than 16 chars", ref.Name)
	}
}

func TestFindSample_OK(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata")
	ref := FindSample("chh", dir)
	if ref.Status != SampleOK {
		t.Errorf("status = %d, want SampleOK", ref.Status)
	}
	if ref.FilePath == "" {
		t.Error("FilePath is empty")
	}
}

func TestFindSample_NotFound(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata")
	ref := FindSample("nonexistent", dir)
	if ref.Status != SampleNotFound {
		t.Errorf("status = %d, want SampleNotFound", ref.Status)
	}
}

func TestEscapeName(t *testing.T) {
	tests := []struct {
		name   string
		maxLen int
		want   string
	}{
		{"short", 16, "short"},
		{"exactly16chars!!", 16, "exactly16chars!!"},
		{"toolongfilename_extra", 16, "toolongfilename_"},
	}
	for _, tt := range tests {
		got := EscapeName(tt.name, tt.maxLen)
		if got != tt.want {
			t.Errorf("EscapeName(%q, %d) = %q, want %q", tt.name, tt.maxLen, got, tt.want)
		}
	}
}
