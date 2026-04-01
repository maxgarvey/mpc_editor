package pgm

import (
	"testing"
)

func TestGetSetByte(t *testing.T) {
	buf := NewEmptyBuffer(16)
	buf.SetByte(0, 42)
	buf.SetByte(5, 255)
	if got := buf.GetByte(0); got != 42 {
		t.Errorf("GetByte(0) = %d, want 42", got)
	}
	if got := buf.GetByte(5); got != 255 {
		t.Errorf("GetByte(5) = %d, want 255", got)
	}
}

func TestGetSetShort(t *testing.T) {
	buf := NewEmptyBuffer(16)

	// Positive value
	buf.SetShort(0, 1234)
	if got := buf.GetShort(0); got != 1234 {
		t.Errorf("GetShort(0) = %d, want 1234", got)
	}

	// Negative value (tuning uses signed shorts)
	buf.SetShort(2, -3600)
	if got := buf.GetShort(2); got != -3600 {
		t.Errorf("GetShort(2) = %d, want -3600", got)
	}

	// Verify little-endian byte order
	buf.SetShort(4, 0x0102) // 258
	if buf.GetByte(4) != 0x02 || buf.GetByte(5) != 0x01 {
		t.Errorf("little-endian: bytes = [%02x, %02x], want [02, 01]", buf.GetByte(4), buf.GetByte(5))
	}
}

func TestGetSetInt(t *testing.T) {
	buf := NewEmptyBuffer(16)
	buf.SetInt(0, 0x00002A04)
	if got := buf.GetInt(0); got != 0x2A04 {
		t.Errorf("GetInt(0) = %d, want %d", got, 0x2A04)
	}
	// Verify little-endian
	if buf.GetByte(0) != 0x04 || buf.GetByte(1) != 0x2A {
		t.Errorf("little-endian int: bytes = [%02x, %02x, ...], want [04, 2a, ...]",
			buf.GetByte(0), buf.GetByte(1))
	}
}

func TestGetSetString(t *testing.T) {
	buf := NewEmptyBuffer(32)

	// Normal string
	if err := buf.SetString(0, "hello"); err != nil {
		t.Fatal(err)
	}
	if got := buf.GetString(0); got != "hello" {
		t.Errorf("GetString(0) = %q, want %q", got, "hello")
	}

	// Max length string (16 chars)
	if err := buf.SetString(16, "1234567890123456"); err != nil {
		t.Fatal(err)
	}
	if got := buf.GetString(16); got != "1234567890123456" {
		t.Errorf("GetString(16) = %q, want 16-char string", got)
	}

	// Too long
	if err := buf.SetString(0, "12345678901234567"); err == nil {
		t.Error("SetString should reject strings > 16 chars")
	}

	// Overwrite shorter string (verify null padding clears old data)
	if err := buf.SetString(0, "short"); err != nil {
		t.Fatal(err)
	}
	if got := buf.GetString(0); got != "short" {
		t.Errorf("after overwrite: GetString(0) = %q, want %q", got, "short")
	}
}

func TestGetSetRange(t *testing.T) {
	buf := NewEmptyBuffer(16)
	r := Range{Low: 10, High: 127}
	buf.SetRange(0, r)
	got := buf.GetRange(0)
	if got.Low != 10 || got.High != 127 {
		t.Errorf("GetRange(0) = %+v, want {10, 127}", got)
	}
}

func TestRangeContains(t *testing.T) {
	r := Range{Low: 0, High: 100}
	if !r.Contains(50) {
		t.Error("Range{0,100}.Contains(50) should be true")
	}
	if r.Contains(101) {
		t.Error("Range{0,100}.Contains(101) should be false")
	}
	if r.Contains(-1) {
		t.Error("Range{0,100}.Contains(-1) should be false")
	}
}
