package device

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// mustMkdir creates a directory or fails the test.
func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

// mustWrite creates a file or fails the test.
func mustWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestDetectMPC_WithAutoload(t *testing.T) {
	base := t.TempDir()
	vol := filepath.Join(base, "MPC_CF")
	mustMkdir(t, filepath.Join(vol, "AUTOLOAD"))
	mustWrite(t, filepath.Join(vol, "test.pgm"), []byte("pgm"))
	mustWrite(t, filepath.Join(vol, "kick.wav"), []byte("wav"))

	dev := detectMPC(base)
	if dev == nil {
		t.Fatal("expected device to be detected")
	}
	if dev.VolumeName != "MPC_CF" {
		t.Errorf("VolumeName = %q, want %q", dev.VolumeName, "MPC_CF")
	}
	if dev.MountPath != vol {
		t.Errorf("MountPath = %q, want %q", dev.MountPath, vol)
	}
	if !dev.HasAutoload {
		t.Error("expected HasAutoload = true")
	}
	if dev.PGMCount != 1 {
		t.Errorf("PGMCount = %d, want 1", dev.PGMCount)
	}
}

func TestDetectMPC_NoAutoload(t *testing.T) {
	base := t.TempDir()
	vol := filepath.Join(base, "USBDRIVE")
	mustMkdir(t, vol)
	mustWrite(t, filepath.Join(vol, "readme.txt"), []byte("hi"))

	dev := detectMPC(base)
	if dev != nil {
		t.Errorf("expected nil device, got %+v", dev)
	}
}

func TestDetectMPC_EmptyBase(t *testing.T) {
	base := t.TempDir()
	dev := detectMPC(base)
	if dev != nil {
		t.Errorf("expected nil device, got %+v", dev)
	}
}

func TestDetectMPC_NonexistentBase(t *testing.T) {
	dev := detectMPC("/nonexistent/path/12345")
	if dev != nil {
		t.Errorf("expected nil device, got %+v", dev)
	}
}

func TestDetectMPC_MultiplePGMs(t *testing.T) {
	base := t.TempDir()
	vol := filepath.Join(base, "AKAI")
	mustMkdir(t, filepath.Join(vol, "AUTOLOAD"))
	for _, name := range []string{"beat1.pgm", "beat2.PGM", "beat3.pgm"} {
		mustWrite(t, filepath.Join(vol, name), []byte("pgm"))
	}

	dev := detectMPC(base)
	if dev == nil {
		t.Fatal("expected device")
	}
	if dev.PGMCount != 3 {
		t.Errorf("PGMCount = %d, want 3", dev.PGMCount)
	}
}

func TestDetector_StartAndCurrent(t *testing.T) {
	base := t.TempDir()
	d := New(WithBasePath(base), WithInterval(50*time.Millisecond))

	// No device yet.
	if d.Current() != nil {
		t.Error("expected nil before start")
	}

	// Start detector, let it poll once.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go d.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	// Still no device.
	if d.Current() != nil {
		t.Error("expected nil with empty base")
	}

	// "Connect" an MPC.
	vol := filepath.Join(base, "MPC1000")
	mustMkdir(t, filepath.Join(vol, "AUTOLOAD"))
	time.Sleep(100 * time.Millisecond)

	dev := d.Current()
	if dev == nil {
		t.Fatal("expected device after mount")
	}
	if dev.VolumeName != "MPC1000" {
		t.Errorf("VolumeName = %q, want %q", dev.VolumeName, "MPC1000")
	}

	// "Disconnect" — remove the directory.
	if err := os.RemoveAll(vol); err != nil {
		t.Fatalf("remove %s: %v", vol, err)
	}
	time.Sleep(100 * time.Millisecond)

	if d.Current() != nil {
		t.Error("expected nil after disconnect")
	}
}

func TestDetector_Scan(t *testing.T) {
	base := t.TempDir()
	vol := filepath.Join(base, "CF_CARD")
	mustMkdir(t, filepath.Join(vol, "AUTOLOAD"))

	d := New(WithBasePath(base))
	dev := d.Scan()
	if dev == nil {
		t.Fatal("expected device from Scan()")
	}
	if dev.VolumeName != "CF_CARD" {
		t.Errorf("VolumeName = %q, want %q", dev.VolumeName, "CF_CARD")
	}

	// Also accessible via Current() after Scan.
	if d.Current() == nil {
		t.Error("expected Current() to return device after Scan()")
	}
}
